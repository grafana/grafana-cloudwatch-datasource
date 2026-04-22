package cloudwatch

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/experimental/featuretoggles"
	schemas "github.com/grafana/schemads"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/utils"
)

// pluginCtxWithFeatureToggle returns a PluginContext with the dsAbstractionApp
// feature toggle enabled, which is required for grafanaSQL queries to be processed.
func pluginCtxWithFeatureToggle() backend.PluginContext {
	return backend.PluginContext{
		GrafanaConfig: backend.NewGrafanaCfg(map[string]string{
			featuretoggles.EnabledFeatures: "dsAbstractionApp",
		}),
	}
}

// grafanaSQLQuery builds a minimal grafanaSQL query JSON with the given table
// and optional extra fields encoded in extraFields (merged at the JSON level).
func grafanaSQLQueryJSON(table string, extra map[string]interface{}) []byte {
	m := map[string]interface{}{
		"refId":      "A",
		"grafanaSql": true,
		"table":      table,
	}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

// unmarshalNormalized decodes a normalised query JSON into a plain map for
// assertion-friendly access.
func unmarshalNormalized(t *testing.T, q backend.DataQuery) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(q.JSON, &m))
	return m
}

// normalizeGrafanaSQLRequestUnderTest runs grafanaSQL normalization with a test datasource
// so schema-based wildcard dimension injection can resolve namespace keys.
func normalizeGrafanaSQLRequestUnderTest(req *backend.QueryDataRequest) (*backend.QueryDataRequest, map[string]struct{}) {
	ds := newTestDatasource()
	return ds.normalizeGrafanaSQLRequest(context.Background(), req)
}

// dimensionsFromMap extracts the dimensions field as map[string][]string from
// the normalised query map returned by unmarshalNormalized.
func dimensionsFromMap(t *testing.T, m map[string]interface{}) map[string][]string {
	t.Helper()
	raw, ok := m["dimensions"]
	if !ok {
		return map[string][]string{}
	}
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("dimensions is not a map: %T", raw)
	}
	result := make(map[string][]string, len(rawMap))
	for k, v := range rawMap {
		switch vt := v.(type) {
		case []interface{}:
			strs := make([]string, len(vt))
			for i, elem := range vt {
				strs[i], _ = elem.(string)
			}
			result[k] = strs
		case string:
			result[k] = []string{vt}
		}
	}
	return result
}

// ---- normalizeGrafanaSQLRequest — pass-through and gating ----

func TestNormalizeGrafanaSQLRequest_NonGrafanaSQL(t *testing.T) {
	t.Run("non-grafanaSQL query passes through unchanged", func(t *testing.T) {
		qJSON := []byte(`{"refId":"A","type":"timeSeriesQuery","namespace":"AWS/EC2","metricName":"CPUUtilization","statistic":"Average"}`)
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{{RefID: "A", JSON: qJSON}},
		}
		out, refIDs := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, string(qJSON), string(out.Queries[0].JSON))
		assert.NotContains(t, refIDs, "A")
	})

	t.Run("grafanaSQL query with empty table passes through unchanged", func(t *testing.T) {
		qJSON := []byte(`{"refId":"A","grafanaSql":true,"table":""}`)
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{{RefID: "A", JSON: qJSON}},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, string(qJSON), string(out.Queries[0].JSON))
	})

	t.Run("nil request returns nil", func(t *testing.T) {
		out, _ := normalizeGrafanaSQLRequestUnderTest(nil)
		assert.Nil(t, out)
	})

	t.Run("empty query list returns empty", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		assert.Empty(t, out.Queries)
	})
}

func TestNormalizeGrafanaSQLRequest_FeatureGating(t *testing.T) {
	t.Run("drops grafanaSQL query when GrafanaConfig is nil", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", nil)},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		assert.Empty(t, out.Queries)
	})

	t.Run("drops only grafanaSQL queries from a mixed request", func(t *testing.T) {
		nativeJSON := []byte(`{"refId":"B","type":"timeSeriesQuery","namespace":"AWS/EC2"}`)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				GrafanaConfig: backend.NewGrafanaCfg(map[string]string{}),
			},
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", nil)},
				{RefID: "B", JSON: nativeJSON},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, "B", out.Queries[0].RefID)
	})
}

