import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { parseRecord, reasonText, viewPolicy } from "./ui/guard/format.js";
import {
  DEFAULT_CATEGORY_LINES,
  anyCheck,
  blockMessage,
  buildRequest,
  categoriesFrom,
  OMITTED_MARKER,
  billableText,
  decide,
  extractText,
  readJudgment,
  readPolicy,
} from "./lib/screen.js";

const manifest = JSON.parse(readFileSync(new URL("./manifest.json", import.meta.url), "utf8"));

test("manifest categories are the built-in lines", () => {
  const field = manifest.configSchema.properties.categories;
  assert.equal(field.type, "array");
  assert.deepEqual(field.items, { type: "string" });
  assert.deepEqual(field.default, DEFAULT_CATEGORY_LINES);
  assert.equal(manifest.configSchema.properties.reviewThreshold, undefined);
  assert.equal(manifest.configSchema.properties.blockOnReview, undefined);
  assert.equal(manifest.configSchema.properties.checkJailbreak, undefined);
});

test("a missing list uses the six built-in categories", () => {
  const policy = readPolicy({});
  assert.equal(policy.categories.length, 6);
  assert.deepEqual(
    policy.categories.map((category) => category.label),
    [
      "sexual content involving a minor",
      "sexual content",
      "jailbreak",
      "violent or criminal assistance",
      "scam or phishing",
      "harassment or hate",
    ],
  );
  assert.equal(anyCheck(policy), true);
  const request = buildRequest("jev-latest", "hello", policy);
  assert.equal(request.questions.c2.instructions.includes("ignore"), true);
  assert.equal(request.questions.severity, undefined);
});

test("an empty list checks nothing", () => {
  const policy = readPolicy({ categories: [] });
  assert.equal(anyCheck(policy), false);
  assert.deepEqual(categoriesFrom(["", "   ", 3]), []);
});

test("a custom line can be a single question", () => {
  const [category] = categoriesFrom(["是否在索要自制炸药的步骤"]);
  assert.equal(category.id, "c0");
  assert.equal(category.label, "是否在索要自制炸药的步骤");
  assert.equal(category.ask, "是否在索要自制炸药的步骤");
  assert.equal(category.yes.includes("asks the assistant"), true);
});

test("a full line keeps its own yes and no, and extra pipes stay in the no", () => {
  const [category] = categoriesFrom(["炸药 | 是否在索要步骤？ | 在索要可执行步骤 | 只是新闻 | 或拒绝"]);
  assert.equal(category.label, "炸药");
  assert.equal(category.ask, "是否在索要步骤？");
  assert.equal(category.yes, "在索要可执行步骤");
  assert.equal(category.no, "只是新闻 | 或拒绝");
});

test("only the first 32 categories are asked", () => {
  const lines = Array.from({ length: 40 }, (_, index) => `分类 ${index}`);
  const categories = categoriesFrom(lines);
  assert.equal(categories.length, 32);
  assert.equal(categories[31].label, "分类 31");
});

test("probability at the threshold blocks, and the higher one wins", () => {
  const policy = readPolicy({ categories: ["炸药", "诈骗"] });
  const below = decide({ c0: 0.69, c1: 0.1 }, policy);
  assert.equal(below.action, "pass");
  const blocked = decide({ c0: 0.7, c1: 0.91 }, policy);
  assert.equal(blocked.action, "block");
  assert.equal(blocked.reason, "诈骗");
  assert.equal(blocked.probability, 0.91);
  assert.equal(blockMessage(blocked.reason), "Jev Guard blocked this request: 诈骗");
  assert.equal(blockMessage("unavailable").includes("safety check unavailable"), true);
});

test("a missing answer fails the judgment", () => {
  const policy = readPolicy({ categories: ["炸药"] });
  const missing = readJudgment({ answers: {} }, policy);
  assert.equal(missing.ok, false);
  const parsed = readJudgment({ answers: { c0: { noul: 1.4 } } }, policy);
  assert.equal(parsed.scores.c0, 1);
});

