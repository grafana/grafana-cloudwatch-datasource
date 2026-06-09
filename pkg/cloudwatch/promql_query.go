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

type promQLQueryModel struct {
	Region           string `json:"region"`
	PromqlExpression string `json:"promqlExpression"`
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

		stepSecs := q.Interval.Seconds()
		if stepSecs < 1 {
			stepSecs = 1
		}

		params := url.Values{}
		params.Set("query", model.PromqlExpression)
		params.Set("start", strconv.FormatInt(q.TimeRange.From.Unix(), 10))
		params.Set("end", strconv.FormatInt(q.TimeRange.To.Unix(), 10))
		params.Set("step", strconv.FormatFloat(stepSecs, 'f', 0, 64))

		body, status, err := ds.promqlSignedGet(ctx, region, "/api/v1/query_range", params, 60*time.Second)
		if err != nil {
			// Source already tagged inside promqlSignedGet (downstream for network/credentials, plugin for build/sign failures).
			resp.Responses[q.RefID] = backend.ErrorResponseWithErrorSource(err)
			continue
		}
		if status != http.StatusOK {
			resp.Responses[q.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("CloudWatch PromQL API returned %d: %s", status, body)))
			continue
		}

		var promResp prometheusRangeResponse
		if err := json.Unmarshal(body, &promResp); err != nil {
			resp.Responses[q.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("failed to parse response: %w", err)))
			continue
		}
		if promResp.Status != "success" {
			resp.Responses[q.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("PromQL error (%s): %s", promResp.ErrorType, promResp.Error)))
			continue
		}

		resp.Responses[q.RefID] = backend.DataResponse{Frames: convertPromRangeResultToDataFrames(promResp, q.RefID)}
	}

	return resp, nil
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
