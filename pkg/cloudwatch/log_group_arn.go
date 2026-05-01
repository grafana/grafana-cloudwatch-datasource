package cloudwatch

import "strings"

// LogsAccountSelfSentinel is the required accountId value for the virtual logs
// table when querying the datasource's own account (non–monitoring-account or
// same-account scope). It is omitted when building AWS ResourceRequest.AccountId.
const LogsAccountSelfSentinel = "self"

// LogGroupTableParameterSeparator separates log group name and ARN in the
// combined `logGroup` schemads table parameter. `|` matches the delimiter used in
// metrics table ids (e.g. metrics|AWS/EC2). It cannot appear in log group names
// ([.\-_/#A-Za-z0-9]+) or in standard CloudWatch Logs ARNs.
const LogGroupTableParameterSeparator = "|"

// FormatLogGroupTableParameter builds the wire value for LogGroupTableParameter.
// Either side may be empty (e.g. name-only from manual entry); callers listing
// API results typically pass both non-empty.
func FormatLogGroupTableParameter(name, arn string) string {
	return name + LogGroupTableParameterSeparator + arn
}

// ParseLogGroupTableParameter decodes LogGroupTableParameter values produced by
// FormatLogGroupTableParameter (name|arn), or a bare log group name. A bare log
// group ARN alone is invalid — the name must be provided by the caller.
func ParseLogGroupTableParameter(v string) (name, arn string, ok bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", "", false
	}
	if strings.Contains(v, LogGroupTableParameterSeparator) {
		parts := strings.SplitN(v, LogGroupTableParameterSeparator, 2)
		name = strings.TrimSpace(parts[0])
		arn = strings.TrimSpace(parts[1])
		if name == "" && arn == "" {
			return "", "", false
		}
	} else if strings.HasPrefix(v, "arn:") {
		return "", "", false
	} else {
		name = v
	}
	if name == "" {
		return "", "", false
	}
	return name, arn, true
}

// logsTableAccountIDForAPI maps table parameter accountId to the pointer passed
// to DescribeLogGroups / GetLogGroupFields. Empty and LogsAccountSelfSentinel
// mean same-account (nil). Other values including "all" are passed through.
func logsTableAccountIDForAPI(accountKey string) *string {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" || accountKey == LogsAccountSelfSentinel {
		return nil
	}
	return &accountKey
}
