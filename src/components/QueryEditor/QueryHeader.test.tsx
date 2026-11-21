import { render, screen, waitFor } from '@testing-library/react';
import selectEvent from 'react-select-event';

import { setupMockedDataSource } from '../../__mocks__/CloudWatchDataSource';
import { validLogsQuery, validMetricSearchBuilderQuery } from '../../__mocks__/queries';
import { LogsQueryLanguage } from '../../dataquery';
import { DEFAULT_CWLI_QUERY_STRING } from '../../defaultQueries';

import QueryHeader from './QueryHeader';

const ds = setupMockedDataSource({
  variables: [],
});
ds.datasource.resources.getRegions = jest.fn().mockResolvedValue([]);

describe('QueryHeader', () => {
  describe('when changing region', () => {
    const { datasource } = setupMockedDataSource();
    datasource.resources.getRegions = jest.fn().mockResolvedValue([
      { value: 'us-east-2', label: 'us-east-2' },
      { value: 'us-east-1', label: 'us-east-1' },
    ]);
    it('should reset account id and log groups if new region is not monitoring account', async () => {
      const onChange = jest.fn();
      datasource.resources.isMonitoringAccount = jest.fn().mockResolvedValue(false);
      render(
        <QueryHeader
          datasource={datasource}
          query={{
            ...validMetricSearchBuilderQuery,
            region: 'us-east-1',
            accountId: 'all',
            logGroups: [{ arn: 'arn', name: 'name' }],
          }}
          onChange={onChange}
          onRunQuery={jest.fn()}
          dataIsStale={false}
        />
      );
      await waitFor(() => expect(screen.queryByText('us-east-1')).toBeInTheDocument());
      await selectEvent.select(screen.getByLabelText(/Region/), 'us-east-2', { container: document.body });
      expect(onChange).toHaveBeenCalledWith({
        ...validMetricSearchBuilderQuery,
        region: 'us-east-2',
        accountId: undefined,
        logGroups: [],
      });
    });

    it('should reset log groups but not account id if new region is a monitoring account', async () => {
      const onChange = jest.fn();
      datasource.resources.isMonitoringAccount = jest.fn().mockResolvedValue(true);

      render(
        <QueryHeader
          datasource={datasource}
          query={{
            ...validMetricSearchBuilderQuery,
            region: 'us-east-1',
            accountId: '123',
            logGroups: [{ arn: 'arn', name: 'name' }],
          }}
          onChange={onChange}
          onRunQuery={jest.fn()}
          dataIsStale={false}
        />
      );
      await waitFor(() => expect(screen.queryByText('us-east-1')).toBeInTheDocument());
      await selectEvent.select(screen.getByLabelText(/Region/), 'us-east-2', { container: document.body });
      expect(onChange).toHaveBeenCalledWith({
        ...validMetricSearchBuilderQuery,
        region: 'us-east-2',
        accountId: '123',
        logGroups: [],
      });
    });

    it('should not call isMonitoringAccount if its a logs query', async () => {
      const onChange = jest.fn();
      datasource.resources.isMonitoringAccount = jest.fn().mockResolvedValue(true);

      render(
        <QueryHeader
          dataIsStale={false}
          datasource={datasource}
          query={{ ...validLogsQuery, region: 'us-east-1' }}
          onChange={onChange}
          onRunQuery={jest.fn()}
        />
      );
      await waitFor(() => expect(screen.queryByText('us-east-1')).toBeInTheDocument());
      await selectEvent.select(screen.getByLabelText(/Region/), 'us-east-2', { container: document.body });
      expect(datasource.resources.isMonitoringAccount).not.toHaveBeenCalledWith('us-east-2');
    });
  });

  describe('when changing query mode', () => {
    const { datasource } = setupMockedDataSource();
    it('should set expression to empty when switching to metrics mode', async () => {
      const onChange = jest.fn();
      datasource.resources.isMonitoringAccount = jest.fn().mockResolvedValue(false);
      render(
        <QueryHeader
          datasource={datasource}
          query={{ ...validMetricSearchBuilderQuery, queryMode: 'Logs', expression: 'foo' }}
          onChange={onChange}
          onRunQuery={jest.fn()}
          dataIsStale={false}
        />
      );
      expect(await screen.findByText('CloudWatch Logs')).toBeInTheDocument();
      await selectEvent.select(await screen.findByLabelText('Query mode'), 'CloudWatch Metrics', {
        container: document.body,
      });
      expect(onChange).toHaveBeenCalledWith({
        ...validMetricSearchBuilderQuery,
        queryLanguage: LogsQueryLanguage.CWLI,
        logGroupNames: undefined,
        logGroups: [],
        sqlExpression: '',
        expression: '',
      });
    });
  });
  describe('log expression', () => {
    const { datasource } = setupMockedDataSource();
    it('should disable run query button when empty', async () => {
      const onChange = jest.fn();
      datasource.resources.isMonitoringAccount = jest.fn().mockResolvedValue(false);
      render(
        <QueryHeader
          datasource={datasource}
          query={{ ...validMetricSearchBuilderQuery, queryMode: 'Logs', expression: '' }}
          onChange={onChange}
          onRunQuery={jest.fn()}
          dataIsStale={false}
        />
      );
      expect(await screen.findByText('Run queries')).toBeInTheDocument();
      expect(screen.getByText('Run queries').closest('button')).toBeDisabled();
    });
    it('should enable run query button when set', async () => {
      const onChange = jest.fn();
      datasource.resources.isMonitoringAccount = jest.fn().mockResolvedValue(false);
      render(
        <QueryHeader
          datasource={datasource}
          query={{ ...validMetricSearchBuilderQuery, queryMode: 'Logs', expression: DEFAULT_CWLI_QUERY_STRING }}
          onChange={onChange}
          onRunQuery={jest.fn()}
          dataIsStale={false}
        />
      );
      expect(await screen.findByText('Run queries')).toBeInTheDocument();
      expect(screen.getByText('Run queries').closest('button')).not.toBeDisabled();
    });
  });
});
