export const REFRESH_MS = 10 * 60 * 1000;
export const KV_VALUE_LIMIT = 256 * 1024;
export const ACTOR_KEY = "actor";

const AUTHORS = {
  thsottiaux: "Tibo",
  ajambrosino: "Andrew",
  _mohansolo: "Varun",
  claudedevs: "ClaudeDevs",
  lydiahallie: "Lydia",
};

export const PRODUCTS = [
  {
    id: "codex",
    path: "codex-resets",
    zh: "Codex",
    en: "Codex",
    leadZh: "Tibo 和 Andrew 公布的 Codex 额度重置与重置卡。",
    leadEn: "Codex quota resets and reset cards announced by Tibo and Andrew.",
  },
  {
    id: "antigravity",
    path: "antigravity-resets",
    zh: "Antigravity",
    en: "Antigravity",
    leadZh: "Varun 公布的 Antigravity 额度重置与重置卡。",
    leadEn: "Antigravity quota resets and reset cards announced by Varun.",
  },
  {
    id: "claude",
    path: "claude-resets",
    zh: "Claude",
    en: "Claude",
    leadZh: "ClaudeDevs 和 Lydia 公布的 Claude 额度重置与重置卡。",
    leadEn: "Claude quota resets and reset cards announced by ClaudeDevs and Lydia.",
  },
];

const zh = {
  refresh: "刷新",
  loading: "正在读取重置日历…",
  readOnly: "当前账号不能写入插件存储。已有的共享快照会继续用；没有快照时，这次结果只留在当前弹窗里。",
  saveFailed: "快照没有写入插件存储。",
  tooLarge: "这份日历超过 256 KiB，没有写入插件存储。",
  stillFresh: "仍使用 10 分钟内的快照。",
  limited: (when) => `AIHOT 限制了请求频率，请在${when}后再试。`,
  limitedCached: (when) => `AIHOT 限制了请求频率，下面仍是上次保存的日历。请在${when}后再试。`,
  limitedDetail: (detail, when) => `${detail}。请在${when}后再试。`,
  staleHttp: "这次没有取到新数据，下面仍是上次保存的日历。",
  badJson: "返回的日历无法解析。",
  badJsonCached: "这次返回无法解析，下面仍是上次保存的日历。",
  http: (status) => `读取日历失败（HTTP ${status}）。`,
  networkDenied: "还没有授予 AIHOT 日历地址。启用插件时勾选这条出站规则。",
  bridge: (message) => message || "读取日历失败。",
  emptyPending: "当前没有等待生效的重置",
  lastCard: (day) => `上一次重置卡发放：${day}确认`,
  lastReset: (day) => `上一次额度重置：${day}`,
  confirmNote: "确认帖的时间不是到账时间。这里不预测还没宣布的下一次重置。",
  inferred: "公告里的时间是推算的。",
  basisPost: "依据是确认帖",
  basisReceipt: "依据是到账核实",
  statsReset: "近 90 天额度重置",
  statsCard: "近 90 天发重置卡",
  statsGap: "重置间隔中位数",
  statsLast: "上次额度重置",
  times: (count) => `${count} 次`,
  days: (count) => `${count} 天`,
  none: "—",
  calendar: "重置日历",
  calendarLead: "点日期查看当天的额度重置和重置卡。",
  monthResets: (count) => `本月额度重置 ${count} 次`,
  monthCards: (count) => `本月发重置卡 ${count} 次`,
  prevMonth: "上个月",
  nextMonth: "下个月",
  today: "今天",
  noDay: "这一天没有记录",
  recent: "最近原帖",
  original: "英文原文",
  translation: "中文",
  copyPost: "原帖链接",
  copyPage: "AIHOT 链接",
  copied: "链接已复制",
  waiting: "等待生效",
  card: "重置卡",
  reset: "额度重置",
  weekdays: ["一", "二", "三", "四", "五", "六", "日"],
  clock: "全部为北京时间",
  coverage: (day) => `日历从 ${day} 起`,
  checked: (stamp) => `最近检查 ${stamp}`,
  monitor: {
    healthy: "监控正常",
    delayed: "检查延迟",
    attention: "需要关注",
    unknown: "状态未知",
  },
  presentation: {
    announced: "已宣布，等待生效",
    in_progress: "正在发放",
    confirmed: "已确认",
    expired_unconfirmed: "预计时间已过，尚未确认",
    likely_completed: "预计已生效",
  },
  outage: "服务异常说明",
  recovered: (stamp) => `已恢复 ${stamp}`,
  retryIn: (amount, unit) => (unit === "minute" ? `${amount}分钟` : `${amount}秒`),
};

