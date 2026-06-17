package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const (
	instantRefIDSuffix = "-Instant"
	formatTimeSeries   = "time_series"
	formatTable        = "table"
	legendFormatVerbose = ""
	legendFormatAuto    = "__auto"
)

var legendTemplateRe = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

type promQLQueryModel struct {
	Region           string `json:"region"`
	PromqlExpression string `json:"promqlExpression"`
	Instant          bool   `json:"instant,omitempty"`
	Range            bool   `json:"range,omitempty"`
	Interval         string `json:"interval,omitempty"`
	Format           string `json:"format,omitempty"`
	LegendFormat *string `json:"legendFormat,omitempty"`
}

func (m promQLQueryModel) effectiveModes() (instant, rangeQuery bool) {
	if !m.Instant && !m.Range {
		return false, true
	}
	return m.Instant, m.Range
}

func resolveStepSeconds(calculated time.Duration, minStep string) float64 {
	step := calculated.Seconds()
	if minStep != "" {
		if d, err := gtime.ParseIntervalStringToTimeDuration(minStep); err == nil && d.Seconds() > step {
			step = d.Seconds()
		}
	}
	if step < 1 {
		step = 1
	}
	return step
}

type prometheusRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric     map[string]string `json:"metric"`
			Values     [][]interface{}   `json:"values"`
			Histograms [][]interface{}   `json:"histograms"`
		} `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

type prometheusInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric    map[string]string `json:"metric"`
			Value     []interface{}     `json:"value,omitempty"`
			Histogram []interface{}     `json:"histogram,omitempty"`
		} `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

func (ds *DataSource) executePromQLQuery(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()

	for _, q := range req.Queries {
		var model promQLQueryModel
		if err := json.Unmarshal(q.JSON, &model); err != nil {
			resp.Responses[q.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("failed to parse PromQL query: %w", err)))
			continue
		}

		if strings.TrimSpace(model.PromqlExpression) == "" {
			resp.Responses[q.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("PromQL expression is required")))
			continue
		}

		region := model.Region
		if region == defaultRegion || region == "" {
			region = ds.Settings.Region
		}

		instant, rangeQuery := model.effectiveModes()

		legendFormat := legendFormatAuto
		if model.LegendFormat != nil {
			legendFormat = *model.LegendFormat
		}

		if rangeQuery {
			resp.Responses[q.RefID] = ds.executePromQLRange(ctx, region, model.PromqlExpression, model.Interval, model.Format, legendFormat, q)
		}

		if instant {
			instantRefID := q.RefID
			if rangeQuery {
				instantRefID = q.RefID + instantRefIDSuffix
			}
			resp.Responses[instantRefID] = ds.executePromQLInstant(ctx, region, model.PromqlExpression, model.Format, legendFormat, q, instantRefID)
		}
	}

	return resp, nil
}

func (ds *DataSource) executePromQLRange(ctx context.Context, region, expression, minStep, format, legendFormat string, q backend.DataQuery) backend.DataResponse {
	stepSecs := resolveStepSeconds(q.Interval, minStep)

	params := url.Values{}
	params.Set("query", expression)
	params.Set("start", strconv.FormatInt(q.TimeRange.From.Unix(), 10))
	params.Set("end", strconv.FormatInt(q.TimeRange.To.Unix(), 10))
	params.Set("step", strconv.FormatFloat(stepSecs, 'f', 0, 64))

	body, status, err := ds.promqlSignedGet(ctx, region, "/api/v1/query_range", params, 60*time.Second)
	if err != nil {
		return backend.ErrorResponseWithErrorSource(err)
	}
	if status != http.StatusOK {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("CloudWatch PromQL API returned %d: %s", status, body)))
	}

	var promResp prometheusRangeResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("failed to parse response: %w", err)))
	}
	if promResp.Status != "success" {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("PromQL error (%s): %s", promResp.ErrorType, promResp.Error)))
	}

	if format == formatTable {
		return backend.DataResponse{Frames: convertPromRangeResultToTable(promResp, q.RefID)}
	}
	return backend.DataResponse{Frames: convertPromRangeResultToDataFrames(promResp, q.RefID, legendFormat)}
}

