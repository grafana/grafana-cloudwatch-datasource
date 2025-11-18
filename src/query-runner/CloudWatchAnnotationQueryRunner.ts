import { Observable } from 'rxjs';

import { DataQueryRequest, DataQueryResponse, DataSourceInstanceSettings } from '@grafana/data';
import { TemplateSrv } from '@grafana/runtime';

import { isLogsAnnotationQuery } from '../guards';
import { CloudWatchAnnotationQuery, CloudWatchJsonData, CloudWatchQuery } from '../types';

import { CloudWatchRequest } from './CloudWatchRequest';

// This class handles execution of CloudWatch annotation queries
export class CloudWatchAnnotationQueryRunner extends CloudWatchRequest {
  constructor(instanceSettings: DataSourceInstanceSettings<CloudWatchJsonData>, templateSrv: TemplateSrv) {
    super(instanceSettings, templateSrv);
  }

  handleAnnotationQuery(
    queries: CloudWatchAnnotationQuery[],
    options: DataQueryRequest<CloudWatchQuery>,
    queryFn: (request: DataQueryRequest<CloudWatchQuery>) => Observable<DataQueryResponse>
  ): Observable<DataQueryResponse> {
    return queryFn({
      ...options,
      targets: queries.map((query) => {
        const baseQuery = {
          ...query,
          region: this.templateSrv.replace(this.getActualRegion(query.region)),
          type: 'annotationQuery',
          datasource: this.ref,
        };

        // Handle logs annotations
        if (isLogsAnnotationQuery(query)) {
          return {
            ...baseQuery,
            expression: this.templateSrv.replace(query.expression ?? ''),
            logGroups: query.logGroups || [],
            logGroupNames: query.logGroupNames || [],
            queryLanguage: query.queryLanguage,
          };
        }

        // Handle metrics annotations (default for backward compatibility)
        return {
          ...baseQuery,
          statistic: this.templateSrv.replace(query.statistic),
          namespace: this.templateSrv.replace(query.namespace),
          metricName: this.templateSrv.replace(query.metricName),
          dimensions: this.convertDimensionFormat(query.dimensions ?? {}, {}),
          period: query.period ?? '',
          actionPrefix: query.actionPrefix ?? '',
          alarmNamePrefix: query.alarmNamePrefix ?? '',
        };
      }),
    });
  }
}
