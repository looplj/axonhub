import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createContext, runInContext } from "node:vm";

const script = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "bridge.js"), "utf8");
const origin = "http://127.0.0.1:8090";

class MessageEvent {
  constructor(type, init = {}) {
    this.type = type;
    this.data = init.data;
    this.source = init.source;
    this.origin = init.origin;
  }
}

function createStyle() {
  const values = new Map();
  return {
    values,
    setProperty(name, value) {
      values.set(name, value);
    },
    removeProperty(name) {
      values.delete(name);
    },
  };
}

function setup() {
  const sent = [];
  const timers = [];
  let timerSeq = 0;
  const listeners = new Map();
  const style = createStyle();
  const document = { documentElement: { style } };
  const window = {
    location: { href: `${origin}/admin/plugin-ui/token-value/ui/overview/index.html` },
    addEventListener(type, fn) {
      if (!listeners.has(type)) {
        listeners.set(type, new Set());
      }
      listeners.get(type).add(fn);
    },
    removeEventListener(type, fn) {
      listeners.get(type)?.delete(fn);
    },
    dispatchEvent(event) {
      for (const fn of [...(listeners.get(event.type) ?? [])]) {
        fn(event);
      }
    },
    setTimeout(fn, ms) {
      const id = ++timerSeq;
      timers.push({ id, fn, ms });
      return id;
    },
    clearTimeout(id) {
      const index = timers.findIndex((timer) => timer.id === id);
      if (index >= 0) {
        timers.splice(index, 1);
      }
    },
    queueMicrotask(fn) {
      queueMicrotask(fn);
    },
  };
  window.parent = {
    postMessage(data, targetOrigin) {
      sent.push({ data, origin: targetOrigin });
    },
  };
  const sandbox = {
    window,
    document,
    URL,
    Promise,
    queueMicrotask,
  };
  createContext(sandbox);
  runInContext(script, sandbox, { filename: "bridge.js" });
  return {
    window,
    style,
    sent,
    advance(ms) {
      const due = timers.filter((timer) => timer.ms <= ms);
      for (const timer of due) {
        const index = timers.indexOf(timer);
        if (index >= 0) {
          timers.splice(index, 1);
        }
      }
      for (const timer of due) {
        timer.fn();
      }
    },
  };
}

const initPayload = {
  theme: "dark",
  colorScheme: "zinc",
  locale: "en",
  projectId: 3,
  page: "main",
  panel: null,
  slot: null,
  config: { threshold: 1 },
  canWriteKV: false,
};

function deliver(env, data, source = env.window.parent, eventOrigin = origin) {
  env.window.dispatchEvent(new MessageEvent("message", {
    data,
    source,
    origin: eventOrigin,
  }));
}

function initMessage(sessionId = "sess-1") {
  return {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "init",
    sessionId,
    payload: initPayload,
  };
}

async function handshake(env, sessionId = "sess-1") {
  const pending = env.window.axonhub.ready();
  deliver(env, initMessage(sessionId));
  return pending;
}

function calls(env) {
  return env.sent.filter((message) => message.data.type === "call");
}

function json(value) {
  return JSON.parse(JSON.stringify(value));
}

function reply(id, body, sessionId = "sess-1") {
  return {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "reply",
    sessionId,
    id,
    ...body,
  };
}

test("duplicate ready sends one hello and returns the same promise", async () => {
  const env = setup();
  const first = env.window.axonhub.ready();
  const second = env.window.axonhub.ready();
  assert.equal(first, second);
  const hellos = env.sent.filter((message) => message.data.type === "hello");
  assert.equal(hellos.length, 1);
  assert.equal(hellos[0].origin, origin);
  assert.deepEqual(json(hellos[0].data), {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "hello",
  });

  deliver(env, initMessage());
  const init = await first;
  assert.equal(init.page, "main");
  assert.equal(init.theme, "dark");
  assert.equal(env.sent.filter((message) => message.data.type === "hello").length, 1);
  assert.equal(env.window.axonhub.ready(), first);
});

test("late onInit receives the cached init and unsubscribe stops later calls", async () => {
  const env = setup();
  let early = 0;
  const removeEarly = env.window.axonhub.onInit(() => {
    early += 1;
  });
  removeEarly();
  await handshake(env);
  assert.equal(early, 0);

  let late = null;
  const removeLate = env.window.axonhub.onInit((init) => {
    late = init;
  });
  assert.equal(late, null);
  await Promise.resolve();
  assert.equal(late.page, "main");
  assert.equal(late.config.threshold, 1);
  const seen = late;
  removeLate();
  deliver(env, initMessage("sess-2"));
  await Promise.resolve();
  assert.equal(late, seen);
});

