// Pure query construction and response shaping. No DOM, bridge, or clock access.

export const LIST_FIELDS = `id createdAt updatedAt status modelID stream source userAgent metricsLatencyMs metricsFirstTokenLatencyMs apiKey { id name status type }`;

export const WINDOW_OPTIONS = [
  { id: "5m", ms: 5 * 60 * 1000 },
  { id: "10m", ms: 10 * 60 * 1000 },
  { id: "15m", ms: 15 * 60 * 1000 },
  { id: "30m", ms: 30 * 60 * 1000 },
  { id: "1h", ms: 60 * 60 * 1000 },
  { id: "2h", ms: 2 * 60 * 60 * 1000 },
  { id: "today", ms: null },
];

export const DEFAULT_WINDOW = "15m";
export const LIMITS = [25, 50, 100, 200];
export const DEFAULT_LIMIT = 50;
export const REFRESH_SECONDS = [2, 5, 10, 20, 30, 60];
export const DEFAULT_REFRESH = 2;
export const STATS_MIN_MS = 10 * 1000;
// In-flight work is queried on its own so a full history page cannot hide a
// long request that started before the window.
export const INFLIGHT_LIMIT = 200;
// In-flight rows are cursor-paged with a hard budget. Above it the page says
// how many rows it loaded of how many exist instead of silently dropping them.
export const INFLIGHT_BUDGET = 1000;
// A request that vanished but cannot be confirmed either way stays visible as
// stale for this long, then leaves the active set.
export const STALE_TTL_MS = 60 * 1000;
// One overview lane per concurrent request. The page keeps at most this many
// live lanes and reports the rest as an explicit +N.
export const TRACK_CAP = 10;

export function isInFlightStatus(status) {
  return status === "pending" || status === "processing";
}

export function windowMsFor(id) {
  const option = WINDOW_OPTIONS.find((entry) => entry.id === id);
  if (!option) return WINDOW_OPTIONS.find((entry) => entry.id === DEFAULT_WINDOW).ms;
  return option.ms;
}

export function clampWindow(value) {
  return WINDOW_OPTIONS.some((entry) => entry.id === value) ? value : DEFAULT_WINDOW;
}

export function sinceFor(id, now) {
  const date = now instanceof Date ? now : new Date(now);
  if (id === "today") {
    const start = new Date(date.getTime());
    start.setHours(0, 0, 0, 0);
    return start;
  }
  return new Date(date.getTime() - windowMsFor(id));
}

export function clampLimit(value) {
  const number = Number(value);
  return LIMITS.includes(number) ? number : DEFAULT_LIMIT;
}

export function clampRefresh(value) {
  const number = Number(value);
  return REFRESH_SECONDS.includes(number) ? number : DEFAULT_REFRESH;
}

// Stats are allowed to lag: refresh at most once per STATS_MIN_MS.
export function statsDue(lastStatsAt, now) {
  const current = now instanceof Date ? now.getTime() : Number(now);
  const last = Number(lastStatsAt);
  if (!Number.isFinite(last) || last <= 0) return true;
  if (!Number.isFinite(current)) return true;
  return current - last >= STATS_MIN_MS;
}

export function gqlString(value) {
  return `"${String(value).replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n").replace(/\r/g, "\\r")}"`;
}

function channelField(withChannel) {
  // channelID stays in every selection so a failed name lookup never loses the
  // channel identity.
  return withChannel ? "channel { id name } channelID" : "channelID";
}

// History is the page of rows created inside the selected window. In-flight
// rows are fetched separately so this limit never drops them.
export function historyQuery(withChannel) {
  return `query LiveHistory($since: Time!, $first: Int!) {
    requests(
      first: $first
      orderBy: { field: CREATED_AT, direction: DESC }
      where: { createdAtGTE: $since }
    ) {
      edges { node { ${LIST_FIELDS} ${channelField(withChannel)} } }
    }
  }`;
}

