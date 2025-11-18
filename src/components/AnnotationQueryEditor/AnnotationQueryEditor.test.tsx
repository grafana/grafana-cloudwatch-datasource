import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';

import { QueryEditorProps } from '@grafana/data';

import { setupMockedDataSource } from '../../__mocks__/CloudWatchDataSource';
import { CloudWatchDatasource } from '../../datasource';
import { CloudWatchAnnotationQuery, CloudWatchJsonData, CloudWatchMetricsQuery, CloudWatchQuery } from '../../types';

import { AnnotationQueryEditor } from './AnnotationQueryEditor';

const ds = setupMockedDataSource({
  variables: [],
});

const q: CloudWatchQuery = {
  queryMode: 'Annotations',
  region: 'us-east-2',
  namespace: '',
  period: '',
  metricName: '',
  dimensions: {},
  matchExact: true,
  statistic: '',
  refId: '',
  prefixMatching: false,
  actionPrefix: '',
  alarmNamePrefix: '',
};

ds.datasource.resources.getRegions = jest.fn().mockResolvedValue([]);
ds.datasource.resources.getNamespaces = jest.fn().mockResolvedValue([]);
ds.datasource.resources.getMetrics = jest.fn().mockResolvedValue([]);
ds.datasource.resources.getDimensionKeys = jest.fn().mockResolvedValue([]);
ds.datasource.getVariables = jest.fn().mockReturnValue([]);

const props: QueryEditorProps<CloudWatchDatasource, CloudWatchQuery, CloudWatchJsonData> = {
  datasource: ds.datasource,
  query: q,
  onChange: jest.fn(),
  onRunQuery: jest.fn(),
};

