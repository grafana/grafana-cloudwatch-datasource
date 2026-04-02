package cloudwatch

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	schemas "github.com/grafana/schemads"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/kinds/dataquery"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/utils"
)

// grafanaSQLQuery is a typed representation of the JSON payload produced by
// normalizeGrafanaSQLRequest. It mirrors the unexported models.metricsDataQuery
// struct so that ParseMetricDataQueries can unmarshal it correctly.
type grafanaSQLQuery struct {
	dataquery.CloudWatchMetricsQuery
	Type string `json:"type"`
}

// normalizeGrafanaSQLRequest rewrites queries that carry a grafanaSQL payload
// (set by dsAbstraction) into the native CloudWatch MetricStat query JSON that
// executeTimeSeriesQuery already understands. Non-grafanaSQL queries are passed
// through unchanged.
//
// MetricStat (MetricQueryTypeSearch + MetricEditorModeBuilder) is always used
// rather than Metric Insights because:
//   - The schemads column model (dimension keys + statistic) maps directly onto
//     MetricStat's namespace/metricName/dimensions/statistic fields.
//   - MetricStat supports the full statistics surface (IQM, percentiles, TM…),
//     whereas Metric Insights only supports AVG/MIN/MAX/SUM/COUNT.
//   - The dsAbstraction SQL engine (go-mysql-server) already handles GROUP BY,
//     ORDER BY, and LIMIT against the returned frames; the datasource only needs
//     to return the right raw data.
func normalizeGrafanaSQLRequest(req *backend.QueryDataRequest) *backend.QueryDataRequest {
	if req == nil || len(req.Queries) == 0 {
		return req
	}

	grafanaConfig := req.PluginContext.GrafanaConfig
	queries := make([]backend.DataQuery, 0, len(req.Queries))
	for _, q := range req.Queries {
		var query schemas.Query
		if err := json.Unmarshal(q.JSON, &query); err != nil {
			queries = append(queries, q)
			continue
		}
		if !query.GrafanaSql || query.Table == "" {
			queries = append(queries, q)
			continue
		}

		if grafanaConfig == nil {
			backend.Logger.Warn("grafanaConfig is not set, skipping grafanaSQL query", "refId", q.RefID)
			continue
		}
		if !grafanaConfig.FeatureToggles().IsEnabled("dsAbstractionApp") {
			backend.Logger.Warn("dsAbstractionApp feature toggle is not enabled, skipping grafanaSQL query", "refId", q.RefID)
			continue
		}

		namespace, metricName := splitTableName(query.Table)
		if namespace == "" {
			backend.Logger.Warn("grafanaSQL query has unrecognised table format, skipping", "refId", q.RefID, "table", query.Table)
			queries = append(queries, q)
			continue
		}

		dimensions, statistic := applyFilters(query.Filters)
		if statistic == "" {
			statistic = "Average"
		}

		region, _ := query.TableParameterValues["region"].(string)
		accountId, _ := query.TableParameterValues["accountId"].(string)

		dims := make(dataquery.Dimensions, len(dimensions))
		for k, vals := range dimensions {
			dims[k] = dataquery.StringOrArrayOfString{ArrayOfString: vals}
		}

		normalized := grafanaSQLQuery{
			Type: timeSeriesQuery,
			CloudWatchMetricsQuery: dataquery.CloudWatchMetricsQuery{
				Region:     region,
				Namespace:  namespace,
				Statistic:  &statistic,
				Dimensions: &dims,
				MatchExact: utils.Pointer(true),
			},
		}
		if metricName != "" {
			normalized.MetricName = &metricName
		}
		if accountId != "" {
			normalized.AccountId = &accountId
		}

		jsonBytes, err := json.Marshal(normalized)
		if err != nil {
			backend.Logger.Warn("failed to marshal normalised grafanaSQL query, skipping", "refId", q.RefID, "error", fmt.Sprintf("%v", err))
			queries = append(queries, q)
			continue
		}

		queries = append(queries, backend.DataQuery{
			RefID:         q.RefID,
			QueryType:     q.QueryType,
			MaxDataPoints: q.MaxDataPoints,
			Interval:      q.Interval,
			TimeRange:     q.TimeRange,
			JSON:          jsonBytes,
		})
	}

	return &backend.QueryDataRequest{
		PluginContext: req.PluginContext,
		Headers:       req.Headers,
		Queries:       queries,
	}
}

// applyFilters translates schemads ColumnFilter predicates into a CloudWatch
// dimensions map and an optional statistic value.
//
// The "statistic" column is special-cased: its value is routed to the returned
// statistic string rather than added to the dimensions map. All other columns
// are treated as CloudWatch dimension keys.
//
// Only OperatorEquals and OperatorIn are meaningful for CloudWatch dimensions;
// other operators are silently ignored.
func applyFilters(filters []schemas.ColumnFilter) (dimensions map[string][]string, statistic string) {
	dimensions = make(map[string][]string)

	for _, f := range filters {
		if f.Name == "" || len(f.Conditions) == 0 {
			continue
		}

		for _, condition := range f.Conditions {
			if condition.Operator != schemas.OperatorEquals && condition.Operator != schemas.OperatorIn {
				continue
			}

			values := extractConditionValues(condition)
			if len(values) == 0 {
				continue
			}

			if f.Name == statisticColumn.Name {
				statistic = values[0]
				continue
			}

			dimensions[f.Name] = append(dimensions[f.Name], values...)
		}
	}

	return dimensions, statistic
}

// extractConditionValues collects non-empty string values from a filter
// condition, preferring Values over the singular Value field.
func extractConditionValues(condition schemas.FilterCondition) []string {
	var out []string
	for _, v := range condition.Values {
		if s := anyToStr(v); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		if s := anyToStr(condition.Value); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func anyToStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
