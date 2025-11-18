package cloudwatch

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/kinds/dataquery"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type annotationEvent struct {
	Title string
	Time  time.Time
	Tags  string
	Text  string
}

func (ds *DataSource) executeAnnotationQuery(ctx context.Context, model DataQueryJson, query backend.DataQuery) (*backend.QueryDataResponse, error) {
	result := backend.NewQueryDataResponse()

	// Route to logs annotation handler if annotation type is logs
	if model.AnnotationType != nil && *model.AnnotationType == dataquery.CloudWatchAnnotationTypeLogs {
		return ds.executeLogsAnnotationQuery(ctx, model, query)
	}

	// Default to metrics annotation (CloudWatch Alarms) for backward compatibility
	statistic := ""

	if model.Statistic != nil {
		statistic = *model.Statistic
	}

	var period int32

	if model.Period != nil && *model.Period != "" {
		p, err := strconv.ParseInt(*model.Period, 10, 32)
		if err != nil {
			return nil, backend.DownstreamError(fmt.Errorf("query period must be an int"))
		}
		period = int32(p)
	}

	prefixMatching := false
	if model.PrefixMatching != nil {
		prefixMatching = *model.PrefixMatching
	}
	if period == 0 && !prefixMatching {
		period = 300
	}

	actionPrefix := model.ActionPrefix
	alarmNamePrefix := model.AlarmNamePrefix

	cli, err := ds.getCWClient(ctx, model.Region)
	if err != nil {
		result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(fmt.Errorf("%v: %w", "failed to get client", err))
		return result, nil
	}

	var alarmNames []*string
	metricName := ""
	if model.MetricName != nil {
		metricName = *model.MetricName
	}

	dimensions := dataquery.Dimensions{}
	if model.Dimensions != nil {
		dimensions = *model.Dimensions
	}

	if prefixMatching {
		params := &cloudwatch.DescribeAlarmsInput{
			MaxRecords:      aws.Int32(100),
			ActionPrefix:    actionPrefix,
			AlarmNamePrefix: alarmNamePrefix,
		}
		resp, err := cli.DescribeAlarms(ctx, params)
		if err != nil {
			result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("%v: %w", "failed to call cloudwatch:DescribeAlarms", err)))
			return result, nil
		}
		alarmNames = filterAlarms(resp, model.Namespace, metricName, dimensions, statistic, period)
	} else {
		if model.Region == "" || model.Namespace == "" || metricName == "" || statistic == "" {
			return result, backend.DownstreamError(errors.New("invalid annotations query"))
		}

		var qd []cloudwatchtypes.Dimension
		for k, v := range dimensions {
			for _, vvv := range v.ArrayOfString {
				qd = append(qd, cloudwatchtypes.Dimension{
					Name:  aws.String(k),
					Value: aws.String(vvv),
				})
			}
		}
		params := &cloudwatch.DescribeAlarmsForMetricInput{
			Namespace:  aws.String(model.Namespace),
			MetricName: aws.String(metricName),
			Dimensions: qd,
			Statistic:  cloudwatchtypes.Statistic(statistic),
			Period:     aws.Int32(period),
		}
		resp, err := cli.DescribeAlarmsForMetric(ctx, params)
		if err != nil {
			result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("%v: %w", "failed to call cloudwatch:DescribeAlarmsForMetric", err)))
			return result, nil
		}
		for _, alarm := range resp.MetricAlarms {
			alarmNames = append(alarmNames, alarm.AlarmName)
		}
	}

	annotations := make([]*annotationEvent, 0)
	for _, alarmName := range alarmNames {
		params := &cloudwatch.DescribeAlarmHistoryInput{
			AlarmName:  alarmName,
			StartDate:  aws.Time(query.TimeRange.From),
			EndDate:    aws.Time(query.TimeRange.To),
			MaxRecords: aws.Int32(100),
		}
		resp, err := cli.DescribeAlarmHistory(ctx, params)
		if err != nil {
			result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("%v: %w", "failed to call cloudwatch:DescribeAlarmHistory", err)))
			return result, nil
		}
		for _, history := range resp.AlarmHistoryItems {
			annotations = append(annotations, &annotationEvent{
				Time:  *history.Timestamp,
				Title: *history.AlarmName,
				Tags:  string(history.HistoryItemType),
				Text:  *history.HistorySummary,
			})
		}
	}

	respD := result.Responses[query.RefID]
	respD.Frames = append(respD.Frames, transformAnnotationToTable(annotations, query))
	result.Responses[query.RefID] = respD

	return result, err
}

func transformAnnotationToTable(annotations []*annotationEvent, query backend.DataQuery) *data.Frame {
	frame := data.NewFrame(query.RefID,
		data.NewField("time", nil, []time.Time{}),
		data.NewField("title", nil, []string{}),
		data.NewField("tags", nil, []string{}),
		data.NewField("text", nil, []string{}),
	)

	for _, a := range annotations {
		frame.AppendRow(a.Time, a.Title, a.Tags, a.Text)
	}

	frame.Meta = &data.FrameMeta{
		Custom: map[string]any{
			"rowCount": len(annotations),
		},
	}

	return frame
}

