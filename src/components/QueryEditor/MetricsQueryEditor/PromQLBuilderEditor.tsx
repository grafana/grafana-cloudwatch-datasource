import { Space } from '@grafana/ui';

import { CloudWatchDatasource } from '../../../datasource';
import { CloudWatchMetricsQuery } from '../../../types';

export interface PromQLBuilderEditorProps {
  query: CloudWatchMetricsQuery;
  onChange: (query: CloudWatchMetricsQuery) => void;
  datasource: CloudWatchDatasource;
}

export const PromQLBuilderEditor = (_props: PromQLBuilderEditorProps) => {
  return (
    <>
      <Space v={0.5} />
      <div>builder coming soon.</div>
    </>
  );
};
