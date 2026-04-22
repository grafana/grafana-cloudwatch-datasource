package cloudwatch

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/grafana/grafana-aws-sdk/pkg/cloudWatchConsts"
	schemas "github.com/grafana/schemads"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models/resources"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/services"
)

// StatisticTableHintValueKey is the key in schemas.Query.TableHintValues for the
// metric statistic (uppercase per schemads).
const StatisticTableHintValueKey = "STATISTIC"

// Schemads table parameter keys for CloudWatch metrics tables (JSON wire names).
// MetricName identifies CloudWatch MetricStat.Metric.MetricName, which the API requires.
const (
	RegionTableParameter     = "region"
	AccountIdTableParameter  = "accountId"
	MetricNameTableParameter = "metricName"
)

var (
	dimensionOperators = []schemas.Operator{
		schemas.OperatorEquals,
		schemas.OperatorIn,
	}

	timeColumn = schemas.Column{
		Name: "time",
		Type: schemas.ColumnTypeTimestamp,
	}
	valueColumn = schemas.Column{
		Name: "value",
		Type: schemas.ColumnTypeFloat64,
	}

	// Advertised on each metrics table as Table.TableHints.
	statisticTableHint = schemas.TableHint{
		Name:        "statistic",
		Description: "CloudWatch metric statistic. Standard values include Average, Minimum, Maximum, Sum, SampleCount, and IQM; extended statistics include percentiles (e.g. p99) and trimmed means (e.g. TM(90:10)). Syntax must match CloudWatch MetricStat.",
		HasValue:    true,
	}

	metricsTableParameters = []schemas.TableParameter{
		{Name: RegionTableParameter, Root: true, Required: true},
		{Name: AccountIdTableParameter, DependsOn: []string{RegionTableParameter}, Required: false},
		{Name: MetricNameTableParameter, DependsOn: []string{RegionTableParameter}, Required: true},
	}
)

// SchemaProvider implements the schemads SchemaHandler, TablesHandler,
// ColumnsHandler, and TableParameterValuesHandler interfaces for CloudWatch
// metrics. Tables are identified as "metrics|<namespace>" (e.g.
// "metrics|AWS/EC2"); the metric name is the metricName table parameter.
// Columns are dimension keys. The "metrics|" prefix distinguishes metrics
// tables from any future table types (e.g. "logs|...").
type SchemaProvider struct {
	ds *DataSource
}

// NewSchemaProvider creates a new SchemaProvider backed by ds.
func NewSchemaProvider(ds *DataSource) *SchemaProvider {
	return &SchemaProvider{ds: ds}
}

const metricsTablePrefix = "metrics|"

// metricsTableNamespace parses a "metrics|<namespace>" table identifier and
// returns the CloudWatch namespace. ok is false if table does not use the
// metrics prefix or the namespace is empty.
func metricsTableNamespace(table string) (namespace string, ok bool) {
	if !strings.HasPrefix(table, metricsTablePrefix) {
		return "", false
	}
	ns := table[len(metricsTablePrefix):]
	if ns == "" || strings.Contains(ns, "|") {
		return "", false
	}
	return ns, true
}

// Schema implements schemas.SchemaHandler.
func (p *SchemaProvider) Schema(ctx context.Context, req *schemas.SchemaRequest) (*schemas.SchemaResponse, error) {
	ctx = instrumentContext(ctx, "schema/fullSchema", req.PluginContext)
	// Namespace errors are non-fatal; return whatever tables succeeded alongside the error string.
	tables, tableErrs := p.getAllTables(ctx)

	// Pre-populate the root "region" table parameter values for every table.
	// A failure here is also non-fatal; the schema is still usable without pre-populated regions.
	var tableParamValues map[string]map[string][]string
	regionNames, regionErr := p.getRegionNames(ctx)
	if regionErr == nil && len(regionNames) > 0 {
		tableParamValues = make(map[string]map[string][]string, len(tables))
		for _, t := range tables {
			tableParamValues[t.Name] = map[string][]string{RegionTableParameter: regionNames}
		}
	}

	// Collect all errors into a single string for the schema response.
	var errStr string
	var errParts []string
	if len(tableErrs) > 0 {
		namespaces := make([]string, 0, len(tableErrs))
		for ns := range tableErrs {
			namespaces = append(namespaces, ns)
		}
		sort.Strings(namespaces)
		for _, ns := range namespaces {
			errParts = append(errParts, fmt.Sprintf("%s: %s", ns, tableErrs[ns]))
		}
	}
	if regionErr != nil {
		errParts = append(errParts, fmt.Sprintf("regions: %s", regionErr))
	}
	if len(errParts) > 0 {
		errStr = strings.Join(errParts, "; ")
	}

	return &schemas.SchemaResponse{
		FullSchema: &schemas.Schema{
			Tables:               tables,
			TableParameterValues: tableParamValues,
		},
		Errors: errStr,
	}, nil
}

