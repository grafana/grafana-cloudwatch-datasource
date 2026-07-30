// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...require('./.config/jest.config'),
  // Override the scaffolded react-inlinesvg mock with our own copy so it survives
  // `@grafana/create-plugin` scaffolding updates resetting .config/jest/mocks/react-inlinesvg.tsx
  moduleNameMapper: {
    ...require('./.config/jest.config').moduleNameMapper,
    'react-inlinesvg': require('path').resolve(__dirname, 'jest', 'mocks', 'react-inlinesvg.tsx'),
  },
  // Ensure ESM-only packages are transformed (e.g., monaco-editor)
  transformIgnorePatterns: [
    require('./.config/jest/utils').nodeModulesToTransform([
      ...require('./.config/jest/utils').grafanaESModules,
      '@grafana/plugin-ui',
      'monaco-editor',
      'marked',
      'react-calendar',
      'get-user-locale',
      'memoize',
      'mimic-function',
      '@wojtekmaj',
      '@grafana/aws-sdk',
      '@grafana/prometheus',
      '@prometheus-io/lezer-promql',
      'monaco-promql',
      '@lezer',
      '@marcbachmann/cel-js',
    ]),
  ],
};
