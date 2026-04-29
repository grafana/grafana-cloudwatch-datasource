# CloudWatch Logs schemads Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing Grafana `schemads` integration so CloudWatch **Logs** (Logs Insights) can participate in schema discovery and, when applicable, Grafana SQL (`grafanaSQL`) execution alongside the current **metrics** tables.

**Architecture:** Keep a single `SchemaProvider` (or a small internal split) that dispatches on table identity: `metrics|<namespace>` (existing) vs a new **virtual logs table** (see naming below). Reuse `LogGroupsService.GetLogGroups` and `GetLogGroupFields` for parameter enumeration and column metadata—the same APIs already backing `/log-groups` and `/log-group-fields`. Add request normalization in `normalizeGrafanaSQLRequest` for logs-shaped `schemas.Query` payloads that produces a `CloudWatchLogsQuery` with `queryLanguage: SQL` and an expression compatible with `executeLogActions` / `StartQuery`. Gate runtime behavior on `dsAbstractionApp` and align with Grafana core’s expectations for logs tabular frames.

**Tech Stack:** Go (`github.com/grafana/schemads`), AWS SDK v2 `cloudwatchlogs`, existing `pkg/cloudwatch/services/log_groups.go`, `resource_handler.go`, `sql.go`, `log_actions.go`.

---

## Preconditions and product alignment

