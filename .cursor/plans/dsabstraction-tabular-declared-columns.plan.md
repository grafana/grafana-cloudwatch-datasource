---
name: grafanaSQL tabular vs declared schema
overview: >
  Abandon the "Series" / arbitrary label-key approach. dsAbstraction (grafana-enterprise) requires every field in the tabular frame to match a column declared in schemads for that table (and optional SELECT projection). Fix by (1) constraining convertToTabular output to declared dimension names, and (2) populating only those dimensions from CloudWatch—never emitting undeclared columns such as Series.
status: path-B implemented (schema wildcard injection in normalizeGrafanaSQLRequest)
---

# grafanaSQL tabular rows must match declared schema columns

## Why the previous approach fails

- CloudWatch metrics schema (see [`schema.go`](pkg/cloudwatch/schema.go)) advertises **`time`**, **`value`**, and **dimension keys** for the namespace/metric (from hardcoded maps or ListMetrics). There is **no** `Series` column.
- [`convertToTabular`](pkg/cloudwatch/sql.go) builds one field per key in `valueField.Labels`. If we put identity in a synthetic **`Series`** label, the flattened frame gains a column **not present** in [`metricsColumns`](pkg/cloudwatch/schema.go) / `Columns()` output, which **breaks dsAbstraction** validation (vtable expects fields ⊆ declared schema + projection).
- **Conclusion:** Identity must appear under **existing dimension column names** (e.g. `InstanceId`), not new names.

## schemads query contract (relevant fields)

From `github.com/grafana/schemads` `Query`:

- `Table`, `Filters`, `TableParameterValues`, `TableHintValues`
- **`Columns []string`** — SELECT projection; **`nil` means all columns** (per schemads docs in `types.go`)

## Target behavior

1. **Tabular output columns** = `{ time, value } ∪ D` where `D` is a set of **dimension names** that are:
   - Declared for `metrics|<namespace>|<metric>` via schema, **and**
   - Either listed in `schemas.Query.Columns` (when non-nil), or all dimension columns when `Columns` is nil (same semantics as schemads).

2. **No extra fields** from `data.Labels` keys that are not in `D` (e.g. drop `Series` if ever present).

3. **Values** for each row: for each `d ∈ D`, the cell must come from CloudWatch’s per-series identity—either:
   - Already mapped by existing [`parseLabels`](pkg/cloudwatch/response_parser.go) when the underlying `CloudWatchQuery` has matching **dimension keys** (filters / wildcard dimensions), or
   - A **new mapping** from `MetricDataResult.Label` into **named dimensions** that exist in `D` (metric-specific parsing or label template), **without** inventing non-schema column names.

## Implementation directions (choose / combine after spike)

### A. Thread SQL projection into `convertToTabular`

Today [`normalizeGrafanaSQLRequest`](pkg/cloudwatch/sql.go) replaces grafanaSQL JSON with native CloudWatch JSON and only returns `map[refID]struct{}`. **Lose** `schemas.Query.Columns` and `Table`.

- Extend the return to carry per refID: **`schemas.Query` minimal snapshot** (`Table`, `Columns`, optional `Filters`) or precomputed **allowed dimension names** for tabular projection.
- Update [`QueryData`](pkg/cloudwatch/cloudwatch.go) to pass that map into **`convertToTabular`** (signature change).
- **`convertToTabular`:** compute `D` = `Columns` ∩ schema dimensions for `Table`, or all schema dimensions if `Columns == nil` (may require **resolving schema** inside the datasource for that table—`SchemaProvider.Columns` or a small helper reusing [`dimensionColumnsForNamespace`](pkg/cloudwatch/schema.go) + `splitTableName`).

### B. Populate labels only with declared dimensions

- **`buildDataFrames` / `parseLabels`:** ensure `valueField.Labels` keys ⊆ metric dimension keys used for that query. Avoid adding **`Series`** for grafanaSQL tabular unless schema gains such a column (not recommended).
- For **inferred SEARCH with no user filters:** spike whether to inject **wildcard dimensions** from schema so CloudWatch SEARCH + [`buildSearchExpressionLabel`](pkg/cloudwatch/metric_data_query_builder.go) can emit `${PROP('Dim.X')}` for each dimension, allowing `parseLabels` to fill **declared** keys (large behavior change—needs product sign-off).

### C. Documentation / guardrails (short term)

- If undimensioned queries cannot be mapped to schema columns reliably, document that **grafanaSQL** multi-series tabular requires **filters** (or explicit dimension columns in SQL) so pushdown dimensions match schema.

## Verification

- Contract tests: flattened frame field names ⊆ `{time, value} ∪ declared dimensions` for the table and projection.
- Run dsAbstraction / enterprise integration tests if available in CI.

## References

- [`metricsColumns`](pkg/cloudwatch/schema.go) — declared shape.
- [`Query.Columns`](https://github.com/grafana/schemads/blob/main/types.go) — projection.
- Original superseded plan: “Search label parity” that added `Series` — **do not implement** as-is for dsAbstraction.
