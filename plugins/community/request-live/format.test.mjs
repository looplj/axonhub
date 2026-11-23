import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  CLIENTS,
  LINGER_MS,
  REQUEST_HUES,
  SPARE_HUES,
  buildStage,
  channelStatusText,
  formatTime,
  clientLabel,
  identifyClient,
  laneOffsetFor,
  requestHue,
  rowMotion,
  statusLine,
  toggleSelection,
  visibleExecutionIds,
} from "./ui/live/format.js";
import { iconSvg } from "./ui/live/icons.js";
import {
  DEFAULT_LIMIT,
  DEFAULT_REFRESH,
  DEFAULT_WINDOW,
  INFLIGHT_BUDGET,
  INFLIGHT_LIMIT,
  REFRESH_SECONDS,
  STATS_MIN_MS,
  STALE_TTL_MS,
  TRACK_CAP,
  WINDOW_OPTIONS,
  attachExecutions,
  backfillDecision,
  channelRetryPlan,
  channelRetrySucceeded,
  clampLimit,
  clampRefresh,
  clampWindow,
  commitIfCurrent,
  executionIds,
  executionsQuery,
  hasForbidden,
  historyQuery,
  inflightQuery,
  isForbiddenError,
  mapRequest,
  mergeRequests,
  nextInflightCursor,
  pageInfoOf,
  readStateNodes,
  readTotals,
  requestStateQuery,
  retiredLiveIds,
  settle,
  shouldRetryWithoutChannel,
  sinceFor,
  statsAccept,
  statsDue,
  statsQuery,
  statsTotalsForWindow,
  statsVisible,
  statsWindowMatches,
  transportErrors,
  windowMsFor,
} from "./ui/live/query.js";

function isNearPureRed(hex) {
  const raw = String(hex).trim().toLowerCase();
  if (raw === "#f00" || raw === "#ff0000") return true;
  const body = raw.replace(/^#/, "");
  const full = body.length === 3 ? body.split("").map((char) => char + char).join("") : body;
  if (full === "ff0000") return true;
  const r = Number.parseInt(full.slice(0, 2), 16);
  const g = Number.parseInt(full.slice(2, 4), 16);
  const b = Number.parseInt(full.slice(4, 6), 16);
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const delta = max - min;
  if (!delta) return false;
  let hue;
  if (max === r) hue = ((g - b) / delta) % 6;
  else if (max === g) hue = (b - r) / delta + 2;
  else hue = (r - g) / delta + 4;
  hue *= 60;
  if (hue < 0) hue += 360;
  const light = (max + min) / 2;
  const sat = delta / (255 - Math.abs(2 * light - 255));
  return (hue <= 8 || hue >= 352) && sat > 0.85;
}

function stage(requests, extra) {
  return buildStage({
    requests,
    now: extra.now,
    window: extra.window,
    selectedId: extra.selectedId ?? extra.focusId ?? null,
    clientFilter: extra.clientFilter ?? "all",
    limit: extra.limit,
    totals: extra.totals,
    historyCount: extra.historyCount,
    lingering: extra.lingering,
    stale: extra.stale,
    inFlightTotal: extra.inFlightTotal,
    inFlightMore: extra.inFlightMore,
  });
}

function request(partial) {
  return {
    status: "completed",
    modelID: "m",
    stream: false,
    source: "api",
    userAgent: "cursor/1",
    apiKey: { id: "k", name: "key", status: "enabled", type: "user" },
    channel: { id: "c", name: "ch" },
    ...partial,
  };
}

test("identifyClient matches known agents, playground, and unknown products", () => {
  const claude = identifyClient("claude-cli/1.2.3", "api");
  assert.deepEqual(claude, { id: "claude", name: "Claude Code", hue: "#d97757" });
  assert.equal(identifyClient("claude-cli-extra/9", "api").id, "claude");

  const cursor = identifyClient("cursor/0.50", "api");
  assert.equal(cursor.id, "cursor");
  assert.equal(cursor.name, "Cursor");
  assert.equal(cursor.hue, "#14b8a6");

  const playground = identifyClient(
    "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
    "playground",
  );
  assert.deepEqual(playground, { id: "playground", name: "Playground", hue: "#8b5cf6" });

  const unknown = identifyClient("", "api");
  assert.deepEqual(unknown, { id: "unknown", name: "未知客户端", hue: "#64748b" });
  assert.equal(identifyClient("   ", "api").id, "unknown");
  assert.equal(identifyClient("", "test").id, "test");

  const python = identifyClient("Python-urllib/3.11", "api");
  const again = identifyClient("Python-urllib/3.11", "api");
  assert.equal(python.id, "python-urllib");
  assert.equal(python.name, "Python-urllib");
  assert.ok(SPARE_HUES.includes(python.hue));
  assert.equal(python.hue, again.hue);
  assert.equal(isNearPureRed(python.hue), false);
});

test("unknown client name follows the locale and a suffix is not a failover", () => {
  const unknown = identifyClient("", "api");
  assert.equal(clientLabel(unknown, "en"), "Unknown client");
  assert.equal(clientLabel(unknown, "zh-CN"), "未知客户端");
  assert.equal(clientLabel(identifyClient("cursor/1", "api"), "zh-CN"), "Cursor");
  assert.equal(rowMotion("ch1:", "ch1:abcd"), "refine");
  assert.equal(rowMotion("", "ch1:abcd"), "assign");
  assert.equal(rowMotion("ch1:abcd", "ch2:abcd"), "switch");
  assert.equal(rowMotion("ch1:aaaa", "ch1:bbbb"), "switch");
  assert.equal(rowMotion("ch1:abcd", "ch1:abcd"), "same");
});

test("known and spare hues are not red", () => {
  for (const client of CLIENTS) assert.equal(isNearPureRed(client.hue), false, client.id);
  for (const hue of SPARE_HUES) assert.equal(isNearPureRed(hue), false, hue);
});

test("live window keeps old in-flight work and drops stale completed requests", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const view = stage([
    request({ id: "processing-old", createdAt: at(120), status: "processing" }),
    request({ id: "completed-stale", createdAt: at(16), status: "completed" }),
    request({ id: "completed-fresh", createdAt: at(10), status: "completed" }),
  ], { now, window: "live" });
  const ids = view.log.map((row) => row.id);
  assert.ok(ids.includes("processing-old"));
  assert.ok(ids.includes("completed-fresh"));
  assert.equal(ids.includes("completed-stale"), false);
});

test("today window uses the local day boundary", () => {
  const now = new Date(2026, 10, 8, 0, 10, 0);
  const start = new Date(2026, 10, 8, 0, 0, 0);
  const view = stage([
    request({ id: "yesterday-done", createdAt: new Date(2026, 10, 7, 23, 50, 0).toISOString(), status: "completed" }),
    request({ id: "yesterday-live", createdAt: new Date(2026, 10, 7, 22, 0, 0).toISOString(), status: "processing" }),
    request({ id: "before-midnight", createdAt: new Date(start.getTime() - 1).toISOString(), status: "completed" }),
    request({ id: "at-midnight", createdAt: start.toISOString(), status: "completed" }),
    request({ id: "today-done", createdAt: new Date(2026, 10, 8, 0, 1, 0).toISOString(), status: "completed" }),
  ], { now, window: "today" });
  const ids = view.log.map((row) => row.id);
  assert.deepEqual(ids.slice().sort(), ["at-midnight", "today-done", "yesterday-live"]);
});

