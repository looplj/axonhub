import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const code = ts
  .transpileModule(readFileSync(new URL('./nav-items.ts', import.meta.url), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  })
  .outputText.replace(/^import [\s\S]*?;\n/m, '');
const iconNames = [...readFileSync(new URL('./nav-items.ts', import.meta.url), 'utf8').matchAll(/\b(Icon\w+)\b/g)].map((m) => m[1]);
const bindings = `const { ${[...new Set(iconNames)].join(', ')} } = new Proxy({}, { get: (_, name) => name });\n`;
const navItems = await import(`data:text/javascript;base64,${Buffer.from(bindings + code).toString('base64')}`);

test('resolveNavModule maps routes to their module', () => {
  const { resolveNavModule } = navItems;

  assert.equal(resolveNavModule('/'), 'admin');
  assert.equal(resolveNavModule('/channels'), 'admin');
  assert.equal(resolveNavModule('/projects'), 'admin');
  assert.equal(resolveNavModule('/system'), 'admin');
  assert.equal(resolveNavModule('/system/requests'), 'admin');
  assert.equal(resolveNavModule('/dashboard/channel-success-rates'), 'admin');
  assert.equal(resolveNavModule('/project/requests'), 'project');
  assert.equal(resolveNavModule('/project/traces/abc?tab=spans'), 'project');
  assert.equal(resolveNavModule('/project/usage-stats/'), 'project');
  assert.equal(resolveNavModule('/plugins'), 'plugins');
  assert.equal(resolveNavModule('/plugins/demo/pages/home'), 'plugins');
});

test('resolveNavModule returns null for pages outside every module', () => {
  const { resolveNavModule } = navItems;

  assert.equal(resolveNavModule('/settings/profile'), null);
  assert.equal(resolveNavModule('/help-center'), null);
  assert.equal(resolveNavModule('/channelsx'), null);
});

test('every nav url belongs to the module that lists it', () => {
  const { NAV_MODULE_DEFS, resolveNavModule } = navItems;

  for (const module of NAV_MODULE_DEFS) {
    for (const group of module.groups) {
      for (const item of group.items) {
        assert.equal(resolveNavModule(item.url), module.id, item.url);
      }
    }
  }
});

test('flattened nav urls keep the full set of sidebar routes', () => {
  const { NAV_ITEM_URLS, PROJECT_NAV_ITEM_URLS } = navItems;

  assert.deepEqual(
    [...NAV_ITEM_URLS].sort(),
    [
      '/',
      '/channels',
      '/data-storages',
      '/models',
      '/plugins',
      '/project/api-keys',
      '/project/playground',
      '/project/prompts',
      '/project/requests',
      '/project/roles',
      '/project/threads',
      '/project/traces',
      '/project/usage-stats',
      '/project/users',
      '/projects',
      '/prompt-protection-rules',
      '/roles',
      '/system',
      '/system/integrations',
      '/system/operations',
      '/system/requests',
      '/users',
    ]
  );
  assert.ok(PROJECT_NAV_ITEM_URLS.every((url) => url.startsWith('/project/')));
});

test('pickFallbackNavUrl skips hidden routes', () => {
  const { pickFallbackNavUrl, PROJECT_NAV_ITEM_URLS } = navItems;

  assert.equal(pickFallbackNavUrl('/', [], true), '/');
  assert.equal(pickFallbackNavUrl('/', ['/'], true), '/projects');
  assert.equal(pickFallbackNavUrl('/project/requests', ['/project/requests'], false), PROJECT_NAV_ITEM_URLS[0]);
  assert.equal(pickFallbackNavUrl('/x', ['/x', ...PROJECT_NAV_ITEM_URLS], false), '/x');
});