test("onTheme delivers the current theme once and skips unchanged updates", async () => {
  const env = setup();
  const seen = [];
  await handshake(env);
  const remove = env.window.axonhub.onTheme((theme) => {
    seen.push(`${theme.theme}:${theme.colorScheme}`);
  });
  await Promise.resolve();
  assert.deepEqual(seen, ["dark:zinc"]);

  deliver(env, {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "theme",
    sessionId: "sess-1",
    payload: { theme: "dark", colorScheme: "zinc" },
  });
  deliver(env, {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "theme",
    sessionId: "sess-1",
    payload: { theme: "light", colorScheme: "zinc" },
  });
  assert.deepEqual(seen, ["dark:zinc", "light:zinc"]);
  remove();
  deliver(env, {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "theme",
    sessionId: "sess-1",
    payload: { theme: "dark", colorScheme: "slate" },
  });
  assert.deepEqual(seen, ["dark:zinc", "light:zinc"]);
});

test("theme tokens are written onto the document element and unsafe values are dropped", async () => {
  const env = setup();
  const tokens = {
    "--background": "oklch(0.22 0.002 106)",
    "--foreground": "#f5f5f4",
    "--font-sans": "Inter, ui-sans-serif, system-ui, sans-serif",
    "--radius": "0.75rem",
    "--shadow-sm": "0 20px 40px -15px rgba(176, 93, 46, 0.1)",
    "--font-mono": "JetBrains Mono, ui-monospace, SFMono-Regular, monospace",
    "--destructive-foreground": "#ffffff",
    "--sidebar": "#000000",
    "--background-evil": "red",
  };
  const pending = env.window.axonhub.ready();
  deliver(env, {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "init",
    sessionId: "sess-1",
    payload: { ...initPayload, tokens: { ...tokens, "--card": "red; color: url(https://evil.example)" } },
  });
  const init = await pending;
  assert.equal(init.tokens["--background"], tokens["--background"]);
  assert.equal(init.tokens["--card"], undefined);
  assert.equal(init.tokens["--sidebar"], undefined);
  assert.equal(env.style.values.get("--background"), tokens["--background"]);
  assert.equal(env.style.values.get("--font-sans"), tokens["--font-sans"]);
  assert.equal(env.style.values.get("--font-mono"), tokens["--font-mono"]);
  assert.equal(env.style.values.get("--shadow-sm"), tokens["--shadow-sm"]);
  assert.equal(env.style.values.get("--destructive-foreground"), "#ffffff");
  assert.equal(env.style.values.has("--card"), false);
  assert.equal(env.style.values.has("--sidebar"), false);
  assert.equal(env.style.values.get("color-scheme"), "dark");

  const seen = [];
  env.window.axonhub.onTheme((theme) => {
    seen.push(theme.tokens["--background"]);
  });
  await Promise.resolve();
  assert.deepEqual(seen, [tokens["--background"]]);

  deliver(env, {
    protocol: "axonhub.plugin-ui",
    version: 1,
    type: "theme",
    sessionId: "sess-1",
    payload: {
      theme: "light",
      colorScheme: "zinc",
      tokens: { "--background": "#f4f5f7", "--foreground": "\nred" },
    },
  });
  assert.equal(env.style.values.get("--background"), "#f4f5f7");
  assert.equal(env.style.values.has("--foreground"), false);
  assert.equal(env.style.values.has("--destructive-foreground"), false);
  assert.equal(env.style.values.get("color-scheme"), "light");
  assert.deepEqual(seen, [tokens["--background"], "#f4f5f7"]);
});

test("concurrent calls match replies by id, including a finished id", async () => {
  const env = setup();
  await handshake(env);
  const first = env.window.axonhub.graphql("query { a }");
  const second = env.window.axonhub.graphql("query { b }");
  const sent = calls(env);
  assert.equal(sent.length, 2);
  assert.notEqual(sent[0].data.id, sent[1].data.id);
  assert.equal(sent[0].origin, origin);

  deliver(env, reply(sent[1].data.id, { ok: true, result: { data: { b: 2 } } }));
  deliver(env, reply(sent[0].data.id, { ok: true, result: { data: { a: 1 } } }));
  assert.deepEqual(json(await first), { data: { a: 1 } });
  assert.deepEqual(json(await second), { data: { b: 2 } });

  let extra = 0;
  first.then(() => {
    extra += 1;
  });
  deliver(env, reply(sent[0].data.id, { ok: true, result: { data: { a: 9 } } }));
  await Promise.resolve();
  assert.equal(extra, 1);
});

