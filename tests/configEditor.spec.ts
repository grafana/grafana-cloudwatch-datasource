import { test, expect } from '@grafana/plugin-e2e';

// Smoke test: the datasource config editor must actually mount and render.
// Unit tests run in jsdom, so they don't catch build/runtime regressions that
// only surface in a real browser against the bundled plugin.
test('datasource config editor renders', async ({ createDataSourceConfigPage, page }) => {
  await createDataSourceConfigPage({ type: 'cloudwatch' });

  // A CloudWatch-specific field only appears if the editor rendered successfully
  // (a crash would surface an error boundary instead).
  await expect(page.getByText('Namespaces of Custom Metrics')).toBeVisible();
});
