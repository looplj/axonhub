import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { registerHooks } from 'node:module';
import { join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';

// schema.ts imports through the '@/...' alias and omits file extensions, so the test
// resolves both the way the bundler does before importing the production modules.
const srcRoot = join(import.meta.dirname, '..', '..', '..');

function resolveWithExtension(basePath) {
  for (const ext of ['.ts', '.tsx', '.mjs', '.js']) {
    if (existsSync(`${basePath}${ext}`)) {
      return `${basePath}${ext}`;
    }
  }
  return null;
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    let candidate = null;
    if (specifier.startsWith('@/')) {
      candidate = join(srcRoot, specifier.slice(2));
    } else if ((specifier.startsWith('./') || specifier.startsWith('../')) && context.parentURL?.startsWith('file://')) {
      candidate = join(fileURLToPath(context.parentURL), '..', specifier);
    }

    if (candidate) {
      const resolved = resolveWithExtension(candidate);
      if (resolved) {
        return { url: pathToFileURL(resolved).href, shortCircuit: true };
      }
    }

    return nextResolve(specifier, context);
  },
});

const {
  getVertexServiceAccountIdentity,
  maskVertexCredentialRef,
  parseVertexServiceAccountJSON,
  toVertexServiceAccountInputs,
  vertexServiceAccountCredentialSchema,
} = await import('./vertex-service-account.ts');
const { createChannelInputSchema, updateChannelInputSchema } = await import('./schema.ts');

const serviceAccount = {
  type: 'service_account',
  project_id: 'sa-project',
  private_key_id: 'key-id-1',
  private_key: '-----BEGIN PRIVATE KEY-----\nMIIBVwIBADANBgkqhkiG9w0BAQEFAASCAT8wggE7\n-----END PRIVATE KEY-----\n',
  client_email: 'vertex-sa@sa-project.iam.gserviceaccount.com',
  client_id: '1045',
  token_uri: 'https://oauth2.googleapis.com/token',
};

function serviceAccountJSON(overrides = {}) {
  return JSON.stringify({ ...serviceAccount, ...overrides });
}

function vertexCreateInput(credentials) {
  return {
    type: 'gemini_vertex',
    baseURL: 'https://aiplatform.googleapis.com',
    name: 'vertex-mixed',
    supportedModels: ['gemini-3-pro'],
    defaultTestModel: 'gemini-3-pro',
    credentials,
  };
}

function vertexUpdateInput(credentials) {
  // The edit dialog always submits the form type, which is what scopes Vertex validation.
  return {
    type: 'gemini_vertex',
    name: 'vertex-mixed',
    credentials,
  };
}

test('parses a service account key and keeps only the fields the channel needs', () => {
  const parsed = parseVertexServiceAccountJSON(serviceAccountJSON());

  assert.equal(parsed.ok, true);
  assert.equal(parsed.value.projectID, 'sa-project');
  assert.equal(parsed.value.clientEmail, 'vertex-sa@sa-project.iam.gserviceaccount.com');
  assert.match(parsed.value.privateKey, /BEGIN PRIVATE KEY/);
});

test('trims surrounding whitespace and an untidy project_id', () => {
  const parsed = parseVertexServiceAccountJSON(`\n  ${serviceAccountJSON({ project_id: '  sa-project  ' })}  \n`);

  assert.equal(parsed.ok, true);
  assert.equal(parsed.value.projectID, 'sa-project');
});

test('rejects malformed and unrelated credential JSON with distinct reasons', () => {
  assert.deepEqual(parseVertexServiceAccountJSON('{"type":'), { ok: false, error: 'invalidJSON' });
  assert.deepEqual(parseVertexServiceAccountJSON('[]'), { ok: false, error: 'invalidJSON' });
  assert.deepEqual(parseVertexServiceAccountJSON('null'), { ok: false, error: 'invalidJSON' });
  assert.deepEqual(parseVertexServiceAccountJSON(serviceAccountJSON({ type: 'authorized_user' })), {
    ok: false,
    error: 'notServiceAccount',
  });
  assert.deepEqual(parseVertexServiceAccountJSON(serviceAccountJSON({ client_email: '' })), {
    ok: false,
    error: 'missingClientEmail',
  });
  assert.deepEqual(parseVertexServiceAccountJSON(serviceAccountJSON({ private_key: '' })), {
    ok: false,
    error: 'missingPrivateKey',
  });
  assert.deepEqual(parseVertexServiceAccountJSON(serviceAccountJSON({ private_key: 'not a pem key' })), {
    ok: false,
    error: 'invalidPrivateKey',
  });
});

test('accepts a JSON-only Vertex channel and defaults its region to global', () => {
  const result = createChannelInputSchema.safeParse(
    vertexCreateInput({
      apiKeys: [],
      vertexApiKeys: [],
      vertexServiceAccounts: [
        { projectID: 'route-project', region: 'global', jsonData: serviceAccountJSON() },
        { projectID: 'route-project', jsonData: serviceAccountJSON({ project_id: 'route-project' }) },
      ],
    })
  );

  assert.equal(result.success, true, result.success ? '' : JSON.stringify(result.error.issues));
  assert.equal(result.data.credentials.vertexApiKeys.length, 0);
  assert.equal(result.data.credentials.vertexServiceAccounts.length, 2);
  assert.equal(result.data.credentials.vertexServiceAccounts[1].region, 'global');
});

