import assert from 'node:assert/strict';
import { readFileSync, rmSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { join as joinPath } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const srcRoot = joinPath(import.meta.dirname, '..', '..', '..');
const schemaSource = readFileSync(joinPath(srcRoot, 'features/channels/data/schema.ts'), 'utf8');

// schema.ts pulls pageInfoSchema from the gql barrel, which drags in GraphQL client
// modules that cannot be imported in a plain node test. The pagination schema plays no
// part in channel validation, so inline an equivalent and transpile the rest.
const stubbed = schemaSource
  .replace(/^import \{ pageInfoSchema \} from '@\/gql\/pagination';$/m, "import { z as zodForStub } from 'zod';\nconst pageInfoSchema = zodForStub.object({ hasNextPage: zodForStub.boolean(), hasPreviousPage: zodForStub.boolean(), startCursor: zodForStub.string().optional(), endCursor: zodForStub.string().optional() });")
  .replace(/^import \{ z \} from 'zod';$/m, "import { z } from 'zod';\n");

assert.notEqual(stubbed, schemaSource, 'the pageInfoSchema import stub must apply');

// The transpiled module must stay inside frontend/ so node resolves 'zod' through the
// package's own node_modules rather than a directory outside it.
const outFile = joinPath(import.meta.dirname, `.schema-under-test-${process.pid}.mjs`);
writeFileSync(outFile, ts.transpileModule(stubbed, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText);

const { createChannelInputSchema, updateChannelInputSchema } = await import(pathToFileURL(outFile).href);
test.after(() => rmSync(outFile, { force: true }));

function vertexRow(overrides = {}) {
  return { projectID: 'miguo-vertex-prod-1', apiKey: 'AQ.key-1', region: 'global', ...overrides };
}

const baseChannel = {
  name: 'Test Channel',
  baseURL: 'https://aiplatform.googleapis.com/v1',
  supportedModels: ['gemini-3.8-flash'],
  defaultTestModel: 'gemini-3.8-flash',
};

function issuePaths(result) {
  assert.equal(result.success, false, 'expected validation to fail');
  return result.error.issues.map((issue) => `${issue.path.join('.')}::${issue.message}`);
}

test('a Vertex channel rejects a row with a missing project ID', () => {
  const result = createChannelInputSchema.safeParse({
    ...baseChannel,
    type: 'gemini_vertex',
    credentials: { vertexApiKeys: [vertexRow({ projectID: '' })] },
  });

  const paths = issuePaths(result);
  assert.ok(
    paths.some((entry) => entry.startsWith('credentials.vertexApiKeys.0.projectID::')),
    `expected a projectID error at the row path, got ${JSON.stringify(paths)}`
  );
});

test('a Vertex channel rejects a row with a missing API key', () => {
  const result = createChannelInputSchema.safeParse({
    ...baseChannel,
    type: 'gemini_vertex',
    credentials: { vertexApiKeys: [vertexRow({ apiKey: '' })] },
  });

  const paths = issuePaths(result);
  assert.ok(
    paths.some((entry) => entry.startsWith('credentials.vertexApiKeys.0.apiKey::')),
    `expected an apiKey error at the row path, got ${JSON.stringify(paths)}`
  );
});

test('a Vertex channel rejects a project ID containing a path separator', () => {
  const result = createChannelInputSchema.safeParse({
    ...baseChannel,
    type: 'gemini_vertex',
    credentials: { vertexApiKeys: [vertexRow({ projectID: 'projects/evil' })] },
  });

  const paths = issuePaths(result);
  assert.ok(
    paths.some((entry) => entry.startsWith('credentials.vertexApiKeys.0.projectID::')),
    `expected a projectID error at the row path, got ${JSON.stringify(paths)}`
  );
});

test('a Vertex channel accepts a complete binding', () => {
  const result = createChannelInputSchema.safeParse({
    ...baseChannel,
    type: 'gemini_vertex',
    credentials: { vertexApiKeys: [vertexRow({ region: 'us-central1' })] },
  });

  assert.equal(result.success, true, JSON.stringify(result.error?.issues));
});

// An ordinary channel never submits vertexApiKeys, because onSubmit strips the field.
// A leftover row from toggling "Use Vertex AI" on and back off must not block it: the
// row is invisible in that state, so a save would fail with no field to correct.
test('an ordinary channel saves despite a leftover empty Vertex row', () => {
  const result = createChannelInputSchema.safeParse({
    ...baseChannel,
    type: 'openai',
    baseURL: 'https://api.openai.com/v1',
    credentials: {
      apiKeys: ['sk-test'],
      vertexApiKeys: [{ projectID: '', apiKey: '', region: 'global' }],
    },
  });

  assert.equal(result.success, true, JSON.stringify(result.error?.issues));
});

// The same rule at the write boundary: a Vertex key must not stand in for the API key of
// a channel that is not a Vertex channel.
test('an ordinary channel still requires its own API key', () => {
  const result = createChannelInputSchema.safeParse({
    ...baseChannel,
    type: 'openai',
    baseURL: 'https://api.openai.com/v1',
    credentials: { vertexApiKeys: [vertexRow()] },
  });

  const paths = issuePaths(result);
  assert.ok(
    paths.some((entry) => entry.startsWith('credentials.apiKeys::')),
    `expected the ordinary API-key requirement to fire, got ${JSON.stringify(paths)}`
  );
});

test('an update rejects an emptied Vertex binding list', () => {
  const result = updateChannelInputSchema.safeParse({
    id: 'gid://axonhub/Channel/1',
    type: 'gemini_vertex',
    credentials: { vertexApiKeys: [] },
  });

  const paths = issuePaths(result);
  assert.ok(
    paths.some((entry) => entry === 'credentials.vertexApiKeys::At least one Vertex API key is required'),
    `expected the empty-list error, got ${JSON.stringify(paths)}`
  );
});

test('an update accepts a credential-only payload without Vertex rows', () => {
  const result = updateChannelInputSchema.safeParse({
    id: 'gid://axonhub/Channel/1',
    credentials: { apiKeys: ['sk-test'] },
  });

  assert.equal(result.success, true, JSON.stringify(result.error?.issues));
});
