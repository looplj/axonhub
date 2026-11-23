import { TRACK_CAP } from "./query.js";

export const SPARE_HUES = ["#8b5cf6", "#22c55e", "#0d9488", "#0891b2", "#ca8a04", "#db2777", "#2563eb", "#65a30d"];

// Lane colors are keyed by request id, never by position, so a request keeps
// its color across polls. Red is reserved for failed/canceled requests.
export const REQUEST_HUES = [
  ...SPARE_HUES,
  "#7c3aed", "#0f766e", "#0369a1", "#a16207", "#be185d", "#1d4ed8", "#4d7c0f", "#0e7490",
  "#9333ea", "#15803d",
];

export const LINGER_MS = 3000;

export const CLIENTS = [
  { id: "claude", name: "Claude Code", hue: "#d97757", aliases: ["claude-cli", "claude-code"] },
  { id: "codex", name: "Codex", hue: "#6366f1", aliases: ["codex", "acp-extension-codex"] },
  { id: "cursor", name: "Cursor", hue: "#14b8a6", aliases: ["cursor"] },
  { id: "gemini", name: "Gemini CLI", hue: "#0ea5e9", aliases: ["geminicli", "gemini-cli"] },
  { id: "copilot", name: "Copilot", hue: "#a855f7", aliases: ["copilot", "github-copilot"] },
  { id: "opencode", name: "OpenCode", hue: "#eab308", aliases: ["opencode"] },
  { id: "pi", name: "Pi", hue: "#10b981", aliases: ["pi", "pi-desktop"] },
  { id: "crush", name: "Crush", hue: "#ec4899", aliases: ["crush"] },
  { id: "goose", name: "Goose", hue: "#84cc16", aliases: ["goose"] },
  { id: "cline", name: "Cline", hue: "#22c55e", aliases: ["cline"] },
  { id: "zed", name: "Zed", hue: "#0d9488", aliases: ["zed"] },
  { id: "kimi", name: "Kimi CLI", hue: "#0891b2", aliases: ["kimicli", "kimi-cli"] },
  { id: "grok", name: "Grok", hue: "#ca8a04", aliases: ["grok-shell", "grok-pager", "xai-grok-build"] },
  { id: "droid", name: "Factory", hue: "#db2777", aliases: ["factory-cli"] },
  { id: "curl", name: "curl", hue: "#2563eb", aliases: ["curl"] },
  { id: "playground", name: "Playground", hue: "#8b5cf6", aliases: [] },
  { id: "test", name: "Test", hue: "#65a30d", aliases: [] },
  { id: "unknown", name: "未知客户端", hue: "#64748b", aliases: [] },
];

const LIVE_MS = 15 * 60 * 1000;
const CLIENT_CAP = 6;
const KEY_CAP = 8;
const CHANNEL_CAP = 12;
const STATIC_LINGER_LIMIT = 3;

const byId = new Map(CLIENTS.map((client) => [client.id, client]));

function hashOf(value) {
  let hash = 0;
  const text = String(value);
  for (let index = 0; index < text.length; index += 1) {
    hash = (hash * 31 + text.charCodeAt(index)) >>> 0;
  }
  return hash;
}

export function requestHue(id) {
  return REQUEST_HUES[hashOf(id) % REQUEST_HUES.length];
}

// Lanes sharing the same client/key/channel are vertically offset so their
// paths never collapse into one line.
export function laneOffsetFor(position, total) {
  if (!Number.isFinite(position) || !Number.isFinite(total) || total <= 1) return 0;
  const spacing = Math.min(9, 56 / (total + 1));
  return (position - (total - 1) / 2) * spacing;
}

export function toggleSelection(selectedId, id) {
  if (!id) return "";
  return selectedId === id ? "" : id;
}

export function isLiveStatus(status) {
  return status === "pending" || status === "processing";
}

