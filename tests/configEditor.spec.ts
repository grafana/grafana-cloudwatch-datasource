import { expect, test } from '@grafana/plugin-e2e';

import { CloudWatchJsonData } from '../src/types';

const PROVISIONING_FILE = 'datasources.yml';

test.describe('Config editor', () => {
  test.describe.configure({ mode: 'serial' });

  test.describe('rendering', () => {
    test(
      'smoke: should render config editor',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, page }) => {
        await createDataSourceConfigPage({ type: 'cloudwatch' });
        await expect(page.getByRole('heading', { name: 'Connection Details' })).toBeVisible();
      }
    );

    test('should render Connection Details section', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: 'cloudwatch' });
      await expect(page.getByRole('heading', { name: 'Connection Details', exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Authentication', exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Assume Role', exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Additional Settings', exact: true })).toBeVisible();
      await expect(page.getByText('Authentication Provider').first()).toBeVisible();
      await expect(page.getByText('Default Region').first()).toBeVisible();
      await expect(page.getByText('Endpoint').first()).toBeVisible();
    });

    test('should render Cloudwatch Logs section', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: 'cloudwatch' });
      await expect(page.getByRole('heading', { name: 'Cloudwatch Logs', exact: true })).toBeVisible();
      await expect(page.getByText('Query Result Timeout').first()).toBeVisible();
      await expect(page.getByText('Default Log Groups').first()).toBeVisible();
    });

    test('should render X-ray trace link section', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: 'cloudwatch' });
      await expect(page.getByRole('heading', { name: 'X-ray trace link', exact: true })).toBeVisible();
    });
  });

  test.describe('provisioned datasource', () => {
    test('should load provisioned authentication settings', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await gotoDataSourceConfigPage(ds.uid);
      await expect(page.getByText('Access & secret key')).toBeVisible();
      await expect(page.getByRole('textbox', { name: /Access Key ID/ })).toBeDisabled();
      await expect(page.getByRole('textbox', { name: /Secret Access Key|Configured/ }).first()).toBeDisabled();
    });

    test('should load provisioned endpoint and region', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await gotoDataSourceConfigPage(ds.uid);
      await expect(
        page.getByRole('textbox', { name: /Endpoint/ })
      ).toHaveValue(ds.jsonData.endpoint ?? '');
      await expect(page.getByText(ds.jsonData.defaultRegion ?? '').first()).toBeVisible();
    });
  });

  test.describe('save & test', () => {
    test('should pass health check for provisioned datasource', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await gotoDataSourceConfigPage(ds.uid);
      await page.getByRole('button', { name: /^(Save & test|Test)$/ }).click();
      await expect(
        page.getByText('Successfully queried the CloudWatch metrics API')
      ).toBeVisible({ timeout: 15_000 });
      await expect(page.getByText('Successfully queried the CloudWatch logs API')).toBeVisible();
    });

    test('should show error alert when backend is unreachable', async ({
      createDataSourceConfigPage,
      page,
    }) => {
      const configPage = await createDataSourceConfigPage({ type: 'cloudwatch' });
      // Point the datasource at an unreachable endpoint so CheckHealth fails at
      // the network layer.
      await page
        .getByRole('textbox', { name: /Endpoint/ })
        .fill('http://unreachable.invalid:4566');
      await configPage.saveAndTest();
      await expect(page.getByRole('alert')).toBeVisible({ timeout: 15_000 });
    });
  });
});