const en = {
  refresh: "Refresh",
  loading: "Loading the reset calendar…",
  readOnly: "This account cannot save plugin data. A shared snapshot is still used when one exists; otherwise this result stays in the current dialog.",
  saveFailed: "The snapshot was not saved.",
  tooLarge: "This calendar is over 256 KiB, so it was not saved.",
  stillFresh: "Still using the snapshot from the last 10 minutes.",
  limited: (when) => `AIHOT is rate limiting requests. Try again in ${when}.`,
  limitedCached: (when) => `AIHOT is rate limiting requests. The calendar below is the last saved snapshot. Try again in ${when}.`,
  limitedDetail: (detail, when) => `${detail} Try again in ${when}.`,
  staleHttp: "The new calendar did not load. The last saved snapshot is shown below.",
  badJson: "The calendar response could not be parsed.",
  badJsonCached: "The new response could not be parsed. The last saved snapshot is shown below.",
  http: (status) => `The calendar request failed (HTTP ${status}).`,
  networkDenied: "The AIHOT calendar URL has not been granted. Enable that outbound rule for this plugin.",
  bridge: (message) => message || "The calendar request failed.",
  emptyPending: "Nothing is waiting to take effect",
  lastCard: (day) => `Latest reset card: confirmed ${day}`,
  lastReset: (day) => `Latest quota reset: ${day}`,
  confirmNote: "The confirmation post is not the moment a balance arrives. Unannounced future resets are not predicted.",
  inferred: "The announced time is inferred.",
  basisPost: "Based on a confirmation post",
  basisReceipt: "Based on a balance check",
  statsReset: "Quota resets, 90 days",
  statsCard: "Reset cards, 90 days",
  statsGap: "Median reset gap",
  statsLast: "Latest quota reset",
  times: (count) => String(count),
  days: (count) => `${count} d`,
  none: "—",
  calendar: "Calendar",
  calendarLead: "Choose a day to see its resets and cards.",
  monthResets: (count) => `${count} quota resets this month`,
  monthCards: (count) => `${count} reset cards this month`,
  prevMonth: "Previous month",
  nextMonth: "Next month",
  today: "Today",
  noDay: "Nothing recorded on this day",
  recent: "Recent posts",
  original: "Original",
  translation: "Translation",
  copyPost: "Source link",
  copyPage: "AIHOT link",
  copied: "Link copied",
  waiting: "Waiting",
  card: "Card",
  reset: "Reset",
  weekdays: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
  clock: "Times are Beijing time",
  coverage: (day) => `Calendar starts ${day}`,
  checked: (stamp) => `Last check ${stamp}`,
  monitor: {
    healthy: "Monitor healthy",
    delayed: "Checks delayed",
    attention: "Needs attention",
    unknown: "Status unknown",
  },
  presentation: {
    announced: "Announced, waiting",
    in_progress: "In progress",
    confirmed: "Confirmed",
    expired_unconfirmed: "Window passed, unconfirmed",
    likely_completed: "Likely already in effect",
  },
  outage: "Outage note",
  recovered: (stamp) => `Recovered ${stamp}`,
  retryIn: (amount, unit) => (unit === "minute" ? `${amount} min` : `${amount} sec`),
};

export function copyFor(locale) {
  return String(locale || "").toLowerCase().startsWith("zh") ? zh : en;
}

export function productById(id) {
  return PRODUCTS.find((item) => item.id === id) || PRODUCTS[0];
}

export function endpoint(productId) {
  return `https://aihot.news/api/v1/${productById(productId).path}`;
}

export function storageKeys(productId) {
  const id = productById(productId).id;
  return { body: `body.${id}`, meta: `meta.${id}` };
}

