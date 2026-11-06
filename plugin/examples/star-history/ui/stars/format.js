const PAGE_SIZE = 100;
const WALK_LIMIT = 400;
const WALK_CAP = 4;
const SAMPLE_CAP = 6;

export { PAGE_SIZE };

const copy = {
  en: {
    title: "Star history",
    lead: "Star history for looplj/axonhub. The browser does not connect to api.github.com; the host sends both requests.",
    chart: "Cumulative stars",
    loading: "Loading…",
    loadingRepo: "Reading the repository…",
    loadingPage: (current, total) => `Reading stargazers ${current} / ${total}…`,
    repo: "Repository",
    stars: "Stars",
    range: "Time range",
    url: "Address",
    full: "Every star in this range was read.",
    sampled: (count) => `Sampled ${count} pages of 100, including the first and last page.`,
    empty: "This repository has no stars in the pages that were read.",
    http: (status, body) => (body ? `GitHub returned HTTP ${status}: ${body}` : `GitHub returned HTTP ${status}.`),
    bridge: (code) => `Host request failed: ${code}`,
    badJSON: "GitHub returned a response that is not JSON.",
    badRepo: "GitHub did not return a repository name and star count.",
    none: "None",
    chartSummary: (stars, start, end) => `Cumulative stars, ${stars}, ${start} to ${end}`,
  },
  zh: {
    title: "Star 历史",
    lead: "looplj/axonhub 的 star 历史。浏览器不会连接 api.github.com，两次请求都由宿主代发。",
    chart: "累计 Star",
    loading: "正在加载…",
    loadingRepo: "正在读取仓库…",
    loadingPage: (current, total) => `正在读取 stargazers ${current} / ${total}…`,
    repo: "仓库",
    stars: "Star 数",
    range: "时间范围",
    url: "地址",
    full: "这个范围内的 star 已全部读取。",
    sampled: (count) => `按每页 100 条抽样了 ${count} 页，包含第一页和最后一页。`,
    empty: "读到的页面里还没有 star。",
    http: (status, body) => (body ? `GitHub 返回 HTTP ${status}：${body}` : `GitHub 返回 HTTP ${status}。`),
    bridge: (code) => `宿主请求失败：${code}`,
    badJSON: "GitHub 返回的内容不是 JSON。",
    badRepo: "GitHub 没有返回仓库名和 star 数。",
    none: "无",
    chartSummary: (stars, start, end) => `累计 Star，${stars}，${start} 至 ${end}`,
  },
};

export function copyFor(locale) {
  return String(locale || "").toLowerCase().startsWith("zh") ? copy.zh : copy.en;
}

export function excerpt(body) {
  const text = String(body || "").replace(/\s+/g, " ").trim();
  if (text.length <= 160) {
    return text;
  }
  return `${text.slice(0, 157)}…`;
}

export function formatCount(value, locale) {
  return new Intl.NumberFormat(locale || undefined).format(value);
}

export function formatDay(value, locale) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat(locale || undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(date);
}

// Counts of 400 or less walk from page 1. Larger counts take at most six
// pages spread across the range, always including page 1 and the last page.
export function planHistory(total) {
  const count = Number.isFinite(total) ? Math.max(0, Math.floor(total)) : 0;
  const lastPage = Math.max(1, Math.ceil(count / PAGE_SIZE));
  if (count <= WALK_LIMIT) {
    return { mode: "walk", cap: Math.min(WALK_CAP, lastPage), sampled: false };
  }
  const pages = spreadPages(lastPage, SAMPLE_CAP);
  return { mode: "sample", pages, sampled: pages.length < lastPage };
}

export function spreadPages(lastPage, count) {
  const last = Math.max(1, Math.floor(lastPage));
  const want = Math.max(1, Math.min(count, last));
  if (last <= want) {
    return Array.from({ length: last }, (_, index) => index + 1);
  }
  const used = new Set([1, last]);
  const inner = want - 2;
  for (let i = 1; i <= inner; i += 1) {
    let page = Math.round(1 + (i * (last - 1)) / (want - 1));
    page = Math.min(last - 1, Math.max(2, page));
    if (used.has(page)) {
      page = nearestUnused(page, used, last);
    }
    if (page > 1 && page < last) {
      used.add(page);
    }
  }
  return [...used].sort((a, b) => a - b);
}

function nearestUnused(page, used, lastPage) {
  for (let distance = 1; distance < lastPage; distance += 1) {
    const lower = page - distance;
    const upper = page + distance;
    if (lower > 1 && lower < lastPage && !used.has(lower)) {
      return lower;
    }
    if (upper > 1 && upper < lastPage && !used.has(upper)) {
      return upper;
    }
  }
  return page;
}

export function pointsFromPages(pageResults) {
  const points = [];
  for (const result of pageResults || []) {
    const page = Number(result?.page);
    const items = Array.isArray(result?.items) ? result.items : [];
    if (!Number.isInteger(page) || page < 1) {
      continue;
    }
    items.forEach((item, index) => {
      const time = Date.parse(item && item.starred_at);
      if (Number.isNaN(time)) {
        return;
      }
      points.push({ t: time, y: (page - 1) * PAGE_SIZE + index + 1 });
    });
  }
  points.sort((a, b) => a.t - b.t || a.y - b.y);
  return points;
}

export function hasNextLink(headers) {
  const link = headerValue(headers, "link");
  if (!link) {
    return false;
  }
  return link.split(",").some((part) => /;\s*rel="?next"?/i.test(part));
}

function headerValue(headers, name) {
  if (!headers || typeof headers !== "object") {
    return "";
  }
  const wanted = name.toLowerCase();
  for (const [key, value] of Object.entries(headers)) {
    if (String(key).toLowerCase() === wanted) {
      return String(value ?? "");
    }
  }
  return "";
}
