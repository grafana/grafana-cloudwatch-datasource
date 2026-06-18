import { type TimeRange, dateTime } from '@grafana/data';
import { type PrometheusLanguageProviderInterface } from '@grafana/prometheus';
import { type PrometheusDatasource } from '@grafana/prometheus/dist/types/datasource';
import { type PromMetricsMetadata } from '@grafana/prometheus/dist/types/types';
import { type ResourcesAPI } from '../../../resources/ResourcesAPI';

export class CloudWatchPromQLLanguageProvider implements PrometheusLanguageProviderInterface {
  datasource: PrometheusDatasource;
  resources: ResourcesAPI;
  region: string;

  private latestLabelKeys: string[] = [];
  private latestMetrics: string[] = [];

  private labelKeysCache: Map<string, string[]> = new Map();
  private labelValuesCache: Map<string, Map<string, string[]>> = new Map();

  constructor(datasource: PrometheusDatasource, resources: ResourcesAPI, region: string) {
    this.datasource = datasource;
    this.resources = resources;
    this.region = region;
  }

  updateRegion = (newRegion: string) => {
    this.region = newRegion;
    this.latestLabelKeys = [];
    this.latestMetrics = [];
    this.labelKeysCache = new Map();
    this.labelValuesCache = new Map();
  };

  /**
   * Initializes the language provider by fetching metrics and label keys.
   * All calls use the limit parameter from datasource configuration (default: 40,000 if not set).
   * When no timeRange provided, we will use the default time range (now/now-6h)
   */
  start = async (timeRange?: TimeRange): Promise<unknown[]> => {
    const range: TimeRange = timeRange ?? {
      from: dateTime().subtract(6, 'hours'),
      to: dateTime(),
      raw: { from: 'now-6h', to: 'now' },
    };

    await Promise.all([this.queryLabelKeys(range), this.queryLabelValues(range, '__name__')]);
    return [];
  };

  /**
   * Returns already cached list of available label keys.
   * If there are no cached label keys, it returns an empty array.
   */
  retrieveLabelKeys = (): string[] => {
    return this.latestLabelKeys;
  };

  /**
   * Returns already cached list of all available metric names.
   * If there are no cached metrics, it returns an empty array.
   */
  retrieveMetrics = (): string[] => {
    return this.latestMetrics;
  };

  /**
   * Fetches all available label keys that match the specified criteria.
   * This method queries Prometheus for label keys within the specified time range.
   * The results can be filtered using the match parameter and limited in size.
   */
  queryLabelKeys = async (timeRange: TimeRange, match?: string, limit?: number): Promise<string[]> => {
    const start = snapToTenMinutes(timeRange.from.unix());
    const end = snapToTenMinutes(timeRange.to.unix());
    const key = cacheKey(start, end, match, limit);

    const cached = this.labelKeysCache.get(key);
    if (cached) {
      return cached;
    }

    const keys = await this.resources.getPromQLLabelKeys(this.region, match, start, end, limit).catch(() => []);
    this.labelKeysCache.set(key, keys);
    this.latestLabelKeys = keys;
    return keys;
  };

  /**
   * Fetches all values for a specific label key that match the specified criteria.
   * This method queries Prometheus for label values within the specified time range.
   * Results can be filtered using the match parameter to find values in specific contexts.
   */
  queryLabelValues = async (
    timeRange: TimeRange,
    labelKey: string,
    match?: string,
    limit?: number
  ): Promise<string[]> => {
    const start = snapToTenMinutes(timeRange.from.unix());
    const end = snapToTenMinutes(timeRange.to.unix());
    const key = cacheKey(start, end, match, limit);

    let cacheForLabelKey = this.labelValuesCache.get(labelKey);
    if (cacheForLabelKey) {
      const cached = cacheForLabelKey.get(key);
      if (cached) {
      return cached;
    }
    } else {
      cacheForLabelKey = new Map();
      this.labelValuesCache.set(labelKey, cacheForLabelKey);
    }

    const values = await this.resources
      .getPromQLLabelValues(this.region, labelKey, match, start, end, limit)
      .catch(() => []);
    cacheForLabelKey.set(key, values);

    if (labelKey === '__name__') {
      this.latestMetrics = values;
    }

    return values;
  };

  request = async (_url: string, _params?: unknown, _options?: unknown): Promise<unknown> => undefined;

  /**
   * CloudWatch does not support the Prometheus /api/v1/metadata endpoint
   * Completions still work; metric type and help text will be empty in the dropdown
   */
  retrieveMetricsMetadata = (): PromMetricsMetadata => ({});

  /**
   * CloudWatch does not support the Prometheus /api/v1/metadata endpoint
   * Completions still work; metric type and help text will be empty in the dropdown
   */
  queryMetricsMetadata = async (_limit?: number): Promise<PromMetricsMetadata> => ({});

  retrieveHistogramMetrics = (): string[] => [];

  fetchSuggestions = async (
    _timeRange?: unknown,
    _queries?: unknown,
    _scopes?: unknown,
    _adhocFilters?: unknown,
    _labelName?: string,
    _limit?: number,
    _requestId?: string
  ): Promise<string[]> => [];
}

function snapToTenMinutes(unixSecs: number): number {
  const TEN_MINUTES = 600;
  return Math.floor(unixSecs / TEN_MINUTES) * TEN_MINUTES;
}

function cacheKey(snappedStart: number, snappedEnd: number, match?: string, limit?: number): string {
  return `${snappedStart}:${snappedEnd}:${match ?? ''}:${limit ?? ''}`;
}
