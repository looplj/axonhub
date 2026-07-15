import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
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


test('Cline has localized channel and provider labels', () => {
  for (const locale of ['en', 'zh-CN']) {
    const messages = parseLocale(locale);

    assert.equal(messages['channels.types.cline'], 'Cline');
    assert.equal(messages['channels.providers.cline'], 'Cline');
  }
});
