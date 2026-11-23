import {
  ACTOR_KEY,
  PRODUCTS,
  authorName,
  copyFor,
  countInMonth,
  endpoint,
  eventDay,
  excerpt,
  formatDay,
  formatMonth,
  formatRemaining,
  formatStamp,
  interpretBridgeError,
  interpretResponse,
  isPending,
  monthBounds,
  monthGrid,
  nextFetch,
  shanghaiDay,
  parseMeta,
  parseSnapshot,
  planWrites,
  postText,
  problemDetail,
  productById,
  shiftMonth,
  storageKeys,
  summarize,
  userAgent,
} from "./format.js";

const axonhub = window.axonhub;
const lead = document.querySelector("#lead");
const clock = document.querySelector("#clock");
const refresh = document.querySelector("#refresh");
const products = document.querySelector("#products");
const access = document.querySelector("#access");
const banner = document.querySelector("#banner");
const status = document.querySelector("#status");
const content = document.querySelector("#content");

const READ_KV = `query ReadPluginKV($key: String!) { pluginKV(key: $key) { value } }`;
const WRITE_KV = `mutation WritePluginKV($key: String!, $value: String!) { setPluginKV(key: $key, value: $value) { value } }`;

let text = copyFor("en");
let locale = "en";
let canWrite = false;
let actor = "";
let current = "codex";
let busy = false;
let generation = 0;
let month = "";
let selectedDay = "";
let original = false;
let persistNote = "";
const memory = new Map();

if (!axonhub) {
  status.textContent = "AxonHub bridge did not load";
} else {
  refresh.addEventListener("click", () => {
    load(current, true).catch(showFailure);
  });
  boot().catch(showFailure);
}

async function boot() {
  const init = await axonhub.ready();
  locale = init.locale || "en";
  text = copyFor(locale);
  canWrite = init.canWriteKV === true;
  document.documentElement.lang = locale;
  refresh.textContent = text.refresh;
  access.textContent = text.readOnly;
  access.hidden = canWrite;
  for (const product of PRODUCTS) {
    const button = document.createElement("button");
    button.type = "button";
    button.role = "tab";
    button.textContent = locale.startsWith("zh") ? product.zh : product.en;
    button.addEventListener("click", () => {
      if (button.getAttribute("aria-selected") === "true") return;
      original = false;
      month = "";
      selectedDay = "";
      load(product.id, false).catch(showFailure);
    });
    products.append(button);
  }
  actor = await loadActor();
  await load("codex", false);
}

async function loadActor() {
  const saved = await readKey(ACTOR_KEY);
  if (saved && /^[A-Za-z0-9_.:-]{1,128}$/.test(saved)) return saved;
  const created = crypto.randomUUID();
  if (canWrite) await writeKey(ACTOR_KEY, created);
  return created;
}

async function load(productId, manual) {
  const token = ++generation;
  persistNote = "";
  current = productId;
  paintTabs();
  const product = productById(productId);
  lead.textContent = locale.startsWith("zh") ? product.leadZh : product.leadEn;
  setBusy(true, text.loading);
  const state = await ensureState(productId);
  if (token !== generation) return;
  const now = Date.now();
  const plan = nextFetch(state, now);
  if (!plan.fetch) {
    setBusy(false, "");
    if (manual && plan.reason === "fresh") axonhub.toast(text.stillFresh).catch(() => {});
    renderState(state, plan.reason === "retry" ? plan.retryAt : 0);
    return;
  }
  let decision;
  try {
    const headers = { Accept: "application/json", "User-Agent": userAgent(actor) };
    if (state.body && state.meta.etag) headers["If-None-Match"] = state.meta.etag;
    const result = await axonhub.fetch(endpoint(productId), { method: "GET", headers });
    decision = interpretResponse({
      status: result?.status,
      headers: result?.headers,
      body: result?.body,
      cachedBody: state.body,
      cachedMeta: state.meta,
      now,
    });
  } catch (error) {
    decision = interpretBridgeError(error, state, now);
  }
  if (token !== generation) return;
  await applyDecision(productId, state, decision);
  if (token !== generation) return;
  setBusy(false, "");
  renderState(memory.get(productId), 0, decision);
}

