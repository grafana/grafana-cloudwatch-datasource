import { useEffect, useMemo } from 'react';

import { TimeRange } from '@grafana/data';
import { MonacoQueryFieldLazy } from '@grafana/prometheus';
import { type PrometheusDatasource } from '@grafana/prometheus/dist/types/datasource';

import { CloudWatchDatasource } from '../../../datasource';
import { CloudWatchMetricsQuery } from '../../../types';

import { CloudWatchPromQLLanguageProvider } from './CloudWatchPromQLLanguageProvider';

export interface Props {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  datasource: CloudWatchDatasource;
  timeRange: TimeRange;
}

export const PromQLCodeEditor = ({ query, onChange, onRunQuery, datasource, timeRange }: Props) => {
  const prometheusDatasourceShim = useMemo(() => {
    return makePrometheusDatasourceShim(datasource);
  }, [datasource]);

  const languageProvider = useMemo(() => {
    return new CloudWatchPromQLLanguageProvider(prometheusDatasourceShim, datasource.resources, query.region);
  }, [prometheusDatasourceShim, datasource.resources]);

  useEffect(() => {
    languageProvider.updateRegion(query.region);
    languageProvider.start(timeRange);
  }, [languageProvider, query.region]);

  return (
    <MonacoQueryFieldLazy
      initialValue={query.promqlExpression ?? ''}
      languageProvider={languageProvider}
      history={[]}
      placeholder="Enter a PromQL expression"
      onRunQuery={(value) => {
        onChange({ ...query, promqlExpression: value });
        onRunQuery();
      }}
      onBlur={(value) => {
        if (value !== query.promqlExpression) {
          onChange({ ...query, promqlExpression: value });
        }
      }}
      datasource={prometheusDatasourceShim}
      timeRange={timeRange}
    />
  );
};

function makePrometheusDatasourceShim(datasource: CloudWatchDatasource): PrometheusDatasource {
  return {
    interpolateString: (value: string) => datasource.templateSrv.replace(value),
  } as PrometheusDatasource;
}