export function userAgent(actorId) {
  const actor = String(actorId || "").trim() || "session";
  return `aihot-api/2.0.0 aihot-actor/${actor} axonhub-reset-watch/0.1.0`;
}

export function byteLength(value) {
  return new TextEncoder().encode(String(value ?? "")).length;
}

export function emptyMeta() {
  return { etag: "", fetchedAt: "", retryAt: "" };
}

export function parseMeta(raw) {
  if (!raw) return emptyMeta();
  try {
    const value = JSON.parse(raw);
    if (!value || typeof value !== "object" || Array.isArray(value)) return emptyMeta();
    return {
      etag: typeof value.etag === "string" ? value.etag : "",
      fetchedAt: typeof value.fetchedAt === "string" ? value.fetchedAt : "",
      retryAt: typeof value.retryAt === "string" ? value.retryAt : "",
    };
  } catch {
    return emptyMeta();
  }
}

export function authorName(handle) {
  const name = String(handle || "").trim();
  return AUTHORS[name] || name;
}

export function shanghaiDay(iso) {
  const time = Date.parse(iso || "");
  if (!Number.isFinite(time)) return "";
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(time));
}

export function eventDay(event) {
  if (!event || typeof event !== "object") return "";
  if (typeof event.occurredOn === "string" && /^\d{4}-\d{2}-\d{2}$/.test(event.occurredOn)) return event.occurredOn;
  return shanghaiDay(event.confirmedAt || event.schedule?.from || event.estimate?.from || event.createdAt || event.updatedAt);
}

export function isPending(event) {
  const status = event?.presentation?.status;
  if (status === "announced" || status === "in_progress" || status === "expired_unconfirmed") return true;
  if (status) return false;
  return event?.status === "announced";
}

export function parseSnapshot(raw) {
  let data;
  try {
    data = JSON.parse(raw || "");
  } catch {
    return { ok: false };
  }
  if (!data || typeof data !== "object" || data.schemaVersion !== 1 || !Array.isArray(data.events)) return { ok: false };
  const today = /^\d{4}-\d{2}-\d{2}$/.test(data.today || "") ? data.today : shanghaiDay(data.checkedAt);
  return {
    ok: true,
    snapshot: {
      today,
      checkedAt: typeof data.checkedAt === "string" ? data.checkedAt : "",
      historyFrom: typeof data.historyFrom === "string" ? data.historyFrom : "",
      events: data.events,
      activities: Array.isArray(data.activities) ? data.activities : [],
      monitor: data.monitor && typeof data.monitor === "object" ? data.monitor : null,
      outage: data.outage && typeof data.outage === "object" ? data.outage : null,
    },
  };
}

function addDays(day, amount) {
  const [year, month, date] = day.split("-").map(Number);
  const value = new Date(Date.UTC(year, month - 1, date + amount));
  const monthText = String(value.getUTCMonth() + 1).padStart(2, "0");
  const dateText = String(value.getUTCDate()).padStart(2, "0");
  return `${value.getUTCFullYear()}-${monthText}-${dateText}`;
}

function diffDays(start, end) {
  const [sy, sm, sd] = start.split("-").map(Number);
  const [ey, em, ed] = end.split("-").map(Number);
  return Math.round((Date.UTC(ey, em - 1, ed) - Date.UTC(sy, sm - 1, sd)) / 86400000);
}

function latestSettled(events, type) {
  return events
    .filter((event) => event.type === type && !isPending(event) && eventDay(event))
    .sort((left, right) => eventDay(left).localeCompare(eventDay(right)) || String(left.confirmedAt || "").localeCompare(String(right.confirmedAt || "")))
    .at(-1) || null;
}

