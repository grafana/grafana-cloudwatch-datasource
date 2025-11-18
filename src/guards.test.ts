import { CloudWatchAnnotationQuery, CloudWatchLogsQuery, CloudWatchMetricsQuery, CloudWatchQuery } from './types';
import { isLogsAnnotationQuery, isMetricsAnnotationQuery, isCloudWatchAnnotationQuery } from './guards';

describe('Annotation Query Guards', () => {
  describe('isMetricsAnnotationQuery', () => {
    it('should return true when annotationType is "metrics"', () => {
      const query: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'metrics',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
      };

      expect(isMetricsAnnotationQuery(query)).toBe(true);
    });

    it('should return true when annotationType is undefined (backward compatibility)', () => {
      const query: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
      };

      expect(isMetricsAnnotationQuery(query)).toBe(true);
    });

    it('should return false when annotationType is "logs"', () => {
      const query: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        expression: 'fields @timestamp, @message',
        logGroups: [],
      };

      expect(isMetricsAnnotationQuery(query)).toBe(false);
    });

    it('should return false for non-annotation queries', () => {
      const metricsQuery: CloudWatchMetricsQuery = {
        refId: 'A',
        id: '',
        queryMode: 'Metrics',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
        period: '',
        expression: '',
        dimensions: {},
      };

      expect(isMetricsAnnotationQuery(metricsQuery)).toBe(false);
    });

    it('should return false for logs queries', () => {
      const logsQuery: CloudWatchLogsQuery = {
        refId: 'A',
        id: '',
        queryMode: 'Logs',
        region: 'us-east-1',
        expression: 'fields @timestamp',
        logGroups: [],
      };

      expect(isMetricsAnnotationQuery(logsQuery)).toBe(false);
    });
  });

  describe('isLogsAnnotationQuery', () => {
    it('should return true when annotationType is "logs"', () => {
      const query: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        expression: 'fields @timestamp, @message',
        logGroups: [],
      };

      expect(isLogsAnnotationQuery(query)).toBe(true);
    });

    it('should return false when annotationType is "metrics"', () => {
      const query: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'metrics',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
      };

      expect(isLogsAnnotationQuery(query)).toBe(false);
    });

    it('should return false when annotationType is undefined', () => {
      const query: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
      };

      expect(isLogsAnnotationQuery(query)).toBe(false);
    });

    it('should return false for non-annotation queries', () => {
      const metricsQuery: CloudWatchMetricsQuery = {
        refId: 'A',
        id: '',
        queryMode: 'Metrics',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
        period: '',
        expression: '',
        dimensions: {},
      };

      expect(isLogsAnnotationQuery(metricsQuery)).toBe(false);
    });

    it('should return false for logs queries', () => {
      const logsQuery: CloudWatchLogsQuery = {
        refId: 'A',
        id: '',
        queryMode: 'Logs',
        region: 'us-east-1',
        expression: 'fields @timestamp',
        logGroups: [],
      };

      expect(isLogsAnnotationQuery(logsQuery)).toBe(false);
    });
  });

  describe('Type Narrowing', () => {
    it('should narrow type correctly for metrics annotations', () => {
      const query: CloudWatchQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'metrics',
        region: 'us-east-1',
        namespace: 'AWS/EC2',
        metricName: 'CPUUtilization',
        statistic: 'Average',
      } as CloudWatchAnnotationQuery;

      if (isMetricsAnnotationQuery(query)) {
        // Should have access to metrics annotation properties
        expect(query.namespace).toBe('AWS/EC2');
        expect(query.metricName).toBe('CPUUtilization');
        expect(query.statistic).toBe('Average');
      }
    });

    it('should narrow type correctly for logs annotations', () => {
      const query: CloudWatchQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        expression: 'fields @timestamp, @message',
        logGroups: [],
      } as CloudWatchAnnotationQuery;

      if (isLogsAnnotationQuery(query)) {
        // Should have access to logs annotation properties
        expect(query.expression).toBe('fields @timestamp, @message');
        expect(query.logGroups).toEqual([]);
      }
    });
  });
});