export function inflightQuery(withChannel) {
  return `query LiveInFlight($first: Int!, $after: Cursor) {
    requests(
      first: $first
      after: $after
      orderBy: { field: CREATED_AT, direction: DESC }
      where: { statusIn: [pending, processing] }
    ) {
      pageInfo { hasNextPage endCursor }
      edges { node { ${LIST_FIELDS} ${channelField(withChannel)} } }
    }
  }`;
}

// Terminal state for requests that were visible last cycle and are gone now.
// Only a bounded set of request ids is ever asked for.
export function requestStateQuery(ids, withChannel) {
  const fields = ids.map((id, index) => (
    `r${index}: node(id: ${gqlString(id)}) { ... on Request { ${LIST_FIELDS} ${channelField(withChannel)} } }`
  ));
  return `query LiveRequestState { ${fields.join(" ")} }`;
}

// Counts are their own request. AxonHub rejects first: 0, so every count uses
// a valid first: 1 page and reads only totalCount.
export function statsQuery() {
  return `query LiveStats($since: Time!) {
    window: requests(first: 1, where: { createdAtGTE: $since }) { totalCount }
    inFlight: requests(first: 1, where: { statusIn: [pending, processing] }) { totalCount }
    failed: requests(first: 1, where: { and: [ { status: failed }, { createdAtGTE: $since } ] }) { totalCount }
  }`;
}

// A request can retry more than eight times. Fetch the newest eight attempts,
// so the last one is the attempt that actually served the request, and keep
// totalCount to label the attempt number correctly.
export function executionsQuery(ids, withChannel) {
  const selection = `id createdAt status modelID responseStatusCode errorMessage metricsLatencyMs metricsFirstTokenLatencyMs channelAPIKeyIndex channelAPIKeySuffix ${channelField(withChannel)}`;
  const fields = ids.map((id, index) => (
    `r${index}: node(id: ${gqlString(id)}) { ... on Request { id executions(first: 8, orderBy: { field: CREATED_AT, direction: DESC }) { totalCount edges { node { ${selection} } } } } }`
  ));
  return `query LiveExecutions { ${fields.join(" ")} }`;
}

export function countOf(connection) {
  const value = connection && connection.totalCount;
  return Number.isInteger(value) && value >= 0 ? value : null;
}

export function readTotals(data) {
  const requests = countOf(data && data.window);
  const inFlight = countOf(data && data.inFlight);
  const failed = countOf(data && data.failed);
  if (requests == null || inFlight == null || failed == null) return null;
  return { requests, inFlight, failed };
}

export function nodesOf(connection) {
  const edges = connection && Array.isArray(connection.edges) ? connection.edges : [];
  const nodes = [];
  for (const edge of edges) {
    if (edge && edge.node) nodes.push(edge.node);
  }
  return nodes;
}

export function connectionOf(response) {
  return response && response.data && response.data.requests ? response.data.requests : null;
}

export function errorsOf(response) {
  return Array.isArray(response && response.errors) ? response.errors : [];
}

export function isForbiddenError(error) {
  if (!error) return false;
  const code = String((error.extensions && error.extensions.code) || "").toUpperCase();
  const message = String(error.message || "").toLowerCase();
  return code === "FORBIDDEN" || message.includes("forbidden") || message.includes("permission");
}

// A general FORBIDDEN may be caused by asking for channel objects. The caller is
// allowed one retry that selects channelID only.
export function hasForbidden(errors) {
  return Array.isArray(errors) && errors.some(isForbiddenError);
}

export function shouldRetryWithoutChannel(withChannel, errors) {
  return Boolean(withChannel) && hasForbidden(errors);
}

// Retry only the side whose own query hit the channel-object FORBIDDEN.
export function channelRetryPlan(withChannel, historyErrors, inflightErrors) {
  if (!withChannel) return { history: false, inflight: false };
  return {
    history: hasForbidden(historyErrors),
    inflight: hasForbidden(inflightErrors),
  };
}

export function transportErrors(outcome) {
  if (!outcome || outcome.ok) return [];
  return [{ message: (outcome.error && outcome.error.message) || "Request failed." }];
}

export function channelRetrySucceeded(historyReady, inflightReady, errors) {
  return Boolean(historyReady) && Boolean(inflightReady) && Array.isArray(errors) && errors.length === 0;
}

