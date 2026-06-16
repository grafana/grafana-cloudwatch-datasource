import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';

import { CustomVariableModel } from '@grafana/data';
import { monacoTypes } from '@grafana/ui';

import { setupMockedTemplateService, logGroupNamesVariable } from '../../../__mocks__/CloudWatchDataSource';
import {
  diffQuery,
  diffModifierQuery,
  emptyQuery,
  filterQuery,
  newCommandQuery,
  sortQuery,
} from '../../../__mocks__/cloudwatch-logs-test-data';
import { ResourcesAPI } from '../../../resources/ResourcesAPI';
import { ResourceResponse } from '../../../resources/types';
import { LogGroup, LogGroupField } from '../../../types';
import cloudWatchLogsLanguageDefinition, { CLOUDWATCH_LOGS_LANGUAGE_DEFINITION_ID } from '../definition';
import { DIFF_MODIFIERS, LOGS_COMMANDS, LOGS_FUNCTION_OPERATORS, SORT_DIRECTION_KEYWORDS, language } from '../language';

import { LogsCompletionItemProvider } from './CompletionItemProvider';

const getSuggestions = async (
  value: string,
  position: monacoTypes.IPosition,
  variables: CustomVariableModel[] = [],
  logGroups: LogGroup[] = [],
  fields: Array<ResourceResponse<LogGroupField>> = []
) => {
  const setup = new LogsCompletionItemProvider(
    {
      getActualRegion: () => 'us-east-2',
    } as ResourcesAPI,
    setupMockedTemplateService(variables),
    { region: 'default', logGroups }
  );

  setup.resources.getLogGroupFields = jest.fn().mockResolvedValue(fields);
  const provider = setup.getCompletionProvider(monaco, cloudWatchLogsLanguageDefinition);
  const model = monaco.editor.createModel(value, CLOUDWATCH_LOGS_LANGUAGE_DEFINITION_ID);
  const { suggestions } = await provider.provideCompletionItems(model, position);
  return suggestions;
};

describe('LogsCompletionItemProvider', () => {
  let tokenizer: monaco.IDisposable;

  beforeAll(() => {
    monaco.languages.register({ id: CLOUDWATCH_LOGS_LANGUAGE_DEFINITION_ID });
    tokenizer = monaco.languages.setMonarchTokensProvider(CLOUDWATCH_LOGS_LANGUAGE_DEFINITION_ID, language);
  });

  afterEach(() => {
    for (const m of monaco.editor.getModels()) {
      m.dispose();
    }
  });

  afterAll(() => {
    tokenizer?.dispose();
  });

  describe('getSuggestions', () => {
    it('returns commands for an empty query', async () => {
      const suggestions = await getSuggestions(emptyQuery.query, emptyQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(LOGS_COMMANDS));
    });

    it('returns commands for a query when a new command is started', async () => {
      const suggestions = await getSuggestions(newCommandQuery.query, newCommandQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(LOGS_COMMANDS));
    });

    it('returns sort order directions for the sort keyword', async () => {
      const suggestions = await getSuggestions(sortQuery.query, sortQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(SORT_DIRECTION_KEYWORDS));
    });

    it('returns function suggestions after a command', async () => {
      const suggestions = await getSuggestions(sortQuery.query, sortQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(LOGS_FUNCTION_OPERATORS));
    });

    it('returns diff modifiers after the diff keyword', async () => {
      const suggestions = await getSuggestions(diffQuery.query, diffQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(DIFF_MODIFIERS));
    });

    it('returns diff modifiers when partially typed', async () => {
      const suggestions = await getSuggestions(diffModifierQuery.query, diffModifierQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(DIFF_MODIFIERS));
    });

    it('returns `in []` snippet for the `in` keyword', async () => {
      const suggestions = await getSuggestions(filterQuery.query, filterQuery.position);
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(['in []']));
    });

    it('returns template variables appended to list of suggestions', async () => {
      const suggestions = await getSuggestions(newCommandQuery.query, newCommandQuery.position, [
        logGroupNamesVariable,
      ]);
      const suggestionLabels = suggestions.map((s) => s.label);
      const expectedTemplateVariableLabel = `$${logGroupNamesVariable.name}`;
      const expectedLabels = [...LOGS_COMMANDS, expectedTemplateVariableLabel];
      expect(suggestionLabels).toEqual(expect.arrayContaining(expectedLabels));
    });

    it('fetches fields when logGroups are set', async () => {
      const suggestions = await getSuggestions(
        sortQuery.query,
        sortQuery.position,
        [],
        [{ arn: 'foo', name: 'bar' }],
        [{ value: { name: '@field' } }]
      );
      const suggestionLabels = suggestions.map((s) => s.label);
      expect(suggestionLabels).toEqual(expect.arrayContaining(['@field']));
    });
  });
});
