import { categoriesFrom, copyFor, formatTime, headingFrom, maxNotesFrom, noteKey, parseNote } from "./format.js";

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
const categoryField = document.querySelector("#category-field");
const categoryLabel = document.querySelector("#category-label");
const categorySelect = document.querySelector("#category");
const filters = document.querySelector("#filters");
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
let categories = [];
// "" shows every note.
let activeCategory = "";
let noteItems = [];

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
  categories = categoriesFrom(init.config);
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
  categoryLabel.textContent = text.category;
  categoryField.hidden = categories.length === 0;
  categorySelect.disabled = !canWrite;
  categorySelect.replaceChildren(
    ...categories.map((name) => {
      const option = document.createElement("option");
      option.value = name;
      option.textContent = name;
      return option;
    }),
  );
  filters.setAttribute("aria-label", text.filterLabel);
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
  noteItems = items.map((item) => ({ ...item, note: parseNote(item.value) }));
  // The limit counts every stored note, not only the ones the filter shows.
  notesCount.textContent = text.count(noteItems.length, limit);
  renderNotes();
}

function renderNotes() {
  renderFilters();
  const visible = activeCategory === "" ? noteItems : noteItems.filter((item) => item.note.category === activeCategory);
  notes.replaceChildren(...visible.map(renderNote));
  notesEmpty.hidden = visible.length > 0;
  notesEmpty.textContent = noteItems.length > 0 ? text.emptyCategory : text.emptyNotes;
}

function renderFilters() {
  filters.hidden = categories.length === 0;
  if (categories.length === 0) {
    activeCategory = "";
    return;
  }
  if (activeCategory !== "" && !categories.includes(activeCategory)) {
    activeCategory = "";
  }
  const counts = new Map();
  for (const item of noteItems) {
    counts.set(item.note.category, (counts.get(item.note.category) ?? 0) + 1);
  }
  const chips = [["", text.allCategories, noteItems.length], ...categories.map((name) => [name, name, counts.get(name) ?? 0])];
  filters.replaceChildren(
    ...chips.map(([value, label, count]) => {
      const chip = document.createElement("button");
      chip.type = "button";
      chip.className = "chip";
      chip.setAttribute("aria-pressed", String(value === activeCategory));
      chip.textContent = label;
      const badge = document.createElement("span");
      badge.textContent = String(count);
      chip.append(badge);
      chip.addEventListener("click", () => {
        activeCategory = value;
        renderNotes();
      });
      return chip;
    }),
  );
}

function renderNote(item) {
  const parsed = item.note;
  const row = document.createElement("li");
  const body = document.createElement("div");
  const title = document.createElement("strong");
  title.textContent = parsed.title;
  const time = document.createElement("time");
  time.dateTime = item.updatedAt || "";
  time.textContent = formatTime(item.updatedAt, locale);
  const copy = document.createElement("p");
  copy.textContent = parsed.body;
  body.append(title);
  // A category removed from config still shows on notes filed under it.
  if (parsed.category) {
    const tag = document.createElement("span");
    tag.className = "tag";
    tag.textContent = parsed.category;
    body.append(tag);
  }
  body.append(time, copy);

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
  const category = categories.length > 0 ? categorySelect.value : "";
  if (!title || !body || !canWrite || writing) {
    return;
  }
  const current = noteItems.length;
  if (current >= limit) {
    await axonhub.toast(text.limit, { kind: "warning" });
    return;
  }
  writing = true;
  saveButton.disabled = true;
  try {
    const result = await axonhub.graphql(SAVE_NOTE, {
      key: noteKey(),
      value: JSON.stringify(category ? { title, body, category } : { title, body }),
    });
    const message = graphqlMessage(result);
    if (message) {
      throw new Error(message);
    }
    form.reset();
    // Keep the chosen category for the next note and show the one just saved.
    categorySelect.value = category;
    if (activeCategory !== "" && activeCategory !== category) {
      activeCategory = "";
    }
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
