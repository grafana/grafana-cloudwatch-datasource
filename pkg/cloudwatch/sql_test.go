package cloudwatch

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/experimental/featuretoggles"
	schemas "github.com/grafana/schemads"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, string(qJSON), string(out.Queries[0].JSON))
	})

	t.Run("grafanaSQL query with empty table passes through unchanged", func(t *testing.T) {
		qJSON := []byte(`{"refId":"A","grafanaSql":true,"table":""}`)
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{{RefID: "A", JSON: qJSON}},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, string(qJSON), string(out.Queries[0].JSON))
	})

	t.Run("nil request returns nil", func(t *testing.T) {
		assert.Nil(t, normalizeGrafanaSQLRequest(nil))
	})

	t.Run("empty query list returns empty", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{},
		}
		out := normalizeGrafanaSQLRequest(req)
		assert.Empty(t, out.Queries)
	})
}

func TestNormalizeGrafanaSQLRequest_FeatureGating(t *testing.T) {
	t.Run("drops grafanaSQL query when GrafanaConfig is nil", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", nil)},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		assert.Empty(t, out.Queries)
	})

	t.Run("drops only grafanaSQL queries from a mixed request", func(t *testing.T) {
		nativeJSON := []byte(`{"refId":"B","type":"timeSeriesQuery","namespace":"AWS/EC2"}`)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				GrafanaConfig: backend.NewGrafanaCfg(map[string]string{}),
			},
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", nil)},
				{RefID: "B", JSON: nativeJSON},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, "B", out.Queries[0].RefID)
	})
}

// ---- normalizeGrafanaSQLRequest — successful normalisation ----

func TestNormalizeGrafanaSQLRequest_Normalization(t *testing.T) {
	t.Run("normalises a basic metrics table query", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, timeSeriesQuery, m["type"])
		assert.Equal(t, "AWS/EC2", m["namespace"])
		assert.Equal(t, "CPUUtilization", m["metricName"])
		assert.Equal(t, "us-east-1", m["region"])
		assert.Equal(t, "Average", m["statistic"], "should default to Average")
		assert.Equal(t, false, m["matchExact"], "no dimension filters → matchExact should be false")
	})

	t.Run("maps all table fields: namespace (with slash), metricName, region, accountId, refId, and maxDataPoints", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "myRef", MaxDataPoints: 500, JSON: grafanaSQLQueryJSON("metrics|AWS/ApplicationELB|ActiveConnectionCount", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{
						"region":    "eu-west-1",
						"accountId": "111122223333",
					},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "AWS/ApplicationELB", m["namespace"])
		assert.Equal(t, "ActiveConnectionCount", m["metricName"])
		assert.Equal(t, "eu-west-1", m["region"])
		assert.Equal(t, "111122223333", m["accountId"])
		assert.Equal(t, "myRef", out.Queries[0].RefID)
		assert.Equal(t, int64(500), out.Queries[0].MaxDataPoints)
	})

	t.Run("custom namespace placeholder table (empty metricName)", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|Custom.App|", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-west-2"},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "Custom.App", m["namespace"])
		assert.NotContains(t, m, "metricName")
	})

	t.Run("passes through unchanged when table has no metrics prefix", func(t *testing.T) {
		qJSON := grafanaSQLQueryJSON("logs|/aws/lambda/fn", nil)
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries:       []backend.DataQuery{{RefID: "A", JSON: qJSON}},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)
		assert.Equal(t, string(qJSON), string(out.Queries[0].JSON))
	})

	t.Run("omits accountId field when not present in tableParameterValues", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		_, hasAccountId := m["accountId"]
		assert.False(t, hasAccountId, "accountId should not be present when not specified")
	})
}

// ---- normalizeGrafanaSQLRequest — statistic filter ----

