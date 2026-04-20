package cloudwatch

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	schemas "github.com/grafana/schemads"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/kinds/dataquery"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/utils"
)

// normalizeGrafanaSQLRequest rewrites queries that carry a grafanaSQL payload
// (set by dsAbstraction) into the native CloudWatch MetricStat query JSON that
// executeTimeSeriesQuery already understands. Non-grafanaSQL queries are passed
// through unchanged.
//
// When grafanaSQL cannot be applied (GrafanaConfig missing, dsAbstractionApp
// disabled, or unrecoverable marshal failure where noted below), those queries
// are omitted rather than passed through: the raw grafanaSQL JSON is not valid
// input for CloudWatch metric execution, unlike a bad table name where the
// caller may still rely on native query fields.
//
// The second return value is the set of refIDs that were rewritten. Callers
// must post-process those responses with convertToTabular so the SQL engine
// receives flat table frames rather than time-series-multi frames.
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
func normalizeGrafanaSQLRequest(req *backend.QueryDataRequest) (*backend.QueryDataRequest, map[string]struct{}) {
	if req == nil || len(req.Queries) == 0 {
		return req, nil
	}

	grafanaConfig := req.PluginContext.GrafanaConfig
	queries := make([]backend.DataQuery, 0, len(req.Queries))
	grafanaSQLRefIDs := make(map[string]struct{})
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

		// matchExact is set to true only when dimension filters are present so
		// that CloudWatch returns an exact MetricStat result. When there are no
		// dimension filters the query should return all series for the metric,
		// which requires matchExact: false so that an inferred SEARCH expression
		// is used instead of a dimensionless MetricStat (which would return only
		// the aggregate rollup).
		normalized := models.MetricsDataQuery{
			Type: timeSeriesQuery,
			CloudWatchMetricsQuery: dataquery.CloudWatchMetricsQuery{
				Region:     region,
				Namespace:  namespace,
				Statistic:  &statistic,
				Dimensions: &dims,
				MatchExact: utils.Pointer(len(dimensions) > 0),
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

		grafanaSQLRefIDs[q.RefID] = struct{}{}
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
	}, grafanaSQLRefIDs
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

// convertToTabular converts the FrameTypeTimeSeriesMulti frames returned by
// executeTimeSeriesQuery into a single flat table frame for each grafanaSQL
// refID. The dsAbstraction SQL engine (vtable.go) maps rows by field name, so
// dimension values that are encoded as field labels must become real named
// fields alongside "time" and "value".
//
// For example, two series frames:
//
//	Frame 1: time=[t1,t2]  value(InstanceId=i-111)=[10,11]
//	Frame 2: time=[t1,t2]  value(InstanceId=i-222)=[8,9]
//
// become one flat frame:
//
//	time=[t1,t1,t2,t2]  value=[10,8,11,9]  InstanceId=[i-111,i-222,i-111,i-222]
func convertToTabular(resp *backend.QueryDataResponse, grafanaSQLRefIDs map[string]struct{}) {
	for refID := range grafanaSQLRefIDs {
		dr, ok := resp.Responses[refID]
		if !ok || len(dr.Frames) == 0 {
			continue
		}

		// Collect all label keys across all frames so every output row has the
		// same columns regardless of which series carries which labels.
		labelKeySet := make(map[string]struct{})
		for _, f := range dr.Frames {
			_, valueField := timeAndValueFields(f)
			if valueField == nil {
				continue
			}
			for k := range valueField.Labels {
				labelKeySet[k] = struct{}{}
			}
		}
		// Sort for deterministic column ordering.
		labelKeys := make([]string, 0, len(labelKeySet))
		for k := range labelKeySet {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)

		// Build output fields.
		timeField := data.NewField("time", nil, []time.Time{})
		valueField := data.NewField("value", nil, []*float64{})
		dimFields := make([]*data.Field, len(labelKeys))
		for i, k := range labelKeys {
			dimFields[i] = data.NewField(k, nil, []string{})
		}

		// Append rows from every series frame.
		for _, f := range dr.Frames {
			tf, vf := timeAndValueFields(f)
			if tf == nil || vf == nil {
				continue
			}

			for i := range tf.Len() {
				t, ok := fieldTimeAt(tf, i)
				if !ok {
					continue
				}
				v := fieldFloat64At(vf, i)
				timeField.Append(t)
				valueField.Append(v)
				for j, k := range labelKeys {
					dimFields[j].Append(vf.Labels[k])
				}
			}
		}

		// Sort all rows by ascending time. Build a row-index slice, sort it by
		// the time field, then reorder every field in lockstep.
		n := timeField.Len()
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			ta, _ := timeField.At(idx[a]).(time.Time)
			tb, _ := timeField.At(idx[b]).(time.Time)
			return ta.Before(tb)
		})
		allFields := append([]*data.Field{timeField, valueField}, dimFields...)
		for _, field := range allFields {
			sorted := data.NewFieldFromFieldType(field.Type(), n)
			sorted.Name = field.Name
			for newPos, oldPos := range idx {
				sorted.Set(newPos, field.At(oldPos))
			}
			*field = *sorted
		}

		// Assemble the flat output frame preserving RefID and Meta.
		outFrame := &data.Frame{
			Name:  refID,
			RefID: refID,
		}
		outFrame.Fields = append(outFrame.Fields, timeField, valueField)
		outFrame.Fields = append(outFrame.Fields, dimFields...)
		if len(dr.Frames) > 0 {
			outFrame.Meta = dr.Frames[0].Meta
		}

		dr.Frames = data.Frames{outFrame}
		resp.Responses[refID] = dr
	}
}

// timeAndValueFields returns the first time field and the first value field
// from a frame, or nil if either is absent.
func timeAndValueFields(f *data.Frame) (*data.Field, *data.Field) {
	var tf, vf *data.Field
	for _, field := range f.Fields {
		if field == nil {
			continue
		}
		if tf == nil && field.Type() == data.FieldTypeTime {
			tf = field
		}
		if vf == nil && (field.Type() == data.FieldTypeFloat64 || field.Type() == data.FieldTypeNullableFloat64) {
			vf = field
		}
	}
	return tf, vf
}

// fieldTimeAt returns the time.Time value at index i from a time field,
// handling both time.Time and *time.Time underlying types.
func fieldTimeAt(f *data.Field, i int) (time.Time, bool) {
	v := f.At(i)
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t != nil {
			return *t, true
		}
	}
	return time.Time{}, false
}

// fieldFloat64At returns the *float64 value at index i from a value field,
// handling both float64 and *float64 underlying types.
func fieldFloat64At(f *data.Field, i int) *float64 {
	v := f.At(i)
	switch n := v.(type) {
	case float64:
		return &n
	case *float64:
		return n
	}
	return nil
}