test("client caps keep the selected client and report the hidden count", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const specs = [
    ["cursor", "cursor/1", 1, "processing"],
    ["claude", "claude-cli/1", 2, "processing"],
    ["codex", "codex/1", 3, "processing"],
    ["gemini", "geminicli/1", 4, "processing"],
    ["copilot", "copilot/1", 5, "processing"],
    ["opencode", "opencode/1", 6, "processing"],
    ["pi", "pi/1", 7, "processing"],
    ["crush", "crush/1", 8, "processing"],
    ["goose", "goose/1", 9, "completed"],
    ["cline", "cline/1", 10, "completed"],
  ];
  const requests = specs.map(([client, userAgent, minutes, status]) => request({
    id: `c-${client}`,
    userAgent,
    createdAt: at(minutes),
    status,
    channel: { id: `ch-${client}`, name: client },
  }));
  const overview = stage(requests, { now, window: "live" });
  assert.equal(overview.clients.length, 8);
  assert.equal(overview.nodeOverflow.clients, 2);
  assert.equal(overview.focusId, "");

  const selected = stage(requests, { now, window: "live", selectedId: "c-goose" });
  const ids = selected.clients.map((client) => client.id);
  assert.equal(ids.length, 9);
  assert.ok(ids.includes("goose"));
  assert.ok(ids.includes("cursor"));
  assert.equal(ids.includes("cline"), false);
  assert.equal(selected.nodeOverflow.clients, 1);
  assert.equal(selected.focusId, "c-goose");
});

test("channel rows split on suffix and keep the latest execution", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const split = stage([request({
    id: "split",
    createdAt: now.toISOString(),
    executions: [
      {
        id: "e1",
        createdAt: new Date(now.getTime() - 2000).toISOString(),
        status: "failed",
        modelID: "m",
        channel: { id: "c1", name: "Alpha" },
        channelAPIKeySuffix: "aaaa",
        errorMessage: "old",
        responseStatusCode: 500,
      },
      {
        id: "e2",
        createdAt: new Date(now.getTime() - 1000).toISOString(),
        status: "completed",
        modelID: "m",
        metricsLatencyMs: 321,
        channel: { id: "c1", name: "Alpha" },
        channelAPIKeySuffix: "bbbb",
      },
    ],
  })], { now, window: "live" });
  assert.equal(split.channels.length, 2);
  assert.deepEqual(split.channels.map((row) => row.suffix).sort(), ["aaaa", "bbbb"]);

  const merged = stage([request({
    id: "merged",
    createdAt: now.toISOString(),
    metricsLatencyMs: 999,
    executions: [
      {
        id: "e1",
        createdAt: new Date(now.getTime() - 2000).toISOString(),
        status: "failed",
        modelID: "m",
        metricsLatencyMs: 50,
        channel: { id: "c1", name: "Alpha" },
        channelAPIKeySuffix: "aaaa",
        errorMessage: "x".repeat(90),
        responseStatusCode: 500,
      },
      {
        id: "e2",
        createdAt: new Date(now.getTime() - 1000).toISOString(),
        status: "completed",
        modelID: "mapped",
        metricsLatencyMs: 321,
        channel: { id: "c1", name: "Alpha" },
        channelAPIKeySuffix: "aaaa",
      },
    ],
  })], { now, window: "live" });
  assert.equal(merged.channels.length, 1);
  assert.equal(merged.channels[0].kind, "completed");
  assert.equal(merged.channels[0].latencyMs, 321);
  assert.equal(merged.channels[0].errorMessage, "");
});

test("overview selects nothing and every processing lane answers its own request", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const requests = [
    request({
      id: "older",
      createdAt: new Date(now.getTime() - 60 * 1000).toISOString(),
      status: "processing",
      channel: { id: "ch-old", name: "Old" },
    }),
    request({
      id: "newer",
      createdAt: now.toISOString(),
      status: "processing",
      channel: { id: "ch-new", name: "New" },
    }),
  ];
  const overview = stage(requests, { now, window: "live" });
  assert.equal(overview.focusId, "");
  assert.equal(overview.focus, null);
  assert.equal(overview.counts.inFlight, 2);
  assert.equal(overview.tracks.length, 2);
  assert.equal(overview.channels.find((row) => row.channelId === "ch-new").kind, "answering");
  assert.equal(overview.channels.find((row) => row.channelId === "ch-old").kind, "answering");

  // A newer arrival never steals an explicit selection.
  const selected = stage([
    ...requests,
    request({
      id: "newest",
      createdAt: new Date(now.getTime() + 1000).toISOString(),
      status: "processing",
      channel: { id: "ch-newest", name: "Newest" },
    }),
  ], { now, window: "live", selectedId: "older" });
  assert.equal(selected.focusId, "older");
  assert.equal(selected.channels.find((row) => row.channelId === "ch-old").kind, "answering");
  assert.equal(selected.channels.find((row) => row.channelId === "ch-new").kind, "other");
});

test("a channel key shared by two in-flight requests shows the focused one", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const attempt = (id, offset) => ({
    id,
    createdAt: new Date(now.getTime() - offset).toISOString(),
    status: "processing",
    modelID: "m",
    channel: { id: "c1", name: "Alpha" },
    channelAPIKeySuffix: "c946",
  });
  const view = stage([
    request({
      id: "older",
      createdAt: new Date(now.getTime() - 30 * 1000).toISOString(),
      status: "processing",
      executions: [attempt("e-old", 500)],
    }),
    request({
      id: "newer",
      createdAt: now.toISOString(),
      status: "processing",
      executions: [attempt("e-new", 2000)],
    }),
  ], { now, window: "live" });
  assert.equal(view.focusId, "");
  assert.equal(view.tracks.length, 2);
  assert.equal(view.channels.length, 1);
  assert.equal(view.channels[0].kind, "answering");
  assert.equal(view.channels[0].tracks.length, 2);
  assert.deepEqual(view.channels[0].tracks.map((track) => track.index).sort(), [1, 2]);
  assert.equal(new Set(view.channels[0].tracks.map((track) => track.hue)).size, 2);
});

test("failed errors truncate at 80 characters", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const message = "e".repeat(81);
  const view = stage([request({
    id: "bad",
    createdAt: now.toISOString(),
    status: "failed",
    executions: [{
      id: "e",
      createdAt: now.toISOString(),
      status: "failed",
      modelID: "m",
      responseStatusCode: 429,
      errorMessage: message,
      channel: { id: "c9", name: "Nine" },
      channelAPIKeySuffix: "zzzz",
    }],
  })], { now, window: "live" });
  assert.equal(view.channels[0].kind, "failed");
  assert.equal(view.channels[0].errorMessage, `${"e".repeat(80)}…`);
  assert.equal(view.channels[0].responseStatusCode, 429);

  const exact = stage([request({
    id: "exact",
    createdAt: now.toISOString(),
    status: "canceled",
    executions: [{
      id: "e",
      createdAt: now.toISOString(),
      status: "canceled",
      modelID: "m",
      errorMessage: "e".repeat(80),
      channel: { id: "c8", name: "Eight" },
    }],
  })], { now, window: "live" });
  assert.equal(exact.channels[0].kind, "failed");
  assert.equal(exact.channels[0].errorMessage, "e".repeat(80));
});

