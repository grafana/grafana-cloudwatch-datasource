import { expect, test, type E2ESelectorGroups, type PanelEditPage } from '@grafana/plugin-e2e';
import { type Locator, type Page } from '@playwright/test';
import { type BackendDataSourceResponse } from '@grafana/runtime';

const PROVISIONED_FILE = 'datasources.yml';
const LIVE_DATA_SOURCE = { name: 'amazon_eks', type: 'audit' };
const LIVE_DATA_SOURCE_KEY = `${LIVE_DATA_SOURCE.name}.${LIVE_DATA_SOURCE.type}`;
const LIVE_QUERY = 'stats count(*) as event_count';

async function selectLogsMode(panelEditPage: PanelEditPage, selectors: E2ESelectorGroups, queryEditor: Locator) {
  const queryMode = queryEditor.getByLabel('Query mode');
  await queryMode.click();
  await panelEditPage
    .getByGrafanaSelector(selectors.components.Select.option)
    .getByText('CloudWatch Logs', { exact: true })
    .click();
  await expect(queryEditor.getByText('Query scope', { exact: true })).toBeVisible();
}

async function selectLiveDataSource(page: Page, queryEditor: Locator) {
  await queryEditor.getByRole('button', { name: 'Select data sources', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Select data sources', exact: true })).toBeVisible();

  const search = page.getByRole('textbox', { name: 'data source search', exact: true });
  await search.fill(LIVE_DATA_SOURCE.type);
  await expect(page.getByLabel(LIVE_DATA_SOURCE_KEY, { exact: true })).toBeVisible();
  await expect(page.getByLabel('amazon_eks.api_server', { exact: true })).not.toBeVisible();

  await page
    .getByRole('row', { name: LIVE_DATA_SOURCE_KEY, exact: true })
    .getByText(LIVE_DATA_SOURCE_KEY, { exact: true })
    .click();
  await expect(page.getByText('1 data source selected', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Apply selection', exact: true }).click();
}

test.describe('Query editor', () => {
  test.beforeEach(async ({ panelEditPage, readProvisionedDataSource }) => {
    const ds = await readProvisionedDataSource({ fileName: PROVISIONED_FILE });
    await panelEditPage.datasource.set(ds.name);
  });

  test('smoke: should render the query editor', { tag: '@plugins' }, async ({ panelEditPage, page, selectors }) => {
    const queryEditor = panelEditPage.getQueryEditorRow('A');
    await expect(queryEditor.getByLabel('Query mode')).toBeVisible();
    await expect(queryEditor.getByLabel('Region:')).toBeVisible();

    await queryEditor.getByLabel('Query mode').click();
    console.log(await page.getByRole('option').evaluateAll((options) => options.map((option) => option.outerHTML)));
    const queryModeOptions = panelEditPage.getByGrafanaSelector(selectors.components.Select.option);
    await expect(queryModeOptions.getByText('CloudWatch Metrics', { exact: true })).toBeVisible();
    await expect(queryModeOptions.getByText('CloudWatch Logs', { exact: true })).toBeVisible();
  });

  test('should render log group and data source selectors for Logs Insights queries', async ({
    panelEditPage,
    selectors,
  }) => {
    const queryEditor = panelEditPage.getQueryEditorRow('A');
    await selectLogsMode(panelEditPage, selectors, queryEditor);

    await expect(queryEditor.getByLabel('Logs Mode:')).toBeVisible();
    await expect(queryEditor.getByLabel('Query language:')).toBeVisible();
    await expect(queryEditor.getByRole('button', { name: 'Select log groups', exact: true })).toBeVisible();
    await expect(queryEditor.getByRole('button', { name: 'Select data sources', exact: true })).toBeVisible();
  });

  test('should filter and select CloudWatch Logs data sources by name or type', async ({
    panelEditPage,
    page,
    selectors,
  }) => {
    const queryEditor = panelEditPage.getQueryEditorRow('A');
    await selectLogsMode(panelEditPage, selectors, queryEditor);
    await selectLiveDataSource(page, queryEditor);

    await expect(queryEditor.getByText(LIVE_DATA_SOURCE_KEY, { exact: true })).toBeVisible();
    await expect(queryEditor.getByRole('button', { name: 'Clear selection', exact: true })).toBeVisible();
  });

  test('should scope a Logs Insights query to the selected data source name and type', async ({
    panelEditPage,
    page,
    selectors,
  }) => {
    const queryEditor = panelEditPage.getQueryEditorRow('A');
    await panelEditPage.setVisualization('Table');
    await selectLogsMode(panelEditPage, selectors, queryEditor);
    await selectLiveDataSource(page, queryEditor);

    const editor = panelEditPage.getByGrafanaSelector(selectors.components.CodeEditor.container, {
      root: queryEditor,
    });
    const editorInput = editor.getByRole('textbox');
    await editorInput.click();
    await page.keyboard.press('Control+A');
    await page.keyboard.insertText(LIVE_QUERY);
    await expect(editorInput).toHaveValue(LIVE_QUERY);

    let responseBody: Partial<BackendDataSourceResponse> | undefined;
    const requestPromise = panelEditPage.waitForQueryDataRequest(
      (request) => request.postDataJSON()?.queries?.[0]?.subtype === 'StartQuery'
    );
    // Logs Insights queries poll GetQueryResults after StartQuery, so wait for
    // the final response before asserting on the panel data.
    const responsePromise = panelEditPage.refreshPanel({
      waitForResponsePredicateCallback: async (response) => {
        if (!response.url().includes(selectors.apis.DataSource.query)) {
          return false;
        }

        const body = (await response.json().catch(() => undefined)) as Partial<BackendDataSourceResponse> | undefined;
        const result = body?.results?.A;
        const status = result?.frames?.[0]?.schema?.meta?.custom?.Status;
        if (result?.error || status === 'Complete') {
          responseBody = body;
          return true;
        }
        return false;
      },
    });
    const [request, response] = await Promise.all([requestPromise, responsePromise]);
    const requestBody = request.postDataJSON();
    const result = responseBody?.results?.A;

    expect(response.ok()).toBe(true);
    expect(requestBody.queries[0].logDataSources).toEqual([LIVE_DATA_SOURCE]);
    expect(result?.error).toBeUndefined();
    expect(result?.frames?.[0]?.schema?.meta?.custom?.Status).toBe('Complete');
    await expect(panelEditPage.panel.fieldNames).toContainText(['event_count']);
    await expect(panelEditPage.panel.data.filter({ hasText: /^[1-9]\d*$/ })).toHaveCount(1);
  });
});
