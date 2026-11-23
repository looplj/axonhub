import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const root = import.meta.dirname;
const page = join(root, "ui", "day");
const read = (file) => readFileSync(join(page, file), "utf8");

const scope = {};
new Function("globalThis", read("view.js"))(scope);
const V = scope.DayView;

const bgScope = {};
new Function("globalThis", read("backgrounds.js"))(bgScope);
const B = bgScope.DayBackgrounds;

const now = new Date(2026, 10, 9, 23, 4);

const overview = {
  totalRequests: 412,
  totalInputTokens: 1_700_000,
  totalCachedInputTokens: 500_000,
  totalUncachedInputTokens: 1_200_000,
  totalOutputTokens: 300_000,
  totalCost: 12.345,
};

test("asks the server for aggregates, not usage rows", () => {
  const view = read("view.js");
  const app = read("app.js");
  assert.match(view, /dimension: "apiKey"/);
  assert.match(view, /analyticsOverview/);
  assert.match(view, /analyticsDailyStats/);
  assert.match(V.modelsQuery(2), /dimension: "model"/);
  assert.match(V.modelsQuery(2), /\$f0: AnalyticsFilter!/);
  assert.match(V.modelsQuery(2), /\$f1: AnalyticsFilter!/);
  assert.equal(V.DAY_QUERY.includes("totalCount"), true);
  for (const source of [view, app]) {
    assert.equal(source.includes("promptTokens"), false);
    assert.equal(source.includes("edges"), false);
    assert.equal(/\bfetch\(|XMLHttpRequest|WebSocket/.test(source), false);
  }
  assert.match(app, /apiKeyGUID/);
  assert.equal(V.apiKeyGUID("12"), "gid://axonhub/APIKey/12");
  assert.equal(V.apiKeyGUID("gid://axonhub/APIKey/12"), "gid://axonhub/APIKey/12");
  assert.equal(V.apiKeyGUID(""), "");
});

test("API keys become the receipt's agents, with models already aggregated", () => {
  const day = V.assemble({
    date: "2026-11-09",
    today: "2026-11-09",
    currency: "USD",
    errors: 3,
    unpriced: 2,
    overview,
    keys: [
      { id: "7", name: "claude", cost: 10, requestCount: 300, inputTokens: 1_100_000, cachedInputTokens: 400_000, outputTokens: 200_000 },
      { id: "8", name: "codex", cost: 2.3, requestCount: 100, inputTokens: 200_000, cachedInputTokens: 50_000, outputTokens: 90_000 },
    ],
    modelsByKey: {
      7: [
        { id: "claude-opus-4-1", name: "claude-opus-4-1", cost: 1, inputTokens: 10, cachedInputTokens: 0, outputTokens: 1 },
        { id: "claude-haiku-4-5", name: "claude-haiku-4-5", cost: 9, inputTokens: 10, cachedInputTokens: 0, outputTokens: 1 },
      ],
    },
    days: [
      { date: "2026-11-08", requestCount: 10, inputTokens: 1000, cachedInputTokens: 0, outputTokens: 0, cost: 2.5 },
      { date: "2026-11-09", requestCount: 412, inputTokens: 1_700_000, cachedInputTokens: 500_000, outputTokens: 300_000, cost: 12.345 },
    ],
  });
  assert.equal(day.input, 1_200_000);
  assert.equal(day.cacheRead, 500_000);
  assert.equal(day.calls, 412);
  // 412 - 400 requests were not attached to an API key
  assert.equal(day.agents.some((agent) => agent.id === ""), true);
  const card = V.card(day, "en", {}, now);
  assert.equal(card.title, "DAILY AI STATEMENT");
  assert.equal(card.tagline, "Routed through AxonHub.");
  assert.equal(card.agentHead[0], "API KEY / MODEL");
  assert.equal(card.total, "$12.35");
  assert.deepEqual(card.agents.map((agent) => agent.name), ["CLAUDE", "CODEX", "NO API KEY"]);
  assert.deepEqual(card.agents[0].models.map((model) => model.name), ["CLAUDE-HAIKU-4-5", "CLAUDE-OPUS-4-1"]);
  assert.equal(card.footer[0][1], "AxonHub");
  assert.equal(card.tokens[0].value, "2.00M");
  assert.equal(card.tokens.at(-1).value, "29%");
  assert.equal(card.note, "2 requests without a price");
  assert.equal(V.card(day, "zh", {}, now).tagline, "经 AxonHub 路由");
});

test("streak, average and currency stay on the receipt", () => {
  const days = [
    { date: "2026-11-06", calls: 10, tokens: 2000, cost: 2 },
    { date: "2026-11-07", calls: 10, tokens: 3000, cost: 3 },
    { date: "2026-11-08", calls: 10, tokens: 4000, cost: 4 },
    { date: "2026-11-09", calls: 412, tokens: 10_000_000, cost: 12.345 },
  ];
  assert.equal(V.streak(days, "2026-11-09", "2026-11-09"), 4);
  assert.equal(V.streak(days.slice(0, 3), "2026-11-09", "2026-11-09"), 3);
  assert.deepEqual(V.average(days, "2026-11-09"), { days: 3, tokens: 3000, cost: 3 });
  assert.equal(V.versus({ cost: 12.345 }, { cost: 2.5, tokens: 1 }), "4.9×");
  assert.equal(V.money(12.345, "CNY"), "¥12.35");
  assert.equal(V.money(0.004, "USD"), "<$0.01");
  assert.equal(V.fileName("2026-11-09"), "axonhub 2026-11-09");
});

test("a calendar day in the analytics timezone", () => {
  assert.equal(V.startOfDay("2026-11-09", "Asia/Shanghai").toISOString(), "2026-11-08T16:00:00.000Z");
  assert.equal(V.endOfDay("2026-11-09", "Asia/Shanghai").toISOString(), "2026-11-09T15:59:59.999Z");
  assert.equal(V.calendarDate(new Date("2026-11-08T16:00:00.000Z"), "Asia/Shanghai"), "2026-11-09");
  assert.equal(V.shift("2026-03-01", -1), "2026-02-28");
});

test("every language has every word, and the port note stays off the card", () => {
  const keys = Object.keys(V.TEXT.en).sort();
  const readme = readFileSync(join(root, "README.md"), "utf8");
  assert.match(readme, /移植自/);
  assert.match(read("view.js"), /移植自 magpie/);
  for (const t of Object.values(V.TEXT)) {
    assert.deepEqual(Object.keys(t).sort(), keys);
    assert.equal(/移植|ported|portiert|magpie/i.test(t.tagline + t.footer.flat().join("")), false);
    assert.equal(t.footer[0][1], "AxonHub");
  }
  for (const match of read("index.html").matchAll(/data-t(?:-label)?="([^"]+)"/g)) assert.ok(keys.includes(match[1]));
  assert.deepEqual(
    [V.langOf("zh-CN"), V.langOf("zh-TW"), V.langOf("zh-Hant-HK"), V.langOf("ja-JP"), V.langOf("de-DE"), V.langOf("fr")],
    ["zh", "zh-TW", "zh-TW", "ja", "de", "en"],
  );
});

test("the receipt face and the photos come with the page", () => {
  const fonts = {};
  new Function("globalThis", read("fonts.js"))(fonts);
  assert.deepEqual(Object.keys(fonts.DayFonts).sort(), ["400", "700", "800"]);
  for (const body of Object.values(fonts.DayFonts)) assert.equal(Buffer.from(body, "base64").subarray(0, 4).toString(), "wOF2");
  assert.match(read("fonts.LICENSE.txt"), /SIL OPEN FONT LICENSE/);
  const thumbs = {};
  new Function("globalThis", read("bg/thumbs.js"))(thumbs);
  for (const backdrop of B.list.filter((item) => item.photo)) {
    const photos = {};
    new Function("globalThis", read(`bg/${backdrop.id}.js`))(photos);
    assert.match(photos.DayPhotos[backdrop.id], /^data:image\/jpeg;base64,\/9j\//);
    assert.match(thumbs.DayThumbs[backdrop.id], /^data:image\/jpeg;base64,\/9j\//);
  }
  assert.equal(readdirSync(page).includes("style.css"), true);
});

test("a saved backdrop is made sound before it is drawn", () => {
  assert.deepEqual(B.norm(null), { bg: "teal", pad: 76, blur: 0, dim: 0, image: "" });
  assert.deepEqual(B.norm({ bg: "seigaiha", pad: 500, blur: -3, dim: "40" }), { bg: "seigaiha", pad: 160, blur: 0, dim: 40, image: "" });
  assert.equal(B.norm({ bg: "custom" }).bg, "teal");
  assert.deepEqual(B.norm({ bg: "custom", image: "data:image/jpeg;base64,AAAA", pad: 24 }), { bg: "custom", pad: 24, blur: 0, dim: 0, image: "data:image/jpeg;base64,AAAA" });
  for (const id of [...B.list.map((item) => item.id), "custom"]) assert.equal(typeof B.meta(id).photo, "boolean");
  assert.match(read("app.js"), /pop\.classList\.toggle\("photo", B\.meta\(bg\.bg\)\.photo === true\)/);
});
