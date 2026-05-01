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
// FormatLogGroupTableParameter, or a bare log group name, or a bare log group ARN.
func ParseLogGroupTableParameter(v string) (name, arn string, ok bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", "", false
	}
	sep := LogGroupTableParameterSeparator
	if strings.Contains(v, sep) {
		parts := strings.SplitN(v, sep, 2)
		name = strings.TrimSpace(parts[0])
		arn = strings.TrimSpace(parts[1])
		if name == "" && arn == "" {
			return "", "", false
		}
	} else if strings.HasPrefix(v, "arn:") {
		arn = v
	} else {
		name = v
	}
	if name == "" && arn != "" {
		var nok bool
		name, nok = LogGroupNameFromARN(arn)
		if !nok {
			return "", "", false
		}
	}
	if name == "" {
		return "", "", false
	}
	return name, arn, true
}

// LogGroupNameFromARN extracts the log group name segment from a standard
// CloudWatch Logs group ARN (…:log-group:name or …:log-group:name:*).
// Log stream ARNs and other shapes are rejected.
func LogGroupNameFromARN(arn string) (string, bool) {
	arn = strings.TrimSpace(arn)
	if arn == "" {
		return "", false
	}
	const sep = ":log-group:"
	i := strings.Index(arn, sep)
	if i < 0 {
		return "", false
	}
	name := arn[i+len(sep):]
	if strings.Contains(name, ":log-stream:") {
		return "", false
	}
	name = strings.TrimSuffix(name, ":*")
	if name == "" {
		return "", false
	}
	return name, true
}

// logsTableAccountIDForAPI maps table parameter accountId to the pointer passed
// to DescribeLogGroups / GetLogGroupFields. Empty and LogsAccountSelfSentinel
// mean same-account (nil). Other values including "all" are passed through.
func logsTableAccountIDForAPI(accountKey string) *string {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" || accountKey == LogsAccountSelfSentinel {
		return nil
	}
	s := accountKey
	return &s
}