// Tables implements schemas.TablesHandler.
func (p *SchemaProvider) Tables(ctx context.Context, req *schemas.TablesRequest) (*schemas.TablesResponse, error) {
	ctx = instrumentContext(ctx, "schema/tables", req.PluginContext)
	tables, errs := p.getAllTables(ctx)

	names := make([]string, len(tables))
	tableParamMap := make(map[string][]schemas.TableParameter, len(tables))
	tableHintsMap := make(map[string][]schemas.TableHint, len(tables))
	for i, t := range tables {
		names[i] = t.Name
		tableParamMap[t.Name] = t.TableParameters
		tableHintsMap[t.Name] = t.TableHints
	}

	return &schemas.TablesResponse{
		Tables:          names,
		TableParameters: tableParamMap,
		TableHints:      tableHintsMap,
		Errors:          errs,
	}, nil
}

// Columns implements schemas.ColumnsHandler.
func (p *SchemaProvider) Columns(ctx context.Context, req *schemas.ColumnsRequest) (*schemas.ColumnsResponse, error) {
	ctx = instrumentContext(ctx, "schema/columns", req.PluginContext)
	region := req.TableParameters[RegionTableParameter]
	if region == "" {
		region = defaultRegion
	}

	var accountId *string
	if id := req.TableParameters[AccountIdTableParameter]; id != "" {
		accountId = &id
	}

	cols := make(map[string][]schemas.Column, len(req.Tables))
	errs := make(map[string]string)
	for _, tableName := range req.Tables {
		if tableName == "" {
			continue
		}
		namespace, nsOK := metricsTableNamespace(tableName)
		if !nsOK {
			errs[tableName] = fmt.Sprintf("unrecognised table format %q: expected metrics|<namespace>", tableName)
			continue
		}
		dimCols, err := p.dimensionColumnsForNamespace(ctx, region, accountId, namespace)
		if err != nil {
			errs[tableName] = err.Error()
			continue
		}
		cols[tableName] = metricsColumns(dimCols)
	}
	return &schemas.ColumnsResponse{Columns: cols, Errors: errs}, nil
}

// TableParameterValues implements schemas.TableParameterValuesHandler.
func (p *SchemaProvider) TableParameterValues(ctx context.Context, req *schemas.TableParameterValuesRequest) (*schemas.TableParametersValuesResponse, error) {
	ctx = instrumentContext(ctx, "schema/tableParameterValues", req.PluginContext)
	result := make(map[string][]string)

	switch req.TableParameter {
	case RegionTableParameter:
		regionNames, err := p.getRegionNames(ctx)
		if err != nil {
			return nil, err
		}
		result[RegionTableParameter] = regionNames

	case AccountIdTableParameter:
		region := req.DependencyValues[RegionTableParameter]
		if region == "" {
			break
		}
		service, err := p.ds.GetAccountsService(ctx, region)
		if err != nil {
			return &schemas.TableParametersValuesResponse{TableParameterValues: result}, nil
		}
		accountResponses, err := service.GetAccountsForCurrentUserOrRole(ctx)
		if err != nil || len(accountResponses) == 0 {
			// Not a monitoring account or access denied — return empty, not an error.
			break
		}
		accountIds := make([]string, 0, len(accountResponses)+1)
		accountIds = append(accountIds, "all")
		for _, ar := range accountResponses {
			accountIds = append(accountIds, ar.Value.Id)
		}
		result[AccountIdTableParameter] = accountIds

	case MetricNameTableParameter:
		namespace, nsOK := metricsTableNamespace(req.Table)
		if !nsOK || namespace == "" {
			break
		}
		if metrics, known := cloudWatchConsts.NamespaceMetricsMap[namespace]; known {
			result[MetricNameTableParameter] = metrics
			break
		}
		region := req.DependencyValues[RegionTableParameter]
		if region == "" {
			break
		}
		var rr *resources.ResourceRequest
		if id := strings.TrimSpace(req.DependencyValues[AccountIdTableParameter]); id != "" {
			rr = &resources.ResourceRequest{Region: region, AccountId: &id}
		}
		svc, err := p.ds.GetListMetricsService(ctx, region)
		if err != nil {
			return &schemas.TableParametersValuesResponse{
				TableParameterValues: result,
				Errors:               map[string]string{MetricNameTableParameter: err.Error()},
			}, nil
		}
		metricRows, err := svc.GetMetricsByNamespace(ctx, resources.MetricsRequest{
			ResourceRequest: rr,
			Namespace:       namespace,
		})
		if err != nil {
			return &schemas.TableParametersValuesResponse{
				TableParameterValues: result,
				Errors:               map[string]string{MetricNameTableParameter: err.Error()},
			}, nil
		}
		names := make([]string, 0, len(metricRows))
		for _, row := range metricRows {
			if row.Value.Name != "" {
				names = append(names, row.Value.Name)
			}
		}
		sort.Strings(names)
		result[MetricNameTableParameter] = names

	default:
		return &schemas.TableParametersValuesResponse{
			Errors: map[string]string{req.TableParameter: fmt.Sprintf("unsupported table parameter %q", req.TableParameter)},
		}, nil
	}

	return &schemas.TableParametersValuesResponse{TableParameterValues: result}, nil
}

