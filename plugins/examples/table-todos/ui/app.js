const copy = {
  zh: { title: "待办清单", input: "待办标题", placeholder: "输入待办事项", add: "添加", remove: "删除", more: "加载更多", empty: "还没有待办。", loading: "正在读取…", denied: "需要插件管理权限才能查看和编辑待办。", missing: "这条待办已被删除。", bridge: "无法连接插件页面桥接。" },
  en: { title: "To-do list", input: "To-do title", placeholder: "Enter a to-do", add: "Add", remove: "Delete", more: "Load more", empty: "No to-dos yet.", loading: "Loading…", denied: "Plugin management permission is required to view and edit this list.", missing: "This to-do was already deleted.", bridge: "The plugin page bridge is unavailable." },
};

const form = document.querySelector("#add-form");
const input = document.querySelector("#new-title");
const addButton = document.querySelector("#add-button");
const status = document.querySelector("#status");
const empty = document.querySelector("#empty");
const list = document.querySelector("#todos");
const more = document.querySelector("#more");

let messages = copy.zh;
let todos;
let rows = [];
let cursor = null;
let pending = false;
let loaded = false;

async function load(reset = true) {
  const options = { first: 30, orderBy: [{ column: "id", direction: "desc" }] };
  if (!reset && cursor) options.after = cursor;
  const page = await todos.list(options);
  rows = reset ? page.rows : [...rows, ...page.rows];
  cursor = page.nextCursor;
  loaded = true;
}

function render() {
  input.disabled = pending;
  addButton.disabled = pending;
  more.disabled = pending;
  more.hidden = cursor == null;
  empty.hidden = pending || !loaded || rows.length > 0;
  list.setAttribute("aria-busy", String(pending));
  list.replaceChildren(...rows.map(renderRow));
}

function renderRow(row) {
  const item = document.createElement("li");
  const label = document.createElement("label");
  const checkbox = document.createElement("input");
  checkbox.type = "checkbox";
  checkbox.checked = row.done;
  checkbox.disabled = pending;
  checkbox.addEventListener("change", () => {
    const done = checkbox.checked;
    void run(async () => {
      const updated = await todos.update(row.id, { done });
      if (updated == null) {
        await load();
        throw new Error(messages.missing);
      }
      rows = rows.map((candidate) => candidate.id === row.id ? updated : candidate);
    });
  });
  const title = document.createElement("span");
  title.textContent = row.title;
  label.append(checkbox, title);

  const remove = document.createElement("button");
  remove.type = "button";
  remove.textContent = messages.remove;
  remove.setAttribute("aria-label", messages.remove + " " + row.title);
  remove.disabled = pending;
  remove.addEventListener("click", () => {
    void run(async () => {
      await todos.delete(row.id);
      await load();
    });
  });
  item.append(label, remove);
  return item;
}

async function run(work) {
  if (pending) return;
  pending = true;
  status.textContent = messages.loading;
  render();
  try {
    await work();
    status.textContent = "";
  } catch (error) {
    status.textContent = error.message;
  } finally {
    pending = false;
    render();
  }
}

async function start() {
  if (!window.axonhub) throw new Error(messages.bridge);
  const { locale, canWriteKV } = await window.axonhub.ready();
  messages = locale?.startsWith("zh") ? copy.zh : copy.en;
  document.documentElement.lang = locale?.startsWith("zh") ? "zh-CN" : "en";
  document.title = messages.title;
  document.querySelector("#title").textContent = messages.title;
  input.setAttribute("aria-label", messages.input);
  input.placeholder = messages.placeholder;
  addButton.textContent = messages.add;
  more.textContent = messages.more;
  empty.textContent = messages.empty;
  if (!canWriteKV) {
    status.textContent = messages.denied;
    return;
  }

  todos = window.axonhub.tables.open("todos");
  form.hidden = false;
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const title = input.value.trim();
    if (!title) return;
    void run(async () => {
      await todos.insert({ title, done: false });
      input.value = "";
      await load();
    });
  });
  more.addEventListener("click", () => void run(() => load(false)));
  await run(load);
}

start().catch((error) => { status.textContent = error.message; });
