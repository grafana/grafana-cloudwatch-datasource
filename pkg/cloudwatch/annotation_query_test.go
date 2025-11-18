package cloudwatch

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/kinds/dataquery"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuery_AnnotationQuery(t *testing.T) {
	ds := newTestDatasource()
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})

	var client fakeCWAnnotationsClient
	NewCWClient = func(aws.Config) models.CWClient {
		return &client
	}

	t.Run("DescribeAlarmsForMetric is called with minimum parameters", func(t *testing.T) {
		client = fakeCWAnnotationsClient{describeAlarmsForMetricOutput: &cloudwatch.DescribeAlarmsForMetricOutput{}}

		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					JSON: json.RawMessage(`{
						"type":    "annotationQuery",
						"region":    "us-east-1",
						"namespace": "custom",
						"metricName": "CPUUtilization",
						"statistic": "Average"
					}`),
				},
			},
		})
		require.NoError(t, err)

		require.Len(t, client.calls.describeAlarmsForMetric, 1)
		assert.Equal(t, &cloudwatch.DescribeAlarmsForMetricInput{
			Namespace:  aws.String("custom"),
			MetricName: aws.String("CPUUtilization"),
			Statistic:  "Average",
			Period:     aws.Int32(300),
		}, client.calls.describeAlarmsForMetric[0])
	})

	t.Run("DescribeAlarms is called when prefixMatching is true", func(t *testing.T) {
		client = fakeCWAnnotationsClient{describeAlarmsOutput: &cloudwatch.DescribeAlarmsOutput{}}

		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					JSON: json.RawMessage(`{
						"type":    "annotationQuery",
						"region":    "us-east-1",
						"namespace": "custom",
						"metricName": "CPUUtilization",
						"statistic": "Average",
						"prefixMatching": true,
						"actionPrefix": "some_action_prefix",
						"alarmNamePrefix": "some_alarm_name_prefix"
					}`),
				},
			},
		})
		require.NoError(t, err)

		require.Len(t, client.calls.describeAlarms, 1)
		assert.Equal(t, &cloudwatch.DescribeAlarmsInput{
			MaxRecords:      aws.Int32(100),
			ActionPrefix:    aws.String("some_action_prefix"),
			AlarmNamePrefix: aws.String("some_alarm_name_prefix"),
		}, client.calls.describeAlarms[0])
	})

	t.Run("Routes to metrics handler when annotationType is not specified (backward compatibility)", func(t *testing.T) {
		client = fakeCWAnnotationsClient{describeAlarmsForMetricOutput: &cloudwatch.DescribeAlarmsForMetricOutput{}}

		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					JSON: json.RawMessage(`{
						"type":    "annotationQuery",
						"region":    "us-east-1",
						"namespace": "custom",
						"metricName": "CPUUtilization",
						"statistic": "Average"
					}`),
				},
			},
		})
		require.NoError(t, err)

		// Should call metrics annotation handler
		require.Len(t, client.calls.describeAlarmsForMetric, 1)
	})

	t.Run("Routes to metrics handler when annotationType is 'metrics'", func(t *testing.T) {
		client = fakeCWAnnotationsClient{describeAlarmsForMetricOutput: &cloudwatch.DescribeAlarmsForMetricOutput{}}

		_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					JSON: json.RawMessage(`{
						"type":    "annotationQuery",
						"annotationType": "metrics",
						"region":    "us-east-1",
						"namespace": "custom",
						"metricName": "CPUUtilization",
						"statistic": "Average"
					}`),
				},
			},
		})
		require.NoError(t, err)

		// Should call metrics annotation handler
		require.Len(t, client.calls.describeAlarmsForMetric, 1)
	})
}