// getAllTables returns one schemas.Table per CloudWatch namespace (built-in
// from cloudWatchConsts plus custom namespaces from datasource settings).
// Errors are collected per namespace and returned alongside whatever tables
// succeeded.
func (p *SchemaProvider) getAllTables(ctx context.Context) ([]schemas.Table, map[string]string) {
	var tables []schemas.Table
	errs := make(map[string]string)

	namespaces := make([]string, 0, len(cloudWatchConsts.NamespaceMetricsMap))
	for ns := range cloudWatchConsts.NamespaceMetricsMap {
		namespaces = append(namespaces, ns)
	}

	for _, namespace := range namespaces {
		dimCols, err := p.dimensionColumnsForNamespace(ctx, defaultRegion, nil, namespace)
		if err != nil {
			errs[namespace] = err.Error()
			continue
		}
		tables = append(tables, schemas.Table{
			Name:            metricsTablePrefix + namespace,
			TableParameters: metricsTableParameters,
			TableHints:      metricsTableHints(),
			Columns:         metricsColumns(dimCols),
		})
	}

	// Custom namespaces from datasource settings (skip duplicates of built-in namespaces).
	if p.ds.Settings.Namespace != "" {
		customSeen := make(map[string]struct{})
		for _, customNS := range strings.Split(p.ds.Settings.Namespace, ",") {
			customNS = strings.TrimSpace(customNS)
			if customNS == "" {
				continue
			}
			if _, exists := cloudWatchConsts.NamespaceMetricsMap[customNS]; exists {
				continue
			}
			if _, dup := customSeen[customNS]; dup {
				continue
			}
			customSeen[customNS] = struct{}{}

			dimCols, err := p.dimensionColumnsForNamespace(ctx, defaultRegion, nil, customNS)
			if err != nil {
				errs[customNS] = err.Error()
				continue
			}
			tables = append(tables, schemas.Table{
				Name:            metricsTablePrefix + customNS,
				TableParameters: metricsTableParameters,
				TableHints:      metricsTableHints(),
				Columns:         metricsColumns(dimCols),
			})
		}
	}

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})
	if len(errs) == 0 {
		return tables, nil
	}
	return tables, errs
}