test("wrong source, wrong origin, and wrong session are ignored", async () => {
  const env = setup();
  let settled = false;
  const pending = env.window.axonhub.ready();
  pending.then(() => {
    settled = true;
  }, () => {
    settled = true;
  });
  deliver(env, initMessage(), { postMessage() {} }, origin);
  deliver(env, initMessage(), env.window.parent, "https://evil.test");
  deliver(env, {
    protocol: "axonhub.other",
    version: 1,
    type: "init",
    sessionId: "sess-1",
    payload: initPayload,
  });
  deliver(env, { ...initMessage(), version: 2 });
  await Promise.resolve();
  assert.equal(settled, false);

  deliver(env, initMessage());
  assert.equal((await pending).page, "main");

  const call = env.window.axonhub.graphql("query { a }");
  const id = calls(env).at(-1).data.id;
  let callSettled = false;
  call.then(() => {
    callSettled = true;
  }, () => {
    callSettled = true;
  });
  deliver(env, reply(id, { ok: true, result: { data: { a: 1 } } }, "old-session"));
  await Promise.resolve();
  assert.equal(callSettled, false);
  deliver(env, reply(id, { ok: true, result: { data: { a: 4 } } }));
  assert.deepEqual(json(await call), { data: { a: 4 } });
});

test("ready and call timeouts reject TIMEOUT and ignore a late reply", async () => {
  const readyEnv = setup();
  const ready = readyEnv.window.axonhub.ready();
  const readyRejected = assert.rejects(ready, (error) => {
    assert.equal(error.name, "AxonhubBridgeError");
    assert.equal(error.code, "TIMEOUT");
    return true;
  });
  readyEnv.advance(10000);
  await readyRejected;
  assert.equal(readyEnv.sent.filter((message) => message.data.type === "hello").length, 1);
  readyEnv.window.axonhub.ready();
  assert.equal(readyEnv.sent.filter((message) => message.data.type === "hello").length, 1);

  const env = setup();
  await handshake(env);
  const call = env.window.axonhub.graphql("query { a }");
  const id = calls(env).at(-1).data.id;
  const rejected = assert.rejects(call, (error) => {
    assert.equal(error.code, "TIMEOUT");
    assert.equal(error.status, undefined);
    return true;
  });
  env.advance(14999);
  env.advance(15000);
  await rejected;
  deliver(env, reply(id, { ok: true, result: { data: { a: 1 } } }));
  await assert.rejects(call, (error) => error.code === "TIMEOUT");
});

test("dispose and pagehide reject pending calls with DISPOSED", async () => {
  const env = setup();
  await handshake(env);
  const pending = env.window.axonhub.graphql("query { a }");
  const id = calls(env).at(-1).data.id;
  env.window.__axonhubDispose();
  await assert.rejects(pending, (error) => {
    assert.equal(error.code, "DISPOSED");
    return true;
  });
  deliver(env, reply(id, { ok: true, result: { data: { a: 1 } } }));
  await assert.rejects(env.window.axonhub.graphql("query { b }"), (error) => error.code === "DISPOSED");
  assert.equal((await env.window.axonhub.ready()).page, "main");

  const hidden = setup();
  const waiting = hidden.window.axonhub.graphql("query { a }");
  hidden.window.dispatchEvent({ type: "pagehide" });
  await assert.rejects(waiting, (error) => error.code === "DISPOSED");
});

test("graphql keeps partial data and errors and does not retry", async () => {
  const env = setup();
  await handshake(env);
  const envelope = {
    data: { pluginKV: { value: "1" } },
    errors: [{ message: "partial", path: ["pluginKV"] }],
  };
  const pending = env.window.axonhub.graphql(
    "query Read($key: String!) { pluginKV(key: $key) { value } }",
    { key: "settings" },
  );
  assert.equal(calls(env).length, 1);
  deliver(env, reply(calls(env)[0].data.id, { ok: true, result: envelope }));
  assert.deepEqual(json(await pending), envelope);
  assert.equal(calls(env).length, 1);

  const cleared = env.window.axonhub.navigate("/plugins/acme");
  deliver(env, reply(calls(env).at(-1).data.id, { ok: true, result: null }));
  assert.equal(await cleared, undefined);
});

