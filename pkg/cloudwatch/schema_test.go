package cloudwatch

import (
	"context"
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
	t.Run("splits metrics.namespace.metricName into namespace and metricName", func(t *testing.T) {
		ns, metric := splitTableName("metrics.AWS/EC2.CPUUtilization")
		assert.Equal(t, "AWS/EC2", ns)
		assert.Equal(t, "CPUUtilization", metric)
	})

	t.Run("handles namespace with multiple slashes", func(t *testing.T) {
		ns, metric := splitTableName("metrics.AWS/ApplicationELB.ActiveConnectionCount")
		assert.Equal(t, "AWS/ApplicationELB", ns)
		assert.Equal(t, "ActiveConnectionCount", metric)
	})

	t.Run("returns empty strings for non-metrics-prefixed table names", func(t *testing.T) {
		ns, metric := splitTableName("AWS/EC2.CPUUtilization")
		assert.Equal(t, "", ns)
		assert.Equal(t, "", metric)
	})

	t.Run("returns empty metricName when no second dot after prefix", func(t *testing.T) {
		ns, metric := splitTableName("metrics.AWS/EC2")
		assert.Equal(t, "AWS/EC2", ns)
		assert.Equal(t, "", metric)
	})
}

// ---- Tables -----------------------------------------------------------------

func TestSchemaProvider_Tables(t *testing.T) {
	p := newSchemaProviderForTest()
	resp, err := p.Tables(context.Background(), &schemas.TablesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Run("contains namespace.metricName entries for hardcoded namespaces", func(t *testing.T) {
		// AWS/EC2 is always in the hardcoded map; verify at least one of its
		// metrics appears with the expected dot-notation name.
		ec2Metrics, ok := cloudWatchConsts.NamespaceMetricsMap["AWS/EC2"]
		require.True(t, ok, "AWS/EC2 should be in hardcoded map")
		require.NotEmpty(t, ec2Metrics)

		tableSet := make(map[string]struct{}, len(resp.Tables))
		for _, name := range resp.Tables {
			tableSet[name] = struct{}{}
		}
		expected := "metrics.AWS/EC2." + ec2Metrics[0]
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

	t.Run("includes entries for custom namespaces from settings", func(t *testing.T) {
		customP := newSchemaProviderForTest(func(ds *DataSource) {
			ds.Settings.Namespace = "Custom/Namespace"
		})
		customResp, err := customP.Tables(context.Background(), &schemas.TablesRequest{})
		require.NoError(t, err)

		found := false
		for _, name := range customResp.Tables {
			if name == "metrics.Custom/Namespace." {
				found = true
				break
			}
		}
		assert.True(t, found, "expected placeholder table for custom namespace")
	})
}

// ---- Columns ----------------------------------------------------------------

func TestSchemaProvider_Columns(t *testing.T) {
	p := newSchemaProviderForTest()

	t.Run("returns dimension key columns for a known namespace", func(t *testing.T) {
		dimKeys, err := services.GetHardCodedDimensionKeysByNamespace("AWS/EC2")
		require.NoError(t, err)
		require.NotEmpty(t, dimKeys)

		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics.AWS/EC2.CPUUtilization"},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)

		cols, ok := resp.Columns["metrics.AWS/EC2.CPUUtilization"]
		assert.True(t, ok, "expected columns for metrics.AWS/EC2.CPUUtilization")
		assert.Len(t, cols, len(dimKeys))

		colNames := make(map[string]struct{}, len(cols))
		for _, c := range cols {
			colNames[c.Name] = struct{}{}
			assert.Equal(t, schemas.ColumnTypeString, c.Type)
			assert.Contains(t, c.Operators, schemas.OperatorEquals)
			assert.Contains(t, c.Operators, schemas.OperatorIn)
		}
		_, hasInstanceId := colNames["InstanceId"]
		assert.True(t, hasInstanceId, "expected InstanceId dimension key for AWS/EC2")
	})

	t.Run("correctly parses namespace from metrics.namespace.metricName table string", func(t *testing.T) {
		resp, err := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics.AWS/EC2.NetworkIn"},
		})
		require.NoError(t, err)

		// Columns for metrics.AWS/EC2.NetworkIn should be the same as
		// metrics.AWS/EC2.CPUUtilization — dimension keys are per-namespace.
		colsNetworkIn, ok := resp.Columns["metrics.AWS/EC2.NetworkIn"]
		assert.True(t, ok)

		resp2, _ := p.Columns(context.Background(), &schemas.ColumnsRequest{
			Tables: []string{"metrics.AWS/EC2.CPUUtilization"},
		})
		colsCPU := resp2.Columns["metrics.AWS/EC2.CPUUtilization"]
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
			Tables: []string{"metrics.Unknown/NS.SomeMetric"},
		})
		require.NoError(t, err)
		cols := resp.Columns["metrics.Unknown/NS.SomeMetric"]
		assert.Empty(t, cols)
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
			Tables: []string{"metrics.AWS/EC2.CPUUtilization", "metrics.Unknown/NS.SomeMetric"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Columns, "metrics.AWS/EC2.CPUUtilization", "known namespace should still be populated")
		assert.NotContains(t, resp.Columns, "metrics.Unknown/NS.SomeMetric", "failed table should be absent from columns")
		assert.Contains(t, resp.Errors, "metrics.Unknown/NS.SomeMetric", "error should be set for the failed table")
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
			Tables: []string{"metrics.AWS/EC2.CPUUtilization", "metrics.AWS/RDS.CPUUtilization"},
		})
		require.NoError(t, err)
		assert.Contains(t, resp.Columns, "metrics.AWS/EC2.CPUUtilization")
		assert.Contains(t, resp.Columns, "metrics.AWS/RDS.CPUUtilization")
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
}

