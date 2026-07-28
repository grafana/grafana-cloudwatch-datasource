import { test, expect } from '@grafana/plugin-e2e';

// Smoke test: the query editor must mount and render in Explore. Guards against
// build/runtime regressions in the bundled plugin that unit tests can't see.
test('explore query editor renders', async ({ explorePage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await explorePage.datasource.set(ds.name);

  // The query header (Region field + Query mode selector) only renders if the
  // editor mounted successfully.
  await expect(page.getByLabel('Query mode')).toBeVisible();
  await expect(page.getByText('Region')).toBeVisible();
});
