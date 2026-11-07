const copy = {
  en: {
    title: "Star history",
    lead: "Current stars for looplj/axonhub, and the chart from star-history.com. The browser does not connect to those hosts; the host sends both requests.",
    loading: "Loading…",
    loadingRepo: "Reading the repository…",
    loadingChart: "Reading the star history chart…",
    repo: "Repository",
    stars: "Stars",
    url: "Address",
    chartAlt: "Star history chart for looplj/axonhub",
    http: (name, status, body) => (body ? `${name} returned HTTP ${status}: ${body}` : `${name} returned HTTP ${status}.`),
    bridge: (code) => `Host request failed: ${code}`,
    badJSON: "GitHub returned a response that is not JSON.",
    badRepo: "GitHub did not return a repository name and star count.",
    badChart: "star-history returned a response that is not an SVG chart.",
    none: "None",
  },
  zh: {
    title: "Star 历史",
    lead: "looplj/axonhub 当前的 star 数，以及 star-history.com 的历史图。浏览器不会连接这些地址，两次请求都由宿主代发。",
    loading: "正在加载…",
    loadingRepo: "正在读取仓库…",
    loadingChart: "正在读取 star 历史图…",
    repo: "仓库",
    stars: "Star 数",
    url: "地址",
    chartAlt: "looplj/axonhub 的 star 历史图",
    http: (name, status, body) => (body ? `${name} 返回 HTTP ${status}：${body}` : `${name} 返回 HTTP ${status}。`),
    bridge: (code) => `宿主请求失败：${code}`,
    badJSON: "GitHub 返回的内容不是 JSON。",
    badRepo: "GitHub 没有返回仓库名和 star 数。",
    badChart: "star-history 返回的内容不是 SVG 图。",
    none: "无",
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

export function chartURL(theme) {
  const query = new URLSearchParams({
    repos: "looplj/axonhub",
    type: "date",
    legend: "top-left",
  });
  if (theme === "dark") {
    query.set("theme", "dark");
  }
  return `https://api.star-history.com/chart?${query.toString()}`;
}

export function svgDataURL(svg) {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}
