import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// The create/edit dialog must expose every credential field that the channel
// schema requires for `anthropic_gcp`. The schema has required
// credentials.gcp.{region,projectID,jsonData} since the type was registered, and
// the form defaults, the GraphQL read block and the update guard all carry a gcp
// shape, but no control ever let a user select the type or fill those fields — so
// the dialog could only ever produce a payload the schema rejects.
// See https://github.com/looplj/axonhub/issues/871.
const dialogPath = new URL('./channels-action-dialog.tsx', import.meta.url);
const source = readFileSync(dialogPath, 'utf8');
const schemaSource = readFileSync(new URL('../data/schema.ts', import.meta.url), 'utf8');

// Confirm the dialog still parses as TSX, so a syntax slip cannot masquerade as a
// passing wiring assertion.
const ast = ts.createSourceFile('channels-action-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
assert.deepEqual(
  ast.parseDiagnostics.map((d) => ts.flattenDiagnosticMessageText(d.messageText, '\n')),
  [],
  'channels-action-dialog.tsx must parse without diagnostics'
);

const enLocale = JSON.parse(readFileSync(new URL('../../../locales/en/channels.json', import.meta.url), 'utf8'));
const zhLocale = JSON.parse(readFileSync(new URL('../../../locales/zh-CN/channels.json', import.meta.url), 'utf8'));

// Derived from the schema rather than hardcoded, so the invariant keeps holding if
// a required gcp field is ever added. Both the create and the update schema declare
// the same three, so dedupe: the dialog owes one field per distinct path.
const anthropicGcpBlock = schemaSource.slice(schemaSource.indexOf("data.type === 'anthropic_gcp'"));
const requiredGcpFields = [
  ...new Set([...anthropicGcpBlock.matchAll(/path:\s*\[\s*'credentials',\s*'gcp',\s*'(\w+)'\s*\]/g)].map((m) => m[1])),
];

test('schema requires gcp credential fields for anthropic_gcp on create and on update', () => {
  assert.deepEqual(
    [...requiredGcpFields].sort(),
    ['jsonData', 'projectID', 'region'],
    'expected the schema to require exactly region/projectID/jsonData for anthropic_gcp'
  );
  const declarations = [...schemaSource.matchAll(/path:\s*\[\s*'credentials',\s*'gcp',\s*'(\w+)'\s*\]/g)].length;
  assert.equal(declarations, 6, 'both the create and the update schema must require all three gcp fields');
});

test('dialog renders a form field for every gcp credential the schema requires', () => {
  for (const field of requiredGcpFields) {
    const rendered =
      source.includes(`name='credentials.gcp.${field}'`) || source.includes(`name="credentials.gcp.${field}"`);
    assert.ok(rendered, `credentials.gcp.${field} is required by the schema but has no FormField in the dialog`);
  }
});

test('the gcp fields are reachable: a variant control can select anthropic_gcp', () => {
  assert.match(source, /useAnthropicGcp/, 'the dialog needs state for the Anthropic GCP variant');
  const derived = source.slice(source.indexOf('const derivedChannelType'), source.indexOf('const formSchema'));
  assert.match(derived, /'anthropic_gcp'/, 'derivedChannelType must be able to yield anthropic_gcp');
});

test('the variant checkbox is rendered in the anthropic/messages group and labelled in both locales', () => {
  const labelKey = 'channels.dialogs.fields.apiFormat.anthropicGCP.label';
  assert.ok(source.includes(labelKey), `the dialog must render the ${labelKey} checkbox label`);
  for (const [name, locale] of [
    ['en', enLocale],
    ['zh-CN', zhLocale],
  ]) {
    assert.ok(typeof locale[labelKey] === 'string' && locale[labelKey].length > 0, `${name} locale is missing ${labelKey}`);
  }
  const anthropicGroup = source.slice(source.indexOf("selectedProvider === 'anthropic' && ("));
  assert.ok(
    anthropicGroup.indexOf('useAnthropicGcp') < anthropicGroup.indexOf('useKimiCoding'),
    'the GCP checkbox belongs to the anthropic/messages variant group'
  );
});

test('every gcp field label and placeholder key exists in both locales', () => {
  for (const field of requiredGcpFields) {
    for (const suffix of ['label', 'placeholder']) {
      const key = `channels.dialogs.fields.gcp.${field}.${suffix}`;
      assert.ok(source.includes(key), `the dialog must use ${key}`);
      for (const [name, locale] of [
        ['en', enLocale],
        ['zh-CN', zhLocale],
      ]) {
        assert.ok(typeof locale[key] === 'string' && locale[key].length > 0, `${name} locale is missing ${key}`);
      }
    }
  }
});

test('the gcp variant state is restored on both the row change and the reopen reset', () => {
  // Same idiom as the quota-routing test: a mapping that only feeds one of the two
  // reset paths silently keeps a stale variant after the dialog is reopened.
  const restores = source.split("initialRow.type === 'anthropic_gcp'").length - 1;
  assert.ok(restores >= 2, `expected the anthropic_gcp variant to be recalled from the row at least twice, found ${restores}`);
  assert.match(source, /setUseAnthropicGcp\(false\)/, 'the variant must be cleared when the dialog reopens without a row');
});

test('the two anthropic vertex/bedrock variants stay mutually exclusive', () => {
  const awsHandler = source.slice(source.indexOf('const handleAnthropicAwsChange'), source.indexOf('const handleAnthropicGcpChange'));
  assert.match(awsHandler, /setUseAnthropicGcp\(false\)/, 'selecting AWS Bedrock must clear the GCP variant');
  const gcpHandler = source.slice(source.indexOf('const handleAnthropicGcpChange'), source.indexOf('const handleKimiCodingChange'));
  assert.match(gcpHandler, /setUseAnthropicAws\(false\)/, 'selecting GCP must clear the AWS Bedrock variant');
});

// The owner ruled on 2026-02-20 that gemini_vertex works with an API key and does
// not need GCP credentials, so the gcp fields must stay scoped to anthropic_gcp.
// This guards that ruling against a well-meaning "unify the vertex types" change.
test('gcp credential fields are not offered for gemini_vertex', () => {
  for (const field of requiredGcpFields) {
    const fieldIdx = source.indexOf(`name='credentials.gcp.${field}'`);
    assert.ok(fieldIdx > 0, `credentials.gcp.${field} must have a FormField`);
    // Inspect the actual enclosing JSX condition: the `{ ... && (` gate that wraps
    // the field, not the text between fields (which never contains the gate).
    const gateIdx = source.lastIndexOf('&& (', fieldIdx);
    assert.ok(gateIdx > 0, `credentials.gcp.${field} must sit inside a conditional render`);
    const condition = source.slice(source.lastIndexOf('{', gateIdx) + 1, gateIdx);
    assert.match(
      condition,
      /selectedType === 'anthropic_gcp'/,
      `credentials.gcp.${field} must be gated on selectedType === 'anthropic_gcp'`
    );
    assert.ok(
      !condition.includes('gemini_vertex'),
      'gemini_vertex must keep using an API key, not gcp credentials'
    );
  }
});

// greptile P1 on #2644: the anthropic/messages variants were gated on the API
// format alone, so a variant checkbox left over from a previous provider leaked
// across a provider switch whose format list also contains anthropic/messages
// (anthropic -> moonshot keeps the format, so the format-change reset never
// fires) and the dialog submitted e.g. a Moonshot form as an anthropic_gcp
// channel. Each variant must be gated on the provider that owns it.
test('derivedChannelType gates each anthropic/messages variant on its owning provider', () => {
  const derived = source.slice(source.indexOf('const derivedChannelType'), source.indexOf('const formSchema'));
  for (const [provider, flag, type] of [
    ['anthropic', 'useAnthropicAws', 'anthropic_aws'],
    ['anthropic', 'useAnthropicGcp', 'anthropic_gcp'],
    ['moonshot', 'useKimiCoding', 'moonshot_coding'],
  ]) {
    assert.match(
      derived,
      new RegExp(`selectedProvider === '${provider}' && ${flag}\\) return '${type}'`),
      `${flag} must only yield ${type} while the ${provider} provider is selected`
    );
  }
});

test('switching providers clears the variant checkboxes owned by the previous provider', () => {
  const handler = source.slice(source.indexOf('const handleProviderChange'), source.indexOf('const handleApiFormatChange'));
  const leaveAnthropic = handler.slice(handler.indexOf("provider !== 'anthropic'"), handler.indexOf("provider !== 'moonshot'"));
  assert.match(leaveAnthropic, /setUseAnthropicAws\(false\)/, 'leaving anthropic must clear the AWS Bedrock variant');
  assert.match(leaveAnthropic, /setUseAnthropicGcp\(false\)/, 'leaving anthropic must clear the GCP variant');
  const leaveMoonshot = handler.slice(handler.indexOf("provider !== 'moonshot'"));
  assert.match(leaveMoonshot, /setUseKimiCoding\(false\)/, 'leaving moonshot must clear the Kimi coding variant');
});

// coderabbit Minor on #2644: the provider radio stays enabled in edit mode for
// non-OAuth channels, so editing an anthropic_gcp channel and switching the
// provider away leaves the loaded credentials.gcp blob (region/projectID/
// jsonData) in the form state, and it rides into updateInput.credentials. The
// backend replaces the whole credentials object, so the stale blob lands on the
// non-GCP channel. The edit path must strip it.
test('editing away from anthropic_gcp strips the stale gcp blob from the update payload', () => {
  const editCleanup = source.slice(source.indexOf('const finalChannelType'), source.indexOf('updateChannelSettings.mutateAsync'));
  assert.match(
    editCleanup,
    /finalChannelType !== 'anthropic_gcp' && updateInput\.credentials\?\.gcp/,
    'the edit path must inspect updateInput.credentials.gcp when the final type is not anthropic_gcp'
  );
  assert.match(
    editCleanup,
    /delete updateInput\.credentials\.gcp/,
    'a stale gcp blob must be removed from the update payload'
  );
});

// Stripping the blob is not enough: if the cleared credentials object is then
// dropped by the all-empty guard, the backend keeps the old credentials object
// and the stale gcp survives. The guard must stay off right after a strip so the
// gcp-less credentials object is still sent.
test('the all-empty credentials guard must not swallow a just-stripped gcp blob', () => {
  const editCleanup = source.slice(source.indexOf('const finalChannelType'), source.indexOf('updateChannelSettings.mutateAsync'));
  assert.match(editCleanup, /clearedStaleGcpCredentials = true/, 'the edit path must remember that it stripped a stale gcp blob');
  const guard = editCleanup.slice(
    editCleanup.lastIndexOf('if (', editCleanup.indexOf('delete updateInput.credentials;'))
  );
  assert.match(
    guard,
    /!clearedStaleGcpCredentials/,
    'the delete updateInput.credentials guard must be disabled when a stale gcp blob was just stripped'
  );
});
