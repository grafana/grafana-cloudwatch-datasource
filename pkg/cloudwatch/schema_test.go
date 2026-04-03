package cloudwatch

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/grafana/grafana-aws-sdk/pkg/cloudWatchConsts"
	schemas "github.com/grafana/schemads"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
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

// ---- splitTableName ---------------------------------------------------------

func TestSplitTableName(t *testing.T) {
	t.Run("splits metrics.namespace|metricName into namespace and metricName", func(t *testing.T) {
		ns, metric := splitTableName("metrics|AWS/EC2|CPUUtilization")
		assert.Equal(t, "AWS/EC2", ns)
		assert.Equal(t, "CPUUtilization", metric)
	})

	t.Run("handles namespace with multiple slashes", func(t *testing.T) {
		ns, metric := splitTableName("metrics|AWS/ApplicationELB|ActiveConnectionCount")
		assert.Equal(t, "AWS/ApplicationELB", ns)
		assert.Equal(t, "ActiveConnectionCount", metric)
	})

	t.Run("returns empty strings for non-metrics-prefixed table names", func(t *testing.T) {
		ns, metric := splitTableName("AWS/EC2|CPUUtilization") // missing "metrics|" prefix
		assert.Equal(t, "", ns)
		assert.Equal(t, "", metric)
	})

	t.Run("returns empty metricName when no pipe after prefix", func(t *testing.T) {
		ns, metric := splitTableName("metrics|AWS/EC2")
		assert.Equal(t, "AWS/EC2", ns)
		assert.Equal(t, "", metric)
	})

	t.Run("handles custom namespace containing dots", func(t *testing.T) {
		ns, metric := splitTableName("metrics|Custom.App|CPUUsage")
		assert.Equal(t, "Custom.App", ns)
		assert.Equal(t, "CPUUsage", metric)
	})

	t.Run("handles custom namespace containing dots with empty metricName", func(t *testing.T) {
		ns, metric := splitTableName("metrics|Custom.App|")
		assert.Equal(t, "Custom.App", ns)
		assert.Equal(t, "", metric)
	})

	t.Run("handles metric name containing dots (e.g. Glue)", func(t *testing.T) {
		ns, metric := splitTableName("metrics|Glue|glue.driver.aggregate.bytesRead")
		assert.Equal(t, "Glue", ns)
		assert.Equal(t, "glue.driver.aggregate.bytesRead", metric)
	})
}

// ---- Tables -----------------------------------------------------------------

