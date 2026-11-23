const copy = {
  en: {
    title: "Jev Guard",
    summary: (threshold, count, fail, keyMissing) => {
      const parts = [`Block at ${threshold}`, `${count} categories`, fail];
      if (keyMissing) {
        parts.push("API key not saved");
      }
      return parts.join(" · ");
    },
    empty: "No blocked requests yet.",
    refresh: "Refresh",
    clear: "Clear",
    remove: "Remove",
    cleared: "Block log cleared",
    removed: "Record removed",
    loading: "Loading…",
    count: (used, max) => `${used} / ${max}`,
    failClosed: "block when the check fails",
    failOpen: "allow when the check fails",
    blocked: "Blocked",
  },
  zh: {
    title: "请求审查",
    summary: (threshold, count, fail, keyMissing) => {
      const parts = [`拦截 ≥ ${threshold}`, `${count} 个分类`, fail];
      if (keyMissing) {
        parts.push("API Key 未保存");
      }
      return parts.join(" · ");
    },
    empty: "还没有拦截记录。",
    refresh: "刷新",
    clear: "清空",
    remove: "删除",
    cleared: "拦截记录已清空",
    removed: "已删除这条记录",
    loading: "加载中…",
    count: (used, max) => `${used} / ${max}`,
    failClosed: "检查失败时拦截",
    failOpen: "检查失败时放行",
    blocked: "拦截",
  },
};

const reasons = {
  en: {
    child_sexual: "sexual content involving a minor",
    sexual: "sexual content",
    jailbreak: "jailbreak",
    harmful: "violent or criminal assistance",
    fraud: "scam or phishing",
    hate: "harassment or hate",
    missing_api_key: "API key missing",
    fetch_failed: "Jev request failed",
    bad_json: "Jev response was not JSON",
    missing_answers: "Jev response had no answers",
    exception: "Hook error",
    unavailable: "safety check unavailable",
  },
  zh: {
    child_sexual: "涉未成年性内容",
    sexual: "色情",
    "sexual content involving a minor": "涉未成年性内容",
    "sexual content": "色情",
    jailbreak: "破限",
    harmful: "暴力犯罪",
    "violent or criminal assistance": "暴力犯罪",
    fraud: "诈骗",
    "scam or phishing": "诈骗",
    hate: "仇恨骚扰",
    "harassment or hate": "仇恨骚扰",
    missing_api_key: "未保存 API Key",
    fetch_failed: "Jev 请求失败",
    bad_json: "Jev 响应不是 JSON",
    missing_answers: "Jev 响应缺少答案",
    exception: "钩子异常",
    unavailable: "检查失败",
  },
};

export function copyFor(locale) {
  const name = String(locale || "").toLowerCase();
  if (name === "zh" || name.startsWith("zh-")) {
    return copy.zh;
  }
  return copy.en;
}

export function reasonText(reason, locale) {
  const name = String(locale || "").toLowerCase();
  const table = name === "zh" || name.startsWith("zh-") ? reasons.zh : reasons.en;
  if (reason && table[reason]) {
    return table[reason];
  }
  if (typeof reason === "string" && reason.startsWith("http_")) {
    return "HTTP " + reason.slice(5);
  }
  if (typeof reason === "string" && reason.startsWith("missing_")) {
    return reason;
  }
  return reason || "";
}

export function viewPolicy(config) {
  const src = config || {};
  const labels = categoryLabels(src.categories);
  return {
    block: numberOr(src.blockThreshold, 0.7),
    labels,
    failClosed: src.failClosed === true,
    keySet: src.apiKey === "******" || (typeof src.apiKey === "string" && src.apiKey !== "" && src.apiKey !== "******"),
    keep: wholeOr(src.keep, 20, 1, 40),
  };
}

export function categoryLabels(value) {
  if (!Array.isArray(value)) {
    return [];
  }
  const labels = [];
  for (let i = 0; i < value.length && labels.length < 32; i++) {
    if (typeof value[i] !== "string") {
      continue;
    }
    const name = value[i].split("|")[0].trim();
    if (name === "") {
      continue;
    }
    labels.push(name.length > 80 ? name.slice(0, 79) + "…" : name);
  }
  return labels;
}

export function parseRecord(value) {
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== "object" || parsed.action === "review") {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function recordScores(record, locale) {
  if (!record || typeof record !== "object") {
    return [];
  }
  if (Array.isArray(record.scores)) {
    return record.scores.filter((item) => item && typeof item.label === "string" && typeof item.probability === "number");
  }
  const hazards = record.hazards;
  if (!hazards || typeof hazards !== "object") {
    return [];
  }
  return Object.keys(hazards)
    .filter((id) => typeof hazards[id] === "number")
    .map((id) => ({ label: reasonText(id, locale), probability: hazards[id] }));
}

export function headingProbability(record, scores) {
  if (record && typeof record.probability === "number" && Number.isFinite(record.probability)) {
    return record.probability;
  }
  let best = null;
  for (let i = 0; i < scores.length; i++) {
    if (best === null || scores[i].probability > best) {
      best = scores[i].probability;
    }
  }
  return best;
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
  }).format(date);
}

export function percent(value) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "";
  }
  return `${Math.round(value * 1000) / 10}%`;
}

function numberOr(value, fallback) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function wholeOr(value, fallback, min, max) {
  if (typeof value !== "number" || !Number.isInteger(value)) {
    return fallback;
  }
  if (value < min) {
    return min;
  }
  if (value > max) {
    return max;
  }
  return value;
}
