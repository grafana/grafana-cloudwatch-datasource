import { Grammar } from 'prismjs';

import { CompletionItem } from '@grafana/ui';

export const QUERY_COMMANDS: CompletionItem[] = [
  {
    label: 'fields',
    documentation: 'Retrieves the specified fields from log events',
  },
  { label: 'display', documentation: 'Specifies which fields to display in the query results' },
  {
    label: 'filter',
    documentation: 'Filters the results of a query based on one or more conditions',
  },
  {
    label: 'where',
    documentation: 'Alias for the filter command; accepts identical syntax.',
  },
  {
    label: 'stats',
    documentation: 'Calculates aggregate statistics based on the values of log fields',
  },
  { label: 'sort', documentation: 'Sorts the retrieved log events' },
  { label: 'limit', documentation: 'Specifies the number of log events returned by the query' },
  {
    label: 'parse',
    documentation:
      'Extracts data from a log field, creating one or more ephemeral fields that you can process further in the query',
  },
  {
    label: 'pattern',
    documentation: 'Automatically clusters your log data into shared text-structure patterns.',
  },
  {
    label: 'dedup',
    documentation: 'Removes duplicate results based on the values of the fields you specify.',
  },
  {
    label: 'diff',
    documentation:
      'Compares the log events in the requested time period with those from a previous period of equal length.',
  },
  {
    label: 'anomaly',
    documentation: 'Identifies unusual patterns in your log data using machine learning.',
  },
  {
    label: 'addtotals',
    documentation: 'Computes row totals and column totals for numeric fields.',
  },
  {
    label: 'relevantfields',
    documentation: 'Displays the most relevant fields for filtered results. Must follow a filter clause.',
  },
  {
    label: 'expand',
    documentation: 'Expands nested JSON fields into top-level queryable fields.',
  },
  {
    label: 'filterIndex',
    documentation:
      'Restricts the scan to log groups indexed on the specified field and containing its value, reducing scanned volume.',
  },
  {
    label: 'unnest',
    documentation: 'Flattens a list into multiple records, producing one record per element in the list.',
  },
  {
    label: 'unmask',
    documentation: 'Displays all content of a log event that was masked by a data protection policy.',
  },
  {
    label: 'lookup',
    documentation: 'Enriches log events with data from a lookup table by matching field values.',
  },
  {
    label: 'join',
    documentation:
      'Combines log events from a source log group with events from another log group or query result on a matching field.',
  },
  {
    label: 'autoregress',
    documentation: "Creates lagged (previous-row) copies of a field's values.",
  },
  {
    label: 'logcompare',
    documentation: 'Compares the current time window against a baseline window shifted back by a duration.',
  },
  {
    label: 'filldown',
    documentation: 'Carries the last non-null value forward to fill gaps.',
  },
  {
    label: 'fillmissing',
    documentation: 'Inserts rows for empty time bins after stats by bin(), optionally filling fields with a constant.',
  },
  {
    label: 'cidrlookup',
    documentation: 'Enriches events by matching an IP field against CIDR ranges in a lookup table.',
  },
  {
    label: 'outlier',
    documentation: 'Detects statistical outliers based on the interquartile range (IQR).',
  },
  {
    label: 'accum',
    documentation: 'Computes a running cumulative sum of a numeric field.',
  },
  {
    label: 'appendcols',
    documentation: 'Appends columns from a sub-query to the current results by positional row matching.',
  },
  {
    label: 'sessionize',
    documentation: 'Groups events into sessions by identity fields and inactivity gap.',
  },
  {
    label: 'countFrequent',
    documentation: 'Returns an approximate count of each unique field-value combination, sorted in descending order.',
  },
];

export const COMPARISON_OPERATORS = ['=', '!=', '<', '<=', '>', '>='];
export const ARITHMETIC_OPERATORS = ['+', '-', '*', '/', '^', '%'];

