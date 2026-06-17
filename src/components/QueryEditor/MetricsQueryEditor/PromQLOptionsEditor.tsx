import { FormEvent } from 'react';

import { CoreApp, SelectableValue } from '@grafana/data';
import { EditorField, QueryOptionGroup } from '@grafana/plugin-ui';
import { AutoSizeInput, Box, RadioButtonGroup, Select } from '@grafana/ui';

import { CloudWatchMetricsQuery } from '../../../types';

export interface Props {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  onRunQuery: () => void;
  app?: CoreApp;
}

type PromQLQueryType = 'range' | 'instant' | 'both';
type PromQLFormat = 'time_series' | 'table';

const baseOptions: Array<SelectableValue<PromQLQueryType>> = [
  { label: 'Range', value: 'range', description: 'Run query over a range of time' },
  { label: 'Instant', value: 'instant', description: 'Run query against a single point in time' },
];

const bothOption: SelectableValue<PromQLQueryType> = {
  label: 'Both',
  value: 'both',
  description: 'Run both instant and range queries',
};

const formatOptions: Array<SelectableValue<PromQLFormat>> = [
  { label: 'Time series', value: 'time_series' },
  { label: 'Table', value: 'table' },
];

function getQueryType(query: CloudWatchMetricsQuery): PromQLQueryType {
  if (query.instant && query.range) {
    return 'both';
  }
  if (query.instant) {
    return 'instant';
  }
  return 'range';
}

function applyQueryType(query: CloudWatchMetricsQuery, queryType: PromQLQueryType): CloudWatchMetricsQuery {
  switch (queryType) {
    case 'instant':
      return { ...query, instant: true, range: false };
    case 'both':
      return { ...query, instant: true, range: true };
    case 'range':
    default:
      return { ...query, instant: false, range: true };
  }
}

export const PromQLOptionsEditor = ({ query, onChange, onRunQuery, app }: Props) => {
  const queryType = getQueryType(query);
  const canRenderBoth = app !== CoreApp.UnifiedAlerting;
  const queryTypeOptions = canRenderBoth ? [...baseOptions, bothOption] : baseOptions;

  const onQueryTypeChange = (next: PromQLQueryType) => {
    onChange(applyQueryType(query, next));
    onRunQuery();
  };

  const onMinStepChange = (event: FormEvent<HTMLInputElement>) => {
    onChange({ ...query, interval: event.currentTarget.value.trim() });
    onRunQuery();
  };

  const format: PromQLFormat = query.format ?? 'time_series';
  const formatOption = formatOptions.find((o) => o.value === format) ?? formatOptions[0];

  const onFormatChange = (next: SelectableValue<PromQLFormat>) => {
    onChange({ ...query, format: next.value });
    onRunQuery();
  };

  return (
    <Box backgroundColor="secondary" borderRadius="default" marginTop={0.5}>
      <QueryOptionGroup
        title="Options"
        collapsedInfo={[
          `Min step: ${query.interval || 'auto'}`,
          `Format: ${formatOption.label}`,
          `Type: ${queryType}`,
        ]}
      >
        <EditorField
          label="Min step"
          tooltip="An additional lower bound for the step parameter of range queries. Accepts duration strings like '10s' or '1m'. Empty means auto."
        >
          <AutoSizeInput
            type="text"
            placeholder="auto"
            minWidth={10}
            onCommitChange={onMinStepChange}
            defaultValue={query.interval}
          />
        </EditorField>
        <EditorField label="Format">
          <Select options={formatOptions} value={formatOption} onChange={onFormatChange} width={20} />
        </EditorField>
        <EditorField label="Type">
          <RadioButtonGroup options={queryTypeOptions} value={queryType} onChange={onQueryTypeChange} />
        </EditorField>
      </QueryOptionGroup>
    </Box>
  );
};
