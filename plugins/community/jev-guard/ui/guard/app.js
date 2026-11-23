import { copyFor, formatWhen, headingProbability, parseRecord, percent, reasonText, recordScores, viewPolicy } from "./format.js";

const axonhub = window.axonhub;

const LIST_QUERY = `query JevGuard($after: String) {
  pluginKVList(prefix: "d.", first: 100, after: $after) {
    nextCursor
    entries { key value updatedAt }
  }
}`;

const DELETE_QUERY = `mutation DeleteJevGuard($key: String!) {
  deletePluginKV(key: $key)
}`;

const title = document.querySelector("#title");
const summary = document.querySelector("#summary");
const status = document.querySelector("#status");
const count = document.querySelector("#count");
const refreshButton = document.querySelector("#refresh");
const clearButton = document.querySelector("#clear");
const list = document.querySelector("#records");
const empty = document.querySelector("#empty");

let text = copyFor("en");
let locale = "en";
let keep = 20;
let threshold = 0.7;
let canWrite = false;
let busy = false;

if (!axonhub) {
  status.textContent = "AxonHub bridge did not load";
} else {
  refreshButton.addEventListener("click", () => {
    refresh().catch(showStatus);
  });
  clearButton.addEventListener("click", () => {
    clearLog().catch(showStatus);
  });
  load().catch(showStatus);
}

async function load() {
  status.textContent = copyFor("en").loading;
  const init = await axonhub.ready();
  locale = init.locale || "en";
  text = copyFor(locale);
  document.documentElement.lang = locale;
  const current = viewPolicy(init.config);
  keep = current.keep;
  threshold = current.block;
  canWrite = init.canWriteKV === true;
  title.textContent = text.title;
  summary.textContent = text.summary(
    percent(current.block),
    current.labels.length,
    current.failClosed ? text.failClosed : text.failOpen,
    !current.keySet,
  );
  refreshButton.textContent = text.refresh;
  clearButton.textContent = text.clear;
  clearButton.hidden = !canWrite;
  empty.textContent = text.empty;
  status.textContent = "";
  await refresh();
}

async function refresh() {
  const items = await listRecords();
  const visible = items.filter((item) => parseRecord(item.value));
  visible.sort((a, b) => String(b.key).localeCompare(String(a.key)));
  count.textContent = visible.length === 0 ? "" : text.count(visible.length, keep);
  list.replaceChildren();
  empty.hidden = visible.length > 0;
  for (const item of visible) {
    list.append(renderItem(item));
  }
}

function renderItem(item) {
  const record = parseRecord(item.value) || {};
  const scores = recordScores(record, locale).sort((a, b) => b.probability - a.probability);
  const row = document.createElement("li");
  const head = document.createElement("div");
  head.className = "row-top";
  const reason = document.createElement("strong");
  reason.className = "reason";
  reason.textContent = reasonText(record.reason, locale) || text.blocked;
  head.append(reason);
  const probability = headingProbability(record, scores);
  if (typeof probability === "number") {
    const mark = document.createElement("span");
    mark.className = "prob";
    mark.textContent = percent(probability);
    head.append(mark);
  }
  const when = document.createElement("time");
  when.dateTime = record.at || item.updatedAt || "";
  when.textContent = formatWhen(record.at || item.updatedAt, locale);
  head.append(when);
  row.append(head);

  if (record.excerpt) {
    const quote = document.createElement("p");
    quote.className = "excerpt";
    quote.textContent = record.excerpt;
    row.append(quote);
  }

  const bits = [];
  if (record.requestType) {
    bits.push(record.requestType);
  }
  if (record.model) {
    bits.push(record.model);
  }
  if (record.apiFormat) {
    bits.push(record.apiFormat);
  }
  if (bits.length > 0) {
    const meta = document.createElement("p");
    meta.className = "meta";
    meta.textContent = bits.join(" · ");
    row.append(meta);
  }

  const others = scores.filter(
    (score) => score.probability >= threshold && score.label !== record.reason && score.label !== reason.textContent,
  );
  if (others.length > 0) {
    const chips = document.createElement("ul");
    chips.className = "chips";
    for (const score of others) {
      const chip = document.createElement("li");
      chip.textContent = `${score.label} ${percent(score.probability)}`;
      chips.append(chip);
    }
    row.append(chips);
  }

  if (canWrite && item.key) {
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "remove";
    remove.textContent = text.remove;
    remove.addEventListener("click", () => {
      removeRecord(item.key).catch(showStatus);
    });
    row.append(remove);
  }
  return row;
}

async function listRecords() {
  const items = [];
  let after = null;
  do {
    const result = await axonhub.graphql(LIST_QUERY, { after });
    const message = graphqlMessage(result);
    if (message) {
      throw new Error(message);
    }
    const page = result.data?.pluginKVList;
    items.push(...(page?.entries ?? []));
    after = page?.nextCursor ?? null;
  } while (after !== null);
  return items;
}

async function removeRecord(key) {
  if (!canWrite || busy) {
    return;
  }
  busy = true;
  try {
    await deleteKey(key);
    await refresh();
    await axonhub.toast(text.removed, { kind: "success" });
  } finally {
    busy = false;
  }
}

async function clearLog() {
  if (!canWrite || busy) {
    return;
  }
  busy = true;
  clearButton.disabled = true;
  try {
    const items = await listRecords();
    for (const item of items) {
      await deleteKey(item.key);
    }
    await deleteKey("seq");
    await refresh();
    await axonhub.toast(text.cleared, { kind: "success" });
  } finally {
    busy = false;
    clearButton.disabled = false;
  }
}

async function deleteKey(key) {
  const result = await axonhub.graphql(DELETE_QUERY, { key });
  const message = graphqlMessage(result);
  if (message) {
    throw new Error(message);
  }
}

function graphqlMessage(result) {
  return result?.errors?.[0]?.message || "";
}

function showStatus(error) {
  status.textContent = error instanceof Error ? error.message : String(error);
}