// ColumnValues implements schemas.ColumnValuesHandler. For each requested
// column (dimension key) it calls ListMetrics to enumerate the distinct values
// that exist for the given namespace + metricName combination. One ListMetrics
// API call is made per column, bounded by the datasource's ListMetricsPageLimit.
func (p *SchemaProvider) ColumnValues(ctx context.Context, req *schemas.ColumnValuesRequest) (*schemas.ColumnValuesResponse, error) {
	ctx = instrumentContext(ctx, "schema/columnValues", req.PluginContext)
	namespace, nsOK := metricsTableNamespace(req.Table)
	if !nsOK {
		return nil, fmt.Errorf("unrecognised table format %q: expected metrics|<namespace>", req.Table)
	}

	region := req.TableParameters[RegionTableParameter]
	if region == "" {
		return nil, fmt.Errorf("%s is a required table parameter", RegionTableParameter)
	}

	metricName := strings.TrimSpace(req.TableParameters[MetricNameTableParameter])
	if metricName == "" {
		return nil, fmt.Errorf("%s is a required table parameter when requesting dimension values", MetricNameTableParameter)
	}

	var accountId *string
	if id := req.TableParameters[AccountIdTableParameter]; id != "" {
		accountId = &id
	}

	columnValues := make(map[string][]string, len(req.Columns))

	// When no columns are specified, expand to dimension keys only.
	columns := req.Columns
	if len(columns) == 0 {
		dimCols, err := p.dimensionColumnsForNamespace(ctx, region, accountId, namespace)
		if err != nil {
			return nil, fmt.Errorf("could not enumerate columns for table %q: %w", req.Table, err)
		}
		dimNames := make([]string, len(dimCols))
		for i, c := range dimCols {
			dimNames[i] = c.Name
		}
		columns = dimNames
	}

	// Dimension columns are served via ListMetrics. time, value, and the statistic
	// hint identifier have no enumerable values here; skip them.
	var dimensionCols []string
	for _, col := range columns {
		switch col {
		case timeColumn.Name, valueColumn.Name, statisticTableHint.Name:
			// no enumerable values for data columns / statistic hint
		default:
			dimensionCols = append(dimensionCols, col)
		}
	}

	if len(dimensionCols) == 0 {
		return &schemas.ColumnValuesResponse{ColumnValues: columnValues}, nil
	}

	service, err := p.ds.GetListMetricsService(ctx, region)
	if err != nil {
		return &schemas.ColumnValuesResponse{
			ColumnValues: columnValues,
			Errors:       map[string]string{"": err.Error()},
		}, nil
	}

	dimValuesReq := resources.DimensionValuesForKeysRequest{
		ResourceRequest: &resources.ResourceRequest{Region: region, AccountId: accountId},
		Namespace:       namespace,
		MetricName:      metricName,
		DimensionKeys:   dimensionCols,
		DimensionFilter: []*resources.Dimension{},
	}
	p.ds.logger.FromContext(ctx).Info("Getting dimension values", "columns", dimensionCols, "namespace", namespace, "metricName", metricName)
	dimValues, err := service.GetDimensionValuesForKeys(ctx, dimValuesReq)
	if err != nil {
		p.ds.logger.FromContext(ctx).Error("Error getting dimension values", "columns", dimensionCols, "namespace", namespace, "metricName", metricName, "error", err)
		colErrors := make(map[string]string, len(dimensionCols))
		for _, col := range dimensionCols {
			colErrors[col] = err.Error()
		}
		return &schemas.ColumnValuesResponse{ColumnValues: columnValues, Errors: colErrors}, nil
	}
	for col, vals := range dimValues {
		// Prepend "*" so users can select "any value" for a dimension, matching the
		// frontend behaviour in FilterItem.tsx which also prepends "*" to dimension
		// value lists.
		if len(vals) > 0 {
			columnValues[col] = append([]string{"*"}, vals...)
		}
	}

	return &schemas.ColumnValuesResponse{ColumnValues: columnValues}, nil
}

func metricsTableHints() []schemas.TableHint {
	return []schemas.TableHint{statisticTableHint}
}

// metricsColumns assembles the full column list for a metrics table: time, value,
// then dimension keys.
func metricsColumns(dimCols []schemas.Column) []schemas.Column {
	cols := make([]schemas.Column, 0, 2+len(dimCols))
	cols = append(cols, timeColumn, valueColumn)
	cols = append(cols, dimCols...)
	return cols
}

// dimensionColumnsForNamespace returns the dimension keys for namespace as
// schemas.Column values with equality operators. For namespaces not present
// in the hardcoded map (e.g. custom namespaces), it falls back to a
// ListMetrics API call using the provided region and optional accountId.
func (p *SchemaProvider) dimensionColumnsForNamespace(ctx context.Context, region string, accountId *string, namespace string) ([]schemas.Column, error) {
	dimKeyResponses, err := services.GetHardCodedDimensionKeysByNamespace(namespace)
	if err != nil {
		// Not a known AWS namespace — query ListMetrics for the custom namespace.
		svc, svcErr := p.ds.GetListMetricsService(ctx, region)
		if svcErr != nil {
			return nil, svcErr
		}
		dimKeyResponses, err = svc.GetDimensionKeysByDimensionFilter(ctx, resources.DimensionKeysRequest{
			ResourceRequest: &resources.ResourceRequest{Region: region, AccountId: accountId},
			Namespace:       namespace,
			DimensionFilter: []*resources.Dimension{},
		})
		if err != nil {
			return nil, err
		}
	}
	cols := make([]schemas.Column, 0, len(dimKeyResponses))
	for _, r := range dimKeyResponses {
		cols = append(cols, schemas.Column{
			Name:      r.Value,
			Type:      schemas.ColumnTypeString,
			Operators: dimensionOperators,
		})
	}
	return cols, nil
}

// dimensionColumnNamesForNamespace returns dimension column names for namespace in the
// same order as [SchemaProvider.dimensionColumnsForNamespace] (hardcoded map or ListMetrics).
func (p *SchemaProvider) dimensionColumnNamesForNamespace(ctx context.Context, region string, accountId *string, namespace string) ([]string, error) {
	cols, err := p.dimensionColumnsForNamespace(ctx, region, accountId, namespace)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	return names, nil
}

// getRegionNames fetches the list of available AWS regions and returns their names.
func (p *SchemaProvider) getRegionNames(ctx context.Context) ([]string, error) {
	service, err := p.ds.GetRegionsService(ctx, defaultRegion)
	if err != nil {
		return nil, err
	}
	responses, err := service.GetRegions(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(responses))
	for _, r := range responses {
		names = append(names, r.Value.Name)
	}
	return names, nil
}