test("channel status copy omits empty parts", () => {
  assert.equal(channelStatusText("answering", "zh-CN"), "回答中…");
  assert.equal(channelStatusText("other", "en"), "Answering another request…");
  assert.equal(channelStatusText("pending", "zh-Hant"), "等待中");
  assert.equal(channelStatusText("pending", "en-US"), "Waiting");
  assert.equal(channelStatusText("completed", "zh", { latencyMs: 1200 }), "已完成 · 1.2s");
  assert.equal(channelStatusText("failed", "en", { code: 500, message: "boom" }), "Failed · 500 · boom");
  assert.equal(channelStatusText("failed", "en", { code: 500, message: "" }), "Failed · 500");
  assert.equal(channelStatusText("failed", "zh-CN", { message: "boom" }), "失败 · boom");
  assert.equal(channelStatusText("failed", "en", {}), "Failed");
});

test("selection is explicit, survives new requests, and clears when it leaves the window", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const idle = stage([
    request({ id: "kept", createdAt: at(5), status: "completed", modelID: "old-model" }),
    request({ id: "live", createdAt: at(1), status: "processing", modelID: "new-model" }),
  ], { now, window: "live" });
  assert.equal(idle.focusId, "");
  assert.equal(idle.focus, null);
  assert.equal(idle.tracks.length, 1);
  assert.equal(idle.tracks[0].id, "live");

  const selected = stage([
    request({ id: "kept", createdAt: at(5), status: "completed", modelID: "old-model" }),
    request({ id: "live", createdAt: at(1), status: "processing", modelID: "new-model" }),
  ], { now, window: "live", selectedId: "kept" });
  assert.equal(selected.focusId, "kept");
  assert.equal(selected.focus.modelID, "old-model");

  const released = stage([
    request({ id: "gone", createdAt: at(120), status: "completed" }),
    request({ id: "stay", createdAt: at(1), status: "processing" }),
  ], { now, window: "live", selectedId: "gone" });
  assert.equal(released.focusId, "");
});

test("status line names the client and model without a routing reason", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const view = stage([request({
    id: "s1",
    createdAt: new Date(now.getTime() - 1000).toISOString(),
    status: "completed",
    modelID: "gpt-4o",
    stream: true,
    userAgent: "cursor/0.50",
    metricsLatencyMs: 1200,
    channel: { id: "c", name: "west" },
    executions: [
      {
        id: "e1",
        createdAt: new Date(now.getTime() - 900).toISOString(),
        status: "failed",
        modelID: "gpt-4o",
        channel: { id: "c", name: "west" },
        channelAPIKeySuffix: "aa",
      },
      {
        id: "e2",
        createdAt: new Date(now.getTime() - 500).toISOString(),
        status: "completed",
        modelID: "gpt-4o",
        metricsLatencyMs: 800,
        channel: { id: "c", name: "west" },
        channelAPIKeySuffix: "aa",
      },
    ],
  })], { now, window: "live", selectedId: "s1" });
  const line = statusLine(view, "en");
  assert.match(line, /Cursor/);
  assert.match(line, /gpt-4o/);
  assert.match(line, /attempt 2/);
  const chinese = statusLine(view, "zh-CN");
  assert.match(chinese, /第 2 次执行/);
  assert.match(chinese, /gpt-4o/);
  for (const sentence of [line, chinese]) {
    const folded = sentence.toLowerCase();
    for (const word of ["路由", "route", "because", "why"]) {
      assert.equal(folded.includes(word), false, word);
    }
  }
  assert.equal(formatTime(new Date(2026, 0, 2, 3, 4, 5), "zh-CN"), "03:04:05");
});

test("window totals replace the page counts while the list stays the latest page", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const requests = [0, 1, 2].map((index) => request({
    id: `r${index}`,
    createdAt: new Date(now.getTime() - index * 1000).toISOString(),
    status: index === 0 ? "failed" : "completed",
  }));
  const view = stage(requests, {
    now,
    window: "today",
    limit: 50,
    totals: { requests: 80, inFlight: 4, failed: 7 },
  });
  assert.equal(view.shown, 3);
  assert.equal(view.log.length, 3);
  assert.equal(view.counts.requests, 80);
  assert.equal(view.counts.inFlight, 4);
  assert.equal(view.counts.failed, 7);
  assert.deepEqual(view.note, { kind: "list", count: 3 });
});

test("a complete window uses the totals and does not add a sample note", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const view = stage([
    request({ id: "a", createdAt: now.toISOString(), status: "failed" }),
    request({ id: "b", createdAt: now.toISOString(), status: "processing" }),
  ], {
    now,
    window: "live",
    limit: 50,
    totals: { requests: 2, inFlight: 1, failed: 1 },
  });
  assert.equal(view.counts.requests, 2);
  assert.equal(view.counts.inFlight, 1);
  assert.equal(view.counts.failed, 1);
  assert.equal(view.note, null);
});

test("a client filter counts the fetched page when the window is larger", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const view = stage([
    request({ id: "cursor", userAgent: "cursor/1", createdAt: now.toISOString(), status: "failed" }),
    request({ id: "claude", userAgent: "claude-cli/1", createdAt: now.toISOString(), status: "processing" }),
  ], {
    now,
    window: "today",
    limit: 2,
    clientFilter: "cursor",
    totals: { requests: 40, inFlight: 3, failed: 9 },
  });
  assert.equal(view.shown, 1);
  assert.equal(view.counts.requests, 1);
  assert.equal(view.counts.inFlight, 0);
  assert.equal(view.counts.failed, 1);
  assert.deepEqual(view.note, { kind: "sample", count: 2 });
});

test("a client filter over a complete window is that client's real count", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const view = stage([
    request({ id: "cursor", userAgent: "cursor/1", createdAt: now.toISOString() }),
    request({ id: "claude", userAgent: "claude-cli/1", createdAt: now.toISOString() }),
  ], {
    now,
    window: "today",
    limit: 50,
    clientFilter: "cursor",
    totals: { requests: 2, inFlight: 0, failed: 0 },
  });
  assert.equal(view.counts.requests, 1);
  assert.equal(view.note, null);
});

test("without totals a full page is labeled as the latest rows", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const requests = [0, 1].map((index) => request({
    id: `r${index}`,
    createdAt: new Date(now.getTime() - index * 1000).toISOString(),
  }));
  const view = stage(requests, { now, window: "live", limit: 2 });
  assert.equal(view.counts.requests, 2);
  assert.deepEqual(view.note, { kind: "sample", count: 2 });
});

test("the list note counts the history page, not extra old in-flight rows", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const view = stage([
    request({ id: "long", createdAt: at(120), status: "processing" }),
    request({ id: "done", createdAt: at(1), status: "completed" }),
  ], {
    now,
    window: "live",
    limit: 50,
    totals: { requests: 80, inFlight: 1, failed: 4 },
    historyCount: 50,
  });
  assert.equal(view.windowCount, 2);
  assert.equal(view.pageCount, 50);
  assert.deepEqual(view.note, { kind: "list", count: 50 });
});

