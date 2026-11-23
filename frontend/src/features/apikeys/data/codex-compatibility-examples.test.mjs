import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { registerHooks } from 'node:module';
import { extname, join, relative } from 'node:path';
import test, { mock } from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';
import ts from 'typescript';

const dataDir = import.meta.dirname;
const srcRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

function loadDataModule(relativePath) {
  const source = read(relativePath);
  const transpiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  }).outputText;

  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);
}

const { buildCodexCompatibilityExamples } = await loadDataModule('features/apikeys/data/codex-compatibility-examples.ts');

test('codexCompatibility separates config.toml, auth.json, and shell env with standalone copy payloads', () => {
  const apiKey = 'ah-secret-key-1234';
  const maskedApiKey = 'ah-...1234';
  const examples = buildCodexCompatibilityExamples({
    serverURL: 'http://localhost:8090',
    apiKey,
    maskedApiKey,
  });

  assert.equal(examples.baseUrl, 'http://localhost:8090/codex');

  // 1. config.toml must contain valid TOML assignments only (no JSON object, no shell exports)
  assert.equal(examples.configToml.language, 'toml');
  assert.equal(examples.configToml.real, examples.configToml.display);
  assert.match(examples.configToml.real, /^model = "gpt-6\.1-sol"/m);
  assert.match(examples.configToml.real, /^openai_base_url = "http:\/\/localhost:8090\/codex"/m);
  assert.match(examples.configToml.real, /^\[features\]/m);
  assert.doesNotMatch(examples.configToml.real, /personal_access_token|OPENAI_API_KEY|export\s+CODEX_AUTHAPI_BASE_URL|\{|\}/);

  // 2. auth.json must parse directly as valid JSON (both real and masked display payloads)
  assert.equal(examples.authJson.language, 'json');
  assert.deepEqual(JSON.parse(examples.authJson.real), {
    personal_access_token: apiKey,
    OPENAI_API_KEY: apiKey,
  });
  assert.deepEqual(JSON.parse(examples.authJson.display), {
    personal_access_token: maskedApiKey,
    OPENAI_API_KEY: maskedApiKey,
  });
  assert.ok(!examples.authJson.display.includes(apiKey));

  // 3. shell env snippet contains only the export / Windows instruction
  assert.equal(examples.shellEnv.language, 'bash');
  assert.match(examples.shellEnv.real, /^export CODEX_AUTHAPI_BASE_URL="http:\/\/localhost:8090\/codex"/m);
  assert.doesNotMatch(examples.shellEnv.real, /\[features\]|personal_access_token|OPENAI_API_KEY/);
});

const spyKey = '__codexCompatDialogTestSpy';

function createInitialSpyState() {
  return {
    codexCompatibilityEnabled: true,
    apiKey: 'ah-secret-key-1234',
    capturedButtons: [],
    clipboardCopies: [],
    hookCopies: [],
  };
}

globalThis[spyKey] = createInitialSpyState();

