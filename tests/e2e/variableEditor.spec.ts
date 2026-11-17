import { expect, test } from '@grafana/plugin-e2e';

const PROVISIONED_FILE = 'datasources.yml';

// GRAFANA_URL is set only by the Cloud cron workflow (playwright-cloud); its presence signals
// a run against the shared Cloud instance rather than local/PR CI.
const isCloudRun = !!process.env.GRAFANA_URL;

test.describe('Variable query editor', () => {
  // The shared Cloud instance doesn't apply the local provisioning, so the provisioned
  // datasource name matches nothing there.
  test.beforeEach(() => {
    test.skip(isCloudRun, 'Uses the locally provisioned datasource, not applied on Cloud.');
  });

  test('should offer the Accounts query type', async ({ variableEditPage, readProvisionedDataSource, page }) => {
    const ds = await readProvisionedDataSource({ fileName: PROVISIONED_FILE });
    await variableEditPage.setVariableType('Query');
    await variableEditPage.datasource.set(ds.name);

    const queryType = page.getByLabel('Query type', { exact: true });
    await expect(queryType).toBeVisible();
    await queryType.click();

    await expect(page.getByRole('option', { name: 'Log Groups', exact: true })).toBeVisible();
    await expect(page.getByRole('option', { name: 'Accounts', exact: true })).toBeVisible();
  });
});