test("format and page sources stay inside the plugin boundary", () => {
  const formatSource = readFileSync(new URL("./ui/live/format.js", import.meta.url), "utf8");
  assert.equal(formatSource.includes("document"), false);
  assert.equal(formatSource.includes("axonhub"), false);
  assert.equal(formatSource.includes("Date.now"), false);
  assert.equal(/new Date\(\s*\)/.test(formatSource), false);

  const querySource = readFileSync(new URL("./ui/live/query.js", import.meta.url), "utf8");
  assert.equal(querySource.includes("document"), false);
  assert.equal(querySource.includes("axonhub"), false);
  assert.equal(querySource.includes("Date.now"), false);
  assert.equal(/new Date\(\s*\)/.test(querySource), false);

  const appSource = readFileSync(new URL("./ui/live/app.js", import.meta.url), "utf8");
  for (const word of ["requestBody", "responseBody", "requestHeaders", "responseHeaders", "responseChunks", "requestURL", "usageLogs"]) {
    assert.equal(appSource.includes(word), false, word);
  }
  assert.equal(appSource.includes("alert("), false);

  const claude = iconSvg("claude");
  assert.match(claude, /<svg/);
  assert.match(claude, /currentColor/);
  assert.equal(claude.includes("<script"), false);
  assert.equal(/\son[a-z]+=/i.test(claude), false);
  assert.match(iconSvg("python-urllib"), />P</);
});

test("query builders keep the window, the separate in-flight query, and first: 1 counts", () => {
  const history = historyQuery(true);
  assert.match(history, /first: \$first/);
  assert.match(history, /createdAtGTE: \$since/);
  assert.equal(history.includes("statusIn"), false);
  assert.match(history, /channel \{ id name \} channelID/);
  assert.equal(history.includes("totalCount"), false);

  const historyNoChannel = historyQuery(false);
  assert.equal(historyNoChannel.includes("channel { id name }"), false);
  assert.match(historyNoChannel, /channelID/);

  const inflight = inflightQuery(true);
  assert.match(inflight, /statusIn: \[pending, processing\]/);
  assert.equal(inflight.includes("createdAtGTE"), false);
  assert.match(inflight, /\$after: Cursor/);
  assert.match(inflight, /pageInfo \{ hasNextPage endCursor \}/);
  assert.equal(inflight.includes("totalCount"), false);
  assert.match(inflight, /channel \{ id name \} channelID/);
  for (const query of [history, historyNoChannel, inflight]) {
    assert.match(query, /channelID/);
  }

  const stats = statsQuery();
  assert.equal((stats.match(/first: 1/g) || []).length, 3);
  for (const query of [history, inflight, stats, executionsQuery(["a"], false)]) {
    assert.equal(query.includes("first: 0"), false);
  }
  assert.equal(executionsQuery(["a"], false).includes("channel { id name }"), false);

  const executions = executionsQuery(["a", "b"], true);
  assert.match(executions, /executions\(first: 8, orderBy: \{ field: CREATED_AT, direction: DESC \}\)/);
  assert.match(executions, /totalCount/);
  assert.match(executions, /channel \{ id name \} channelID/);

  const state = requestStateQuery(["a", "b"], false);
  assert.match(state, /r0: node\(id: "a"\)/);
  assert.match(state, /r1: node\(id: "b"\)/);
  assert.match(state, /\.\.\. on Request/);
  assert.match(state, /channelID/);
  assert.equal(requestStateQuery(["a"], true).includes("channel { id name }"), true);
});

test("stats read only totalCount from a valid first page", () => {
  assert.deepEqual(readTotals({
    window: { totalCount: 80, edges: [{ node: { id: "a" } }] },
    inFlight: { totalCount: 4 },
    failed: { totalCount: 7 },
  }), { requests: 80, inFlight: 4, failed: 7 });
  assert.equal(readTotals({ window: { totalCount: 1 }, inFlight: null, failed: { totalCount: 0 } }), null);
  assert.equal(readTotals({ window: { edges: [] }, inFlight: { totalCount: 0 }, failed: { totalCount: 0 } }), null);
  assert.equal(readTotals({ window: { totalCount: -1 }, inFlight: { totalCount: 0 }, failed: { totalCount: 0 } }), null);
});

test("time windows, row limits, refresh options, and stats throttling have stable defaults", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  assert.equal(DEFAULT_WINDOW, "15m");
  assert.equal(windowMsFor("5m"), 5 * 60 * 1000);
  assert.equal(windowMsFor("10m"), 10 * 60 * 1000);
  assert.equal(windowMsFor("15m"), 15 * 60 * 1000);
  assert.equal(windowMsFor("30m"), 30 * 60 * 1000);
  assert.equal(windowMsFor("1h"), 60 * 60 * 1000);
  assert.equal(windowMsFor("2h"), 120 * 60 * 1000);
  assert.equal(windowMsFor("today"), null);
  assert.equal(windowMsFor("bad"), 15 * 60 * 1000);
  assert.equal(clampWindow("bad"), DEFAULT_WINDOW);
  assert.equal(WINDOW_OPTIONS.map((option) => option.id).join(","), "5m,10m,15m,30m,1h,2h,today");

  assert.equal(sinceFor("30m", now).getTime(), now.getTime() - 30 * 60 * 1000);
  assert.equal(sinceFor("today", now).getTime(), new Date(2026, 5, 1, 0, 0, 0).getTime());

  assert.equal(DEFAULT_LIMIT, 50);
  assert.equal(clampLimit("200"), 200);
  assert.equal(clampLimit("7"), 50);
  assert.equal(clampLimit("0"), 50);
  assert.ok(INFLIGHT_LIMIT > 0);
  assert.equal(DEFAULT_REFRESH, 2);
  assert.deepEqual(REFRESH_SECONDS, [2, 5, 10, 20, 30, 60]);
  assert.equal(clampRefresh("20"), 20);
  assert.equal(clampRefresh("3"), 2);

  assert.equal(statsDue(0, now), true);
  assert.equal(statsDue(now.getTime() - STATS_MIN_MS + 1, now), false);
  assert.equal(statsDue(now.getTime() - STATS_MIN_MS, now), true);
  assert.equal(STATS_MIN_MS, 10 * 1000);
});

test("in-flight and history merge by request id and keep old long requests", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const longRequest = request({
    id: "long",
    createdAt: at(180),
    status: "processing",
    userAgent: "cursor/1",
  });
  const merged = mergeRequests(
    [longRequest, request({ id: "fresh-inflight", createdAt: at(2), status: "processing" })],
    [
      request({ id: "long", createdAt: at(180), status: "completed" }),
      request({ id: "old-done", createdAt: at(180), status: "completed" }),
      request({ id: "fresh-inflight", createdAt: at(2), status: "processing" }),
      request({ id: "fresh-done", createdAt: at(1), status: "completed" }),
    ],
  );
  assert.equal(merged.length, 4);
  assert.equal(merged.find((item) => item.id === "long").status, "processing");
  assert.equal(merged.find((item) => item.id === "fresh-inflight").status, "processing");

  const view = stage(merged, { now, window: "live", windowMs: windowMsFor("30m") });
  const ids = view.log.map((row) => row.id);
  assert.ok(ids.includes("long"));
  assert.ok(ids.includes("fresh-inflight"));
  assert.ok(ids.includes("fresh-done"));
  assert.equal(ids.includes("old-done"), false);
});

