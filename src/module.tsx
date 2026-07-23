import { DashboardLoadedEvent, DataSourcePlugin } from '@grafana/data';
import { initPluginTranslations } from '@grafana/i18n';
import { loadResources } from '@grafana/prometheus';
import { getAppEvents } from '@grafana/runtime';

import LogsCheatSheet from './components/CheatSheet/LogsCheatSheet';
import { ConfigEditor } from './components/ConfigEditor/ConfigEditor';
import { MetaInspector } from './components/MetaInspector/MetaInspector';
import { QueryEditor } from './components/QueryEditor/QueryEditor';
import { CloudWatchDatasource } from './datasource';
import pluginJson from './plugin.json';
import { onDashboardLoadedHandler } from './tracking';
import { CloudWatchJsonData, CloudWatchQuery } from './types';

// Initialize translations for the bundled @grafana/prometheus components (PromQL query
// mode). This sets up the bundled @grafana/i18n instance that those components' t()/Trans
// calls rely on; without it they throw in dev builds.
if (process.env.NODE_ENV !== 'test') {
  void initPluginTranslations(pluginJson.id, [loadResources]);
}

export const plugin = new DataSourcePlugin<CloudWatchDatasource, CloudWatchQuery, CloudWatchJsonData>(
  CloudWatchDatasource
)
  .setQueryEditorHelp(LogsCheatSheet)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor)
  .setMetadataInspector(MetaInspector);

// Subscribe to on dashboard loaded event so that we can track plugin adoption
getAppEvents().subscribe<DashboardLoadedEvent<CloudWatchQuery>>(DashboardLoadedEvent, onDashboardLoadedHandler);
