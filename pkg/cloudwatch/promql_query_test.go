package cloudwatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertPromRangeResultToDataFrames(t *testing.T) {
	t.Run("converts a single series to a data frame named after __name__", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{"__name__": "http_requests_total", "job": "api"},
				Values: [][]interface{}{
					{float64(1000000), "42.5"},
					{float64(1000060), "43.0"},
				},
			},
		}

		frames := convertPromRangeResultToDataFrames(resp, "A", 60)
		require.Len(t, frames, 1)

		frame := frames[0]
		assert.Equal(t, "A", frame.RefID)
		require.Len(t, frame.Fields, 2)
		require.NotNil(t, frame.Meta)
		assert.Equal(t, float64(60), frame.Meta.Custom.(map[string]interface{})["period"])

		timeField := frame.Fields[0]
		assert.Equal(t, "Time", timeField.Name)
		assert.Equal(t, 2, timeField.Len())
		assert.Equal(t, time.Unix(1000000, 0).UTC(), timeField.At(0))
		assert.Equal(t, time.Unix(1000060, 0).UTC(), timeField.At(1))

		valueField := frame.Fields[1]
		// Value field is named after the __name__ label so @grafana/prometheus's transformV2
		// auto-legend path (which checks `field.labels.__name__ === field.name`) works.
		assert.Equal(t, "http_requests_total", valueField.Name)
		assert.Equal(t, 2, valueField.Len())
		assert.Equal(t, 42.5, valueField.At(0))
		assert.Equal(t, 43.0, valueField.At(1))
		assert.Equal(t, "api", valueField.Labels["job"])
		assert.Equal(t, "http_requests_total", valueField.Labels["__name__"])
	})

	t.Run("falls back to 'Value' when __name__ is missing", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{"instance": "host1"},
				Values: [][]interface{}{{float64(1000), "1.0"}},
			},
		}

		frames := convertPromRangeResultToDataFrames(resp, "A", 60)
		require.Len(t, frames, 1)
		assert.Equal(t, "Value", frames[0].Fields[1].Name)
	})

	t.Run("skips malformed points", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{},
				Values: [][]interface{}{
					{float64(1000), "valid"},  // unparseable float — skipped
					{float64(1001), "99.9"},   // valid
					{"not-a-ts", "1.0"},       // bad timestamp — skipped
					{float64(1002)},           // too short — skipped
				},
			},
		}

		frames := convertPromRangeResultToDataFrames(resp, "C", 60)
		require.Len(t, frames, 1)
		assert.Equal(t, 1, frames[0].Fields[0].Len())
	})

	t.Run("returns empty frames for empty result", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		frames := convertPromRangeResultToDataFrames(resp, "D", 60)
		assert.Empty(t, frames)
	})
}

func TestConvertPromInstantResultToDataFrames(t *testing.T) {
	t.Run("converts a vector result to single-point frames", func(t *testing.T) {
		resp := prometheusInstantResponse{Status: "success"}
		resp.Data.ResultType = "vector"
		resp.Data.Result = []prometheusInstantSeries{
			{
				Metric: map[string]string{"__name__": "up", "job": "api"},
				Value:  []interface{}{float64(1000000), "1"},
			},
			{
				Metric: map[string]string{"__name__": "up", "job": "web"},
				Value:  []interface{}{float64(1000000), "0"},
			},
		}

		frames := convertPromInstantResultToDataFrames(resp, "A")
		require.Len(t, frames, 2)

		first := frames[0]
		assert.Equal(t, "A", first.RefID)
		require.Len(t, first.Fields, 2)
		assert.Equal(t, 1, first.Fields[0].Len())
		assert.Equal(t, time.Unix(1000000, 0).UTC(), first.Fields[0].At(0))
		assert.Equal(t, "up", first.Fields[1].Name)
		assert.Equal(t, 1.0, first.Fields[1].At(0))
		assert.Equal(t, "api", first.Fields[1].Labels["job"])
	})

	t.Run("skips malformed instant points", func(t *testing.T) {
		resp := prometheusInstantResponse{Status: "success"}
		resp.Data.Result = []prometheusInstantSeries{
			{Metric: map[string]string{}, Value: []interface{}{float64(1000), "notanumber"}},
			{Metric: map[string]string{}, Value: []interface{}{"bad-ts", "1.0"}},
			{Metric: map[string]string{}, Value: []interface{}{float64(1000)}},
			{Metric: map[string]string{}, Value: []interface{}{float64(1000), "5.5"}},
		}

		frames := convertPromInstantResultToDataFrames(resp, "B")
		require.Len(t, frames, 1)
		assert.Equal(t, 5.5, frames[0].Fields[1].At(0))
	})

	t.Run("falls back to histogram sum/count", func(t *testing.T) {
		resp := prometheusInstantResponse{Status: "success"}
		resp.Data.Result = []prometheusInstantSeries{
			{
				Metric:    map[string]string{"__name__": "http_request_duration"},
				Histogram: []interface{}{float64(2000000), map[string]interface{}{"sum": "100", "count": "4"}},
			},
		}

		frames := convertPromInstantResultToDataFrames(resp, "C")
		require.Len(t, frames, 1)
		assert.Equal(t, 25.0, frames[0].Fields[1].At(0))
	})

	t.Run("returns empty frames for empty result", func(t *testing.T) {
		resp := prometheusInstantResponse{Status: "success"}
		frames := convertPromInstantResultToDataFrames(resp, "D")
		assert.Empty(t, frames)
	})
}

func TestPromQLQueryModelEffectiveModes(t *testing.T) {
	tests := []struct {
		name        string
		model       promQLQueryModel
		wantInstant bool
		wantRange   bool
	}{
		{"neither set defaults to range", promQLQueryModel{}, false, true},
		{"instant only", promQLQueryModel{Instant: true}, true, false},
		{"range only", promQLQueryModel{Range: true}, false, true},
		{"both true", promQLQueryModel{Instant: true, Range: true}, true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotInstant, gotRange := tc.model.effectiveModes()
			assert.Equal(t, tc.wantInstant, gotInstant)
			assert.Equal(t, tc.wantRange, gotRange)
		})
	}
}

func TestResolveStepSeconds(t *testing.T) {
	tests := []struct {
		name       string
		calculated time.Duration
		minStep    string
		want       float64
	}{
		{"empty min step uses calculated", 30 * time.Second, "", 30},
		{"min step larger than calculated wins", 30 * time.Second, "1m", 60},
		{"min step smaller than calculated is ignored", 60 * time.Second, "10s", 60},
		{"floor of 1s when both are zero", 0, "", 1},
		{"floor of 1s when calculated is sub-second", 500 * time.Millisecond, "", 1},
		{"unparseable min step is ignored", 30 * time.Second, "invalid", 30},
		{"Prometheus duration syntax supported", 30 * time.Second, "1h", 3600},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveStepSeconds(tc.calculated, tc.minStep))
		})
	}
}