func TestQueryData_NoQueriesAfterGrafanaSQLNormalization(t *testing.T) {
	ds := newTestDatasource()
	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	t.Run("returns error when all queries were grafanaSQL and GrafanaConfig is nil", func(t *testing.T) {
		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			Queries: []backend.DataQuery{{
				RefID:     "A",
				TimeRange: tr,
				JSON:      grafanaSQLQueryJSON("metrics|AWS/EC2", nil),
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no queries to execute")
	})

	t.Run("returns error when all queries were grafanaSQL and dsAbstractionApp is off", func(t *testing.T) {
		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				GrafanaConfig: backend.NewGrafanaCfg(map[string]string{}),
			},
			Queries: []backend.DataQuery{{
				RefID:     "A",
				TimeRange: tr,
				JSON:      grafanaSQLQueryJSON("metrics|AWS/EC2", nil),
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no queries to execute")
	})

	t.Run("returns error when request had no queries", func(t *testing.T) {
		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			Queries: []backend.DataQuery{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no queries to execute")
	})
}

// ---- normalizeGrafanaSQLRequest — successful normalisation ----

func TestNormalizeGrafanaSQLRequest_Normalization(t *testing.T) {
	t.Run("normalises a basic metrics table query", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
				})},
			},
		}
		out, refIDs := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, timeSeriesQuery, m["type"])
		assert.Equal(t, "AWS/EC2", m["namespace"])
		assert.Equal(t, "CPUUtilization", m["metricName"])
		assert.Equal(t, "us-east-1", m[RegionTableParameter])
		assert.Equal(t, "Average", m["statistic"], "should default to Average")
		assert.Equal(t, false, m["matchExact"], "schema-only wildcards use non-schema SEARCH so partial-dimension series are included")
		dims := dimensionsFromMap(t, m)
		for _, k := range []string{"AutoScalingGroupName", "ImageId", "InstanceId", "InstanceType"} {
			assert.Equal(t, []string{"*"}, dims[k], "dimension %q", k)
		}
		assert.Len(t, dims, 4)
		assert.Contains(t, refIDs, "A")
	})

	t.Run("maps all table fields: namespace (with slash), metricName, region, accountId, refId, and maxDataPoints", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "myRef", MaxDataPoints: 500, JSON: grafanaSQLQueryJSON("metrics|AWS/ApplicationELB", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{
						RegionTableParameter:     "eu-west-1",
						AccountIdTableParameter:  "111122223333",
						MetricNameTableParameter: "ActiveConnectionCount",
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "AWS/ApplicationELB", m["namespace"])
		assert.Equal(t, "ActiveConnectionCount", m["metricName"])
		assert.Equal(t, "eu-west-1", m[RegionTableParameter])
		assert.Equal(t, "111122223333", m[AccountIdTableParameter])
		assert.Equal(t, "myRef", out.Queries[0].RefID)
		assert.Equal(t, int64(500), out.Queries[0].MaxDataPoints)
	})

	t.Run("custom namespace without metricName is not rewritten", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|Custom.App", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-west-2"},
				})},
			},
		}
		out, refIDs := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)
		assert.NotContains(t, refIDs, "A")
		var q schemas.Query
		require.NoError(t, json.Unmarshal(out.Queries[0].JSON, &q))
		assert.True(t, q.GrafanaSql)
		assert.Equal(t, "metrics|Custom.App", q.Table)
	})

	t.Run("custom namespace with metricName is normalised", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|Custom.App", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{
						RegionTableParameter:     "us-west-2",
						MetricNameTableParameter: "Invocations",
					},
				})},
			},
		}
		out, refIDs := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)
		assert.Contains(t, refIDs, "A")
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "Custom.App", m["namespace"])
		assert.Equal(t, "Invocations", m["metricName"])
	})

	t.Run("passes through unchanged when table has no metrics prefix", func(t *testing.T) {
		qJSON := grafanaSQLQueryJSON("logs|/aws/lambda/fn", nil)
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{{RefID: "A", JSON: qJSON}},
		}
		out, refIDs := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, string(qJSON), string(out.Queries[0].JSON))
		assert.NotContains(t, refIDs, "A")
	})

	t.Run("mixed request: only the normalised grafanaSQL refID is in the returned set", func(t *testing.T) {
		nativeJSON := []byte(`{"refId":"B","type":"timeSeriesQuery","namespace":"AWS/EC2","metricName":"CPUUtilization","statistic":"Average"}`)
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
				})},
				{RefID: "B", JSON: nativeJSON},
			},
		}
		out, refIDs := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 2)
		assert.Contains(t, refIDs, "A")
		assert.NotContains(t, refIDs, "B")
	})

	t.Run("omits accountId field when not present in tableParameterValues", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		_, hasAccountId := m[AccountIdTableParameter]
		assert.False(t, hasAccountId, "accountId should not be present when not specified")
	})

}

