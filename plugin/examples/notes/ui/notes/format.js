const copy = {
  en: {
    notes: "Notes",
    channels: "Channels",
    title: "Title",
    body: "Body",
    save: "Save note",
    delete: "Delete",
    openChannels: "Open channels",
    emptyNotes: "No notes yet.",
    emptyChannels: "No channels in this project.",
    writeHint: "This account cannot change plugin data. Saving needs write_plugins.",
    channelHint: "Names and status only. Credentials stay on the host.",
    channelDenied: "Channel list needs the plugin grant read_channels and the same permission on this user.",
    saved: "Note saved",
    deleted: "Note deleted",
    limit: "This page has reached its note limit.",
    loading: "Loading…",
    locale: "Language",
    project: "Project",
    page: "Page",
    writable: "Can write",
    yes: "Yes",
    no: "No",
    none: "None",
    count: (used, max) => `${used} / ${max}`,
  },
  zh: {
    notes: "笔记",
    channels: "渠道",
    title: "标题",
    body: "内容",
    save: "保存笔记",
    delete: "删除",
    openChannels: "打开渠道",
    emptyNotes: "还没有笔记。",
    emptyChannels: "当前项目没有渠道。",
    writeHint: "当前账号不能修改插件数据。保存需要 write_plugins。",
    channelHint: "这里只显示名称和状态，凭证留在宿主侧。",
    channelDenied: "渠道列表需要插件被授予 read_channels，并且当前用户也有该权限。",
    saved: "笔记已保存",
    deleted: "笔记已删除",
    limit: "笔记数量已达到上限。",
    loading: "正在加载…",
    locale: "语言",
    project: "项目",
    page: "页面",
    writable: "可写",
    yes: "是",
    no: "否",
    none: "无",
    count: (used, max) => `${used} / ${max}`,
  },
};

export function copyFor(locale) {
  return String(locale || "").toLowerCase().startsWith("zh") ? copy.zh : copy.en;
}

export function noteKey() {
  return `note.${crypto.randomUUID()}`;
}

export function parseNote(value) {
  try {
    const parsed = JSON.parse(value);
    if (parsed && typeof parsed.title === "string" && typeof parsed.body === "string") {
      return { title: parsed.title, body: parsed.body };
    }
  } catch {
    // Stored values are strings. Older or hand-written rows may not be JSON.
  }
  return { title: String(value ?? ""), body: "" };
}

export function formatTime(value, locale) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat(locale || undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function headingFrom(config, text) {
  const heading = config && typeof config.heading === "string" ? config.heading.trim() : "";
  return heading || text.notes;
}

export function maxNotesFrom(config) {
  const value = config && typeof config.maxNotes === "number" ? config.maxNotes : 20;
  if (!Number.isInteger(value)) {
    return 20;
  }
  return Math.min(50, Math.max(1, value));
}
