import { useEffect, useMemo } from 'react';

import { CoreApp, TimeRange } from '@grafana/data';
import { PromQueryField } from '@grafana/prometheus';
import { type PrometheusDatasource } from '@grafana/prometheus/dist/types/datasource';
import { type PromQuery } from '@grafana/prometheus/dist/types/types';
import { Stack } from '@grafana/ui';

import { CloudWatchDatasource } from '../../../datasource';
import { CloudWatchMetricsQuery } from '../../../types';

import { CloudWatchPromQLLanguageProvider } from './CloudWatchPromQLLanguageProvider';
import { PromQLOptionsEditor } from './PromQLOptionsEditor';

export interface Props {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  datasource: CloudWatchDatasource;
  timeRange: TimeRange;
  app?: CoreApp;
}

export const PromQLCodeEditor = ({ query, onChange, onRunQuery, datasource, timeRange, app }: Props) => {
  const prometheusDatasourceShim = useMemo(() => {
    const shim = makePrometheusDatasourceShim(datasource);
    shim.languageProvider = new CloudWatchPromQLLanguageProvider(shim, datasource.resources, query.region);
    return shim;
  }, [datasource]);

  useEffect(() => {
    const languageProvider = prometheusDatasourceShim.languageProvider as CloudWatchPromQLLanguageProvider;
    languageProvider.updateRegion(query.region);
    languageProvider.start(timeRange);
  }, [prometheusDatasourceShim, query.region, timeRange]);

  const promQuery: PromQuery = {
    refId: query.refId,
    expr: query.promqlExpression ?? '',
  };

  const handleChange = (next: PromQuery) => {
    if (next.expr !== query.promqlExpression) {
      onChange({ ...query, promqlExpression: next.expr });
    }
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
      <PromQLOptionsEditor query={query} onChange={onChange} onRunQuery={onRunQuery} app={app} />
    </Stack>
  );
};

function makePrometheusDatasourceShim(datasource: CloudWatchDatasource): PrometheusDatasource {
  return {
    interpolateString: (value: string) => datasource.templateSrv.replace(value),
  } as PrometheusDatasource;
}
