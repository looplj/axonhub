import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, cpSync, rmSync, symlinkSync, writeFileSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const componentsDir = import.meta.dirname;
const srcRoot = join(componentsDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

// Extract a variable statement by name, wherever it is nested (both target
// handlers live inside WebhookSettings), statement-level sibling of
// quota-settings.test.mjs's brace-balanced extractFunction.
function extractStatement(source, name) {
  const ast = ts.createSourceFile('webhook-settings.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  let statement;
  const visit = (node) => {
    if (statement) {
      return;
    }
    if (
      ts.isVariableStatement(node) &&
      node.declarationList.declarations.length === 1 &&
      ts.isIdentifier(node.declarationList.declarations[0].name) &&
      node.declarationList.declarations[0].name.text === name
    ) {
      statement = node;
      return;
    }
    node.forEachChild(visit);
  };
  ast.statements.forEach(visit);
  assert.ok(statement, `${name} not found`);
  return statement.getText(ast);
}

function assertRenameRewritesRawAndTrimmedNames(handlerSource, label) {
  assert.match(
    handlerSource,
    /targetName\.trim\(\) === previousName \? trimmedName : targetName/,
    `${label}: a rename must rewrite subscription entries carrying the raw untrimmed previous name as well as the trimmed one`
  );
}

const source = read('features/system/components/webhook-settings.tsx');

// Execute the two target-management handlers instead of matching their source
// text, so a dropped raw-name match or a regression in the removal's
// remaining-owner check fails the test rather than passing silently. Both
// handlers live inside WebhookSettings, so the statements are lifted out and
// run against a fake setFormData.
const handlerSource = [extractStatement(source, 'removeTarget'), extractStatement(source, 'handleTargetNameChange')].join('\n');
const { outputText } = ts.transpileModule(
  `const useCallback = (fn) => fn;
let __setFormDataTarget = null;
const setFormData = (updater) => __setFormDataTarget(updater);
${handlerSource}
export function __bindSetFormData(target) {
  __setFormDataTarget = target;
}
export { removeTarget, handleTargetNameChange };`,
  { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 } }
);
const lifted = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

// Runs a handler against initial state and returns the state its
// setFormData updater produces. The lifted handlers close over the
// component-local setFormData, so the injected module exposes a delegating
// binding the tests point at their capture.
function apply(label, initial, handlerName, ...args) {
  const handler = handlerName === 'removeTarget' ? lifted.removeTarget : lifted.handleTargetNameChange;
  let next = null;
  lifted.__bindSetFormData((updater) => {
    next = typeof updater === 'function' ? updater({ targets: initial.targets, subscriptions: initial.subscriptions }) : updater;
  });
  handler(...args);
  assert.ok(next, `${label}: handler must call setFormData`);
  return next;
}

test('rename rewrites subscription entries carrying the raw untrimmed previous name', () => {
  const next = apply(
    'raw untrimmed previous name',
    {
      targets: [{ name: 'API ' }],
      subscriptions: [{ event: 'provider.quota.warning', targetNames: ['API '] }],
    },
    'handleTargetNameChange',
    0,
    'api'
  );
  assert.deepEqual(next.subscriptions, [{ event: 'provider.quota.warning', targetNames: ['api'] }]);
  assert.equal(next.targets[0].name, 'api');
});

test('rename matching the trimmed previous name rewrites without touching other entries', () => {
  const next = apply(
    'trimmed previous name',
    {
      targets: [{ name: 'API' }],
      subscriptions: [
        { event: 'provider.quota.warning', targetNames: ['API', 'prod'] },
        { event: 'provider.quota.exhausted', targetNames: ['prod'] },
      ],
    },
    'handleTargetNameChange',
    0,
    ' gateway '
  );
  assert.deepEqual(
    next.subscriptions,
    [
      { event: 'provider.quota.warning', targetNames: ['gateway', 'prod'] },
      { event: 'provider.quota.exhausted', targetNames: ['prod'] },
    ],
    'only the trimmed previous name is rewritten; sibling names and events are untouched'
  );
  assert.equal(next.targets[0].name, ' gateway ');
});

test('removing one of two targets sharing the same trimmed name keeps the survivor subscribed', () => {
  const initial = {
    targets: [{ name: 'Prod' }, { name: 'Prod ' }],
    subscriptions: [{ event: 'provider.quota.warning', targetNames: ['Prod'] }],
  };
  for (const index of [0, 1]) {
    const next = apply(`remove shared owner #${index}`, initial, 'removeTarget', index);
    assert.equal(next.targets.length, 1);
    assert.deepEqual(next.subscriptions, [{ event: 'provider.quota.warning', targetNames: ['Prod'] }]);
    assert.ok(next.targets[0].name.trim() === 'Prod', 'the surviving target keeps its trimmed name and its subscriptions');
  }
});

test('removing the sole owner of a trimmed name drops that name and empty subscriptions', () => {
  const next = apply(
    'sole owner',
    {
      targets: [{ name: 'Prod ' }, { name: 'Other' }],
      subscriptions: [
        { event: 'provider.quota.warning', targetNames: ['Prod', 'Other'] },
        { event: 'provider.quota.exhausted', targetNames: ['Prod'] },
      ],
    },
    'removeTarget',
    0
  );
  assert.deepEqual(
    next.targets.map((target) => target.name),
    ['Other']
  );
  assert.deepEqual(
    next.subscriptions,
    [{ event: 'provider.quota.warning', targetNames: ['Other'] }],
    'the shared entry keeps names other targets claim; the entry owned only by the removed target is dropped'
  );
});

test('renaming one of two targets sharing the same trimmed name keeps the shared name and adds the new one once', () => {
  const next = apply(
    'shared trimmed name',
    {
      targets: [{ name: 'Prod' }, { name: 'Prod ' }],
      subscriptions: [
        { event: 'provider.quota.warning', targetNames: ['Prod', 'staging'] },
        { event: 'provider.quota.exhausted', targetNames: ['Prod'] },
      ],
    },
    'handleTargetNameChange',
    0,
    'Staging 2'
  );
  assert.deepEqual(
    next.subscriptions,
    [
      { event: 'provider.quota.warning', targetNames: ['Prod', 'staging', 'Staging 2'] },
      { event: 'provider.quota.exhausted', targetNames: ['Prod', 'Staging 2'] },
    ],
    'the sibling keeps its shared-name coverage and the renamed target gains the new name exactly once'
  );
  assert.equal(next.targets[0].name, 'Staging 2');
  assert.equal(next.targets[1].name, 'Prod ');
});

test('the rename branch rewrites the raw untrimmed previous name as well as the trimmed one', () => {
  assertRenameRewritesRawAndTrimmedNames(extractStatement(source, 'handleTargetNameChange'), 'handleTargetNameChange');
});

test('negative control: a rename branch without the raw-name match is rejected by the validator', () => {
  const mock = 'targetNames: subscription.targetNames.map((targetName) => (targetName === previousName ? trimmedName : targetName))';
  assert.throws(() => assertRenameRewritesRawAndTrimmedNames(mock, 'mock'), /raw untrimmed/);
});

test('sandbox regression: stripping the raw-name match from a copy makes this suite fail', { skip: process.env.SKIP_SANDBOX_TEST }, () => {
  const sandbox = mkdtempSync(join(tmpdir(), 'webhook-settings-'));
  try {
    for (const rel of ['features/system/components/webhook-settings.tsx']) {
      cpSync(join(srcRoot, rel), join(sandbox, rel));
    }
    // The lifted-handler import needs the workspace 'typescript' package
    // (srcRoot is frontend/src; node_modules lives one level up).
    symlinkSync(join(srcRoot, '..', 'node_modules'), join(sandbox, 'node_modules'));
    const componentPath = join(sandbox, 'features/system/components/webhook-settings.tsx');
    const original = read('features/system/components/webhook-settings.tsx');
    const stripped = original.replace(
      /targetName\.trim\(\) === previousName \? trimmedName : targetName/,
      'targetName === previousName ? trimmedName : targetName'
    );
    assert.notEqual(stripped, original, 'sandbox mutation did not change the fixture');
    writeFileSync(componentPath, stripped);
    cpSync(join(componentsDir, 'webhook-settings.test.mjs'), join(sandbox, 'features/system/components/webhook-settings.test.mjs'));

    let output = '';
    try {
      // Strip the outer runner's child-context vars: a nested `node --test` that
      // inherits NODE_TEST_CONTEXT assumes a parent runner owns reporting and exits 0.
      const { NODE_TEST_CONTEXT: _runnerContext, NODE_TEST_WORKER_ID: _runnerWorker, ...innerEnv } = process.env;
      execFileSync(process.execPath, ['--test', 'features/system/components/webhook-settings.test.mjs'], {
        cwd: sandbox,
        encoding: 'utf8',
        env: { ...innerEnv, SKIP_SANDBOX_TEST: '1' },
      });
      assert.fail('sandboxed suite unexpectedly passed without the raw-name match');
    } catch (err) {
      output = err.stdout || String(err);
    }
    assert.match(output, /raw untrimmed previous name/, 'sandbox must fail on the raw-name rename test');
    assert.match(output, /fail 2\b/, 'the executed raw-name test and the source validator must both fail in the sandbox');
  } finally {
    rmSync(sandbox, { recursive: true, force: true });
  }
});