function concurrentSet(now, count, partial) {
  return Array.from({ length: count }, (_, index) => request({
    id: `r${index}`,
    createdAt: new Date(now.getTime() - index * 1000).toISOString(),
    status: "processing",
    userAgent: "cursor/1",
    apiKey: { id: "k", name: "key", status: "enabled", type: "user" },
    channel: { id: "c1", name: "Alpha" },
    executions: [{
      id: `e${index}`,
      createdAt: new Date(now.getTime() - index * 1000).toISOString(),
      status: "processing",
      modelID: "m",
      channel: { id: "c1", name: "Alpha" },
      channelAPIKeySuffix: "same",
    }],
    ...partial,
  }));
}

test("request colors are stable per id, unique enough, and never red", () => {
  assert.ok(REQUEST_HUES.length >= TRACK_CAP);
  for (const hue of REQUEST_HUES) assert.equal(isNearPureRed(hue), false, hue);
  const ids = ["r0", "r1", "r2", "long-request-id", "9f3b2a", "r0"];
  for (const id of ids) {
    assert.equal(requestHue(id), requestHue(id));
    assert.equal(isNearPureRed(requestHue(id)), false, id);
  }
  assert.equal(requestHue("failure-is-red"), requestHue("failure-is-red"));
});

test("five, ten, and twelve identical concurrent requests each get a visible lane", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  for (const count of [5, 10, 12]) {
    const view = stage(concurrentSet(now, count), { now, window: "live" });
    const expected = Math.min(count, TRACK_CAP);
    assert.equal(view.focusId, "", `no auto selection at ${count}`);
    assert.equal(view.tracks.length, expected, `lanes at ${count}`);
    assert.equal(view.trackOverflow, Math.max(0, count - TRACK_CAP), `overflow at ${count}`);
    assert.equal(new Set(view.tracks.map((track) => track.id)).size, expected);
    assert.deepEqual(view.tracks.map((track) => track.trackIndex), Array.from({ length: expected }, (_, index) => index + 1));
    for (const track of view.tracks) {
      assert.equal(track.rowId, "c1:same");
      assert.equal(track.clientId, "cursor");
      assert.equal(track.keyId, "k");
      assert.equal(track.live, true);
    }
    const offsets = Array.from({ length: expected }, (_, index) => laneOffsetFor(index, expected));
    assert.equal(new Set(offsets).size, expected, `distinct lane offsets at ${count}`);
    for (let index = 1; index < offsets.length; index += 1) assert.ok(offsets[index] > offsets[index - 1]);
    for (const offset of offsets) assert.ok(Math.abs(offset) <= 24);
    assert.equal(view.channels.length, 1);
    assert.equal(view.channels[0].tracks.length, expected);
    // The list still shows every request, including overflow lanes.
    assert.equal(view.log.length, count);
    assert.equal(view.counts.inFlight, count);
  }
});

test("multiple clients and channels keep their own lanes and nodes", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (seconds) => new Date(now.getTime() - seconds * 1000).toISOString();
  const view = stage([
    request({
      id: "cursor-a",
      userAgent: "cursor/1",
      createdAt: at(1),
      status: "processing",
      channel: { id: "c1", name: "Alpha" },
    }),
    request({
      id: "claude-b",
      userAgent: "claude-cli/1",
      createdAt: at(2),
      status: "processing",
      channel: { id: "c2", name: "Beta" },
    }),
    request({
      id: "cursor-c",
      userAgent: "cursor/1",
      createdAt: at(3),
      status: "processing",
      channel: { id: "c3", name: "Gamma" },
    }),
  ], { now, window: "live" });
  assert.equal(view.tracks.length, 3);
  assert.deepEqual(view.tracks.map((track) => track.id), ["cursor-c", "claude-b", "cursor-a"]);
  assert.deepEqual(
    view.tracks.map((track) => [track.clientId, track.rowId]),
    [["cursor", "c3:"], ["claude", "c2:"], ["cursor", "c1:"]],
  );
  assert.equal(view.clients.length, 2);
  assert.equal(view.channels.length, 3);
  assert.equal(view.focusId, "");
});

test("click toggles selection and Esc-style clearing returns to the overview", () => {
  assert.equal(toggleSelection("", "a"), "a");
  assert.equal(toggleSelection("a", "b"), "b");
  assert.equal(toggleSelection("a", "a"), "");
  assert.equal(toggleSelection("a", ""), "");
  assert.equal(toggleSelection("", ""), "");

  const now = new Date(2026, 5, 1, 12, 0, 0);
  const requests = concurrentSet(now, 3);
  const selected = stage(requests, { now, window: "live", selectedId: toggleSelection("", "r1") });
  assert.equal(selected.focusId, "r1");
  assert.equal(selected.focus.id, "r1");
  const cleared = stage(requests, { now, window: "live", selectedId: toggleSelection("r1", "r1") });
  assert.equal(cleared.focusId, "");
  assert.equal(cleared.focus, null);
});

test("a completed lane lingers for about three seconds and then falls back to static history", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const requests = [
    request({ id: "done", createdAt: new Date(now.getTime() - 8000).toISOString(), status: "completed" }),
    request({ id: "bad", createdAt: new Date(now.getTime() - 9000).toISOString(), status: "failed" }),
  ];
  const live = stage(requests, {
    now,
    window: "live",
    lingering: [
      { id: "done", doneAt: now.getTime() - 1000 },
      { id: "bad", doneAt: now.getTime() - 1000 },
    ],
  });
  assert.equal(live.tracks.length, 2);
  assert.equal(live.trackOverflow, 0);
  const done = live.tracks.find((track) => track.id === "done");
  const bad = live.tracks.find((track) => track.id === "bad");
  assert.equal(done.lingering, true);
  assert.equal(done.static, false);
  assert.equal(done.live, false);
  assert.equal(bad.failed, true);
  assert.equal(bad.hue, "#b42318");
  assert.equal(bad.lingering, true);
  assert.equal(live.staticOverview, false);

  const expired = stage(requests, {
    now,
    window: "live",
    lingering: [
      { id: "done", doneAt: now.getTime() - LINGER_MS },
      { id: "bad", doneAt: now.getTime() - LINGER_MS },
    ],
  });
  assert.equal(expired.tracks.every((track) => track.static), true);
  assert.equal(expired.tracks.every((track) => !track.lingering), true);
  assert.equal(expired.staticOverview, true);
});

test("no in-flight work shows a static recent-history overview, not one permanent focus", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const view = stage([
    request({ id: "one", createdAt: at(1), status: "completed" }),
    request({ id: "two", createdAt: at(2), status: "completed" }),
    request({ id: "three", createdAt: at(3), status: "failed" }),
  ], { now, window: "live" });
  assert.equal(view.counts.inFlight, 0);
  assert.equal(view.focusId, "");
  assert.equal(view.focus, null);
  assert.equal(view.tracks.length, 3);
  assert.equal(view.staticOverview, true);
  assert.equal(view.tracks.every((track) => track.static && !track.live), true);
  assert.equal(view.tracks.find((track) => track.id === "three").hue, "#b42318");
  assert.equal(view.log.length, 3);
});

