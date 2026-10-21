#!/usr/bin/env node
// Black-box E2E for the client-cancellation cooldown.
//
// Boots a throwaway AxonHub (own sqlite file + port), two mock OpenAI upstreams,
// then drives real HTTP traffic: initialize -> login -> channels -> API key ->
// streaming request aborted mid-flight (client cancellation) -> assert cooldown.
//
// Usage: node scripts/e2e/client-cancellation.e2e.mjs
// Env:   E2E_BIN (default ./axonhub.exe), E2E_APP_PORT (default 8091)

import http from 'node:http';
import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const APP_PORT = Number(process.env.E2E_APP_PORT || 8091);
const BASE = 'http://127.0.0.1:' + APP_PORT;
const BIN = process.env.E2E_BIN || join(process.cwd(), 'axonhub.exe');
const OWNER = { email: 'e2e-owner@example.com', password: 'e2e-password-123' };

const results = [];
function check(name, ok, detail) {
  results.push({ name, ok });
  console.log((ok ? 'PASS' : 'FAIL') + '  ' + name + (detail ? '  -> ' + detail : ''));
  if (!ok) {
    process.exitCode = 1;
  }
}

const sleep = function (ms) {
  return new Promise(function (r) { setTimeout(r, ms); });
};

// ---------------------------------------------------------------- mock upstream

function startMockUpstream(port) {
  const stats = { requests: 0 };
  const server = http.createServer(function (req, res) {
    let body = '';
    req.on('data', function (c) { body += c; });
    req.on('end', function () {
      stats.requests += 1;
      let payload = {};
      try {
        payload = JSON.parse(body || '{}');
      } catch {
        payload = {};
      }
      const wantsStream = payload.stream === true;
      const slow = payload.user === 'slow';

      if (!wantsStream) {
        const done = {
          id: 'chatcmpl-e2e',
          object: 'chat.completion',
          created: Math.floor(Date.now() / 1000),
          model: payload.model || 'gpt-4',
          choices: [{ index: 0, message: { role: 'assistant', content: 'ok' }, finish_reason: 'stop' }],
          usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 },
        };
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(done));
        return;
      }

      res.writeHead(200, {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
        Connection: 'keep-alive',
      });

      let sent = 0;
      let timer = null;
      const stop = function () {
        if (timer) {
          clearInterval(timer);
          timer = null;
        }
      };
      const emit = function () {
        if (res.writableEnded || res.destroyed) {
          stop();
          return;
        }
        sent += 1;
        const chunk = {
          id: 'chatcmpl-e2e',
          object: 'chat.completion.chunk',
          created: Math.floor(Date.now() / 1000),
          model: payload.model || 'gpt-4',
          choices: [{ index: 0, delta: { content: 'tok' + sent + ' ' }, finish_reason: null }],
        };
        res.write('data: ' + JSON.stringify(chunk) + '\n\n');
        if (!slow && sent >= 2) {
          res.write('data: [DONE]\n\n');
          res.end();
          stop();
        }
      };
      timer = setInterval(emit, slow ? 200 : 60);
      req.on('close', stop);
      res.on('close', stop);
      emit();
    });
  });

  return new Promise(function (resolve) {
    server.listen(port, '127.0.0.1', function () {
      resolve({ server: server, stats: stats, port: port });
    });
  });
}

// ------------------------------------------------------------------- app boot

async function waitForHealth(timeoutMs) {
  const deadline = Date.now() + (timeoutMs || 60000);
  while (Date.now() < deadline) {
    try {
      const res = await fetch(BASE + '/health');
      if (res.ok) {
        return true;
      }
    } catch {
      // not up yet
    }
    await sleep(250);
  }
  throw new Error('axonhub did not become healthy');
}

const appLog = [];

