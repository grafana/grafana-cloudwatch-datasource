package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogGroupNameFromARN(t *testing.T) {
	t.Run("parses standard log group ARN", func(t *testing.T) {
		name, ok := LogGroupNameFromARN("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/foo")
		require.True(t, ok)
		assert.Equal(t, "/aws/lambda/foo", name)
	})

	t.Run("strips trailing :*", func(t *testing.T) {
		name, ok := LogGroupNameFromARN("arn:aws:logs:eu-west-1:1:log-group:my-group:*")
		require.True(t, ok)
		assert.Equal(t, "my-group", name)
	})

	t.Run("rejects log stream ARNs", func(t *testing.T) {
		_, ok := LogGroupNameFromARN("arn:aws:logs:us-east-1:1:log-group:my-group:log-stream:abc")
		assert.False(t, ok)
	})

	t.Run("rejects empty and malformed", func(t *testing.T) {
		for _, s := range []string{"", "arn:aws:s3:::bucket", "arn:aws:logs:us-east-1:1:log-group:"} {
			_, ok := LogGroupNameFromARN(s)
			assert.False(t, ok, s)
		}
	})
}

func TestFormatAndParseLogGroupTableParameter(t *testing.T) {
	sep := LogGroupTableParameterSeparator
	t.Run("round-trip name and ARN", func(t *testing.T) {
		s := FormatLogGroupTableParameter("/aws/lambda/x", "arn:aws:logs:us-east-1:1:log-group:/aws/lambda/x")
		n, a, ok := ParseLogGroupTableParameter(s)
		require.True(t, ok)
		assert.Equal(t, "/aws/lambda/x", n)
		assert.Equal(t, "arn:aws:logs:us-east-1:1:log-group:/aws/lambda/x", a)
		assert.Contains(t, s, sep)
	})

	t.Run("name only", func(t *testing.T) {
		n, a, ok := ParseLogGroupTableParameter(FormatLogGroupTableParameter("myname", ""))
		require.True(t, ok)
		assert.Equal(t, "myname", n)
		assert.Equal(t, "", a)
	})

	t.Run("bare ARN", func(t *testing.T) {
		arn := "arn:aws:logs:us-east-1:111111111111:log-group:/g"
		n, a, ok := ParseLogGroupTableParameter(arn)
		require.True(t, ok)
		assert.Equal(t, "/g", n)
		assert.Equal(t, arn, a)
	})

	t.Run("separator with ARN only", func(t *testing.T) {
		arn := "arn:aws:logs:us-east-1:1:log-group:/z"
		n, a, ok := ParseLogGroupTableParameter(sep + arn)
		require.True(t, ok)
		assert.Equal(t, "/z", n)
		assert.Equal(t, arn, a)
	})
}

func TestLogsTableAccountIDForAPI(t *testing.T) {
	assert.Nil(t, logsTableAccountIDForAPI(""))
	assert.Nil(t, logsTableAccountIDForAPI("   "))
	assert.Nil(t, logsTableAccountIDForAPI(LogsAccountSelfSentinel))

	p := logsTableAccountIDForAPI("123456789012")
	require.NotNil(t, p)
	assert.Equal(t, "123456789012", *p)

	pAll := logsTableAccountIDForAPI("all")
	require.NotNil(t, pAll)
	assert.Equal(t, "all", *pAll)
}