test('keeps API keys and service accounts on the same channel', () => {
  const result = createChannelInputSchema.safeParse(
    vertexCreateInput({
      apiKeys: [],
      vertexApiKeys: [{ projectID: 'key-project', apiKey: 'AIza-key', region: 'us-central1' }],
      vertexServiceAccounts: [{ projectID: 'sa-project', region: 'europe-west4', jsonData: serviceAccountJSON() }],
    })
  );

  assert.equal(result.success, true, result.success ? '' : JSON.stringify(result.error.issues));
  assert.deepEqual(result.data.credentials.vertexApiKeys, [
    { projectID: 'key-project', apiKey: 'AIza-key', region: 'us-central1' },
  ]);
  assert.equal(result.data.credentials.vertexServiceAccounts[0].projectID, 'sa-project');
  assert.equal(result.data.credentials.vertexServiceAccounts[0].region, 'europe-west4');
});

test('requires at least one Vertex credential across both lists', () => {
  const result = createChannelInputSchema.safeParse(
    vertexCreateInput({
      apiKeys: [],
      vertexApiKeys: [{ projectID: '', apiKey: '', region: 'global' }],
      vertexServiceAccounts: [{ projectID: '', region: 'global', jsonData: '' }],
    })
  );

  assert.equal(result.success, false);
  assert.ok(
    result.error.issues.some(
      (issue) =>
        issue.message === 'At least one Vertex API key or service account JSON is required' &&
        issue.path.join('.') === 'credentials.vertexApiKeys'
    ),
    JSON.stringify(result.error.issues)
  );
});

test('reports an invalid JSON row on its own list and row index', () => {
  const result = createChannelInputSchema.safeParse(
    vertexCreateInput({
      apiKeys: [],
      vertexApiKeys: [{ projectID: 'key-project', apiKey: 'AIza-key', region: 'global' }],
      vertexServiceAccounts: [{ projectID: 'route-project', region: 'global', jsonData: '{"type":"service_account"' }],
    })
  );

  assert.equal(result.success, false);
  assert.ok(
    result.error.issues.some(
      (issue) =>
        issue.path.join('.') === 'credentials.vertexServiceAccounts.0.jsonData' &&
        issue.message === 'Vertex service account JSON is not valid JSON'
    ),
    JSON.stringify(result.error.issues)
  );
});

test('reports a half-filled API key row instead of silently dropping it', () => {
  const result = createChannelInputSchema.safeParse(
    vertexCreateInput({
      apiKeys: [],
      vertexApiKeys: [{ projectID: 'key-project', apiKey: '', region: 'global' }],
      vertexServiceAccounts: [],
    })
  );

  assert.equal(result.success, false);
  assert.ok(
    result.error.issues.some(
      (issue) => issue.path.join('.') === 'credentials.vertexApiKeys.0.apiKey' && issue.message === 'Vertex API key is required'
    ),
    JSON.stringify(result.error.issues)
  );
});

test('rejects a service account row whose credential type is not service_account', () => {
  const result = vertexServiceAccountCredentialSchema('global').safeParse({
    projectID: 'route-project',
    region: 'global',
    jsonData: serviceAccountJSON({ type: 'authorized_user' }),
  });

  assert.equal(result.success, false);
  assert.equal(
    result.error.issues[0]?.message,
    'Vertex service account JSON must have "type": "service_account"'
  );
});

test('update accepts JSON-only credentials and rejects clearing both lists', () => {
  const jsonOnly = updateChannelInputSchema.safeParse(
    vertexUpdateInput({
      vertexServiceAccounts: [{ projectID: 'sa-project', region: 'global', jsonData: serviceAccountJSON() }],
    })
  );
  assert.equal(jsonOnly.success, true, jsonOnly.success ? '' : JSON.stringify(jsonOnly.error.issues));

  const cleared = updateChannelInputSchema.safeParse(
    vertexUpdateInput({ vertexApiKeys: [], vertexServiceAccounts: [] })
  );
  assert.equal(cleared.success, false);
});

test('builds submit payloads without credentialRef and without blank drafts', () => {
  const payload = toVertexServiceAccountInputs([
    { projectID: ' route-project ', region: '', jsonData: ` ${serviceAccountJSON()} `, credentialRef: 'vertex-sa:abc123' },
    { projectID: '', region: 'global', jsonData: '   ' },
  ]);

  assert.deepEqual(payload, [{ projectID: 'route-project', region: 'global', jsonData: serviceAccountJSON() }]);
  assert.equal('credentialRef' in payload[0], false);
});

test('exposes a credential identity that never falls back to raw JSON', () => {
  assert.deepEqual(getVertexServiceAccountIdentity(serviceAccountJSON()), {
    clientEmail: 'vertex-sa@sa-project.iam.gserviceaccount.com',
    projectID: 'sa-project',
  });
  assert.deepEqual(getVertexServiceAccountIdentity('{"private_key":"secret"}'), { clientEmail: '', projectID: '' });
  assert.equal(maskVertexCredentialRef('vertex-sa:0123456789abcdef'), 'vertex-sa:****cdef');
});