// ---- normalizeGrafanaSQLRequest — statistic table hint ----

func TestNormalizeGrafanaSQLRequest_StatisticTableHint(t *testing.T) {
	t.Run("tableHintValues STATISTIC sets the statistic field", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"tableHintValues":      map[string]interface{}{StatisticTableHintValueKey: "Sum"},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "Sum", m["statistic"])

		dims := dimensionsFromMap(t, m)
		_, hasStatistic := dims["statistic"]
		assert.False(t, hasStatistic, "statistic should not appear in dimensions")
	})

	t.Run("no statistic hint defaults to Average", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "Average", m["statistic"])
	})

	t.Run("extended statistic (p99) is accepted via table hint", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"tableHintValues":      map[string]interface{}{StatisticTableHintValueKey: "p99"},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "p99", m["statistic"])
	})
}

// ---- normalizeGrafanaSQLRequest — dimension filters ----

func TestNormalizeGrafanaSQLRequest_DimensionFilters(t *testing.T) {
	t.Run("OperatorEquals maps to single-value dimension", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"filters": []map[string]interface{}{
						{
							"name": "InstanceId",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "i-12345"},
							},
						},
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Equal(t, []string{"i-12345"}, dims["InstanceId"])
	})

	t.Run("OperatorIn maps to multi-value dimension", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"filters": []map[string]interface{}{
						{
							"name": "InstanceId",
							"conditions": []map[string]interface{}{
								{"operator": "in", "values": []string{"i-11111", "i-22222"}},
							},
						},
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.ElementsMatch(t, []string{"i-11111", "i-22222"}, dims["InstanceId"])
	})

	t.Run("multiple dimension filters and statistic table hint are all applied", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"tableHintValues":      map[string]interface{}{StatisticTableHintValueKey: "Sum"},
					"filters": []map[string]interface{}{
						{
							"name": "InstanceId",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "i-abc"},
							},
						},
						{
							"name": "AutoScalingGroupName",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "my-asg"},
							},
						},
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Equal(t, []string{"i-abc"}, dims["InstanceId"])
		assert.Equal(t, []string{"my-asg"}, dims["AutoScalingGroupName"])
		assert.Equal(t, "Sum", m["statistic"])
		_, hasStatisticDim := dims["statistic"]
		assert.False(t, hasStatisticDim, "statistic should not appear as a dimension")
	})

	t.Run("unsupported operator (like) is ignored then schema wildcards apply", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"filters": []map[string]interface{}{
						{
							"name": "InstanceId",
							"conditions": []map[string]interface{}{
								{"operator": "like", "value": "%prod%"},
							},
						},
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		for _, k := range []string{"AutoScalingGroupName", "ImageId", "InstanceId", "InstanceType"} {
			assert.Equal(t, []string{"*"}, dims[k], "dimension %q", k)
		}
		assert.Equal(t, false, m["matchExact"])
	})

	t.Run("empty filter value is ignored then schema wildcards apply", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"filters": []map[string]interface{}{
						{
							"name": "InstanceId",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": ""},
							},
						},
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Len(t, dims, 4)
		assert.Equal(t, false, m["matchExact"])
	})

	t.Run("null/missing filters inject schema wildcard dimensions", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"filters":              nil,
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		for _, k := range []string{"AutoScalingGroupName", "ImageId", "InstanceId", "InstanceType"} {
			assert.Equal(t, []string{"*"}, dims[k], "dimension %q", k)
		}
		assert.Len(t, dims, 4)
		assert.Equal(t, false, m["matchExact"])
	})
}