export const NUMERIC_OPERATORS = [
  {
    label: 'abs',
    detail: 'abs(a)',
    documentation: 'Absolute value.',
  },
  {
    label: 'ceil',
    detail: 'ceil(a)',
    documentation: 'Round to ceiling (the smallest integer that is greater than the value of a).',
  },
  {
    label: 'floor',
    detail: 'floor(a)',
    documentation: 'Round to floor (the largest integer that is smaller than the value of a).',
  },
  {
    label: 'greatest',
    detail: 'greatest(a,b, ... z)',
    documentation: 'Returns the largest value.',
  },
  {
    label: 'least',
    detail: 'least(a, b, ... z)',
    documentation: 'Returns the smallest value.',
  },
  {
    label: 'log',
    detail: 'log(a)',
    documentation: 'Natural logarithm.',
  },
  {
    label: 'sqrt',
    detail: 'sqrt(a)',
    documentation: 'Square root.',
  },
  {
    label: 'round',
    detail: 'round(a [, d])',
    documentation:
      'Rounds the value of a. With one argument, rounds to the nearest integer; with two arguments, rounds to d decimal places.',
  },
  {
    label: 'haversine',
    detail: 'haversine(lat1, lon1, lat2, lon2)',
    documentation:
      'Computes the great-circle distance in kilometers between two geographic points given as latitude/longitude in degrees.',
  },
  {
    label: 'toNumber',
    detail: 'toNumber(fieldname)',
    documentation: 'Converts a string field value to a numeric value.',
  },
  {
    label: 'toInt',
    detail: 'toInt(fieldname)',
    documentation: 'Converts a field value to a 32-bit integer.',
  },
  {
    label: 'toLong',
    detail: 'toLong(fieldname)',
    documentation: 'Converts a field value to a 64-bit long integer.',
  },
  {
    label: 'toDouble',
    detail: 'toDouble(fieldname)',
    documentation: 'Converts a field value to a double-precision floating point number.',
  },
];

export const GENERAL_FUNCTIONS = [
  {
    label: 'ispresent',
    detail: 'ispresent(fieldname)',
    documentation: 'Returns true if the field exists.',
  },
  {
    label: 'coalesce',
    detail: 'coalesce(fieldname1, fieldname2, ... fieldnamex)',
    documentation: 'Returns the first non-null value from the list.',
  },
  {
    label: 'case',
    detail: 'case(cond1, val1, cond2, val2, ..., [default])',
    documentation:
      'Evaluates conditions in order and returns the value for the first true condition. Returns the optional default if none match. Supports up to 10 branches.',
  },
  {
    label: 'if',
    detail: 'if(condition, trueValue, falseValue)',
    documentation: 'Returns trueValue if the condition is true, otherwise falseValue.',
  },
  {
    label: 'isNumeric',
    detail: 'isNumeric(fieldname)',
    documentation: 'Returns true if the field value can be parsed as a number.',
  },
  {
    label: 'messageSize',
    detail: 'messageSize(fieldname)',
    documentation: 'Returns the byte length of a string field.',
  },
  {
    label: 'queryStartTime',
    detail: 'queryStartTime()',
    documentation: 'Returns the query window start time as epoch milliseconds.',
  },
  {
    label: 'queryEndTime',
    detail: 'queryEndTime()',
    documentation: 'Returns the query window end time as epoch milliseconds.',
  },
  {
    label: 'queryTimeRange',
    detail: 'queryTimeRange()',
    documentation: 'Returns the query window duration in milliseconds.',
  },
];

