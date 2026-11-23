import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  KV_VALUE_LIMIT,
  REFRESH_MS,
  endpoint,
  eventDay,
  interpretBridgeError,
  interpretResponse,
  isPending,
  monthGrid,
  nextFetch,
  planWrites,
  summarize,
  userAgent,
} from "./ui/resets/format.js";

const manifest = JSON.parse(readFileSync(new URL("./manifest.json", import.meta.url), "utf8"));
const network = new RegExp(manifest.network[0].regex);
const now = Date.parse("2026-11-08T02:40:00.000Z");

function event(partial) {
  return {
    type: "direct_reset",
    status: "confirmed",
    presentation: { status: "confirmed" },
    confirmedAt: "2026-10-28T08:00:00.000+08:00",
    ...partial,
  };
}

test("network rule matches only the three full calendars", () => {
  assert.equal(manifest.ui.actions[0].slot, "app.header.actions");
  assert.equal(manifest.ui.pages, undefined);
  for (const id of ["codex", "antigravity", "claude"]) {
    assert.equal(network.test(endpoint(id)), true);
  }
  assert.equal(network.test("https://aihot.news/api/v1/codex-resets/recent"), false);
  assert.equal(network.test("https://aihot.news/api/v1/items"), false);
  assert.equal(network.test("https://aihot.news/api/v1/codex-resets?x=1"), false);
});

test("user agent identifies this plugin and is not a browser", () => {
  const agent = userAgent("11111111-2222-4333-8444-555555555555");
  assert.match(agent, /^aihot-api\/2\.0\.0 aihot-actor\/11111111-2222-4333-8444-555555555555 axonhub-reset-watch\/0\.1\.0$/);
  assert.equal(/mozilla|chrome|safari/i.test(agent), false);
});

test("a likely completed announcement is not still waiting", () => {
  assert.equal(isPending(event({ status: "announced", presentation: { status: "likely_completed" } })), false);
  assert.equal(isPending(event({ status: "announced", presentation: { status: "in_progress" } })), true);
  assert.equal(isPending(event({ status: "announced", presentation: null })), true);
});

test("occurredOn wins over the confirmation timestamp", () => {
  assert.equal(eventDay(event({ occurredOn: "2026-10-27", confirmedAt: "2026-10-28T08:00:00.000+08:00" })), "2026-10-27");
});

test("ninety day counts skip anything still waiting", () => {
  const summary = summarize(
    [
      event({ confirmedAt: "2026-08-10T08:00:00.000+08:00" }),
      event({ confirmedAt: "2026-10-26T08:00:00.000+08:00" }),
      event({ confirmedAt: "2026-10-28T08:00:00.000+08:00" }),
      event({ type: "reset_credit", confirmedAt: "2026-11-02T07:27:20.000+08:00" }),
      event({ status: "announced", presentation: { status: "announced" }, confirmedAt: null, estimate: { from: "2026-11-09T08:00:00.000+08:00" } }),
    ],
    "2026-11-08",
  );
  assert.equal(summary.resets, 2);
  assert.equal(summary.cards, 1);
  assert.equal(summary.medianDays, 2);
  assert.equal(eventDay(summary.lastReset), "2026-10-28");
  assert.equal(eventDay(summary.lastCard), "2026-11-02");
});

test("November 2026 starts on Sunday, so the Monday grid leads with blanks", () => {
  const cells = monthGrid("2026-11");
  assert.equal(cells.length % 7, 0);
  assert.equal(cells[6], "2026-11-01");
  assert.equal(cells.filter(Boolean).length, 30);
});

