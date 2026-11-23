import {
  ENDPOINT,
  MAX_TEXT,
  anyCheck,
  apiKeyOf,
  blockMessage,
  buildRequest,
  decide,
  excerptOf,
  extractText,
  fitText,
  readJudgment,
  readPolicy,
} from "./lib/screen.js";

const skipped = {};

export async function onRequest(ctx, req) {
  const policy = readPolicy(ctx && ctx.config);
  if (!anyCheck(policy)) {
    return;
  }
  try {
    const outcome = await judge(req, ctx, policy);
    apply(ctx, policy, outcome);
  } catch (err) {
    if (err && err.name === "PluginRequestReject") {
      throw err;
    }
    apply(ctx, policy, {
      action: policy.failClosed ? "block" : "pass",
      reason: "exception",
      probability: null,
      scores: [],
      text: "",
      failure: true,
    });
  }
}

async function judge(req, ctx, policy) {
  const raw = extractText(req).trim();
  if (raw.length < policy.minChars) {
    return pass();
  }
  const key = apiKeyOf(ctx && ctx.config);
  if (key === "") {
    return unavailable(policy, "missing_api_key");
  }
  const text = fitText(raw, MAX_TEXT);
  let response;
  try {
    response = await ctx.fetch(ENDPOINT, {
      method: "POST",
      headers: {
        Authorization: "Bearer " + key,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(buildRequest(policy.model, text, policy)),
    });
  } catch (err) {
    return unavailable(policy, "fetch_failed");
  }
  const status = response && typeof response.status === "number" ? response.status : 0;
  if (status < 200 || status >= 300) {
    return unavailable(policy, "http_" + status);
  }
  let payload;
  try {
    payload = JSON.parse(response.body);
  } catch (err) {
    return unavailable(policy, "bad_json");
  }
  const judgment = readJudgment(payload, policy);
  if (!judgment.ok) {
    return unavailable(policy, judgment.error);
  }
  const decision = decide(judgment.scores, policy);
  return {
    action: decision.action,
    reason: decision.reason,
    probability: decision.probability,
    scores: decision.scores,
    text: text,
    failure: false,
  };
}

function apply(ctx, policy, outcome) {
  if (outcome.action === "pass" && outcome.failure) {
    warnOnce(ctx, outcome.reason);
    return;
  }
  if (outcome.action !== "block") {
    return;
  }
  remember(ctx, policy, outcome);
  ctx.log.info("request blocked", fields(outcome));
  ctx.reject(403, blockMessage(outcome.failure ? "unavailable" : outcome.reason));
}

function remember(ctx, policy, outcome) {
  try {
    const seq = nextSeq(ctx);
    ctx.kv.set(recordKey(seq), JSON.stringify(record(ctx, policy, outcome, seq)));
    trim(ctx, policy.keep);
  } catch (err) {
    ctx.log.warn("decision was not stored", { reason: outcome.reason });
  }
}

function record(ctx, policy, outcome, seq) {
  const meta = ctx && ctx.request ? ctx.request : {};
  const body = {
    seq: seq,
    at: new Date().toISOString(),
    reason: outcome.reason || "",
    probability: outcome.probability,
    scores: outcome.scores || [],
    model: typeof meta.model === "string" ? meta.model : "",
    requestType: typeof meta.requestType === "string" ? meta.requestType : "",
    apiFormat: typeof meta.apiFormat === "string" ? meta.apiFormat : "",
    projectId: typeof meta.projectId === "number" ? meta.projectId : null,
  };
  const excerpt = excerptOf(outcome.text, policy.storeExcerpt);
  if (excerpt !== "") {
    body.excerpt = excerpt;
  }
  return body;
}

function nextSeq(ctx) {
  const raw = ctx.kv.get("seq");
  const current = raw == null || raw === "" ? 0 : Number(raw);
  const seq = isFinite(current) && current >= 0 ? Math.floor(current) + 1 : 1;
  ctx.kv.set("seq", String(seq));
  return seq;
}

function trim(ctx, keep) {
  const page = ctx.kv.list({ prefix: "d.", first: 100 });
  const entries = page && page.entries ? page.entries : [];
  let extra = entries.length - keep;
  if (extra > 6) {
    extra = 6;
  }
  for (let i = 0; i < extra; i++) {
    if (entries[i] && entries[i].key) {
      ctx.kv.delete(entries[i].key);
    }
  }
}

function recordKey(seq) {
  const digits = String(seq);
  const width = 10;
  let body = digits;
  if (digits.length < width) {
    body = "0000000000".slice(0, width - digits.length) + digits;
  }
  return "d." + body;
}

function warnOnce(ctx, reason) {
  if (skipped[reason]) {
    return;
  }
  skipped[reason] = true;
  ctx.log.warn("safety check skipped", { reason: reason });
}

function fields(outcome) {
  return {
    reason: outcome.reason || "",
    probability: outcome.probability,
  };
}

function pass() {
  return { action: "pass", reason: "", probability: null, scores: [], text: "", failure: false };
}

function unavailable(policy, reason) {
  if (policy.failClosed) {
    return { action: "block", reason: reason, probability: null, scores: [], text: "", failure: true };
  }
  return { action: "pass", reason: reason, probability: null, scores: [], text: "", failure: true };
}