export function summarize(events, today) {
  const list = Array.isArray(events) ? events : [];
  const lastReset = latestSettled(list, "direct_reset");
  const lastCard = latestSettled(list, "reset_credit");
  if (!/^\d{4}-\d{2}-\d{2}$/.test(today || "")) {
    return { resets: 0, cards: 0, medianDays: null, lastReset, lastCard, start: "" };
  }
  const start = addDays(today, -89);
  const resets = list.filter((event) => event.type === "direct_reset" && !isPending(event) && eventDay(event) >= start && eventDay(event) <= today);
  const cards = list.filter((event) => event.type === "reset_credit" && !isPending(event) && eventDay(event) >= start && eventDay(event) <= today);
  const days = resets.map(eventDay).sort();
  let medianDays = null;
  if (days.length >= 2) {
    const gaps = days.slice(1).map((day, index) => diffDays(days[index], day)).sort((left, right) => left - right);
    const mid = Math.floor(gaps.length / 2);
    medianDays = gaps.length % 2 === 1 ? gaps[mid] : Math.round((gaps[mid - 1] + gaps[mid]) / 2);
  }
  return { resets: resets.length, cards: cards.length, medianDays, lastReset, lastCard, start };
}

export function countInMonth(events, monthId) {
  const counts = { resets: 0, cards: 0, waiting: 0 };
  for (const event of events || []) {
    const day = eventDay(event);
    if (!day.startsWith(`${monthId}-`)) continue;
    if (isPending(event)) counts.waiting += 1;
    else if (event.type === "reset_credit") counts.cards += 1;
    else if (event.type === "direct_reset") counts.resets += 1;
  }
  return counts;
}

export function monthGrid(monthId) {
  const [year, month] = monthId.split("-").map(Number);
  const lead = (new Date(Date.UTC(year, month - 1, 1)).getUTCDay() + 6) % 7;
  const count = new Date(Date.UTC(year, month, 0)).getUTCDate();
  const cells = Array(lead).fill(null);
  for (let day = 1; day <= count; day += 1) cells.push(`${monthId}-${String(day).padStart(2, "0")}`);
  while (cells.length % 7 !== 0) cells.push(null);
  return cells;
}

export function shiftMonth(monthId, delta) {
  const [year, month] = monthId.split("-").map(Number);
  const value = new Date(Date.UTC(year, month - 1 + delta, 1));
  return `${value.getUTCFullYear()}-${String(value.getUTCMonth() + 1).padStart(2, "0")}`;
}

export function monthBounds(snapshot) {
  const days = [];
  for (const event of snapshot?.events || []) {
    const day = eventDay(event);
    if (day) days.push(day);
  }
  const history = shanghaiDay(snapshot?.historyFrom);
  if (history) days.push(history);
  if (snapshot?.today) days.push(snapshot.today);
  days.sort();
  const start = days[0] || snapshot?.today || "";
  const end = snapshot?.today || days[days.length - 1] || "";
  return { start: start.slice(0, 7), end: end.slice(0, 7) };
}

export function formatStamp(iso) {
  const time = Date.parse(iso || "");
  if (!Number.isFinite(time)) return "";
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: "Asia/Shanghai",
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(time));
  const pick = (type) => parts.find((part) => part.type === type)?.value || "";
  return `${Number(pick("month"))}/${Number(pick("day"))} ${pick("hour")}:${pick("minute")}`;
}

export function formatDay(day, locale) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day || "")) return "";
  const [year, month, date] = day.split("-").map(Number);
  if (String(locale || "").toLowerCase().startsWith("zh")) return `${month}月${date}日`;
  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric", timeZone: "UTC" }).format(new Date(Date.UTC(year, month - 1, date)));
}

export function formatMonth(monthId, locale) {
  const [year, month] = monthId.split("-").map(Number);
  if (String(locale || "").toLowerCase().startsWith("zh")) return `${year}年${month}月`;
  return new Intl.DateTimeFormat("en", { month: "long", year: "numeric", timeZone: "UTC" }).format(new Date(Date.UTC(year, month - 1, 1)));
}

export function formatRemaining(ms, text) {
  const seconds = Math.max(1, Math.ceil(ms / 1000));
  if (seconds >= 60) return text.retryIn(Math.ceil(seconds / 60), "minute");
  return text.retryIn(seconds, "second");
}

