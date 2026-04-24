package resources

// DimensionValuesForKeysRequest is used to fetch values for multiple dimension
// keys in a single ListMetrics call. Unlike DimensionValuesRequest it carries
// multiple DimensionKeys rather than a single DimensionKey.
type DimensionValuesForKeysRequest struct {
	*ResourceRequest
	Namespace       string
	MetricName      string
	DimensionKeys   []string
	DimensionFilter []*Dimension
}
