import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const componentsDir = import.meta.dirname;
const srcRoot = join(componentsDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

const source = read('features/system/components/quota-settings.tsx');

test('ChannelMultiSelect toggles selection on handleSelect', () => {
  // handleSelect toggles: if already included, filter out; otherwise append.
  assert.match(
    source,
    /handleSelect.*value\.includes\(channelId\).*value\.filter.*\[\.\.\.value, channelId\]/s,
    'handleSelect should toggle: remove if present, append if absent'
  );
});

test('ChannelMultiSelect handleRemove filters out the channel', () => {
  assert.match(
    source,
    /handleRemove.*channelId.*value\.filter\(\(v\)\s*=>\s*v\s*!==\s*channelId\)/s,
    'handleRemove should filter the clicked channelId from value'
  );
});

test('ChannelMultiSelect displays selected channel names via Badge', () => {
  // Selected IDs render as Badge chips showing the channel name (or fallback to id).
  assert.match(
    source,
    /value\.map\(.*Badge.*channel\?\.name\s*\|\|\s*channelId/s,
    'selected channels should render as Badge chips showing channel name or id fallback'
  );
});

test('ChannelMultiSelect Badge contains Button that calls handleRemove', () => {
  assert.match(
    source,
    /Badge[^>]*>[\s\S]*?Button[\s\S]*?onClick=\{\(\)\s*=>\s*handleRemove\(channelId\)\}/s,
    'Button inside Badge should call handleRemove for that channel'
  );
});

test('ChannelMultiSelect button label reflects selection count', () => {
  assert.match(
    source,
    /value\.length > 0.*t\(['"]system\.quota\.enforcement\.allowedChannels\.selectedCount['"]/s,
    'button should show selection count when channels are selected'
  );
});

test('ChannelMultiSelect Check icon visibility tracks selection state', () => {
  assert.match(
    source,
    /Check.*className=.*value\.includes\(channel\.id\)\s*\?\s*'opacity-100'\s*:\s*'opacity-0'/s,
    'Check icon should be visible only for selected channels'
  );
});
