import type * as monacoType from 'monaco-editor/esm/vs/editor/editor.api';

// CloudWatch Logs: https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/CWL_QuerySyntax.html
interface CloudWatchLogsLanguage extends monacoType.languages.IMonarchLanguage {
  commands: string[];
  operators: string[];
  builtinFunctions: string[];
}

export const DIFF = 'diff';
export const DISPLAY = 'display';
export const FIELDS = 'fields';
export const FILTER = 'filter';
export const PATTERN = 'pattern';
export const STATS = 'stats';
export const SORT = 'sort';
export const LIMIT = 'limit';
export const PARSE = 'parse';
export const DEDUP = 'dedup';
export const ANOMALY = 'anomaly';
export const ADDTOTALS = 'addtotals';
export const RELEVANTFIELDS = 'relevantfields';
export const EXPAND = 'expand';
export const FILTERINDEX = 'filterIndex';
export const UNNEST = 'unnest';
export const UNMASK = 'unmask';
export const LOOKUP = 'lookup';
export const JOIN = 'join';
export const WHERE = 'where';
export const AUTOREGRESS = 'autoregress';
export const LOGCOMPARE = 'logcompare';
export const FILLDOWN = 'filldown';
export const FILLMISSING = 'fillmissing';
export const CIDRLOOKUP = 'cidrlookup';
export const OUTLIER = 'outlier';
export const ACCUM = 'accum';
export const APPENDCOLS = 'appendcols';
export const SESSIONIZE = 'sessionize';
export const COUNTFREQUENT = 'countFrequent';

export const LOGS_COMMANDS = [
  DISPLAY,
  FIELDS,
  FILTER,
  WHERE,
  PATTERN,
  STATS,
  SORT,
  LIMIT,
  PARSE,
  DEDUP,
  DIFF,
  ANOMALY,
  ADDTOTALS,
  RELEVANTFIELDS,
  EXPAND,
  FILTERINDEX,
  UNNEST,
  UNMASK,
  LOOKUP,
  JOIN,
  AUTOREGRESS,
  LOGCOMPARE,
  FILLDOWN,
  FILLMISSING,
  CIDRLOOKUP,
  OUTLIER,
  ACCUM,
  APPENDCOLS,
  SESSIONIZE,
  COUNTFREQUENT,
];

export const LOGS_LOGIC_OPERATORS = ['and', 'or', 'not'];

export const LOGS_FUNCTION_OPERATORS = [
  // math
  'abs',
  'ceil',
  'floor',
  'greatest',
  'least',
  'log',
  'sqrt',
  'round',
  'haversine',
  'toNumber',
  'toInt',
  'toLong',
  'toDouble',
  // datetime
  'bin',
  'datefloor',
  'dateceil',
  'fromMillis',
  'toMillis',
  'now',
  'parseDate',
  'formatDate',
  'strftime',
  // general
  'ispresent',
  'coalesce',
  'case',
  'if',
  'isNumeric',
  'messageSize',
  'queryStartTime',
  'queryEndTime',
  'queryTimeRange',
  // json
  'jsonParse',
  'jsonStringify',
  'jsonArraySize',
  'jsonArrayContains',
  // ip
  'isValidIp',
  'isValidIpV4',
  'isValidIpV6',
  'isIpInSubnet',
  'isIpv4InSubnet',
  'isIpv6InSubnet',
  'ipv4ToNumber',
  'isPrivateIP',
  'isPublicIP',
  'isReservedIP',
  // stats aggregation
  'avg',
  'count',
  'count_distinct',
  'max',
  'min',
  'pct',
  'stddev',
  'sum',
  // stats non-aggregation
  'earliest',
  'latest',
  'sortsFirst',
  'sortsLast',
  // strings
  'isempty',
  'isblank',
  'concat',
  'ltrim',
  'rtrim',
  'trim',
  'strlen',
  'toupper',
  'tolower',
  'substr',
  'replace',
  'regexReplace',
  'strcontains',
  'startsWith',
  'endsWith',
  'split',
  // encoding
  'urlencode',
  'urldecode',
  'base64encode',
  'base64decode',
  // hex
  'hexToAscii',
  'hexToDec',
  'decToHex',
];