async function ensureState(productId) {
  if (memory.has(productId)) return memory.get(productId);
  const keys = storageKeys(productId);
  const [body, metaRaw] = await Promise.all([readKey(keys.body), readKey(keys.meta)]);
  const state = { body: body || "", meta: parseMeta(metaRaw) };
  memory.set(productId, state);
  return state;
}

async function applyDecision(productId, state, decision) {
  persistNote = "";
  const next = { body: decision.body || "", meta: decision.meta || state.meta };
  memory.set(productId, next);
  if (!canWrite || !decision.persist) return;
  const keys = storageKeys(productId);
  const plan = planWrites(true, decision);
  if (plan.skippedBody) persistNote = text.tooLarge;
  for (const write of plan.writes) {
    const key = write.key === "body" ? keys.body : keys.meta;
    const saved = await writeKey(key, write.value);
    if (!saved && canWrite && !persistNote) persistNote = text.saveFailed;
  }
}

async function readKey(key) {
  try {
    const result = await axonhub.graphql(READ_KV, { key });
    if (result?.errors?.length) return "";
    return result?.data?.pluginKV?.value || "";
  } catch {
    return "";
  }
}

async function writeKey(key, value) {
  if (!canWrite) return false;
  try {
    const result = await axonhub.graphql(WRITE_KV, { key, value });
    const error = result?.errors?.[0];
    if (!error) return true;
    const code = error.extensions?.code || "";
    if (code === "FORBIDDEN" || /write_plugins|permission/i.test(error.message || "")) {
      canWrite = false;
      access.hidden = false;
    }
    return false;
  } catch {
    return false;
  }
}

function renderState(state, retryAt, decision) {
  const parsed = state?.body ? parseSnapshot(state.body) : { ok: false };
  if (!parsed.ok) {
    content.hidden = true;
    content.replaceChildren();
    banner.hidden = true;
    clock.textContent = text.clock;
    status.classList.remove("busy");
    status.textContent = retryAt ? text.limited(formatRemaining(retryAt - Date.now(), text)) : hardError(decision);
    return;
  }
  const warning = decision?.warning || (retryAt ? "429" : "");
  paintBanner(warning, decision, retryAt);
  if (!month) {
    month = (parsed.snapshot.today || eventDay(parsed.snapshot.events[0]) || "2026-01").slice(0, 7);
    selectedDay = parsed.snapshot.today || "";
  }
  status.textContent = "";
  content.hidden = false;
  paintSnapshot(parsed.snapshot);
}

function hardError(decision) {
  if (!decision) return text.badJson;
  if (decision.code === "FORBIDDEN") return text.networkDenied;
  if (decision.error === "429" || decision.code === "RATE_LIMITED") {
    const when = formatRemaining(retryLeft(decision), text);
    const detail = problemDetail(decision.detail);
    return detail ? text.limitedDetail(detail, when) : text.limited(when);
  }
  if (decision.error === "bad-json") return text.badJson;
  if (decision.error === "http") return problemDetail(decision.detail) || text.http(decision.status || 0);
  if (decision.error === "bridge" || decision.error === "empty-304") return text.bridge(decision.message);
  return text.badJson;
}

function retryLeft(decision) {
  const retryAt = Date.parse(decision?.meta?.retryAt || "");
  if (!Number.isFinite(retryAt)) return 10 * 60 * 1000;
  return Math.max(0, retryAt - Date.now());
}

function paintBanner(warning, decision, retryAt) {
  banner.classList.toggle("warn", Boolean(warning));
  if (warning === "429") {
    const when = formatRemaining(retryAt ? retryAt - Date.now() : retryLeft(decision), text);
    banner.textContent = text.limitedCached(when);
  } else if (warning === "bad-json") banner.textContent = text.badJsonCached;
  else if (warning === "stale" || warning === "bridge") banner.textContent = problemDetail(decision?.detail) || decision?.message || text.staleHttp;
  else banner.textContent = persistNote;
  banner.hidden = !banner.textContent;
}

function paintSnapshot(snapshot) {
  const summary = summarize(snapshot.events, snapshot.today);
  const bounds = monthBounds(snapshot);
  clock.textContent = clockLine(snapshot);
  const layout = element("div", "layout");
  layout.append(calendar(snapshot, bounds), side(snapshot, summary));
  content.replaceChildren(hero(snapshot, summary), stats(summary), layout);
}