test("twelve concurrent lanes keep a selected overflow request visible", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const requests = concurrentSet(now, 12);
  const view = stage(requests, { now, window: "live", selectedId: "r11" });
  assert.equal(view.tracks.length, TRACK_CAP);
  assert.equal(view.trackOverflow, 2);
  assert.ok(view.tracks.some((track) => track.id === "r11"));
  assert.equal(view.focusId, "r11");
  assert.equal(view.focus.trackIndex, view.tracks.find((track) => track.id === "r11").trackIndex);
});

test("an old long request keeps its lane while the window only shows recent history", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (minutes) => new Date(now.getTime() - minutes * 60 * 1000).toISOString();
  const view = stage([
    request({ id: "long", createdAt: at(180), status: "processing", userAgent: "cursor/1" }),
    request({ id: "fresh", createdAt: at(1), status: "completed" }),
  ], { now, window: "live" });
  assert.equal(view.tracks.length, 1);
  assert.equal(view.tracks[0].id, "long");
  assert.equal(view.tracks[0].live, true);
  assert.equal(view.staticOverview, false);
  assert.ok(view.log.some((row) => row.id === "long"));
});

test("more than eight attempts uses the newest eight and the final channel", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (seconds) => new Date(now.getTime() - seconds * 1000).toISOString();
  const edges = [];
  // The query returns newest first; attempts 5 through 12 are the page.
  for (let attempt = 12; attempt >= 5; attempt -= 1) {
    const final = attempt === 12;
    edges.push({ node: {
      id: `e${attempt}`,
      createdAt: at(13 - attempt),
      status: final ? "completed" : "failed",
      modelID: "m",
      metricsLatencyMs: final ? 700 : 20,
      channel: { id: final ? "c-final" : `c${attempt}`, name: final ? "Final" : `Old ${attempt}` },
      channelAPIKeySuffix: final ? "zzzz" : "aaaa",
      errorMessage: final ? "" : "retry",
      responseStatusCode: final ? 200 : 500,
    } });
  }
  const data = { r0: { id: "long", executions: { totalCount: 12, edges } } };
  const attached = attachExecutions([{ id: "long" }], ["long"], data);
  assert.equal(attached[0].executions.length, 8);
  assert.equal(attached[0].executionTotalCount, 12);
  const view = stage([request({
    id: "long",
    createdAt: at(30),
    status: "completed",
    executions: attached[0].executions,
    executionTotalCount: attached[0].executionTotalCount,
  })], { now, window: "live", selectedId: "long" });
  assert.equal(view.focus.executionIndex, 12);
  assert.equal(view.focus.channelName, "Final");
  assert.equal(view.focus.rowId, "c-final:zzzz");
  const finalRow = view.channels.find((row) => row.channelId === "c-final");
  assert.equal(finalRow.kind, "completed");
  assert.equal(finalRow.latencyMs, 700);
});

test("execution lookups cover ten concurrent requests and the selected request", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (seconds) => new Date(now.getTime() - seconds * 1000).toISOString();
  const requests = Array.from({ length: 12 }, (_, index) => request({
    id: `r${index}`,
    createdAt: at(index),
    status: "processing",
  }));
  const plain = executionIds(requests, "");
  assert.equal(plain.length, TRACK_CAP);
  assert.deepEqual(plain, ["r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9"]);

  const liveSelected = executionIds(requests, "r11");
  assert.equal(liveSelected.length, TRACK_CAP);
  assert.ok(liveSelected.includes("r11"));
  assert.equal(liveSelected.includes("r9"), false);

  const completed = [...requests, request({ id: "done", createdAt: at(20), status: "completed" })];
  const completedSelected = executionIds(completed, "done");
  assert.equal(completedSelected.length, TRACK_CAP + 1);
  assert.ok(completedSelected.includes("done"));
});

test("a general FORBIDDEN allows one no-channel retry and keeps channelID", () => {
  assert.equal(isForbiddenError({ extensions: { code: "FORBIDDEN" } }), true);
  assert.equal(isForbiddenError({ message: "Forbidden" }), true);
  assert.equal(isForbiddenError({ message: "boom" }), false);
  assert.equal(hasForbidden([{ extensions: { code: "FORBIDDEN" } }]), true);
  assert.equal(shouldRetryWithoutChannel(true, [{ extensions: { code: "FORBIDDEN" } }]), true);
  assert.equal(shouldRetryWithoutChannel(false, [{ extensions: { code: "FORBIDDEN" } }]), false);
  assert.equal(shouldRetryWithoutChannel(true, [{ message: "other" }]), false);

  assert.equal(channelRetrySucceeded(true, true, []), true);
  assert.equal(channelRetrySucceeded(true, false, []), false);
  assert.equal(channelRetrySucceeded(true, true, [{ message: "still forbidden" }]), false);

  const forbidden = [{ extensions: { code: "FORBIDDEN" } }];
  assert.deepEqual(channelRetryPlan(true, forbidden, []), { history: true, inflight: false });
  assert.deepEqual(channelRetryPlan(true, [], forbidden), { history: false, inflight: true });
  assert.deepEqual(channelRetryPlan(false, forbidden, forbidden), { history: false, inflight: false });
  assert.deepEqual(transportErrors({ ok: false, error: { message: "down" } }), [{ message: "down" }]);
  assert.deepEqual(transportErrors({ ok: true, value: {} }), []);

  const withChannel = historyQuery(true);
  assert.match(withChannel, /channel \{ id name \} channelID/);
  const withoutChannel = historyQuery(false);
  assert.equal(withoutChannel.includes("channel { id name }"), false);
  assert.match(withoutChannel, /channelID/);
  assert.equal(mapRequest({ id: "a", channelID: "c9" }).channel.id, "c9");
  assert.equal(mapRequest({ id: "b", channelID: "c9", channel: { id: "c9", name: "Nine" } }).channel.name, "Nine");
});

test("stats keep a global throttle across window changes and reject only stale stats", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const last = now.getTime() - 4000;
  assert.equal(statsDue(last, now), false);
  assert.equal(statsDue(last, new Date(now.getTime() + STATS_MIN_MS)), true);

  const appSource = readFileSync(new URL("./ui/live/app.js", import.meta.url), "utf8");
  assert.equal((appSource.match(/statsAt = 0/g) || []).length, 1);
  assert.match(appSource, /statsVersion \+= 1/);

  assert.equal(statsAccept(1, 1, 1, 1), true);
  assert.equal(statsAccept(1, 2, 1, 1), false);
  assert.equal(statsAccept(1, 1, 1, 2), false);
  assert.equal(statsAccept(2, 2, 3, 3), true);
});

test("successful stats stay visible when the first list request returned no rows", () => {
  assert.equal(statsVisible(true, 0), true);
  assert.equal(statsVisible(false, 0), false);
  assert.equal(statsVisible(false, 3), true);

  const now = new Date(2026, 5, 1, 12, 0, 0);
  const view = stage([], {
    now,
    window: "live",
    totals: { requests: 7, inFlight: 3, failed: 2 },
  });
  assert.equal(view.shown, 0);
  assert.equal(view.tracks.length, 0);
  assert.deepEqual(view.counts, { requests: 7, inFlight: 3, failed: 2 });
  assert.equal(view.scope, "window");
});