export const STRING_FUNCTIONS = [
  {
    label: 'isempty',
    detail: 'isempty(fieldname)',
    documentation: 'Returns true if the field is missing or is an empty string.',
  },
  {
    label: 'isblank',
    detail: 'isblank(fieldname)',
    documentation: 'Returns true if the field is missing, an empty string, or contains only white space.',
  },
  {
    label: 'concat',
    detail: 'concat(string1, string2, ... stringz)',
    documentation: 'Concatenates the strings.',
  },
  {
    label: 'ltrim',
    detail: 'ltrim(string) or ltrim(string1, string2)',
    documentation:
      'Remove white space from the left of the string. If the function has a second string argument, it removes the characters of string2 from the left of string1.',
  },
  {
    label: 'rtrim',
    detail: 'rtrim(string) or rtrim(string1, string2)',
    documentation:
      'Remove white space from the right of the string. If the function has a second string argument, it removes the characters of string2 from the right of string1.',
  },
  {
    label: 'trim',
    detail: 'trim(string) or trim(string1, string2)',
    documentation:
      'Remove white space from both ends of the string. If the function has a second string argument, it removes the characters of string2 from both sides of string1.',
  },
  {
    label: 'strlen',
    detail: 'strlen(string)',
    documentation: 'Returns the length of the string in Unicode code points.',
  },
  {
    label: 'toupper',
    detail: 'toupper(string)',
    documentation: 'Converts the string to uppercase.',
  },
  {
    label: 'tolower',
    detail: 'tolower(string)',
    documentation: 'Converts the string to lowercase.',
  },
  {
    label: 'substr',
    detail: 'substr(string1, x), or substr(string1, x, y)',
    documentation:
      'Returns a substring from the index specified by the number argument to the end of the string. If the function has a second number argument, it contains the length of the substring to be retrieved.',
  },
  {
    label: 'replace',
    detail: 'replace(string1, string2, string3)',
    documentation: 'Replaces all instances of string2 in string1 with string3.',
  },
  {
    label: 'regexReplace',
    detail: 'regexReplace(fieldname, pattern, replacement)',
    documentation:
      'Replaces all substrings matching the regular expression pattern with replacement. Uses RE2 regex syntax.',
  },
  {
    label: 'strcontains',
    detail: 'strcontains(string1, string2)',
    documentation: 'Returns 1 if string1 contains string2 and 0 otherwise.',
  },
  {
    label: 'startsWith',
    detail: 'startsWith(string, searchValue)',
    documentation: 'Returns 1 if string starts with searchValue and 0 otherwise.',
  },
  {
    label: 'endsWith',
    detail: 'endsWith(string, searchValue)',
    documentation: 'Returns 1 if string ends with searchValue and 0 otherwise.',
  },
  {
    label: 'split',
    detail: 'split(string, delimiter)',
    documentation: 'Splits a string by the specified delimiter and returns an array of substrings.',
  },
  {
    label: 'urlencode',
    detail: 'urlencode(string)',
    documentation: 'URL-encodes the string.',
  },
  {
    label: 'urldecode',
    detail: 'urldecode(string)',
    documentation: 'URL-decodes the string.',
  },
  {
    label: 'base64encode',
    detail: 'base64encode(string)',
    documentation: 'Base64-encodes the string.',
  },
  {
    label: 'base64decode',
    detail: 'base64decode(string)',
    documentation: 'Base64-decodes the string.',
  },
  {
    label: 'hexToAscii',
    detail: 'hexToAscii(value)',
    documentation: 'Converts a hexadecimal string to ASCII text.',
  },
  {
    label: 'hexToDec',
    detail: 'hexToDec(value)',
    documentation: 'Converts a hexadecimal string to a decimal integer.',
  },
  {
    label: 'decToHex',
    detail: 'decToHex(value)',
    documentation:
      'Converts a decimal integer to a lowercase hex string with a 0x prefix. Negative numbers produce a -0x prefix; non-integers are truncated.',
  },
];

export const DATETIME_FUNCTIONS = [
  {
    label: 'bin',
    detail: 'bin(period)',
    documentation: 'Rounds the value of @timestamp to the given period and then truncates.',
  },
  {
    label: 'datefloor',
    detail: 'datefloor(a, period)',
    documentation: 'Truncates the timestamp to the given period.',
  },
  {
    label: 'dateceil',
    detail: 'dateceil(a, period)',
    documentation: 'Rounds up the timestamp to the given period and then truncates.',
  },
  {
    label: 'fromMillis',
    detail: 'fromMillis(fieldname)',
    documentation:
      'Interprets the input field as the number of milliseconds since the Unix epoch and converts it to a timestamp.',
  },
  {
    label: 'toMillis',
    detail: 'toMillis(fieldname)',
    documentation:
      'Converts the timestamp found in the named field into a number representing the milliseconds since the Unix epoch.',
  },
  {
    label: 'now',
    detail: 'now()',
    documentation: 'Returns the time that query processing started, in epoch seconds.',
  },
  {
    label: 'parseDate',
    detail: 'parseDate(fieldname, format [, timezone])',
    documentation: 'Parses a date string to epoch milliseconds using a Java DateTimeFormatter pattern.',
  },
  {
    label: 'formatDate',
    detail: 'formatDate(timestamp, format [, timezone])',
    documentation: 'Formats a timestamp using a strftime-style format string. Also available as the alias strftime.',
  },
  {
    label: 'strftime',
    detail: 'strftime(timestamp, format [, timezone])',
    documentation: 'Alias for formatDate. Formats a timestamp using a strftime-style format string.',
  },
];

