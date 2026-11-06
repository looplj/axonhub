import { copyFor, formatWhen, keepFrom, parseRecord } from "./format.js";

const axonhub = window.axonhub;

const LIST_QUERY = `query EventLog($after: String) {
  pluginKVList(prefix: "event.", first: 100, after: $after) {
    nextCursor
    entries { key value updatedAt }
  }
}`;

const DELETE_QUERY = `mutation DeleteEvent($key: String!) {
  deletePluginKV(key: $key)
}`;

const title = document.querySelector("#title");
const hint = document.querySelector("#hint");
const status = document.querySelector("#status");
const count = document.querySelector("#count");
const refreshButton = document.querySelector("#refresh");
const clearButton = document.querySelector("#clear");
const list = document.querySelector("#events");
const empty = document.querySelector("#empty");

let text = copyFor("en");
let locale = "en";
let keep = 20;
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
  keep = keepFrom(init.config);
  canWrite = init.canWriteKV === true;
  title.textContent = text.title;
  hint.textContent = text.hint;
  refreshButton.textContent = text.refresh;
  clearButton.textContent = text.clear;
  clearButton.hidden = !canWrite;
  empty.textContent = text.empty;
  status.textContent = "";
  await refresh();
}

async function refresh() {
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
  items.sort((a, b) => String(b.key).localeCompare(String(a.key)));
  count.textContent = text.count(items.length, keep);
  list.replaceChildren();
  empty.hidden = items.length > 0;
  for (const item of items) {
    list.append(renderItem(item));
  }
}

function renderItem(item) {
  const record = parseRecord(item.value) || {};
  const row = document.createElement("li");
  const body = document.createElement("div");
  const heading = document.createElement("strong");
  heading.textContent = record.type || item.key;
  const when = document.createElement("time");
  when.dateTime = record.occurredAt || item.updatedAt || "";
  when.textContent = formatWhen(record.occurredAt || item.updatedAt, locale);
  const line = document.createElement("p");
  line.textContent = summaryLine(record);
  body.append(heading, when, line);
  row.append(body);
  return row;
}

function summaryLine(record) {
  const parts = [];
  if (record.channelName) {
    parts.push(text.channel + " " + record.channelName);
  }
  if (record.model) {
    parts.push(text.model + " " + record.model);
  }
  if (record.credential) {
    parts.push(text.credential + " " + record.credential);
  }
  if (record.detail) {
    parts.push(record.detail);
  }
  return parts.join(" · ");
}

async function clearLog() {
  if (!canWrite || busy) {
    return;
  }
  busy = true;
  clearButton.disabled = true;
  try {
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
    for (const item of items) {
      const result = await axonhub.graphql(DELETE_QUERY, { key: item.key });
      const message = graphqlMessage(result);
      if (message) {
        throw new Error(message);
      }
    }
    const seq = await axonhub.graphql(DELETE_QUERY, { key: "seq" });
    const seqMessage = graphqlMessage(seq);
    if (seqMessage) {
      throw new Error(seqMessage);
    }
    await refresh();
    await axonhub.toast(text.cleared, { kind: "success" });
  } finally {
    busy = false;
    clearButton.disabled = false;
  }
}

function graphqlMessage(result) {
  return result?.errors?.[0]?.message || "";
}

function showStatus(error) {
  status.textContent = error instanceof Error ? error.message : String(error);
}