// ---- normalizeGrafanaSQLRequest — matchExact ----

func TestNormalizeGrafanaSQLRequest_MatchExact(t *testing.T) {
	t.Run("matchExact is false when no user filters (schema wildcard dimensions)", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, false, m["matchExact"], "schema-only wildcards: use non-schema SEARCH for partial-dimension series")
	})

	t.Run("matchExact is true when dimension filters are present", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"filters": []map[string]interface{}{
						{
							"name": "InstanceId",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "i-12345"},
							},
						},
					},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, true, m["matchExact"], "dimension filters present → matchExact should be true")
	})

	t.Run("matchExact is false when only statistic hint is set (schema wildcards, no user dimension filters)", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{RegionTableParameter: "us-east-1", MetricNameTableParameter: "CPUUtilization"},
					"tableHintValues":      map[string]interface{}{StatisticTableHintValueKey: "Sum"},
				})},
			},
		}
		out, _ := normalizeGrafanaSQLRequestUnderTest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, false, m["matchExact"], "statistic hint is not a dimension filter")
	})
}

// ---- applyFilters unit tests ----

func TestApplyFilters(t *testing.T) {
	t.Run("empty filter name is skipped", func(t *testing.T) {
		dims := applyFilters([]schemas.ColumnFilter{
			{Name: "", Conditions: []schemas.FilterCondition{
				{Operator: schemas.OperatorEquals, Value: "foo"},
			}},
		})
		assert.Empty(t, dims)
	})

	t.Run("filter with no conditions is skipped", func(t *testing.T) {
		dims := applyFilters([]schemas.ColumnFilter{
			{Name: "InstanceId", Conditions: nil},
		})
		assert.Empty(t, dims)
	})

	t.Run("Values slice takes precedence over Value", func(t *testing.T) {
		dims := applyFilters([]schemas.ColumnFilter{
			{Name: "InstanceId", Conditions: []schemas.FilterCondition{
				{Operator: schemas.OperatorIn, Values: []any{"i-111", "i-222"}, Value: "i-ignored"},
			}},
		})
		assert.ElementsMatch(t, []string{"i-111", "i-222"}, dims["InstanceId"])
		assert.NotContains(t, dims["InstanceId"], "i-ignored")
	})

}

// ---- convertToTabular unit tests ----

// makeTimeSeriesFrame builds a minimal FrameTypeTimeSeriesMulti frame as
// produced by buildDataFrames: a Time field and a nullable float64 Value field
// whose Labels carry the dimension key-value pairs.
func makeTimeSeriesFrame(refID string, labels data.Labels, timestamps []time.Time, values []*float64) *data.Frame {
	tf := data.NewField(data.TimeSeriesTimeFieldName, nil, timestamps)
	vf := data.NewField(data.TimeSeriesValueFieldName, labels, values)
	frame := data.NewFrame(refID, tf, vf)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
	return frame
}

