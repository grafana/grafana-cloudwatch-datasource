import { expect, test } from '@grafana/plugin-e2e';
import { Page } from '@playwright/test';

import { type CloudWatchJsonData } from '../../src/types';

const PLUGIN_TYPE = 'cloudwatch';
const PROVISIONED_FILE = 'datasources.yml';

// GRAFANA_URL is set only by the Cloud cron workflow (playwright-cloud); its presence signals
// a run against the shared Cloud instance rather than local/PR CI.
const isCloudRun = !!process.env.GRAFANA_URL;

// Selects a Private Data Source Connect network in the datasource config editor. The
// combobox is a Grafana-core element, present only when PDC is available on the instance
// (i.e. the shared Cloud instance), so it is called only when a network name is injected.
async function configurePDC(page: Page, networkName: string) {
  await page.getByRole('combobox', { name: 'Private data source connect' }).click();
  await page.getByText(networkName).click();
}

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

    // Trace capture is off here because a trace records the request body that carries the
    // injected credentials. They also stay out of the DOM, because the datasource is created
    // through the API: core renders Secret Access Key as a plain input, not a password input.
    test.describe('injected credentials', () => {
      test.use({ trace: 'off' });

      test(
        'valid injected credentials should pass the health check',
        { tag: '@aws' },
        async ({ createDataSource, gotoDataSourceConfigPage, page }) => {
          // Consumes the AWS test-account credentials injected by the Cloud cron workflow
          // (playwright-cloud) from the data-sources Vault mount as AWS_ACCESS_KEY_ID /
          // AWS_SECRET_ACCESS_KEY / AWS_DEFAULT_REGION. Skipped when they are absent (fork PRs,
          // or local runs with no Vault access).
          const accessKey = process.env.AWS_ACCESS_KEY_ID;
          const secretKey = process.env.AWS_SECRET_ACCESS_KEY;
          const region = process.env.AWS_DEFAULT_REGION;
          test.skip(
            !accessKey || !secretKey || !region,
            'Requires the injected AWS credentials (Cloud cron / CI Vault)'
          );

          // Mirrors provisioning/datasources/datasources.yml, which is the same auth shape.
          const ds = await createDataSource({
            type: PLUGIN_TYPE,
            jsonData: { authType: 'keys', defaultRegion: region ?? '' },
            secureJsonData: { accessKey: accessKey ?? '', secretKey: secretKey ?? '' },
          });
          const configPage = await gotoDataSourceConfigPage(ds.uid);

          if (process.env.DS_PDC_NETWORK_NAME) {
            await configurePDC(page, process.env.DS_PDC_NETWORK_NAME);
          }

          const response = await configPage.saveAndTest();
          expect(response.ok()).toBe(true);
        }
      );
    });
  });
});
