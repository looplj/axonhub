import assert from 'node:assert/strict';
import test from 'node:test';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const dataDir = import.meta.dirname;
const srcRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

function parseLocale(locale) {
  return JSON.parse(read(`locales/${locale}/channels.json`));
}

test('Cline is available as a channel type in frontend schemas and configs', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsConfig = read('features/channels/data/config_channels.ts');
  const providersConfig = read('features/channels/data/config_providers.ts');

  assert.match(schema, /channelTypeSchema[\s\S]*'cline'/, 'channelTypeSchema should accept cline');
  assert.match(channelsConfig, /cline:\s*{[\s\S]*channelType:\s*'cline'/, 'CHANNEL_CONFIGS should define cline');
  assert.match(channelsConfig, /cline:\s*{[\s\S]*baseURL:\s*'https:\/\/api\.cline\.bot\/api\/v1'/, 'Cline should use the documented API base URL');
  assert.match(channelsConfig, /cline:\s*{[\s\S]*apiFormat:\s*OPENAI_CHAT_COMPLETIONS/, 'Cline should use OpenAI Chat Completions in the UI');
  assert.match(channelsConfig, /CHANNEL_TYPE_TO_PROVIDER[\s\S]*cline:\s*'cline'/, 'Cline should map to the Cline provider');
  assert.match(providersConfig, /cline:\s*{[\s\S]*channelTypes:\s*\[\s*'cline'\s*\]/, 'PROVIDER_CONFIGS should expose a Cline provider');
});

test('Qiniu exposes OpenAI and Anthropic channel variants after AtlasCloud', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsConfig = read('features/channels/data/config_channels.ts');
  const providersConfig = read('features/channels/data/config_providers.ts');

  assert.match(schema, /channelTypeSchema[\s\S]*'qiniu'[\s\S]*'qiniu_anthropic'/);
  assert.match(channelsConfig, /qiniu:\s*{[\s\S]*baseURL:\s*'https:\/\/api\.qnaigc\.com\/v1'[\s\S]*apiFormat:\s*OPENAI_CHAT_COMPLETIONS/);
  assert.match(channelsConfig, /qiniu_anthropic:\s*{[\s\S]*baseURL:\s*'https:\/\/api\.qnaigc\.com'[\s\S]*apiFormat:\s*ANTHROPIC_MESSAGES/);
  assert.match(providersConfig, /qiniu:\s*{[\s\S]*channelTypes:\s*\[\s*'qiniu_anthropic',\s*'qiniu'\s*\]/);
  assert.ok(channelsConfig.indexOf('atlascloud:') < channelsConfig.indexOf('qiniu:'));
  assert.ok(providersConfig.indexOf('atlascloud:') < providersConfig.indexOf('qiniu:'));
});

test('Fenno exposes a third-party Codex channel', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsConfig = read('features/channels/data/config_channels.ts');
  const providersConfig = read('features/channels/data/config_providers.ts');

  assert.match(schema, /channelTypeSchema[\s\S]*'fenno'/);
  assert.match(channelsConfig, /fenno:\s*{[\s\S]*baseURL:\s*'https:\/\/api\.fenno\.ai'[\s\S]*apiFormat:\s*OPENAI_RESPONSES[\s\S]*icon:\s*FennoIcon/);
  assert.match(channelsConfig, /fenno:\s*{[\s\S]*color:\s*'bg-\[#EEF2FF\] text-\[#3155C6\] border-\[#C7D2FE\]'/);
  assert.match(providersConfig, /fenno:\s*{[\s\S]*icon:\s*FennoIcon[\s\S]*channelTypes:\s*\[\s*'fenno'\s*\]/);
  const fennoIcon = read('features/channels/components/fenno-icon.tsx');
  assert.match(fennoIcon, /@\/assets\/fenno-icon\.webp/);
  assert.doesNotMatch(fennoIcon, /https?:\/\//);
  assert.ok(existsSync(join(srcRoot, 'assets/fenno-icon.webp')));
  assert.ok(channelsConfig.indexOf('qiniu_anthropic:') < channelsConfig.indexOf('fenno:'));
  assert.ok(providersConfig.indexOf('qiniu:') < providersConfig.indexOf('fenno:'));
});

test('Bailian exposes native Responses with the documented Beijing models', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsConfig = read('features/channels/data/config_channels.ts');
  const providersConfig = read('features/channels/data/config_providers.ts');

  assert.match(schema, /channelTypeSchema[\s\S]*'bailian_responses'/);
  assert.match(
    channelsConfig,
    /bailian_responses:\s*{[\s\S]*baseURL:\s*'https:\/\/dashscope\.aliyuncs\.com\/compatible-mode\/v1'[\s\S]*apiFormat:\s*OPENAI_RESPONSES/
  );
  assert.match(providersConfig, /bailian:\s*{[\s\S]*channelTypes:\s*\[\s*'bailian',\s*'bailian_anthropic',\s*'bailian_responses'\s*\]/);

  const configBlock = channelsConfig.match(/bailian_responses:\s*{([\s\S]*?)\n  },\n  bailian_anthropic:/)?.[1] ?? '';
  const modelsBlock = configBlock.match(/defaultModels:\s*\[([\s\S]*?)\]/)?.[1] ?? '';
  const models = [...modelsBlock.matchAll(/'([^']+)'/g)].map((match) => match[1]);
  assert.deepEqual(models, [
    'qwen3.8-max',
    'qwen3.7-max',
    'qwen3.7-max-2026-05-20',
    'qwen3.7-max-2026-06-08',
    'qwen3-max',
    'qwen3-max-2026-01-23',
    'qwen3.7-plus',
    'qwen3.7-plus-2026-05-26',
    'qwen3.6-plus',
    'qwen3.6-plus-2026-04-02',
    'qwen3.5-plus',
    'qwen3.5-plus-2026-04-20',
    'qwen3.5-plus-2026-02-15',
    'qwen3.7-flash',
    'qwen3.7-flash-2026-07-15',
    'qwen3.6-flash',
    'qwen3.6-flash-2026-04-16',
    'qwen3.5-flash',
    'qwen3.5-flash-2026-02-23',
    'qwen3.8-2.4t-a95b',
    'qwen3.8-27b',
    'qwen3.6-35b-a3b',
    'qwen3.5-397b-a17b',
    'qwen3.5-122b-a10b',
    'qwen3.5-27b',
    'qwen3.5-35b-a3b',
    'qwen-plus',
    'qwen-flash',
    'qwen3-coder-plus',
    'qwen3-coder-flash',
    'qwen3.5-ocr',
    'qwen-plus-character',
    'qwen-flash-character',
    'deepseek-v4-pro',
    'deepseek-v4-pro-0813',
    'deepseek-v4-flash',
    'deepseek-v4-flash-0731',
    'glm-5.2',
  ]);

  const en = parseLocale('en');
  const zh = parseLocale('zh-CN');
  assert.equal(en['channels.types.bailian_responses'], 'Bailian (Responses)');
  assert.equal(zh['channels.types.bailian_responses'], '百炼 (Responses)');
});

test('Cline has localized channel and provider labels', () => {
  for (const locale of ['en', 'zh-CN']) {
    const messages = parseLocale(locale);

    assert.equal(messages['channels.types.cline'], 'Cline');
    assert.equal(messages['channels.providers.cline'], 'Cline');
  }
});

test('xAI subscription is exposed as an OAuth Responses channel', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsConfig = read('features/channels/data/config_channels.ts');
  const providersConfig = read('features/channels/data/config_providers.ts');
  const channelColumns = read('features/channels/components/channels-columns.tsx');

  assert.match(schema, /channelTypeSchema[\s\S]*'xai_subscription'/);
  assert.equal((schema.match(/data\.type === 'xai_subscription'/g) ?? []).length, 1, 'create schema should validate xAI OAuth credentials');
  assert.match(schema, /effectiveType === 'xai_subscription'/, 'update schema should validate xAI OAuth credentials');
  assert.match(
    schema,
    /requiresJSON\s*=\s*isCopilot\s*\|\|\s*type\s*===\s*'xai_subscription'[\s\S]*if\s*\(requiresJSON\s*&&\s*!apiKey\.trim\(\)\.startsWith\('\{'\)\)/,
    'xAI subscription should reject a plain API key before the generic JSON early return'
  );
  assert.match(
    channelsConfig,
    /xai_subscription:\s*{[\s\S]*baseURL:\s*'https:\/\/cli-chat-proxy\.grok\.com\/v1'[\s\S]*apiFormat:\s*OPENAI_RESPONSES/
  );
  assert.match(providersConfig, /xai_subscription:\s*{[\s\S]*channelTypes:\s*\[\s*'xai_subscription'\s*\]/);
  assert.match(
    channelColumns,
    /channel\.type !== 'xai_subscription'\s*&&\s*\([\s\S]*setOpen\('endpoints'\)/,
    'xAI subscription channels should not expose an endpoint editor that the server rejects'
  );
});

test('channel proxy connection reuse setting is submitted, echoed, and localized', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsData = read('features/channels/data/channels.ts');
  const proxyDialog = read('features/channels/components/channels-proxy-dialog.tsx');

  assert.match(
    schema,
    /proxyConfigSchema[\s\S]*disableConnectionReuse:\s*z\.boolean\(\)\.optional\(\)/,
    'ProxyConfig schema should accept disableConnectionReuse'
  );

  const proxySelections = channelsData.match(/proxy\s*\{[\s\S]*?\}/g) ?? [];
  assert.equal(proxySelections.length, 5, 'all five channel proxy selections should be covered by this assertion');
  for (const selection of proxySelections) {
    assert.match(selection, /disableConnectionReuse/, 'channel proxy queries should echo disableConnectionReuse');
  }
  assert.match(channelsData, /proxy\?:\s*ProxyConfig;/, 'channel test input should use the shared ProxyConfig type');

  assert.match(proxyDialog, /name='disableConnectionReuse'/, 'proxy dialog should render the connection reuse switch');
  const submitSection = proxyDialog.slice(proxyDialog.indexOf('const onSubmit'), proxyDialog.indexOf('const handleTest'));
  const testSection = proxyDialog.slice(proxyDialog.indexOf('const handleTest'), proxyDialog.indexOf('return ('));
  assert.match(
    submitSection,
    /const proxyConfig[\s\S]*disableConnectionReuse:\s*values\.disableConnectionReuse/,
    'channel save payload should send disableConnectionReuse'
  );
  assert.match(
    testSection,
    /const proxyConfig[\s\S]*disableConnectionReuse:\s*values\.disableConnectionReuse/,
    'channel test payload should send disableConnectionReuse'
  );
  const presetPayload = submitSection.match(/saveProxyPreset\.mutate\(\{[\s\S]*?\}\);/)?.[0] ?? '';
  assert.doesNotMatch(presetPayload, /disableConnectionReuse/, 'proxy presets should remain address and credential only');
  assert.match(
    proxyDialog,
    /channels\.dialogs\.proxy\.fields\.disableConnectionReuse\.description/,
    'proxy dialog should render the explanatory text below the option'
  );

  const en = parseLocale('en');
  assert.equal(en['channels.dialogs.proxy.fields.disableConnectionReuse.label'], 'Use a new proxy connection for every request');
  assert.equal(
    en['channels.dialogs.proxy.fields.disableConnectionReuse.description'],
    'Enable this for proxy pools such as Resin that rotate nodes per connection. Each request will create a new proxy connection, increasing CONNECT and TLS handshake overhead.'
  );

  const zh = parseLocale('zh-CN');
  assert.equal(zh['channels.dialogs.proxy.fields.disableConnectionReuse.label'], '每次请求使用新的代理连接');
  assert.equal(
    zh['channels.dialogs.proxy.fields.disableConnectionReuse.description'],
    '适用于 Resin 等按连接切换节点的代理池。开启后每个请求都会重新建立代理连接，并增加 CONNECT 与 TLS 握手开销。'
  );
});