- [ ] **Confirm Grafana platform scope:** Metrics Grafana SQL is documented in `pkg/cloudwatch/sql.go` as depending on `dsAbstractionApp` and `PluginContext.GrafanaConfig`. Logs SQL normalization will need the same toggles and possibly **core dsAbstraction** updates if the SQL engine does not yet emit `schemas.Query` for logs tables. Schedule a short sync with the Grafana datasources / dsAbstraction owners before locking the `schemas.Query` → native JSON mapping.
- [ ] **Choose the user-visible table model** using [Option A vs Option B tradeoffs](#table-model-option-a-vs-option-b) below; record the decision in this doc or an ADR.

---

## Table model: Option A vs Option B

Two viable shapes appear in comparable implementations: CloudWatch **metrics** schemads (table id + **parameters**) vs **Prometheus** schemads ([`pkg/promlib/resource/schema.go`](file:///Users/isabellasiu/grafana-plugins/grafana-prometheus-datasource/pkg/promlib/resource/schema.go)), where **each metric name is its own table id** with no `metricName` parameter.

### Option A — Single virtual table + table parameters

**Shape:** One table id, e.g. `logs`, with **required** `TableParameters`: `region`, optional `accountId`, and **log group** (name or ARN) selected via `TableParameterValues` / UI dependency chain—same **mental model as** `metrics|<namespace>` + `metricName`.

**Pros**

- **Stable cardinality in `Schema()` / `Tables()`:** One row in the table list plus parameter metadata; no need to list every log group up front.
- **Aligns with existing CloudWatch metrics code paths** in [`pkg/cloudwatch/schema.go`](pkg/cloudwatch/schema.go): `TableParameterValues` for `metricName`, `Columns` keyed by table + params.
- **Cross-account and multi-region:** Parameters encode context explicitly; less ambiguity than encoding everything in a string table name.
- **Large accounts:** Avoids returning tens of thousands of table names in a single `Tables()` response.

**Cons**

- **More UI / engine complexity:** The SQL or schema UI must support **dependent parameters** (pick region → then account → then log group) like metrics namespace → metric name.
- **`TableParameterValues` cost:** Listing log groups still requires `DescribeLogGroups` (paginated); must cache and rate-limit, similar to Task 4 in Phase 1.
- **Discovery ergonomics:** Users cannot “pick from full table list” in one dropdown if the engine expects parameters first (depends on how dsAbstraction renders parameters).

---

### Option B — One schemads table id per log group (Prometheus-style)

**Shape:** Table id encodes the log group, e.g. `logs|<arn>` or `logs|<region>/<encodedName>` (exact encoding is an implementation detail). **`Tables()` / full schema** return **many** table entries—mirroring Prometheus where **each metric name** is a table and `Columns(metric)` loads labels per metric.

**Pros**

- **Matches Prometheus schemads:** Familiar for teams building against [`SchemaProvider`](file:///Users/isabellasiu/grafana-plugins/grafana-prometheus-datasource/pkg/promlib/resource/schema.go); **`TableParameterValues` can stay `nil`** for logs (as Prometheus does for those handlers).
- **Simpler `normalizeGrafanaSQLRequest`:** `query.Table` alone identifies the log group (after parsing the id); fewer moving parts in `TableParameterValues` at query time.
- **Lazy columns fit naturally:** `Columns()` receives concrete table names and calls `GetLogGroupFields` per table—same as Prometheus `fetchLabelNames(ctx, metric)`.

**Cons**

- **`Tables()` / `Schema()` size:** Unbounded with account size—same class of problem as listing all metric names in Prometheus, but log groups can be **very** numerous; responses may be slow, memory-heavy, or require **pagination** at the schemads protocol layer (if supported) or **prefix filtering** (DescribeLogGroups API supports prefixes—could narrow listed “tables”).
- **Repeated listing:** Every full schema refresh may re-fetch or re-cache large lists unless aggressively cached.
- **Encoding:** ARNs and names contain characters that need consistent escaping in table ids; mistakes create brittle SQL identifiers.

---

### Comparison summary

| Dimension | Option A (virtual `logs` + params) | Option B (one table id per log group) |
|-----------|-------------------------------------|----------------------------------------|
| Parity with **metrics** in this plugin | Strong | Weaker |
| Parity with **Prometheus** schemads | Weaker | Strong |
| **`Schema()` / `Tables()` payload size** | Small, fixed | Large, grows with log groups |
| **`TableParameterValues` implementation** | Required for log group picklists | Optional / nil if table id encodes group |
| **Cross-account clarity** | Params explicit | Must encode account in table id or assume context |
| **AWS API pressure** | Batched listing for picklists; `GetLogGroupFields` per resolved group | Same field API per table; listing groups if building full table list |

### Typical log group counts — does reality bias toward A?

There is **no single “typical”** profile; cardinality spans orders of magnitude.

- **Small accounts** (single app, few Lambdas, one cluster): often **tens** of log groups. Option **B** can remain tractable if `Tables()` is cached, paginated, or scoped (prefix/region).
- **Common production setups** (multiple services, API Gateway, ECS/EKS, Lambda fleets, VPC Flow Logs, audit logs): **hundreds** of groups per region is routine; multi-region multiplies visibility.
- **Large enterprises / observability-heavy tenants** (many teams, accounts, Lambdas each with distinct groups, retention policies): **thousands** of log groups per account or **aggregated across organization** is plausible.

**Implication:** A **large fraction of Grafana CloudWatch users who matter for “at scale” product reviews** will have enough log groups that returning **one schemads table name per group** (Option B unscoped) becomes **slow, heavy, or awkward** without strong pagination and caching. That supports **biasing the default architecture toward Option A** (or toward a **scoped Option B**, e.g. prefix-filtered table lists only).

**Caveat:** The **median** hobby/small workspace might never hit thousands of groups—**count alone** does not force A for every deployment; **worst-case + consistency with metrics** are the stronger reasons to prefer A as the **default** in this plugin.

### Recommendation (non-binding)

- Prefer **Option A** when **consistency with CloudWatch metrics** and **bounded schema responses** outweigh Prometheus symmetry—typical for this repo’s existing [`SchemaProvider`](pkg/cloudwatch/schema.go) patterns.
- Prefer **Option B** when **dsAbstraction / SQL UX** is proven on **Prometheus-style table lists** and product accepts **pagination + caching** for `Tables()` (and possibly scope limits: prefix, region, or “recent” groups).

Hybrid approaches (e.g. Option B only for a filtered subset, or Option A with optional “expand to table list” in the UI) are possible but increase scope; record if chosen.

---

## Prefix filtering in the schema

**Yes.** Prefix filtering is both **AWS-supported** and **already wired** in this plugin’s log-group listing: [`LogGroupsService.GetLogGroups`](pkg/cloudwatch/services/log_groups.go) passes `LogGroupNamePrefix` into `DescribeLogGroupsInput`. [`LogsRequest`](pkg/cloudwatch/models/resources/resource_request.go) also carries `LogGroupNamePrefix` / pattern-style fields for cross-account flows.

**How to expose it in schemads**

1. **Optional table parameter** — Add something like `logGroupNamePrefix` to the logs table’s `TableParameters` in [`pkg/cloudwatch/schema.go`](pkg/cloudwatch/schema.go): `Required: false`, `DependsOn: [region]` (and optionally `accountId` if you want ordering after account selection). Same pattern as `RegionTableParameter` → `AccountIdTableParameter` → `MetricNameTableParameter` for metrics.

2. **Thread into AWS calls** — In `TableParameterValues` when resolving values for the log group identifier parameter, read `req.TableParameters["logGroupNamePrefix"]` (exact key matches your constant) and set [`resources.LogGroupsRequest.LogGroupNamePrefix`](pkg/cloudwatch/models/resources/resource_request.go) before calling `GetLogGroups`.

3. **Cache keys** — Include the prefix (or a sentinel for “empty”) in cache keys for listed log group names so cached lists are not mixed across prefixes.

4. **Option B (`Tables()` lists many ids)** — Prefix becomes important to keep responses bounded: either **require** a non-empty prefix before listing “tables,” or **document** that full enumeration is only available with prefix empty and may paginate slowly (product decision).

5. **Grafana SQL / `normalizeGrafanaSQLRequest`** — If prefix affects which log group is selected, persist the chosen prefix in `TableParameterValues` on the wire so execution matches discovery (same as any other parameter).

6. **UI / dsAbstraction** — The schema layer only **advertises** the parameter; the SQL UI must render an optional “prefix” control before the log-group dropdown. Confirm with core that dependent optional parameters behave as expected.

**Limits:** AWS prefix filtering is a **prefix** on the log group **name**, not a general substring regex (unless you use a different API path or cross-account pattern behavior—see existing service logic for `IncludeLinkedAccounts`). Document that for users.

---

## File map (create vs modify)

| Responsibility | Files |
|----------------|--------|
| Logs table id + parameter constants; dispatch in schema handlers | Modify: `pkg/cloudwatch/schema.go` |
| Cache keys + TTL helpers for log group field lists (and optionally log group name lists) | Modify: `pkg/cloudwatch/schema_cache.go` (or extend existing cache usage in `DataSource`) |
| Tests for logs schema paths | Modify: `pkg/cloudwatch/schema_test.go`, `pkg/cloudwatch/schema_cache_test.go` |
| Grafana SQL → `CloudWatchLogsQuery` rewrite | Modify: `pkg/cloudwatch/sql.go`; tests in `pkg/cloudwatch/sql_test.go` |
| Ensure rewritten logs queries reach `executeLogActions` | Modify: `pkg/cloudwatch/cloudwatch.go` (`QueryData` routing), possibly `pkg/cloudwatch/log_actions.go` if unmarshalling needs new fields |
| Wire-only / imports | `pkg/cloudwatch/cloudwatch.go` (`NewSchemaDatasource` already wraps `SchemaProvider`; no second datasource if dispatch stays in one provider) |

---

## Phase 1: Schema surface (discovery only)

**Objective:** `Schema`, `Tables`, `Columns`, `TableParameterValues`, and optionally `ColumnValues` respond correctly for the `logs` table without changing query execution yet.

### Task 1: Constants and parsing

**Files:** `pkg/cloudwatch/schema.go`

- [ ] Add `logsTableName = "logs"` (or `logs|` prefix pattern if you prefer symmetry with `metrics|`—but then avoid empty suffix; prefer single id `logs`).
- [ ] Add `LogGroupNameTableParameter` / `LogGroupArnTableParameter` constants (pick **one** canonical identifier for AWS calls; ARN is unambiguous for cross-account).
- [ ] Add `func isLogsTable(table string) bool` and use it in handlers.

### Task 2: Include the logs table in `getAllTables`

- [ ] Append one `schemas.Table` for `logs` with:
  - `TableParameters`: `[region, accountId?, optional logGroupNamePrefix, logGroupArn or logGroupName]` — mirror metrics’ dependency chain; include **optional prefix** per [Prefix filtering in the schema](#prefix-filtering-in-the-schema).
  - `TableHints`: empty or document future hints (e.g. query language); do not invent hints until SQL mapping exists.
  - `Columns`: **either** omit concrete columns at full-schema time **or** supply a minimal static set (`@timestamp`, `@message` / `message`) plus note that full discovery requires `TableParameterValues` for region + log group. Prefer matching metrics: lightweight placeholders in `Schema()`, richer columns in `Columns()` when params are present.

### Task 3: `Columns()` for logs

- [ ] When `req.Tables` contains `logs`, read `region` and optional `accountId` from `req.TableParameters`; require log group identifier.
- [ ] Call existing `LogGroupsService.GetLogGroupFields` with `resources.LogGroupFieldsRequest` (same shapes as `LogGroupFieldsHandler`).
- [ ] Map each returned field to `schemas.Column`: default `ColumnTypeString` unless you map known system fields (`@timestamp` → timestamp).
- [ ] Return per-table errors in `Errors` map on API failure (same pattern as metrics namespaces).

### Task 4: `TableParameterValues()` for logs

- [ ] Implement value lists for `LogGroupNameTableParameter` (or ARN parameter): use `GetLogGroups` with pagination, respect existing `ListMetricsPageLimit`-style limits if present for logs, and pass **`LogGroupNamePrefix` from the optional prefix table parameter** when set ([Prefix filtering in the schema](#prefix-filtering-in-the-schema)).
- [ ] Reuse `region` and `accountId` from request context the same way metrics reuse them for `MetricNameTableParameter`.

### Task 5: `ColumnValues()` for logs (optional in v1)

- [ ] CloudWatch does not provide a direct “distinct values for arbitrary log field” API like `ListMetrics`. **Options:** (a) skip `ColumnValues` for logs and return empty + documented limitation; (b) run a constrained Insights query sample (expensive, rate limits); (c) return only fields from `GetLogGroupFields` without value enumeration. **Recommendation:** (a) or (c) for v1; document in schema column descriptions.

### Task 6: Tests

- [ ] Extend `schema_test.go` with table-router tests: `Columns` for `logs` calls mocked `GetLogGroupFields`.
- [ ] Extend `schema_cache_test.go` if new cache keys are introduced for field lists.

**Verification:** `go test ./pkg/cloudwatch/ -run Schema -count=1` (adjust pattern); manual CallResource against `abstractionSchema/*` routes if integration harness exists.

---

## Phase 2: Grafana SQL normalization (execution)

**Objective:** When `schemas.Query` has `GrafanaSql: true` and `Table` identifies logs, rewrite `backend.DataQuery.JSON` into a native `CloudWatchLogsQuery` that the existing logs pipeline executes.

### Task 7: Extend `normalizeGrafanaSQLRequest`

**Files:** `pkg/cloudwatch/sql.go`, `pkg/cloudwatch/sql_test.go`

- [ ] After unmarshalling `schemas.Query`, branch: if `isLogsTable(query.Table)` then **do not** require `metricsTableNamespace`.
- [ ] Extract `region`, optional `accountId`, log group id from `TableParameterValues`.
- [ ] Map `query.Filters` to a Logs Insights SQL fragment or to fields on `models.LogsQuery`—**this requires the agreed SQL semantics** (e.g. `WHERE` → `filter` in Insights SQL, `LIMIT`, time range from request `TimeRange`, not from SQL alone).
- [ ] Set `queryMode: logs`, `queryLanguage: SQL`, `expression` to the final string expected by `buildFinalQueryString` / `StartQuery` (reuse helpers in `log_actions.go` where possible).
- [ ] Register refID in `grafanaSQLRefIDs` if tabular post-processing applies; confirm whether `convertToTabular` is metrics-only or must be skipped for logs frames.

### Task 8: `QueryData` routing

**Files:** `pkg/cloudwatch/cloudwatch.go`

- [ ] Ensure normalized logs queries are not dropped by `normalizeGrafanaSQLRequest`’s “omit on failure” paths.
- [ ] Confirm `executeLogActions` receives requests with `type: logAction` (or whatever key `log_actions.go` expects); align JSON tags with `models.LogsQuery` unmarshalling.

### Task 9: Integration tests

- [ ] Add `sql_test.go` cases: input `schemas.Query` logs payload → unmarshalled `CloudWatchLogsQuery` fields match expectations; optional golden string for SQL expression.

**Verification:** `go test ./pkg/cloudwatch/ -run normalizeGrafanaSQL -count=1`

---

## Phase 3: Operational concerns

- [ ] **Rate limits:** `DescribeLogGroups` and `GetLogGroupFields` can throttle; reuse caching patterns from `getOrSetSchemaDimensionKeys` with distinct key prefixes for logs.
- [ ] **Cross-account:** Same as UI: pass `accountId` into resource requests when present.
- [ ] **IAM:** Document required permissions (`logs:DescribeLogGroups`, `logs:GetLogGroupFields`, `logs:StartQuery`, etc.) in plugin docs if not already complete.
- [ ] **Feature toggle:** Keep logs schemads + SQL behind `dsAbstractionApp` until product says otherwise (consistent with metrics).

---

## Self-review (spec coverage)

| Requirement | Task phase |
|-------------|------------|
| Schemads discovery for logs | Phase 1 |
| Grafana SQL execution path | Phase 2 |
| Caching / performance | Phase 3, Tasks 4–5 |
| Tests | Phases 1–2 |

**Placeholder scan:** No unresolved “TBD” for core decisions—only explicit **product choices** (table id string, ARN vs name, ColumnValues v1 scope) to resolve in Preconditions.

---

## Execution handoff

Plan complete: `docs/superpowers/plans/2026-04-29-cloudwatch-logs-schemads.md`.

**1. Subagent-driven (recommended)** — Use superpowers:subagent-driven-development: one subagent per task, review between tasks.

**2. Inline execution** — Use superpowers:executing-plans in this session with checkpoints after each phase.

Which approach do you want?