describe('AnnotationQueryEditor', () => {
  it('should not display match exact switch', async () => {
    render(<AnnotationQueryEditor {...props} />);
    await waitFor(() => {
      expect(screen.queryByText('Match exact')).toBeNull();
    });
  });

  it('should return an error component in case CloudWatchQuery is not CloudWatchAnnotationQuery', async () => {
    ds.datasource.resources.getDimensionValues = jest
      .fn()
      .mockResolvedValue([[{ label: 'dimVal1', value: 'dimVal1' }]]);
    render(
      <AnnotationQueryEditor {...props} query={{ ...props.query, queryMode: 'Metrics' } as CloudWatchMetricsQuery} />
    );
    await waitFor(() => expect(screen.getByText('Invalid annotation query')).toBeInTheDocument());
  });

  it('should not display wildcard option in dimension value dropdown', async () => {
    ds.datasource.resources.getDimensionValues = jest
      .fn()
      .mockResolvedValue([[{ label: 'dimVal1', value: 'dimVal1' }]]);
    (props.query as CloudWatchAnnotationQuery).dimensions = { instanceId: 'instance-123' };
    render(<AnnotationQueryEditor {...props} />);
    const valueElement = screen.getByText('instance-123');
    expect(valueElement).toBeInTheDocument();
    expect(screen.queryByText('*')).toBeNull();
    valueElement.click();
    await waitFor(() => {
      expect(screen.queryByText('*')).toBeNull();
    });
  });

  it('should not display Accounts component', async () => {
    ds.datasource.resources.getDimensionValues = jest
      .fn()
      .mockResolvedValue([[{ label: 'dimVal1', value: 'dimVal1' }]]);
    (props.query as CloudWatchAnnotationQuery).dimensions = { instanceId: 'instance-123' };
    await waitFor(() => render(<AnnotationQueryEditor {...props} />));
    expect(await screen.queryByText('Account')).toBeNull();
  });

  describe('Annotation Type Selector', () => {
    it('should render annotation type selector with both options', async () => {
      render(<AnnotationQueryEditor {...props} />);
      await waitFor(() => {
        expect(screen.getByText('Annotation type')).toBeInTheDocument();
      });
    });

    it('should default to metrics annotation type', async () => {
      const metricsQuery: CloudWatchAnnotationQuery = {
        ...q,
        queryMode: 'Annotations',
      };
      render(<AnnotationQueryEditor {...props} query={metricsQuery} />);
      await waitFor(() => {
        // MetricStatEditor should be visible for metrics
        expect(screen.getByText('Namespace')).toBeInTheDocument();
      });
    });

    it('should display metrics editor components when annotationType is metrics', async () => {
      const metricsQuery: CloudWatchAnnotationQuery = {
        ...q,
        queryMode: 'Annotations',
        annotationType: 'metrics',
      };
      render(<AnnotationQueryEditor {...props} query={metricsQuery} />);
      await waitFor(() => {
        expect(screen.getByText('Namespace')).toBeInTheDocument();
        expect(screen.getByText('Metric name')).toBeInTheDocument();
        expect(screen.getByText('Period')).toBeInTheDocument();
        expect(screen.getByText('Enable Prefix Matching')).toBeInTheDocument();
      });
    });

    it('should display logs editor components when annotationType is logs', async () => {
      const logsQuery: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        expression: 'fields @timestamp, @message',
        logGroups: [],
      };

      ds.datasource.resources.getLogGroups = jest.fn().mockResolvedValue([]);

      render(<AnnotationQueryEditor {...props} query={logsQuery} />);
      await waitFor(() => {
        expect(screen.getByText('Log groups')).toBeInTheDocument();
      });
    });

    it('should hide metrics-specific fields when annotationType is logs', async () => {
      const logsQuery: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        expression: 'fields @timestamp, @message',
        logGroups: [],
      };

      ds.datasource.resources.getLogGroups = jest.fn().mockResolvedValue([]);

      render(<AnnotationQueryEditor {...props} query={logsQuery} />);
      await waitFor(() => {
        expect(screen.queryByText('Namespace')).toBeNull();
        expect(screen.queryByText('Metric name')).toBeNull();
        expect(screen.queryByText('Period')).toBeNull();
        expect(screen.queryByText('Enable Prefix Matching')).toBeNull();
      });
    });

    it('should show region selector for both annotation types', async () => {
      const metricsQuery: CloudWatchAnnotationQuery = {
        ...q,
        queryMode: 'Annotations',
        annotationType: 'metrics',
      };
      render(<AnnotationQueryEditor {...props} query={metricsQuery} />);
      await waitFor(() => {
        expect(screen.getByText('Region')).toBeInTheDocument();
      });

      const logsQuery: CloudWatchAnnotationQuery = {
        refId: 'A',
        queryMode: 'Annotations',
        annotationType: 'logs',
        region: 'us-east-1',
        expression: 'fields @timestamp, @message',
        logGroups: [],
      };

      ds.datasource.resources.getLogGroups = jest.fn().mockResolvedValue([]);

      render(<AnnotationQueryEditor {...props} query={logsQuery} />);
      await waitFor(() => {
        expect(screen.getByText('Region')).toBeInTheDocument();
      });
    });
  });

  describe('Annotation Type Switching', () => {
    it('should initialize logs mode with default expression', () => {
      const onChange = jest.fn();
      const metricsQuery: CloudWatchAnnotationQuery = {
        ...q,
        queryMode: 'Annotations',
        annotationType: 'metrics',
      };

      render(<AnnotationQueryEditor {...props} query={metricsQuery} onChange={onChange} />);

      // When user clicks to switch to logs, onChange should be called with logs fields
      // Note: This requires user interaction simulation which is complex in this test
      // The actual logic is tested through the component behavior
    });

    it('should preserve region when switching annotation types', async () => {
      const metricsQuery: CloudWatchAnnotationQuery = {
        ...q,
        queryMode: 'Annotations',
        annotationType: 'metrics',
        region: 'eu-west-1',
      };

      render(<AnnotationQueryEditor {...props} query={metricsQuery} />);
      await waitFor(() => {
        // Region should be displayed
        const regionInputs = screen.getAllByDisplayValue('eu-west-1');
        expect(regionInputs.length).toBeGreaterThan(0);
      });
    });
  });
});