// Execution lookups follow exactly the live lanes of the current client filter,
// then add the selected request for its detail pane.
export function visibleExecutionIds(tracks, selectedId) {
  const ids = [];
  for (const track of Array.isArray(tracks) ? tracks : []) {
    if (track && (track.live || track.lingering) && track.id && !ids.includes(track.id)) ids.push(track.id);
  }
  if (selectedId && !ids.includes(selectedId)) ids.push(selectedId);
  return ids;
}

function present(client) {
  return { id: client.id, name: client.name, hue: client.hue };
}

export function spareHue(id) {
  return SPARE_HUES[hashOf(id) % SPARE_HUES.length];
}

function productOf(userAgent) {
  const trimmed = String(userAgent || "").trim();
  if (!trimmed) return { raw: "", token: "" };
  const cut = trimmed.search(/[/\s]/);
  const raw = cut === -1 ? trimmed : trimmed.slice(0, cut);
  return { raw, token: raw.toLowerCase() };
}

export function identifyClient(userAgent, source) {
  const origin = String(source || "").toLowerCase();
  if (origin === "playground") return present(byId.get("playground"));
  const { raw, token } = productOf(userAgent);
  if (!token) {
    if (origin === "test") return present(byId.get("test"));
    return present(byId.get("unknown"));
  }
  for (const client of CLIENTS) {
    if (token === client.id || client.aliases.includes(token)) return present(client);
  }
  let best = null;
  let bestLength = 0;
  for (const client of CLIENTS) {
    for (const alias of client.aliases) {
      if (alias && token.startsWith(alias) && alias.length > bestLength) {
        best = client;
        bestLength = alias.length;
      }
    }
  }
  if (best) return present(best);
  return { id: token, name: raw, hue: spareHue(token) };
}

export function isChinese(locale) {
  return String(locale || "").toLowerCase().startsWith("zh");
}

export function formatTime(date, locale) {
  const value = date instanceof Date ? date : new Date(date);
  if (Number.isNaN(value.getTime())) return "--:--:--";
  const pad = (part) => String(part).padStart(2, "0");
  void locale;
  return `${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}`;
}

