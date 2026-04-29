package models

import (
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/kinds/dataquery"
)

type LogsQuery struct {
	dataquery.CloudWatchLogsQuery
	// GrafanaSqlLogs marks queries rewritten from Grafana SQL (dsAbstraction) so QueryData
	// can route them through the synchronous logs execution path like alert/public-dashboard queries.
	GrafanaSqlLogs bool `json:"grafanaSqlLogs,omitempty"`
	StartTime      *int64
	EndTime        *int64
	Limit          *int32
	LogGroupName   string
	LogStreamName  string
	QueryId        string
	QueryString    string
	StartFromHead  bool
	Subtype        string
}
