import {
  buildStage,
  channelStatusText,
  clientLabel,
  formatLatency,
  formatTime,
  isChinese,
  isLiveStatus,
  laneOffsetFor,
  LINGER_MS,
  statusLine,
  toggleSelection,
  visibleExecutionIds,
} from "./format.js";
import { iconSvg } from "./icons.js";
import {
  DEFAULT_LIMIT,
  DEFAULT_REFRESH,
  DEFAULT_WINDOW,
  INFLIGHT_BUDGET,
  INFLIGHT_LIMIT,
  LIMITS,
  REFRESH_SECONDS,
  TRACK_CAP,
  WINDOW_OPTIONS,
  attachExecutions,
  backfillDecision,
  channelRetryPlan,
  channelSelectionForbidden,
  clampLimit,
  clampRefresh,
  clampWindow,
  connectionOf,
  errorsOf,
  executionsQuery,
  hasForbidden,
  historyQuery,
  inflightQuery,
  isForbiddenError,
  mapRequest,
  mergeRequests,
  nextInflightCursor,
  nodesOf,
  pageInfoOf,
  readStateNodes,
  readTotals,
  requestStateQuery,
  retiredLiveIds,
  settle,
  sinceFor,
  statsAccept,
  statsDue,
  statsQuery,
  statsVisible,
  statsWindowMatches,
  statsTotalsForWindow,
  transportErrors,
  windowMsFor,
} from "./query.js";