const MOCK_MODULES = {
  'lucide-react': `
    const Icon = () => null;
    export const Copy = Icon;
    export const CopyIcon = Icon;
    export const Eye = Icon;
    export const EyeOff = Icon;
    export const AlertTriangle = Icon;
    export const Link = Icon;
    export const CheckIcon = Icon;
  `,
  shiki: `
    export async function codeToHtml(code) {
      return '<pre>' + code + '</pre>';
    }
  `,
  'react-i18next': `
    export function useTranslation() {
      return { t: (key) => key };
    }
  `,
  'lib/clipboard': `
    export async function copyTextToClipboard(text) {
      globalThis['${spyKey}'].clipboardCopies.push(text);
    }
  `,
  'lib/app-path': `
    export function getBasePath() {
      return '';
    }
  `,
  'lib/utils': `
    export function cn(...inputs) {
      return inputs.filter(Boolean).join(' ');
    }
  `,
  'hooks/use-copy-to-clipboard': `
    export function useCopyToClipboard({ text }) {
      return {
        isCopied: false,
        handleCopy: () => {
          globalThis['${spyKey}'].hookCopies.push(text);
        },
      };
    }
  `,
  'components/ui/tabs': `
    import { createContext, useContext } from 'react';
    export const TabScopeContext = createContext('');
    export function Tabs({ children }) {
      return <div data-testid='tabs'>{children}</div>;
    }
    export function TabsList({ children }) {
      return <div>{children}</div>;
    }
    export function TabsTrigger({ value, children }) {
      return <button type='button' data-tab-trigger={value}>{children}</button>;
    }
    export function TabsContent({ value, children }) {
      return (
        <TabScopeContext.Provider value={value}>
          <div data-tab-content={value}>{children}</div>
        </TabScopeContext.Provider>
      );
    }
  `,
  'components/ui/button': `
    import { useContext } from 'react';
    import { TabScopeContext } from '@/components/ui/tabs';
    export function Button({ onClick, children, ...props }) {
      const tab = useContext(TabScopeContext);
      if (tab && onClick) {
        globalThis['${spyKey}'].capturedButtons.push({ tab, onClick });
      }
      return <button type='button' onClick={onClick} {...props}>{children}</button>;
    }
  `,
  'components/ui/dialog': `
    const Pass = ({ children }) => <div>{children}</div>;
    export const Dialog = Pass;
    export const DialogContent = Pass;
    export const DialogDescription = Pass;
    export const DialogHeader = Pass;
    export const DialogTitle = Pass;
  `,
  'components/ui/alert': `
    const Pass = ({ children }) => <div>{children}</div>;
    export const Alert = Pass;
    export const AlertDescription = Pass;
  `,
  'components/ui/tooltip': `
    const Pass = ({ children }) => <>{children}</>;
    export const Tooltip = Pass;
    export const TooltipContent = Pass;
    export const TooltipTrigger = Pass;
  `,
  'features/models/data/models': `
    export function useQueryAllModels() {
      return { data: undefined };
    }
  `,
  'features/system/data/codex-compatibility-api': `
    export function useCodexCompatibilityEnabled() {
      return { data: globalThis['${spyKey}'].codexCompatibilityEnabled };
    }
  `,
  'features/apikeys/context/apikeys-context': `
    export function useApiKeysContext() {
      return {
        isDialogOpen: { view: true },
        closeDialog: () => {},
        selectedApiKey: {
          id: 'key-1',
          name: 'Test Key',
          type: 'personal',
          key: globalThis['${spyKey}'].apiKey,
        },
      };
    }
  `,
  'features/apikeys/data/apikeys': `
    export function useApiKey() {
      return { data: undefined };
    }
  `,
};

function normalizeToSrcRel(targetPath) {
  const rel = relative(srcRoot, targetPath).replaceAll('\\', '/');
  if (rel.startsWith('..')) return null;
  return rel.replace(/\.(?:tsx?|mjs|js)$/, '');
}

function resolveWithExtension(basePath) {
  if (existsSync(basePath) && !extname(basePath)) {
    // Directories are not module files; probe explicit extensions below.
  } else if (existsSync(basePath)) {
    return basePath;
  }
  for (const ext of ['.tsx', '.ts', '.mjs', '.js']) {
    if (existsSync(`${basePath}${ext}`)) {
      return `${basePath}${ext}`;
    }
  }
  return null;
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (MOCK_MODULES[specifier]) {
      return { url: `virtual-mock:${specifier}`, shortCircuit: true };
    }

    let candidatePath = null;
    if (specifier.startsWith('@/')) {
      candidatePath = join(srcRoot, specifier.slice(2));
    } else if ((specifier.startsWith('./') || specifier.startsWith('../')) && context.parentURL?.startsWith('file://')) {
      const parentDir = join(fileURLToPath(context.parentURL), '..');
      candidatePath = join(parentDir, specifier);
    }

    if (candidatePath) {
      const srcRel = normalizeToSrcRel(candidatePath);
      if (srcRel && MOCK_MODULES[srcRel]) {
        return { url: `virtual-mock:${srcRel}`, shortCircuit: true };
      }
      const resolvedFile = resolveWithExtension(candidatePath);
      if (resolvedFile) {
        return { url: pathToFileURL(resolvedFile).href, shortCircuit: true };
      }
    }

    const safeContext = context.parentURL?.startsWith('virtual-mock:') ? { ...context, parentURL: import.meta.url } : context;
    return nextResolve(specifier, safeContext);
  },
  load(url, context, nextLoad) {
    if (url.startsWith('virtual-mock:')) {
      const key = url.slice('virtual-mock:'.length);
      const transpiled = ts.transpileModule(MOCK_MODULES[key], {
        fileName: `${key}.tsx`,
        compilerOptions: {
          module: ts.ModuleKind.ESNext,
          target: ts.ScriptTarget.ES2023,
          jsx: ts.JsxEmit.ReactJSX,
        },
      }).outputText;
      return { format: 'module', shortCircuit: true, source: transpiled };
    }

    if (url.startsWith('file://') && (url.endsWith('.ts') || url.endsWith('.tsx'))) {
      const filePath = fileURLToPath(url);
      const source = readFileSync(filePath, 'utf8');
      const transpiled = ts.transpileModule(source, {
        fileName: filePath,
        compilerOptions: {
          module: ts.ModuleKind.ESNext,
          target: ts.ScriptTarget.ES2023,
          jsx: ts.JsxEmit.ReactJSX,
        },
      }).outputText;
      return { format: 'module', shortCircuit: true, source: transpiled };
    }

    return nextLoad(url, context);
  },
});

