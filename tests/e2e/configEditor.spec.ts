import { expect, test } from '@grafana/plugin-e2e';

import { type CloudWatchJsonData } from '../../src/types';

const PLUGIN_TYPE = 'cloudwatch';
const PROVISIONED_FILE = 'datasources.yml';

// GRAFANA_URL is set only by the Cloud cron workflow (playwright-cloud); its presence signals
// a run against the shared Cloud instance rather than local/PR CI.
const isCloudRun = !!process.env.GRAFANA_URL;

test.describe('Config editor', () => {
  test.describe('rendering', () => {
    test(
      'smoke: should render config editor',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
        const ds = await readProvisionedDataSource({ fileName: PROVISIONED_FILE });
        await createDataSourceConfigPage({ type: ds.type });

        await expect(page.getByRole('heading', { name: 'Connection Details', exact: true })).toBeVisible();
        await expect(page.getByText('Namespaces of Custom Metrics', { exact: true })).toBeVisible();
      }
    );

    test('should render CloudWatch Logs settings', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await expect(page.getByText('Cloudwatch Logs', { exact: true })).toBeVisible();
      await expect(page.getByLabel('Query Result Timeout')).toBeVisible();
      await expect(page.getByText('Default Log Groups', { exact: true })).toBeVisible();
    });
  });

  test.describe('provisioned datasource', () => {
    // The shared Cloud instance doesn't apply the local provisioning/datasources/datasources.yml,
    // so this assertion of the provisioned region can't run there (grafana/clickhouse-datasource#1934).
    test.beforeEach(() => {
      test.skip(isCloudRun, 'Provisioned-datasource assertions require local provisioning, not applied on Cloud.');
    });

    test('should load the provisioned default region', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONED_FILE });
      await gotoDataSourceConfigPage(ds.uid);

      const defaultRegion = ds.jsonData.defaultRegion;
      expect(defaultRegion).toBeTruthy();
      await expect(page.getByLabel('Default Region')).toBeVisible();
      await expect(page.getByText(defaultRegion!, { exact: true })).toBeVisible();
    });
  });

  test.describe('save & test', () => {
    test(
      'should pass health check for the provisioned datasource',
      { tag: '@aws' },
      async ({ readProvisionedDataSource, gotoDataSourceConfigPage }) => {
        test.skip(isCloudRun, 'Health-checks the locally-provisioned datasource, not applied on Cloud.');
        const ds = await readProvisionedDataSource({ fileName: PROVISIONED_FILE });
        const configPage = await gotoDataSourceConfigPage(ds.uid);

        const response = await configPage.saveAndTest();
        expect(response.ok()).toBe(true);
      }
    );

    test('should show an authentication error for invalid access and secret keys', async ({
      createDataSourceConfigPage,
      page,
    }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await page.getByRole('combobox', { name: 'Authentication Provider', exact: true }).click();
      await page.getByText('Access & secret key', { exact: true }).click();
      await page.getByLabel('Access Key ID').fill('fake-access-key');
      await page.getByLabel('Secret Access Key').fill('fake-secret-key');
      await page.getByLabel('Default Region').click();
      await page.getByText('us-east-2', { exact: true }).click();

      const response = await configPage.saveAndTest();
      expect(response.ok()).toBe(false);
      await expect(configPage).toHaveAlert('error');
    });

    // The health check with real injected credentials lives in configEditorCredentials.spec.ts,
    // which turns trace capture off at file scope.
  });
});
