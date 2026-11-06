import {
  PAGE_SIZE,
  copyFor,
  excerpt,
  formatCount,
  formatDay,
  hasNextLink,
  planHistory,
  pointsFromPages,
} from "./format.js";

const axonhub = window.axonhub;

// Query strings are not part of manifest.network, so pagination stays on these two URLs.
const REPO_URL = "https://api.github.com/repos/looplj/axonhub";
const STARGAZERS_URL = "https://api.github.com/repos/looplj/axonhub/stargazers";
const REPO_ACCEPT = "application/vnd.github+json";
const STAR_ACCEPT = "application/vnd.github.star+json";

const heading = document.querySelector("#heading");
const lead = document.querySelector("#lead");
const meta = document.querySelector("#meta");
const status = document.querySelector("#status");
const chartTitle = document.querySelector("#chart-title");
const chartNote = document.querySelector("#chart-note");
const chart = document.querySelector("#chart");

let text = copyFor("en");
let locale = "en";

if (!axonhub) {
  applyCopy();
  status.textContent = "AxonHub bridge did not load";
} else {
  load().catch(showStatus);
}

async function load() {
  setStatus(text.loading, true);
  const init = await axonhub.ready();
  locale = init.locale || "en";
  document.documentElement.lang = locale;
  text = copyFor(document.documentElement.lang);
  applyCopy();
  setStatus(text.loadingRepo, true);

  const repoResponse = await readJSON(REPO_URL, REPO_ACCEPT);
  const repo = repoFrom(repoResponse.data);
  renderMeta(repo, null);

  const history = await readHistory(repo.stars);
  const points = pointsFromPages(history.pages);
  renderMeta(repo, points);
  drawChart(points, repo.stars);
  chartNote.textContent = noteFor(repo.stars, history, points);
  setStatus("", true);
}

function applyCopy() {
  heading.textContent = text.title;
  lead.textContent = text.lead;
  chartTitle.textContent = text.chart;
}

function noteFor(stars, history, points) {
  if (stars === 0 || points.length === 0) {
    return text.empty;
  }
  if (history.sampled) {
    return text.sampled(history.pages.length);
  }
  return text.full;
}

async function readHistory(stars) {
  const plan = planHistory(stars);
  if (plan.mode === "walk") {
    const pages = [];
    for (let page = 1; page <= plan.cap; page += 1) {
      setStatus(text.loadingPage(page, plan.cap), true);
      const result = await readStargazers(page);
      pages.push(result);
      if (result.items.length < PAGE_SIZE || !result.hasNext) {
        break;
      }
    }
    return { pages, sampled: false };
  }

  const pages = [];
  for (let index = 0; index < plan.pages.length; index += 1) {
    setStatus(text.loadingPage(index + 1, plan.pages.length), true);
    pages.push(await readStargazers(plan.pages[index]));
  }
  return { pages, sampled: plan.sampled };
}

async function readStargazers(page) {
  const response = await readJSON(`${STARGAZERS_URL}?per_page=100&page=${page}`, STAR_ACCEPT);
  if (!Array.isArray(response.data)) {
    throw new Error(text.badJSON);
  }
  return {
    page,
    items: response.data,
    hasNext: hasNextLink(response.headers),
  };
}

async function readJSON(url, accept) {
  const result = await axonhub.fetch(url, {
    method: "GET",
    headers: {
      Accept: accept,
      "User-Agent": "axonhub-example-star-history",
    },
  });
  const statusCode = result && Number.isFinite(result.status) ? result.status : 0;
  if (statusCode < 200 || statusCode >= 300) {
    const body = result && typeof result.body === "string" ? result.body : "";
    throw new Error(text.http(statusCode, excerpt(body)));
  }
  const body = result && typeof result.body === "string" ? result.body : "";
  if (!body) {
    return { data: null, headers: headerMap(result) };
  }
  try {
    return { data: JSON.parse(body), headers: headerMap(result) };
  } catch {
    throw new Error(text.badJSON);
  }
}

function headerMap(result) {
  const headers = result && result.headers;
  return headers && typeof headers === "object" ? headers : {};
}

