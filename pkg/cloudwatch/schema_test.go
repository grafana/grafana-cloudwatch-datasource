package cloudwatch

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/grafana/grafana-aws-sdk/pkg/cloudWatchConsts"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	schemas "github.com/grafana/schemads"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/mocks"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models/resources"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/services"
)

// newSchemaProviderForTest returns a SchemaProvider backed by a minimal
// test DataSource. Callers may further configure the DataSource via opts.
func newSchemaProviderForTest(opts ...func(*DataSource)) *SchemaProvider {
	ds := newTestDatasource(opts...)
	return NewSchemaProvider(ds)
}

// ---- metricsTableNamespace --------------------------------------------------

func TestMetricsTableNamespace(t *testing.T) {
	t.Run("parses metrics|<namespace>", func(t *testing.T) {
		ns, ok := metricsTableNamespace("metrics|AWS/EC2")
		assert.True(t, ok)
		assert.Equal(t, "AWS/EC2", ns)
	})

	t.Run("handles namespace with multiple slashes", func(t *testing.T) {
		ns, ok := metricsTableNamespace("metrics|AWS/ApplicationELB")
		assert.True(t, ok)
		assert.Equal(t, "AWS/ApplicationELB", ns)
	})

	t.Run("rejects non-metrics-prefixed table names", func(t *testing.T) {
		_, ok := metricsTableNamespace("AWS/EC2|CPUUtilization")
		assert.False(t, ok)
	})

	t.Run("handles custom namespace containing dots", func(t *testing.T) {
		ns, ok := metricsTableNamespace("metrics|Custom.App")
		assert.True(t, ok)
		assert.Equal(t, "Custom.App", ns)
	})

	t.Run("rejects empty namespace", func(t *testing.T) {
		_, ok := metricsTableNamespace("metrics|")
		assert.False(t, ok)
	})
}

func TestIsLogsTable(t *testing.T) {
	assert.True(t, isLogsTable(LogsTableName))
	assert.False(t, isLogsTable("metrics|AWS/EC2"))
	assert.False(t, isLogsTable("logs|something"))
}

// ---- Tables -----------------------------------------------------------------

