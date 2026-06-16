package cloudwatch

import (
	"context"
	"errors"
	"testing"

	schemas "github.com/grafana/schemads"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/mocks"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models/resources"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/services"
)

func TestSchemaMetadataCache_CustomMetricNames_CacheHit(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	mockSvc := &mocks.ListMetricsServiceMock{}
	mockSvc.On("GetMetricsByNamespace", mock.MatchedBy(func(r resources.MetricsRequest) bool {
		return r.Namespace == "My/Custom/NS"
	})).
		Return([]resources.ResourceResponse[resources.Metric]{
			{Value: resources.Metric{Name: "Zeta", Namespace: "My/Custom/NS"}},
			{Value: resources.Metric{Name: "Alpha", Namespace: "My/Custom/NS"}},
		}, nil).Once()
	services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
		return mockSvc
	}

	p := newSchemaProviderForTest(func(ds *DataSource) {
		ds.Settings.Namespace = "My/Custom/NS"
	})
	req := &schemas.TableParameterValuesRequest{
		Table:            "metrics|My/Custom/NS",
		TableParameter:   MetricNameTableParameter,
		DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
	}

	resp1, err := p.TableParameterValues(context.Background(), req)
	require.NoError(t, err)
	resp2, err := p.TableParameterValues(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, []string{"Alpha", "Zeta"}, resp1.TableParameterValues[MetricNameTableParameter])
	assert.Equal(t, resp1.TableParameterValues[MetricNameTableParameter], resp2.TableParameterValues[MetricNameTableParameter])
	mockSvc.AssertNumberOfCalls(t, "GetMetricsByNamespace", 1)
}

func TestSchemaMetadataCache_CustomMetricNames_ErrorNotCached(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	mockSvc := &mocks.ListMetricsServiceMock{}
	mockSvc.On("GetMetricsByNamespace", mock.MatchedBy(func(r resources.MetricsRequest) bool {
		return r.Namespace == "My/Custom/NS"
	})).
		Return([]resources.ResourceResponse[resources.Metric](nil), errors.New("API unavailable")).Twice()
	services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
		return mockSvc
	}

	p := newSchemaProviderForTest(func(ds *DataSource) {
		ds.Settings.Namespace = "My/Custom/NS"
	})
	req := &schemas.TableParameterValuesRequest{
		Table:            "metrics|My/Custom/NS",
		TableParameter:   MetricNameTableParameter,
		DependencyValues: map[string]string{RegionTableParameter: "us-east-1"},
	}

	resp1, err1 := p.TableParameterValues(context.Background(), req)
	require.NoError(t, err1)
	require.Contains(t, resp1.Errors[MetricNameTableParameter], "API unavailable")

	resp2, err2 := p.TableParameterValues(context.Background(), req)
	require.NoError(t, err2)
	require.Contains(t, resp2.Errors[MetricNameTableParameter], "API unavailable")

	mockSvc.AssertNumberOfCalls(t, "GetMetricsByNamespace", 2)
}

func TestSchemaMetadataCache_CustomDimensionKeys_CacheHit(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	mockSvc := &mocks.ListMetricsServiceMock{}
	mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
		Return([]resources.ResourceResponse[string]{
			{Value: "Zebra"},
			{Value: "Apple"},
		}, nil).Once()
	services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
		return mockSvc
	}

	p := newSchemaProviderForTest()
	colsReq := &schemas.ColumnsRequest{
		Tables:          []string{"metrics|Custom/SchemaNS"},
		TableParameters: map[string]string{RegionTableParameter: "us-east-1"},
	}

	resp1, err := p.Columns(context.Background(), colsReq)
	require.NoError(t, err)
	resp2, err := p.Columns(context.Background(), colsReq)
	require.NoError(t, err)

	c1 := resp1.Columns["metrics|Custom/SchemaNS"]
	c2 := resp2.Columns["metrics|Custom/SchemaNS"]
	require.NotEmpty(t, c1)
	assert.Equal(t, len(c1), len(c2))
	mockSvc.AssertNumberOfCalls(t, "GetDimensionKeysByDimensionFilter", 1)
}

func TestSchemaMetadataCache_CustomDimensionKeys_ErrorNotCached(t *testing.T) {
	origNewListMetricsService := services.NewListMetricsService
	t.Cleanup(func() { services.NewListMetricsService = origNewListMetricsService })

	mockSvc := &mocks.ListMetricsServiceMock{}
	mockSvc.On("GetDimensionKeysByDimensionFilter", mock.Anything).
		Return([]resources.ResourceResponse[string](nil), errors.New("throttled")).Twice()
	services.NewListMetricsService = func(_ models.MetricsClientProvider) models.ListMetricsProvider {
		return mockSvc
	}

	p := newSchemaProviderForTest()
	colsReq := &schemas.ColumnsRequest{
		Tables:          []string{"metrics|Custom/ErrNS"},
		TableParameters: map[string]string{RegionTableParameter: "us-east-1"},
	}

	resp1, err1 := p.Columns(context.Background(), colsReq)
	require.NoError(t, err1)
	require.Contains(t, resp1.Errors["metrics|Custom/ErrNS"], "throttled")

	resp2, err2 := p.Columns(context.Background(), colsReq)
	require.NoError(t, err2)
	require.Contains(t, resp2.Errors["metrics|Custom/ErrNS"], "throttled")

	mockSvc.AssertNumberOfCalls(t, "GetDimensionKeysByDimensionFilter", 2)
}
