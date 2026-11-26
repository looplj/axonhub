import assert from "node:assert/strict";
import test from "node:test";
import { onTask } from "./plugin.js";
import { countAvailable, possibleMergeIDs } from "./matching.js";

function offer(id, canonical = "dev/x", version = "1") {
  return { id, canonical, identity: JSON.stringify([canonical, version, ""]) };
}

test("prefix matching uses the native importer's identity and target occupancy rules", () => {
  const ids = new Set(["mirror/one/x", "other/x"]);
  const compatible = [offer("mirror/one/x"), offer("other/x", "dev/x", "")];
  const available = (offers, occupied = []) => countAvailable(ids, new Set(occupied), ids, true, offers);
  assert.equal(available(compatible), 1);
  assert.equal(available(compatible, ["x"]), 2);
  assert.equal(available([offer("mirror/one/x"), offer("other/x", "dev/x", "2")]), 2);
  assert.equal(available([offer("mirror/one/x")]), 2);
  assert.equal(available([offer("mirror/one/x"), offer("other/x", "")]), 2);
  assert.equal(countAvailable(ids, new Set(), ids, false, compatible), 2);
  const aliases = new Set(["dev/x", "other/x"]);
  assert.equal(countAvailable(aliases, new Set(), aliases, true, [offer("other/x")]), 1);
  assert.equal(countAvailable(aliases, new Set(), aliases, true, [offer("other/x"), offer("second/x")]), 2);
});

test("only request cross-provider identities for possible channel merges", () => {
  const ids = (channels, imported, hasProviders) => [...possibleMergeIDs(new Set(channels), new Set(imported), hasProviders)].sort();
  assert.deepEqual(ids(["x"], [], true), []);
  assert.deepEqual(ids(["a/x"], [], false), []);
  assert.deepEqual(ids(["a/x", "b/x"], ["a/x", "b/x"], true), []);
  assert.deepEqual(ids(["a/x", "b/x"], ["x"], true), []);
  assert.deepEqual(ids(["a/x", "b/x"], [], false), ["a/x", "b/x", "x"]);
  assert.deepEqual(ids(["a/x"], [], true), ["a/x", "x"]);
});

test("task reads one offer for freshness and pages identities for a potential provider merge", () => {
  for (const { providers, version, cursors, available } of [
    { providers: [], version: "1", cursors: [null], available: 1 },
    { providers: ["source"], version: "1", cursors: [null, "second"], available: 1 },
    { providers: ["source"], version: "2", cursors: [null, "second"], available: 2 },
  ]) {
    const reads = [];
    let summary;
    onTask({
      config: { providers, channels: true, ignorePrefix: true },
      graphql(document, { after, first, providers: filter }) {
        if (document.includes("query Channels")) {
          return { data: { channels: { edges: [{ node: { supportedModels: ["relay/x"] } }], pageInfo: {} } } };
        }
        if (document.includes("query Models")) return { data: { models: { edges: [], pageInfo: {} } } };
        assert.deepEqual(filter, []);
        if (!providers.length) assert.equal(first, 1);
        reads.push(after);
        const quote = after ? offer("relay/x", "dev/x", version) : offer("x");
        const offers = [{ ...quote, modelID: quote.id, provider: after ? "relay" : "source", rates: [] }];
        return { data: { modelCatalogOffers: { source: "upstream", offers, nextCursor: after ? null : "second" } } };
      },
      kv: { set(_key, value) { summary = JSON.parse(value); } },
      ui: { setBadge() {} },
    });
    assert.deepEqual(reads, cursors);
    assert.equal(summary.available, available);
  }
});

test("pages all sources, counts once per model, publishes only complete upstream results", () => {
  const reads = [];
  let lastSummary, badge, failure;
  const binding = { follow: true, provider: "original", modelID: "bound", canonical: "dev/x", value: 3 };
  const ctx = {
    config: { providers: [" extra ", "extra"] },
    graphql(document, variables) {
      const next = variables.after === null;
      const connection = (nodes) => ({ edges: nodes.map((node) => ({ node })), pageInfo: { hasNextPage: next, endCursor: "next" } });
      if (document.includes("query Channels")) {
        return { data: { channels: connection([{ supportedModels: next ? ["unmatched", "shared"] : ["shared", "archived-alias"] }]) } };
      }
      if (document.includes("query Models")) {
        const model = next ? { modelID: "local", status: "enabled", catalogAliases: [], catalogPrices: [
          { ...binding, field: "input" }, { ...binding, field: "output" }, { ...binding, follow: false, field: "cacheRead" },
        ] } : { modelID: "archived", status: "archived", catalogAliases: ["archived-alias"], catalogPrices: [{ ...binding, field: "input" }] };
        return { data: { models: connection([model]) } };
      }
      reads.push(variables);
      if (!next && (failure === "cursor" || failure === "once")) {
        if (failure === "once") failure = "recovered";
        return { errors: [{ message: "model catalog snapshot changed, restart pagination" }] };
      }
      const changedOffer = { provider: "original", modelID: "bound", canonical: "dev/x", rates: [{ field: "input", base: 0 }, { field: "output", base: 4 }] };
      const offers = next ? [{ provider: "extra", modelID: "shared", rates: [] }] : [
        changedOffer,
        { provider: "original", modelID: "not-subscribed", rates: [] },
      ];
      if (next && failure === "once") offers.push({ provider: "extra", modelID: "withdrawn", rates: [] }, changedOffer);
      if (!next && failure === "recovered") offers.length = 0;
      return { data: { modelCatalogOffers: { source: !next && failure === "fallback" ? "fallback" : "upstream", fetchedAt: "2026-11-26", offers, nextCursor: next ? "second" : null } } };
    },
    kv: { set(_key, value) { lastSummary = JSON.parse(value); } },
    ui: { setBadge(_target, value) { if (failure === "badge") throw Error("badge failed"); badge = value; } },
  };
  onTask(ctx);
  assert.deepEqual(lastSummary, { scope: { channels: true, providers: ["extra"], ignorePrefix: false }, fetchedAt: "2026-11-26", available: 2, changed: 1 });
  assert.deepEqual(badge, { count: 3 });
  assert.deepEqual(reads.map((r) => [r.refresh, r.after, r.providers]), [[true, null, ["extra", "original"]], [false, "second", ["extra", "original"]]]);
  failure = "once";
  reads.length = 0;
  onTask(ctx);
  assert.deepEqual(reads.map((r) => r.after), [null, "second", null, "second"]);
  assert.equal(lastSummary.available, 2);
  assert.equal(lastSummary.changed, 0);
  const previous = lastSummary;
  ctx.config = { channels: false, providers: [] };
  for (failure of ["cursor", "fallback"]) {
    reads.length = 0;
    assert.throws(() => onTask(ctx));
    assert.equal(reads.length, failure === "cursor" ? 4 : 2);
    assert.equal(lastSummary, previous);
    assert.deepEqual(badge, { count: 2 });
  }
  failure = "badge";
  assert.throws(() => onTask(ctx), /badge failed/);
  assert.deepEqual(lastSummary.scope, { channels: false, providers: [], ignorePrefix: false });
  assert.deepEqual(badge, { count: 2 });
  failure = undefined;
  onTask(ctx);
  assert.deepEqual(badge, { count: 1 });
});