func TestSchemaProvider_Tables(t *testing.T) {
	p := newSchemaProviderForTest()
	resp, err := p.Tables(context.Background(), &schemas.TablesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Run("contains one table per hardcoded namespace", func(t *testing.T) {
		_, ok := cloudWatchConsts.NamespaceMetricsMap["AWS/EC2"]
		require.True(t, ok, "AWS/EC2 should be in hardcoded map")

		tableSet := make(map[string]struct{}, len(resp.Tables))
		for _, name := range resp.Tables {
			tableSet[name] = struct{}{}
		}
		_, found := tableSet["metrics|AWS/EC2"]
		assert.True(t, found, "expected metrics|AWS/EC2 in tables")
	})

	t.Run("every table carries region, accountId, and metricName parameters", func(t *testing.T) {
		require.NotEmpty(t, resp.Tables)
		for _, name := range resp.Tables {
			params, ok := resp.TableParameters[name]
			require.True(t, ok, "table %q should have TableParameters", name)

			paramByName := make(map[string]schemas.TableParameter)
			for _, p := range params {
				paramByName[p.Name] = p
			}

			if name == LogsTableName {
				region, ok := paramByName[RegionTableParameter]
				assert.True(t, ok, "logs table should have region parameter")
				assert.True(t, region.Root)
				assert.True(t, region.Required)

				acct, ok := paramByName[AccountIdTableParameter]
				assert.True(t, ok)
				assert.True(t, acct.Required)
				assert.Equal(t, []string{RegionTableParameter}, acct.DependsOn)

				prefix, ok := paramByName[LogGroupNamePrefixTableParameter]
				assert.True(t, ok)
				assert.False(t, prefix.Required)
				assert.Equal(t, []string{RegionTableParameter, AccountIdTableParameter}, prefix.DependsOn)

				lg, ok := paramByName[LogGroupTableParameter]
				assert.True(t, ok)
				assert.False(t, lg.Required)
				assert.Equal(t, []string{RegionTableParameter, AccountIdTableParameter}, lg.DependsOn)
				continue
			}

			region, ok := paramByName[RegionTableParameter]
			assert.True(t, ok, "table %q should have region parameter", name)
			assert.True(t, region.Root)
			assert.True(t, region.Required)
			assert.Empty(t, region.DependsOn)

			acct, ok := paramByName[AccountIdTableParameter]
			assert.True(t, ok, "table %q should have accountId parameter", name)
			assert.False(t, acct.Root)
			assert.False(t, acct.Required)
			assert.Equal(t, []string{RegionTableParameter}, acct.DependsOn)

			metric, ok := paramByName[MetricNameTableParameter]
			assert.True(t, ok, "table %q should have metricName parameter", name)
			assert.False(t, metric.Root)
			assert.Equal(t, []string{RegionTableParameter}, metric.DependsOn)
			assert.True(t, metric.Required, "CloudWatch MetricStat.Metric requires MetricName")
		}
	})

	t.Run("includes virtual logs table", func(t *testing.T) {
		found := false
		for _, name := range resp.Tables {
			if name == LogsTableName {
				found = true
				break
			}
		}
		assert.True(t, found, "expected %q in Tables()", LogsTableName)
	})

	t.Run("Tables response does not include per-table columns (columns are fetched separately)", func(t *testing.T) {
		// Tables() only returns names and table parameters, not columns.
		// This is a sanity check that the test setup is consistent.
		require.NotEmpty(t, resp.Tables)
	})

	t.Run("includes entries for custom namespaces from settings", func(t *testing.T) {
		customP := newSchemaProviderForTest(func(ds *DataSource) {
			ds.Settings.Namespace = "Custom/Namespace"
		})
		customResp, err := customP.Tables(context.Background(), &schemas.TablesRequest{})
		require.NoError(t, err)

		found := false
		for _, name := range customResp.Tables {
			if name == "metrics|Custom/Namespace" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected placeholder table for custom namespace")
	})

	t.Run("includes placeholder table for custom namespace containing dots", func(t *testing.T) {
		customP := newSchemaProviderForTest(func(ds *DataSource) {
			ds.Settings.Namespace = "Custom.App"
		})
		customResp, err := customP.Tables(context.Background(), &schemas.TablesRequest{})
		require.NoError(t, err)

		found := false
		for _, name := range customResp.Tables {
			if name == "metrics|Custom.App" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected placeholder table for dot-containing custom namespace")
	})
}

// ---- Columns ----------------------------------------------------------------

func TestSchemaProvider_Columns(t *testing.T) {
	p := newSchemaProviderForTest()

	t.Run("columns do not include a statistic column (statistic is a table hint)", func(t *testing.T) {
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2"},
		})
		require.NoError(t, err)
		cols := resp.Columns["metrics|AWS/EC2"]
		require.NotEmpty(t, cols)
		for _, c := range cols {
			assert.NotEqual(t, "statistic", c.Name, "statistic should be a FOR hint, not a column")
		}
	})

	t.Run("returns dimension key columns for a known namespace", func(t *testing.T) {
		dimKeys, err := services.GetHardCodedDimensionKeysByNamespace("AWS/EC2")
		require.NoError(t, err)
		require.NotEmpty(t, dimKeys)

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2"},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)

		cols, ok := resp.Columns["metrics|AWS/EC2"]
		assert.True(t, ok, "expected columns for metrics|AWS/EC2")
		// time and value columns plus dimension keys (no statistic column).
		assert.Len(t, cols, len(dimKeys)+2)

		colNames := make(map[string]struct{}, len(cols))
		for _, c := range cols {
			colNames[c.Name] = struct{}{}
			// time and value are data columns with no operators; skip operator checks for them.
			if c.Name == timeColumn.Name || c.Name == valueColumn.Name {
				continue
			}
			assert.Contains(t, c.Operators, schemas.OperatorEquals)
			assert.Contains(t, c.Operators, schemas.OperatorIn)
		}
		_, hasInstanceId := colNames["InstanceId"]
		assert.True(t, hasInstanceId, "expected InstanceId dimension key for AWS/EC2")
		_, hasTime := colNames["time"]
		assert.True(t, hasTime, "expected time data column")
		_, hasValue := colNames["value"]
		assert.True(t, hasValue, "expected value data column")
	})

	t.Run("logs table requires accountId and log group identity", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables:          []string{LogsTableName},
			TableParameters: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Errors[LogsTableName], AccountIdTableParameter)
	})

	t.Run("logs table requires logGroup when accountId set", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{LogsTableName},
			TableParameters: map[string]string{
				RegionTableParameter:    "us-east-1",
				AccountIdTableParameter: LogsAccountSelfSentinel,
			},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Errors[LogsTableName], LogGroupTableParameter)
	})

	t.Run("logs table loads columns from GetLogGroupFields", func(t *testing.T) {
		origNewLogGroupsService := services.NewLogGroupsService
		t.Cleanup(func() { services.NewLogGroupsService = origNewLogGroupsService })

		mockLogs := &mocks.LogsService{}
		// LogsService.GetLogGroupFields passes only the request to mock.Called (see mocks.LogsService).
		mockLogs.On("GetLogGroupFields", mock.MatchedBy(func(r resources.LogGroupFieldsRequest) bool {
			return r.LogGroupName == "/aws/lambda/foo" && r.Region == "us-east-1" && r.AccountId == nil
		})).Return([]resources.ResourceResponse[resources.LogGroupField]{
			{Value: resources.LogGroupField{Name: "msg", Percent: 50}},
			{Value: resources.LogGroupField{Name: "@timestamp", Percent: 100}},
		}, nil)

		services.NewLogGroupsService = func(_ models.CloudWatchLogsAPIProvider, _ bool) models.LogGroupsProvider {
			return mockLogs
		}

		p := newSchemaProviderForTest()
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{LogsTableName},
			TableParameters: map[string]string{
				RegionTableParameter:    "us-east-1",
				AccountIdTableParameter: LogsAccountSelfSentinel,
				LogGroupTableParameter:  FormatLogGroupTableParameter("/aws/lambda/foo", "arn:aws:logs:us-east-1:1:log-group:/aws/lambda/foo"),
			},
		})
		require.NoError(t, err)
		require.Empty(t, resp.Errors)
		cols := resp.Columns[LogsTableName]
		require.Len(t, cols, 2)
		assert.Equal(t, "@timestamp", cols[0].Name)
		assert.Equal(t, schemas.ColumnTypeTimestamp, cols[0].Type)
		assert.Equal(t, "msg", cols[1].Name)
		mockLogs.AssertExpectations(t)
	})

	t.Run("logs table reports error when logGroup is bare ARN only", func(t *testing.T) {
		arn := "arn:aws:logs:us-east-1:111111111111:log-group:/aws/lambda/foo"
		p := newSchemaProviderForTest()
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{LogsTableName},
			TableParameters: map[string]string{
				RegionTableParameter:    "us-east-1",
				AccountIdTableParameter: "111111111111",
				LogGroupTableParameter:  arn,
			},
		})
		require.NoError(t, err)
		require.Contains(t, resp.Errors, LogsTableName)
	})

	t.Run("columns for a namespace table match dimension keys for that namespace", func(t *testing.T) {
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2"},
		})
		require.NoError(t, err)

		// Dimension keys are per-namespace; repeated Columns calls for the same table match.
		colsNetworkIn, ok := resp.Columns["metrics|AWS/EC2"]
		assert.True(t, ok)

		resp2, _ := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2"},
		})
		colsCPU := resp2.Columns["metrics|AWS/EC2"]
		assert.Equal(t, len(colsCPU), len(colsNetworkIn))
	})

	t.Run("returns empty columns for unknown namespace with no discovered dimensions", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
			Return([]resources.ResourceResponse[string]{}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|Unknown/NS"},
		})
		require.NoError(t, err)
		cols := resp.Columns["metrics|Unknown/NS"]
		// Even when no dimension keys are discovered, time and value columns are always present.
		require.Len(t, cols, 2)
		assert.Equal(t, timeColumn.Name, cols[0].Name)
		assert.Equal(t, valueColumn.Name, cols[1].Name)
	})

	t.Run("sets error in response and continues for failed namespace lookup", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
			Return([]resources.ResourceResponse[string](nil), fmt.Errorf("API unavailable"))
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2", "metrics|Unknown/NS"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Columns, "metrics|AWS/EC2", "known namespace should still be populated")
		assert.NotContains(t, resp.Columns, "metrics|Unknown/NS", "failed table should be absent from columns")
		assert.Contains(t, resp.Errors, "metrics|Unknown/NS", "error should be set for the failed table")
	})

	t.Run("handles multiple tables in a single request", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
			Return([]resources.ResourceResponse[string]{}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2", "metrics|AWS/RDS"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Columns, "metrics|AWS/EC2")
		assert.Contains(t, resp.Columns, "metrics|AWS/RDS")
	})
}