test("stats own their window independently of the list fetch", () => {
  const totals = { requests: 9, inFlight: 3, failed: 1 };
  assert.equal(statsWindowMatches("10m", "10m"), true);
  assert.equal(statsWindowMatches("10m", "15m"), false);
  assert.equal(statsWindowMatches("", "10m"), false);
  assert.equal(statsTotalsForWindow("10m", "10m", totals, true), totals);
  assert.equal(statsTotalsForWindow("10m", "10m", totals, false), null);
  assert.equal(statsTotalsForWindow("10m", "15m", totals, true), null);

  const now = new Date(2026, 5, 1, 12, 0, 0);
  const view = stage([], { now, window: "10m", totals });
  assert.equal(view.scope, "window");
  assert.deepEqual(view.counts, totals);
});

test("a visible live request that disappears is retired by id, never assumed complete", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const previous = new Map([
    ["visible", request({ id: "visible", createdAt: new Date(now.getTime() - 1000).toISOString(), status: "processing" })],
    ["selected", request({ id: "selected", createdAt: new Date(now.getTime() - 2000).toISOString(), status: "processing" })],
    ["done", request({ id: "done", createdAt: new Date(now.getTime() - 3000).toISOString(), status: "completed" })],
  ]);
  const current = [request({ id: "done", createdAt: new Date(now.getTime() - 3000).toISOString(), status: "completed" })];
  assert.deepEqual(retiredLiveIds(previous, current, ["visible", "done"], "selected"), ["visible", "selected"]);
  assert.deepEqual(retiredLiveIds(previous, current, ["done"], "done"), []);

  // A missing response is unknown; an explicit null node means gone.
  const unresolved = readStateNodes({}, ["visible"]).get("visible");
  assert.deepEqual(unresolved, { resolved: false, request: null });
  const gone = readStateNodes({ r0: null }, ["visible"]).get("visible");
  assert.deepEqual(gone, { resolved: true, request: null });
  const states = readStateNodes({ r0: { id: "visible", status: "completed", channelID: "c9" } }, ["visible"]);
  assert.equal(states.get("visible").request.channel.id, "c9");
});

test("eighty seconds of failed backfill never leaves a request permanently active", () => {
  const since = 0;
  for (let seconds = 0; seconds <= 80; seconds += 10) {
    const decision = backfillDecision(seconds * 1000, since, null, false);
    if (seconds * 1000 < STALE_TTL_MS) assert.equal(decision, "stale", `at ${seconds}s`);
    else assert.equal(decision, "expire", `at ${seconds}s`);
  }
  assert.equal(backfillDecision(80_000, 0, null, true), "gone");
  assert.equal(backfillDecision(80_000, 0, request({ id: "a", status: "processing" }), true), "active");
  assert.equal(backfillDecision(80_000, 0, request({ id: "a", status: "completed" }), true), "terminal");

  const now = new Date(2026, 5, 1, 12, 0, 0);
  const stale = request({
    id: "stale-old",
    createdAt: new Date(now.getTime() - 2 * 60 * 60 * 1000).toISOString(),
    status: "stale",
    channel: { id: "c-old", name: "Old" },
  });
  const view = stage([], { now, window: "live", stale: [stale] });
  assert.equal(view.tracks.length, 1);
  assert.equal(view.tracks[0].stale, true);
  assert.equal(view.tracks[0].live, false);
  assert.equal(view.tracks[0].static, false);
  assert.equal(view.staticOverview, false);
  assert.equal(view.staleCount, 1);
  assert.equal(channelStatusText("stale", "zh-CN"), "状态未知");
});

test("a terminal-state request outside the history window still gets a lane", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const old = request({
    id: "old-long",
    createdAt: new Date(now.getTime() - 3 * 60 * 60 * 1000).toISOString(),
    status: "completed",
    channel: { id: "c-old", name: "Old" },
  });
  const view = stage([], {
    now,
    window: "live",
    lingering: [{ id: "old-long", doneAt: now.getTime(), request: old }],
  });
  assert.equal(view.shown, 0);
  assert.equal(view.tracks.length, 1);
  assert.equal(view.tracks[0].id, "old-long");
  assert.equal(view.tracks[0].lingering, true);
  assert.equal(view.tracks[0].rowId, "c-old:");
  assert.equal(view.staticOverview, false);
  assert.deepEqual(visibleExecutionIds(view.tracks, ""), ["old-long"]);
});

test("a lingering terminal request keeps attempt 12 and the final channel", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const terminal = request({
    id: "old-retry",
    createdAt: new Date(now.getTime() - 4 * 60 * 60 * 1000).toISOString(),
    status: "completed",
    executionTotalCount: 12,
    executions: [{
      id: "e12",
      createdAt: new Date(now.getTime() - 3 * 60 * 60 * 1000).toISOString(),
      status: "completed",
      modelID: "m",
      metricsLatencyMs: 700,
      channel: { id: "c-final", name: "Final" },
      channelAPIKeySuffix: "zzzz",
    }],
  });
  const view = stage([], {
    now,
    window: "live",
    selectedId: "old-retry",
    lingering: [{ id: "old-retry", doneAt: now.getTime(), request: terminal }],
  });
  assert.equal(view.tracks.length, 1);
  assert.equal(view.focus.executionIndex, 12);
  assert.equal(view.focus.channelName, "Final");
  assert.equal(view.focus.rowId, "c-final:zzzz");
  assert.equal(view.channels[0].latencyMs, 700);
});

test("execution lookups follow the filtered visible lanes and then the selection", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (seconds) => new Date(now.getTime() - seconds * 1000).toISOString();
  const requests = [
    request({ id: "cursor-1", userAgent: "cursor/1", createdAt: at(1), status: "processing" }),
    request({ id: "cursor-2", userAgent: "cursor/1", createdAt: at(2), status: "processing" }),
    request({ id: "claude-1", userAgent: "claude-cli/1", createdAt: at(3), status: "processing" }),
    request({ id: "done", userAgent: "cursor/1", createdAt: at(4), status: "completed" }),
  ];
  const filtered = stage(requests, { now, window: "live", clientFilter: "cursor", selectedId: "done" });
  assert.deepEqual(filtered.tracks.filter((track) => track.live).map((track) => track.id).sort(), ["cursor-1", "cursor-2"]);
  assert.deepEqual(visibleExecutionIds(filtered.tracks, "done").sort(), ["cursor-1", "cursor-2", "done"]);

  const many = stage(concurrentSet(now, 12), { now, window: "live" });
  assert.equal(many.tracks.filter((track) => track.live).length, TRACK_CAP);
  assert.equal(visibleExecutionIds(many.tracks, "").length, TRACK_CAP);

  const appSource = readFileSync(new URL("./ui/live/app.js", import.meta.url), "utf8");
  assert.match(appSource, /filterEl\.addEventListener\("change"[\s\S]*?poll\(\);/);
});