export const SORT_DIRECTION_KEYWORDS = ['asc', 'desc'];
export const DIFF_MODIFIERS = ['previousDay', 'previousWeek', 'previousMonth'];
export const PARSE_FORMATS = ['logfmt'];
export const LOGS_KEYWORDS = [
  'like',
  'by',
  'in',
  'as',
  ...SORT_DIRECTION_KEYWORDS,
  ...DIFF_MODIFIERS,
  ...PARSE_FORMATS,
];

export const language: CloudWatchLogsLanguage = {
  defaultToken: 'invalid',
  id: 'logs',
  ignoreCase: true,
  brackets: [
    { open: '[', close: ']', token: 'delimiter.square' },
    { open: '(', close: ')', token: 'delimiter.parenthesis' },
  ],
  commands: [...LOGS_COMMANDS, ...LOGS_KEYWORDS],
  operators: LOGS_LOGIC_OPERATORS,
  builtinFunctions: LOGS_FUNCTION_OPERATORS,
  tokenizer: {
    root: [
      { include: '@comments' },
      { include: '@regexes' },
      { include: '@whitespace' },
      { include: '@fieldNames' },
      { include: '@variables' },
      { include: '@strings' },
      { include: '@numbers' },

      [/\|\|/, 'operator'],
      [/[,.:\|]/, 'delimiter'],
      [/[()\[\]]/, 'delimiter.parenthesis'],
      [
        /[\w@#$]+/,
        {
          cases: {
            '@commands': 'keyword',
            '@builtinFunctions': 'predefined',
            '@operators': 'operator',
            '@default': 'identifier',
          },
        },
      ],
      [/[+\-*/^%=!<>]/, 'operator'], // handles the math operators
    ],
    variables: [
      [/\${/, { token: 'variable', next: '@variable_bracket' }],
      [/\$[a-zA-Z0-9-_]+/, 'variable'],
    ],
    variable_bracket: [
      [/[a-zA-Z0-9-_:]+/, 'variable'],
      [/}/, { token: 'variable', next: '@pop' }],
    ],
    fieldNames: [[/(@[_a-zA-Z]+[_.0-9a-zA-Z]*)|(`((\\`)|([^`]))*?`)/, 'identifier']],
    whitespace: [[/\s+/, 'white']],
    comments: [
      [/^#.*/, 'comment'],
      [/\s+#.*/, 'comment'],
    ],
    numbers: [
      [/0[xX][0-9a-fA-F]*/, 'number'],
      [/[$][+-]*\d*(\.\d*)?/, 'number'],
      [/((\d+(\.\d*)?)|(\.\d+))([eE][\-+]?\d+)?/, 'number'],
    ],
    strings: [
      [/'/, { token: 'string', next: '@string' }],
      [/"/, { token: 'string', next: '@string_double' }],
      [/`/, { token: 'identifier', next: '@string_backtick' }],
    ],
    string: [
      [/[^']+/, 'string'],
      [/''/, 'string'],
      [/'/, { token: 'string', next: '@pop' }],
    ],
    string_double: [
      [/[^\\"]+/, 'string'],
      [/"/, 'string', '@pop'],
    ],
    string_backtick: [
      [/[^\\`]+/, 'identifier'],
      [/`/, 'identifier', '@pop'],
    ],
    regexes: [[/\/.*?\/(?=\s*\||\s*$|,)/, 'regexp']],
  },
};

export const conf: monacoType.languages.LanguageConfiguration = {
  comments: {
    lineComment: '#',
  },
  brackets: [
    ['{', '}'],
    ['[', ']'],
    ['(', ')'],
  ],
  autoClosingPairs: [
    { open: '{', close: '}' },
    { open: '[', close: ']' },
    { open: '(', close: ')' },
    { open: '"', close: '"' },
    { open: "'", close: "'" },
    { open: '`', close: '`' },
  ],
  surroundingPairs: [
    { open: '{', close: '}' },
    { open: '[', close: ']' },
    { open: '(', close: ')' },
    { open: '"', close: '"' },
    { open: "'", close: "'" },
    { open: '`', close: '`' },
  ],
};