func filterAlarms(alarms *cloudwatch.DescribeAlarmsOutput, namespace string, metricName string,
	dimensions dataquery.Dimensions, statistic string, period int32) []*string {
	alarmNames := make([]*string, 0)

	for _, alarm := range alarms.MetricAlarms {
		if namespace != "" && *alarm.Namespace != namespace {
			continue
		}
		if metricName != "" && *alarm.MetricName != metricName {
			continue
		}

		matchDimension := true
		if len(dimensions) != 0 {
			if len(alarm.Dimensions) != len(dimensions) {
				matchDimension = false
			} else {
				for _, d := range alarm.Dimensions {
					if _, ok := dimensions[*d.Name]; !ok {
						matchDimension = false
					}
				}
			}
		}
		if !matchDimension {
			continue
		}

		if string(alarm.Statistic) != statistic {
			continue
		}

		if period != 0 && *alarm.Period != period {
			continue
		}

		alarmNames = append(alarmNames, alarm.AlarmName)
	}

	return alarmNames
}

func (ds *DataSource) executeLogsAnnotationQuery(ctx context.Context, model DataQueryJson, query backend.DataQuery) (*backend.QueryDataResponse, error) {
	result := backend.NewQueryDataResponse()

	// Validate logs annotation query
	if model.Expression == nil || *model.Expression == "" {
		result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(errors.New("expression is required for logs annotations")))
		return result, nil
	}

	if (model.LogGroups == nil || len(model.LogGroups) == 0) && (model.LogGroupNames == nil || len(model.LogGroupNames) == 0) {
		result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(errors.New("log groups are required for logs annotations")))
		return result, nil
	}

	region := model.Region
	if region == "" || region == defaultRegion {
		region = ds.Settings.Region
	}

	logsClient, err := ds.getCWLogsClient(ctx, region)
	if err != nil {
		result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(fmt.Errorf("%v: %w", "failed to get logs client", err))
		return result, nil
	}

	// Create logs query from annotation query
	logsQuery := models.LogsQuery{
		CloudWatchLogsQuery: dataquery.CloudWatchLogsQuery{
			Region:        region,
			Expression:    model.Expression,
			LogGroups:     model.LogGroups,
			LogGroupNames: model.LogGroupNames,
			QueryLanguage: model.QueryLanguage,
		},
		QueryString: *model.Expression,
	}

	// Execute the logs query synchronously
	getQueryResultsOutput, err := ds.syncQuery(ctx, logsClient, query, logsQuery, ds.Settings.LogsTimeout.Duration)
	if err != nil {
		result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(backend.DownstreamError(fmt.Errorf("%v: %w", "failed to execute logs query", err)))
		return result, nil
	}

	// Transform logs results to annotations
	annotations, err := logsResultsToAnnotations(getQueryResultsOutput)
	if err != nil {
		result.Responses[query.RefID] = backend.ErrorResponseWithErrorSource(fmt.Errorf("%v: %w", "failed to transform logs results", err))
		return result, nil
	}

	respD := result.Responses[query.RefID]
	respD.Frames = append(respD.Frames, transformAnnotationToTable(annotations, query))
	result.Responses[query.RefID] = respD

	return result, nil
}

func logsResultsToAnnotations(results *cloudwatchlogs.GetQueryResultsOutput) ([]*annotationEvent, error) {
	annotations := make([]*annotationEvent, 0)

	for _, row := range results.Results {
		var timestamp time.Time
		var message string
		title := "Log Event"
		tags := ""

		// Extract fields from the log result
		for _, field := range row {
			if field.Field == nil || field.Value == nil {
				continue
			}

			switch *field.Field {
			case "@timestamp":
				// Parse timestamp
				t, err := time.Parse(time.RFC3339, *field.Value)
				if err != nil {
					// Try parsing as Unix timestamp in milliseconds
					var ms int64
					_, err := fmt.Sscanf(*field.Value, "%d", &ms)
					if err == nil {
						timestamp = time.Unix(0, ms*int64(time.Millisecond))
					}
				} else {
					timestamp = t
				}
			case "@message":
				message = *field.Value
			case "@logStream":
				tags = *field.Value
			default:
				// Use the first non-standard field as title if available
				if title == "Log Event" && *field.Field != "@ptr" && *field.Field != "@log" {
					title = fmt.Sprintf("%s: %s", *field.Field, *field.Value)
				}
			}
		}

		// Only create annotation if we have a timestamp
		if !timestamp.IsZero() {
			annotations = append(annotations, &annotationEvent{
				Time:  timestamp,
				Title: title,
				Tags:  tags,
				Text:  message,
			})
		}
	}

	return annotations, nil
}