export const IP_FUNCTIONS = [
  {
    label: 'isValidIp',
    detail: 'isValidIp(fieldname)',
    documentation: 'Returns true if the field is a valid v4 or v6 IP address.',
  },
  {
    label: 'isValidIpV4',
    detail: 'isValidIpV4(fieldname)',
    documentation: 'Returns true if the field is a valid v4 IP address.',
  },
  {
    label: 'isValidIpV6',
    detail: 'isValidIpV6(fieldname)',
    documentation: 'Returns true if the field is a valid v6 IP address.',
  },
  {
    label: 'isIpInSubnet',
    detail: 'isIpInSubnet(fieldname, string)',
    documentation: 'Returns true if the field is a valid v4 or v6 IP address within the specified v4 or v6 subnet.',
  },
  {
    label: 'isIpv4InSubnet',
    detail: 'isIpv4InSubnet(fieldname, string)',
    documentation: 'Returns true if the field is a valid v4 IP address within the specified v4 subnet.',
  },
  {
    label: 'isIpv6InSubnet',
    detail: 'isIpv6InSubnet(fieldname, string)',
    documentation: 'Returns true if the field is a valid v6 IP address within the specified v6 subnet.',
  },
  {
    label: 'ipv4ToNumber',
    detail: 'ipv4ToNumber(fieldname)',
    documentation: 'Converts an IPv4 address string to its numeric representation.',
  },
  {
    label: 'isPrivateIP',
    detail: 'isPrivateIP(fieldname)',
    documentation: 'Returns true if the IP address is in a private range (RFC 1918).',
  },
  {
    label: 'isPublicIP',
    detail: 'isPublicIP(fieldname)',
    documentation: 'Returns true if the IP address is publicly routable.',
  },
  {
    label: 'isReservedIP',
    detail: 'isReservedIP(fieldname)',
    documentation: 'Returns true if the IP address is in a reserved range.',
  },
];

export const JSON_FUNCTIONS = [
  {
    label: 'jsonParse',
    detail: 'jsonParse(fieldname)',
    documentation:
      'Returns a map or list when the input is a string representation of a JSON object or array; returns empty otherwise.',
  },
  {
    label: 'jsonStringify',
    detail: 'jsonStringify(fieldname)',
    documentation: 'Returns a JSON string from a map or list.',
  },
  {
    label: 'jsonArraySize',
    detail: 'jsonArraySize(fieldname)',
    documentation: 'Returns the element count of a JSON array string field.',
  },
  {
    label: 'jsonArrayContains',
    detail: 'jsonArrayContains(fieldname, value)',
    documentation: 'Returns true if the JSON array in the field contains the specified value (case-sensitive).',
  },
];

export const BOOLEAN_FUNCTIONS = [
  {
    label: 'ispresent',
    detail: 'ispresent(fieldname)',
    documentation: 'Returns true if the field exists.',
  },
  {
    label: 'isempty',
    detail: 'isempty(fieldname)',
    documentation: 'Returns true if the field is missing or is an empty string.',
  },
  {
    label: 'isblank',
    detail: 'isblank(fieldname)',
    documentation: 'Returns true if the field is missing, an empty string, or contains only white space.',
  },
  {
    label: 'isNumeric',
    detail: 'isNumeric(fieldname)',
    documentation: 'Returns true if the field value can be parsed as a number.',
  },
  {
    label: 'strcontains',
    detail: 'strcontains(string1, string2)',
    documentation: 'Returns 1 if string1 contains string2 and 0 otherwise.',
  },
  ...IP_FUNCTIONS,
];

