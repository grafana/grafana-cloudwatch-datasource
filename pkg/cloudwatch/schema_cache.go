package cloudwatch

import (
	"fmt"
	"sort"

	"github.com/patrickmn/go-cache"
)

func schemaMetricsCacheKey(region, accountKey, namespace string) string {
	return fmt.Sprintf("schema-metrics|%s|%s|%s", region, accountKey, namespace)
}

func schemaDimKeysCacheKey(region, accountKey, namespace string) string {
	return fmt.Sprintf("schema-dimkeys|%s|%s|%s", region, accountKey, namespace)
}

// schemaMetadataAccountKey normalizes optional account id for cache keys
// (empty string when unset; preserves literal "all" for linked accounts).
func schemaMetadataAccountKey(accountId *string) string {
	if accountId == nil || *accountId == "" {
		return ""
	}
	return *accountId
}

// getOrSetSchemaMetadataStrings caches non-empty []string schema discovery results
// (metric names, dimension keys) under cacheKey. fetch is only invoked on miss;
// values are sorted before store so hits return a stable order.
func (ds *DataSource) getOrSetSchemaMetadataStrings(cacheKey string, fetch func() ([]string, error)) ([]string, error) {
	if ds.schemaMetadataCache == nil {
		return fetch()
	}
	if v, ok := ds.schemaMetadataCache.Get(cacheKey); ok {
		out := v.([]string)
		return append([]string(nil), out...), nil
	}
	raw, err := fetch()
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return append([]string(nil), raw...), nil
	}
	sorted := append([]string(nil), raw...)
	sort.Strings(sorted)
	stored := append([]string(nil), sorted...)
	ds.schemaMetadataCache.Set(cacheKey, stored, cache.DefaultExpiration)
	return append([]string(nil), sorted...), nil
}

func (ds *DataSource) getOrSetSchemaMetricNames(region, accountKey, namespace string, fetch func() ([]string, error)) ([]string, error) {
	return ds.getOrSetSchemaMetadataStrings(schemaMetricsCacheKey(region, accountKey, namespace), fetch)
}

func (ds *DataSource) getOrSetSchemaDimensionKeys(region, accountKey, namespace string, fetch func() ([]string, error)) ([]string, error) {
	return ds.getOrSetSchemaMetadataStrings(schemaDimKeysCacheKey(region, accountKey, namespace), fetch)
}
