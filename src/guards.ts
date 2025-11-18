import { AnnotationQuery } from '@grafana/data';

import {
  CloudWatchAnnotationQuery,
  CloudWatchLogsAnomaliesQuery,
  CloudWatchLogsQuery,
  CloudWatchMetricsQuery,
  CloudWatchQuery,
  LogsMode,
} from './types';

export const isCloudWatchLogsQuery = (cloudwatchQuery: CloudWatchQuery): cloudwatchQuery is CloudWatchLogsQuery =>
  cloudwatchQuery.queryMode === 'Logs';

export const isLogsAnomaliesQuery = (
  cloudwatchQuery: CloudWatchQuery
): cloudwatchQuery is CloudWatchLogsAnomaliesQuery => {
  if (isCloudWatchLogsQuery(cloudwatchQuery)) {
    return cloudwatchQuery.logsMode === LogsMode.Anomalies;
  }
  return false;
};

export const isCloudWatchMetricsQuery = (cloudwatchQuery: CloudWatchQuery): cloudwatchQuery is CloudWatchMetricsQuery =>
  cloudwatchQuery.queryMode === 'Metrics' || !cloudwatchQuery.hasOwnProperty('queryMode'); // in early versions of this plugin, queryMode wasn't defined in a CloudWatchMetricsQuery

export const isCloudWatchAnnotationQuery = (
  cloudwatchQuery: CloudWatchQuery
): cloudwatchQuery is CloudWatchAnnotationQuery => cloudwatchQuery.queryMode === 'Annotations';

export const isMetricsAnnotationQuery = (query: CloudWatchQuery): query is CloudWatchAnnotationQuery => {
  if (!isCloudWatchAnnotationQuery(query)) {
    return false;
  }
  // Default to metrics for backward compatibility (when annotationType is not set)
  return !query.annotationType || query.annotationType === 'metrics';
};

export const isLogsAnnotationQuery = (query: CloudWatchQuery): query is CloudWatchAnnotationQuery => {
  if (!isCloudWatchAnnotationQuery(query)) {
    return false;
  }
  return query.annotationType === 'logs';
};

export const isCloudWatchAnnotation = (query: unknown): query is AnnotationQuery<CloudWatchAnnotationQuery> =>
  (query as AnnotationQuery<CloudWatchAnnotationQuery>).target?.queryMode === 'Annotations';