func TestNormalizeGrafanaSQLRequest_StatisticFilter(t *testing.T) {
	t.Run("statistic filter sets the statistic field, not a dimension", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
					"filters": []map[string]interface{}{
						{
							"name": "statistic",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "Sum"},
							},
						},
					},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		require.Len(t, out.Queries, 1)

		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "Sum", m["statistic"])

		dims := dimensionsFromMap(t, m)
		_, hasStatistic := dims["statistic"]
		assert.False(t, hasStatistic, "statistic should not appear in dimensions")
	})

	t.Run("no statistic filter defaults to Average", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, "Average", m["statistic"])
	})

	t.Run("extended statistic (p99) is accepted as-is", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
					"filters": []map[string]interface{}{
						{
							"name": "statistic",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "p99"},
							},
						},
					},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
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
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
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
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Equal(t, []string{"i-12345"}, dims["InstanceId"])
	})

	t.Run("OperatorIn maps to multi-value dimension", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
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
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.ElementsMatch(t, []string{"i-11111", "i-22222"}, dims["InstanceId"])
	})

	t.Run("multiple dimension filters and a statistic filter are all applied", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
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
						{
							"name": "statistic",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "Sum"},
							},
						},
					},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Equal(t, []string{"i-abc"}, dims["InstanceId"])
		assert.Equal(t, []string{"my-asg"}, dims["AutoScalingGroupName"])
		assert.Equal(t, "Sum", m["statistic"])
		_, hasStatisticDim := dims["statistic"]
		assert.False(t, hasStatisticDim, "statistic should not appear as a dimension")
	})

	t.Run("unsupported operator (like) is ignored", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
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
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Empty(t, dims, "unsupported operator should not add any dimension")
	})

	t.Run("empty filter value is ignored", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
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
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Empty(t, dims)
	})

	t.Run("null/missing filters produce empty dimensions", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
					"filters":              nil,
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		dims := dimensionsFromMap(t, m)
		assert.Empty(t, dims)
	})
}

// ---- normalizeGrafanaSQLRequest — matchExact ----

func TestNormalizeGrafanaSQLRequest_MatchExact(t *testing.T) {
	t.Run("matchExact is false when no dimension filters are present", func(t *testing.T) {
		// Without dimension filters the query must use an inferred SEARCH
		// expression so CloudWatch returns one series per dimension combination
		// rather than the dimensionless aggregate rollup.
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, false, m["matchExact"], "no dimension filters → matchExact should be false")
	})

	t.Run("matchExact is true when dimension filters are present", func(t *testing.T) {
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
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
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, true, m["matchExact"], "dimension filters present → matchExact should be true")
	})

	t.Run("matchExact is false when the only filter is statistic (no dimension filters)", func(t *testing.T) {
		// The statistic column is not a CloudWatch dimension; a query that
		// only filters on statistic still has no dimension filters and must
		// use matchExact: false.
		req := &backend.QueryDataRequest{
			PluginContext: pluginCtxWithFeatureToggle(),
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: grafanaSQLQueryJSON("metrics|AWS/EC2|CPUUtilization", map[string]interface{}{
					"tableParameterValues": map[string]interface{}{"region": "us-east-1"},
					"filters": []map[string]interface{}{
						{
							"name": "statistic",
							"conditions": []map[string]interface{}{
								{"operator": "=", "value": "Sum"},
							},
						},
					},
				})},
			},
		}
		out := normalizeGrafanaSQLRequest(req)
		m := unmarshalNormalized(t, out.Queries[0])
		assert.Equal(t, false, m["matchExact"], "statistic-only filter → matchExact should be false")
	})
}

// ---- applyFilters unit tests ----

func TestApplyFilters(t *testing.T) {
	t.Run("empty filter name is skipped", func(t *testing.T) {
		dims, stat := applyFilters([]schemas.ColumnFilter{
			{Name: "", Conditions: []schemas.FilterCondition{
				{Operator: schemas.OperatorEquals, Value: "foo"},
			}},
		})
		assert.Empty(t, dims)
		assert.Empty(t, stat)
	})

	t.Run("filter with no conditions is skipped", func(t *testing.T) {
		dims, stat := applyFilters([]schemas.ColumnFilter{
			{Name: "InstanceId", Conditions: nil},
		})
		assert.Empty(t, dims)
		assert.Empty(t, stat)
	})

	t.Run("Values slice takes precedence over Value", func(t *testing.T) {
		dims, _ := applyFilters([]schemas.ColumnFilter{
			{Name: "InstanceId", Conditions: []schemas.FilterCondition{
				{Operator: schemas.OperatorIn, Values: []any{"i-111", "i-222"}, Value: "i-ignored"},
			}},
		})
		assert.ElementsMatch(t, []string{"i-111", "i-222"}, dims["InstanceId"])
		assert.NotContains(t, dims["InstanceId"], "i-ignored")
	})

}
