import { ChangeEvent } from 'react';

import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { EditorField, EditorHeader, EditorRow, EditorSwitch, InlineSelect } from '@grafana/plugin-ui';
import { Alert, Input, Space } from '@grafana/ui';

import { CloudWatchDatasource } from '../../datasource';
import { DEFAULT_CWLI_QUERY_STRING } from '../../defaultQueries';
import { isCloudWatchAnnotationQuery, isLogsAnnotationQuery } from '../../guards';
import { useRegions } from '../../hooks';
import { CloudWatchAnnotationType, CloudWatchJsonData, CloudWatchQuery, LogsQueryLanguage, MetricStat } from '../../types';
import { LogsQLCodeEditor } from '../QueryEditor/LogsQueryEditor/code-editors/LogsQLCodeEditor';
import { LogGroupsFieldWrapper } from '../shared/LogGroups/LogGroupsField';
import { MetricStatEditor } from '../shared/MetricStatEditor/MetricStatEditor';

export type Props = QueryEditorProps<CloudWatchDatasource, CloudWatchQuery, CloudWatchJsonData>;

const annotationTypeOptions: Array<SelectableValue<CloudWatchAnnotationType>> = [
  { label: 'CloudWatch Metrics', value: 'metrics', description: 'Query CloudWatch alarms as annotations' },
  { label: 'CloudWatch Logs', value: 'logs', description: 'Query log events as annotations' },
];

// Dashboard Settings -> Annotations -> New Query
export const AnnotationQueryEditor = (props: Props) => {
  const { query, onChange, datasource } = props;
  const [regions, regionIsLoading] = useRegions(datasource);

  if (!isCloudWatchAnnotationQuery(query)) {
    return (
      <Alert severity="error" title="Invalid annotation query" topSpacing={2}>
        {JSON.stringify(query, null, 4)}
      </Alert>
    );
  }

  const isLogsMode = isLogsAnnotationQuery(query);
  // Default to metrics for backward compatibility
  const annotationType = query.annotationType || 'metrics';

  const onAnnotationTypeChange = (newType: CloudWatchAnnotationType) => {
    if (newType === 'logs') {
      onChange({
        ...query,
        annotationType: 'logs',
        // Initialize logs fields with defaults
        expression: query.expression || DEFAULT_CWLI_QUERY_STRING,
        logGroups: query.logGroups || [],
        queryLanguage: query.queryLanguage || LogsQueryLanguage.CWLI,
        // Clear metrics-specific fields
        namespace: undefined,
        metricName: undefined,
        dimensions: undefined,
        statistic: undefined,
        period: undefined,
        actionPrefix: undefined,
        alarmNamePrefix: undefined,
        prefixMatching: undefined,
      });
    } else {
      onChange({
        ...query,
        annotationType: 'metrics',
        // Clear logs-specific fields
        expression: undefined,
        logGroups: undefined,
        logGroupNames: undefined,
        queryLanguage: undefined,
      });
    }
  };

  return (
    <>
      <EditorHeader>
        <InlineSelect
          label="Region"
          value={regions.find((v) => v.value === query.region)}
          placeholder="Select region"
          allowCustomValue
          onChange={({ value: region }) => region && onChange({ ...query, region })}
          options={regions}
          isLoading={regionIsLoading}
        />
        <InlineSelect
          aria-label="Annotation type"
          label="Annotation type"
          value={annotationType}
          options={annotationTypeOptions}
          onChange={({ value }) => value && onAnnotationTypeChange(value)}
          inputId={`cloudwatch-annotation-type-${query.refId}`}
          id={`cloudwatch-annotation-type-${query.refId}`}
        />
      </EditorHeader>
      <Space v={0.5} />
      {isLogsMode ? (
        <>
          <LogGroupsFieldWrapper
            region={query.region}
            datasource={datasource}
            // eslint-disable-next-line @typescript-eslint/no-deprecated
            legacyLogGroupNames={query.logGroupNames}
            logGroups={query.logGroups}
            onChange={(logGroups) => {
              onChange({ ...query, logGroups, logGroupNames: undefined });
            }}
            //legacy props
            legacyOnChange={(logGroupNames) => {
              onChange({ ...query, logGroupNames });
            }}
          />
          <LogsQLCodeEditor query={query} datasource={datasource} onChange={onChange} />
        </>
      ) : (
        <>
          <MetricStatEditor
            {...props}
            refId={query.refId}
            metricStat={query as MetricStat}
            disableExpressions={true}
            onChange={(metricStat: MetricStat) => onChange({ ...query, ...metricStat })}
          ></MetricStatEditor>
          <Space v={0.5} />
          <EditorRow>
            <EditorField label="Period" width={26} tooltip="Minimum interval between points in seconds.">
              <Input
                value={query.period || ''}
                placeholder="auto"
                onChange={(event: ChangeEvent<HTMLInputElement>) => onChange({ ...query, period: event.target.value })}
              />
            </EditorField>
            <EditorField label="Enable Prefix Matching" optional={true}>
              <EditorSwitch
                value={query.prefixMatching}
                onChange={(e) => {
                  onChange({
                    ...query,
                    prefixMatching: e.currentTarget.checked,
                  });
                }}
              />
            </EditorField>
            <EditorField label="Action" optional={true} disabled={!query.prefixMatching}>
              <Input
                value={query.actionPrefix || ''}
                onChange={(event: ChangeEvent<HTMLInputElement>) =>
                  onChange({ ...query, actionPrefix: event.target.value })
                }
              />
            </EditorField>
            <EditorField label="Alarm Name" optional={true} disabled={!query.prefixMatching}>
              <Input
                value={query.alarmNamePrefix || ''}
                onChange={(event: ChangeEvent<HTMLInputElement>) =>
                  onChange({ ...query, alarmNamePrefix: event.target.value })
                }
              />
            </EditorField>
          </EditorRow>
        </>
      )}
    </>
  );
};
