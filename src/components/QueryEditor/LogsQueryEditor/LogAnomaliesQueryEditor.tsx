import { EditorField, EditorRow } from '@grafana/plugin-ui';
import { Combobox, Input } from '@grafana/ui';
import { CloudWatchLogsAnomaliesQuery } from 'dataquery';


interface Props {
  query: CloudWatchLogsAnomaliesQuery;
  onChange: (value: CloudWatchLogsAnomaliesQuery) => void;
}

const supressionStateOptions = [
  { label: 'All', value: 'all' },
  { label: 'Suppressed', value: 'suppressed' },
  { label: 'Unsuppressed', value: 'unsuppressed' },
];

export const LogAnomaliesQueryEditor = (props: Props) => {
  return (
    <>
      <EditorRow>
        <EditorField label="Anomaly Detection ARN">
          <Input
            value={props.query.anomalyDetectionARN || ''}
            onChange={(e) => {
              props.onChange({ ...props.query, anomalyDetectionARN: e.currentTarget.value });
            }}
          />
        </EditorField>
        <EditorField label="Supression state">
          <Combobox
            value={props.query.suppressionState ?? 'all'}
            options={supressionStateOptions}
            onChange={(e) => {
              props.onChange({ ...props.query, suppressionState: e.value });
            }}
          />
        </EditorField>
      </EditorRow>
    </>
  );
};