function startApp(dbPath) {
  const child = spawn(BIN, [], {
    cwd: process.cwd(),
    env: Object.assign({}, process.env, {
      AXONHUB_SERVER_HOST: '127.0.0.1',
      AXONHUB_SERVER_PORT: String(APP_PORT),
      AXONHUB_DB_DIALECT: 'sqlite3',
      AXONHUB_DB_DSN: 'file:' + dbPath + '?_fk=1&_pragma=journal_mode(WAL)',
      AXONHUB_LOG_LEVEL: 'warn',
    }),
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.on('data', function (d) {
    for (const line of String(d).split(/\r?\n/)) {
      if (line.trim()) {
        appLog.push(line);
      }
    }
  });
  child.stderr.on('data', function (d) { process.stderr.write('[app] ' + d); });
  return child;
}

// ------------------------------------------------------------------ helpers

let token = '';

async function gql(query, variables) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) {
    headers.Authorization = 'Bearer ' + token;
  }
  const res = await fetch(BASE + '/admin/graphql', {
    method: 'POST',
    headers: headers,
    body: JSON.stringify({ query: query, variables: variables || {} }),
  });
  const json = await res.json();
  if (json.errors && json.errors.length) {
    throw new Error('graphql error: ' + JSON.stringify(json.errors));
  }
  return json.data;
}

async function postJSON(path, body) {
  const res = await fetch(BASE + path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  let json = text;
  try {
    json = JSON.parse(text);
  } catch {
    json = text;
  }
  return { status: res.status, body: json };
}

const CHANNEL_FIELDS =
  'id name status errorMessage autoDisabledAt autoDisableExpiresAt disabledAPIKeys { key expiresAt reason }';

async function listChannels() {
  const data = await gql('query { queryChannels(input: {}) { edges { node { ' + CHANNEL_FIELDS + ' } } } }');
  return data.queryChannels.edges.map(function (e) { return e.node; });
}

async function findChannel(name) {
  const channels = await listChannels();
  for (const c of channels) {
    if (c.name === name) {
      return c;
    }
  }
  return undefined;
}

async function waitForChannel(name, predicate, timeoutMs) {
  const deadline = Date.now() + (timeoutMs || 10000);
  let last;
  while (Date.now() < deadline) {
    last = await findChannel(name);
    if (last && predicate(last)) {
      return last;
    }
    await sleep(150);
  }
  return last;
}

async function setChannelStatus(id, status) {
  await gql(
    'mutation ($id: ID!, $status: ChannelStatus!) { updateChannelStatus(id: $id, status: $status) { id status } }',
    { id: id, status: status }
  );
}

async function ensureModel(modelID) {
  const data = await gql(
    'mutation ($input: CreateModelInput!) { createModel(input: $input) { id modelID } }',
    {
      input: {
        developer: 'openai',
        modelID: modelID,
        type: 'chat',
        name: modelID,
        icon: 'OpenAI',
        group: 'openai',
        modelCard: {},
        settings: { associations: [] },
      },
    }
  );
  return data.createModel;
}

async function setCancellationPolicy(policy) {
  const data = await gql('mutation ($input: UpdateRetryPolicyInput!) { updateRetryPolicy(input: $input) }', {
    input: {
      enabled: true,
      maxChannelRetries: 1,
      maxSingleChannelRetries: 0,
      retryDelayMs: 0,
      clientCancellationPolicy: policy,
    },
  });
  if (data.updateRetryPolicy !== true) {
    throw new Error('updateRetryPolicy returned false');
  }
}

async function readCancellationPolicy() {
  const data = await gql(
    'query { retryPolicy { clientCancellationPolicy { enabled threshold cooldownMinutes action scope projectOverrides { projectId enabled threshold cooldownMinutes action } } } }'
  );
  return data.retryPolicy.clientCancellationPolicy;
}

function dumpAppLog(match) {
  const seen = appLog.filter(function (line) { return match.test(line); });
  for (const line of seen.slice(-12)) {
    console.log('  [app] ' + line.slice(0, 400));
  }
}

async function createChannel(name, baseURL, apiKeys, enable) {
  const data = await gql(
    'mutation ($input: CreateChannelInput!) { createChannel(input: $input) { id name status } }',
    {
      input: {
        type: 'openai',
        name: name,
        baseURL: baseURL,
        credentials: { apiKeys: apiKeys },
        supportedModels: ['gpt-4'],
        defaultTestModel: 'gpt-4',
      },
    }
  );
  // Channels are created disabled by the API; enable them unless the phase
  // wants the channel to stay out of the candidate pool.
  if (enable !== false) {
    await setChannelStatus(data.createChannel.id, 'enabled');
  }
  return data.createChannel;
}

// Fires a streaming completion and aborts it mid-flight: this is what a user
// pressing "stop" does to the connection.
async function canceledStreamRequest(apiKey, user, abortAfterMs) {
  const controller = new AbortController();
  const timer = setTimeout(function () { controller.abort(); }, abortAfterMs || 900);
  try {
    const res = await fetch(BASE + '/v1/chat/completions', {
      method: 'POST',
      signal: controller.signal,
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + apiKey },
      body: JSON.stringify({
        model: 'gpt-4',
        stream: true,
        user: user || 'slow',
        messages: [{ role: 'user', content: 'hello' }],
      }),
    });
    if (!res.ok) {
      return { ok: false, aborted: false, status: res.status, body: await res.text() };
    }
    const reader = res.body.getReader();
    await reader.read();
    controller.abort();
    try {
      await reader.cancel();
    } catch {
      // already closed
    }
    return { ok: true, aborted: true };
  } catch (err) {
    const name = err && err.name ? err.name : '';
    return { ok: false, aborted: name === 'AbortError' || String(err).indexOf('abort') >= 0, error: String(err) };
  } finally {
    clearTimeout(timer);
  }
}