// ---- ColumnValues -----------------------------------------------------------

func TestSchemaProvider_ColumnValues(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	t.Run("returns dimension values for requested columns", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesByDimensionFilter", mock.MatchedBy(func(r resources.DimensionValuesRequest) bool {
			return r.DimensionKey == "InstanceId"
		})).Return([]resources.ResourceResponse[string]{
			{Value: "i-11111111"},
			{Value: "i-22222222"},
		}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics.AWS/EC2.CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"i-11111111", "i-22222222"}, resp.ColumnValues["InstanceId"])
		assert.Empty(t, resp.Errors)
	})

	t.Run("passes namespace and metricName from table to the service", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesByDimensionFilter", mock.MatchedBy(func(r resources.DimensionValuesRequest) bool {
			return r.Namespace == "AWS/EC2" && r.MetricName == "CPUUtilization" && r.DimensionKey == "InstanceId"
		})).Return([]resources.ResourceResponse[string]{{Value: "i-abc"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics.AWS/EC2.CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		mockSvc.AssertExpectations(t)
		assert.Equal(t, []string{"i-abc"}, resp.ColumnValues["InstanceId"])
	})

	t.Run("passes accountId to the service when present", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesByDimensionFilter", mock.MatchedBy(func(r resources.DimensionValuesRequest) bool {
			return r.AccountId != nil && *r.AccountId == "111122223333"
		})).Return([]resources.ResourceResponse[string]{{Value: "i-xyz"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics.AWS/EC2.CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{"region": "us-east-1", "accountId": "111122223333"},
		})
		require.NoError(t, err)
		mockSvc.AssertExpectations(t)
		assert.Equal(t, []string{"i-xyz"}, resp.ColumnValues["InstanceId"])
	})

	t.Run("returns empty map without error when region is missing", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics.AWS/EC2.CPUUtilization",
			Columns:         []string{"InstanceId"},
			TableParameters: map[string]string{},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.ColumnValues)
		assert.Empty(t, resp.Errors)
	})

	t.Run("returns empty map without error for non-metrics table prefix", func(t *testing.T) {
		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "logs./aws/lambda/fn",
			Columns:         []string{"someField"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.Empty(t, resp.ColumnValues)
	})

	t.Run("makes one ListMetrics call per requested column", func(t *testing.T) {
		mockSvc := &mocks.ListMetricsServiceMock{}
		mockSvc.On("GetDimensionValuesByDimensionFilter", mock.MatchedBy(func(r resources.DimensionValuesRequest) bool {
			return r.DimensionKey == "InstanceId"
		})).Return([]resources.ResourceResponse[string]{{Value: "i-111"}}, nil)
		mockSvc.On("GetDimensionValuesByDimensionFilter", mock.MatchedBy(func(r resources.DimensionValuesRequest) bool {
			return r.DimensionKey == "AutoScalingGroupName"
		})).Return([]resources.ResourceResponse[string]{{Value: "my-asg"}}, nil)
		services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
			return mockSvc
		}

		p := newSchemaProviderForTest()
		resp, err := p.ColumnValues(context.Background(), &schemas.ColumnValuesRequest{
			Table:           "metrics.AWS/EC2.CPUUtilization",
			Columns:         []string{"InstanceId", "AutoScalingGroupName"},
			TableParameters: map[string]string{"region": "us-east-1"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"i-111"}, resp.ColumnValues["InstanceId"])
		assert.Equal(t, []string{"my-asg"}, resp.ColumnValues["AutoScalingGroupName"])
		mockSvc.AssertNumberOfCalls(t, "GetDimensionValuesByDimensionFilter", 2)
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