function repoFrom(data) {
  const fullName = data && typeof data.full_name === "string" ? data.full_name : "";
  const htmlURL = data && typeof data.html_url === "string" ? data.html_url : "";
  const stars = data && typeof data.stargazers_count === "number" ? data.stargazers_count : NaN;
  if (!fullName || !Number.isInteger(stars) || stars < 0) {
    throw new Error(text.badRepo);
  }
  return { fullName, htmlURL, stars };
}

function renderMeta(repo, points) {
  const range = timeRange(points);
  const rows = [
    [text.repo, repo.fullName],
    [text.stars, formatCount(repo.stars, locale)],
    [text.range, range || text.none],
    [text.url, repo.htmlURL || text.none],
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

function timeRange(points) {
  if (!points || points.length === 0) {
    return "";
  }
  const start = formatDay(points[0].t, locale);
  const end = formatDay(points[points.length - 1].t, locale);
  if (!start || !end) {
    return "";
  }
  if (start === end) {
    return start;
  }
  return `${start} – ${end}`;
}

function drawChart(points, totalStars) {
  chart.replaceChildren();
  const width = 720;
  const height = 300;
  chart.setAttribute("viewBox", `0 0 ${width} ${height}`);
  if (!points.length) {
    chart.setAttribute("aria-label", text.empty);
    return;
  }

  const pad = { top: 16, right: 12, bottom: 28, left: 64 };
  const innerW = width - pad.left - pad.right;
  const innerH = height - pad.top - pad.bottom;
  const minT = points[0].t;
  const maxT = points[points.length - 1].t;
  let maxY = Math.max(totalStars || 0, 1);
  for (const point of points) {
    if (point.y > maxY) {
      maxY = point.y;
    }
  }
  const xOf = (time) => {
    if (maxT === minT) {
      return pad.left + innerW / 2;
    }
    return pad.left + ((time - minT) / (maxT - minT)) * innerW;
  };
  const yOf = (count) => pad.top + innerH - (count / maxY) * innerH;
  const coords = points.map((point) => [xOf(point.t), yOf(point.y)]);

  for (const fraction of [0, 0.5, 1]) {
    const value = maxY * fraction;
    const y = yOf(value);
    chart.append(svgEl("line", {
      class: "grid",
      x1: pad.left,
      y1: y,
      x2: width - pad.right,
      y2: y,
    }));
    const label = svgEl("text", { x: pad.left - 8, y: y + 4, "text-anchor": "end" });
    label.textContent = formatCount(Math.round(value), locale);
    chart.append(label);
  }

  const baseline = yOf(0);
  const area = [
    `M ${round(coords[0][0])} ${round(baseline)}`,
    ...coords.map(([x, y]) => `L ${round(x)} ${round(y)}`),
    `L ${round(coords[coords.length - 1][0])} ${round(baseline)}`,
    "Z",
  ].join(" ");
  chart.append(svgEl("path", { class: "area", d: area }));
  chart.append(svgEl("polyline", {
    class: "line",
    points: coords.map(([x, y]) => `${round(x)},${round(y)}`).join(" "),
  }));
  if (coords.length === 1) {
    chart.append(svgEl("circle", {
      class: "dot",
      cx: coords[0][0],
      cy: coords[0][1],
      r: 3.5,
    }));
  }

  const startLabel = svgEl("text", { x: pad.left, y: height - 8 });
  startLabel.textContent = formatDay(minT, locale);
  const endLabel = svgEl("text", {
    x: width - pad.right,
    y: height - 8,
    "text-anchor": "end",
  });
  endLabel.textContent = formatDay(maxT, locale);
  chart.append(startLabel, endLabel);
  chart.setAttribute(
    "aria-label",
    text.chartSummary(formatCount(totalStars, locale), startLabel.textContent, endLabel.textContent),
  );
}

function svgEl(name, attrs) {
  const node = document.createElementNS("http://www.w3.org/2000/svg", name);
  for (const [key, value] of Object.entries(attrs)) {
    node.setAttribute(key, String(value));
  }
  return node;
}

function round(value) {
  return Math.round(value * 10) / 10;
}

function showStatus(error) {
  const code = error && typeof error === "object" && typeof error.code === "string" ? error.code : "";
  const message = code ? text.bridge(code) : error instanceof Error ? error.message : String(error);
  setStatus(message, false);
}

function setStatus(message, busy) {
  status.textContent = message;
  status.classList.toggle("busy", Boolean(busy) && message !== "");
}
