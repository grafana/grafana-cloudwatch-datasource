package cloudwatch

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPromQLForwardParams(t *testing.T) {
	t.Run("forwards start, end, limit unchanged", func(t *testing.T) {
		in := url.Values{
			"start": []string{"1000"},
			"end":   []string{"2000"},
			"limit": []string{"500"},
		}
		out := buildPromQLForwardParams(in)
		assert.Equal(t, "1000", out.Get("start"))
		assert.Equal(t, "2000", out.Get("end"))
		assert.Equal(t, "500", out.Get("limit"))
	})

	t.Run("renames match to match[] for AWS PromQL", func(t *testing.T) {
		in := url.Values{"match": []string{`{__name__="CPUUtilization"}`}}
		out := buildPromQLForwardParams(in)
		assert.Equal(t, `{__name__="CPUUtilization"}`, out.Get("match[]"))
		assert.Equal(t, "", out.Get("match"), "original `match` key should not be forwarded")
	})

	t.Run("drops empty values", func(t *testing.T) {
		in := url.Values{
			"start": []string{""},
			"end":   []string{""},
			"match": []string{""},
		}
		out := buildPromQLForwardParams(in)
		assert.Empty(t, out, "no params should be forwarded when all are empty")
	})

	t.Run("does not forward region or other unrelated params", func(t *testing.T) {
		in := url.Values{
			"region":   []string{"us-east-1"},
			"labelKey": []string{"InstanceId"},
			"start":    []string{"1000"},
		}
		out := buildPromQLForwardParams(in)
		assert.Equal(t, "", out.Get("region"))
		assert.Equal(t, "", out.Get("labelKey"))
		assert.Equal(t, "1000", out.Get("start"))
	})
}

func TestDecodePromQLStringListResponse(t *testing.T) {
	t.Run("parses success response with values", func(t *testing.T) {
		data, err := decodePromQLStringListResponse([]byte(`{"status":"success","data":["a","b","c"]}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, data)
	})

	t.Run("returns empty slice (not nil) when data is null", func(t *testing.T) {
		data, err := decodePromQLStringListResponse([]byte(`{"status":"success","data":null}`))
		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Empty(t, data)
	})

	t.Run("returns empty slice when data field is omitted", func(t *testing.T) {
		data, err := decodePromQLStringListResponse([]byte(`{"status":"success"}`))
		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Empty(t, data)
	})

	t.Run("errors when status is not success", func(t *testing.T) {
		_, err := decodePromQLStringListResponse([]byte(`{"status":"error","errorType":"bad_data","error":"invalid match expression"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad_data")
		assert.Contains(t, err.Error(), "invalid match expression")
	})

	t.Run("errors on malformed JSON", func(t *testing.T) {
		_, err := decodePromQLStringListResponse([]byte(`{"status":`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("errors when status field is missing entirely", func(t *testing.T) {
		_, err := decodePromQLStringListResponse([]byte(`{"data":["a"]}`))
		require.Error(t, err)
	})
}