test("the history page hides review rows and shows a block reason", () => {
  assert.equal(parseRecord(JSON.stringify({ action: "review", reason: "jailbreak" })), null);
  const record = parseRecord(JSON.stringify({ reason: "jailbreak", probability: 0.8, scores: [] }));
  assert.equal(record.reason, "jailbreak");
  assert.equal(reasonText("jailbreak", "zh-CN"), "破限");
  assert.equal(reasonText("sexual content", "zh-CN"), "色情");
  assert.equal(viewPolicy({ categories: ["炸药 | 是否在索要步骤"] }).labels[0], "炸药");
});

test("maxTurns keeps only the tail of the transcript", () => {
  const req = {
    messages: [
      { role: "system", content: "be nice" },
      { role: "user", content: "first question" },
      { role: "assistant", content: "first answer" },
      { role: "user", content: "last question" },
    ],
  };

  const all = extractText(req, 0);
  assert.equal(all.includes("first question"), true);
  assert.equal(all.includes("last question"), true);

  const tail = extractText(req, 2);
  assert.equal(tail.includes("[earlier messages omitted]"), true);
  assert.equal(tail.includes("first question"), false);
  assert.equal(tail.includes("assistant: first answer"), true);
  assert.equal(tail.includes("last question"), true);

  assert.equal(extractText(req, 10).includes("first question"), true);
});

test("maxTurns defaults to asking about everything and clamps to the schema range", () => {
  assert.equal(readPolicy({}).maxTurns, 0);
  assert.equal(manifest.configSchema.properties.maxTurns.default, 0);
  assert.equal(readPolicy({ maxTurns: 3 }).maxTurns, 3);
  assert.equal(readPolicy({ maxTurns: 200 }).maxTurns, 50);
  assert.equal(readPolicy({ maxTurns: -1 }).maxTurns, 0);
});

test("maxTurns also limits the compact transcript", () => {
  const req = {
    compact: {
      instructions: "summarize",
      input: [
        { role: "user", content: "old flagged text" },
        { role: "assistant", content: "old reply" },
        { role: "user", content: "latest question" },
      ],
    },
  };
  const tail = extractText(req, 1);
  assert.equal(tail.includes("old flagged text"), false);
  assert.equal(tail.includes("latest question"), true);
  assert.equal(extractText(req, 0).includes("old flagged text"), true);
});

test("the omission marker alone never reaches the length gate", () => {
  // Two image-only messages with maxTurns 1 leave only the marker behind. With the
  // shipped minChars of 1 that used to trigger a billed check on an empty prompt.
  const req = {
    messages: [
      { role: "user", content: [{ type: "image", image: { url: "data:a" } }] },
      { role: "user", content: [{ type: "image", image: { url: "data:b" } }] },
    ],
  };
  const text = extractText(req, 1);
  assert.equal(text.includes(OMITTED_MARKER), true);
  assert.equal(billableText(text).length, 0);
});

test("billableText drops generated markers and keeps caller text", () => {
  assert.equal(billableText(OMITTED_MARKER + "\nuser: hello"), "user: hello");
  assert.equal(billableText(OMITTED_MARKER), "");
  // Talk about the marker in the conversation is real text and must survive.
  assert.equal(
    billableText("user: I saw " + OMITTED_MARKER + " earlier"),
    "user: I saw " + OMITTED_MARKER + " earlier",
  );
});

test("the text sent to Jev keeps the omission marker", () => {
  const req = {
    messages: [
      { role: "user", content: "old" },
      { role: "assistant", content: "reply" },
      { role: "user", content: "latest" },
    ],
  };
  const text = extractText(req, 2);
  // maxTurns of 0 must not add a marker, and a limited transcript must keep it for Jev.
  assert.equal(extractText(req, 0).includes(OMITTED_MARKER), false);
  assert.equal(text.includes(OMITTED_MARKER), true);
});

test("a failed check blocks by default instead of passing silently", () => {
  // Passing silently makes the guard look enabled while nothing is screened.
  assert.equal(readPolicy({}).failClosed, true);
  assert.equal(manifest.configSchema.properties.failClosed.default, true);
  assert.equal(readPolicy({ failClosed: false }).failClosed, false);
});