function hero(snapshot, summary) {
  const pending = snapshot.events.filter(isPending);
  const card = element("section", "card");
  const line = element("p", pending.length ? "pending" : "pending ok");
  line.textContent = pending.length ? text.presentation[pending[0].presentation?.status] || text.waiting : text.emptyPending;
  card.append(line);
  const subject = pending[0] || summary.lastCard || summary.lastReset;
  if (subject) {
    const title = element("h2");
    const day = formatDay(eventDay(subject), locale);
    if (pending.length) title.textContent = subject.displayLabel || subject.title || text.waiting;
    else if (subject.type === "reset_credit") title.textContent = text.lastCard(day);
    else title.textContent = text.lastReset(day);
    card.append(title);
  }
  card.append(element("p", "muted", text.confirmNote));
  if (snapshot.outage) card.append(outageBlock(snapshot.outage));
  return card;
}

function stats(summary) {
  const row = element("section", "stats");
  const last = summary.lastReset ? formatDay(eventDay(summary.lastReset), locale) : text.none;
  const gap = summary.medianDays == null ? text.none : text.days(summary.medianDays);
  for (const [label, value] of [
    [text.statsReset, text.times(summary.resets)],
    [text.statsCard, text.times(summary.cards)],
    [text.statsGap, gap],
    [text.statsLast, last],
  ]) {
    const cell = element("div", "stat");
    cell.append(element("span", "", label), element("strong", "", value));
    row.append(cell);
  }
  return row;
}

function calendar(snapshot, bounds) {
  const section = element("section", "card");
  section.append(element("h2", "", text.calendar), element("p", "muted", text.calendarLead));
  const bar = element("div", "month-bar");
  const counts = countInMonth(snapshot.events, month);
  bar.append(monthButton(-1, bounds, text.prevMonth), element("strong", "", formatMonth(month, locale)), monthButton(1, bounds, text.nextMonth));
  section.append(bar, element("p", "muted", `${text.monthResets(counts.resets)} · ${text.monthCards(counts.cards)}`));
  const week = element("div", "week");
  for (const name of text.weekdays) week.append(element("span", "", name));
  const grid = element("div", "grid");
  const grouped = new Map();
  for (const event of snapshot.events) {
    const day = eventDay(event);
    if (!day) continue;
    const list = grouped.get(day) || [];
    list.push(event);
    grouped.set(day, list);
  }
  for (const day of monthGrid(month)) {
    if (!day) {
      grid.append(element("span", "blank"));
      continue;
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = `day-button${day === snapshot.today ? " today" : ""}`;
    button.setAttribute("aria-pressed", day === selectedDay ? "true" : "false");
    button.append(element("strong", "", String(Number(day.slice(8)))));
    const tags = element("span", "tags");
    for (const event of (grouped.get(day) || []).slice(0, 2)) tags.append(element("span", `tag${isPending(event) ? " wait" : ""}`, tagLabel(event)));
    if (tags.childNodes.length) button.append(tags);
    button.addEventListener("click", () => {
      selectedDay = day;
      paintSnapshot(snapshot);
    });
    grid.append(button);
  }
  section.append(week, grid);
  return section;
}

function side(snapshot, summary) {
  const wrap = element("div", "side");
  wrap.append(dayCard(snapshot), postsCard(snapshot, summary));
  return wrap;
}

function dayCard(snapshot) {
  const card = element("section", "card");
  const day = selectedDay || snapshot.today;
  const title = formatDay(day, locale);
  card.append(element("h3", "", day === snapshot.today ? `${title} · ${text.today}` : title));
  const events = snapshot.events.filter((event) => eventDay(event) === day);
  if (!events.length) {
    card.append(element("p", "muted", text.noDay));
    return card;
  }
  for (const event of events) card.append(eventBlock(event));
  return card;
}

function postsCard(snapshot, summary) {
  const card = element("section", "card");
  card.append(element("h3", "", text.recent));
  const posts = snapshot.activities.slice(0, 3);
  if (!posts.length) {
    const event = summary.lastCard || summary.lastReset;
    const post = event?.posts?.[0];
    if (post) card.append(postBlock(post, event?.url));
    return card;
  }
  for (const post of posts) card.append(postBlock(post));
  return card;
}

function eventBlock(event) {
  const block = element("article", "event");
  block.append(element("h3", "", event.displayLabel || event.title || tagLabel(event)));
  const timing = event.estimate?.label || event.schedule?.label || "";
  if (timing) block.append(element("p", "muted", timing));
  if (event.presentation?.timeInferred) block.append(element("p", "muted", text.inferred));
  const basis = event.confirmationBasis === "receipt_review" ? text.basisReceipt : event.confirmationBasis === "source_post" ? text.basisPost : "";
  if (basis && !isPending(event)) block.append(element("p", "muted", basis));
  const post = event.posts?.[0];
  if (post) block.append(postBlock(post, event.url));
  return block;
}

function postBlock(post, pageURL) {
  const block = element("div", "post-block");
  const who = element("p", "muted");
  who.append(element("strong", "", authorName(post.author)));
  if (post.author) who.append(` @${post.author}`);
  if (post.publishedAt) who.append(` · ${formatStamp(post.publishedAt)}`);
  if (post.stage) who.append(` · ${post.stage}`);
  block.append(who);
  const body = element("p", "post", excerpt(postText(post, original), 420));
  block.append(body);
  const actions = element("div", "actions");
  if (post.originalText || post.fullOriginalText) {
    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "text-button";
    toggle.textContent = original ? text.translation : text.original;
    toggle.addEventListener("click", () => {
      original = !original;
      const parsed = parseSnapshot(memory.get(current)?.body || "");
      if (parsed.ok) paintSnapshot(parsed.snapshot);
    });
    actions.append(toggle);
  }
  if (post.url) actions.append(copyButton(text.copyPost, post.url));
  if (pageURL) actions.append(copyButton(text.copyPage, pageURL));
  if (actions.childNodes.length) block.append(actions);
  return block;
}

function outageBlock(outage) {
  const block = element("div", "event");
  block.append(element("h3", "", text.outage));
  block.append(element("p", "post", excerpt(original ? outage.originalText : outage.text || outage.originalText, 420)));
  if (outage.recoveredAt) block.append(element("p", "muted", text.recovered(formatStamp(outage.recoveredAt))));
  if (outage.url) block.append(copyButton(text.copyPost, outage.url));
  return block;
}

function copyButton(label, url) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "text-button";
  button.textContent = label;
  button.addEventListener("click", () => copyLink(url));
  return button;
}

