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

// standardStatistics mirrors the frontend's standardStatistics list in
// src/standardStatistics.ts. Extended statistics (e.g. p99, TM(5:95)) are
// accepted as free-form input by the frontend and are not included here.
var standardStatistics = []string{"Average", "Maximum", "Minimum", "Sum", "SampleCount", "IQM"}

var (
	dimensionOperators = []schemas.Operator{
		schemas.OperatorEquals,
		schemas.OperatorIn,
	}

	// timeColumn and valueColumn are the two data columns present in every
	// metrics table. They are prepended to the column list so the SQL engine
	// exposes them as queryable columns (SELECT time, value FROM ...).
	// Neither has enumerable values, so ColumnValues skips them.
	timeColumn = schemas.Column{
		Name: "time",
		Type: schemas.ColumnTypeTimestamp,
	}
	valueColumn = schemas.Column{
		Name: "value",
		Type: schemas.ColumnTypeFloat64,
	}

	statisticColumn = schemas.Column{
		Name:      "statistic",
		Type:      schemas.ColumnTypeString,
		Operators: []schemas.Operator{schemas.OperatorEquals},
	}

	tableParameters = []schemas.TableParameter{
		{Name: "region", Root: true, Required: true},
		{Name: "accountId", DependsOn: []string{"region"}, Required: false},
	}
)

// SchemaProvider implements the schemads SchemaHandler, TablesHandler,
// ColumnsHandler, and TableParameterValuesHandler interfaces for CloudWatch
// metrics. Tables are identified as "metrics|<namespace>|<metricName>" (e.g.
// "metrics|AWS/EC2|CPUUtilization"); columns are dimension keys.
// The "|" separator is used throughout because both namespace names
// (e.g. "Custom.App") and metric names (e.g. Glue's
// "glue.driver.aggregate.bytesRead") may contain dots.
type SchemaProvider struct {
	ds *DataSource
}

// NewSchemaProvider creates a new SchemaProvider backed by ds.
func NewSchemaProvider(ds *DataSource) *SchemaProvider {
	return &SchemaProvider{ds: ds}
}

const metricsTablePrefix = "metrics|"

// splitTableName parses a "metrics|<namespace>|<metricName>" table identifier
// and returns the namespace and metricName components.
// The "metrics|" prefix distinguishes metrics tables from any future table
// types (e.g. "logs|/aws/lambda/fn") and enables unambiguous query routing.
// The "|" separator is used throughout because both namespace names
// (e.g. "Custom.App") and metric names (e.g. Glue's
// "glue.driver.aggregate.bytesRead") may contain dots.
// Returns ("", "") if the string does not start with the metrics prefix.
func splitTableName(table string) (namespace, metricName string) {
	if !strings.HasPrefix(table, metricsTablePrefix) {
		return "", ""
	}
	inner := table[len(metricsTablePrefix):]
	if i := strings.Index(inner, "|"); i >= 0 {
		return inner[:i], inner[i+1:]
	}
	return inner, ""
}

