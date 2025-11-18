import { AnnotationQuery } from '@grafana/data';

import { CloudWatchAnnotationSupport } from './annotationSupport';
import { CloudWatchAnnotationQuery, LegacyAnnotationQuery } from './types';

const metricStatAnnotationQuery: CloudWatchAnnotationQuery = {
  queryMode: 'Annotations',
  region: 'us-east-2',
  namespace: 'AWS/EC2',
  period: '300',
  metricName: 'CPUUtilization',
  dimensions: { InstanceId: 'i-123' },
  matchExact: true,
  statistic: 'Average',
  refId: 'anno',
  prefixMatching: false,
  actionPrefix: '',
  alarmNamePrefix: '',
};

const prefixMatchingAnnotationQuery: CloudWatchAnnotationQuery = {
  queryMode: 'Annotations',
  region: 'us-east-2',
  namespace: '',
  period: '300',
  metricName: '',
  dimensions: undefined,
  statistic: 'Average',
  refId: 'anno',
  prefixMatching: true,
  actionPrefix: 'arn',
  alarmNamePrefix: 'test-alarm',
};

const annotationQuery: AnnotationQuery<CloudWatchAnnotationQuery> = {
  name: 'Anno',
  enable: false,
  iconColor: '',
  target: metricStatAnnotationQuery!,
};

const legacyAnnotationQuery: LegacyAnnotationQuery = {
  name: 'Anno',
  enable: false,
  iconColor: '',
  region: '',
  namespace: 'AWS/EC2',
  period: '300',
  metricName: 'CPUUtilization',
  dimensions: { InstanceId: 'i-123' },
  matchExact: true,
  statistic: '',
  refId: '',
  prefixMatching: false,
  actionPrefix: '',
  alarmNamePrefix: '',
  target: {
    limit: 0,
    matchAny: false,
    tags: [],
    type: '',
  },
  alias: '',
  builtIn: 0,
  datasource: undefined,
  expression: '',
  hide: false,
  id: '',
  type: '',
  statistics: [],
};