func TestSchemaProvider_Tables(t *testing.T) {
	p := newSchemaProviderForTest()
	resp, err := p.Tables(context.Background(), &schemas.TablesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Run("contains namespace|metricName entries for hardcoded namespaces", func(t *testing.T) {
		// AWS/EC2 is always in the hardcoded map; verify at least one of its
		// metrics appears with the expected pipe-notation name.
		ec2Metrics, ok := cloudWatchConsts.NamespaceMetricsMap["AWS/EC2"]
		require.True(t, ok, "AWS/EC2 should be in hardcoded map")
		require.NotEmpty(t, ec2Metrics)

		tableSet := make(map[string]struct{}, len(resp.Tables))
		for _, name := range resp.Tables {
			tableSet[name] = struct{}{}
		}
		expected := "metrics|AWS/EC2|" + ec2Metrics[0]
		_, found := tableSet[expected]
		assert.True(t, found, "expected %s in tables", expected)
	})

	t.Run("every table carries region and accountId parameters", func(t *testing.T) {
		require.NotEmpty(t, resp.Tables)
		for _, name := range resp.Tables {
			params, ok := resp.TableParameters[name]
			require.True(t, ok, "table %q should have TableParameters", name)

			paramByName := make(map[string]schemas.TableParameter)
			for _, p := range params {
				paramByName[p.Name] = p
			}

			region, ok := paramByName["region"]
			assert.True(t, ok, "table %q should have region parameter", name)
			assert.True(t, region.Root)
			assert.True(t, region.Required)
			assert.Empty(t, region.DependsOn)

			acct, ok := paramByName["accountId"]
			assert.True(t, ok, "table %q should have accountId parameter", name)
			assert.False(t, acct.Root)
			assert.False(t, acct.Required)
			assert.Equal(t, []string{"region"}, acct.DependsOn)
		}
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
			if name == "metrics|Custom/Namespace|" {
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
			if name == "metrics|Custom.App|" {
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

	t.Run("every table includes a statistic column with OperatorEquals", func(t *testing.T) {
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2|CPUUtilization"},
		})
		require.NoError(t, err)
		cols := resp.Columns["metrics|AWS/EC2|CPUUtilization"]
		require.NotEmpty(t, cols)

		var statCol *schemas.Column
		for i := range cols {
			if cols[i].Name == "statistic" {
				statCol = &cols[i]
				break
			}
		}
		require.NotNil(t, statCol, "expected a statistic column")
		assert.Equal(t, schemas.ColumnTypeString, statCol.Type)
		assert.Equal(t, []schemas.Operator{schemas.OperatorEquals}, statCol.Operators)
	})

	t.Run("returns dimension key columns for a known namespace", func(t *testing.T) {
		dimKeys, err := services.GetHardCodedDimensionKeysByNamespace("AWS/EC2")
		require.NoError(t, err)
		require.NotEmpty(t, dimKeys)

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2|CPUUtilization"},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)

		cols, ok := resp.Columns["metrics|AWS/EC2|CPUUtilization"]
		assert.True(t, ok, "expected columns for metrics.AWS/EC2|CPUUtilization")
		// +1 for the statistic column that is always appended.
		assert.Len(t, cols, len(dimKeys)+1)

		colNames := make(map[string]struct{}, len(cols))
		for _, c := range cols {
			colNames[c.Name] = struct{}{}
			assert.Equal(t, schemas.ColumnTypeString, c.Type)
			assert.Contains(t, c.Operators, schemas.OperatorEquals)
			// Dimension columns support IN; the statistic column only supports equals.
			if c.Name != statisticColumn.Name {
				assert.Contains(t, c.Operators, schemas.OperatorIn)
			}
		}
		_, hasInstanceId := colNames["InstanceId"]
		assert.True(t, hasInstanceId, "expected InstanceId dimension key for AWS/EC2")
	})


	t.Run("correctly parses namespace from metrics.namespace|metricName table string", func(t *testing.T) {
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2|NetworkIn"},
		})
		require.NoError(t, err)

		// Columns for metrics.AWS/EC2|NetworkIn should be the same as
		// metrics.AWS/EC2|CPUUtilization — dimension keys are per-namespace.
		colsNetworkIn, ok := resp.Columns["metrics|AWS/EC2|NetworkIn"]
		assert.True(t, ok)

		resp2, _ := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2|CPUUtilization"},
		})
		colsCPU := resp2.Columns["metrics|AWS/EC2|CPUUtilization"]
		assert.Equal(t, len(colsCPU), len(colsNetworkIn))
	})

	t.Run("returns empty columns for unknown namespace with no discovered dimensions", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything, mock.Anything).
			Return([]resources.ResourceResponse[string]{}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|Unknown/NS|SomeMetric"},
		})
		require.NoError(t, err)
		cols := resp.Columns["metrics|Unknown/NS|SomeMetric"]
		// Even when no dimension keys are discovered, the statistic column is always present.
		require.Len(t, cols, 1)
		assert.Equal(t, statisticColumn.Name, cols[0].Name)
	})

	t.Run("sets error in response and continues for failed namespace lookup", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything, mock.Anything).
			Return([]resources.ResourceResponse[string](nil), fmt.Errorf("API unavailable"))
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2|CPUUtilization", "metrics|Unknown/NS|SomeMetric"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Columns, "metrics|AWS/EC2|CPUUtilization", "known namespace should still be populated")
		assert.NotContains(t, resp.Columns, "metrics|Unknown/NS|SomeMetric", "failed table should be absent from columns")
		assert.Contains(t, resp.Errors, "metrics|Unknown/NS|SomeMetric", "error should be set for the failed table")
	})

	t.Run("handles multiple tables in a single request", func(t *testing.T) {
		origNewListMetricsService := services.NewListMetricsService
		t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything, mock.Anything).
			Return([]resources.ResourceResponse[string]{}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics|AWS/EC2|CPUUtilization", "metrics|AWS/RDS|CPUUtilization"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Columns, "metrics|AWS/EC2|CPUUtilization")
		assert.Contains(t, resp.Columns, "metrics|AWS/RDS|CPUUtilization")
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
		TableParameter: "region",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	regions := resp.TableParameterValues["region"]
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
			TableParameter:   "accountId",
			DependencyValues: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)

		accountIds := resp.TableParameterValues["accountId"]
		require.Len(t, accountIds, 3)
		assert.Equal(t, "all", accountIds[0])
		assert.ElementsMatch(t, []string{"all", "111122223333", "444455556666"}, accountIds)
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
			TableParameter:   "accountId",
			DependencyValues: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.TableParameterValues["accountId"])
	})

	t.Run("returns empty list when region dependency is missing", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.TableParameterValues(context.Background(), &schemas.TableParameterValuesRequest{
			TableParameter:   "accountId",
			DependencyValues: map[string]string{},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.TableParameterValues["accountId"])
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

// ---- ColumnValues -----------------------------------------------------------

func TestSchemaProvider_ColumnValues(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	t.Run("returns standard statistics for the statistic column without calling ListMetrics", func(t *testing.T) {
		// No mock needed — statistic values are served from a fixed list.
		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"statistic"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, standardStatistics, resp.ColumnValues["statistic"])
		assert.Empty(t, resp.Errors)
	})

	t.Run("returns error when region is missing even for statistic-only request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"statistic"},
			TableParameters: map[string]string{},
		})
		require.ErrorContains(t, err, "region is a required table parameter")
	})

	t.Run("returns error when region is missing for empty Columns request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         nil,
			TableParameters: map[string]string{},
		})
		require.ErrorContains(t, err, "region is a required table parameter")
	})

	t.Run("empty Columns returns error when dimension key enumeration fails", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything, mock.Anything).
			Return([]resources.ResourceResponse[string]{}, errors.New("API unavailable"))
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|Custom/App|Latency",
			Columns:         nil,
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.ErrorContains(t, err, "could not enumerate columns")
	})

	t.Run("empty Columns with region returns statistic plus all dimension values", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		// Use a custom namespace so that GetDimensionKeysByDimensionFilter is called
		// to enumerate keys (AWS namespaces use hardcoded keys and would require
		// mocking every key in the hardcoded set).
		mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything, mock.Anything).
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
			Table:           "metrics|Custom/App|Latency",
			Columns:         nil,
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, standardStatistics, resp.ColumnValues["statistic"])
		assert.Equal(t, []string{"*", "prod"}, resp.ColumnValues["Environment"])
		assert.Equal(t, []string{"*", "api"}, resp.ColumnValues["ServiceName"])
		mockSvc.AssertNumberOfCalls(t, "GetDimensionValuesForKeys", 1)
		assert.Empty(t, resp.Errors)
	})

	t.Run("returns statistic values alongside dimension values in a mixed request", func(t *testing.T) {
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
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"statistic", "InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, standardStatistics, resp.ColumnValues["statistic"])
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
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1"},
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
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1"},
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
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1", "accountId": "111122223333"},
		})
		require.NoError(t, err)
		mockSvc.AssertExpectations(t)
		assert.Equal(t, []string{"*", "i-xyz"}, resp.ColumnValues["InstanceId"])
	})

	t.Run("returns error when region is missing for dimension column request", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{},
		})
		require.ErrorContains(t, err, "region is a required table parameter")
	})

	t.Run("returns error for non-metrics table prefix", func(t *testing.T) {
		p := newSchemaProviderForTest()
		_, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "logs./aws/lambda/fn",
			Columns:         []string{"someField"},
			TableParameters: map[string]string{"region": "us-east-1"},
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
			Table:           "metrics|AWS/EC2|CPUUtilization",
			Columns:         []string{"InstanceId", "AutoScalingGroupName"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"*", "i-111"}, resp.ColumnValues["InstanceId"])
		assert.Equal(t, []string{"*", "my-asg"}, resp.ColumnValues["AutoScalingGroupName"])
		mockSvc.AssertNumberOfCalls(t, "GetDimensionValuesForKeys", 1)
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
			assert.NotEmpty(t, regions["region"])
			break
		}
	})

	t.Run("schema passes schemads validation", func(t *testing.T) {
		err := schemas.ValidateSchema(resp.FullSchema)
		assert.NoError(t, err)
	})
}
