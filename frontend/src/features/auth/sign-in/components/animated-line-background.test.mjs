import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const code = ts
  .transpileModule(readFileSync(new URL('./animated-line-background.tsx', import.meta.url), 'utf8'), {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2023,
      jsx: ts.JsxEmit.React,
      jsxFactory: 'createElement',
    },
  })
  .outputText.replace(
    "import { useCallback, useEffect, useRef } from 'react';",
    'const { useCallback, useEffect, useRef, createElement } = globalThis.animationTest;'
  )
  .replace("'./animated-line-background.engine'", JSON.stringify(new URL('./animated-line-background.engine.ts', import.meta.url).href))
  .replaceAll('import.meta.env.DEV', 'true');
let moduleIndex = 0;

async function setup(t, reduced = false) {
  const names = ['window', 'document', 'requestAnimationFrame', 'cancelAnimationFrame', 'animationTest'];
  const previous = new Map(names.map((name) => [name, Object.getOwnPropertyDescriptor(globalThis, name)]));
  const effects = [];
  const frames = new Map();
  let frameId = 0;
  const mediaQuery = new EventTarget();
  mediaQuery.matches = reduced;
  const browserWindow = new EventTarget();
  Object.assign(browserWindow, {
    innerWidth: 800,
    innerHeight: 600,
    location: { search: '?__axonhub_debug_animation=1' },
    matchMedia: () => mediaQuery,
  });
  const document = new EventTarget();
  document.visibilityState = 'visible';
  document.getElementById = () => null;
  const context = new Proxy({}, { get: () => () => {} });
  const canvas = { width: 0, height: 0, getContext: () => context };
  Object.assign(globalThis, {
    window: browserWindow,
    document,
    requestAnimationFrame: (callback) => {
      frames.set(++frameId, callback);
      return frameId;
    },
    cancelAnimationFrame: (id) => frames.delete(id),
    animationTest: {
      useCallback: (callback) => callback,
      useRef: (current) => ({ current }),
      useEffect: (effect) => effects.push(effect),
      createElement: (_type, props) => {
        props.ref.current = canvas;
      },
    },
  });
  const cleanups = [];
  t.after(() => {
    cleanups
      .splice(0)
      .reverse()
      .forEach((cleanup) => cleanup?.());
    for (const [name, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else delete globalThis[name];
    }
  });
  const module = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}#${moduleIndex++}`);
  module.default();
  effects.forEach((effect) => cleanups.push(effect()));
  return {
    frames,
    snapshot: () => browserWindow.__AXONHUB_SIGNIN_ANIMATION__.snapshot(),
    motion: (matches) => {
      mediaQuery.matches = matches;
      mediaQuery.dispatchEvent(new Event('change'));
    },
    visibility: (state) => {
      document.visibilityState = state;
      document.dispatchEvent(new Event('visibilitychange'));
    },
    tick: (timestamp) => {
      const pending = [...frames.values()];
      frames.clear();
      pending.forEach((callback) => callback(timestamp));
    },
    unmount: () =>
      cleanups
        .splice(0)
        .reverse()
        .forEach((cleanup) => cleanup?.()),
  };
}

test('changing motion preference stops the running canvas and resumes without catching up', async (t) => {
  const animation = await setup(t);
  animation.tick(100);
  animation.tick(120);
  assert.ok(animation.snapshot().simulationStepCount > 0);
  animation.motion(true);
  assert.equal(animation.frames.size, 0);
  const stopped = animation.snapshot();
  animation.tick(10_000);
  assert.deepEqual(animation.snapshot(), stopped);
  animation.motion(false);
  assert.equal(animation.frames.size, 1);
  animation.tick(20_000);
  assert.equal(animation.snapshot().simulationStepCount, stopped.simulationStepCount);
  animation.tick(20_020);
  assert.ok(animation.snapshot().simulationStepCount > stopped.simulationStepCount);
});

test('initial reduced motion renders a static frame and hidden pages never resume animation', async (t) => {
  const animation = await setup(t, true);
  assert.equal(animation.frames.size, 0);
  assert.ok(animation.snapshot().renderCount > 0);
  animation.visibility('hidden');
  animation.motion(false);
  assert.equal(animation.frames.size, 0);
  animation.visibility('visible');
  assert.equal(animation.frames.size, 1);
  animation.unmount();
  animation.motion(true);
  animation.motion(false);
  assert.equal(animation.frames.size, 0);
});