func (ds *DataSource) executePromQLInstant(ctx context.Context, region, expression, format, legendFormat string, q backend.DataQuery, refID string) backend.DataResponse {
	params := url.Values{}
	params.Set("query", expression)
	params.Set("time", strconv.FormatInt(q.TimeRange.To.Unix(), 10))

	body, status, err := ds.promqlSignedGet(ctx, region, "/api/v1/query", params, 60*time.Second)
	if err != nil {
		return backend.ErrorResponseWithErrorSource(err)
	}
	if status != http.StatusOK {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("CloudWatch PromQL API returned %d: %s", status, body)))
	}

	var promResp prometheusInstantResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("failed to parse response: %w", err)))
	}
	if promResp.Status != "success" {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("PromQL error (%s): %s", promResp.ErrorType, promResp.Error)))
	}

	if format == formatTable {
		return backend.DataResponse{Frames: convertPromInstantResultToTable(promResp, refID)}
	}
	return backend.DataResponse{Frames: convertPromInstantResultToDataFrames(promResp, refID, legendFormat)}
}

func convertPromRangeResultToDataFrames(promResp prometheusRangeResponse, refID, legendFormat string) data.Frames {
	var frames data.Frames

	labelSets := make([]map[string]string, len(promResp.Data.Result))
	for i, s := range promResp.Data.Result {
		labelSets[i] = s.Metric
	}
	commonLabels := commonLabelsForAutoLegend(legendFormat, labelSets)

	for _, series := range promResp.Data.Result {
		times := make([]time.Time, 0)
		values := make([]float64, 0)

		for _, point := range series.Values {
			if len(point) != 2 {
				continue
			}
			ts, ok := point[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := point[1].(string)
			if !ok {
				continue
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}
			times = append(times, time.Unix(int64(ts), 0).UTC())
			values = append(values, val)
		}

		if len(times) == 0 {
			for _, point := range series.Histograms {
				if len(point) != 2 {
					continue
				}
				ts, ok := point[0].(float64)
				if !ok {
					continue
				}
				h, ok := point[1].(map[string]interface{})
				if !ok {
					continue
				}
				sum, count := histogramSumCount(h)
				if count == 0 {
					continue
				}
				times = append(times, time.Unix(int64(ts), 0).UTC())
				values = append(values, sum/count)
			}
		}

		valueField := data.NewField("Value", data.Labels(series.Metric), values)
		applyLegendFormat(valueField, legendFormat, series.Metric, commonLabels)
		frame := data.NewFrame(refID,
			data.NewField("Time", nil, times),
			valueField,
		)
		frame.RefID = refID
		frames = append(frames, frame)
	}

	return frames
}

func convertPromInstantResultToDataFrames(promResp prometheusInstantResponse, refID, legendFormat string) data.Frames {
	var frames data.Frames

	labelSets := make([]map[string]string, len(promResp.Data.Result))
	for i, s := range promResp.Data.Result {
		labelSets[i] = s.Metric
	}
	commonLabels := commonLabelsForAutoLegend(legendFormat, labelSets)

	for _, series := range promResp.Data.Result {
		ts, val, ok := extractInstantPoint(series.Value)
		if !ok {
			if h, hasHistogram := extractInstantHistogram(series.Histogram); hasHistogram {
				ts = h.ts
				val = h.value
				ok = true
			}
		}
		if !ok {
			continue
		}

		valueField := data.NewField("Value", data.Labels(series.Metric), []float64{val})
		applyLegendFormat(valueField, legendFormat, series.Metric, commonLabels)
		frame := data.NewFrame(refID,
			data.NewField("Time", nil, []time.Time{time.Unix(int64(ts), 0).UTC()}),
			valueField,
		)
		frame.RefID = refID
		frames = append(frames, frame)
	}

	return frames
}

func applyLegendFormat(field *data.Field, legendFormat string, labels, commonLabels map[string]string) {
	var rendered string

	switch legendFormat {
	case legendFormatVerbose:
		rendered = renderNameAndLabels(labels, nil)
	case legendFormatAuto:
		rendered = renderNameAndLabels(labels, commonLabels)
	default:
		rendered = substituteLabelPlaceholders(legendFormat, labels)
	}

	if rendered == "" {
		return
	}

	if field.Config == nil {
		field.Config = &data.FieldConfig{}
	}

	field.Config.DisplayNameFromDS = rendered
}

func renderNameAndLabels(labels, commonLabels map[string]string) string {
	name := labels["__name__"]
	keys := make([]string, 0, len(labels))

	for k, v := range labels {
		cv, isCommon := commonLabels[k]
		if k == "__name__" || (isCommon && cv == v) {
			continue
		}

		keys = append(keys, k)
	}

	sort.Strings(keys)
	if len(keys) == 0 {
		return name
	}
	
	return name + formatLabelBraces(keys, labels)
}

func formatLabelBraces(keys []string, labels map[string]string) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func substituteLabelPlaceholders(template string, labels map[string]string) string {
	return legendTemplateRe.ReplaceAllStringFunc(template, func(match string) string {
		groups := legendTemplateRe.FindStringSubmatch(match)
		if len(groups) < 2 {
			return ""
		}
		return labels[groups[1]]
	})
}

func commonLabelsForAutoLegend(legendFormat string, labelSets []map[string]string) map[string]string {
	if legendFormat != legendFormatAuto || len(labelSets) <= 1 {
		return nil
	}
	common := make(map[string]string, len(labelSets[0]))
	for k, v := range labelSets[0] {
		common[k] = v
	}
	for _, ls := range labelSets[1:] {
		for k, v := range common {
			if other, ok := ls[k]; !ok || other != v {
				delete(common, k)
			}
		}
	}
	return common
}

func convertPromRangeResultToTable(promResp prometheusRangeResponse, refID string) data.Frames {
	metrics := make([]map[string]string, 0, len(promResp.Data.Result))
	for _, s := range promResp.Data.Result {
		metrics = append(metrics, s.Metric)
	}

	labelNames := collectLabelNames(metrics)
	times := make([]time.Time, 0)
	values := make([]float64, 0)
	labelCols := initLabelColumns(labelNames)

	for _, series := range promResp.Data.Result {
		for _, point := range series.Values {
			ts, val, ok := parseStringPoint(point)
			if !ok {
				continue
			}
			times = append(times, time.Unix(int64(ts), 0).UTC())
			values = append(values, val)
			appendLabelRow(labelCols, labelNames, series.Metric)
		}
		if len(series.Values) == 0 {
			for _, point := range series.Histograms {
				ts, val, ok := parseHistogramPoint(point)
				if !ok {
					continue
				}
				times = append(times, time.Unix(int64(ts), 0).UTC())
				values = append(values, val)
				appendLabelRow(labelCols, labelNames, series.Metric)
			}
		}
	}

	return data.Frames{buildTableFrame(refID, times, values, labelNames, labelCols)}
}

func convertPromInstantResultToTable(promResp prometheusInstantResponse, refID string) data.Frames {
	metrics := make([]map[string]string, 0, len(promResp.Data.Result))
	for _, s := range promResp.Data.Result {
		metrics = append(metrics, s.Metric)
	}
	labelNames := collectLabelNames(metrics)
	times := make([]time.Time, 0)
	values := make([]float64, 0)
	labelCols := initLabelColumns(labelNames)

	for _, series := range promResp.Data.Result {
		ts, val, ok := extractInstantPoint(series.Value)
		if !ok {
			if h, hasHistogram := extractInstantHistogram(series.Histogram); hasHistogram {
				ts, val, ok = h.ts, h.value, true
			}
		}
		if !ok {
			continue
		}
		times = append(times, time.Unix(int64(ts), 0).UTC())
		values = append(values, val)
		appendLabelRow(labelCols, labelNames, series.Metric)
	}

	return data.Frames{buildTableFrame(refID, times, values, labelNames, labelCols)}
}

func collectLabelNames(labelSets []map[string]string) []string {
	seen := map[string]struct{}{}
	for _, ls := range labelSets {
		for k := range ls {
			seen[k] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func initLabelColumns(names []string) map[string][]string {
	cols := make(map[string][]string, len(names))
	for _, n := range names {
		cols[n] = make([]string, 0)
	}
	return cols
}

func appendLabelRow(cols map[string][]string, names []string, labels map[string]string) {
	for _, n := range names {
		cols[n] = append(cols[n], labels[n])
	}
}

// parseStringPoint handles the [timestamp, "value-string"] shape used by both /query_range Values
// entries and /query Value (vector) entries.
func parseStringPoint(point []interface{}) (float64, float64, bool) {
	if len(point) != 2 {
		return 0, 0, false
	}
	ts, ok := point[0].(float64)
	if !ok {
		return 0, 0, false
	}
	valStr, ok := point[1].(string)
	if !ok {
		return 0, 0, false
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, 0, false
	}
	return ts, val, true
}

func parseHistogramPoint(point []interface{}) (float64, float64, bool) {
	if len(point) != 2 {
		return 0, 0, false
	}
	ts, ok := point[0].(float64)
	if !ok {
		return 0, 0, false
	}
	h, ok := point[1].(map[string]interface{})
	if !ok {
		return 0, 0, false
	}
	sum, count := histogramSumCount(h)
	if count == 0 {
		return 0, 0, false
	}
	return ts, sum / count, true
}

func buildTableFrame(refID string, times []time.Time, values []float64, labelNames []string, labelCols map[string][]string) *data.Frame {
	fields := make([]*data.Field, 0, 2+len(labelNames))
	fields = append(fields, data.NewField("Time", nil, times))
	for _, name := range labelNames {
		fields = append(fields, data.NewField(name, nil, labelCols[name]))
	}
	fields = append(fields, data.NewField("Value", nil, values))

	frame := data.NewFrame(refID, fields...)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{PreferredVisualization: data.VisTypeTable}
	return frame
}

func extractInstantPoint(point []interface{}) (ts float64, val float64, ok bool) {
	if len(point) != 2 {
		return 0, 0, false
	}
	ts, ok = point[0].(float64)
	if !ok {
		return 0, 0, false
	}
	valStr, isStr := point[1].(string)
	if !isStr {
		return 0, 0, false
	}
	parsed, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, 0, false
	}
	return ts, parsed, true
}

type instantHistogramPoint struct {
	ts    float64
	value float64
}

func extractInstantHistogram(point []interface{}) (instantHistogramPoint, bool) {
	if len(point) != 2 {
		return instantHistogramPoint{}, false
	}
	ts, ok := point[0].(float64)
	if !ok {
		return instantHistogramPoint{}, false
	}
	h, ok := point[1].(map[string]interface{})
	if !ok {
		return instantHistogramPoint{}, false
	}
	sum, count := histogramSumCount(h)
	if count == 0 {
		return instantHistogramPoint{}, false
	}
	return instantHistogramPoint{ts: ts, value: sum / count}, true
}

func histogramSumCount(h map[string]interface{}) (sum, count float64) {
	if s, ok := h["sum"]; ok {
		switch v := s.(type) {
		case float64:
			sum = v
		case string:
			sum, _ = strconv.ParseFloat(v, 64)
		}
	}
	if c, ok := h["count"]; ok {
		switch v := c.(type) {
		case float64:
			count = v
		case string:
			count, _ = strconv.ParseFloat(v, 64)
		}
	}
	return sum, count
}
