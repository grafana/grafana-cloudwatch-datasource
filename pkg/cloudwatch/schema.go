package cloudwatch

import (
	"context"
	"sort"
	"strings"

	"github.com/grafana/grafana-aws-sdk/pkg/cloudWatchConsts"
	schemas "github.com/grafana/schemads"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models/resources"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/services"
)

var (
	dimensionOperators = []schemas.Operator{
		schemas.OperatorEquals,
		schemas.OperatorIn,
	}

	tableParameters = []schemas.TableParameter{
		{Name: "region", Root: true, Required: true},
		{Name: "accountId", DependsOn: []string{"region"}, Required: false},
	}
)

// SchemaProvider implements the schemads SchemaHandler, TablesHandler,
// ColumnsHandler, and TableParameterValuesHandler interfaces for CloudWatch
// metrics. Tables are identified as "namespace.metricName" (e.g.
// "AWS/EC2.CPUUtilization"); columns are dimension keys.
type SchemaProvider struct {
	ds *DataSource
}

// NewSchemaProvider creates a new SchemaProvider backed by ds.
func NewSchemaProvider(ds *DataSource) *SchemaProvider {
	return &SchemaProvider{ds: ds}
}

const metricsTablePrefix = "metrics."

// splitTableName parses a three-part "metrics.namespace.metricName" table
// identifier and returns the namespace and metricName components.
// The "metrics." prefix distinguishes metrics tables from any future logs
// tables (e.g. "logs./aws/lambda/fn") and enables unambiguous query routing.
// Returns ("", "") if the string does not start with the metrics prefix.
func splitTableName(table string) (namespace, metricName string) {
	if !strings.HasPrefix(table, metricsTablePrefix) {
		return "", ""
	}
	inner := table[len(metricsTablePrefix):]
	if i := strings.Index(inner, "."); i >= 0 {
		return inner[:i], inner[i+1:]
	}
	return inner, ""
}

// Schema implements schemas.SchemaHandler.
func (p *SchemaProvider) Schema(ctx context.Context, _ *schemas.SchemaRequest) (*schemas.SchemaResponse, error) {
	// Namespace errors are non-fatal for the full schema; return whatever succeeded.
	tables, _ := p.getAllTables(ctx)

	// Pre-populate the root "region" table parameter values for every table.
	// A failure here is also non-fatal; the schema is still usable without pre-populated regions.
	var tableParamValues map[string]map[string][]string
	if regionNames, err := p.getRegionNames(ctx); err == nil && len(regionNames) > 0 {
		tableParamValues = make(map[string]map[string][]string, len(tables))
		for _, t := range tables {
			tableParamValues[t.Name] = map[string][]string{"region": regionNames}
		}
	}

	return &schemas.SchemaResponse{
		FullSchema: &schemas.Schema{
			Tables:               tables,
			TableParameterValues: tableParamValues,
		},
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
	cols := make(map[string][]schemas.Column, len(req.Tables))
	errs := make(map[string]string)
	for _, tableName := range req.Tables {
		if tableName == "" {
			continue
		}
		namespace, _ := splitTableName(tableName)
		dimCols, err := p.dimensionColumnsForNamespace(ctx, namespace)
		if err != nil {
			errs[tableName] = err.Error()
			continue
		}
		cols[tableName] = dimCols
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
		dimCols, err := p.dimensionColumnsForNamespace(ctx, namespace)
		if err != nil {
			errs[namespace] = err.Error()
			continue
		}
		for _, metric := range metrics {
			tables = append(tables, schemas.Table{
				Name:            metricsTablePrefix + namespace + "." + metric,
				TableParameters: tableParameters,
				Columns:         dimCols,
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
			dimCols, err := p.dimensionColumnsForNamespace(ctx, customNS)
			if err != nil {
				errs[customNS] = err.Error()
				continue
			}
			tables = append(tables, schemas.Table{
				Name:            metricsTablePrefix + customNS + ".",
				TableParameters: tableParameters,
				Columns:         dimCols,
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
//
// Note: unlike the existing /dimension-values HTTP endpoint, this handler does
// not support narrowing results via an existing dimension filter, because
// ColumnValuesRequest has no field for that. Values returned are therefore
// all known values for the dimension key across the metric, not scoped to any
// other already-selected dimensions.
func (p *SchemaProvider) ColumnValues(ctx context.Context, req *schemas.ColumnValuesRequest) (*schemas.ColumnValuesResponse, error) {
	columnValues := make(map[string][]string, len(req.Columns))
	errors := make(map[string]string)

	namespace, metricName := splitTableName(req.Table)
	if namespace == "" {
		return &schemas.ColumnValuesResponse{ColumnValues: columnValues}, nil
	}

	region := req.TableParameters["region"]
	if region == "" {
		return &schemas.ColumnValuesResponse{ColumnValues: columnValues}, nil
	}

	service, err := p.ds.GetListMetricsService(ctx, region)
	if err != nil {
		return &schemas.ColumnValuesResponse{
			ColumnValues: columnValues,
			Errors:       map[string]string{"": err.Error()},
		}, nil
	}

	var accountId *string
	if id := req.TableParameters["accountId"]; id != "" {
		accountId = &id
	}
	resourceReq := &resources.ResourceRequest{
		Region:    region,
		AccountId: accountId,
	}

	for _, col := range req.Columns {
		values, err := service.GetDimensionValuesByDimensionFilter(ctx, resources.DimensionValuesRequest{
			ResourceRequest: resourceReq,
			Namespace:       namespace,
			MetricName:      metricName,
			DimensionKey:    col,
			DimensionFilter: []*resources.Dimension{},
		})
		if err != nil {
			errors[col] = err.Error()
			continue
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = v.Value
		}
		columnValues[col] = strs
	}

	return &schemas.ColumnValuesResponse{ColumnValues: columnValues, Errors: errors}, nil
}

// dimensionColumnsForNamespace returns the dimension keys for namespace as
// schemas.Column values with equality operators. For namespaces not present
// in the hardcoded map (e.g. custom namespaces), it falls back to a
// ListMetrics API call using the datasource's configured default region.
func (p *SchemaProvider) dimensionColumnsForNamespace(ctx context.Context, namespace string) ([]schemas.Column, error) {
	dimKeyResponses, err := services.GetHardCodedDimensionKeysByNamespace(namespace)
	if err != nil {
		// Not a known AWS namespace — query ListMetrics for the custom namespace.
		svc, svcErr := p.ds.GetListMetricsService(ctx, defaultRegion)
		if svcErr != nil {
			return nil, svcErr
		}
		dimKeyResponses, err = svc.GetDimensionKeysByDimensionFilter(ctx, resources.DimensionKeysRequest{
			ResourceRequest: &resources.ResourceRequest{Region: defaultRegion},
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
