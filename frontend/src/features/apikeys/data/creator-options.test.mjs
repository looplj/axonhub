import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const source = ts.transpileModule(readFileSync(new URL('./creator-options.ts', import.meta.url), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2023 },
}).outputText;

function setup(projectId) {
  const requests = [];
  const response = { users: { edges: [{ node: { id: 'user-1', email: 'user@example.com', firstName: null, lastName: null } }] } };
  const mocks = {
    '@tanstack/react-query': { useQuery: (options) => options },
    'react-i18next': { useTranslation: () => ({ t: (key) => key }) },
    '@/stores/projectStore': { useSelectedProjectId: () => projectId },
    '@/hooks/use-error-handler': { useErrorHandler: () => ({ handleError() {} }) },
    '@/gql/graphql': { graphqlRequest: async (...args) => { requests.push(args); return response; } },
  };
  const exports = {};
  vm.runInNewContext(source, { exports, require: (name) => mocks[name] ?? require(name) });
  return { hook: exports.useApiKeyCreators, requests, response };
}

test('creator query stays disabled without user permission or a selected project', () => {
  assert.equal(setup('project-1').hook(false).enabled, false);
  assert.equal(setup(null).hook(true).enabled, false);
  assert.equal(setup('project-1').hook(true).enabled, true);
});

test('creator query uses the selected project and accepts users without names or role data', async () => {
  const { hook, requests, response } = setup('project-1');
  const result = await hook(true).queryFn();
  assert.deepEqual(result, response.users);
  assert.equal(requests[0][2]['X-Project-ID'], 'project-1');
});

test('creator caches are isolated when switching projects', () => {
  const first = setup('project-1').hook(true);
  const second = setup('project-2').hook(true);
  assert.notDeepEqual(first.queryKey, second.queryKey);
});
