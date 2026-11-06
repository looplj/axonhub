import { copyFor, formatTime, headingFrom, maxNotesFrom, noteKey, parseNote } from "./format.js";

const axonhub = window.axonhub;

const NOTES_QUERY = `query Notes($after: String) {
  pluginKVList(prefix: "note.", first: 100, after: $after) {
    nextCursor
    entries {
      key
      value
      updatedAt
    }
  }
}`;

const SAVE_NOTE = `mutation SaveNote($key: String!, $value: String!) {
  setPluginKV(key: $key, value: $value) {
    key
    updatedAt
  }
}`;

const DELETE_NOTE = `mutation DeleteNote($key: String!) {
  deletePluginKV(key: $key)
}`;

const CHANNELS_QUERY = `query Channels {
  channels(first: 20) {
    edges {
      node {
        name
        status
        type
      }
    }
  }
}`;

const heading = document.querySelector("#heading");
const meta = document.querySelector("#meta");
const status = document.querySelector("#status");
const notesTitle = document.querySelector("#notes-title");
const notesCount = document.querySelector("#notes-count");
const form = document.querySelector("#note-form");
const titleInput = document.querySelector("#title");
const bodyInput = document.querySelector("#body");
const titleLabel = document.querySelector("#title-label");
const bodyLabel = document.querySelector("#body-label");
const saveButton = document.querySelector("#save");
const writeHint = document.querySelector("#write-hint");
const notes = document.querySelector("#notes");
const notesEmpty = document.querySelector("#notes-empty");
const channelsTitle = document.querySelector("#channels-title");
const channelsHint = document.querySelector("#channels-hint");
const channels = document.querySelector("#channels");
const openChannels = document.querySelector("#open-channels");

let text = copyFor("en");
let locale = "en";
let limit = 20;
let canWrite = false;
let writing = false;

if (!axonhub) {
  status.textContent = "AxonHub bridge did not load";
} else {
  openChannels.addEventListener("click", () => {
    axonhub.navigate("/channels").catch(showStatus);
  });

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    saveNote().catch(showStatus);
  });

  load().catch(showStatus);
}

async function load() {
  status.textContent = copyFor("en").loading;
  const init = await axonhub.ready();
  locale = init.locale || "en";
  text = copyFor(locale);
  document.documentElement.lang = locale;
  limit = maxNotesFrom(init.config);
  canWrite = init.canWriteKV === true;
  heading.textContent = headingFrom(init.config, text);
  applyCopy();
  renderMeta(init);
  status.textContent = "";
  await Promise.all([refreshNotes(), refreshChannels()]);
}

function applyCopy() {
  notesTitle.textContent = text.notes;
  channelsTitle.textContent = text.channels;
  titleLabel.textContent = text.title;
  bodyLabel.textContent = text.body;
  saveButton.textContent = text.save;
  writeHint.textContent = text.writeHint;
  notesEmpty.textContent = text.emptyNotes;
  openChannels.textContent = text.openChannels;
  titleInput.disabled = !canWrite;
  bodyInput.disabled = !canWrite;
  saveButton.disabled = !canWrite;
  writeHint.hidden = canWrite;
}

function renderMeta(init) {
  const rows = [
    [text.locale, locale],
    [text.project, init.projectId == null ? text.none : String(init.projectId)],
    [text.page, init.page || text.none],
    [text.writable, canWrite ? text.yes : text.no],
  ];
  meta.replaceChildren();
  for (const [name, value] of rows) {
    const term = document.createElement("dt");
    term.textContent = name;
    const detail = document.createElement("dd");
    detail.textContent = value;
    meta.append(term, detail);
  }
}

async function refreshNotes() {
  const items = [];
  let after = null;
  do {
    const result = await axonhub.graphql(NOTES_QUERY, { after });
    const message = graphqlMessage(result);
    if (message) {
      throw new Error(message);
    }
    const page = result.data?.pluginKVList;
    items.push(...(page?.entries ?? []));
    after = page?.nextCursor ?? null;
  } while (after !== null);
  items.sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)));
  notesCount.textContent = text.count(items.length, limit);
  notes.replaceChildren();
  notesEmpty.hidden = items.length > 0;
  for (const item of items) {
    notes.append(renderNote(item));
  }
}

function renderNote(item) {
  const parsed = parseNote(item.value);
  const row = document.createElement("li");
  const body = document.createElement("div");
  const title = document.createElement("strong");
  title.textContent = parsed.title;
  const time = document.createElement("time");
  time.dateTime = item.updatedAt || "";
  time.textContent = formatTime(item.updatedAt, locale);
  const copy = document.createElement("p");
  copy.textContent = parsed.body;
  body.append(title, time, copy);

  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary";
  button.textContent = text.delete;
  button.disabled = !canWrite;
  button.addEventListener("click", () => {
    deleteNote(item.key).catch(showStatus);
  });
  row.append(body, button);
  return row;
}

async function saveNote() {
  const title = titleInput.value.trim();
  const body = bodyInput.value.trim();
  if (!title || !body || !canWrite || writing) {
    return;
  }
  const current = notes.querySelectorAll("li").length;
  if (current >= limit) {
    await axonhub.toast(text.limit, { kind: "warning" });
    return;
  }
  writing = true;
  saveButton.disabled = true;
  try {
    const result = await axonhub.graphql(SAVE_NOTE, {
      key: noteKey(),
      value: JSON.stringify({ title, body }),
    });
    const message = graphqlMessage(result);
    if (message) {
      throw new Error(message);
    }
    form.reset();
    await refreshNotes();
    await axonhub.toast(text.saved, { kind: "success" });
  } finally {
    writing = false;
    saveButton.disabled = !canWrite;
  }
}

async function deleteNote(key) {
  if (!canWrite) {
    return;
  }
  const result = await axonhub.graphql(DELETE_NOTE, { key });
  const message = graphqlMessage(result);
  if (message) {
    throw new Error(message);
  }
  await refreshNotes();
  await axonhub.toast(text.deleted, { kind: "success" });
}

async function refreshChannels() {
  const result = await axonhub.graphql(CHANNELS_QUERY);
  channels.replaceChildren();
  if (graphqlCode(result) === "FORBIDDEN") {
    channelsHint.textContent = text.channelDenied;
    return;
  }
  const message = graphqlMessage(result);
  if (message) {
    channelsHint.textContent = message;
    return;
  }
  const edges = result.data?.channels?.edges ?? [];
  channelsHint.textContent = edges.length === 0 ? text.emptyChannels : text.channelHint;
  for (const edge of edges) {
    const node = edge?.node;
    if (!node) {
      continue;
    }
    const row = document.createElement("li");
    const name = document.createElement("strong");
    name.textContent = node.name || "";
    const detail = document.createElement("span");
    detail.textContent = [node.type, node.status].filter(Boolean).join(" · ");
    row.append(name, detail);
    channels.append(row);
  }
}

function graphqlCode(result) {
  return result?.errors?.[0]?.extensions?.code || "";
}

function graphqlMessage(result) {
  return result?.errors?.[0]?.message || "";
}

function showStatus(error) {
  status.textContent = error instanceof Error ? error.message : String(error);
}