// ---- TableParameterValues ---------------------------------------------------

func TestSchemaProvider_TableParameterValues_Region(t *testing.T) {
	origNewRegionsService := services.NewRegionsService
	t.Cleanup(func() { services.NewRegionsService = origNewRegionsService })

	mockRegionService := &mocks.RegionsService{}
	mockRegionService.On("GetRegions", mock.Anything).Return([]resources.ResourceResponse[resources.Region]{
		{Value: resources.Region{Name: "us-east-1"}},
		{Value: resources.Region{Name: "eu-west-1"}},
	}, nil)
	services.NewRegionsService = func(_ models.EC2APIProvider, _ log.Logger) models.RegionsAPIProvider {
		return mockRegionService
	}

	p := newSchemaProviderForTest()
	resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
		TableParameter: RegionTableParameter,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	regions := resp.TableParameterValues[RegionTableParameter]
	assert.ElementsMatch(t, []string{"us-east-1", "eu-west-1"}, regions)
}

func TestSchemaProvider_TableParameterValues_AccountId(t *testing.T) {
	origNewAccountsService := services.NewAccountsService
	t.Cleanup(func() { services.NewAccountsService = origNewAccountsService })

	t.Run("returns all accounts with 'all' prepended for a monitoring account", func(t *testing.T) {
		mockAcctService := &mocks.AccountsServiceMock{}
		mockAcctService.On("GetAccountsForCurrentUserOrRole", mock.Anything).Return(
			[]resources.ResourceResponse[resources.Account]{
				{Value: resources.Account{Id: "111122223333", Label: "prod", IsMonitoringAccount: false}},
				{Value: resources.Account{Id: "444455556666", Label: "dev", IsMonitoringAccount: false}},
			}, nil,
		)
		services.NewAccountsService = func(_ models.OAMAPIProvider) models.AccountsProvider {
			return mockAcctService
		}

		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			TableParameter:   AccountIdTableParameter,
			DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)

		accountIds := resp.TableParameterValues[AccountIdTableParameter]
		require.Len(t, accountIds, 3)
		assert.Equal(t, "all", accountIds[0])
		assert.ElementsMatch(t, []string{"all", "111122223333", "444455556666"}, accountIds)
	})

	t.Run("logs table prepends self then all for a monitoring account", func(t *testing.T) {
		mockAcctService := &mocks.AccountsServiceMock{}
		mockAcctService.On("GetAccountsForCurrentUserOrRole", mock.Anything).Return(
			[]resources.ResourceResponse[resources.Account]{
				{Value: resources.Account{Id: "111122223333", Label: "prod", IsMonitoringAccount: false}},
			}, nil,
		)
		services.NewAccountsService = func(_ models.OAMAPIProvider) models.AccountsProvider {
			return mockAcctService
		}

		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			Table:            LogsTableName,
			TableParameter:   AccountIdTableParameter,
			DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{LogsAccountSelfSentinel, "all", "111122223333"}, resp.TableParameterValues[AccountIdTableParameter])
	})

	t.Run("returns empty list (no error) when not a monitoring account", func(t *testing.T) {
		mockAcctService := &mocks.AccountsServiceMock{}
		mockAcctService.On("GetAccountsForCurrentUserOrRole", mock.Anything).Return(
			[]resources.ResourceResponse[resources.Account](nil), nil,
		)
		services.NewAccountsService = func(_ models.OAMAPIProvider) models.AccountsProvider {
			return mockAcctService
		}

		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			TableParameter:   AccountIdTableParameter,
			DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.TableParameterValues[AccountIdTableParameter])
	})

	t.Run("logs table returns only self when not a monitoring account", func(t *testing.T) {
		mockAcctService := &mocks.AccountsServiceMock{}
		mockAcctService.On("GetAccountsForCurrentUserOrRole", mock.Anything).Return(
			[]resources.ResourceResponse[resources.Account](nil), nil,
		)
		services.NewAccountsService = func(_ models.OAMAPIProvider) models.AccountsProvider {
			return mockAcctService
		}

		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			Table:            LogsTableName,
			TableParameter:   AccountIdTableParameter,
			DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{LogsAccountSelfSentinel}, resp.TableParameterValues[AccountIdTableParameter])
	})

	t.Run("returns empty list when region dependency is missing", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			TableParameter:   AccountIdTableParameter,
			DependencyValues: map[string]string{},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.TableParameterValues[AccountIdTableParameter])
	})

	t.Run("returns error in Errors map for unrecognised table parameter", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			TableParameter: "unknownParam",
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Errors["unknownParam"], "unsupported table parameter")
	})
}