export function statsVisible(statsLoaded, shown) {
  return Boolean(statsLoaded) || (Number.isFinite(shown) && shown > 0);
}

export function pageInfoOf(connection) {
  const info = connection && connection.pageInfo;
  return {
    hasNext: Boolean(info && info.hasNextPage),
    endCursor: info && typeof info.endCursor === "string" && info.endCursor ? info.endCursor : null,
  };
}

// Returns the cursor for another in-flight page, or null when the data is done
// or the hard budget is reached.
export function nextInflightCursor(connection, loaded, budget = INFLIGHT_BUDGET) {
  if (!Number.isFinite(loaded) || !Number.isFinite(budget) || loaded >= budget) return null;
  const info = pageInfoOf(connection);
  if (!info.hasNext || !info.endCursor) return null;
  return info.endCursor;
}

// Requests that were visible and live last cycle but are absent now are
// candidates for a bounded terminal-state lookup.
export function retiredLiveIds(previous, current, visibleIds, selectedId) {
  const currentIds = new Set((current || []).map((item) => item && item.id).filter(Boolean));
  const previousList = previous instanceof Map ? [...previous.values()] : (previous || []);
  const previousById = new Map(previousList.filter(Boolean).map((item) => [item.id, item]));
  const wanted = [];
  const seen = new Set();
  for (const id of [...(visibleIds || []), selectedId]) {
    if (!id || seen.has(id)) continue;
    seen.add(id);
    const before = previousById.get(id);
    if (before && isInFlightStatus(before.status) && !currentIds.has(id)) wanted.push(id);
  }
  return wanted.slice(0, TRACK_CAP + 1);
}

export function statsAccept(token, currentToken, version, currentVersion) {
  return token === currentToken && version === currentVersion;
}

export function statsWindowMatches(statsMode, mode) {
  return Boolean(statsMode) && statsMode === mode;
}

// The stats request owns its window; list fetch state must not gate it.
export function statsTotalsForWindow(statsMode, mode, totals, loaded) {
  if (!statsWindowMatches(statsMode, mode) || !totals || !loaded) return null;
  return totals;
}

export function mapStateNodes(data, ids) {
  const requests = [];
  ids.forEach((id, index) => {
    const node = data && data[`r${index}`];
    const mapped = mapRequest(node);
    if (mapped) requests.push(mapped);
  });
  return requests;
}

// Distinguishes an explicit null node (the request no longer exists) from a
// missing/unresolvable response (we do not know yet).
export function readStateNodes(data, ids) {
  const rows = new Map();
  ids.forEach((id, index) => {
    const key = `r${index}`;
    const resolved = Boolean(data) && Object.prototype.hasOwnProperty.call(data, key);
    rows.set(id, {
      resolved,
      request: resolved ? mapRequest(data[key]) : null,
    });
  });
  return rows;
}

// Backfill policy: only an explicit live answer keeps a request active. An
// explicit missing node exits tracking; unknown results go stale and expire.
export function backfillDecision(now, staleSince, request, resolved, ttl = STALE_TTL_MS) {
  if (request) return isInFlightStatus(request.status) ? "active" : "terminal";
  if (resolved) return "gone";
  const since = Number.isFinite(staleSince) ? staleSince : now;
  if (Number.isFinite(now) && Number.isFinite(ttl) && now - since >= ttl) return "expire";
  return "stale";
}

export function channelRef(node) {
  if (node && node.channel && node.channel.id) {
    return { id: node.channel.id, name: node.channel.name || node.channel.id };
  }
  if (node && node.channelID) return { id: node.channelID, name: node.channelID };
  return null;
}

export function mapRequest(node) {
  if (!node || !node.id) return null;
  return {
    id: node.id,
    createdAt: node.createdAt,
    status: String(node.status || "").toLowerCase(),
    modelID: node.modelID || "",
    stream: Boolean(node.stream),
    source: node.source || "",
    userAgent: node.userAgent || "",
    metricsLatencyMs: node.metricsLatencyMs,
    metricsFirstTokenLatencyMs: node.metricsFirstTokenLatencyMs,
    apiKey: node.apiKey && node.apiKey.id ? {
      id: node.apiKey.id,
      name: node.apiKey.name || "",
      status: String(node.apiKey.status || "").toLowerCase(),
      type: node.apiKey.type || "",
    } : null,
    channel: channelRef(node),
  };
}

