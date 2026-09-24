import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const srcRoot = join(import.meta.dirname, '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

// Issue #2229: the load-template popover lives inside ApiKeyProfilesDialog, a modal
// Dialog. When the PopoverContent is portaled to document.body it sits outside the
// dialog, the dialog scroll lock blocks wheel scrolling, and only the first ~4
// templates are reachable. The popover must portal into the dialog content element
// instead — the same portalContainer pattern used by AutoComplete (#287).

const popoverSource = read('features/apikeys/components/apikeys-load-template-popover.tsx');

test('load-template popover accepts a portal container prop', () => {
  assert.match(popoverSource, /portalContainer\?: HTMLElement \| null/);
  assert.match(
    popoverSource,
    /export function ApiKeyLoadTemplatePopover\(\{[\s\S]*?portalContainer[\s\S]*?\}: ApiKeyLoadTemplatePopoverProps\)/
  );
});

test('load-template popover portals PopoverContent into the provided container', () => {
  const popoverContent = popoverSource.match(/<PopoverContent[\s\S]*?>/);
  assert.ok(popoverContent, 'PopoverContent element not found');
  assert.match(popoverContent[0], /container=\{portalContainer\}/);
});

test('profiles dialog passes its dialog content element to the load-template popover', () => {
  const dialogSource = read('features/apikeys/components/apikeys-profiles-dialog.tsx');
  const usage = dialogSource.match(/<ApiKeyLoadTemplatePopover[\s\S]*?\/>/);
  assert.ok(usage, 'ApiKeyLoadTemplatePopover usage not found');
  assert.match(usage[0], /portalContainer=\{dialogContent\}/);
});