// Schema implements schemas.SchemaHandler.
func (p *SchemaProvider) Schema(ctx context.Context, _ *schemas.SchemaRequest) (*schemas.SchemaResponse, error) {
	// Namespace errors are non-fatal; return whatever tables succeeded alongside the error string.
	tables, tableErrs := p.getAllTables(ctx)

	// Pre-populate the root "region" table parameter values for every table.
	// A failure here is also non-fatal; the schema is still usable without pre-populated regions.
	var tableParamValues map[string]map[string][]string
	regionNames, regionErr := p.getRegionNames(ctx)
	if regionErr == nil && len(regionNames) > 0 {
		tableParamValues = make(map[string]map[string][]string, len(tables))
		for _, t := range tables {
			tableParamValues[t.Name] = map[string][]string{"region": regionNames}
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
func (p *SchemaProvider) Tables(ctx context.Context, _ *schemas.TablesRequest) (*schemas.TablesResponse, error) {
	tables, errs := p.getAllTables(ctx)

	names := make([]string, len(tables))
	tableParamMap := make(map[string][]schemas.TableParameter, len(tables))
	for i, t := range tables {
		names[i] = t.Name
		tableParamMap[t.Name] = t.TableParameters
	}

	return &schemas.TablesResponse{
		Tables:          names,
		TableParameters: tableParamMap,
		Errors:          errs,
	}, nil
}

// Columns implements schemas.ColumnsHandler.
func (p *SchemaProvider) Columns(ctx context.Context, req *schemas.ColumnsRequest) (*schemas.ColumnsResponse, error) {
	region := req.TableParameters["region"]
	if region == "" {
		region = defaultRegion
	}

	var accountId *string
	if id := req.TableParameters["accountId"]; id != "" {
		accountId = &id
	}

	cols := make(map[string][]schemas.Column, len(req.Tables))
	errs := make(map[string]string)
	for _, tableName := range req.Tables {
		if tableName == "" {
			continue
		}
		namespace, _ := splitTableName(tableName)
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
	result := make(map[string][]string)

	switch req.TableParameter {
	case "region":
		regionNames, err := p.getRegionNames(ctx)
		if err != nil {
			return nil, err
		}
		result["region"] = regionNames

	case "accountId":
		region := req.DependencyValues["region"]
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
		result["accountId"] = accountIds

	default:
		return &schemas.TableParametersValuesResponse{
			Errors: map[string]string{req.TableParameter: fmt.Sprintf("unsupported table parameter %q", req.TableParameter)},
		}, nil
	}

	return &schemas.TableParametersValuesResponse{TableParameterValues: result}, nil
}

// getAllTables returns a schemas.Table for every known namespace×metric
// combination, sorted by table name. Errors are collected per namespace and
// returned alongside whatever tables succeeded.
func (p *SchemaProvider) getAllTables(ctx context.Context) ([]schemas.Table, map[string]string) {
	var tables []schemas.Table
	errs := make(map[string]string)

	// Hardcoded AWS namespaces from cloudWatchConsts.
	for namespace, metrics := range cloudWatchConsts.NamespaceMetricsMap {
		dimCols, err := p.dimensionColumnsForNamespace(ctx, defaultRegion, nil, namespace)
		if err != nil {
			errs[namespace] = err.Error()
			continue
		}
		for _, metric := range metrics {
			tables = append(tables, schemas.Table{
				Name:            metricsTablePrefix + namespace + "|" + metric,
				TableParameters: tableParameters,
				Columns:         metricsColumns(dimCols),
			})
		}
	}

	// Custom namespaces from datasource settings.
	if p.ds.Settings.Namespace != "" {
		for _, customNS := range strings.Split(p.ds.Settings.Namespace, ",") {
			customNS = strings.TrimSpace(customNS)
			if customNS == "" {
				continue
			}
			// Custom namespaces have no hardcoded metrics — expose a single
			// placeholder table for the namespace itself so it is discoverable.
			dimCols, err := p.dimensionColumnsForNamespace(ctx, defaultRegion, nil, customNS)
			if err != nil {
				errs[customNS] = err.Error()
				continue
			}
			tables = append(tables, schemas.Table{
				Name:            metricsTablePrefix + customNS + "|",
				TableParameters: tableParameters,
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
	namespace, metricName := splitTableName(req.Table)
	if namespace == "" {
		return nil, fmt.Errorf("unrecognised table format %q: expected metrics|<namespace>|<metricName>", req.Table)
	}

	region := req.TableParameters["region"]
	if region == "" {
		return nil, fmt.Errorf("region is a required table parameter")
	}

	var accountId *string
	if id := req.TableParameters["accountId"]; id != "" {
		accountId = &id
	}

	columnValues := make(map[string][]string, len(req.Columns))

	// When no columns are specified, expand to all columns for the table:
	// dimension keys plus the statistic column. time and value are data
	// columns with no enumerable values so they are excluded from the
	// expansion but silently skipped when explicitly requested below.
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
		columns = append(dimNames, statisticColumn.Name)
	}

	// Separate statistic column (served from a fixed list) from dimension
	// columns (served via ListMetrics API). time and value are data columns
	// with no enumerable values; skip them silently.
	var dimensionCols []string
	for _, col := range columns {
		switch col {
		case statisticColumn.Name:
			columnValues[col] = standardStatistics
		case timeColumn.Name, valueColumn.Name:
			// no enumerable values for data columns
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

// metricsColumns assembles the full column list for a metrics table: the two
// fixed data columns (time, value) followed by the dimension key columns and
// the statistic filter column.
func metricsColumns(dimCols []schemas.Column) []schemas.Column {
	cols := make([]schemas.Column, 0, 2+len(dimCols)+1)
	cols = append(cols, timeColumn, valueColumn)
	cols = append(cols, dimCols...)
	cols = append(cols, statisticColumn)
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
