import { expect, test, type ExplorePage } from '@grafana/plugin-e2e';
import type { Page } from '@playwright/test';

import { CloudWatchJsonData } from '../src/types';

const PROVISIONING_FILE = 'datasources.yml';

// Window matches tests/e2e/fixtures/seed-metrics.sh and logs.json.
const FIXTURE_FROM_ISO = '2026-04-21T00:00:00.000Z';
const FIXTURE_TO_ISO = '2026-04-21T04:00:00.000Z';

const NAMESPACE = 'E2E/Demo';
const METRIC = 'RequestCount';
const LOG_GROUP = '/e2e/app';

function exploreUrl(uid: string): string {
  const panes = JSON.stringify({
    a: {
      datasource: uid,
      queries: [{ refId: 'A', datasource: { type: 'cloudwatch', uid } }],
      range: { from: FIXTURE_FROM_ISO, to: FIXTURE_TO_ISO },
    },
  });
  return `/explore?orgId=1&schemaVersion=1&panes=${encodeURIComponent(panes)}`;
}

async function selectQueryMode(page: Page, mode: 'CloudWatch Metrics' | 'CloudWatch Logs') {
  await page.getByRole('combobox', { name: 'Query mode' }).click();
  await page.getByRole('option', { name: mode, exact: true }).click();
  await expect(page.getByText(mode).first()).toBeVisible();
}

async function readQueryResponseBody(explorePage: ExplorePage) {
  let body: any = null;
  const responsePromise = explorePage.waitForQueryDataResponse(async (r) => {
    if (!r.ok()) return false;
    const b: any = await r.json().catch(() => null);
    if (!Array.isArray(b?.results?.A?.frames)) return false;
    body = b;
    return true;
  });
  return { responsePromise, getBody: () => body };
}

test.describe('Query editor', () => {
  test.describe('rendering', () => {
    test(
      'smoke: renders query mode options',
      { tag: '@plugins' },
      async ({ readProvisionedDataSource, page }) => {
        const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
        await page.goto(exploreUrl(ds.uid));
        await page.getByRole('combobox', { name: 'Query mode' }).click();
        await expect(page.getByRole('option', { name: 'CloudWatch Metrics', exact: true })).toBeVisible();
        await expect(page.getByRole('option', { name: 'CloudWatch Logs', exact: true })).toBeVisible();
      }
    );

    test('renders Region selector across modes', async ({ readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await page.goto(exploreUrl(ds.uid));
      await expect(page.getByRole('combobox', { name: /Region/ })).toBeVisible();
    });
  });

  test.describe('CloudWatch Metrics', () => {
    test('renders Builder/Code radios and Namespace/Metric/Statistic fields', async ({
      readProvisionedDataSource,
      page,
    }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await page.goto(exploreUrl(ds.uid));
      await selectQueryMode(page, 'CloudWatch Metrics');
      await expect(page.getByRole('radio', { name: 'Builder' })).toBeChecked();
      await expect(page.getByRole('radio', { name: 'Code' })).toBeVisible();
      await expect(page.getByRole('combobox', { name: 'Namespace' })).toBeVisible();
      await expect(page.getByRole('combobox', { name: 'Metric name' })).toBeVisible();
      await expect(page.getByRole('combobox', { name: 'Statistic' })).toBeVisible();
    });

    test('Code mode shows the query editor', async ({ readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await page.goto(exploreUrl(ds.uid));
      await selectQueryMode(page, 'CloudWatch Metrics');
      await page.getByRole('radio', { name: 'Code' }).click();
      await expect(page.getByRole('radio', { name: 'Code' })).toBeChecked();
    });
  });

  test.describe('CloudWatch Logs', () => {
    test('renders log group selector and Insights editor', async ({
      readProvisionedDataSource,
      page,
    }) => {
      const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
      await page.goto(exploreUrl(ds.uid));
      await selectQueryMode(page, 'CloudWatch Logs');
      await expect(page.getByRole('button', { name: 'Select log groups' })).toBeVisible();
      await expect(page.getByRole('combobox', { name: /Query language/ })).toBeVisible();
    });
  });
});

test.describe('Query editor with fixture data', () => {
  test.describe.configure({ mode: 'serial' });

  test('CloudWatch Metrics: builder query against E2E/Demo returns frames', async ({
    readProvisionedDataSource,
    explorePage,
    page,
  }) => {
    const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
    await page.goto(exploreUrl(ds.uid));
    await selectQueryMode(page, 'CloudWatch Metrics');

    // Custom namespace/metric aren't in the hardcoded option list, so type them
    // and press Enter to accept the "Hit enter to add" entry.
    await page.getByRole('combobox', { name: 'Namespace' }).click();
    await page.keyboard.type(NAMESPACE);
    await page.keyboard.press('Enter');
    await page.getByRole('combobox', { name: 'Metric name' }).click();
    await page.keyboard.type(METRIC);
    await page.keyboard.press('Enter');

    const { responsePromise, getBody } = await readQueryResponseBody(explorePage);
    await page
      .locator('[data-testid="query-editor-row"]')
      .getByRole('button', { name: /Run queries/ })
      .click();
    await responsePromise;
    const body = getBody();
    expect(body?.results?.A?.frames?.length).toBeGreaterThan(0);
  });

  test('CloudWatch Logs: log group selection lists /e2e/app', async ({
    readProvisionedDataSource,
    page,
  }) => {
    const ds = await readProvisionedDataSource<CloudWatchJsonData>({ fileName: PROVISIONING_FILE });
    await page.goto(exploreUrl(ds.uid));
    await selectQueryMode(page, 'CloudWatch Logs');
    await page.getByRole('button', { name: 'Select log groups' }).click();
    await expect(page.getByText(LOG_GROUP)).toBeVisible({ timeout: 15_000 });
  });
});
