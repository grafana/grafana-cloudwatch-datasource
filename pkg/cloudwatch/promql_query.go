package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const instantRefIDSuffix = "-Instant"

type promQLQueryModel struct {
	Region           string `json:"region"`
	PromqlExpression string `json:"promqlExpression"`
	Instant          bool   `json:"instant,omitempty"`
	Range            bool   `json:"range,omitempty"`
}

func (m promQLQueryModel) effectiveModes() (instant, rangeQuery bool) {
	if !m.Instant && !m.Range {
		return false, true
	}
	return m.Instant, m.Range
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

		if rangeQuery {
			resp.Responses[q.RefID] = ds.executePromQLRange(ctx, region, model.PromqlExpression, q)
		}

		if instant {
			instantRefID := q.RefID
			if rangeQuery {
				instantRefID = q.RefID + instantRefIDSuffix
			}
			resp.Responses[instantRefID] = ds.executePromQLInstant(ctx, region, model.PromqlExpression, q, instantRefID)
		}
	}

	return resp, nil
}

func (ds *DataSource) executePromQLRange(ctx context.Context, region, expression string, q backend.DataQuery) backend.DataResponse {
	stepSecs := q.Interval.Seconds()
	if stepSecs < 1 {
		stepSecs = 1
	}

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

	return backend.DataResponse{Frames: convertPromRangeResultToDataFrames(promResp, q.RefID)}
}

func (ds *DataSource) executePromQLInstant(ctx context.Context, region, expression string, q backend.DataQuery, refID string) backend.DataResponse {
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

	return backend.DataResponse{Frames: convertPromInstantResultToDataFrames(promResp, refID)}
}

func convertPromRangeResultToDataFrames(promResp prometheusRangeResponse, refID string) data.Frames {
	var frames data.Frames

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

		frame := data.NewFrame(refID,
			data.NewField("Time", nil, times),
			data.NewField("Value", data.Labels(series.Metric), values),
		)
		frame.RefID = refID
		frames = append(frames, frame)
	}

	return frames
}

func convertPromInstantResultToDataFrames(promResp prometheusInstantResponse, refID string) data.Frames {
	var frames data.Frames

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

		frame := data.NewFrame(refID,
			data.NewField("Time", nil, []time.Time{time.Unix(int64(ts), 0).UTC()}),
			data.NewField("Value", data.Labels(series.Metric), []float64{val}),
		)
		frame.RefID = refID
		frames = append(frames, frame)
	}

	return frames
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