test("in-flight cursor pages cover 201 rows and the budget reports loaded versus total", () => {
  const now = new Date(2026, 5, 1, 12, 0, 0);
  const at = (seconds) => new Date(now.getTime() - seconds * 1000).toISOString();
  const firstPage = {
    totalCount: 201,
    pageInfo: { hasNextPage: true, endCursor: "c1" },
    edges: Array.from({ length: 200 }, (_, index) => ({ node: { id: `p${index}` } })),
  };
  const secondPage = {
    totalCount: 201,
    pageInfo: { hasNextPage: false, endCursor: "c2" },
    edges: [{ node: { id: "p200" } }],
  };
  assert.deepEqual(pageInfoOf(firstPage), { hasNext: true, endCursor: "c1" });
  assert.equal(nextInflightCursor(firstPage, 200), "c1");
  assert.equal(nextInflightCursor(secondPage, 201), null);
  assert.equal(nextInflightCursor(firstPage, INFLIGHT_BUDGET), null);

  const requests = Array.from({ length: 201 }, (_, index) => request({
    id: `r${index}`,
    createdAt: at(201 - index),
    status: "processing",
  }));
  const view = stage(requests, { now, window: "live", inFlightTotal: 201 });
  assert.equal(view.inFlightLoaded, 201);
  assert.equal(view.inFlightTotal, 201);
  assert.equal(view.inFlightTruncated, false);
  assert.equal(view.trackTotal, 201);
  assert.equal(view.trackOverflow, 191);

  const bounded = stage(requests, { now, window: "live", inFlightTotal: 1200 });
  assert.equal(bounded.inFlightTruncated, true);
  assert.equal(bounded.trackOverflow, 1190);
  assert.equal(bounded.inFlightLoaded, 201);

  const lowerBound = stage(requests, { now, window: "live", inFlightMore: true });
  assert.equal(lowerBound.inFlightTotal, null);
  assert.equal(lowerBound.inFlightMore, true);
  assert.equal(lowerBound.trackOverflow, 191);

  const thousand = Array.from({ length: INFLIGHT_BUDGET }, (_, index) => request({
    id: `t${index}`,
    createdAt: at(INFLIGHT_BUDGET - index),
    status: "processing",
  }));
  const atBudget = stage(thousand, { now, window: "live", inFlightMore: true });
  assert.equal(atBudget.inFlightLoaded, INFLIGHT_BUDGET);
  assert.equal(atBudget.inFlightMore, true);
  assert.equal(atBudget.inFlightTotal, null);
  assert.equal(atBudget.trackOverflow, INFLIGHT_BUDGET - TRACK_CAP);
  const fullPage = { pageInfo: { hasNextPage: true, endCursor: "c1000" }, edges: [] };
  assert.equal(pageInfoOf(fullPage).hasNext, true);
});

test("the SVG track click target keeps a 14px hot area", () => {
  const css = readFileSync(new URL("./ui/live/style.css", import.meta.url), "utf8");
  assert.match(css, /\.wires \.track-hit \{[^}]*stroke-width:\s*14/);
  assert.equal(/^\.track-hit\s*\{/m.test(css), false);
});

test("the real load path runs against a mock bridge without a runtime ReferenceError", async () => {
  const elements = new Map();
  const makeElement = () => {
    const el = {
      children: [],
      options: [],
      dataset: {},
      style: { setProperty() {}, animationDelay: "" },
      classList: { add() {}, remove() {}, toggle() {}, contains() { return false; } },
      hidden: false,
      textContent: "",
      title: "",
      value: "",
      tabIndex: 0,
      type: "",
      innerHTML: "",
      scrollTop: 0,
      offsetWidth: 0,
      addEventListener() {},
      append(...nodes) {
        el.children.push(...nodes);
        el.options.push(...nodes);
      },
      replaceChildren(...nodes) {
        el.children = nodes;
        el.options = nodes.slice();
      },
      setAttribute() {},
      getAttribute() { return null; },
      remove() {},
      querySelectorAll() { return []; },
      getBoundingClientRect() { return { left: 0, top: 0, right: 100, bottom: 50, width: 100, height: 50 }; },
      closest() { return null; },
      click() {},
    };
    return el;
  };
  const querySelector = (selector) => {
    if (!elements.has(selector)) elements.set(selector, makeElement());
    return elements.get(selector);
  };
  const documentStub = {
    hidden: true,
    title: "",
    documentElement: { lang: "en", dataset: {} },
    querySelector,
    createElement: () => makeElement(),
    createElementNS: () => makeElement(),
    addEventListener() {},
  };
  const calls = [];
  const bridge = {
    ready: async () => ({ locale: "en" }),
    onTheme: () => () => {},
    graphql: async (query, vars) => {
      calls.push({ query, vars });
      if (query.includes("LiveStats")) {
        return { data: { window: { totalCount: 0 }, inFlight: { totalCount: 0 }, failed: { totalCount: 0 } } };
      }
      if (query.includes("channel { id name }")) {
        return { data: null, errors: [{ message: "Forbidden", extensions: { code: "FORBIDDEN" } }] };
      }
      if (query.includes("LiveHistory")) return { data: { requests: { edges: [] } } };
      if (query.includes("LiveInFlight")) {
        return { data: { requests: { pageInfo: { hasNextPage: false, endCursor: null }, edges: [] } } };
      }
      return { data: {} };
    },
  };
  globalThis.window = { axonhub: bridge, addEventListener() {} };
  globalThis.document = documentStub;
  try {
    const url = new URL("./ui/live/app.js", import.meta.url);
    url.search = `?smoke=${Date.now()}`;
    await import(url.href);
    for (let waited = 0; waited < 1000; waited += 5) {
      const retried = calls.some((call) => call.query.includes("LiveInFlight") && !call.query.includes("channel { id name }"));
      if (retried) break;
      await new Promise((resolve) => setTimeout(resolve, 5));
    }
    await new Promise((resolve) => setTimeout(resolve, 20));
    assert.ok(calls.some((call) => call.query.includes("LiveHistory")));
    assert.ok(calls.some((call) => call.query.includes("LiveInFlight") && !call.query.includes("channel { id name }")));
    assert.equal(elements.get("#hint").hidden, false);
    assert.equal(elements.get("#status").hidden, true);
    assert.equal(elements.get("#status").textContent.includes("TRACK_CAP"), false);
    assert.equal(elements.get("#status").textContent.toLowerCase().includes("permission"), false);
  } finally {
    delete globalThis.window;
    delete globalThis.document;
  }
});

test("one failed query settles while another continues, and stale cycles cannot commit", async () => {
  const [failed, passed] = await Promise.all([
    settle(async () => {
      throw new Error("stats down");
    }),
    settle(async () => "list ok"),
  ]);
  assert.equal(failed.ok, false);
  assert.equal(failed.error.message, "stats down");
  assert.deepEqual(passed, { ok: true, value: "list ok" });

  let committed = "";
  assert.equal(commitIfCurrent(1, 2, () => { committed = "old"; }), false);
  assert.equal(committed, "");
  assert.equal(commitIfCurrent(2, 2, () => { committed = "new"; }), true);
  assert.equal(committed, "new");

  const slow = (async () => {
    await new Promise((resolve) => setTimeout(resolve, 5));
    commitIfCurrent(3, 4, () => { committed = "stale"; });
  })();
  commitIfCurrent(4, 4, () => { committed = "current"; });
  await slow;
  assert.equal(committed, "current");
});
