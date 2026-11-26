import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Execute the shipped component with controlled hooks and mutation boundaries. The
// returned JSX tree retains its real event handlers and status/count computations.
const { outputText } = ts.transpileModule(
  readFileSync(new URL('./channels-api-key-management-dialog.tsx', import.meta.url), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2023 } }
);

function manager() {
  const ref = 'vertex-sa:synthetic-account';
  const state = [];
  let cursor = 0;
  let initialized = false;
  let disabled = false;
  const effects = [];
  const mutation = (mutateAsync = async () => {}) => ({ mutateAsync, isPending: false });
  const hooks = {
    useState(initial) {
      const slot = cursor++;
      if (!(slot in state)) state[slot] = initial;
      return [state[slot], (next) => { state[slot] = typeof next === 'function' ? next(state[slot]) : next; }];
    },
    useMemo: (compute) => compute(),
    useRef: (current) => ({ current }),
    useEffect: (effect) => { if (!initialized) effects.push(effect); },
  };
  const data = {
    useChannelDisabledAPIKeys: () => ({ data: disabled ? [{ key: ref }] : [], isLoading: false }),
    useDisableChannelAPIKey: () => mutation(async () => { disabled = true; }),
    useEnableChannelAPIKey: () => mutation(async () => { disabled = false; }),
    useTestChannelAPIKey: () => mutation(async () => ({ keyPrefix: 'synthetic', success: true, latency: 1, disabled })),
    useUpdateChannel: () => mutation(),
    useDeleteDisabledChannelAPIKeys: () => mutation(),
  };
  const currentRow = {
    id: 'channel-synthetic', type: 'gemini_vertex', name: 'synthetic',
    credentials: { apiKeys: ['synthetic-key'], vertexServiceAccounts: [{ credentialRef: ref, jsonData: '{}', projectID: 'synthetic' }] },
  };
  const modules = {
    react: hooks,
    'react/jsx-runtime': { jsx: (type, props) => ({ type, props }), jsxs: (type, props) => ({ type, props }) },
    'react-i18next': { useTranslation: () => ({ t: (key) => key }) },
    '../context/channels-context': { useChannels: () => ({ currentRow, setOpen() {} }) },
    '../data/channels': data,
    '../data/vertex-service-account': {
      getVertexServiceAccountIdentity: () => ({ clientEmail: 'synthetic@example.invalid' }),
      maskVertexCredentialRef: () => 'masked',
    },
  };
  const exports = {};
  new Function('require', 'exports', outputText)((name) => modules[name] ?? new Proxy({}, { get: (_, key) => key }), exports);
  function render() {
    cursor = 0;
    let tree = exports.ChannelsAPIKeyManagementDialog({ open: true, onOpenChange() {} });
    if (!initialized) {
      initialized = true;
      effects.forEach((effect) => effect());
      cursor = 0;
      tree = exports.ChannelsAPIKeyManagementDialog({ open: true, onOpenChange() {} });
    }
    return tree;
  }
  return { render, isServerDisabled: () => disabled };
}

function nodes(tree) {
  if (Array.isArray(tree)) return tree.flatMap(nodes);
  if (!tree || typeof tree !== 'object') return [];
  return [tree, ...nodes(tree.props?.children)];
}

function accountRow(tree) {
  return nodes(tree).find((node) => node.type === 'TableRow' && node.props.children?.[0]?.props.className === 'font-medium');
}

async function toggle(manager, label) {
  const row = accountRow(manager.render());
  const button = nodes(row).find((node) => node.props?.['aria-label'] === `channels.dialogs.keyManagement.${label}`);
  assert.ok(button, `${label} action must be visible`);
  const popover = nodes(row).find((node) => node.type === 'Popover' && nodes(node).includes(button));
  popover.props.onOpenChange(true);
  const confirmation = nodes(accountRow(manager.render())).find(
    (node) => node.type === 'Popover' && node.props.open
  );
  await nodes(confirmation).find((node) => node.type === 'Button' && node.props.children === 'common.buttons.confirm').props.onClick();
}

test('a tested disabled JSON credential becomes enabled after successful recovery', async () => {
  const view = manager();
  await toggle(view, 'disable');
  const testButton = nodes(accountRow(view.render())).find(
    (node) => node.props?.['aria-label'] === 'channels.dialogs.keyManagement.test'
  );
  await testButton.props.onClick();
  await toggle(view, 'enable');

  assert.equal(view.isServerDisabled(), false);
  const row = accountRow(view.render());
  assert.ok(nodes(row).some((node) => node.props?.['aria-label'] === 'channels.dialogs.keyManagement.disable'));
  assert.equal(nodes(row).some((node) => node.props?.children === 'channels.dialogs.keyManagement.disabled'), false);
});