export function excerpt(value, max = 180) {
  const text = String(value ?? "").replace(/\s+/g, " ").trim();
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}…`;
}

export function postText(post, original) {
  if (!post) return "";
  if (original) return post.fullOriginalText || post.originalText || "";
  return post.fullText || post.text || "";
}

export function nextFetch(state, now) {
  const meta = state?.meta || emptyMeta();
  const retryAt = Date.parse(meta.retryAt || "");
  if (Number.isFinite(retryAt) && retryAt > now) return { fetch: false, reason: "retry", retryAt };
  const fetchedAt = Date.parse(meta.fetchedAt || "");
  if (state?.body && Number.isFinite(fetchedAt) && now - fetchedAt < REFRESH_MS) return { fetch: false, reason: "fresh" };
  return { fetch: true, reason: state?.body ? "stale" : "missing" };
}

function lowerHeaders(headers) {
  const out = {};
  for (const [name, value] of Object.entries(headers || {})) out[String(name).toLowerCase()] = String(value);
  return out;
}

export function retryDeadline(header, now) {
  const fallback = now + REFRESH_MS;
  const raw = String(header ?? "").trim();
  if (!raw) return fallback;
  if (/^\d+$/.test(raw)) return now + Number(raw) * 1000;
  const date = Date.parse(raw);
  if (Number.isFinite(date) && date > now) return date;
  return fallback;
}

export function interpretResponse({ status, headers, body, cachedBody, cachedMeta, now }) {
  const headerMap = lowerHeaders(headers);
  const meta = cachedMeta || emptyMeta();
  const cached = cachedBody || "";
  const code = Number(status);
  if (code === 304) {
    if (!cached) return { ok: false, persist: null, body: "", meta, error: "empty-304", warning: null };
    return {
      ok: true,
      persist: "meta",
      body: cached,
      meta: { etag: headerMap.etag || meta.etag, fetchedAt: new Date(now).toISOString(), retryAt: "" },
      error: null,
      warning: null,
    };
  }
  if (code === 200) {
    const parsed = parseSnapshot(body);
    if (!parsed.ok) {
      return { ok: Boolean(cached), persist: null, body: cached, meta, error: cached ? null : "bad-json", warning: cached ? "bad-json" : null };
    }
    return {
      ok: true,
      persist: "all",
      body,
      meta: { etag: headerMap.etag || "", fetchedAt: new Date(now).toISOString(), retryAt: "" },
      error: null,
      warning: null,
    };
  }
  if (code === 429) {
    const retryAt = new Date(retryDeadline(headerMap["retry-after"], now)).toISOString();
    return {
      ok: Boolean(cached),
      persist: "meta",
      body: cached,
      meta: { etag: meta.etag, fetchedAt: meta.fetchedAt, retryAt },
      error: cached ? null : "429",
      warning: cached ? "429" : null,
      detail: body || "",
    };
  }
  return {
    ok: Boolean(cached),
    persist: null,
    body: cached,
    meta,
    error: cached ? null : "http",
    warning: cached ? "stale" : null,
    status: code,
    detail: body || "",
  };
}

export function interpretBridgeError(error, state, now) {
  const code = error && error.code ? String(error.code) : "";
  if (code === "RATE_LIMITED") {
    return { ...interpretResponse({ status: 429, headers: {}, body: "", cachedBody: state?.body, cachedMeta: state?.meta, now }), code };
  }
  const cached = state?.body || "";
  return {
    ok: Boolean(cached),
    persist: null,
    body: cached,
    meta: state?.meta || emptyMeta(),
    error: cached ? null : "bridge",
    warning: cached ? "bridge" : null,
    code,
    message: error && error.message ? String(error.message) : "",
  };
}

export function planWrites(canWrite, decision) {
  if (!canWrite || !decision?.persist) return { writes: [], skippedBody: false };
  const writes = [];
  let skippedBody = false;
  if (decision.persist === "all" && decision.body) {
    if (byteLength(decision.body) > KV_VALUE_LIMIT) skippedBody = true;
    else writes.push({ key: "body", value: decision.body });
  }
  if ((decision.persist === "all" || decision.persist === "meta") && decision.meta) {
    writes.push({ key: "meta", value: JSON.stringify(decision.meta) });
  }
  return { writes, skippedBody };
}

export function problemDetail(body) {
  try {
    const data = JSON.parse(body || "");
    const detail = data?.detail || data?.title || data?.message;
    return typeof detail === "string" ? detail : "";
  } catch {
    return "";
  }
}