export const AGGREGATION_FUNCTIONS_STATS = [
  {
    label: 'avg',
    detail: 'avg(NumericFieldname)',
    documentation: 'The average of the values in the specified field.',
  },
  {
    label: 'count',
    detail: 'count(fieldname) or count(*)',
    documentation: 'Counts the log records.',
  },
  {
    label: 'count_distinct',
    detail: 'count_distinct(fieldname)',
    documentation: 'Returns the number of unique values for the field.',
  },
  {
    label: 'max',
    detail: 'max(fieldname)',
    documentation: 'The maximum of the values for this log field in the queried logs.',
  },
  {
    label: 'min',
    detail: 'min(fieldname)',
    documentation: 'The minimum of the values for this log field in the queried logs.',
  },
  {
    label: 'pct',
    detail: 'pct(fieldname, value)',
    documentation: 'A percentile indicates the relative standing of a value in a datas.',
  },
  {
    label: 'stddev',
    detail: 'stddev(NumericFieldname)',
    documentation: 'The standard deviation of the values in the specified field.',
  },
  {
    label: 'sum',
    detail: 'sum(NumericFieldname)',
    documentation: 'The sum of the values in the specified field.',
  },
];

export const NON_AGGREGATION_FUNCS_STATS = [
  {
    label: 'earliest',
    detail: 'earliest(fieldname)',
    documentation:
      'Returns the value of fieldName from the log event that has the earliest time stamp in the queried logs.',
  },
  {
    label: 'latest',
    detail: 'latest(fieldname)',
    documentation:
      'Returns the value of fieldName from the log event that has the latest time stamp in the queried logs.',
  },
  {
    label: 'sortsFirst',
    detail: 'sortsFirst(fieldname)',
    documentation: 'Returns the value of fieldName that sorts first in the queried logs.',
  },
  {
    label: 'sortsLast',
    detail: 'sortsLast(fieldname)',
    documentation: 'Returns the value of fieldName that sorts last in the queried logs.',
  },
];

export const STATS_FUNCS = [...AGGREGATION_FUNCTIONS_STATS, ...NON_AGGREGATION_FUNCS_STATS];

export const KEYWORDS = ['as', 'like', 'by', 'in', 'desc', 'asc'];
export const FIELD_AND_FILTER_FUNCTIONS = [
  ...NUMERIC_OPERATORS,
  ...GENERAL_FUNCTIONS,
  ...STRING_FUNCTIONS,
  ...DATETIME_FUNCTIONS,
  ...IP_FUNCTIONS,
  ...JSON_FUNCTIONS,
];

export const FUNCTIONS = [...FIELD_AND_FILTER_FUNCTIONS, ...STATS_FUNCS];

const tokenizer: Grammar = {
  comment: {
    pattern: /^#.*/,
    greedy: true,
  },
  backticks: {
    pattern: /`.*?`/,
    alias: 'string',
    greedy: true,
  },
  quote: {
    pattern: /".*?"/,
    alias: 'string',
    greedy: true,
  },
  regex: {
    pattern: /\/.*?\/(?=\||\s*$|,)/,
    greedy: true,
  },
  'query-command': {
    pattern: new RegExp(`\\b(?:${QUERY_COMMANDS.map((command) => command.label).join('|')})\\b`, 'i'),
    alias: 'function',
  },
  function: {
    pattern: new RegExp(`\\b(?:${FUNCTIONS.map((f) => f.label).join('|')})\\b`, 'i'),
  },
  keyword: {
    pattern: new RegExp(`(\\s+)(${KEYWORDS.join('|')})(?=\\s+)`, 'i'),
    lookbehind: true,
  },
  // 'log-group-name': {
  //   pattern: /[\.\-_/#A-Za-z0-9]+/,
  // },
  'field-name': {
    pattern: /(@?[_a-zA-Z]+[_.0-9a-zA-Z]*)|(`((\\`)|([^`]))*?`)/,
    greedy: true,
  },
  number: /\b-?\d+((\.\d*)?([eE][+-]?\d+)?)?\b/,
  'command-separator': {
    pattern: /\|/,
    alias: 'punctuation',
  },
  'comparison-operator': {
    pattern: /([<>]=?)|(!?=)/,
  },
  punctuation: /[{}()`,.]/,
  whitespace: /\s+/,
};

export default tokenizer;
