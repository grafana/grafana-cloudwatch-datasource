import { defineConfig, globalIgnores } from 'eslint/config';
import { fixupConfigRules } from '@eslint/compat';
import reactPlugin from 'eslint-plugin-react';
import baseConfig from './.config/eslint.config.mjs';

export default defineConfig([
  globalIgnores(['**/node_modules', '**/build', '**/dist', '.yarn', '.config']),
  ...fixupConfigRules([...baseConfig, reactPlugin.configs.flat['jsx-runtime']]),
  {
    rules: {
      'react-hooks/immutability': 'off',
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/preserve-manual-memoization': 'off',
    },
  },
  {
    // The scaffolded base config only lints `src/**/*.{ts,tsx}` with type info; also cover js/jsx.
    files: ['src/**/*.{js,jsx}'],
    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
      },
    },
    rules: {
      '@typescript-eslint/no-deprecated': 'warn',
    },
  },
]);