async function copyLink(url) {
  try {
    await navigator.clipboard.writeText(url);
    await axonhub.toast(text.copied, { kind: "success" });
  } catch {
    await axonhub.toast(url);
  }
}

function monthButton(delta, bounds, label) {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = label;
  const next = shiftMonth(month, delta);
  button.disabled = (delta < 0 && next < bounds.start) || (delta > 0 && next > bounds.end);
  button.addEventListener("click", () => {
    month = next;
    const state = memory.get(current);
    const parsed = parseSnapshot(state?.body || "");
    if (parsed.ok) paintSnapshot(parsed.snapshot);
  });
  return button;
}

function clockLine(snapshot) {
  const parts = [text.clock];
  const stamp = formatStamp(snapshot.monitor?.lastVerifiedAt || snapshot.checkedAt);
  const monitor = text.monitor[snapshot.monitor?.status] || text.monitor.unknown;
  if (stamp) parts.push(`${monitor} · ${text.checked(stamp)}`);
  const coverage = formatDay(shanghaiDay(snapshot.historyFrom), locale);
  if (coverage) parts.push(text.coverage(coverage));
  return parts.join(" · ");
}

function tagLabel(event) {
  if (isPending(event)) return text.waiting;
  if (event.type === "reset_credit") return text.card;
  return text.reset;
}

function paintTabs() {
  [...products.children].forEach((button, index) => {
    button.setAttribute("aria-selected", PRODUCTS[index].id === current ? "true" : "false");
    button.disabled = busy;
  });
  refresh.disabled = busy;
}

function setBusy(value, message) {
  busy = value;
  paintTabs();
  status.classList.toggle("busy", Boolean(message));
  if (message) status.textContent = message;
}

function showFailure(error) {
  setBusy(false, "");
  status.classList.remove("busy");
  status.textContent = error?.message || text.bridge("");
}

function element(tag, className, content) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (content) node.textContent = content;
  return node;
}
