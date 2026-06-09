package cloudwatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertPromRangeResultToDataFrames(t *testing.T) {
	t.Run("converts a single series to a data frame", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []struct {
			Metric     map[string]string `json:"metric"`
			Values     [][]interface{}   `json:"values"`
			Histograms [][]interface{}   `json:"histograms"`
		}{
			{
				Metric: map[string]string{"__name__": "http_requests_total", "job": "api"},
				Values: [][]interface{}{
					{float64(1000000), "42.5"},
					{float64(1000060), "43.0"},
				},
			},
		}

		frames := convertPromRangeResultToDataFrames(resp, "A")
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
		resp.Data.Result = []struct {
			Metric     map[string]string `json:"metric"`
			Values     [][]interface{}   `json:"values"`
			Histograms [][]interface{}   `json:"histograms"`
		}{
			{
				Metric: map[string]string{"instance": "host1"},
				Values: [][]interface{}{{float64(1000), "1.0"}},
			},
			{
				Metric: map[string]string{"instance": "host2"},
				Values: [][]interface{}{{float64(1000), "2.0"}},
			},
		}

		frames := convertPromRangeResultToDataFrames(resp, "B")
		assert.Len(t, frames, 2)
	})

	t.Run("skips malformed points", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		resp.Data.Result = []struct {
			Metric     map[string]string `json:"metric"`
			Values     [][]interface{}   `json:"values"`
			Histograms [][]interface{}   `json:"histograms"`
		}{
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

		frames := convertPromRangeResultToDataFrames(resp, "C")
		require.Len(t, frames, 1)
		assert.Equal(t, 1, frames[0].Fields[0].Len())
	})

	t.Run("returns empty frames for empty result", func(t *testing.T) {
		resp := prometheusRangeResponse{Status: "success"}
		frames := convertPromRangeResultToDataFrames(resp, "D")
		assert.Empty(t, frames)
	})
}
