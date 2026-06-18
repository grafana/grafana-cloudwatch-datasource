import { useEffect, useMemo } from 'react';

import { CoreApp, TimeRange } from '@grafana/data';
import { PromQueryBuilderOptions, PromQueryField } from '@grafana/prometheus';
import { type PrometheusDatasource } from '@grafana/prometheus/dist/types/datasource';
import { type PromQuery } from '@grafana/prometheus/dist/types/types';
import { Stack } from '@grafana/ui';

import { CloudWatchDatasource } from '../../../datasource';
import { CloudWatchMetricsQuery } from '../../../types';

import { CloudWatchPromQLLanguageProvider } from './CloudWatchPromQLLanguageProvider';

export interface Props {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  datasource: CloudWatchDatasource;
  timeRange: TimeRange;
  app?: CoreApp;
}

export const PromQLCodeEditor = ({ query, onChange, onRunQuery, datasource, timeRange, app }: Props) => {
  // query.region seeds the language provider once; subsequent region changes flow through
  // languageProvider.updateRegion() in the effect below. Rebuilding the shim on every region
  // change would throw away the labelKeys/labelValues caches.
  /* eslint-disable react-hooks/exhaustive-deps */
  const prometheusDatasourceShim = useMemo(() => {
    const shim = makePrometheusDatasourceShim(datasource);
    shim.languageProvider = new CloudWatchPromQLLanguageProvider(shim, datasource.resources, query.region);
    return shim;
  }, [datasource]);
  /* eslint-enable react-hooks/exhaustive-deps */

  useEffect(() => {
    const languageProvider = prometheusDatasourceShim.languageProvider as CloudWatchPromQLLanguageProvider;
    languageProvider.updateRegion(query.region);
    languageProvider.start(timeRange);
  }, [prometheusDatasourceShim, query.region, timeRange]);

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
        uiOptions={{ exemplars: false }}
        formatOptions={[
          { label: 'Time series', value: 'time_series' },
          { label: 'Table', value: 'table' },
        ]}
      />
    </Stack>
  );
};

function makePrometheusDatasourceShim(datasource: CloudWatchDatasource): PrometheusDatasource {
  return {
    interpolateString: (value: string) => datasource.templateSrv.replace(value),
  } as PrometheusDatasource;
}
