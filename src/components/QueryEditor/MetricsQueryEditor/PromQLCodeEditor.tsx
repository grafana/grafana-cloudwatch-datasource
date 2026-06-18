import { CoreApp, TimeRange } from '@grafana/data';
import { PromQueryBuilderOptions, PromQueryField, type PromQuery } from '@grafana/prometheus';
import { Stack } from '@grafana/ui';

import { CloudWatchDatasource } from '../../../datasource';
import { CloudWatchMetricsQuery } from '../../../types';

import { useCloudWatchPrometheusDatasource } from './cloudWatchPrometheusDatasourceShim';

export interface Props {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  datasource: CloudWatchDatasource;
  timeRange: TimeRange;
  app?: CoreApp;
}

export const PromQLCodeEditor = ({ query, onChange, onRunQuery, datasource, timeRange, app }: Props) => {
  const prometheusDatasourceShim = useCloudWatchPrometheusDatasource(datasource, query.region, timeRange);

  const promQuery: PromQuery = {
    refId: query.refId,
    expr: query.promqlExpression ?? '',
    format: query.format,
    instant: query.instant,
    range: query.range,
    interval: query.interval,
    legendFormat: query.legendFormat ?? '__auto',
  };

  const handleChange = (next: PromQuery) => {
    onChange({
      ...query,
      promqlExpression: next.expr,
      format: next.format,
      instant: next.instant,
      range: next.range,
      interval: next.interval,
      legendFormat: next.legendFormat,
    });
  };

  return (
    <Stack direction="column" gap={0.5}>
      <PromQueryField
        datasource={prometheusDatasourceShim}
        query={promQuery}
        onChange={handleChange}
        onRunQuery={onRunQuery}
        history={[]}
        range={timeRange}
        app={app}
      />
      <PromQueryBuilderOptions
        query={promQuery}
        app={app}
        onChange={handleChange}
        onRunQuery={onRunQuery}
        uiOptions={{ exemplars: false, disableTypeBoth: true }}
        formatOptions={[
          { label: 'Time series', value: 'time_series' },
          { label: 'Table', value: 'table' },
        ]}
      />
    </Stack>
  );
};
