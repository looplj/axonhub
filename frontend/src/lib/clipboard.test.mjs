import assert from 'node:assert/strict';
import test from 'node:test';
import { copyTextToClipboard } from './clipboard.ts';

function replaceGlobals(values) {
  const previous = new Map();

  for (const [name, value] of Object.entries(values)) {
    previous.set(name, Object.getOwnPropertyDescriptor(globalThis, name));
    Object.defineProperty(globalThis, name, {
      configurable: true,
      value,
    });
  }

  return () => {
    for (const [name, descriptor] of previous) {
      if (descriptor) {
        Object.defineProperty(globalThis, name, descriptor);
      } else {
        delete globalThis[name];
      }
    }
  };
}

function createFallbackDocument(copyResult) {
  const state = {
    appended: false,
    command: null,
    focused: false,
    range: null,
    removed: false,
    selected: false,
    textarea: null,
  };

  const document = {
    body: {
      appendChild(textarea) {
        state.appended = true;
        state.textarea = textarea;
      },
    },
    createElement(name) {
      assert.equal(name, 'textarea');
      return {
        attributes: {},
        focus() {
          state.focused = true;
        },
        remove() {
          state.removed = true;
        },
        select() {
          state.selected = true;
        },
        setAttribute(name, value) {
          this.attributes[name] = value;
        },
        setSelectionRange(start, end) {
          state.range = [start, end];
        },
        style: {},
        value: '',
      };
    },
    execCommand(command) {
      state.command = command;
      return copyResult;
    },
  };

  return { document, state };
}

test('uses the modern Clipboard API when available', async () => {
  let copiedText = null;
  const restore = replaceGlobals({
    navigator: {
      clipboard: {
        async writeText(text) {
          copiedText = text;
        },
      },
    },
  });

  try {
    await copyTextToClipboard('modern');
    assert.equal(copiedText, 'modern');
  } finally {
    restore();
  }
});

test('falls back to the document copy command when the Clipboard API is unavailable', async () => {
  const { document, state } = createFallbackDocument(true);
  const restore = replaceGlobals({ document, navigator: {} });

  try {
    await copyTextToClipboard('fallback');
    assert.equal(state.textarea.value, 'fallback');
    assert.equal(state.textarea.attributes.readonly, '');
    assert.equal(state.command, 'copy');
    assert.equal(state.appended, true);
    assert.equal(state.focused, true);
    assert.equal(state.selected, true);
    assert.deepEqual(state.range, [0, 8]);
    assert.equal(state.removed, true);
  } finally {
    restore();
  }
});

test('rejects when neither clipboard path can copy the text', async () => {
  const { document, state } = createFallbackDocument(false);
  const restore = replaceGlobals({ document, navigator: {} });

  try {
    await assert.rejects(copyTextToClipboard('failure'), /compatibility copy command failed/);
    assert.equal(state.removed, true);
  } finally {
    restore();
  }
});
