import { CoreApp } from '@grafana/data';
import { PromQueryBuilderOptions } from '@grafana/prometheus';
import { type PromQuery } from '@grafana/prometheus/dist/types/types';

import { CloudWatchMetricsQuery } from '../../../types';

export interface Props {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  app?: CoreApp;
}

export const PromQLOptionsEditor = ({ query, onChange, onRunQuery, app }: Props) => {
  const promQuery = toPromQuery(query);

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

  return <PromQueryBuilderOptions query={promQuery} app={app} onChange={handleChange} onRunQuery={onRunQuery} />;
};

function toPromQuery(query: CloudWatchMetricsQuery): PromQuery {
  return {
    refId: query.refId,
    expr: query.promqlExpression ?? '',
    format: query.format,
    instant: query.instant,
    range: query.range,
    interval: query.interval,
    legendFormat: query.legendFormat,
  };
}
