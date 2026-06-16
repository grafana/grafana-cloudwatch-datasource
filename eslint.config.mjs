import { defineConfig, globalIgnores } from 'eslint/config';
import { fixupConfigRules } from '@eslint/compat';
import { createRequire } from 'node:module';
import reactPlugin from 'eslint-plugin-react';

const require = createRequire(import.meta.url);
const grafanaConfig = require('@grafana/eslint-config/flat.js');

export default defineConfig([
  globalIgnores(['**/node_modules', '**/build', '**/dist', '.yarn']),
  ...fixupConfigRules([...grafanaConfig, reactPlugin.configs.flat['jsx-runtime']]),
  {
    rules: {
      'react/prop-types': 'off',
      'react-hooks/immutability': 'off',
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/preserve-manual-memoization': 'off',
    },
  },
  {
    files: ['src/**/*.{ts,tsx,js,jsx}'],
    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
      },
    },
    rules: {
      '@typescript-eslint/no-deprecated': 'warn',
    },
  },
  {
    files: ['tests/**/*'],
    rules: {
      'react-hooks/rules-of-hooks': 'off',
    },
  },
]);