const React = await import('react');
const { renderToStaticMarkup } = await import('react-dom/server');
const { ApiKeysViewDialog } = await import('../components/apikeys-view-dialog.tsx');

test('ApiKeysViewDialog Codex tab renders three destinations with independent copy buttons that copy distinct payloads', async () => {
  const apiKey = 'ah-secret-key-1234';
  const expected = buildCodexCompatibilityExamples({
    serverURL: 'http://localhost:8090',
    apiKey,
    maskedApiKey: `${apiKey.slice(0, 3)}...${apiKey.slice(-4)}`,
  });

  const spy = createInitialSpyState();
  spy.apiKey = apiKey;
  globalThis[spyKey] = spy;

  const html = renderToStaticMarkup(React.createElement(ApiKeysViewDialog));
  assert.match(html, /data-tab-content="codexCompatibility"/);
  assert.match(html, /apikeys\.dialogs\.view\.codexCompatibility\.configTomlTitle/);
  assert.match(html, /apikeys\.dialogs\.view\.codexCompatibility\.authJsonTitle/);
  assert.match(html, /apikeys\.dialogs\.view\.codexCompatibility\.shellEnvTitle/);

  const codexCompatButtons = spy.capturedButtons.filter((entry) => entry.tab === 'codexCompatibility');
  assert.equal(
    codexCompatButtons.length,
    5,
    'expected 2 CopyBaseUrlButton triggers (configToml, shellEnv) and 3 MaskedCodeBlockCopyButton triggers'
  );

  mock.timers.enable({ apis: ['setTimeout'] });
  try {
    for (const button of codexCompatButtons) {
      await button.onClick();
    }
  } finally {
    mock.timers.reset();
  }

  assert.deepEqual(spy.hookCopies, [expected.baseUrl, expected.baseUrl]);
  assert.equal(spy.clipboardCopies.length, 3);
  assert.equal(spy.clipboardCopies[0], expected.configToml.real);
  assert.equal(spy.clipboardCopies[1], expected.authJson.real);
  assert.equal(spy.clipboardCopies[2], expected.shellEnv.real);

  assert.deepEqual(JSON.parse(spy.clipboardCopies[1]), {
    personal_access_token: apiKey,
    OPENAI_API_KEY: apiKey,
  });

  spy.codexCompatibilityEnabled = false;
  spy.capturedButtons = [];
  const disabledHtml = renderToStaticMarkup(React.createElement(ApiKeysViewDialog));
  assert.doesNotMatch(disabledHtml, /data-tab-content="codexCompatibility"/);
  assert.equal(spy.capturedButtons.filter((entry) => entry.tab === 'codexCompatibility').length, 0);

  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(read(`locales/${locale}/apikeys.json`));
    assert.ok(messages['apikeys.dialogs.view.codexCompatibility.configTomlTitle']);
    assert.ok(messages['apikeys.dialogs.view.codexCompatibility.authJsonTitle']);
    assert.ok(messages['apikeys.dialogs.view.codexCompatibility.shellEnvTitle']);
  }
});