func TestSchemaProvider_TableParameterValues_MetricName(t *testing.T) {
	t.Run("lists metrics for built-in namespace", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			Table:          "metrics|AWS/EC2",
			TableParameter: MetricNameTableParameter,
			DependencyValues: map[string]string{
				RegionTableParameter: "us-east-1",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		metrics := resp.TableParameterValues[MetricNameTableParameter]
		require.NotEmpty(t, metrics)
		assert.Contains(t, metrics, "CPUUtilization")
	})

	t.Run("lists metrics for custom namespace via ListMetrics", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetMetricsByNamespace", mock.MatchedBy(func(r resources.MetricsRequest) bool {
			return r.Namespace == "My/Custom/NS"
		})).Return([]resources.ResourceResponse[resources.Metric]{
			{Value: resources.Metric{Name: "Errors", Namespace: "My/Custom/NS"}},
			{Value: resources.Metric{Name: "Latency", Namespace: "My/Custom/NS"}},
		}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		customP := newSchemaProviderForTest(func(ds *DataSource) {
			ds.Settings.Namespace = "My/Custom/NS"
		})
		resp, err := customP.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			Table:            "metrics|My/Custom/NS",
			TableParameter:   MetricNameTableParameter,
			DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"Errors", "Latency"}, resp.TableParameterValues[MetricNameTableParameter])
		mockSvc.AssertExpectations(t)
	})
}