test("host reply status and code map without exposing server internals", async () => {
  const cases = [
    [{ code: "INTERNAL", message: "password=secret", status: 401 }, "UNAUTHENTICATED", 401],
    [{ code: "INTERNAL", message: "sql: boom", status: 403 }, "FORBIDDEN", 403],
    [{ code: "INTERNAL", message: "sql: missing", status: 404 }, "NOT_FOUND", 404],
    [{ code: "INTERNAL", message: "redis timeout detail", status: 429 }, "RATE_LIMITED", 429],
    [{ code: "TIMEOUT", message: "context deadline exceeded: sql", status: 504 }, "TIMEOUT", 504],
    [{ code: "INTERNAL", message: "stack trace /tmp/server.go:9", status: 500 }, "INTERNAL", 500],
    [{ code: "NETWORK_ERROR", message: "dial tcp secret-host:5432" }, "NETWORK_ERROR", undefined],
  ];
  for (const [hostError, code, status] of cases) {
    const env = setup();
    await handshake(env);
    const pending = env.window.axonhub.graphql("query { a }");
    deliver(env, reply(calls(env).at(-1).data.id, { ok: false, error: hostError }));
    const error = await pending.then(() => {
      throw new Error("expected rejection");
    }, (caught) => caught);
    assert.equal(error.name, "AxonhubBridgeError");
    assert.equal(error.code, code);
    assert.equal(error.status, status);
    assert.equal(error.message.includes("password"), false);
    assert.equal(error.message.includes("sql"), false);
    assert.equal(error.message.includes("secret"), false);
    assert.equal(error.message.includes("stack"), false);
    assert.equal(calls(env).length, 1);
  }
});

test("navigate and toast validate arguments before sending", async () => {
  const env = setup();
  await handshake(env);
  const before = calls(env).length;
  await assert.rejects(env.window.axonhub.navigate("//evil.test"), (error) => error.code === "INVALID_ARGUMENT");
  await assert.rejects(env.window.axonhub.navigate("/a\\b"), (error) => error.code === "INVALID_ARGUMENT");
  await assert.rejects(env.window.axonhub.navigate("/a\u0001b"), (error) => error.code === "INVALID_ARGUMENT");
  await assert.rejects(env.window.axonhub.navigate("plugins"), (error) => error.code === "INVALID_ARGUMENT");
  await assert.rejects(env.window.axonhub.toast("  "), (error) => error.code === "INVALID_ARGUMENT");
  assert.equal(calls(env).length, before);

  const pending = env.window.axonhub.toast("Saved");
  const sent = calls(env).at(-1).data;
  assert.equal(sent.method, "toast");
  assert.deepEqual(json(sent.params), { message: "Saved", kind: "info" });
  deliver(env, reply(sent.id, { ok: true, result: null }));
  assert.equal(await pending, undefined);
});

test("page errors are reported for the current session without a token", async () => {
  const env = setup();
  await handshake(env);
  env.window.dispatchEvent({ type: "error", message: "boom at ui/overview/index.html" });
  env.window.dispatchEvent({ type: "unhandledrejection", reason: new Error("rejected render") });
  const reports = env.sent.filter((message) => message.data.type === "pageError");
  assert.equal(reports.length, 2);
  assert.equal(reports[0].data.sessionId, "sess-1");
  assert.equal(reports[0].data.payload.message, "boom at ui/overview/index.html");
  assert.equal(reports[1].data.payload.message, "rejected render");
  assert.equal(JSON.stringify(reports).includes("token-value"), false);
  assert.equal(reports[0].origin, origin);
});

 test("operation names in variables, arguments, strings and fragments are not extra operations", async () => {
  const env = setup();
  await handshake(env);
  const documents = [
    'query Search($query: String!) { search(query: $query) { mutation } }',
    'fragment F on Query { query } query Search { ...F }',
    'query Search($filter: Filter = {query: "# mutation }"}) { search(filter: $filter) }',
    '{ search(query: "subscription") }',
  ];
  for (const query of documents) {
    const result = env.window.axonhub.graphql(query);
    const id = calls(env).at(-1).data.id;
    deliver(env, reply(id, { ok: true, result: { data: {} } }));
    await result;
  }
  await assert.rejects(env.window.axonhub.graphql('fragment F on Query { a } query A { ...F } query B { b }'),
    (error) => error.code === "INVALID_ARGUMENT");
 });
