import { chartURL, copyFor, excerpt, formatCount, svgDataURL } from "./format.js";

const axonhub = window.axonhub;

const REPO_URL = "https://api.github.com/repos/looplj/axonhub";
const REPO_ACCEPT = "application/vnd.github+json";
const CHART_ACCEPT = "image/svg+xml";

const heading = document.querySelector("#heading");
const lead = document.querySelector("#lead");
const meta = document.querySelector("#meta");
const status = document.querySelector("#status");
const chartCard = document.querySelector("#chart-card");
const chart = document.querySelector("#chart");

let text = copyFor("en");
let locale = "en";
let chartTheme = "light";
let chartToken = 0;

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
  chartTheme = themeName(init);
  axonhub.onTheme((next) => {
    const theme = themeName(next);
    if (theme === chartTheme) {
      return;
    }
    chartTheme = theme;
    showChart(theme).then(finishChart).catch(showStatus);
  });

  let failure = null;
  let token = 0;
  try {
    await loadRepo();
  } catch (error) {
    failure = error;
  }
  try {
    token = await showChart(chartTheme);
  } catch (error) {
    failure = failure || error;
  }
  if (failure) {
    throw failure;
  }
  finishChart(token);
}

function finishChart(token) {
  if (token === chartToken) {
    setStatus("", true);
  }
}

function applyCopy() {
  heading.textContent = text.title;
  lead.textContent = text.lead;
  if (chart.getAttribute("src")) {
    chart.alt = text.chartAlt;
  }
}

async function loadRepo() {
  setStatus(text.loadingRepo, true);
  const repoResponse = await readBody(REPO_URL, REPO_ACCEPT, "GitHub");
  let data = null;
  if (repoResponse.body) {
    try {
      data = JSON.parse(repoResponse.body);
    } catch {
      throw new Error(text.badJSON);
    }
  }
  renderMeta(repoFrom(data));
}

async function showChart(theme) {
  const token = ++chartToken;
  setStatus(text.loadingChart, true);
  const response = await readBody(chartURL(theme), CHART_ACCEPT, "star-history");
  if (token !== chartToken) {
    return;
  }
  const svg = response.body.trim();
  if (!svg.startsWith("<svg")) {
    throw new Error(text.badChart);
  }
  chart.alt = text.chartAlt;
  chart.src = svgDataURL(svg);
  chartCard.hidden = false;
  return token;
}

function themeName(state) {
  return state && state.theme === "dark" ? "dark" : "light";
}

async function readBody(url, accept, name) {
  const result = await axonhub.fetch(url, {
    method: "GET",
    headers: {
      Accept: accept,
      "User-Agent": "axonhub-example-star-history",
    },
  });
  const statusCode = result && Number.isFinite(result.status) ? result.status : 0;
  const body = result && typeof result.body === "string" ? result.body : "";
  if (statusCode < 200 || statusCode >= 300) {
    throw new Error(text.http(name, statusCode, excerpt(body)));
  }
  return { body };
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

function renderMeta(repo) {
  const rows = [
    [text.repo, repo.fullName],
    [text.stars, formatCount(repo.stars, locale)],
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

function showStatus(error) {
  const code = error && typeof error === "object" && typeof error.code === "string" ? error.code : "";
  const message = code ? text.bridge(code) : error instanceof Error ? error.message : String(error);
  setStatus(message, false);
}

function setStatus(message, busy) {
  status.textContent = message;
  status.classList.toggle("busy", Boolean(busy) && message !== "");
}