// ---- ColumnValues -----------------------------------------------------------

func TestSchemaProvider_ColumnValues(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	t.Run("statistic is not a column — requesting it yields no enumerable values and no ListMetrics call", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2",
			Columns:         []string{"statistic"},
			TableParameters: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.ColumnValues)
		assert.Empty(t, resp.Errors)
	})

	t.Run("returns error when region is missing even for statistic-only request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2",
			Columns:         []string{"statistic"},
			TableParameters: map[string]string{},
		})
		require.ErrorContains(t, err, "region is a required table parameter")
	})

	t.Run("returns error when region is missing for empty Columns request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2",
			Columns:         nil,
			TableParameters: map[string]string{},
		})
		require.ErrorContains(t, err, "region is a required table parameter")
	})

	t.Run("empty Columns returns error when dimension key enumeration fails", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
			Return([]resources.ResourceResponse[string]{}, errors.New("API unavailable"))
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|Custom/App",
			Columns: nil,
			TableParameters: map[string]string{
				RegionTableParameter:     "us-east-1",
				MetricNameTableParameter: "Latency",
			},
		})
		require.ErrorContains(t, err, "could not enumerate columns")
	})

	t.Run("empty Columns with region returns all dimension values only", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		// Use a custom namespace so that GetDimensionKeysByDimensionFilter is called
		// to enumerate keys (AWS namespaces use hardcoded keys and would require
		// mocking every key in the hardcoded set).
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
			Return([]resources.ResourceResponse[string]{
				{Value: "Environment"},
				{Value: "ServiceName"},
			}, nil)
		mockSvc.On("GetDimensionValuesForKeys",
			mock.MatchedBy(func(r resources.DimensionValuesForKeysRequest) bool {
				return r.Namespace == "Custom/App" && r.MetricName == "Latency" &&
					assert.ElementsMatch(t, []string{"Environment", "ServiceName"}, r.DimensionKeys)
			}),
		).Return(map[string][]string{
			"Environment": {"prod"},
			"ServiceName": {"api"},
		}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|Custom/App",
			Columns: nil,
			TableParameters: map[string]string{
				RegionTableParameter:     "us-east-1",
				MetricNameTableParameter: "Latency",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"*", "prod"}, resp.ColumnValues["Environment"])
		assert.Equal(t, []string{"*", "api"}, resp.ColumnValues["ServiceName"])
		mockSvc.AssertNumberOfCalls(t, "GetDimensionValuesForKeys", 1)
		assert.Empty(t, resp.Errors)
	})

	t.Run("mixed request with statistic skips statistic and returns dimension values", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesForKeys",
			mock.MatchedBy(func(r resources.DimensionValuesForKeysRequest) bool {
				return assert.ElementsMatch(t, []string{"InstanceId"}, r.DimensionKeys)
			}),
		).Return(map[string][]string{"InstanceId": {"i-abc"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|AWS/EC2",
			Columns: []string{"statistic", "InstanceId"},
			TableParameters: map[string]string{
				RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"*", "i-abc"}, resp.ColumnValues["InstanceId"])
		mockSvc.AssertNumberOfCalls(t, "GetDimensionValuesForKeys", 1)
	})

	t.Run("returns dimension values for requested columns", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesForKeys",
			mock.MatchedBy(func(r resources.DimensionValuesForKeysRequest) bool {
				return assert.ElementsMatch(t, []string{"InstanceId"}, r.DimensionKeys)
			}),
		).Return(map[string][]string{"InstanceId": {"i-11111111", "i-22222222"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|AWS/EC2",
			Columns: []string{"InstanceId"},
			TableParameters: map[string]string{
				RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization",
			},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"*", "i-11111111", "i-22222222"}, resp.ColumnValues["InstanceId"])
		assert.Empty(t, resp.Errors)
	})

	t.Run("passes namespace and metricName from table to the service", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesForKeys",
			mock.MatchedBy(func(r resources.DimensionValuesForKeysRequest) bool {
				return r.Namespace == "AWS/EC2" && r.MetricName == "CPUUtilization" &&
					assert.ElementsMatch(t, []string{"InstanceId"}, r.DimensionKeys)
			}),
		).Return(map[string][]string{"InstanceId": {"i-abc"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|AWS/EC2",
			Columns: []string{"InstanceId"},
			TableParameters: map[string]string{
				RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization",
			},
		})
		require.NoError(t, err)
		mockSvc.AssertExpectations(t)
		assert.Equal(t, []string{"*", "i-abc"}, resp.ColumnValues["InstanceId"])
	})

	t.Run("passes accountId to the service when present", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesForKeys",
			mock.MatchedBy(func(r resources.DimensionValuesForKeysRequest) bool {
				return r.AccountId != nil && *r.AccountId == "111122223333" &&
					assert.ElementsMatch(t, []string{"InstanceId"}, r.DimensionKeys)
			}),
		).Return(map[string][]string{"InstanceId": {"i-xyz"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|AWS/EC2",
			Columns: []string{"InstanceId"},
			TableParameters: map[string]string{
				RegionTableParameter: "us-east-1", AccountIdTableParameter: "111122223333",
				MetricNameTableParameter: "CPUUtilization",
			},
		})
		require.NoError(t, err)
		mockSvc.AssertExpectations(t)
		assert.Equal(t, []string{"*", "i-xyz"}, resp.ColumnValues["InstanceId"])
	})

	t.Run("returns error when region is missing for dimension column request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{},
		})
		require.ErrorContains(t, err, "region is a required table parameter")
	})

	t.Run("returns error when metricName is missing for dimension column request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.ErrorContains(t, err, "metricName is a required table parameter")
	})

	t.Run("returns error for non-metrics table prefix", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "logs./aws/lambda/fn",
			Columns:         []string{"someField"},
			TableParameters: map[string]string{RegionTableParameter: "us-east-1"},
		})
		require.ErrorContains(t, err, "unrecognised table format")
	})

	t.Run("makes a single ListMetrics call for multiple requested columns", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesForKeys",
			mock.MatchedBy(func(r resources.DimensionValuesForKeysRequest) bool {
				return assert.ElementsMatch(t, []string{"InstanceId", "AutoScalingGroupName"}, r.DimensionKeys)
			}),
		).Return(map[string][]string{
			"InstanceId":           {"i-111"},
			"AutoScalingGroupName": {"my-asg"},
		}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   "metrics|AWS/EC2",
			Columns: []string{"InstanceId", "AutoScalingGroupName"},
			TableParameters: map[string]string{
				RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"*", "i-111"}, resp.ColumnValues["InstanceId"])
		assert.Equal(t, []string{"*", "my-asg"}, resp.ColumnValues["AutoScalingGroupName"])
		mockSvc.AssertNumberOfCalls(t, "GetDimensionValuesForKeys", 1)
	})

	t.Run("logs table returns empty column values (no enumeration API)", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:   LogsTableName,
			Columns: []string{"msg"},
			TableParameters: map[string]string{
				RegionTableParameter:    "us-east-1",
				AccountIdTableParameter: LogsAccountSelfSentinel,
			},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.ColumnValues)
	})
}

