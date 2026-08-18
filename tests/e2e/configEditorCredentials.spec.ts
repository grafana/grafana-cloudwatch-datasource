import { expect, test } from '@grafana/plugin-e2e';
import { Page } from '@playwright/test';

const PLUGIN_TYPE = 'cloudwatch';

// This test holds real credentials, so it lives in its own file with trace capture off. A trace
// records the request body that carries them. Playwright rejects test.use({ trace }) inside a
// describe group, so file scope is the narrowest scope available.
test.use({ trace: 'off' });

// Selects a Private Data Source Connect network in the datasource config editor. The
// combobox is a Grafana-core element, present only when PDC is available on the instance
// (i.e. the shared Cloud instance), so it is called only when a network name is injected.
async function configurePDC(page: Page, networkName: string) {
  await page.getByRole('combobox', { name: 'Private data source connect' }).click();
  await page.getByText(networkName).click();
}

test.describe('Config editor', () => {
  test.describe('save & test', () => {
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
        test.skip(!accessKey || !secretKey || !region, 'Requires the injected AWS credentials (Cloud cron / CI Vault)');

        // Created through the API rather than typed into the editor, so the credentials stay out
        // of the DOM: core renders Secret Access Key as a plain input, not a password input. The
        // auth shape mirrors provisioning/datasources/datasources.yml.
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