export function formatLatency(ms) {
  if (ms == null || ms === "" || !Number.isFinite(Number(ms)) || Number(ms) < 0) return "—";
  const value = Number(ms);
  if (value < 1000) return `${Math.round(value)}ms`;
  const seconds = value / 1000;
  if (seconds < 60) {
    const rounded = seconds >= 10 ? String(Math.round(seconds)) : String(Math.round(seconds * 10) / 10);
    return `${rounded}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60);
  return `${minutes}m ${rest}s`;
}

function truncateError(message) {
  const text = String(message || "");
  const chars = Array.from(text);
  if (chars.length <= 80) return text;
  return `${chars.slice(0, 80).join("")}…`;
}

export function channelStatusText(kind, locale, extra) {
  const zh = isChinese(locale);
  const info = extra || {};
  if (kind === "answering") return zh ? "回答中…" : "Answering…";
  if (kind === "other") return zh ? "在回答另一个请求…" : "Answering another request…";
  if (kind === "pending") return zh ? "等待中" : "Waiting";
  if (kind === "stale") return zh ? "状态未知" : "Unknown";
  if (kind === "completed") {
    const latency = formatLatency(info.latencyMs ?? info.latency);
    return zh ? `已完成 · ${latency}` : `Completed · ${latency}`;
  }
  if (kind === "failed") {
    const code = info.code ?? info.responseStatusCode ?? info.statusCode;
    const message = String(info.message ?? info.errorMessage ?? "").trim();
    const parts = [zh ? "失败" : "Failed"];
    if (code !== undefined && code !== null && code !== "") parts.push(String(code));
    if (message) parts.push(message);
    return parts.join(" · ");
  }
  return "";
}

export function clientLabel(client, locale) {
  if (!client) return "";
  if (client.id === "unknown") return isChinese(locale) ? "未知客户端" : "Unknown client";
  return client.name || client.id || "";
}

export function statusLine(view, locale) {
  const focus = view && view.focus;
  if (!focus) return "";
  const zh = isChinese(locale);
  const name = clientLabel({ id: focus.clientId, name: focus.clientName }, locale);
  const attempt = zh ? `第 ${focus.executionIndex} 次执行` : `attempt ${focus.executionIndex}`;
  const channel = focus.channelName || (zh ? "无渠道" : "no channel");
  const stream = focus.stream ? (zh ? "流式" : "streaming") : (zh ? "非流式" : "not streaming");
  const elapsed = formatLatency(focus.elapsedMs);
  const end = zh ? "。" : ".";
  return `${name} · ${focus.modelID} · ${attempt} · ${channel} · ${stream} · ${elapsed}${end}`;
}

function splitRow(rowId) {
  const text = String(rowId || "");
  const sep = text.lastIndexOf(":");
  if (sep === -1) return { id: text, suffix: "" };
  return { id: text.slice(0, sep), suffix: text.slice(sep + 1) };
}

// rowMotion distinguishes a real channel change from the suffix arriving
// on a later poll. "assign" is the first channel, "refine" fills in the
// key suffix, "switch" is a different channel or a different key.
export function rowMotion(prev, next) {
  if (!next || prev === next) return "same";
  if (!prev) return "assign";
  const from = splitRow(prev);
  const to = splitRow(next);
  if (!from.id || from.id !== to.id) return "switch";
  if (!from.suffix || !to.suffix || from.suffix === to.suffix) return "refine";
  return "switch";
}

function toTime(value) {
  if (value instanceof Date) return value.getTime();
  if (typeof value === "number") return value;
  return new Date(value).getTime();
}

function numberOrNull(value) {
  if (value == null || value === "") return null;
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

function inFlightStatus(status) {
  return status === "pending" || status === "processing";
}

function inWindow(entry, mode, now, windowMs) {
  if (inFlightStatus(entry.status)) return true;
  if (!Number.isFinite(entry.time)) return false;
  if (mode === "today") {
    const start = new Date(now.getTime());
    start.setHours(0, 0, 0, 0);
    const end = new Date(start.getTime());
    end.setDate(end.getDate() + 1);
    return entry.time >= start.getTime() && entry.time < end.getTime();
  }
  const span = Number.isFinite(windowMs) && windowMs > 0 ? windowMs : LIVE_MS;
  return entry.time >= now.getTime() - span;
}

function keyIdOf(entry) {
  return entry.apiKey && entry.apiKey.id ? entry.apiKey.id : "none";
}

function listExecutions(request) {
  if (!Array.isArray(request.executions) || request.executions.length === 0) return [];
  return request.executions.filter(Boolean).map((execution) => ({
    id: execution.id,
    time: toTime(execution.createdAt),
    status: String(execution.status || "").toLowerCase(),
    modelID: execution.modelID || request.modelID || "",
    responseStatusCode: execution.responseStatusCode,
    errorMessage: execution.errorMessage || "",
    metricsLatencyMs: execution.metricsLatencyMs,
    channelAPIKeySuffix: execution.channelAPIKeySuffix || "",
    channel: execution.channel || null,
  })).sort((left, right) => {
    const delta = (Number.isFinite(left.time) ? left.time : 0) - (Number.isFinite(right.time) ? right.time : 0);
    if (delta) return delta;
    return String(left.id || "").localeCompare(String(right.id || ""));
  });
}

function channelName(channel) {
  if (!channel || !channel.id) return "";
  return channel.name || channel.id;
}

function rowsOf(entry) {
  const executions = listExecutions(entry);
  const rows = [];
  for (const execution of executions) {
    const channel = execution.channel;
    if (!channel || !channel.id) continue;
    const suffix = execution.channelAPIKeySuffix || "";
    rows.push({
      rowId: `${channel.id}:${suffix}`,
      channelId: channel.id,
      name: channelName(channel),
      suffix,
      modelID: execution.modelID || entry.modelID,
      status: execution.status,
      time: Number.isFinite(execution.time) ? execution.time : entry.time,
      code: execution.responseStatusCode,
      error: execution.errorMessage,
      latencyMs: numberOrNull(execution.metricsLatencyMs),
      requestLatencyMs: numberOrNull(entry.metricsLatencyMs),
      requestId: entry.id,
    });
  }
  if (rows.length) return rows;
  if (entry.channel && entry.channel.id) {
    return [{
      rowId: `${entry.channel.id}:`,
      channelId: entry.channel.id,
      name: channelName(entry.channel),
      suffix: "",
      modelID: entry.modelID,
      status: entry.status,
      time: entry.time,
      code: null,
      error: "",
      latencyMs: numberOrNull(entry.metricsLatencyMs),
      requestLatencyMs: numberOrNull(entry.metricsLatencyMs),
      requestId: entry.id,
    }];
  }
  return [];
}

export function currentRowId(request) {
  const entry = normalize(request);
  const rows = rowsOf(entry);
  return rows.length ? rows[rows.length - 1].rowId : "";
}

function normalize(request) {
  const status = String((request && request.status) || "").toLowerCase();
  return {
    id: request && request.id,
    createdAt: request && request.createdAt,
    time: toTime(request && request.createdAt),
    status,
    modelID: (request && request.modelID) || "",
    stream: Boolean(request && request.stream),
    source: (request && request.source) || "",
    userAgent: (request && request.userAgent) || "",
    metricsLatencyMs: request && request.metricsLatencyMs,
    metricsFirstTokenLatencyMs: request && request.metricsFirstTokenLatencyMs,
    apiKey: request && request.apiKey && request.apiKey.id ? {
      id: request.apiKey.id,
      name: request.apiKey.name || "",
      status: request.apiKey.status || "",
      type: request.apiKey.type || "",
    } : null,
    channel: request && request.channel && request.channel.id ? {
      id: request.channel.id,
      name: request.channel.name || "",
    } : null,
    executions: request && request.executions,
    executionTotalCount: request && Number.isInteger(request.executionTotalCount) ? request.executionTotalCount : null,
    client: identifyClient(request && request.userAgent, request && request.source),
  };
}

function newestFirst(left, right) {
  return right.time - left.time || String(right.id).localeCompare(String(left.id));
}

function oldestFirst(left, right) {
  return left.time - right.time || String(left.id).localeCompare(String(right.id));
}

// Node caps keep whole lists readable. Tracks and the selected request are
// always kept, even when that means a column grows past its soft cap.
function capped(items, limit, keepIds) {
  const keyOf = (item) => item.id || item.rowId || "";
  const timeOf = (item) => (Number.isFinite(item.latestAt) ? item.latestAt : (Number.isFinite(item.activeAt) ? item.activeAt : 0));
  const compare = (left, right) => timeOf(right) - timeOf(left) || String(keyOf(left)).localeCompare(String(keyOf(right)));
  const sorted = [...items].sort(compare);
  const keep = new Set((keepIds || []).filter(Boolean));
  const kept = sorted.filter((item) => keep.has(keyOf(item)));
  const room = Math.max(0, limit - kept.length);
  const rest = sorted.filter((item) => !keep.has(keyOf(item))).slice(0, room);
  const shown = [...kept, ...rest].sort(compare);
  return { shown, overflow: Math.max(0, sorted.length - shown.length) };
}

function kindFor(status, requestId, selectedId) {
  if (status === "stale") return "stale";
  if (status === "processing") return selectedId && requestId !== selectedId ? "other" : "answering";
  if (status === "pending") return "pending";
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "completed") return "completed";
  return "pending";
}

function elapsedMs(entry, now) {
  const finished = entry.status === "completed" || entry.status === "failed" || entry.status === "canceled";
  if (finished) {
    const fromRequest = numberOrNull(entry.metricsLatencyMs);
    if (fromRequest != null) return fromRequest;
    const rows = rowsOf(entry);
    if (rows.length && rows[rows.length - 1].latencyMs != null) return rows[rows.length - 1].latencyMs;
  }
  if (!Number.isFinite(entry.time) || !Number.isFinite(now.getTime())) return 0;
  return Math.max(0, now.getTime() - entry.time);
}

function attemptIndex(entry) {
  if (Number.isInteger(entry.executionTotalCount) && entry.executionTotalCount > 0) return entry.executionTotalCount;
  return listExecutions(entry).length || 1;
}

function trackOf(entry, now, extra) {
  const rows = rowsOf(entry);
  const latest = rows.length ? rows[rows.length - 1] : null;
  const live = inFlightStatus(entry.status);
  const failed = entry.status === "failed" || entry.status === "canceled";
  const stale = entry.status === "stale";
  const lingering = Number.isFinite(extra.doneAt);
  return {
    id: entry.id,
    trackIndex: extra.index + 1,
    hue: failed ? "#b42318" : requestHue(entry.id),
    live,
    failed,
    stale,
    static: !live && !lingering && !stale,
    lingering,
    doneAt: lingering ? extra.doneAt : null,
    clientId: entry.client.id,
    clientName: entry.client.name,
    keyId: keyIdOf(entry),
    modelID: entry.modelID,
    stream: entry.stream,
    status: entry.status,
    executionIndex: attemptIndex(entry),
    channelName: latest ? latest.name : "",
    rowId: latest ? latest.rowId : "",
    elapsedMs: elapsedMs(entry, now),
    createdAt: entry.createdAt,
  };
}

export function buildStage({ requests, now, window: mode, windowMs, selectedId, focusId, clientFilter, limit, totals, historyCount, lingering, stale, inFlightTotal, inFlightMore }) {
  const nowDate = now instanceof Date ? now : new Date(now);
  const range = mode === "today" ? "today" : "live";
  const filter = clientFilter && clientFilter !== "all" ? clientFilter : "all";
  const all = (Array.isArray(requests) ? requests : []).filter(Boolean).map(normalize);
  const allById = new Map(all.map((entry) => [entry.id, entry]));
  const windowed = all.filter((entry) => inWindow(entry, range, nowDate, windowMs));
  const availableClients = clientsFrom(windowed);
  const filtered = filter === "all" ? windowed : windowed.filter((entry) => entry.client.id === filter);

  const lingeringAt = new Map();
  for (const item of Array.isArray(lingering) ? lingering : []) {
    if (!item || !item.id) continue;
    const at = toTime(item.doneAt);
    if (!Number.isFinite(at) || nowDate.getTime() - at >= LINGER_MS) continue;
    // A terminal-state lookup can carry a request that is already outside the
    // history window. It stays as a lane for the linger interval.
    const entry = allById.get(item.id) || (item.request ? normalize(item.request) : null);
    if (!entry || inFlightStatus(entry.status)) continue;
    if (filter !== "all" && entry.client.id !== filter) continue;
    lingeringAt.set(item.id, { at, entry });
  }

  // Selection is explicit. Nothing is auto-selected; a new request cannot take
  // the detail view away from the user.
  const wanted = selectedId !== undefined ? selectedId : focusId;
  const selectedFromList = wanted ? filtered.find((entry) => entry.id === wanted) : null;
  const selectedBase = selectedFromList || (wanted && lingeringAt.get(wanted) ? lingeringAt.get(wanted).entry : null);
  const liveSorted = filtered.filter((entry) => inFlightStatus(entry.status)).sort(newestFirst);
  const liveVisible = liveSorted.slice(0, TRACK_CAP);
  if (selectedBase && inFlightStatus(selectedBase.status) && !liveVisible.some((entry) => entry.id === selectedBase.id)) {
    // A selected request that would fall into +N is pulled back into the
    // visible lanes instead of silently disappearing.
    if (liveVisible.length < TRACK_CAP) liveVisible.push(selectedBase);
    else liveVisible[liveVisible.length - 1] = selectedBase;
  }

  const lingeringEntries = [];
  for (const [id, item] of lingeringAt) {
    if (liveVisible.some((entry) => entry.id === id)) continue;
    lingeringEntries.push({ entry: item.entry, doneAt: item.at });
  }
  lingeringEntries.sort((left, right) => right.doneAt - left.doneAt);
  lingeringEntries.length = Math.min(lingeringEntries.length, STATIC_LINGER_LIMIT);

  // Stale requests are not in the list or window; they stay visible without
  // active animation until a retry resolves them or the TTL expires.
  const staleEntries = [];
  for (const item of Array.isArray(stale) ? stale : []) {
    const entry = item ? normalize(item) : null;
    if (!entry || entry.status !== "stale") continue;
    if (filter !== "all" && entry.client.id !== filter) continue;
    if (filtered.some((known) => known.id === entry.id)) continue;
    if (lingeringAt.has(entry.id)) continue;
    staleEntries.push(entry);
  }
  const selectedStale = wanted ? staleEntries.find((entry) => entry.id === wanted) : null;
  const selectedEntry = selectedBase || selectedStale;
  const activeSelection = selectedEntry ? selectedEntry.id : "";

  let entries = [];
  if (!liveVisible.length) {
    if (lingeringEntries.length) {
      entries = lingeringEntries.map((item) => ({ entry: item.entry, doneAt: item.doneAt }));
      if (selectedEntry && !entries.some((item) => item.entry.id === selectedEntry.id)) {
        entries.push({ entry: selectedEntry });
      }
    } else {
      // No in-flight work: keep an overview of the most recent history rather
      // than falling back to one permanent focus row.
      const recent = [...filtered].sort(newestFirst).slice(0, TRACK_CAP);
      if (selectedFromList && !recent.some((entry) => entry.id === selectedFromList.id)) {
        if (recent.length < TRACK_CAP) recent.push(selectedFromList);
        else recent[recent.length - 1] = selectedFromList;
      }
      entries = recent.map((entry) => ({ entry }));
    }
  } else {
    entries = [
      ...liveVisible.map((entry) => ({ entry })),
      ...lingeringEntries.map((item) => ({ entry: item.entry, doneAt: item.doneAt })),
    ];
    if (selectedEntry && !entries.some((item) => item.entry.id === selectedEntry.id)) {
      entries.push({ entry: selectedEntry });
    }
  }
  for (const entry of staleEntries) {
    if (!entries.some((item) => item.entry.id === entry.id)) entries.push({ entry });
  }
  entries.sort((left, right) => oldestFirst(left.entry, right.entry));
  const tracks = entries.map((item, index) => trackOf(item.entry, nowDate, {
    index,
    doneAt: item.doneAt,
  }));
  const selectedTrack = tracks.find((track) => track.id === activeSelection) || null;

  // Lingering terminal requests must still have nodes to draw their lane.
  const stageEntries = [...filtered];
  const stageIds = new Set(filtered.map((entry) => entry.id));
  for (const item of lingeringEntries) {
    if (stageIds.has(item.entry.id)) continue;
    stageIds.add(item.entry.id);
    stageEntries.push(item.entry);
  }
  for (const entry of staleEntries) {
    if (stageIds.has(entry.id)) continue;
    stageIds.add(entry.id);
    stageEntries.push(entry);
  }
  if (selectedEntry && !stageIds.has(selectedEntry.id)) {
    stageIds.add(selectedEntry.id);
    stageEntries.push(selectedEntry);
  }

  const trackClientIds = tracks.map((track) => track.clientId);
  const trackKeyIds = tracks.map((track) => track.keyId);
  const trackRowIds = tracks.map((track) => track.rowId).filter(Boolean);
  const clientCap = capped(
    clientsFrom(stageEntries),
    CLIENT_CAP,
    [selectedEntry ? selectedEntry.client.id : "", ...trackClientIds],
  );
  const clients = clientCap.shown;
  const visibleClientIds = new Set(clients.map((client) => client.id));
  const staged = stageEntries.filter((entry) => visibleClientIds.has(entry.client.id));
  const keyCap = capped(
    keysFrom(staged),
    KEY_CAP,
    [selectedEntry ? keyIdOf(selectedEntry) : "", ...trackKeyIds],
  );
  const keys = keyCap.shown.map((key) => ({ ...key, nodeId: `key:${key.id}` }));
  const visibleKeyIds = new Set(keys.map((key) => key.id));
  const channelCap = capChannels(
    channelRows(staged, activeSelection),
    [selectedEntry ? rowsOf(selectedEntry).map((row) => row.rowId) : [], ...trackRowIds].flat(),
  );
  const channels = channelCap.shown.map((row) => ({
    ...row,
    tracks: tracks.filter((track) => track.rowId === row.rowId).map((track) => ({
      id: track.id,
      index: track.trackIndex,
      hue: track.hue,
      live: track.live,
      failed: track.failed,
    })),
  }));

  const faint = [];
  const pairs = new Set();
  for (const entry of staged) {
    const key = keyIdOf(entry);
    if (!visibleKeyIds.has(key)) continue;
    const token = `${entry.client.id}|${key}`;
    if (pairs.has(token)) continue;
    pairs.add(token);
    faint.push({ from: `client:${entry.client.id}`, to: `key:${key}`, kind: "client-key" });
  }
  for (const row of channels) {
    faint.push({ from: "hub", to: row.nodeId, kind: "hub-channel" });
  }

  const focus = selectedTrack ? {
    ...selectedTrack,
    trackTotal: tracks.length,
  } : null;
  const summary = countsFor(windowed, filtered, totals, filter, limit, historyCount);
  const log = [...filtered].sort(newestFirst).map((entry) => {
    const rows = rowsOf(entry);
    const latest = rows.length ? rows[rows.length - 1] : null;
    const failed = entry.status === "failed" || entry.status === "canceled";
    return {
      id: entry.id,
      createdAt: entry.createdAt,
      client: entry.client,
      hue: failed ? "#b42318" : requestHue(entry.id),
      apiKeyName: entry.apiKey ? entry.apiKey.name : "",
      modelID: entry.modelID,
      status: entry.status,
      channelName: latest ? latest.name : "",
      latencyMs: numberOrNull(entry.metricsLatencyMs),
    };
  });
  const loadedLive = liveSorted.length;
  const knownLiveTotal = Number.isInteger(inFlightTotal) && inFlightTotal >= loadedLive ? inFlightTotal : null;
  const trackTotal = knownLiveTotal == null ? loadedLive : knownLiveTotal;
  const staleCount = tracks.filter((track) => track.stale).length;
  return {
    focusId: activeSelection,
    focus,
    tracks,
    trackTotal,
    trackOverflow: Math.max(0, trackTotal - liveVisible.length),
    inFlightLoaded: loadedLive,
    inFlightTotal: knownLiveTotal,
    inFlightMore: knownLiveTotal == null && Boolean(inFlightMore),
    inFlightTruncated: knownLiveTotal != null && knownLiveTotal > loadedLive,
    staleCount,
    staticOverview: !liveVisible.length
      && !tracks.some((track) => track.lingering)
      && !staleCount
      && tracks.length > 0,
    clients: clients.map((client) => ({ ...client, nodeId: `client:${client.id}` })),
    keys,
    channels,
    nodeOverflow: { clients: clientCap.overflow, keys: keyCap.overflow, channels: channelCap.overflow },
    counts: summary.counts,
    note: summary.note,
    scope: summary.scope,
    windowTotal: summary.windowTotal,
    pageCount: summary.pageCount,
    shown: filtered.length,
    log,
    wires: { faint },
    availableClients,
    windowCount: windowed.length,
  };
}

function countsFor(windowed, filtered, totals, filter, limit, historyCount) {
  const page = windowed.length;
  const historyPage = Number.isInteger(historyCount) && historyCount >= 0 ? historyCount : page;
  const listPage = historyPage || page;
  const sample = {
    requests: filtered.length,
    inFlight: filtered.filter((entry) => inFlightStatus(entry.status)).length,
    failed: filtered.filter((entry) => entry.status === "failed").length,
  };
  const knownTotal = isTotal(totals);
  const truncated = knownTotal
    ? totals.requests > page
    : Number.isFinite(limit) && limit > 0 && page >= limit;
  const aggregated = knownTotal && filter === "all";
  return {
    counts: aggregated
      ? { requests: totals.requests, inFlight: totals.inFlight, failed: totals.failed }
      : sample,
    note: truncated && listPage > 0 ? { kind: aggregated ? "list" : "sample", count: listPage } : null,
    scope: aggregated ? "window" : (filter === "all" ? "list" : "sample"),
    windowTotal: knownTotal ? totals.requests : null,
    pageCount: listPage,
  };
}

function isTotal(totals) {
  return Boolean(totals) && isCount(totals.requests) && isCount(totals.inFlight) && isCount(totals.failed);
}

function isCount(value) {
  return Number.isInteger(value) && value >= 0;
}

function clientsFrom(entries) {
  const groups = new Map();
  for (const entry of entries) {
    let group = groups.get(entry.client.id);
    if (!group) {
      group = {
        id: entry.client.id,
        name: entry.client.name,
        hue: entry.client.hue,
        latestAt: -1,
        latestModel: "",
        requestCount: 0,
      };
      groups.set(entry.client.id, group);
    }
    group.requestCount += 1;
    if (entry.time >= group.latestAt) {
      group.latestAt = entry.time;
      group.latestModel = entry.modelID;
    }
  }
  return [...groups.values()];
}

function keysFrom(entries) {
  const groups = new Map();
  for (const entry of entries) {
    const id = keyIdOf(entry);
    let group = groups.get(id);
    if (!group) {
      group = {
        id,
        name: entry.apiKey ? entry.apiKey.name : "",
        status: entry.apiKey ? entry.apiKey.status : "",
        type: entry.apiKey ? entry.apiKey.type : "",
        latestAt: -1,
      };
      groups.set(id, group);
    }
    if (entry.time >= group.latestAt) {
      group.latestAt = entry.time;
      if (entry.apiKey) {
        group.name = entry.apiKey.name || "";
        group.status = entry.apiKey.status || "";
        group.type = entry.apiKey.type || "";
      }
    }
  }
  return [...groups.values()];
}

// A channel and key can serve several requests at once. The row shows the
// focused request's own attempt there when it has one, so the dot waiting at
// a row never sits beside "answering another request".
function channelRows(entries, focusId) {
  const groups = new Map();
  for (const entry of entries) {
    for (const row of rowsOf(entry)) {
      const current = groups.get(row.rowId);
      const mine = row.requestId === focusId;
      const theirs = Boolean(current) && current.requestId === focusId;
      if (!current || (mine && !theirs) || (mine === theirs && row.time >= current.time)) groups.set(row.rowId, row);
    }
  }
  return [...groups.values()].map((row) => {
    const kind = kindFor(row.status, row.requestId, focusId);
    const failed = kind === "failed";
    return {
      rowId: row.rowId,
      nodeId: `channel:${row.rowId}`,
      channelId: row.channelId,
      name: row.name,
      suffix: row.suffix,
      modelID: row.modelID,
      kind,
      responseStatusCode: failed ? row.code : null,
      errorMessage: failed ? truncateError(row.error) : "",
      latencyMs: kind === "completed" ? (row.latencyMs != null ? row.latencyMs : row.requestLatencyMs) : null,
      requestId: row.requestId,
      activeAt: row.time,
    };
  });
}

function capChannels(rows, keepRowIds) {
  return capped(rows, CHANNEL_CAP, keepRowIds);
}