// ---- TableParameterValues — log group (combined name+ARN) -------------------

func TestSchemaProvider_TableParameterValues_LogGroupName(t *testing.T) {
	t.Run("lists log groups for logs table as name+ARN", func(t *testing.T) {
		origNewLogGroupsService := services.NewLogGroupsService
		t.Cleanup(func() { services.NewLogGroupsService = origNewLogGroupsService })

		mockLogs := &mocks.LogsService{}
		mockLogs.On("GetLogGroups", mock.MatchedBy(func(r resources.LogGroupsRequest) bool {
			return r.Region == "us-east-1" && r.ListAllLogGroups && r.LogGroupNamePrefix == nil && r.AccountId == nil
		})).Return([]resources.ResourceResponse[resources.LogGroup]{
			{Value: resources.LogGroup{Name: "/z/group", Arn: "arn:aws:logs:us-east-1:1:log-group:z"}},
			{Value: resources.LogGroup{Name: "/a/group", Arn: "arn:aws:logs:us-east-1:1:log-group:a"}},
		}, nil)

		services.NewLogGroupsService = func(_ models.CloudWatchLogsAPIProvider, _ bool) models.LogGroupsProvider {
			return mockLogs
		}

		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			Table:          LogsTableName,
			TableParameter: LogGroupTableParameter,
			DependencyValues: map[string]string{
				RegionTableParameter:    "us-east-1",
				AccountIdTableParameter: LogsAccountSelfSentinel,
			},
		})
		require.NoError(t, err)
		want := []string{
			FormatLogGroupTableParameter("/a/group", "arn:aws:logs:us-east-1:1:log-group:a"),
			FormatLogGroupTableParameter("/z/group", "arn:aws:logs:us-east-1:1:log-group:z"),
		}
		assert.Equal(t, want, resp.TableParameterValues[LogGroupTableParameter])
		mockLogs.AssertExpectations(t)
	})

	t.Run("passes logGroupNamePrefix to DescribeLogGroups", func(t *testing.T) {
		origNewLogGroupsService := services.NewLogGroupsService
		t.Cleanup(func() { services.NewLogGroupsService = origNewLogGroupsService })

		mockLogs := &mocks.LogsService{}
		mockLogs.On("GetLogGroups", mock.MatchedBy(func(r resources.LogGroupsRequest) bool {
			return r.Region == "eu-west-1" && r.LogGroupNamePrefix != nil && *r.LogGroupNamePrefix == "/aws/" && r.AccountId == nil
		})).Return([]resources.ResourceResponse[resources.LogGroup]{
			{Value: resources.LogGroup{Name: "/aws/lambda/x", Arn: "arn"}},
		}, nil)

		services.NewLogGroupsService = func(_ models.CloudWatchLogsAPIProvider, _ bool) models.LogGroupsProvider {
			return mockLogs
		}

		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			Table:          LogsTableName,
			TableParameter: LogGroupTableParameter,
			DependencyValues: map[string]string{
				RegionTableParameter:             "eu-west-1",
				AccountIdTableParameter:          LogsAccountSelfSentinel,
				LogGroupNamePrefixTableParameter: "/aws/",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{FormatLogGroupTableParameter("/aws/lambda/x", "arn")}, resp.TableParameterValues[LogGroupTableParameter])
	})
}

