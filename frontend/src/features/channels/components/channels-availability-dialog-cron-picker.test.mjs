import React from 'react';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import ts from 'typescript';

// CronPicker lives inside the availability dialog; a regression that drops the
// dropdown in custom mode would silently kill the preset/"none"/"opposite"
// path out of a custom cron. Render the real dialog JSX (imports replaced by
// minimal stubs, production data helpers and production constants extracted
// unchanged) with React SSR and assert on the static markup.
const componentsDir = import.meta.dirname;
const srcRoot = join(componentsDir, '..', '..', '..');
const read = (relativePath) => readFileSync(join(srcRoot, relativePath), 'utf8');

const enLocales = Object.fromEntries(
  readdirSync(join(srcRoot, 'locales', 'en'))
    .filter((file) => file.endsWith('.json'))
    .flatMap((file) => Object.entries(JSON.parse(read(`locales/en/${file}`))))
);

const extractFunctions = (source, name, names) => {
  const ast = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const nodes = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && names.includes(node.name?.text));
  assert.equal(nodes.length, names.length, `${names.join(', ')} must stay in ${name}`);
  return nodes.map((node) => `export ${node.getText(ast).replace(/^export\s+/, '')}`);
};

const extractConstants = (source, name, names) => {
  const ast = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const nodes = ast.statements.filter(
    (node) =>
      ts.isVariableStatement(node) &&
      node.declarationList.declarations.some((decl) => ts.isIdentifier(decl.name) && names.includes(decl.name.text))
  );
  assert.equal(nodes.length, names.length, `${names.join(', ')} must stay in ${name}`);
  return nodes.map((node) => node.getText(ast));
};

const dataHelpers = extractFunctions(read('features/channels/data/auto-disable.ts'), 'auto-disable.ts', [
  'effectiveAutoDisableMode',
  'toApiKeyAutoDisableRuleFormValues',
  'serializeApiKeyAutoDisableRules',
  'toScheduledDisableFormValue',
  'availabilityPoliciesPayload',
]);
const editorExports = [
  ...extractConstants(read('features/channels/components/api-key-auto-disable-rules-editor.tsx'), 'api-key-auto-disable-rules-editor.tsx', [
    'CRON_PRESETS',
    'CRON_PRESET_LABEL_KEYS',
    'TIMEZONE_OPTIONS',
  ]),
  ...extractFunctions(read('features/channels/components/api-key-auto-disable-rules-editor.tsx'), 'api-key-auto-disable-rules-editor.tsx', [
    'zoneOffsetLabel',
  ]),
];

// The dialog's production JSX with its imports stripped; everything imported
// is provided by the stubs below.
const dialogSource = read('features/channels/components/channels-availability-dialog.tsx').replace(/^import[\s\S]*?;\n/gm, '');

const moduleSource = [
  'const { useState, useCallback, useEffect } = React;',
  'function t(key, params) {',
  '  let out = LOCALES[key] ?? key;',
  '  for (const [name, value] of Object.entries(params || {})) out = out.split(`{{${name}}}`).join(String(value));',
  '  return out;',
  '}',
  'function useTranslation() { return { t, i18n: { language: "en" } }; }',
  'function useRetryPolicy() { return { data: null }; }',
  'function useUpdateChannel() { return { mutateAsync: async () => {}, isPending: false }; }',
  'const toast = { success() {}, error() {} };',
  'function Link({ children }) { return <a>{children}</a>; }',
  'function Button({ children }) { return <button>{children}</button>; }',
  'function Label({ children }) { return <label>{children}</label>; }',
  'function Input({ value, placeholder, onChange, "aria-label": ariaLabel, "data-testid": testId }) {',
  '  return <input value={value} placeholder={placeholder} onChange={onChange} aria-label={ariaLabel} data-testid={testId} />;',
  '}',
  'function cn(...classes) { return classes.filter(Boolean).join(" "); }',
  'function RadioGroup({ children }) { return <div>{children}</div>; }',
  'function RadioGroupItem() { return null; }',
  'function Select({ value, children }) { return <div data-select-value={value}>{children}</div>; }',
  'function SelectTrigger({ className, children }) { return <span className={className} data-select-trigger>{children}</span>; }',
  'function SelectValue({ placeholder }) { return <span data-select-value-content>{placeholder}</span>; }',
  'function SelectContent({ children }) { return <span data-select-content>{children}</span>; }',
  'function SelectItem({ value, children }) { return <span data-select-item-value={value}>{children}</span>; }',
  'function Collapsible({ children }) { return <div>{children}</div>; }',
  'function CollapsibleTrigger({ children }) { return <span>{children}</span>; }',
  'function CollapsibleContent({ children }) { return <span>{children}</span>; }',
  'function Dialog({ children }) { return <div>{children}</div>; }',
  'function DialogContent({ children }) { return <div>{children}</div>; }',
  'function DialogDescription({ children }) { return <div>{children}</div>; }',
  'function DialogFooter({ children }) { return <div>{children}</div>; }',
  'function DialogHeader({ children }) { return <div>{children}</div>; }',
  'function DialogTitle({ children }) { return <div>{children}</div>; }',
  'function ApiKeyAutoDisableRulesEditor() { return null; }',
  'function ApiKeyAutoDisableRulesPreview() { return null; }',
  'const LOCALES = {};',
  'export function setLocales(table) { for (const [key, value] of Object.entries(table)) LOCALES[key] = value; }',
  ...dataHelpers,
  ...editorExports,
  dialogSource,
].join('\n');

