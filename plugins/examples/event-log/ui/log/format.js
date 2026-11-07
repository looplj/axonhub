const copy = {
  en: {
    title: "Event log",
    hint: "Disable or enable one channel credential, then refresh. A completed playground request also appears here.",
    empty: "No events yet.",
    refresh: "Refresh",
    clear: "Clear",
    cleared: "Event log cleared",
    loading: "Loading…",
    count: (used, max) => `${used} / ${max}`,
    channel: "Channel",
    model: "Model",
    credential: "Credential",
  },
  zh: {
    title: "事件记录",
    hint: "在渠道里停用或启用一把凭证，然后刷新。Playground 里一次成功的请求也会出现在这里。",
    empty: "还没有事件。",
    refresh: "刷新",
    clear: "清空",
    cleared: "事件记录已清空",
    loading: "加载中…",
    count: (used, max) => `${used} / ${max}`,
    channel: "渠道",
    model: "模型",
    credential: "凭证",
  },
};

export function copyFor(locale) {
  const name = String(locale || "").toLowerCase();
  if (name === "zh" || name.startsWith("zh-")) {
    return copy.zh;
  }
  return copy.en;
}

export function keepFrom(config) {
  const value = config && typeof config.keep === "number" ? config.keep : 20;
  if (!Number.isInteger(value) || value < 1) {
    return 20;
  }
  return Math.min(value, 50);
}

export function parseRecord(value) {
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== "object") {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function formatWhen(value, locale) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || "";
  }
  return new Intl.DateTimeFormat(locale || "en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}
