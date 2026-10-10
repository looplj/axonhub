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
assert.ok(ast.statements.length > 0, 'channels-action-dialog.tsx must parse');

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
  const gcpBlock = source.slice(source.indexOf("name='credentials.gcp.region'"));
  const gate = gcpBlock.slice(0, gcpBlock.indexOf('<FormField'));
  assert.ok(
    source.includes("selectedType === 'anthropic_gcp'"),
    'the gcp fields must be gated on anthropic_gcp'
  );
  assert.ok(!gate.includes('gemini_vertex'), 'gemini_vertex must keep using an API key, not gcp credentials');
});