func TestConvertToTabular(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 1, 1, 0, 1, 0, 0, time.UTC)

	t.Run("single series becomes flat frame with time, value and dimension fields", func(t *testing.T) {
		resp := &backend.QueryDataResponse{
			Responses: backend.Responses{
				"A": {
					Frames: data.Frames{
						makeTimeSeriesFrame("A", data.Labels{"InstanceId": "i-111"},
							[]time.Time{t0, t1}, []*float64{utils.Pointer(10.5), utils.Pointer(11.2)}),
					},
				},
			},
		}
		convertToTabular(resp, map[string]struct{}{"A": {}})

		require.Len(t, resp.Responses["A"].Frames, 1)
		expected := data.NewFrame("A",
			data.NewField("time", nil, []time.Time{t0, t1}),
			data.NewField("value", nil, []*float64{utils.Pointer(10.5), utils.Pointer(11.2)}),
			data.NewField("InstanceId", nil, []string{"i-111", "i-111"}),
		)
		expected.RefID = "A"
		expected.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
		if diff := cmp.Diff(expected, resp.Responses["A"].Frames[0], data.FrameTestCompareOptions()...); diff != "" {
			t.Errorf("frame mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("multiple series are concatenated into one flat frame", func(t *testing.T) {
		resp := &backend.QueryDataResponse{
			Responses: backend.Responses{
				"A": {
					Frames: data.Frames{
						makeTimeSeriesFrame("A", data.Labels{"InstanceId": "i-111"},
							[]time.Time{t0, t1}, []*float64{utils.Pointer(float64(10)), utils.Pointer(float64(11))}),
						makeTimeSeriesFrame("A", data.Labels{"InstanceId": "i-222"},
							[]time.Time{t0, t1}, []*float64{utils.Pointer(float64(8)), utils.Pointer(float64(9))}),
					},
				},
			},
		}
		convertToTabular(resp, map[string]struct{}{"A": {}})

		require.Len(t, resp.Responses["A"].Frames, 1)
		// After time-sorting: both t0 rows come before both t1 rows.
		expected := data.NewFrame("A",
			data.NewField("time", nil, []time.Time{t0, t0, t1, t1}),
			data.NewField("value", nil, []*float64{
				utils.Pointer(float64(10)), utils.Pointer(float64(8)),
				utils.Pointer(float64(11)), utils.Pointer(float64(9)),
			}),
			data.NewField("InstanceId", nil, []string{"i-111", "i-222", "i-111", "i-222"}),
		)
		expected.RefID = "A"
		expected.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
		if diff := cmp.Diff(expected, resp.Responses["A"].Frames[0], data.FrameTestCompareOptions()...); diff != "" {
			t.Errorf("frame mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("multi-dimensional labels all become fields", func(t *testing.T) {
		resp := &backend.QueryDataResponse{
			Responses: backend.Responses{
				"A": {
					Frames: data.Frames{
						makeTimeSeriesFrame("A",
							data.Labels{"InstanceId": "i-111", "AutoScalingGroupName": "asg-1"},
							[]time.Time{t0}, []*float64{utils.Pointer(float64(5))}),
					},
				},
			},
		}
		convertToTabular(resp, map[string]struct{}{"A": {}})

		require.Len(t, resp.Responses["A"].Frames, 1)
		// Dimension columns are sorted alphabetically: AutoScalingGroupName before InstanceId.
		expected := data.NewFrame("A",
			data.NewField("time", nil, []time.Time{t0}),
			data.NewField("value", nil, []*float64{utils.Pointer(float64(5))}),
			data.NewField("AutoScalingGroupName", nil, []string{"asg-1"}),
			data.NewField("InstanceId", nil, []string{"i-111"}),
		)
		expected.RefID = "A"
		expected.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
		if diff := cmp.Diff(expected, resp.Responses["A"].Frames[0], data.FrameTestCompareOptions()...); diff != "" {
			t.Errorf("frame mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("refIDs not in the set are not converted", func(t *testing.T) {
		originalFrame := makeTimeSeriesFrame("B", data.Labels{"InstanceId": "i-999"},
			[]time.Time{t0}, []*float64{utils.Pointer(float64(1))})
		resp := &backend.QueryDataResponse{
			Responses: backend.Responses{
				"B": {Frames: data.Frames{originalFrame}},
			},
		}
		convertToTabular(resp, map[string]struct{}{"A": {}}) // "B" is not in the set

		// Frame for "B" must be unchanged.
		frames := resp.Responses["B"].Frames
		require.Len(t, frames, 1)
		assert.Equal(t, originalFrame, frames[0])
	})

	t.Run("nil value pointers are preserved", func(t *testing.T) {
		resp := &backend.QueryDataResponse{
			Responses: backend.Responses{
				"A": {
					Frames: data.Frames{
						makeTimeSeriesFrame("A", data.Labels{"InstanceId": "i-111"},
							[]time.Time{t0}, []*float64{nil}),
					},
				},
			},
		}
		convertToTabular(resp, map[string]struct{}{"A": {}})

		require.Len(t, resp.Responses["A"].Frames, 1)
		expected := data.NewFrame("A",
			data.NewField("time", nil, []time.Time{t0}),
			data.NewField("value", nil, []*float64{nil}),
			data.NewField("InstanceId", nil, []string{"i-111"}),
		)
		expected.RefID = "A"
		expected.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
		if diff := cmp.Diff(expected, resp.Responses["A"].Frames[0], data.FrameTestCompareOptions()...); diff != "" {
			t.Errorf("frame mismatch (-want +got):\n%s", diff)
		}
	})
}

