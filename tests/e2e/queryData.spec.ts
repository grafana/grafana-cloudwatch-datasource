import { expect, test } from '@grafana/plugin-e2e';

test.describe('Query data in provisioned dashboard', () => {
  const DASHBOARD_UID = 'e2e-cloudwatch-data';
  test('should return the expected data for a cpu utilization query', { tag: '@aws' }, async ({ gotoPanelEditPage }) => {
    const panelEditPage = await gotoPanelEditPage({ dashboard:{ uid: DASHBOARD_UID }, id: '1', waitUntil: 'networkidle' });
    await panelEditPage.refreshPanel();

    await expect(panelEditPage.panel).toMatchDataSnapshot('metric-math');
  });

    test('should return the expected data for a logs metrics query', { tag: '@aws' }, async ({ gotoPanelEditPage }) => {
    const panelEditPage = await gotoPanelEditPage({ dashboard:{ uid: DASHBOARD_UID }, id: '2', waitUntil: 'networkidle' });
    await panelEditPage.refreshPanel();

    // the global config redacts instance ids; this test keeps only the grafana-logs prefix of each filter name and log group
    await expect(panelEditPage.panel).toMatchDataSnapshot('logs-metrics', {
      redact: [{ pattern: /grafana-logs\S*(?: \S+)?/g, replacement: 'grafana-logs-<redacted>' }],
    });
  });
});
