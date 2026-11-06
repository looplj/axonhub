import { recordKey, summarize } from "./lib/record.js";

const defaultKeep = 20;
const maxKeep = 50;
// seq + record use 2 writes. The host allows 10 writes per call.
const maxDeletesPerEvent = 8;

export function onEvent(event, ctx) {
  const keep = clampKeep(ctx.config.keep);
  const seq = nextSeq(ctx);
  ctx.kv.set(recordKey(seq), JSON.stringify(summarize(event, seq)));
  trim(ctx, keep);
}

function nextSeq(ctx) {
  const raw = ctx.kv.get("seq");
  const current = raw == null || raw === "" ? 0 : Number(raw);
  const seq = isFinite(current) && current >= 0 ? Math.floor(current) + 1 : 1;
  ctx.kv.set("seq", String(seq));
  return seq;
}

function trim(ctx, keep) {
  const keys = listKeys(ctx);
  let extra = keys.length - keep;
  if (extra > maxDeletesPerEvent) {
    extra = maxDeletesPerEvent;
  }
  for (let i = 0; i < extra; i++) {
    ctx.kv.delete(keys[i]);
  }
}

function listKeys(ctx) {
  const keys = [];
  let after = "";
  for (;;) {
    const options = { prefix: "event.", first: 100 };
    if (after !== "") {
      options.after = after;
    }
    const page = ctx.kv.list(options);
    const entries = page && page.entries ? page.entries : [];
    for (let i = 0; i < entries.length; i++) {
      if (entries[i] && entries[i].key) {
        keys.push(entries[i].key);
      }
    }
    const cursor =
      page && typeof page.nextCursor === "string" ? page.nextCursor : "";
    if (cursor === "") {
      return keys;
    }
    after = cursor;
  }
}

function clampKeep(value) {
  if (
    typeof value !== "number" ||
    !isFinite(value) ||
    Math.floor(value) !== value
  ) {
    return defaultKeep;
  }
  if (value < 1) {
    return defaultKeep;
  }
  if (value > maxKeep) {
    return maxKeep;
  }
  return value;
}
