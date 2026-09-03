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

test('channel table shows provider quota only for OAuth channel types', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsData = read('features/channels/data/channels.ts');
  const channelColumns = read('features/channels/components/channels-columns.tsx');

  assert.match(schema, /providerQuotaStatus:\s*providerQuotaStatusSchema\.optional\(\)\.nullable\(\)/);
  assert.match(
    channelsData,
    /providerQuotaStatus\s*\{[\s\S]*status[\s\S]*quotaData[\s\S]*providerType[\s\S]*\}/,
    'channel list query should load the persisted provider quota status'
  );
  const oauthTypes = channelColumns.match(/const OAUTH_CHANNEL_TYPES\s*=\s*new Set<Channel\['type'\]>\(\[([\s\S]*?)\]\);/)?.[1];
  assert.ok(oauthTypes, 'OAuth channel type Set declaration should exist');
  for (const type of ['codex', 'claudecode', 'antigravity', 'github_copilot', 'xai_subscription']) {
    assert.match(oauthTypes, new RegExp(`'${type}'`));
  }
  assert.match(channelColumns, /100\s*-\s*usageRatio\s*\*\s*100/, 'the table should display remaining quota percentage');
  assert.match(channelColumns, /QUOTA_VISIBLE_LIMIT\s*=\s*5/, 'quota cells should initially show at most five rows');
  assert.match(channelColumns, /isExpanded\s*\?\s*limits\s*:\s*limits\.slice\(0,\s*QUOTA_VISIBLE_LIMIT\)/);
  assert.match(channelColumns, /channels\.quota\.expand/);
  assert.match(channelColumns, /channels\.quota\.collapse/);
  assert.doesNotMatch(channelColumns, /limit\.window\s*=\s*labels\[index\]/, 'xAI windows must not be labeled by array position');
  assert.match(channelColumns, /Math\.abs\(limit\.usageRatio\s*-\s*usageRatio\)/, 'legacy xAI limits should match raw billing usage');
  assert.match(
    channelColumns,
    /if\s*\(!OAUTH_CHANNEL_TYPES\.has\(channel\.type\)\)[\s\S]*?>-<\/span>/,
    'non-OAuth channels should display a dash'
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
test('Command Code exposes OpenAI and Anthropic channel variants with shared base URL', () => {
  const schema = read('features/channels/data/schema.ts');
  const channelsConfig = read('features/channels/data/config_channels.ts');
  const providersConfig = read('features/channels/data/config_providers.ts');
  const systemQuotas = read('features/system/data/quotas.ts');
  const dialog = read('features/channels/components/channels-action-dialog.tsx');

  assert.match(schema, /channelTypeSchema[\s\S]*'commandcode'[\s\S]*'commandcode_anthropic'/);
  assert.match(
    schema,
    /commandCodeQuotaSettingsSchema[\s\S]*authCookie:[\s\S]*z\.string\(\)\.optional\(\)\.nullable\(\)/,
    'schema should model the Command Code quota cookie'
  );
  assert.match(schema, /channelSettingsSchema[\s\S]*providerQuota:[\s\S]*channelProviderQuotaSettingsSchema/);
  assert.match(
    channelsConfig,
    /commandcode:\s*{[\s\S]*?channelType:\s*'commandcode'[\s\S]*?baseURL:\s*'https:\/\/api\.commandcode\.ai\/provider\/v1'[\s\S]*?defaultModels:\s*\[\][\s\S]*?apiFormat:\s*OPENAI_CHAT_COMPLETIONS,/,
    'commandcode should use the shared base URL with OpenAI chat completions and no static models'
  );
  assert.match(
    channelsConfig,
    /commandcode_anthropic:\s*{[\s\S]*?channelType:\s*'commandcode_anthropic'[\s\S]*?baseURL:\s*'https:\/\/api\.commandcode\.ai\/provider\/v1'[\s\S]*?defaultModels:\s*\[\][\s\S]*?apiFormat:\s*ANTHROPIC_MESSAGES,/,
    'commandcode_anthropic should use the shared base URL with Anthropic messages and no static models'
  );
  assert.match(channelsConfig, /CHANNEL_TYPE_TO_PROVIDER[\s\S]*commandcode_anthropic:\s*'commandcode'/);
  assert.match(
    providersConfig,
    /commandcode:\s*{[\s\S]*channelTypes:\s*\[\s*'commandcode',\s*'commandcode_anthropic'\s*\]/,
    'PROVIDER_CONFIGS should group both Command Code channel types'
  );
  assert.match(
    systemQuotas,
    /type:\s*'commandcode'\s*\|\s*'commandcode_anthropic';[\s\S]*quotaData: ProviderCommandCodeQuotaData/,
    'quota parsing should type Command Code channels with ProviderCommandCodeQuotaData'
  );
  assert.match(
    dialog,
    /settings\.providerQuota\.commandCode\.authCookie/,
    'the channel dialog should bind the quota cookie input to settings.providerQuota.commandCode.authCookie'
  );
});

test('Command Code has localized channel, provider, cookie field, and quota labels', () => {
  for (const locale of ['en', 'zh-CN']) {
    const channels = parseLocale(locale);
    const system = JSON.parse(read(`locales/${locale}/system.json`));

    assert.equal(channels['channels.types.commandcode'], 'Command Code');
    assert.equal(channels['channels.providers.commandcode'], 'Command Code');
    assert.ok(channels['channels.types.commandcode_anthropic']);
    assert.ok(channels['channels.dialogs.fields.commandCodeQuota.authCookie.placeholder'].includes('__Secure-commandcode_prod_.session_token'));
    assert.ok(channels['channels.dialogs.fields.commandCodeQuota.authCookie.description']);
    assert.ok(system['quota.label.commandcode.top_up']);
    assert.ok(system['quota.label.commandcode.no_windows']);
    assert.ok(system['system.quota.collection.providers.commandcode']);
  }
});