const { outputText } = ts.transpileModule(moduleSource, {
  compilerOptions: { jsx: ts.JsxEmit.React, module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
globalThis.React = React;
const { ChannelsAvailabilityDialog, setLocales } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);
setLocales(enLocales);

function renderDialog(scheduledDisable) {
  return renderToStaticMarkup(
    React.createElement(ChannelsAvailabilityDialog, {
      open: true,
      onOpenChange: () => {},
      currentRow: {
        id: 'gid://axonhub/Channel/1',
        name: 'test-channel',
        policies: { scheduledDisable },
      },
    })
  )
    .replace(/&#x27;/g, "'")
    .replace(/&amp;/g, '&');
}
const count = (html, text) => html.split(text).length - 1;
const inputOf = (html, testId) => {
  const match = html.match(new RegExp(`<input[^>]*data-testid="${testId}"[^>]*>`));
  assert.ok(match, `input ${testId} must be rendered`);
  return match[0];
};

test('custom disable cron keeps the dropdown pinned to the custom option next to the text input', () => {
  const html = renderDialog({
    disableCron: '0 22 * * *',
    disableTimezone: 'UTC',
    reenableMode: 'none',
  });

  // The dropdown is present and pinned to the custom option.
  assert.ok(count(html, 'data-select-value="custom"') >= 1, 'the picker must stay pinned to the custom option');
  assert.ok(count(html, 'data-select-trigger') >= 1, 'the dropdown trigger must not be dropped in custom mode');
  assert.ok(count(html, 'data-select-item-value="custom"') >= 1, 'the custom option must stay in the list');
  assert.ok(html.includes('Custom cron expression'), 'the custom option uses the localized label');

  // The text input for the custom cron is rendered alongside the dropdown.
  const input = inputOf(html, 'availability-scheduled-disable-cron');
  assert.ok(input.includes('value="0 22 * * *"'), 'the custom cron is prefilled in the input');
  assert.ok(input.includes('aria-label="Disable on schedule (cron)"'), 'the input keeps its accessible label');

  // A non-custom picker must not render a text input.
  assert.equal(count(html, 'data-testid="availability-scheduled-reenable-cron"'), 0, 'no re-enable input outside cron mode');
});

test('custom re-enable cron shares the pinned dropdown plus input behavior', () => {
  const html = renderDialog({
    disableCron: '0 * * * *',
    disableTimezone: 'UTC',
    reenableMode: 'cron',
    reenableCron: '0 6 * * *',
  });

  const reenableInput = inputOf(html, 'availability-scheduled-reenable-cron');
  assert.ok(reenableInput.includes('value="0 6 * * *"'), 'the custom re-enable cron is prefilled in the input');
  assert.ok(count(html, 'data-select-value="custom"') >= 1, 'the re-enable picker stays pinned to the custom option');
  assert.ok(
    count(html, 'data-select-item-value="opposite"') >= 1,
    'the re-enable picker keeps its component-specific options next to the custom item'
  );

  // The disable picker holds a preset here, so its input is gone.
  assert.equal(count(html, 'data-testid="availability-scheduled-disable-cron"'), 0, 'a preset selection renders no input');
});

test('preset selections render the dropdown without a text input', () => {
  const html = renderDialog({
    disableCron: '0 * * * *',
    disableTimezone: 'UTC',
    reenableMode: 'none',
  });

  assert.ok(html.includes('data-select-value="0 * * * *"'), 'the picker is pinned to the preset');
  assert.equal(count(html, 'data-select-value="custom"'), 0, 'a preset is not the custom option');
  assert.equal(count(html, 'data-testid="availability-scheduled-disable-cron"'), 0, 'a preset renders no input');
  assert.ok(html.includes('Every hour, on the hour'), 'the selected preset keeps its localized label in the trigger');
});