// A plain non-streaming completion: the "successful request" that must reset
// the consecutive cancellation counter.
async function successfulRequest(apiKey) {
  const res = await fetch(BASE + '/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + apiKey },
    body: JSON.stringify({
      model: 'gpt-4',
      stream: false,
      messages: [{ role: 'user', content: 'hello' }],
    }),
  });
  const text = await res.text();
  if (!res.ok) {
    return { ok: false, status: res.status, body: text };
  }
  return { ok: true, body: text };
}

// Channel status changes invalidate the routing cache lazily, so a request can
// briefly report the model as unknown. Retry until routing settles.
async function waitRoutable(apiKey, timeoutMs) {
  const deadline = Date.now() + (timeoutMs || 25000);
  let last = null;
  while (Date.now() < deadline) {
    last = await successfulRequest(apiKey);
    if (last.ok) {
      return last;
    }
    await sleep(400);
  }
  return last;
}

// Channel status changes propagate to the routing cache lazily, so keep sending
// completed requests until the traffic really lands on the intended upstream.
async function waitRoutedTo(apiKey, mock, timeoutMs) {
  const baseline = mock.stats.requests;
  const deadline = Date.now() + (timeoutMs || 30000);
  let last = null;
  while (Date.now() < deadline) {
    last = await successfulRequest(apiKey);
    if (last.ok && mock.stats.requests > baseline) {
      return { ok: true, response: last, delta: mock.stats.requests - baseline };
    }
    await sleep(400);
  }
  return { ok: false, response: last, delta: mock.stats.requests - baseline };
}

// The model catalog is cached, so the first routed request may still report the
// freshly created model as unknown. Retry until routing works.
async function warmUpModel(apiKey) {
  const deadline = Date.now() + 30000;
  let last = null;
  while (Date.now() < deadline) {
    last = await successfulRequest(apiKey);
    if (last.ok) {
      return last;
    }
    await sleep(500);
  }
  throw new Error('model never became routable: ' + JSON.stringify(last));
}

// ---------------------------------------------------------------------- main

