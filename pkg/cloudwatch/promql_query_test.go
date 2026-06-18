package cloudwatch

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertPromRangeResultToDataFrames(t *testing.T) {
	t.Run("converts a single series to a data frame", func(t *testing.T) {
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

		frames := convertPromRangeResultToDataFrames(resp, "A", "")
		require.Len(t, frames, 1)

		frame := frames[0]
		assert.Equal(t, "A", frame.RefID)
		require.Len(t, frame.Fields, 2)

		timeField := frame.Fields[0]
		assert.Equal(t, "Time", timeField.Name)
		assert.Equal(t, 2, timeField.Len())
		assert.Equal(t, time.Unix(1000000, 0).UTC(), timeField.At(0))
		assert.Equal(t, time.Unix(1000060, 0).UTC(), timeField.At(1))

		valueField := frame.Fields[1]
		assert.Equal(t, "Value", valueField.Name)
		assert.Equal(t, 2, valueField.Len())
		assert.Equal(t, 42.5, valueField.At(0))
		assert.Equal(t, 43.0, valueField.At(1))
		assert.Equal(t, "api", valueField.Labels["job"])
	})

	t.Run("converts multiple series to separate frames", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{"instance": "host1"},
				Values: [][]interface{}{{float64(1000), "1.0"}},
			},
			{
				Metric: map[string]string{"instance": "host2"},
				Values: [][]interface{}{{float64(1000), "2.0"}},
			},
		}

		frames := convertPromRangeResultToDataFrames(resp, "B", "")
		assert.Len(t, frames, 2)
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

		frames := convertPromRangeResultToDataFrames(resp, "C", "")
		require.Len(t, frames, 1)
		assert.Equal(t, 1, frames[0].Fields[0].Len())
	})

	t.Run("returns empty frames for empty result", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		frames := convertPromRangeResultToDataFrames(resp, "D", "")
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

		frames := convertPromInstantResultToDataFrames(resp, "A", "")
		require.Len(t, frames, 2)

		first := frames[0]
		assert.Equal(t, "A", first.RefID)
		require.Len(t, first.Fields, 2)
		assert.Equal(t, 1, first.Fields[0].Len())
		assert.Equal(t, time.Unix(1000000, 0).UTC(), first.Fields[0].At(0))
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

		frames := convertPromInstantResultToDataFrames(resp, "B", "")
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

		frames := convertPromInstantResultToDataFrames(resp, "C", "")
		require.Len(t, frames, 1)
		assert.Equal(t, 25.0, frames[0].Fields[1].At(0))
	})

	t.Run("returns empty frames for empty result", func(t *testing.T) {
		resp := prometheusInstantResponse{Status: "success"}
		frames := convertPromInstantResultToDataFrames(resp, "D", "")
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

func TestConvertPromRangeResultToTable(t *testing.T) {
	t.Run("flattens multiple series into one wide frame", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{"job": "api", "instance": "host1"},
				Values: [][]interface{}{{float64(1000), "1.0"}, {float64(2000), "2.0"}},
			},
			{
				Metric: map[string]string{"job": "web"},
				Values: [][]interface{}{{float64(1000), "3.0"}},
			},
		}

		frames := convertPromRangeResultToTable(resp, "A")
		require.Len(t, frames, 1)

		frame := frames[0]
		assert.Equal(t, "A", frame.RefID)
		require.Len(t, frame.Fields, 4)
		assert.Equal(t, "Time", frame.Fields[0].Name)
		assert.Equal(t, "instance", frame.Fields[1].Name)
		assert.Equal(t, "job", frame.Fields[2].Name)
		assert.Equal(t, "Value", frame.Fields[3].Name)

		assert.Equal(t, 3, frame.Fields[0].Len())
		assert.Equal(t, "host1", frame.Fields[1].At(0))
		assert.Equal(t, "", frame.Fields[1].At(2))
		assert.Equal(t, "api", frame.Fields[2].At(0))
		assert.Equal(t, "web", frame.Fields[2].At(2))
		assert.Equal(t, 1.0, frame.Fields[3].At(0))
		assert.Equal(t, 3.0, frame.Fields[3].At(2))

		assert.NotNil(t, frame.Meta)
		assert.Equal(t, data.VisType("table"), frame.Meta.PreferredVisualization)
	})

	t.Run("returns an empty table frame for empty result", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		frames := convertPromRangeResultToTable(resp, "A")
		require.Len(t, frames, 1)
		assert.Equal(t, 0, frames[0].Fields[0].Len())
	})
}

func TestConvertPromInstantResultToTable(t *testing.T) {
	t.Run("flattens vector result into one row per series", func(t *testing.T) {
		resp := prometheusInstantResponse{Status: "success"}
		resp.Data.Result = []prometheusInstantSeries{
			{
				Metric: map[string]string{"job": "api"},
				Value:  []interface{}{float64(1000), "1.0"},
			},
			{
				Metric: map[string]string{"job": "web"},
				Value:  []interface{}{float64(1000), "2.0"},
			},
		}

		frames := convertPromInstantResultToTable(resp, "A")
		require.Len(t, frames, 1)
		frame := frames[0]

		assert.Equal(t, "A", frame.RefID)
		require.Len(t, frame.Fields, 3)
		assert.Equal(t, 2, frame.Fields[0].Len())
		assert.Equal(t, "api", frame.Fields[1].At(0))
		assert.Equal(t, "web", frame.Fields[1].At(1))
		assert.Equal(t, 1.0, frame.Fields[2].At(0))
		assert.Equal(t, 2.0, frame.Fields[2].At(1))
		assert.Equal(t, data.VisType("table"), frame.Meta.PreferredVisualization)
	})
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

func TestSubstituteLabelPlaceholders(t *testing.T) {
	labels := map[string]string{"job": "api", "instance": "host1"}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"single placeholder substituted", "{{job}}", "api"},
		{"multiple placeholders substituted", "{{job}} - {{instance}}", "api - host1"},
		{"missing label becomes empty string", "{{job}}/{{missing}}", "api/"},
		{"inner whitespace tolerated", "{{ job }}", "api"},
		{"literal text without placeholders passes through", "static-name", "static-name"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, substituteLabelPlaceholders(tc.template, labels))
		})
	}
}