describe('annotationSupport', () => {
  describe('when prepareAnnotation', () => {
    describe('is being called with new style annotations', () => {
      it('should return the same query without changing it', () => {
        const preparedAnnotation = CloudWatchAnnotationSupport.prepareAnnotation(annotationQuery);
        expect(preparedAnnotation).toEqual(annotationQuery);
      });
    });

    describe('is being called with legacy annotations', () => {
      it('should return a new query', () => {
        const preparedAnnotation = CloudWatchAnnotationSupport.prepareAnnotation(legacyAnnotationQuery);
        expect(preparedAnnotation).not.toEqual(annotationQuery);
      });

      it('should set default values if not given', () => {
        const preparedAnnotation = CloudWatchAnnotationSupport.prepareAnnotation(legacyAnnotationQuery);
        expect(preparedAnnotation.target?.statistic).toEqual('Average');
        expect(preparedAnnotation.target?.region).toEqual('default');
        expect(preparedAnnotation.target?.queryMode).toEqual('Annotations');
        expect(preparedAnnotation.target?.refId).toEqual('annotationQuery');
      });

      it('should not set default values if given', () => {
        const annotation = CloudWatchAnnotationSupport.prepareAnnotation({
          ...legacyAnnotationQuery,
          statistic: 'Min',
          region: 'us-east-2',
          queryMode: 'Annotations',
          refId: 'A',
        });
        expect(annotation.target?.statistic).toEqual('Min');
        expect(annotation.target?.region).toEqual('us-east-2');
        expect(annotation.target?.queryMode).toEqual('Annotations');
        expect(annotation.target?.refId).toEqual('A');
      });
    });
  });

  describe('when prepareQuery', () => {
    describe('is being called without a target', () => {
      it('should return undefined', () => {
        const preparedQuery = CloudWatchAnnotationSupport.prepareQuery({
          ...annotationQuery,
          target: undefined,
        });
        expect(preparedQuery).toBeUndefined();
      });
    });

    describe('is being called with a complete metric stat query', () => {
      it('should return the annotation target', () => {
        expect(CloudWatchAnnotationSupport.prepareQuery(annotationQuery)).toEqual(annotationQuery.target);
      });
    });

    describe('is being called with an incomplete metric stat query', () => {
      it('should return undefined', () => {
        const preparedQuery = CloudWatchAnnotationSupport.prepareQuery({
          ...annotationQuery,
          target: {
            ...annotationQuery.target!,
            dimensions: {},
            metricName: '',
            statistic: undefined,
          },
        });
        expect(preparedQuery).toBeUndefined();
      });
    });

    describe('is being called with an incomplete prefix matching query', () => {
      it('should return the annotation target', () => {
        const query = {
          ...annotationQuery,
          target: prefixMatchingAnnotationQuery,
        };
        expect(CloudWatchAnnotationSupport.prepareQuery(query)).toEqual(query.target);
      });
    });

    describe('is being called with an incomplete prefix matching query', () => {
      it('should return undefined', () => {
        const query = {
          ...annotationQuery,
          target: {
            ...prefixMatchingAnnotationQuery,
            actionPrefix: '',
          },
        };
        expect(CloudWatchAnnotationSupport.prepareQuery(query)).toBeUndefined();
      });
    });

    describe('Logs Annotation Validation', () => {
      const validLogsAnnotationQuery: CloudWatchAnnotationQuery = {
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        refId: 'anno',
        expression: 'fields @timestamp, @message | sort @timestamp desc',
        logGroups: [
          {
            arn: 'arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/test',
            name: '/aws/lambda/test',
          },
        ],
      };

      describe('is being called with a complete logs annotation query', () => {
        it('should return the annotation target', () => {
          const query = {
            ...annotationQuery,
            target: validLogsAnnotationQuery,
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toEqual(validLogsAnnotationQuery);
        });
      });

      describe('is being called with logs annotation query missing expression', () => {
        it('should return undefined', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              expression: undefined,
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toBeUndefined();
        });

        it('should return undefined when expression is empty string', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              expression: '',
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toBeUndefined();
        });
      });

      describe('is being called with logs annotation query missing log groups', () => {
        it('should return undefined when logGroups is empty', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              logGroups: [],
              logGroupNames: undefined,
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toBeUndefined();
        });

        it('should return undefined when logGroups is undefined', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              logGroups: undefined,
              logGroupNames: undefined,
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toBeUndefined();
        });
      });

      describe('is being called with logs annotation query using legacy logGroupNames', () => {
        it('should return the annotation target when logGroupNames is provided', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              logGroups: undefined,
              logGroupNames: ['/aws/lambda/test'],
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toEqual(query.target);
        });

        it('should return undefined when logGroupNames is empty', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              logGroups: undefined,
              logGroupNames: [],
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toBeUndefined();
        });
      });

      describe('is being called with logs annotation query with both logGroups and logGroupNames', () => {
        it('should return the annotation target (logGroups takes precedence)', () => {
          const query = {
            ...annotationQuery,
            target: {
              ...validLogsAnnotationQuery,
              logGroups: [
                {
                  arn: 'arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/test',
                  name: '/aws/lambda/test',
                },
              ],
              logGroupNames: ['/aws/lambda/legacy'],
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(query)).toEqual(query.target);
        });
      });
    });

    describe('Backward Compatibility', () => {
      describe('is being called with metrics annotation without annotationType field', () => {
        it('should return the annotation target (defaults to metrics)', () => {
          const queryWithoutType = {
            ...annotationQuery,
            target: {
              queryMode: 'Annotations' as const,
              region: 'us-east-2',
              namespace: 'AWS/EC2',
              metricName: 'CPUUtilization',
              statistic: 'Average',
              refId: 'anno',
            },
          };
          expect(CloudWatchAnnotationSupport.prepareQuery(queryWithoutType)).toEqual(queryWithoutType.target);
        });
      });
    });
  });
});