test("a fresh snapshot and an active retry window do not fetch", () => {
  const fresh = nextFetch({ body: "{}", meta: { etag: "abc", fetchedAt: new Date(now - 60_000).toISOString(), retryAt: "" } }, now);
  assert.equal(fresh.fetch, false);
  assert.equal(fresh.reason, "fresh");
  const waiting = nextFetch({ body: "", meta: { etag: "", fetchedAt: "", retryAt: new Date(now + 60_000).toISOString() } }, now);
  assert.equal(waiting.fetch, false);
  assert.equal(waiting.reason, "retry");
  const stale = nextFetch({ body: "{}", meta: { etag: "abc", fetchedAt: new Date(now - REFRESH_MS - 1).toISOString(), retryAt: "" } }, now);
  assert.equal(stale.fetch, true);
});

test("429 keeps the saved calendar and records when to try again", () => {
  const decision = interpretResponse({
    status: 429,
    headers: { "Retry-After": "120", etag: "new" },
    body: JSON.stringify({ detail: "slow down" }),
    cachedBody: "{\"schemaVersion\":1}",
    cachedMeta: { etag: "old", fetchedAt: "2026-11-08T02:00:00.000Z", retryAt: "" },
    now,
  });
  assert.equal(decision.ok, true);
  assert.equal(decision.persist, "meta");
  assert.equal(decision.body, "{\"schemaVersion\":1}");
  assert.equal(decision.meta.etag, "old");
  assert.equal(decision.meta.fetchedAt, "2026-11-08T02:00:00.000Z");
  assert.equal(Date.parse(decision.meta.retryAt) - now, 120_000);
  assert.equal(decision.warning, "429");
});

test("429 without a saved calendar does not invent one", () => {
  const decision = interpretResponse({ status: 429, headers: {}, body: "", cachedBody: "", cachedMeta: null, now });
  assert.equal(decision.ok, false);
  assert.equal(decision.error, "429");
  assert.equal(decision.persist, "meta");
  assert.equal(decision.body, "");
});

test("304 and 200 update the cache, a bad payload does not replace it", () => {
  const cached = JSON.stringify({ schemaVersion: 1, events: [] });
  const notModified = interpretResponse({
    status: 304,
    headers: { ETag: "same" },
    body: "",
    cachedBody: cached,
    cachedMeta: { etag: "same", fetchedAt: "2026-11-08T01:00:00.000Z", retryAt: "stale" },
    now,
  });
  assert.equal(notModified.persist, "meta");
  assert.equal(notModified.body, cached);
  assert.equal(notModified.meta.fetchedAt, new Date(now).toISOString());
  assert.equal(notModified.meta.retryAt, "");

  const replacement = JSON.stringify({ schemaVersion: 1, today: "2026-11-08", events: [] });
  const ok = interpretResponse({ status: 200, headers: { etag: "next" }, body: replacement, cachedBody: cached, cachedMeta: null, now });
  assert.equal(ok.persist, "all");
  assert.equal(ok.body, replacement);

  const broken = interpretResponse({ status: 200, headers: {}, body: "{", cachedBody: cached, cachedMeta: null, now });
  assert.equal(broken.persist, null);
  assert.equal(broken.body, cached);
  assert.equal(broken.warning, "bad-json");
});

test("a host rate limit uses the same retry record as an upstream 429", () => {
  const decision = interpretBridgeError({ code: "RATE_LIMITED", message: "slow" }, { body: "{}", meta: { etag: "old", fetchedAt: "", retryAt: "" } }, now);
  assert.equal(decision.persist, "meta");
  assert.equal(decision.warning, "429");
  assert.equal(decision.body, "{}");
  const denied = interpretBridgeError({ code: "FORBIDDEN", message: "no" }, { body: "", meta: null }, now);
  assert.equal(denied.persist, null);
  assert.equal(denied.code, "FORBIDDEN");
});

test("readers do not write, and an oversized calendar is not stored", () => {
  const decision = { persist: "all", body: "x".repeat(KV_VALUE_LIMIT + 1), meta: { etag: "", fetchedAt: "", retryAt: "" } };
  assert.deepEqual(planWrites(false, decision), { writes: [], skippedBody: false });
  const planned = planWrites(true, decision);
  assert.equal(planned.skippedBody, true);
  assert.deepEqual(planned.writes.map((write) => write.key), ["meta"]);
});