const SVG_NS = "http://www.w3.org/2000/svg";
const KEY_ICON = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="8.5" cy="9" r="3.25" stroke="currentColor" stroke-width="1.5"/><path d="M11.2 11.2 19 19M15.5 16.2h3.2V19" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
const HUB_ICON = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="12" cy="12" r="3" fill="currentColor"/><path d="M12 3v4M12 17v4M3 12h4M17 12h4" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>`;

const COPY = {
  zh: {
    title: "实时请求",
    filter: "客户端",
    allClients: "全部客户端",
    window: "时间",
    windows: {
      "5m": "近 5 分钟",
      "10m": "近 10 分钟",
      "15m": "近 15 分钟",
      "30m": "近 30 分钟",
      "1h": "近 1 小时",
      "2h": "近 2 小时",
      today: "今天",
    },
    loading: "正在读取…",
    updated: (time) => `更新于 ${time}`,
    emptyWindow: (label) => (label === "今天" ? "今天还没有请求。" : `${label}里还没有请求。`),
    emptyFilter: "这个客户端在当前窗口里没有请求。",
    requests: "窗口共",
    listLatest: "列表最新",
    inFlightNow: "当前进行中",
    failedWindow: "窗口失败",
    limit: "条数",
    limitOption: (count) => `${count} 条`,
    refresh: "刷新",
    refreshOption: (seconds) => `${seconds} 秒`,
    statsWindow: (total, count) => `窗口共 ${total} 条 · 列表最新 ${count} 条`,
    statsWindowComplete: (total) => `窗口共 ${total} 条`,
    statsList: (count) => `列表最新 ${count} 条（窗口总数未取得）`,
    statsSample: (count) => `客户端筛选后的样本统计（历史最新 ${count} 条 + 当前进行中）`,
    statsHint: "窗口统计最多每 10 秒刷新一次；列表、进行中和筛选样本按所选刷新间隔更新。",
    statsUnavailable: "窗口统计暂时不可用，下面是列表样本。",
    emptyFilterSample: (count) => `历史最新 ${count} 条和进行中请求里没有这个客户端的请求。`,
    noKey: "无密钥",
    unassigned: "未关联",
    time: "时间",
    client: "客户端",
    apiKey: "API 密钥",
    model: "模型",
    status: "状态",
    channel: "渠道",
    latency: "耗时",
    colClient: "客户端",
    colKey: "API 密钥",
    colChannel: "渠道",
    requestsN: (count) => `${count} 个请求`,
    statusPending: "等待中",
    statusProcessing: "处理中",
    statusCompleted: "已完成",
    statusFailed: "失败",
    statusCanceled: "已取消",
    statusStale: "状态未知",
    keyEnabled: "启用",
    keyDisabled: "停用",
    keyArchived: "已归档",
    channelHint: "未授予 read_channels，渠道只显示 ID。",
    logTitle: "最近请求",
    pinHint: "点击轨道或列表查看详情；Esc 回到实时",
    pinnedHint: "已选中。再次点击、Esc 或「回到实时」取消",
    tagPinned: "已选中",
    trackTag: (index) => `已选中 · 轨道 ${index}`,
    tagLive: "进行中",
    tagLatest: "最新",
    backToLive: "回到实时",
    tracksLabel: "请求轨道",
    staticOverview: "当前没有进行中的请求，显示最近请求的静态概览。",
    inFlightLimited: (loaded, total) => `进行中请求已加载 ${loaded} / 共 ${total} 条（达到查询上限）。`,
    inFlightMore: (loaded) => `已加载 ${loaded} 条进行中请求，仍有更多（精确总数尚未取得）。`,
    trackOverflow: (count) => `还有 ${count} 条进行中请求没有显示为轨道；点选列表中的请求会自动把它拉回可见范围。`,
    trackOverflowLower: (count) => `至少还有 ${count} 条进行中请求没有显示为轨道；点选列表中的请求会自动把它拉回可见范围。`,
    moreNodes: (count) => `还有 ${count} 个未显示`,
    trackLabel: (index) => `轨道 ${index}`,
    noChannel: "无渠道",
    hubIdle: "空闲",
    hubBusy: (count) => `${count} 进行中`,
    requestFailed: (message) => (message ? `请求失败：${message}` : "请求失败。"),
    loadFailed: "请求列表没有加载。",
    bridgeMissing: "AxonHub bridge 没有加载。",
  },
  en: {
    title: "Live requests",
    filter: "Client",
    allClients: "All clients",
    window: "Window",
    windows: {
      "5m": "Last 5 min",
      "10m": "Last 10 min",
      "15m": "Last 15 min",
      "30m": "Last 30 min",
      "1h": "Last 1 hour",
      "2h": "Last 2 hours",
      today: "Today",
    },
    loading: "Loading…",
    updated: (time) => `Updated ${time}`,
    emptyWindow: (label) => {
      const value = label.toLowerCase();
      return value === "today" ? "No requests yet today." : `No requests in the ${value}.`;
    },
    emptyFilter: "No requests for this client in the current window.",
    requests: "In window",
    listLatest: "List latest",
    inFlightNow: "In flight now",
    failedWindow: "Failed in window",
    limit: "Rows",
    limitOption: (count) => `${count} rows`,
    refresh: "Refresh",
    refreshOption: (seconds) => `${seconds}s`,
    statsWindow: (total, count) => `${total} in window · list shows the latest ${count}`,
    statsWindowComplete: (total) => `${total} in window`,
    statsList: (count) => `List shows the latest ${count} (window total unavailable)`,
    statsSample: (count) => `Client-filtered sample (latest ${count} history rows + in-flight)`,
    statsHint: "Window counts refresh at most every 10 seconds; the list, in-flight rows, and filtered sample follow the selected refresh interval.",
    statsUnavailable: "Window counts are temporarily unavailable; the sample below comes from the list.",
    emptyFilterSample: (count) => `No requests for this client in the latest ${count} history rows or in-flight work.`,
    noKey: "No API key",
    unassigned: "Unassigned",
    time: "Time",
    client: "Client",
    apiKey: "API key",
    model: "Model",
    status: "Status",
    channel: "Channel",
    latency: "Latency",
    colClient: "Client",
    colKey: "API key",
    colChannel: "Channels",
    requestsN: (count) => `${count} requests`,
    statusPending: "Waiting",
    statusProcessing: "Processing",
    statusCompleted: "Completed",
    statusFailed: "Failed",
    statusCanceled: "Canceled",
    statusStale: "Unknown",
    keyEnabled: "Enabled",
    keyDisabled: "Disabled",
    keyArchived: "Archived",
    channelHint: "read_channels is not granted, so channel ids are shown.",
    logTitle: "Recent requests",
    pinHint: "Click a track or a row to inspect it; press Esc to return to live",
    pinnedHint: "Selected. Click again, press Esc, or choose Back to live to clear.",
    tagPinned: "Selected",
    trackTag: (index) => `Selected · track ${index}`,
    tagLive: "Live",
    tagLatest: "Latest",
    backToLive: "Back to live",
    tracksLabel: "Request tracks",
    staticOverview: "No requests in flight. Showing a static overview of recent requests.",
    inFlightLimited: (loaded, total) => `Loaded ${loaded} of ${total} in-flight requests (query budget reached).`,
    inFlightMore: (loaded) => `Loaded ${loaded} in-flight requests; more remain and the exact total is not available yet.`,
    trackOverflow: (count) => `${count} more in-flight requests are not shown as tracks; picking one in the list pulls it back into view.`,
    trackOverflowLower: (count) => `At least ${count} more in-flight requests are not shown as tracks; picking one in the list pulls it back into view.`,
    moreNodes: (count) => `${count} more not shown`,
    trackLabel: (index) => `Track ${index}`,
    noChannel: "No channel",
    hubIdle: "Idle",
    hubBusy: (count) => `${count} in flight`,
    requestFailed: (message) => (message ? `Request failed: ${message}` : "Request failed."),
    loadFailed: "The request list could not be loaded.",
    bridgeMissing: "AxonHub bridge did not load.",
  },
};

const titleEl = document.querySelector("#title");
const filterLabel = document.querySelector("#filter-label");
const filterEl = document.querySelector("#client-filter");
const windowLabel = document.querySelector("#window-label");
const windowEl = document.querySelector("#window");
const limitLabel = document.querySelector("#limit-label");
const limitEl = document.querySelector("#limit");
const refreshLabel = document.querySelector("#refresh-label");
const refreshEl = document.querySelector("#refresh");
const updatedEl = document.querySelector("#updated");
const hintEl = document.querySelector("#hint");
const statusEl = document.querySelector("#status");
const stageScroll = document.querySelector("#stage-scroll");
const stage = document.querySelector("#stage");
const wires = document.querySelector("#wires");
const tickEl = document.querySelector("#tick");
const summaryEl = document.querySelector("#summary");
const focusBar = document.querySelector("#focus-bar");
const focusTag = document.querySelector("#focus-tag");
const clearButton = document.querySelector("#clear-selection");
const overviewNote = document.querySelector("#overview-note");
const statsBlock = document.querySelector("#stats-block");
const sampleNoteEl = document.querySelector("#sample-note");
const statsHintEl = document.querySelector("#stats-hint");
const countsEl = document.querySelector("#counts");
const logPanel = document.querySelector("#log-panel");
const logTitle = document.querySelector("#log-title");
const logHint = document.querySelector("#log-hint");
const logWrap = document.querySelector("#log-wrap");
const emptyEl = document.querySelector("#empty");
const emptyText = document.querySelector("#empty-text");
const colClients = document.querySelector("#col-clients");
const colKeys = document.querySelector("#col-keys");
const colHub = document.querySelector("#col-hub");
const colChannels = document.querySelector("#col-channels");

let locale = document.documentElement.lang || "en";
let text = copy(locale);
let mode = DEFAULT_WINDOW;
let limit = DEFAULT_LIMIT;
let refreshSeconds = DEFAULT_REFRESH;
let fetchedLimit = DEFAULT_LIMIT;
let clientFilter = "all";
let selectedId = "";
let namesDenied = false;
let latestRequests = [];
let latestHistoryCount = 0;
let latestInFlightMore = false;
let latestTotals = null;
let statsAt = 0;
let statsLoaded = false;
let statsError = false;
let statsMode = "";
let statsToken = 0;
let statsVersion = 0;
let lastVisibleTrackIds = [];
let prevRequests = new Map();
let fetchedAt = null;
let hasInFlight = false;
let loadedOnce = false;
let timer = 0;
let fetching = false;
let queued = false;
let generation = 0;
let filterKey = "";
let barFocus = "";
let seenRows = null;
const flashes = new Map();
const lingering = new Map();
const staleTracking = new Map();
let lingerTimer = 0;
let stopTheme = () => {};

applyStaticCopy();
wireControls();

const bridge = window.axonhub;
if (!bridge || typeof bridge.ready !== "function") {
  statusEl.hidden = false;
  statusEl.textContent = text.bridgeMissing;
} else {
  const onPageHide = () => {
    stopTheme();
    clearTimeout(timer);
    clearTimeout(lingerTimer);
  };
  window.addEventListener("pagehide", onPageHide, { once: true });
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      clearTimeout(timer);
      timer = 0;
      return;
    }
    poll();
  });
  window.addEventListener("resize", () => {
    if (loadedOnce) render();
  });
  start().catch(showError);
}

function copy(nextLocale) {
  return isChinese(nextLocale) ? COPY.zh : COPY.en;
}

function applyStaticCopy() {
  text = copy(locale);
  filterKey = "";
  document.title = text.title;
  titleEl.textContent = text.title;
  logTitle.textContent = text.logTitle;
  filterLabel.textContent = text.filter;
  clearButton.textContent = text.backToLive;
  wires.setAttribute("aria-label", text.tracksLabel);
  fillWindow();
  fillLimit();
  fillRefresh();
  if (!fetchedAt) updatedEl.textContent = text.loading;
  if (!hintEl.hidden) hintEl.textContent = text.channelHint;
  statsHintEl.textContent = text.statsHint;
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme && theme.theme ? theme.theme : "light";
}

async function start() {
  const init = await bridge.ready();
  locale = init.locale || "en";
  document.documentElement.lang = locale;
  applyTheme(init);
  stopTheme = bridge.onTheme(applyTheme);
  applyStaticCopy();
  poll();
}

function wireControls() {
  filterEl.addEventListener("change", () => {
    clientFilter = filterEl.value || "all";
    lastVisibleTrackIds = [];
    if (loadedOnce) render();
    // Execution lookups follow the filtered lanes, so refresh them.
    poll();
  });
  windowEl.addEventListener("change", () => {
    const next = clampWindow(windowEl.value);
    windowEl.value = next;
    if (next === mode) return;
    mode = next;
    // Keep the global stats throttle: the new window waits for the remaining
    // interval instead of issuing a fresh stats query immediately.
    latestTotals = null;
    statsMode = "";
    statsLoaded = false;
    statsError = false;
    statsVersion += 1;
    latestHistoryCount = 0;
    lastVisibleTrackIds = [];
    if (loadedOnce) render();
    poll();
  });
  limitEl.addEventListener("change", () => {
    const next = clampLimit(limitEl.value);
    limitEl.value = String(next);
    if (next === limit) return;
    limit = next;
    poll();
  });
  refreshEl.addEventListener("change", () => {
    const next = clampRefresh(refreshEl.value);
    refreshEl.value = String(next);
    if (next === refreshSeconds) return;
    refreshSeconds = next;
    if (!fetching) poll();
  });
  logWrap.addEventListener("click", (event) => {
    const row = event.target.closest("tr[data-id]");
    if (!row) return;
    selectTrack(row.dataset.id);
  });
  logWrap.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    const row = event.target.closest("tr[data-id]");
    if (!row) return;
    event.preventDefault();
    row.click();
  });
  stage.addEventListener("click", (event) => {
    const hit = event.target.closest("[data-track-id]");
    if (!hit) return;
    selectTrack(hit.dataset.trackId);
  });
  stage.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    const hit = event.target.closest("[data-track-id]");
    if (!hit) return;
    event.preventDefault();
    selectTrack(hit.dataset.trackId);
  });
  colChannels.addEventListener("click", (event) => {
    const chip = event.target.closest("[data-track-id]");
    if (!chip) return;
    event.stopPropagation();
    selectTrack(chip.dataset.trackId);
  });
  clearButton.addEventListener("click", () => {
    if (!selectedId) return;
    selectedId = "";
    render();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || !selectedId) return;
    selectedId = "";
    render();
  });
}

function selectTrack(id) {
  selectedId = toggleSelection(selectedId, id);
  render();
}

function windowLabelFor(id) {
  return (text.windows && text.windows[id]) || id;
}

function fillWindow() {
  windowLabel.textContent = text.window;
  windowEl.setAttribute("aria-label", text.window);
  windowEl.replaceChildren();
  for (const option of WINDOW_OPTIONS) {
    const el = document.createElement("option");
    el.value = option.id;
    el.textContent = windowLabelFor(option.id);
    windowEl.append(el);
  }
  windowEl.value = mode;
}

function fillLimit() {
  limitLabel.textContent = text.limit;
  limitEl.setAttribute("aria-label", text.limit);
  limitEl.replaceChildren();
  for (const value of LIMITS) {
    const option = document.createElement("option");
    option.value = String(value);
    option.textContent = text.limitOption(value);
    limitEl.append(option);
  }
  limitEl.value = String(limit);
}

function fillRefresh() {
  refreshLabel.textContent = text.refresh;
  refreshEl.setAttribute("aria-label", text.refresh);
  refreshEl.replaceChildren();
  for (const seconds of REFRESH_SECONDS) {
    const option = document.createElement("option");
    option.value = String(seconds);
    option.textContent = text.refreshOption(seconds);
    refreshEl.append(option);
  }
  refreshEl.value = String(refreshSeconds);
}

function poll() {
  clearTimeout(timer);
  timer = 0;
  if (fetching) {
    queued = true;
    return;
  }
  fetching = true;
  const gen = ++generation;
  load(gen).catch((error) => {
    if (gen === generation) showError(error);
  }).finally(() => {
    fetching = false;
    if (queued) {
      queued = false;
      poll();
      return;
    }
    if (gen === generation) schedule();
  });
}

function schedule() {
  clearTimeout(timer);
  timer = 0;
  if (document.hidden) return;
  timer = setTimeout(poll, refreshSeconds * 1000);
}

async function load(gen) {
  const windowMode = mode;
  const now = new Date();
  const since = sinceFor(windowMode, now).toISOString();
  const first = clampLimit(limit);
  const previousRequests = prevRequests;
  maybeRefreshStats(since, now, windowMode);

  const withChannel = !namesDenied;
  let [historyOutcome, inflight] = await Promise.all([
    settle(() => bridge.graphql(historyQuery(withChannel), { since, first })),
    fetchInflight(withChannel, gen, windowMode),
  ]);
  if (gen !== generation || windowMode !== mode || inflight.aborted) return;

  let historyResponse = historyOutcome.ok ? historyOutcome.value : null;
  let listErrors = [
    ...errorsOf(historyResponse),
    ...transportErrors(historyOutcome),
    ...inflight.errors,
    ...(inflight.error ? [{ message: inflight.error.message }] : []),
  ];

  if (withChannel) {
    // Retry only the side whose own query hit the channel-object FORBIDDEN.
    // A failed retry never replaces an original successful response.
    const plan = channelRetryPlan(true, errorsOf(historyResponse), inflight.errors);
    let historyRetryOk = !plan.history;
    let inflightRetryOk = !plan.inflight;
    if (plan.history) {
      const retry = await settle(() => bridge.graphql(historyQuery(false), { since, first }));
      if (gen !== generation || windowMode !== mode) return;
      const retryResponse = retry.ok ? retry.value : null;
      const retryErrors = [...errorsOf(retryResponse), ...transportErrors(retry)];
      listErrors.push(...retryErrors);
      if (retry.ok && connectionOf(retryResponse) && retryErrors.length === 0) {
        historyRetryOk = true;
        historyOutcome = retry;
        historyResponse = retryResponse;
      } else {
        historyRetryOk = false;
      }
    }
    if (plan.inflight) {
      const retry = await fetchInflight(false, gen, windowMode);
      if (gen !== generation || windowMode !== mode || retry.aborted) return;
      listErrors.push(...retry.errors, ...(retry.error ? [{ message: retry.error.message }] : []));
      if (retry.ok && !retry.error && retry.errors.length === 0) {
        inflightRetryOk = true;
        inflight = retry;
      } else {
        inflightRetryOk = false;
      }
    }
    if ((plan.history || plan.inflight) && historyRetryOk && inflightRetryOk) {
      namesDenied = true;
      showHint(true);
    }
  }

  const inflightReady = inflight.ok;
  const historyReady = Boolean(connectionOf(historyResponse));
  if (!historyReady && !inflightReady) {
    showProblems(listErrors, true);
    return;
  }

  const inflightRequests = inflightReady ? inflight.nodes : [];
  const historyRequests = historyReady
    ? nodesOf(historyResponse.data.requests).map(mapRequest).filter(Boolean)
    : [];
  let requests = mergeRequests(inflightRequests, historyRequests);

  // A request that was visible and live last cycle but is absent now needs a
  // bounded terminal-state lookup. Only an explicit live answer keeps it
  // active; unknown answers go stale for a short TTL; an explicit null exits.
  const currentIds = new Set(requests.map((item) => item.id));
  for (const id of [...staleTracking.keys()]) {
    if (currentIds.has(id)) staleTracking.delete(id);
  }
  const candidates = new Set([
    ...staleTracking.keys(),
    ...retiredLiveIds(previousRequests, requests, lastVisibleTrackIds, selectedId),
  ]);
  const missing = [...candidates].filter((id) => !currentIds.has(id)).slice(0, TRACK_CAP + 1);
  if (missing.length) {
    const stateOutcome = await fetchTerminalStates(missing);
    if (gen !== generation || windowMode !== mode) return;
    if (stateOutcome.errors.length) listErrors = [...listErrors, ...stateOutcome.errors];
    const keep = [];
    for (const id of missing) {
      const row = stateOutcome.rows.get(id) || { resolved: false, request: null };
      const tracked = staleTracking.get(id);
      const since = tracked ? tracked.since : Date.now();
      const decision = backfillDecision(Date.now(), since, row.request, row.resolved);
      if (decision === "active") {
        staleTracking.delete(id);
        keep.push(row.request);
        continue;
      }
      if (decision === "terminal") {
        staleTracking.delete(id);
        lingering.set(id, { at: Date.now(), request: row.request });
        keep.push(row.request);
        armLinger();
        continue;
      }
      if (decision === "gone") {
        // node(id:) explicitly returned null: stop active tracking.
        staleTracking.delete(id);
        continue;
      }
      if (decision === "expire") {
        staleTracking.delete(id);
        continue;
      }
      const base = row.request || (tracked && tracked.request) || previousRequests.get(id);
      if (base) {
        staleTracking.set(id, {
          since,
          request: { ...base, status: "stale" },
        });
      } else {
        staleTracking.delete(id);
      }
    }
    if (keep.length) requests = mergeRequests(requests, keep);
  }

  const preview = buildStage({
    requests,
    now,
    window: windowMode === "today" ? "today" : "live",
    windowMs: windowMsFor(windowMode),
    selectedId,
    clientFilter,
    limit: first,
    totals: statsTotalsForWindow(statsMode, windowMode, latestTotals, statsLoaded),
    historyCount: latestHistoryCount,
    lingering: lingeringItems(),
    stale: staleItems(),
    inFlightTotal: clientFilter === "all" && statsWindowMatches(statsMode, mode) ? (latestTotals && latestTotals.inFlight) : null,
    inFlightMore: clientFilter === "all" ? inflight.more : false,
  });
  const ids = visibleExecutionIds(preview.tracks, selectedId);
  let executionErrors = [];
  if (ids.length) {
    let outcome = await settle(() => bridge.graphql(executionsQuery(ids, !namesDenied)));
    if (gen !== generation || windowMode !== mode) return;
    let result = outcome.ok ? outcome.value : null;
    let resultErrors = errorsOf(result);
    if (!namesDenied && hasForbidden(resultErrors)) {
      const retry = await settle(() => bridge.graphql(executionsQuery(ids, false)));
      if (gen !== generation || windowMode !== mode) return;
      if (retry.ok && errorsOf(retry.value).length === 0) {
        namesDenied = true;
        showHint(true);
        outcome = retry;
        result = retry.value;
        resultErrors = [];
      } else {
        outcome = retry;
        result = retry.ok ? retry.value : null;
        resultErrors = retry.ok && errorsOf(retry.value).length
          ? errorsOf(retry.value)
          : [{ message: retry.error ? retry.error.message : text.loadFailed }];
      }
    }
    executionErrors = resultErrors;
    requests = attachExecutions(requests, ids, (result && result.data) || {});
    // Linger carries its own request object; keep the final attempt and channel
    // from this executions response instead of the state-only fallback.
    const byId = new Map(requests.map((item) => [item.id, item]));
    for (const [id, item] of lingering) {
      const updated = byId.get(id);
      if (updated) lingering.set(id, { ...item, request: updated });
    }
  }
  if (gen !== generation) return;

  noteCompletions(requests, Date.now(), previousRequests);
  showProblems([...listErrors, ...executionErrors], false);
  latestRequests = requests;
  latestInFlightMore = inflight.more;
  latestHistoryCount = historyRequests.length;
  fetchedLimit = first;
  fetchedAt = now;
  prevRequests = new Map(requests.map((item) => [item.id, item]));
  hasInFlight = (latestTotals && latestTotals.inFlight > 0)
    || requests.some((item) => isLiveStatus(item.status))
    || [...lingering.values()].some((item) => isLiveStatus(item.request && item.request.status));
  render({ loaded: true });
  paintTick(true);
}

async function fetchInflight(withChannel, gen, windowMode) {
  let after = null;
  let more = false;
  const nodes = [];
  const errors = [];
  let lastResponse = null;
  while (true) {
    const outcome = await settle(() => bridge.graphql(inflightQuery(withChannel), { first: INFLIGHT_LIMIT, after }));
    if (gen !== generation || windowMode !== mode) return { aborted: true, ok: false, nodes, more, errors };
    if (!outcome.ok) return { ok: nodes.length > 0, error: outcome.error, nodes, more, errors };
    lastResponse = outcome.value;
    errors.push(...errorsOf(lastResponse));
    const connection = connectionOf(lastResponse);
    if (!connection) return { ok: nodes.length > 0, response: lastResponse, nodes, more, errors };
    for (const node of nodesOf(connection)) {
      const mapped = mapRequest(node);
      if (mapped) nodes.push(mapped);
    }
    if (nodes.length >= INFLIGHT_BUDGET) {
      more = pageInfoOf(connection).hasNext;
      break;
    }
    const cursor = nextInflightCursor(connection, nodes.length, INFLIGHT_BUDGET);
    if (!cursor || cursor === after) {
      more = false;
      break;
    }
    after = cursor;
    more = true;
  }
  return { ok: true, response: lastResponse, nodes, more, errors };
}

async function fetchTerminalStates(ids) {
  const outcome = await settle(() => bridge.graphql(requestStateQuery(ids, !namesDenied)));
  let result = outcome.ok ? outcome.value : null;
  let errors = [...errorsOf(result), ...transportErrors(outcome)];
  if (!namesDenied && hasForbidden(errors)) {
    const retry = await settle(() => bridge.graphql(requestStateQuery(ids, false)));
    const retryResult = retry.ok ? retry.value : null;
    const retryErrors = [...errorsOf(retryResult), ...transportErrors(retry)];
    errors = [...errors, ...retryErrors];
    if (retry.ok && retryErrors.length === 0) {
      namesDenied = true;
      showHint(true);
      result = retryResult;
    }
  }
  return { rows: readStateNodes(result && result.data, ids), errors };
}

function noteCompletions(requests, at, previousRequests) {
  for (const item of requests) {
    const status = String(item.status || "").toLowerCase();
    const before = previousRequests.get ? previousRequests.get(item.id) : null;
    if (before && isLiveStatus(before.status) && !isLiveStatus(status)) {
      lingering.set(item.id, { at, request: item });
      armLinger();
    }
  }
}

function armLinger() {
  clearTimeout(lingerTimer);
  lingerTimer = setTimeout(() => {
    lingerTimer = 0;
    if (loadedOnce) render();
  }, LINGER_MS + 150);
}

function pruneLingering() {
  const wall = Date.now();
  for (const [id, item] of lingering) {
    if (wall - item.at >= LINGER_MS) lingering.delete(id);
  }
}

function lingeringItems() {
  return [...lingering].map(([id, item]) => ({ id, doneAt: item.at, request: item.request }));
}

function staleItems() {
  const now = Date.now();
  for (const [id, item] of staleTracking) {
    if (backfillDecision(now, item.since, null, false) === "expire") staleTracking.delete(id);
  }
  return [...staleTracking.values()].map((item) => item.request).filter(Boolean);
}

function maybeRefreshStats(since, now, windowMode) {
  if (!statsDue(statsAt, now)) return;
  statsAt = now.getTime();
  const token = ++statsToken;
  const version = statsVersion;
  Promise.resolve().then(() => bridge.graphql(statsQuery(), { since })).then((response) => {
    if (!statsAccept(token, statsToken, version, statsVersion)) return;
    const totals = readTotals(response && response.data);
    if (!totals) {
      statsError = true;
      render();
      return;
    }
    latestTotals = totals;
    statsMode = windowMode;
    statsLoaded = true;
    statsError = false;
    render();
  }).catch(() => {
    if (!statsAccept(token, statsToken, version, statsVersion)) return;
    statsError = true;
    render();
  });
}

function showHint(denied) {
  hintEl.hidden = !denied;
  hintEl.textContent = denied ? text.channelHint : "";
}

function showProblems(errors, failed) {
  const messages = (errors || [])
    .filter((error) => error && !(namesDenied && (isForbiddenError(error) || channelSelectionForbidden([error]))))
    .map((error) => error.message)
    .filter(Boolean);
  if (!messages.length) {
    statusEl.textContent = failed ? text.loadFailed : "";
    statusEl.hidden = !failed;
    return;
  }
  statusEl.hidden = false;
  statusEl.textContent = messages.join(" ");
}

function showError(error) {
  paintTick(false);
  const message = error && error.message ? error.message : "";
  statusEl.hidden = false;
  statusEl.textContent = text.requestFailed(message);
}

function render(options) {
  const now = fetchedAt || new Date();
  pruneLingering();
  const view = buildStage({
    requests: latestRequests,
    now,
    window: mode === "today" ? "today" : "live",
    windowMs: windowMsFor(mode),
    selectedId,
    clientFilter,
    limit: fetchedLimit,
    totals: statsTotalsForWindow(statsMode, mode, latestTotals, statsLoaded),
    historyCount: latestHistoryCount,
    lingering: lingeringItems(),
    stale: staleItems(),
    inFlightTotal: clientFilter === "all" && statsWindowMatches(statsMode, mode) && latestTotals
      ? latestTotals.inFlight
      : null,
    inFlightMore: clientFilter === "all" ? latestInFlightMore : false,
  });
  if (selectedId && view.focusId !== selectedId) selectedId = "";
  lastVisibleTrackIds = view.tracks.filter((track) => track.live).map((track) => track.id);
  for (const track of view.tracks) {
    if (track.failed && track.lingering && track.rowId) flashes.set(track.rowId, Date.now() + 1600);
  }
  if (options && options.loaded) loadedOnce = true;
  paintChrome(view);
  paintStage(view);
  paintLog(view);
}

function paintChrome(view) {
  fillFilter(view.availableClients);
  updatedEl.textContent = fetchedAt ? text.updated(formatTime(fetchedAt, locale)) : text.loading;
  const empty = view.shown === 0 && view.tracks.length === 0;
  emptyEl.hidden = !empty;
  emptyText.textContent = view.windowCount === 0
    ? text.emptyWindow(windowLabelFor(mode))
    : (view.scope === "sample" ? text.emptyFilterSample(view.pageCount) : text.emptyFilter);
  paintStats(view);
  paintOverviewNote(view, empty);
  paintFocusBar(view, empty);
  logPanel.hidden = view.log.length === 0;
  logHint.textContent = selectedId ? text.pinnedHint : text.pinHint;
}

function paintOverviewNote(view, empty) {
  const parts = [];
  if (!empty && view.staticOverview) parts.push(text.staticOverview);
  if (!empty && view.inFlightTruncated) {
    parts.push(text.inFlightLimited(view.inFlightLoaded, view.inFlightTotal));
  } else if (!empty && view.inFlightMore) {
    parts.push(text.inFlightMore(view.inFlightLoaded));
  }
  if (!empty && view.trackOverflow > 0) {
    parts.push(view.inFlightMore ? text.trackOverflowLower(view.trackOverflow) : text.trackOverflow(view.trackOverflow));
  }
  overviewNote.hidden = !parts.length;
  overviewNote.textContent = parts.join(" ");
}

// Counts have their own lifecycle. A successful stats response stays visible
// even when the first list request failed and returned no rows.
function paintStats(view) {
  const show = statsVisible(statsLoaded, view.shown);
  statsBlock.hidden = !show;
  if (!show) return;
  sampleNoteEl.textContent = statsError && !statsLoaded ? text.statsUnavailable : statsNote(view);
  countsEl.replaceChildren(
    stat(view.scope === "window" ? text.requests : text.listLatest, view.counts.requests, ""),
    stat(text.inFlightNow, view.counts.inFlight, view.counts.inFlight ? "busy" : ""),
    stat(text.failedWindow, view.counts.failed, view.counts.failed ? "bad" : ""),
  );
}

function statsNote(view) {
  if (view.scope === "window") {
    const total = view.windowTotal == null ? view.pageCount : view.windowTotal;
    if (total > view.pageCount) return text.statsWindow(total, view.pageCount);
    return text.statsWindowComplete(total);
  }
  if (view.scope === "list") return text.statsList(view.pageCount);
  return text.statsSample(view.pageCount);
}

function stat(label, value, state) {
  const tile = document.createElement("div");
  tile.className = state ? `stat ${state}` : "stat";
  const head = document.createElement("span");
  head.className = "stat-label";
  const dot = document.createElement("span");
  dot.className = "tick";
  head.append(dot, label);
  const number = document.createElement("span");
  number.className = "stat-value";
  number.textContent = String(value);
  tile.append(head, number);
  return tile;
}

function paintFocusBar(view, empty) {
  const focus = view.focus;
  focusBar.hidden = empty || !focus;
  if (focusBar.hidden) {
    barFocus = "";
    return;
  }
  focusBar.style.setProperty("--client", focus.hue);
  focusTag.textContent = text.trackTag(focus.trackIndex);
  const line = statusLine(view, locale).replace(/[。.]$/, "");
  summaryEl.textContent = line;
  summaryEl.title = line;
  if (focus.id !== barFocus) {
    barFocus = focus.id;
    restartAnimation(focusBar, "in");
  }
}

function paintTick(ok) {
  tickEl.classList.toggle("on", ok);
  tickEl.classList.toggle("busy", ok && hasInFlight);
}

function fillFilter(clients) {
  const current = clientFilter;
  const nextKey = ["all", ...clients.map((client) => `${client.id}\t${clientLabel(client, locale)}`)].join("\n");
  if (nextKey === filterKey && [...filterEl.options].some((option) => option.value === current)) return;
  filterKey = nextKey;
  filterEl.replaceChildren();
  const all = document.createElement("option");
  all.value = "all";
  all.textContent = text.allClients;
  filterEl.append(all);
  for (const client of clients) {
    const option = document.createElement("option");
    option.value = client.id;
    option.textContent = clientLabel(client, locale);
    filterEl.append(option);
  }
  if (current !== "all" && ![...filterEl.options].some((option) => option.value === current)) {
    const extra = document.createElement("option");
    extra.value = current;
    extra.textContent = current;
    filterEl.append(extra);
  }
  filterEl.value = current;
}

function paintStage(view) {
  const empty = view.shown === 0 && view.tracks.length === 0;
  stage.hidden = empty;
  stageScroll.hidden = empty;
  if (empty) {
    wires.replaceChildren();
    return;
  }
  drawColumns(view);
  drawWires(view);
  drawTracks(view);
}

function drawColumns(view) {
  colClients.replaceChildren(columnLabel(text.colClient, view.clients.length, view.nodeOverflow.clients));
  for (const client of view.clients) colClients.append(clientCard(client, view));
  colKeys.replaceChildren(columnLabel(text.colKey, view.keys.length, view.nodeOverflow.keys));
  for (const key of view.keys) colKeys.append(keyCard(key, view));
  colHub.replaceChildren(columnLabel(""), hubCard(view));
  colChannels.replaceChildren(columnLabel(text.colChannel, view.channels.length, view.nodeOverflow.channels));
  for (const row of view.channels) colChannels.append(channelCard(row, view));
}

function columnLabel(value, count, overflow) {
  const label = document.createElement("div");
  label.className = "col-label";
  label.textContent = value;
  if (count != null) {
    const badge = document.createElement("span");
    badge.className = "col-count";
    badge.textContent = String(count);
    label.append(badge);
  }
  if (overflow > 0) {
    const more = document.createElement("span");
    more.className = "col-more";
    more.textContent = `+${overflow}`;
    more.title = text.moreNodes(overflow);
    label.append(more);
  }
  return label;
}

function clientCard(client, view) {
  const card = document.createElement("article");
  card.className = "card client";
  card.dataset.node = client.nodeId;
  card.style.setProperty("--client", client.hue);
  if (view.focus && view.focus.clientId === client.id) card.classList.add("lit");
  const label = clientLabel(client, locale);
  card.title = label;
  card.append(iconEl(iconSvg(client.id)), cardText(label, client.latestModel || text.requestsN(client.requestCount), true));
  return card;
}

function keyCard(key, view) {
  const card = document.createElement("article");
  card.className = "card key";
  card.dataset.node = key.nodeId;
  if (view.focus && view.focus.keyId === key.id) {
    card.classList.add("lit");
    card.style.setProperty("--client", view.focus.hue);
  }
  const name = key.id === "none" ? text.noKey : (key.name || key.id);
  const subtitle = key.id === "none" ? text.unassigned : keyStatus(key.status);
  card.title = name;
  card.append(iconEl(KEY_ICON), cardText(name, subtitle, false));
  return card;
}

function hubCard(view) {
  const slot = document.createElement("div");
  slot.className = "hub-slot";
  const card = document.createElement("article");
  card.className = "card hub";
  card.dataset.node = "hub";
  const busy = view.counts.inFlight > 0;
  if (busy) card.classList.add("busy");
  const mark = document.createElement("span");
  mark.className = "hub-mark";
  mark.innerHTML = HUB_ICON;
  const name = document.createElement("span");
  name.className = "hub-name";
  name.textContent = "AxonHub";
  const state = document.createElement("span");
  state.className = "hub-state";
  state.textContent = busy ? text.hubBusy(view.counts.inFlight) : text.hubIdle;
  card.append(mark, name, state);
  slot.append(card);
  return slot;
}

function channelCard(row, view) {
  const card = document.createElement("article");
  card.className = `channel ${row.kind}`;
  if (flashing(row.rowId)) card.classList.add("flash-failed");
  card.dataset.node = row.nodeId;
  const selectedTrack = view.focus && row.tracks.some((track) => track.id === view.focus.id);
  const hue = selectedTrack ? view.focus.hue : (row.tracks.length ? row.tracks[0].hue : "");
  if (hue && (row.kind === "answering" || row.tracks.length)) card.style.setProperty("--client", hue);
  const main = document.createElement("div");
  main.className = "channel-main";
  const title = document.createElement("div");
  title.className = "channel-title";
  const name = document.createElement("span");
  name.className = "channel-name";
  name.textContent = row.name;
  name.title = row.name;
  title.append(name);
  if (row.suffix) {
    const suffix = document.createElement("span");
    suffix.className = "suffix mono";
    suffix.textContent = row.suffix;
    title.append(suffix);
  }
  main.append(title);
  if (row.modelID) {
    const model = document.createElement("div");
    model.className = "channel-model mono";
    model.textContent = row.modelID;
    main.append(model);
  }
  if (row.kind === "failed" && row.errorMessage) {
    const error = document.createElement("div");
    error.className = "channel-error";
    error.textContent = row.errorMessage;
    error.title = row.errorMessage;
    main.append(error);
  }
  if (row.tracks.length) {
    const chips = document.createElement("div");
    chips.className = "track-chips";
    for (const track of row.tracks) {
      const chip = document.createElement("button");
      chip.type = "button";
      chip.className = "track-chip";
      chip.dataset.trackId = track.id;
      chip.style.setProperty("--track", track.hue);
      chip.classList.toggle("live", track.live);
      chip.classList.toggle("failed", track.failed);
      chip.classList.toggle("selected", Boolean(view.focus && view.focus.id === track.id));
      chip.textContent = String(track.index);
      chip.title = text.trackLabel(track.index);
      chip.setAttribute("aria-label", text.trackLabel(track.index));
      chips.append(chip);
    }
    main.append(chips);
  }
  const status = badge(row.kind, channelStatusText(row.kind, locale, {
    latencyMs: row.latencyMs,
    code: row.responseStatusCode,
  }));
  status.title = channelStatusText(row.kind, locale, {
    latencyMs: row.latencyMs,
    code: row.responseStatusCode,
    message: row.errorMessage,
  });
  card.append(main, status);
  return card;
}

function badge(kind, label) {
  const el = document.createElement("span");
  el.className = `badge ${kind}`;
  el.textContent = label;
  return el;
}

function iconEl(svg) {
  const icon = document.createElement("span");
  icon.className = "icon";
  icon.innerHTML = svg;
  return icon;
}

function cardText(name, sub, monoSub) {
  const box = document.createElement("div");
  box.className = "card-text";
  const title = document.createElement("div");
  title.className = "node-name";
  title.textContent = name;
  const line = document.createElement("div");
  line.className = monoSub ? "sub mono" : "sub";
  line.textContent = sub;
  line.title = sub;
  box.append(title, line);
  return box;
}

function keyStatus(status) {
  if (status === "enabled") return text.keyEnabled;
  if (status === "disabled") return text.keyDisabled;
  if (status === "archived") return text.keyArchived;
  return status || "—";
}

function requestStatus(status) {
  if (status === "pending") return text.statusPending;
  if (status === "processing") return text.statusProcessing;
  if (status === "completed") return text.statusCompleted;
  if (status === "failed") return text.statusFailed;
  if (status === "canceled") return text.statusCanceled;
  if (status === "stale") return text.statusStale;
  return status || "";
}

function flashing(rowId) {
  const until = flashes.get(rowId);
  if (!until) return false;
  if (Date.now() > until) {
    flashes.delete(rowId);
    return false;
  }
  return true;
}

function paintLog(view) {
  const scroll = logWrap.scrollTop;
  logWrap.replaceChildren();
  if (!view.shown) return;
  const table = document.createElement("table");
  const head = document.createElement("thead");
  const headRow = document.createElement("tr");
  const headings = [text.time, text.client, text.apiKey, text.model, text.status, text.channel, text.latency];
  headings.forEach((heading, index) => {
    const cell = document.createElement("th");
    cell.textContent = heading;
    if (index === headings.length - 1) cell.className = "num";
    headRow.append(cell);
  });
  head.append(headRow);
  const body = document.createElement("tbody");
  const seen = new Set();
  for (const row of view.log) {
    seen.add(row.id);
    const tr = document.createElement("tr");
    tr.dataset.id = row.id;
    tr.tabIndex = 0;
    tr.style.setProperty("--client", row.hue || row.client.hue);
    if (selectedId && selectedId === row.id) {
      tr.classList.add("selected");
      tr.setAttribute("aria-current", "true");
    }
    if (seenRows && !seenRows.has(row.id)) tr.classList.add("fresh");
    const who = document.createElement("span");
    who.className = "who";
    who.textContent = clientLabel(row.client, locale);
    tr.append(
      cell(formatTime(row.createdAt, locale), "time mono"),
      cell(who),
      cell(row.apiKeyName || text.noKey, row.apiKeyName ? "" : "muted"),
      cell(row.modelID, "model mono"),
      cell(badge(row.status, requestStatus(row.status))),
      cell(row.channelName || "—", row.channelName ? "" : "muted"),
      cell(row.latencyMs == null ? "—" : formatLatency(row.latencyMs), "num mono"),
    );
    body.append(tr);
  }
  seenRows = seen;
  table.append(head, body);
  logWrap.append(table);
  logWrap.scrollTop = scroll;
}

function cell(content, className) {
  const td = document.createElement("td");
  if (className) td.className = className;
  if (typeof content === "string") td.textContent = content;
  else td.append(content);
  return td;
}

function restartAnimation(el, className) {
  el.classList.remove(className);
  void el.offsetWidth;
  el.classList.add(className);
}

function drawWires(view) {
  const origin = stage.getBoundingClientRect();
  syncSvg(wires, origin);
  wires.replaceChildren();
  for (const wire of view.wires.faint) drawLine(wire.from, wire.to, origin, "wire-faint");
}

function drawLine(fromId, toId, origin, className) {
  const from = pointOf(fromId, origin);
  const to = pointOf(toId, origin);
  if (!from || !to) return;
  const start = [from.right, from.cy];
  const end = [to.left, to.cy];
  const [c1, c2] = wireBends(start, end);
  const path = document.createElementNS(SVG_NS, "path");
  path.setAttribute("d", `M${start[0]} ${start[1]} C${c1[0]} ${c1[1]} ${c2[0]} ${c2[1]} ${end[0]} ${end[1]}`);
  path.setAttribute("class", className);
  wires.append(path);
}

// Every visible request gets its own path. A shared client/key/channel keeps
// the same four columns, while lane offsets and endpoint dots keep several
// requests on the same route visually separate.
function drawTracks(view) {
  const origin = stage.getBoundingClientRect();
  const tracks = view.tracks || [];
  const total = tracks.length;
  const order = tracks.map((track, index) => ({ track, index }));
  if (view.focus) {
    // Draw the selected lane last so it stays above dimmed neighbors.
    order.sort((left, right) => Number(left.track.id === view.focus.id) - Number(right.track.id === view.focus.id));
  }
  for (const { track, index } of order) {
    const lane = laneOffsetFor(index, total);
    const nodeIds = [`client:${track.clientId}`, `key:${track.keyId}`, "hub"];
    if (track.rowId) nodeIds.push(`channel:${track.rowId}`);
    const nodes = [];
    for (const nodeId of nodeIds) {
      const point = pointOf(nodeId, origin);
      if (point) nodes.push(point);
    }
    if (nodes.length < 2) continue;
    const selected = Boolean(view.focus && view.focus.id === track.id);
    const dim = Boolean(view.focus && !selected);
    const classes = ["track"];
    if (track.live) classes.push("live");
    if (track.failed) classes.push("failed");
    if (track.stale) classes.push("stale");
    if (track.static) classes.push("static");
    if (track.lingering) classes.push("linger");
    if (selected) classes.push("selected");
    if (dim) classes.push("dim");
    const d = trackPath(nodes, lane);
    const path = document.createElementNS(SVG_NS, "path");
    path.setAttribute("d", d);
    path.setAttribute("class", classes.join(" "));
    path.setAttribute("stroke", track.hue);
    if (track.lingering && Number.isFinite(track.doneAt)) {
      const age = Math.max(0, Date.now() - track.doneAt);
      path.style.animationDelay = `-${age}ms`;
    }
    const title = document.createElementNS(SVG_NS, "title");
    title.textContent = trackTitle(track);
    path.append(title);
    wires.append(path);

    const hit = document.createElementNS(SVG_NS, "path");
    hit.setAttribute("d", d);
    hit.setAttribute("class", "track-hit");
    hit.setAttribute("data-track-id", track.id);
    hit.setAttribute("tabindex", "0");
    hit.setAttribute("role", "button");
    hit.setAttribute("aria-label", trackTitle(track));
    wires.append(hit);

    const end = nodes[nodes.length - 1];
    const dot = document.createElementNS(SVG_NS, "circle");
    dot.setAttribute("class", selected ? "track-end selected" : (dim ? "track-end dim" : "track-end"));
    dot.setAttribute("cx", String(end.left - 4));
    dot.setAttribute("cy", String(end.cy + lane));
    dot.setAttribute("r", selected ? "4" : "2.6");
    dot.setAttribute("fill", track.hue);
    wires.append(dot);
  }
}

function trackTitle(track) {
  const parts = [
    text.trackLabel(track.trackIndex),
    clientLabel({ id: track.clientId, name: track.clientName }, locale),
    track.modelID,
    requestStatus(track.status),
    track.channelName || text.noChannel,
  ];
  return parts.filter(Boolean).join(" · ");
}

function trackPath(nodes, lane) {
  let d = "";
  for (let index = 0; index < nodes.length - 1; index += 1) {
    const from = nodes[index];
    const to = nodes[index + 1];
    const start = [from.right, from.cy + lane];
    const end = [to.left, to.cy + lane];
    const [c1, c2] = wireBends(start, end);
    if (index === 0) d += `M${start[0]} ${start[1]}`;
    else d += ` L${start[0]} ${start[1]}`;
    d += ` C${c1[0]} ${c1[1]} ${c2[0]} ${c2[1]} ${end[0]} ${end[1]}`;
  }
  return d;
}

function wireBends(start, end) {
  const mid = (start[0] + end[0]) / 2;
  return [[mid, start[1]], [mid, end[1]]];
}

function pointOf(nodeId, origin) {
  if (!nodeId) return null;
  let el = null;
  for (const node of stage.querySelectorAll("[data-node]")) {
    if (node.getAttribute("data-node") === nodeId) {
      el = node;
      break;
    }
  }
  if (!el) return null;
  const rect = el.getBoundingClientRect();
  return {
    el,
    left: rect.left - origin.left,
    right: rect.right - origin.left,
    cx: rect.left - origin.left + rect.width / 2,
    cy: rect.top - origin.top + rect.height / 2,
  };
}

function syncSvg(svg, rect) {
  const width = Math.max(1, rect.width);
  const height = Math.max(1, rect.height);
  svg.setAttribute("width", String(width));
  svg.setAttribute("height", String(height));
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
}