async function main() {
  const tmp = await mkdtemp(join(tmpdir(), 'axonhub-e2e-'));
  const dbPath = join(tmp, 'e2e.db');
  const mockA = await startMockUpstream(9101);
  const mockB = await startMockUpstream(9102);
  const mockC = await startMockUpstream(9103);
  const mockD = await startMockUpstream(9104);
  const app = startApp(dbPath);

  try {
    await waitForHealth();

    // 1. initialize + login
    const init = await postJSON('/admin/system/initialize', {
      ownerEmail: OWNER.email,
      ownerPassword: OWNER.password,
      ownerFirstName: 'E2E',
      ownerLastName: 'Owner',
      brandName: 'E2E Lab',
      preferLanguage: 'en',
    });
    check('system initialize', init.status === 200, 'status=' + init.status);

    const signIn = await postJSON('/admin/auth/signin', { email: OWNER.email, password: OWNER.password });
    token = signIn.body && signIn.body.token ? signIn.body.token : '';
    check('admin sign in', Boolean(token), 'status=' + signIn.status);

    const me = await gql('query { myProjects { id name } }');
    const projectID = String(me.myProjects[0].id);

    // A fresh database has no model catalog, so register the model under test.
    await ensureModel('gpt-4');

    // 2. channels: A is the one we cancel on, B is the fallback that must stay untouched
    const chA = await createChannel('e2e-channel-a', 'http://127.0.0.1:9101/v1', ['sk-e2e-a']);
    // Channel B supports the same model but stays disabled for now, so every
    // request in the cancellation phase must land on channel A.
    const chB = await createChannel('e2e-channel-b', 'http://127.0.0.1:9102/v1', ['sk-e2e-b'], false);
    check('channels created', Boolean(chA.id && chB.id), 'a=' + chA.status + ' b=' + chB.status);

    // 3. policy: 2 consecutive cancellations -> switch channel + 2 minute cooldown
    await setCancellationPolicy({
      enabled: true,
      threshold: 2,
      cooldownMinutes: 2,
      action: 'switch_and_cooldown',
      scope: 'global',
      projectOverrides: [],
    });
    const policy = await readCancellationPolicy();
    check(
      'policy persisted through graphql',
      policy.enabled === true && policy.threshold === 2 && policy.cooldownMinutes === 2 && policy.action === 'switch_and_cooldown',
      JSON.stringify(policy)
    );

    // 4. API key for real /v1 traffic
    const keyData = await gql('mutation ($input: CreateAPIKeyInput!) { createAPIKey(input: $input) { id key } }', {
      input: { name: 'e2e-key', type: 'user', projectID: projectID },
    });
    const apiKey = keyData.createAPIKey.key;
    check('api key created', Boolean(apiKey));

    // 5. warm the model catalog up with a completed request
    const warm = await warmUpModel(apiKey);
    check('a completed request routed to channel A', warm.ok === true, JSON.stringify(warm).slice(0, 160));
    const mockABaseline = mockA.stats.requests;
    const mockBBaseline = mockB.stats.requests;

    // 6. first cancellation: counter only, channel keeps serving
    const c1 = await canceledStreamRequest(apiKey);
    check('first streaming request was aborted by the client', c1.aborted === true, JSON.stringify(c1));
    await sleep(800);
    const afterFirst = await findChannel('e2e-channel-a');
    check('after 1 cancellation channel A is still enabled', afterFirst.status === 'enabled', afterFirst.status);

    // 7. second cancellation: threshold reached -> cooldown
    const c2 = await canceledStreamRequest(apiKey);
    check('second streaming request was aborted by the client', c2.aborted === true, JSON.stringify(c2));

    const cooled = await waitForChannel('e2e-channel-a', function (c) { return c.status === 'disabled'; });
    const expiresAt = cooled.autoDisableExpiresAt ? new Date(cooled.autoDisableExpiresAt).getTime() : 0;
    const remainingMs = expiresAt - Date.now();
    check('channel A entered the cooldown', cooled.status === 'disabled', cooled.status);
    check('cooldown expiry is about 2 minutes ahead', remainingMs > 60000 && remainingMs <= 125000, 'remainingMs=' + remainingMs);
    check(
      'cooldown reason is the cancellation one',
      String(cooled.errorMessage || '').startsWith('Client cancellation cooldown'),
      cooled.errorMessage
    );
    check('channel A is flagged as auto disabled', Boolean(cooled.autoDisabledAt), String(cooled.autoDisabledAt));
    check(
      'channel A saw exactly the two canceled attempts (no extra failover attempt)',
      mockA.stats.requests === mockABaseline + 2,
      'mockA=' + mockA.stats.requests + ' baseline=' + mockABaseline
    );
    check(
      'the cancelled request was not retried on channel B',
      mockB.stats.requests === mockBBaseline,
      'mockB=' + mockB.stats.requests
    );

    // 8. traffic moves to the next channel while A cools down
    await setChannelStatus(chB.id, 'enabled');
    const switched = await waitRoutedTo(apiKey, mockB);
    check('request during the cooldown succeeded on channel B', switched.ok === true, 'delta=' + switched.delta);
    check('channel A received no traffic during the cooldown', mockA.stats.requests === mockABaseline + 2, 'mockA=' + mockA.stats.requests);

    // 9. a successful request resets the streak
    await setChannelStatus(chB.id, 'disabled');
    await setChannelStatus(cooled.id, 'enabled');
    const routedA = await waitRoutedTo(apiKey, mockA);
    check('traffic returns to channel A after it is re-enabled', routedA.ok === true && routedA.delta > 0, 'delta=' + routedA.delta);
    const cancelAgain = await canceledStreamRequest(apiKey);
    check('cancellation after re-enable was aborted', cancelAgain.aborted === true);
    await sleep(500);
    const success = await successfulRequest(apiKey);
    check('successful request completed', success.ok === true, success.ok ? 'ok' : JSON.stringify(success).slice(0, 160));
    await sleep(800);
    const cancelAfterSuccess = await canceledStreamRequest(apiKey);
    check('cancellation after the success was aborted', cancelAfterSuccess.aborted === true);
    await sleep(800);
    const afterSuccess = await findChannel('e2e-channel-a');
    check(
      'a successful request cleared the counter (channel A is still enabled)',
      afterSuccess.status === 'enabled',
      afterSuccess.status
    );

    // 10. cooldown_only cools the credential, not the channel
    await setChannelStatus(cooled.id, 'disabled');
    const chC = await createChannel('e2e-channel-c', 'http://127.0.0.1:9103/v1', ['sk-e2e-c1', 'sk-e2e-c2']);
    await setCancellationPolicy({
      enabled: true,
      threshold: 1,
      cooldownMinutes: 1,
      action: 'cooldown_only',
      scope: 'global',
      projectOverrides: [],
    });
    const routedC = await waitRoutedTo(apiKey, mockC);
    check('requests route to channel C', routedC.ok === true && routedC.delta > 0, 'delta=' + routedC.delta);
    const policyC = await readCancellationPolicy();
    console.log('  policy for cooldown_only phase: ' + JSON.stringify(policyC));
    const c3 = await canceledStreamRequest(apiKey);
    check('cooldown_only request was aborted', c3.aborted === true, JSON.stringify(c3));
    const channelC = await waitForChannel('e2e-channel-c', function (c) {
      return (c.disabledAPIKeys || []).length > 0;
    });
    check('channel C stays enabled', channelC.status === 'enabled', channelC.status);
    check('only one credential of channel C is cooling down', (channelC.disabledAPIKeys || []).length === 1, JSON.stringify(channelC.disabledAPIKeys));
    check(
      'the cooled credential carries an expiry',
      Boolean(channelC.disabledAPIKeys && channelC.disabledAPIKeys[0] && channelC.disabledAPIKeys[0].expiresAt),
      String(channelC.disabledAPIKeys && channelC.disabledAPIKeys[0] && channelC.disabledAPIKeys[0].expiresAt)
    );

    // 11. project scoped override (project-first)
    await setChannelStatus(chC.id, 'disabled');
    await createChannel('e2e-channel-d', 'http://127.0.0.1:9104/v1', ['sk-e2e-d']);
    const numericProjectID = Number(String(projectID).split('/').pop());
    check('project id decoded for the override', numericProjectID > 0, 'projectID=' + projectID + ' numeric=' + numericProjectID);
    await setCancellationPolicy({
      enabled: true,
      threshold: 5,
      cooldownMinutes: 30,
      action: 'switch_and_cooldown',
      scope: 'project_first',
      projectOverrides: [{ projectId: numericProjectID, enabled: true, threshold: 1, cooldownMinutes: 1 }],
    });
    const routedD = await waitRoutedTo(apiKey, mockD);
    check('requests route to channel D', routedD.ok === true && routedD.delta > 0, 'delta=' + routedD.delta);
    const policyD = await readCancellationPolicy();
    console.log('  policy for project override phase: ' + JSON.stringify(policyD));
    const c4 = await canceledStreamRequest(apiKey);
    check('project-override request was aborted', c4.aborted === true, JSON.stringify(c4));
    const channelD = await waitForChannel('e2e-channel-d', function (c) { return c.status === 'disabled'; });
    check(
      'project override threshold (1) cooled the channel down',
      channelD.status === 'disabled',
      'status=' + channelD.status + ' error=' + channelD.errorMessage
    );
  } finally {
    app.kill();
    await sleep(500);
    mockA.server.close();
    mockB.server.close();
    mockC.server.close();
    mockD.server.close();
    await rm(tmp, { recursive: true, force: true }).catch(function () {});
  }

  console.log('\nrelevant app log:');
  dumpAppLog(/cooldown|auto-disabled|cancellation/i);

  const failed = results.filter(function (r) { return !r.ok; });
  console.log('\n' + (results.length - failed.length) + '/' + results.length + ' checks passed');
  if (failed.length) {
    console.log('failed: ' + failed.map(function (f) { return f.name; }).join(' | '));
    process.exitCode = 1;
  }
}

main().catch(function (err) {
  console.error('E2E crashed:', err);
  process.exitCode = 1;
});
