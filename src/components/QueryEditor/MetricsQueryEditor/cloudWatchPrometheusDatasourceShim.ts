import { useEffect, useMemo } from 'react';

import { type ScopedVars, type TimeRange } from '@grafana/data';
import { PrometheusCacheLevel } from '@grafana/prometheus';
import { type PrometheusDatasource } from '@grafana/prometheus/dist/types/datasource';
import { type PromQuery } from '@grafana/prometheus/dist/types/types';

import { type CloudWatchDatasource } from '../../../datasource';

import { CloudWatchPromQLLanguageProvider } from './CloudWatchPromQLLanguageProvider';

/**
 * Minimal stand-in for PrometheusDatasource covering the surface area touched
 * by @grafana/prometheus's query field, options panel, and visual query
 * builder. CloudWatch doesn't extend PrometheusDatasource; this shim lets us
 * reuse the upstream UI without a hard dependency.
 */
export function makeCloudWatchPrometheusDatasourceShim(datasource: CloudWatchDatasource): PrometheusDatasource {
  return {
    interpolateString: (value: string, scopedVars?: ScopedVars) => datasource.templateSrv.replace(value, scopedVars),
    getVariables: () => datasource.getVariables(),
    lookupsDisabled: false,
    // CloudWatchPromQLLanguageProvider has its own 10-minute cache; the upstream
    // cacheLevel is only read for label-value autocomplete debounce timing.
    cacheLevel: PrometheusCacheLevel.Low,
    // The visual builder's hint panel calls getQueryHints on every render and
    // modifyQuery on hint button clicks. CloudWatch-aware implementations are
    // a follow-up; for now we return no hints so the builder mounts cleanly.
    getQueryHints: () => [],
    modifyQuery: (query: PromQuery) => query,
  } as unknown as PrometheusDatasource;
}

/**
 * Builds the shim once per datasource and attaches a CloudWatchPromQLLanguageProvider.
 * Region and time-range changes flow through updateRegion / start() in a useeffect rather
 * than rebuilding the shim, so the language provider's label key/value caches survive.
 */
export function useCloudWatchPrometheusDatasource(
  datasource: CloudWatchDatasource,
  region: string,
  timeRange: TimeRange
): PrometheusDatasource {
  const shim = useMemo(() => {
    const built = makeCloudWatchPrometheusDatasourceShim(datasource);
    built.languageProvider = new CloudWatchPromQLLanguageProvider(built, datasource.resources, region);
    return built;

    // region intentionally omitted from deps; handled by updateRegion in the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [datasource]);

  useEffect(() => {
    const languageProvider = shim.languageProvider as CloudWatchPromQLLanguageProvider;
    languageProvider.updateRegion(region);
    languageProvider.start(timeRange);
  }, [shim, region, timeRange]);

  return shim;
}