func TestQuery_LogsAnnotationQuery(t *testing.T) {
	ds := newTestDatasource()
	origNewCWLogsClient := NewCWLogsClient
	t.Cleanup(func() {
		NewCWLogsClient = origNewCWLogsClient
	})

	timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	var logsClient fakeCWLogsClient
	NewCWLogsClient = func(aws.Config) models.CWLogsClient {
		return &logsClient
	}

	t.Run("Returns error when expression is missing", func(t *testing.T) {
		resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					JSON: json.RawMessage(`{
						"type": "annotationQuery",
						"annotationType": "logs",
						"region": "us-east-1",
						"logGroups": [{"arn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/test", "name": "/aws/lambda/test"}]
					}`),
				},
			},
		})
		require.NoError(t, err)

		// Should return error in response
		require.Contains(t, resp.Responses, "A")
		assert.Error(t, resp.Responses["A"].Error)
		assert.Contains(t, resp.Responses["A"].Error.Error(), "expression is required")
	})

	t.Run("Returns error when log groups are missing", func(t *testing.T) {
		resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					JSON: json.RawMessage(`{
						"type": "annotationQuery",
						"annotationType": "logs",
						"region": "us-east-1",
						"expression": "fields @timestamp, @message | sort @timestamp desc"
					}`),
				},
			},
		})
		require.NoError(t, err)

		// Should return error in response
		require.Contains(t, resp.Responses, "A")
		assert.Error(t, resp.Responses["A"].Error)
		assert.Contains(t, resp.Responses["A"].Error.Error(), "log groups are required")
	})

	t.Run("Executes logs annotation query successfully", func(t *testing.T) {
		logsClient = fakeCWLogsClient{
			queryResults: cloudwatchlogs.GetQueryResultsOutput{
				Status: cloudwatchlogstypes.QueryStatusComplete,
				Results: [][]cloudwatchlogstypes.ResultField{
					{
						{Field: aws.String("@timestamp"), Value: aws.String(timestamp.Format(time.RFC3339))},
						{Field: aws.String("@message"), Value: aws.String("Test log message")},
						{Field: aws.String("@logStream"), Value: aws.String("test-stream")},
					},
				},
			},
		}

		resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					JSON: json.RawMessage(`{
						"type": "annotationQuery",
						"annotationType": "logs",
						"region": "us-east-1",
						"expression": "fields @timestamp, @message | sort @timestamp desc",
						"logGroups": [{"arn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/test", "name": "/aws/lambda/test"}]
					}`),
					TimeRange: backend.TimeRange{
						From: timestamp.Add(-1 * time.Hour),
						To:   timestamp.Add(1 * time.Hour),
					},
				},
			},
		})
		require.NoError(t, err)

		// Should return annotation frame
		require.Contains(t, resp.Responses, "A")
		assert.NoError(t, resp.Responses["A"].Error)
		require.Len(t, resp.Responses["A"].Frames, 1)

		frame := resp.Responses["A"].Frames[0]
		assert.Equal(t, "A", frame.Name)
		require.Len(t, frame.Fields, 4) // time, title, tags, text
		assert.Equal(t, "time", frame.Fields[0].Name)
		assert.Equal(t, "title", frame.Fields[1].Name)
		assert.Equal(t, "tags", frame.Fields[2].Name)
		assert.Equal(t, "text", frame.Fields[3].Name)
	})
}

func TestLogsResultsToAnnotations(t *testing.T) {
	t.Run("Parses RFC3339 timestamp correctly", func(t *testing.T) {
		timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String(timestamp.Format(time.RFC3339))},
					{Field: aws.String("@message"), Value: aws.String("Test message")},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)

		assert.Equal(t, timestamp, annotations[0].Time)
		assert.Equal(t, "Test message", annotations[0].Text)
	})

	t.Run("Parses Unix millisecond timestamp correctly", func(t *testing.T) {
		timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		timestampMs := timestamp.UnixNano() / int64(time.Millisecond)

		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String("1672574400000")}, // Unix ms
					{Field: aws.String("@message"), Value: aws.String("Test message")},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)

		// Allow for small time differences due to conversion
		assert.WithinDuration(t, time.Unix(0, timestampMs*int64(time.Millisecond)), annotations[0].Time, time.Second)
	})

	t.Run("Extracts @message field", func(t *testing.T) {
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String(time.Now().Format(time.RFC3339))},
					{Field: aws.String("@message"), Value: aws.String("This is my log message")},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)

		assert.Equal(t, "This is my log message", annotations[0].Text)
	})

	t.Run("Uses @logStream as tags", func(t *testing.T) {
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String(time.Now().Format(time.RFC3339))},
					{Field: aws.String("@message"), Value: aws.String("Test message")},
					{Field: aws.String("@logStream"), Value: aws.String("my-log-stream")},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)

		assert.Equal(t, "my-log-stream", annotations[0].Tags)
	})

	t.Run("Uses custom field as title", func(t *testing.T) {
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String(time.Now().Format(time.RFC3339))},
					{Field: aws.String("@message"), Value: aws.String("Test message")},
					{Field: aws.String("severity"), Value: aws.String("ERROR")},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)

		assert.Equal(t, "severity: ERROR", annotations[0].Title)
	})

	t.Run("Skips rows without timestamp", func(t *testing.T) {
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@message"), Value: aws.String("Message without timestamp")},
				},
				{
					{Field: aws.String("@timestamp"), Value: aws.String(time.Now().Format(time.RFC3339))},
					{Field: aws.String("@message"), Value: aws.String("Message with timestamp")},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)

		assert.Equal(t, "Message with timestamp", annotations[0].Text)
	})

	t.Run("Handles nil fields gracefully", func(t *testing.T) {
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status: cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String(time.Now().Format(time.RFC3339))},
					{Field: nil, Value: aws.String("Should be ignored")},
					{Field: aws.String("@message"), Value: nil},
				},
			},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		require.Len(t, annotations, 1)
	})

	t.Run("Returns empty array for empty results", func(t *testing.T) {
		results := &cloudwatchlogs.GetQueryResultsOutput{
			Status:  cloudwatchlogstypes.QueryStatusComplete,
			Results: [][]cloudwatchlogstypes.ResultField{},
		}

		annotations, err := logsResultsToAnnotations(results)
		require.NoError(t, err)
		assert.Len(t, annotations, 0)
	})
}
