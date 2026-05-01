package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("bare ARN is rejected", func(t *testing.T) {
		arn := "arn:aws:logs:us-east-1:111111111111:log-group:/g"
		_, _, ok := ParseLogGroupTableParameter(arn)
		assert.False(t, ok)
	})

	t.Run("separator with ARN only is rejected", func(t *testing.T) {
		arn := "arn:aws:logs:us-east-1:1:log-group:/z"
		_, _, ok := ParseLogGroupTableParameter(sep + arn)
		assert.False(t, ok)
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
