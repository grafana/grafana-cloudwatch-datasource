import { expect, test } from '@grafana/plugin-e2e';
import { type APIRequestContext } from '@playwright/test';
import { type BackendDataSourceResponse } from '@grafana/runtime';

const PROVISIONED_FILE = 'datasources.yml';

// GRAFANA_URL is set only by the Cloud cron workflow (playwright-cloud); its presence signals
// a run against the shared Cloud instance rather than local/PR CI.
const isCloudRun = !!process.env.GRAFANA_URL;

// TIME_SERIES(1) is metric math that reads no metric, so any CloudWatch backend returns the value 1
// at every period: a backend round trip that needs no fixture data in the test account.
const EXPRESSION = 'TIME_SERIES(1)';
const PERIOD_SECONDS = 60;
const RANGE_MS = 15 * 60 * 1000;

interface Frame {
  schema?: { fields?: Array<{ name: string }> };
  data?: { values?: unknown[][] };
}

function postMetricsQuery(request: APIRequestContext, datasource: { type: string; uid: string }) {
  const to = Date.now();
  return request.post('/api/ds/query', {
    data: {
      from: String(to - RANGE_MS),
      to: String(to),
      queries: [
        {
          refId: 'A',
          datasource,
          queryMode: 'Metrics',
          metricQueryType: 0,
          metricEditorMode: 1,
          expression: EXPRESSION,
          region: 'default',
          namespace: '',
          metricName: '',
          dimensions: {},
          statistic: 'Average',
          period: String(PERIOD_SECONDS),
          id: '',
          label: '',
          matchExact: true,
          intervalMs: PERIOD_SECONDS * 1000,
          maxDataPoints: 100,
        },
      ],
    },
  });
}

test.describe('Metrics query path', () => {
  let datasource: { type: string; uid: string };
  let createdDataSourceUid: string | undefined;

  test.beforeEach(async ({ readProvisionedDataSource, createDataSource }) => {
    // The Cloud instance does not apply the local provisioning, so create the datasource through
    // the API there, as queryEditor.spec.ts does.
    const provisioned = await readProvisionedDataSource({ fileName: PROVISIONED_FILE });
    const ds = isCloudRun
      ? await createDataSource({
          type: provisioned.type,
          jsonData: { authType: 'keys', defaultRegion: process.env.AWS_DEFAULT_REGION ?? '' },
          secureJsonData: {
            accessKey: process.env.AWS_ACCESS_KEY_ID ?? '',
            secretKey: process.env.AWS_SECRET_ACCESS_KEY ?? '',
          },
        })
      : provisioned;
    datasource = { type: ds.type, uid: ds.uid };
    createdDataSourceUid = isCloudRun ? ds.uid : undefined;
  });

  test.afterEach(async ({ grafanaAPIClient }) => {
    if (!createdDataSourceUid) {
      return;
    }
    await grafanaAPIClient.deleteDataSourceByUID(createdDataSourceUid).catch(() => undefined);
    createdDataSourceUid = undefined;
  });

  test('a metric math query returns one value per period from the backend', { tag: '@aws' }, async ({ request }) => {
    // The first queries on Cloud can fail or return no frames while the datasource connection
    // warms up, so retry the whole round trip with backoff.
    await expect(async () => {
      const response = await postMetricsQuery(request, datasource);
      expect(response.ok(), `status ${response.status()}`).toBe(true);

      const body = (await response.json()) as Partial<BackendDataSourceResponse>;
      const result = body.results?.A as { error?: string; frames?: Frame[] } | undefined;
      expect(result?.error).toBeUndefined();

      const frame = result?.frames?.[0];
      const [timestamps, values] = frame?.data?.values ?? [];
      expect(frame?.schema?.fields?.map((f) => f.name)).toEqual(['Time', 'Value']);
      expect(timestamps?.length, 'one point per period in the range').toBeGreaterThanOrEqual(
        RANGE_MS / 1000 / PERIOD_SECONDS - 1
      );
      expect(values).toHaveLength(timestamps?.length ?? -1);
      expect(
        values?.every((v) => v === 1),
        `values ${JSON.stringify(values)}`
      ).toBe(true);
    }).toPass({ intervals: [1_000, 2_000, 4_000, 8_000], timeout: 60_000 });
  });
});