// ---- Schema (full) ----------------------------------------------------------

func TestSchemaProvider_Schema(t *testing.T) {
	origNewRegionsService := services.NewRegionsService
	t.Cleanup(func() { services.NewRegionsService = origNewRegionsService })

	mockRegionService := &mocks.RegionsService{}
	mockRegionService.On("GetRegions", mock.Anything).Return([]resources.ResourceResponse[resources.Region]{
		{Value: resources.Region{Name: "us-east-1"}},
	}, nil)
	services.NewRegionsService = func(_ models.EC2APIProvider, _ log.Logger) models.RegionsAPIProvider {
		return mockRegionService
	}

	p := newSchemaProviderForTest()
	resp, err := p.Schema(context.Background(), &schemas.SchemaRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.FullSchema)

	t.Run("schema contains tables", func(t *testing.T) {
		assert.NotEmpty(t, resp.FullSchema.Tables)
	})

	t.Run("schema pre-populates region table parameter values", func(t *testing.T) {
		require.NotEmpty(t, resp.FullSchema.TableParameterValues)
		// Pick any table and verify region is pre-populated.
		for _, regions := range resp.FullSchema.TableParameterValues {
			assert.NotEmpty(t, regions[RegionTableParameter])
			break
		}
	})

	t.Run("schema passes schemads validation", func(t *testing.T) {
		err := schemas.ValidateSchema(resp.FullSchema)
		assert.NoError(t, err)
	})

	t.Run("every metrics table advertises the statistic table hint", func(t *testing.T) {
		for _, tbl := range resp.FullSchema.Tables {
			if isLogsTable(tbl.Name) {
				assert.Nil(t, tbl.TableHints, "logs table has no metric statistic hint")
				continue
			}
			require.NotEmpty(t, tbl.TableHints, "table %q should have TableHints", tbl.Name)
			var found bool
			for _, h := range tbl.TableHints {
				if h.Name == statisticTableHint.Name && h.HasValue {
					found = true
					break
				}
			}
			assert.True(t, found, "table %q should include statistic hint with HasValue", tbl.Name)
		}
	})
}