export function mapExecution(node) {
  return {
    id: node.id,
    createdAt: node.createdAt,
    status: String(node.status || "").toLowerCase(),
    modelID: node.modelID || "",
    responseStatusCode: node.responseStatusCode,
    errorMessage: node.errorMessage || "",
    metricsLatencyMs: node.metricsLatencyMs,
    metricsFirstTokenLatencyMs: node.metricsFirstTokenLatencyMs,
    channelAPIKeyIndex: node.channelAPIKeyIndex,
    channelAPIKeySuffix: node.channelAPIKeySuffix || "",
    channel: channelRef(node),
  };
}

export function attachExecutions(requests, ids, data) {
  const byId = new Map(requests.map((item) => [item.id, { ...item }]));
  ids.forEach((id, index) => {
    const node = data[`r${index}`];
    const current = byId.get(id);
    if (!current || !node) return;
    current.executions = nodesOf(node.executions).map(mapExecution);
    const total = node.executions && node.executions.totalCount;
    current.executionTotalCount = Number.isInteger(total) && total >= 0 ? total : null;
  });
  return [...byId.values()];
}

// In-flight rows win over the history copy of the same request id.
export function mergeRequests(inflight, history) {
  const byId = new Map();
  for (const item of Array.isArray(inflight) ? inflight : []) {
    if (item && item.id) byId.set(item.id, item);
  }
  for (const item of Array.isArray(history) ? history : []) {
    if (item && item.id && !byId.has(item.id)) byId.set(item.id, item);
  }
  return [...byId.values()];
}

function newestFirst(left, right) {
  const lt = new Date(left.createdAt).getTime();
  const rt = new Date(right.createdAt).getTime();
  const delta = (Number.isFinite(rt) ? rt : 0) - (Number.isFinite(lt) ? lt : 0);
  if (delta) return delta;
  return String(right.id || "").localeCompare(String(left.id || ""));
}

// Execution lookups mirror the visible lanes: the newest TRACK_CAP in-flight
// requests, plus the selected request if the user is looking at it.
export function executionIds(requests, selectedId) {
  const seen = new Set();
  const inflight = [];
  for (const item of [...(requests || [])].sort(newestFirst)) {
    if (!isInFlightStatus(item.status)) continue;
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    inflight.push(item.id);
    if (inflight.length >= TRACK_CAP) break;
  }
  const ids = inflight;
  const selected = selectedId ? (requests || []).find((item) => item.id === selectedId) : null;
  if (selected && !ids.includes(selectedId)) {
    const selectedLive = isInFlightStatus(selected.status);
    if (selectedLive) {
      if (ids.length < TRACK_CAP) ids.push(selectedId);
      else ids[ids.length - 1] = selectedId;
    } else {
      // A completed selection is an extra detail lookup, not a live lane.
      ids.push(selectedId);
    }
  }
  return ids.filter(Boolean);
}

export function channelSelectionForbidden(errors) {
  if (!Array.isArray(errors)) return false;
  return errors.some((error) => {
    const message = String((error && error.message) || "").toLowerCase();
    const path = Array.isArray(error && error.path) ? error.path.map((part) => String(part).toLowerCase()) : [];
    const aboutChannel = path.includes("channel") || message.includes("channel") || message.includes("read_channels");
    const code = error && error.extensions && error.extensions.code;
    const forbidden = code === "FORBIDDEN" || message.includes("forbidden") || message.includes("permission") || message.includes("read_channels");
    return aboutChannel && forbidden;
  });
}

// try/catch as a value, so one failed query never rejects the whole cycle.
export async function settle(task) {
  try {
    return { ok: true, value: await task() };
  } catch (error) {
    return { ok: false, error };
  }
}

// A slower response from an older cycle must not overwrite a newer one.
export function commitIfCurrent(generation, current, apply) {
  if (generation !== current) return false;
  apply();
  return true;
}