func TestConvertPromRangeResultLegendFormat(t *testing.T) {
	resp := prometheusRangeResponse{Status: "success"}
	resp.Data.Result = []prometheusRangeSeries{
		{
			Metric: map[string]string{"__name__": "RequestCount", "job": "api", "instance": "host1"},
			Values: [][]interface{}{{float64(1000), "1.0"}},
		},
	}

	t.Run("custom template sets DisplayNameFromDS on value field", func(t *testing.T) {
		frames := convertPromRangeResultToDataFrames(resp, "A", "{{job}}/{{instance}}")
		require.Len(t, frames, 1)
		require.NotNil(t, frames[0].Fields[1].Config)
		assert.Equal(t, "api/host1", frames[0].Fields[1].Config.DisplayNameFromDS)
	})

	t.Run("verbose extracts __name__ as the prefix", func(t *testing.T) {
		frames := convertPromRangeResultToDataFrames(resp, "A", "")
		require.Len(t, frames, 1)
		require.NotNil(t, frames[0].Fields[1].Config)
		assert.Equal(t, `RequestCount{instance="host1", job="api"}`, frames[0].Fields[1].Config.DisplayNameFromDS)
	})

	t.Run("__auto on a single-series response keeps every label", func(t *testing.T) {
		frames := convertPromRangeResultToDataFrames(resp, "A", "__auto")
		require.Len(t, frames, 1)
		require.NotNil(t, frames[0].Fields[1].Config)
		assert.Equal(t, `RequestCount{instance="host1", job="api"}`, frames[0].Fields[1].Config.DisplayNameFromDS)
	})

	t.Run("__auto strips labels common to every series", func(t *testing.T) {
		multiResp := prometheusRangeResponse{Status: "success"}
		multiResp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{"__name__": "RequestCount", "job": "api", "instance": "host1"},
				Values: [][]interface{}{{float64(1000), "1.0"}},
			},
			{
				Metric: map[string]string{"__name__": "RequestCount", "job": "api", "instance": "host2"},
				Values: [][]interface{}{{float64(1000), "2.0"}},
			},
		}

		frames := convertPromRangeResultToDataFrames(multiResp, "A", "__auto")
		require.Len(t, frames, 2)
		assert.Equal(t, `RequestCount{instance="host1"}`, frames[0].Fields[1].Config.DisplayNameFromDS)
		assert.Equal(t, `RequestCount{instance="host2"}`, frames[1].Fields[1].Config.DisplayNameFromDS)
	})

	t.Run("__auto falls back to the metric name when every label is common", func(t *testing.T) {
		multiResp := prometheusRangeResponse{Status: "success"}
		multiResp.Data.Result = []prometheusRangeSeries{
			{
				Metric: map[string]string{"__name__": "RequestCount", "job": "api"},
				Values: [][]interface{}{{float64(1000), "1.0"}},
			},
			{
				Metric: map[string]string{"__name__": "RequestCount", "job": "api"},
				Values: [][]interface{}{{float64(1000), "2.0"}},
			},
		}

		frames := convertPromRangeResultToDataFrames(multiResp, "A", "__auto")
		require.Len(t, frames, 2)
		assert.Equal(t, "RequestCount", frames[0].Fields[1].Config.DisplayNameFromDS)
	})
}

func TestRenderNameAndLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		common map[string]string
		want   string
	}{
		{
			name:   "verbose: name plus every label except __name__",
			labels: map[string]string{"__name__": "metric", "job": "api", "instance": "h1"},
			common: nil,
			want:   `metric{instance="h1", job="api"}`,
		},
		{
			name:   "auto: name plus only the distinguishing labels",
			labels: map[string]string{"__name__": "metric", "job": "api", "instance": "h1"},
			common: map[string]string{"job": "api"},
			want:   `metric{instance="h1"}`,
		},
		{
			name:   "name only when no other labels exist",
			labels: map[string]string{"__name__": "metric"},
			want:   "metric",
		},
		{
			name:   "name only when every label is common (auto fully collapses)",
			labels: map[string]string{"__name__": "metric", "job": "api"},
			common: map[string]string{"job": "api"},
			want:   "metric",
		},
		{
			name:   "missing __name__ still renders the label braces",
			labels: map[string]string{"job": "api"},
			want:   `{job="api"}`,
		},
		{
			name:   "missing __name__ and nothing distinguishing returns empty",
			labels: map[string]string{"job": "api"},
			common: map[string]string{"job": "api"},
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, renderNameAndLabels(tc.labels, tc.common))
		})
	}
}

func TestConvertPromInstantResultLegendFormat(t *testing.T) {
	resp := prometheusInstantResponse{Status: "success"}
	resp.Data.ResultType = "vector"
	resp.Data.Result = []prometheusInstantSeries{
		{
			Metric: map[string]string{"job": "api"},
			Value:  []interface{}{float64(1000), "1.0"},
		},
	}

	frames := convertPromInstantResultToDataFrames(resp, "A", "{{job}}")
	require.Len(t, frames, 1)
	require.NotNil(t, frames[0].Fields[1].Config)
	assert.Equal(t, "api", frames[0].Fields[1].Config.DisplayNameFromDS)
}
