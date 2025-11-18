import { setupMockedAnnotationQueryRunner } from '../__mocks__/AnnotationQueryRunner';
import { namespaceVariable, regionVariable } from '../__mocks__/CloudWatchDataSource';
import { CloudWatchAnnotationQuery } from '../types';

describe('CloudWatchAnnotationQueryRunner', () => {
  const queries: CloudWatchAnnotationQuery[] = [
    {
      actionPrefix: '',
      alarmNamePrefix: '',
      datasource: { type: 'cloudwatch' },
      dimensions: { InstanceId: 'i-12345678' },
      matchExact: true,
      metricName: 'CPUUtilization',
      period: '300',
      prefixMatching: false,
      queryMode: 'Annotations',
      refId: 'Anno',
      namespace: `$${namespaceVariable.name}`,
      region: `$${regionVariable.name}`,
      statistic: 'Average',
    },
  ];

  it('should issue the correct query', async () => {
    const { runner, queryMock, request } = setupMockedAnnotationQueryRunner({
      variables: [namespaceVariable, regionVariable],
    });
    await expect(runner.handleAnnotationQuery(queries, request, queryMock)).toEmitValuesWith(() => {
      expect(queryMock.mock.calls[0][0].targets[0]).toMatchObject(
        expect.objectContaining({
          region: regionVariable.current.value,
          namespace: namespaceVariable.current.value,
          metricName: queries[0].metricName,
          dimensions: { InstanceId: ['i-12345678'] },
          statistic: queries[0].statistic,
          period: queries[0].period,
        })
      );
    });
  });

  describe('Logs Annotation Query', () => {
    const logsQueries: CloudWatchAnnotationQuery[] = [
      {
        queryMode: 'Annotations',
        annotationType: 'logs',
        refId: 'Anno',
        region: `$${regionVariable.name}`,
        expression: 'fields @timestamp, @message | filter @message like /$logFilter/',
        logGroups: [
          {
            arn: 'arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/test',
            name: '/aws/lambda/test',
          },
        ],
        queryLanguage: 'CWLI',
      },
    ];

    it('should issue the correct logs annotation query', async () => {
      const { runner, queryMock, request } = setupMockedAnnotationQueryRunner({
        variables: [regionVariable],
      });
      await expect(runner.handleAnnotationQuery(logsQueries, request, queryMock)).toEmitValuesWith(() => {
        expect(queryMock.mock.calls[0][0].targets[0]).toMatchObject(
          expect.objectContaining({
            region: regionVariable.current.value,
            expression: logsQueries[0].expression,
            logGroups: logsQueries[0].logGroups,
            queryLanguage: 'CWLI',
          })
        );
      });
    });

    it('should apply template variable interpolation to expression', async () => {
      const logFilterVariable = {
        name: 'logFilter',
        current: { value: 'ERROR', text: 'ERROR' },
      };

      const queriesWithVars: CloudWatchAnnotationQuery[] = [
        {
          ...logsQueries[0],
          expression: 'fields @timestamp, @message | filter @message like /$logFilter/',
        },
      ];

      const { runner, queryMock, request } = setupMockedAnnotationQueryRunner({
        variables: [regionVariable, logFilterVariable],
      });

      await expect(runner.handleAnnotationQuery(queriesWithVars, request, queryMock)).toEmitValuesWith(() => {
        const target = queryMock.mock.calls[0][0].targets[0];
        expect(target.expression).toBe('fields @timestamp, @message | filter @message like /ERROR/');
      });
    });

    it('should include logGroups array', async () => {
      const { runner, queryMock, request } = setupMockedAnnotationQueryRunner({
        variables: [regionVariable],
      });
      await expect(runner.handleAnnotationQuery(logsQueries, request, queryMock)).toEmitValuesWith(() => {
        const target = queryMock.mock.calls[0][0].targets[0];
        expect(target.logGroups).toEqual(logsQueries[0].logGroups);
        expect(target.logGroups).toHaveLength(1);
        expect(target.logGroups[0].name).toBe('/aws/lambda/test');
      });
    });

    it('should not include metrics-specific fields for logs annotations', async () => {
      const { runner, queryMock, request } = setupMockedAnnotationQueryRunner({
        variables: [regionVariable],
      });
      await expect(runner.handleAnnotationQuery(logsQueries, request, queryMock)).toEmitValuesWith(() => {
        const target = queryMock.mock.calls[0][0].targets[0];
        expect(target.namespace).toBeUndefined();
        expect(target.metricName).toBeUndefined();
        expect(target.statistic).toBeUndefined();
        expect(target.dimensions).toBeUndefined();
        expect(target.period).toBeUndefined();
        expect(target.actionPrefix).toBeUndefined();
        expect(target.alarmNamePrefix).toBeUndefined();
      });
    });
  });

  describe('Backward Compatibility', () => {
    it('should handle annotation queries without annotationType field (defaults to metrics)', async () => {
      const queriesWithoutType: CloudWatchAnnotationQuery[] = [
        {
          // No annotationType field - should default to metrics
          datasource: { type: 'cloudwatch' },
          dimensions: { InstanceId: 'i-12345678' },
          matchExact: true,
          metricName: 'CPUUtilization',
          period: '300',
          prefixMatching: false,
          queryMode: 'Annotations',
          refId: 'Anno',
          namespace: 'AWS/EC2',
          region: 'us-east-1',
          statistic: 'Average',
        },
      ];

      const { runner, queryMock, request} = setupMockedAnnotationQueryRunner({
        variables: [],
      });
      await expect(runner.handleAnnotationQuery(queriesWithoutType, request, queryMock)).toEmitValuesWith(() => {
        const target = queryMock.mock.calls[0][0].targets[0];
        expect(target.namespace).toBe('AWS/EC2');
        expect(target.metricName).toBe('CPUUtilization');
        expect(target.statistic).toBe('Average');
      });
    });
  });
});
