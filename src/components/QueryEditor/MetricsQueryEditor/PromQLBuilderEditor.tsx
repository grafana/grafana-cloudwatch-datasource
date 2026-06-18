import { CoreApp, TimeRange } from '@grafana/data';
import { PromQueryBuilderContainer, PromQueryBuilderOptions } from '@grafana/prometheus';
import { type PromQuery } from '@grafana/prometheus/dist/types/types';
import { Stack } from '@grafana/ui';

import { CloudWatchDatasource } from '../../../datasource';
import { CloudWatchMetricsQuery } from '../../../types';

import { useCloudWatchPrometheusDatasource } from './cloudWatchPrometheusDatasourceShim';

export interface PromQLBuilderEditorProps {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  datasource: CloudWatchDatasource;
  timeRange: TimeRange;
  app?: CoreApp;
  showExplain: boolean;
}

export const PromQLBuilderEditor = ({
  query,
  onChange,
  onRunQuery,
  datasource,
  timeRange,
  app,
  showExplain,
}: PromQLBuilderEditorProps) => {
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
      <PromQueryBuilderContainer
        query={promQuery}
        datasource={prometheusDatasourceShim}
        onChange={handleChange}
        onRunQuery={onRunQuery}
        showExplain={showExplain}
      />
      <PromQueryBuilderOptions query={promQuery} app={app} onChange={handleChange} onRunQuery={onRunQuery} />
    </Stack>
  );
};
