#!/usr/bin/env node
// claude-design-system: local editor server (prebuilt; do not edit).

import os6 from "node:os";

import { createHash as createHash5, randomBytes as randomBytes2, randomUUID } from "node:crypto";
import fs12 from "node:fs";
import fsp4 from "node:fs/promises";
import path12 from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { spawn } from "node:child_process";
import fs2 from "node:fs";
import os from "node:os";

var COMMENTS_FILE = "comments.json";
var COMMENTS_VERSION = 1;
var LIMITS = { text: 1e4, label: 1e3, detail: 4e3, anchor: 2048, section: 200, name: 100 };
function sameAuthor(a, b) {
  return a.kind === "user" && b.kind === "user" && (a.principal ?? "") === (b.principal ?? "");
}
var CommentsError = class extends Error {
  code;
  status;
  constructor(code, message, status = 400) {
    super(message);
    this.code = code;
    this.status = status;
  }
};
var emptyDoc = () => ({ version: COMMENTS_VERSION, threads: [] });
function randomHex(bytes) {
  const buf = new Uint8Array(bytes);
  globalThis.crypto.getRandomValues(buf);
  return Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join("");
}
var newId = (prefix) => `${prefix}_${randomHex(5)}`;
var isRecord = (v) => typeof v === "object" && v !== null && !Array.isArray(v);
var isText = (v, max) => typeof v === "string" && v.length <= max;
function cleanText(v) {
  if (typeof v !== "string") return null;
  const text = v.replace(/\r\n?/g, "\n").trim();
  return text !== "" && text.length <= LIMITS.text ? text : null;
}
function readNewThread(body) {
  if (!isRecord(body)) return null;
  const text = cleanText(body.text);
  if (text === null || !isText(body.anchor, LIMITS.anchor) || body.anchor === "" || !isText(body.label, LIMITS.label)) return null;
  if (body.detail !== void 0 && !isText(body.detail, LIMITS.detail)) return null;
  if (body.section !== void 0 && (!isText(body.section, LIMITS.section) || body.section === "")) return null;
  if (body.area !== void 0 && typeof body.area !== "boolean") return null;
  if (body.send !== void 0 && typeof body.send !== "boolean") return null;
  return {
    anchor: body.anchor,
    label: body.label.trim() || "Comment",
    ...body.detail ? { detail: body.detail } : {},
    ...body.area ? { area: true } : {},
    ...body.section ? { section: body.section } : {},
    text,
    ...body.send ? { send: true } : {}
  };
}
function readReply(body) {
  if (!isRecord(body)) return null;
  const text = cleanText(body.text);
  if (text === null) return null;
  if (body.send !== void 0 && typeof body.send !== "boolean") return null;
  return { text, ...body.send ? { send: true } : {} };
}
var readEdit = readReply;
function readStatus(body) {
  return isRecord(body) && (body.status === "open" || body.status === "resolved") ? body.status : null;
}
function readAgentRequest(body) {
  if (!isRecord(body)) return null;
  const text = cleanText(body.text);
  if (text === null) return null;
  if (body.label !== void 0 && !isText(body.label, LIMITS.label)) return null;
  if (body.selection !== void 0 && !isText(body.selection, LIMITS.detail)) return null;
  const label = typeof body.label === "string" ? body.label.trim() : "";
  const selection = typeof body.selection === "string" ? body.selection.trim() : "";
  return { text, ...label ? { label } : {}, ...selection ? { selection } : {} };
}
function parseDoc(text) {
  const data = JSON.parse(text);
  if (!isRecord(data) || data.version !== COMMENTS_VERSION || !Array.isArray(data.threads)) {
    throw new CommentsError("COMMENTS_FILE_INVALID", `not a version ${COMMENTS_VERSION} comments file (expected {"version": 1, "threads": [...]})`, 500);
  }
  const threads = data.threads.filter(
    (t) => isRecord(t) && typeof t.id === "string" && typeof t.anchor === "string" && typeof t.label === "string" && (t.status === "open" || t.status === "resolved") && Array.isArray(t.messages) && t.messages.every((m) => isRecord(m) && typeof m.text === "string" && isRecord(m.author) && (m.author.kind === "user" || m.author.kind === "agent"))
  );
  for (const t of threads) {
    if (!isRecord(t.location)) t.location = {};
    for (const m of t.messages) if (m.author.principal !== void 0 && typeof m.author.principal !== "string") delete m.author.principal;
  }
  return { version: COMMENTS_VERSION, threads };
}
var formatDoc = (doc) => `${JSON.stringify(doc, null, 2)}
`;
function addThread(doc, input, location, author, now) {
  const thread = {
    id: uniqueId(doc),
    anchor: input.anchor,
    label: input.label,
    ...input.detail ? { detail: input.detail } : {},
    location: { ...location, ...input.section ? { section: input.section } : {}, ...input.area ? { area: true } : {} },
    status: "open",
    createdAt: now,
    updatedAt: now,
    messages: [{ id: newId("m"), author, text: input.text, at: now }]
  };
  if (input.send) sendToAgent(thread, now);
  doc.threads.push(thread);
  return thread;
}
function uniqueId(doc) {
  for (; ; ) {
    const id = newId("c");
    if (!doc.threads.some((t) => t.id === id)) return id;
  }
}
function addMessage(thread, author, text, now) {
  const message = { id: newId("m"), author, text, at: now };
  thread.messages.push(message);
  thread.updatedAt = now;
  if (author.kind === "agent") {
    if (thread.agent) thread.agent = { ...thread.agent, status: "replied", repliedAt: now };
  } else if (thread.status === "resolved") {
    thread.status = "open";
    delete thread.resolvedAt;
  }
  return message;
}
function editMessage(thread, messageId, text, now, by) {
  const message = thread.messages.find((m) => m.id === messageId);
  if (!message) throw new CommentsError("MESSAGE_NOT_FOUND", `No message ${messageId} in ${thread.id}.`, 404);
  if (message.author.kind !== "user" || by && !sameAuthor(message.author, by)) throw new CommentsError("MESSAGE_NOT_EDITABLE", "Only your own messages can be edited.", 403);
  if (message.text !== text) {
    message.text = text;
    message.editedAt = now;
    thread.updatedAt = now;
  }
  return message;
}
function canSendToAgent(thread) {
  const last = thread.messages.at(-1);
  if (thread.status !== "open" || !last || last.author.kind !== "user") return false;
  return !thread.agent || (last.editedAt ?? last.at) > thread.agent.sentAt;
}
function sendToAgent(thread, now) {
  thread.agent = { status: "pending", sentAt: now };
  thread.updatedAt = now;
}
function markDelivered(thread, now, via) {
  if (thread.agent?.status !== "pending") return;
  thread.agent = { ...thread.agent, status: "delivered", deliveredAt: now, via };
}
function setStatus(thread, status, now) {
  if (thread.status === status) return;
  thread.status = status;
  thread.updatedAt = now;
  if (status === "resolved") thread.resolvedAt = now;
  else delete thread.resolvedAt;
}
function findThread(doc, ref) {
  const exact = doc.threads.find((t) => t.id === ref);
  if (exact) return exact;
  const matches = ref.length >= 3 ? doc.threads.filter((t) => t.id.startsWith(ref)) : [];
  if (matches.length === 1) return matches[0];
  throw new CommentsError(
    matches.length ? "AMBIGUOUS_COMMENT" : "COMMENT_NOT_FOUND",
    matches.length ? `${ref} matches ${matches.length} comments; give more of the id.` : `No comment ${ref}.`,
    404
  );
}
function openThreads(doc) {
  return doc.threads.filter((t) => t.status === "open").sort((a, b) => a.createdAt < b.createdAt ? -1 : a.createdAt > b.createdAt ? 1 : 0);
}
var pendingForAgent = (doc) => openThreads(doc).filter((t) => t.agent?.status === "pending");
function agentView(thread, n) {
  return {
    id: thread.id,
    ...n !== void 0 ? { n } : {},
    status: thread.status,
    ...thread.location.file ? { file: thread.location.file } : {},
    where: thread.label,
    ...thread.detail ? { detail: thread.detail } : {},
    location: thread.location,
    anchor: thread.anchor,
    ...thread.agent ? { agent: thread.agent } : {},
    messages: thread.messages.map((m) => ({ from: m.author.kind, name: m.author.name, text: m.text, at: m.at, ...m.editedAt ? { editedAt: m.editedAt } : {} }))
  };
}
function agentViews(doc, threads) {
  const open = openThreads(doc);
  return threads.map((t) => {
    const i = open.indexOf(t);
    return agentView(t, i >= 0 ? i + 1 : void 0);
  });
}
function projectFile(rel) {
  if (!rel) return void 0;
  const body = rel.endsWith("/") ? rel.slice(0, -1) : rel;
  const parts = body.split("/");
  if (!parts.every((part) => part !== "" && part !== ".." && !part.startsWith(".") && !part.includes("\\"))) return void 0;
  return `project/${rel}`;
}

import fs from "node:fs";
import fsp from "node:fs/promises";
import path from "node:path";
var LOCK_STALE_MS = 1e4;
var LOCK_WAIT_MS = 5e3;
var LOCK_RETRY_MS = 20;
var commentsPath = (root) => path.join(root, COMMENTS_FILE);
async function readComments(root) {
  let text;
  try {
    text = await fsp.readFile(commentsPath(root), "utf8");
  } catch (error) {
    if (error.code === "ENOENT") return emptyDoc();
    throw error;
  }
  if (text.trim() === "") return emptyDoc();
  try {
    return parseDoc(text);
  } catch (error) {
    if (error instanceof CommentsError) throw new CommentsError(error.code, `${commentsPath(root)}: ${error.message}`, 500);
    throw new CommentsError("COMMENTS_FILE_INVALID", `${commentsPath(root)} is not valid JSON: ${error.message}`, 500);
  }
}
async function lock(root) {
  const file = `${commentsPath(root)}.lock`;
  const deadline = Date.now() + LOCK_WAIT_MS;
  for (; ; ) {
    try {
      const handle = await fsp.open(file, "wx");
      await handle.writeFile(`${process.pid}
`);
      await handle.close();
      return () => fsp.rm(file, { force: true });
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
    }
    const info = await fsp.stat(file).catch(() => null);
    if (info && Date.now() - info.mtimeMs > LOCK_STALE_MS) {
      await fsp.rm(file, { force: true });
      continue;
    }
    if (Date.now() > deadline) throw new CommentsError("COMMENTS_LOCKED", `${file} is held by another process; try again (or delete it if nothing is running).`, 503);
    await new Promise((resolve) => setTimeout(resolve, LOCK_RETRY_MS));
  }
}
async function writeAtomic(file, text) {
  const temp = path.join(path.dirname(file), `.${path.basename(file)}.${process.pid}.${Date.now().toString(36)}.tmp`);
  await fsp.writeFile(temp, text);
  try {
    await fsp.rename(temp, file);
  } catch (error) {
    await fsp.rm(temp, { force: true });
    throw error;
  }
}
async function updateComments(root, change) {
  const unlock = await lock(root);
  try {
    const doc = await readComments(root);
    const before = formatDoc(doc);
    const result = await change(doc);
    const after = formatDoc(doc);
    const exists = fs.existsSync(commentsPath(root));
    if (after !== before && (exists || doc.threads.length > 0)) await writeAtomic(commentsPath(root), after);
    return result;
  } finally {
    await unlock();
  }
}

var AGENT_GRACE_MS = 6e4;
var AGENT_BUSY_MS = 30 * 6e4;
var REQUEST_TTL_MS = 30 * 6e4;
var REQUESTS_KEPT = 32;
var WAIT_DEFAULT_S = 300;
var WAIT_MAX_S = 3600;
var PRESENCE_CHECK_MS = 5e3;
var SSE_KEEPALIVE_MS = 25e3;
var WATCH_DEBOUNCE_MS = 100;
var POLL_MS = 2e3;
var MAX_BODY_BYTES = 256 * 1024;
var TooLarge = class extends Error {
};
function sendJson(response, status, body) {
  const text = JSON.stringify(body);
  response.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(text),
    "Cache-Control": "no-store",
    "X-Content-Type-Options": "nosniff"
  });
  response.end(text);
}
function readJson(request) {
  return new Promise((resolve, reject) => {
    if (!/^application\/json\b/i.test(request.headers["content-type"] ?? "")) {
      request.resume();
      reject(new CommentsError("INVALID_CONTENT", "expected application/json", 415));
      return;
    }
    const chunks = [];
    let size = 0;
    request.on("data", (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        reject(new TooLarge());
        request.destroy();
        return;
      }
      chunks.push(chunk);
    });
    request.on("end", () => {
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString("utf8") || "null"));
      } catch {
        reject(new CommentsError("INVALID_CONTENT", "malformed JSON", 400));
      }
    });
    request.on("error", reject);
  });
}
function defaultUserName() {
  try {
    return os.userInfo().username.trim() || "you";
  } catch {
    return "you";
  }
}
function createCommentsService(options) {
  const { root, log, agentHook } = options;
  const userName = (options.userName ?? defaultUserName()).slice(0, 100);
  const owner = { author: { kind: "user", name: userName }, canComment: true, canSend: true, admin: true };
  const viewerOf = options.viewer ?? (() => owner);
  const allowWrite = options.allowWrite ?? (() => true);
  const clients = /* @__PURE__ */ new Map();
  const waiters = [];
  const hookRunning = /* @__PURE__ */ new Set();
  const requests = [];
  const requestFates = /* @__PURE__ */ new Map();
  let lastDoc = emptyDoc();
  let lastWaitEnded = 0;
  let lastSent = "";
  let lastPresence = "";
  let queue = Promise.resolve();
  let closed = false;
  const now = () => (/* @__PURE__ */ new Date()).toISOString();
  function presence() {
    const at = Date.now();
    const listening = waiters.length > 0;
    const busy = lastDoc.threads.some(
      (t) => t.status === "open" && t.agent?.status === "delivered" && at - Date.parse(t.agent.deliveredAt ?? t.agent.sentAt) < AGENT_BUSY_MS
    ) || [...requestFates.values()].some((f) => f.state === "working" && at - f.at < AGENT_BUSY_MS);
    const hook = Boolean(agentHook);
    return { available: listening || busy || hook || at - lastWaitEnded < AGENT_GRACE_MS, listening, busy, hook };
  }
  const meOf = (viewer) => ({
    name: viewer.author.name,
    ...viewer.author.principal ? { principal: viewer.author.principal } : {},
    canComment: viewer.canComment,
    canSend: viewer.canSend,
    admin: viewer.admin
  });
  async function shared() {
    lastDoc = await readComments(root);
    return { threads: lastDoc.threads, agent: presence() };
  }
  async function snapshot(viewer) {
    return { ...await shared(), me: meOf(viewer) };
  }
  const frame = (snap) => `event: comments
data: ${JSON.stringify(snap)}

`;
  function broadcast() {
    queue = queue.then(async () => {
      if (closed) return;
      let base;
      try {
        base = await shared();
      } catch (error) {
        log(`comments: reading ${COMMENTS_FILE} failed: ${error.message}`);
        return;
      }
      const text = JSON.stringify(base);
      lastPresence = JSON.stringify(base.agent);
      if (text === lastSent) return;
      lastSent = text;
      for (const [client, viewer] of clients) client.write(frame({ ...base, me: meOf(viewer) }));
    });
    return queue;
  }
  function finishWaiter(waiter, body) {
    if (waiter.done) return;
    waiter.done = true;
    clearTimeout(waiter.timer);
    const i = waiters.indexOf(waiter);
    if (i >= 0) waiters.splice(i, 1);
    if (!waiter.response.writableEnded) sendJson(waiter.response, 200, body);
    lastWaitEnded = Date.now();
  }
  function setRequestState(id, state) {
    requestFates.delete(id);
    requestFates.set(id, { state, at: Date.now() });
    while (requestFates.size > REQUESTS_KEPT) requestFates.delete(requestFates.keys().next().value);
  }
  function dropStaleRequests() {
    const oldest = Date.now() - REQUEST_TTL_MS;
    while (requests.length > 0 && Date.parse(requests[0].sentAt) < oldest) {
      const dropped = requests.shift();
      setRequestState(dropped.id, "over");
      log(`comments: dropped request ${dropped.id}: no agent took it in ${REQUEST_TTL_MS / 6e4} minutes`);
    }
  }
  async function deliver() {
    if (closed) return;
    dropStaleRequests();
    const waiter = waiters[0];
    if (waiter) {
      const views = await updateComments(root, (doc) => {
        const pending = pendingForAgent(doc);
        const at = now();
        for (const t of pending) markDelivered(t, at, "wait");
        return agentViews(doc, pending);
      });
      const handed = requests.splice(0);
      if (views.length === 0 && handed.length === 0) return;
      if (waiter.done) {
        requests.unshift(...handed);
        if (views.length > 0) {
          await updateComments(root, (doc) => {
            for (const t of doc.threads) if (views.some((v) => v.id === t.id) && t.agent?.via === "wait") t.agent = { status: "pending", sentAt: t.agent.sentAt };
          });
        }
        return;
      }
      finishWaiter(waiter, { threads: views, ...handed.length > 0 ? { requests: handed } : {} });
      for (const request of handed) setRequestState(request.id, "working");
      log(`comments: sent to the agent (wait): ${[...views, ...handed].map((v) => v.id).join(", ")}`);
      await broadcast();
      return;
    }
    if (agentHook) await runHook(agentHook);
  }
  async function runHook(command) {
    if (typeof command !== "string") return runHandler(command);
    const doc = await readComments(root);
    for (const thread of pendingForAgent(doc)) {
      if (hookRunning.has(thread.id)) continue;
      hookRunning.add(thread.id);
      const view = agentViews(doc, [thread])[0];
      const child = spawn(command, {
        shell: true,
        cwd: root,
        stdio: ["pipe", "ignore", "pipe"],
        env: { ...process.env, ARTIFACT_ROOT: root, COMMENT_ID: thread.id, COMMENTS_FILE: commentsPath(root) }
      });
      let stderr = "";
      child.stderr?.on("data", (chunk) => {
        stderr = (stderr + chunk.toString("utf8")).slice(-2e3);
      });
      child.stdin?.on("error", () => {
      });
      child.stdin?.end(`${JSON.stringify(view)}
`);
      child.on("error", (error) => {
        hookRunning.delete(thread.id);
        log(`comments: the agent hook failed to start: ${error.message}`);
      });
      child.on("exit", (code) => {
        if (code !== 0) {
          hookRunning.delete(thread.id);
          log(`comments: the agent hook exited with ${code} for ${thread.id}; the comment stays waiting${stderr ? `: ${stderr.trim()}` : ""}`);
          return;
        }
        void updateComments(root, (d) => {
          const t = d.threads.find((x) => x.id === thread.id);
          if (t) markDelivered(t, now(), "hook");
        }).finally(() => hookRunning.delete(thread.id)).then(() => {
          log(`comments: sent to the agent (hook): ${thread.id}`);
          return broadcast();
        }).catch((error) => log(`comments: ${error.message}`));
      });
    }
    for (const request of requests.splice(0)) {
      hookRunning.add(request.id);
      setRequestState(request.id, "working");
      const child = spawn(command, {
        shell: true,
        cwd: root,
        stdio: ["pipe", "ignore", "pipe"],
        env: { ...process.env, ARTIFACT_ROOT: root, REQUEST_ID: request.id, COMMENTS_FILE: commentsPath(root) }
      });
      let stderr = "";
      child.stderr?.on("data", (chunk) => {
        stderr = (stderr + chunk.toString("utf8")).slice(-2e3);
      });
      child.stdin?.on("error", () => {
      });
      child.stdin?.end(`${JSON.stringify({ request })}
`);
      let settled = false;
      const failed = (why) => {
        if (settled) return;
        settled = true;
        hookRunning.delete(request.id);
        requests.unshift(request);
        setRequestState(request.id, "queued");
        log(`comments: the agent hook ${why} for ${request.id}; the request stays waiting${stderr ? `: ${stderr.trim()}` : ""}`);
      };
      child.on("error", (error) => failed(`failed to start (${error.message})`));
      child.on("exit", (code) => {
        if (code !== 0) return failed(`exited with ${code}`);
        settled = true;
        hookRunning.delete(request.id);
        log(`comments: sent to the agent (hook): ${request.id}`);
      });
    }
  }
  async function runHandler(handler) {
    for (const { id } of pendingForAgent(await readComments(root))) {
      if (hookRunning.has(id)) continue;
      hookRunning.add(id);
      try {
        const doc = await readComments(root);
        const thread = pendingForAgent(doc).find((t) => t.id === id);
        if (!thread) continue;
        try {
          await handler.thread(agentViews(doc, [thread])[0]);
        } catch (error) {
          log(`comments: the agent handler failed for ${id}; the comment stays waiting: ${error.message}`);
          continue;
        }
        await updateComments(root, (d) => {
          const t = d.threads.find((x) => x.id === id);
          if (t) markDelivered(t, now(), "hook");
        });
        log(`comments: sent to the agent (handler): ${id}`);
        await broadcast();
      } finally {
        hookRunning.delete(id);
      }
    }
    const failed = [];
    for (const request of requests.splice(0)) {
      setRequestState(request.id, "working");
      try {
        await handler.request(request);
        log(`comments: sent to the agent (handler): ${request.id}`);
      } catch (error) {
        failed.push(request);
        setRequestState(request.id, "queued");
        log(`comments: the agent handler failed for ${request.id}; the request stays waiting: ${error.message}`);
      }
    }
    requests.unshift(...failed);
  }
  let debounce;
  const changed = () => {
    clearTimeout(debounce);
    debounce = setTimeout(() => void broadcast(), WATCH_DEBOUNCE_MS);
  };
  let watcher = null;
  try {
    watcher = fs2.watch(root, (_event, name) => {
      if (name === null || name === COMMENTS_FILE) changed();
    });
    watcher.on("error", () => {
      watcher?.close();
      watcher = null;
    });
  } catch {
    watcher = null;
  }
  let lastStat = "";
  const poll = setInterval(() => {
    fs2.stat(commentsPath(root), (error, info) => {
      const stat = error ? "none" : `${info.mtimeMs}:${info.size}`;
      if (stat !== lastStat) {
        const first = lastStat === "";
        lastStat = stat;
        if (!first) changed();
      }
    });
    if (JSON.stringify(presence()) !== lastPresence && clients.size > 0) void broadcast();
  }, Math.min(POLL_MS, PRESENCE_CHECK_MS));
  poll.unref?.();
  void broadcast();
  async function afterChange(sent) {
    await broadcast();
    if (sent) await deliver();
  }
  function need(allowed, message) {
    if (!allowed) throw new CommentsError("NOT_ALLOWED", message, 403);
  }
  const ONLY_OWNER_SENDS = "Only the owner can send comments to the agent.";
  async function create(request, response, viewer) {
    const input = readNewThread(await readJson(request));
    if (!input) throw new CommentsError("INVALID_COMMENT", "expected {anchor, label, text} with a non-empty text");
    need(!input.send || viewer.canSend, ONLY_OWNER_SENDS);
    let location = {};
    try {
      location = await options.locate?.(input) ?? {};
    } catch (error) {
      log(`comments: could not resolve where a comment points: ${error.message}`);
    }
    const created = await updateComments(root, (doc) => addThread(doc, input, location, viewer.author, now()));
    log(`comments: ${created.id} added on ${created.label}${location.file ? ` (${location.file})` : ""}${input.send ? ", sent to the agent" : ""}`);
    sendJson(response, 201, { thread: created });
    await afterChange(Boolean(input.send));
  }
  function enqueue(input) {
    if (!presence().available) throw new CommentsError("AGENT_UNAVAILABLE", "no agent is waiting: copy the request for the chat instead", 409);
    const queued = { id: newId("r"), ...input, sentAt: now() };
    requests.push(queued);
    setRequestState(queued.id, "queued");
    log(`comments: request ${queued.id}${input.label ? ` (${input.label})` : ""} for the agent`);
    return queued;
  }
  async function ask(request, response) {
    const input = readAgentRequest(await readJson(request));
    if (!input) throw new CommentsError("INVALID_REQUEST", "expected {text} with a non-empty text");
    const queued = enqueue(input);
    sendJson(response, 202, { request: queued });
    await deliver();
  }
  async function sendRequest(input) {
    const queued = enqueue(input);
    await deliver();
    return queued;
  }
  async function events(request, response, viewer) {
    response.writeHead(200, {
      "Content-Type": "text/event-stream; charset=utf-8",
      "Cache-Control": "no-store",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no"
    });
    response.write(": connected\n\n");
    response.write(frame(await snapshot(viewer)));
    clients.set(response, viewer);
    const keepAlive = setInterval(() => response.write(": ping\n\n"), SSE_KEEPALIVE_MS);
    keepAlive.unref?.();
    response.on("close", () => {
      clearInterval(keepAlive);
      clients.delete(response);
    });
  }
  async function wait(request, response, url) {
    const asked = Number(url.searchParams.get("timeout") ?? WAIT_DEFAULT_S);
    const seconds = Number.isFinite(asked) && asked > 0 ? Math.min(asked, WAIT_MAX_S) : WAIT_DEFAULT_S;
    const waiter = { response, done: false };
    waiters.push(waiter);
    for (const [id, fate] of requestFates) if (fate.state === "working") setRequestState(id, "over");
    waiter.timer = setTimeout(() => {
      finishWaiter(waiter, { threads: [] });
      void broadcast();
    }, seconds * 1e3);
    const gone = () => {
      if (waiter.done) return;
      finishWaiter(waiter, { threads: [] });
      void broadcast();
    };
    response.on("close", gone);
    await broadcast();
    await deliver();
  }
  async function handle(request, response, rest) {
    const method = request.method ?? "GET";
    const url = new URL(request.url ?? "/", "http://localhost");
    const parts = rest.split("/").filter(Boolean);
    const reading = method === "GET" || method === "HEAD";
    try {
      if (!reading && !allowWrite(request)) {
        request.resume();
        throw new CommentsError("NOT_ALLOWED", "cross-origin request", 403);
      }
      const viewer = viewerOf(request);
      if (!viewer) {
        request.resume();
        throw new CommentsError("NOT_ALLOWED", "not shared with you", 403);
      }
      const commenting = !reading && parts[0] !== "agent" && parts[1] !== "send";
      if (commenting && !viewer.canComment) {
        request.resume();
        throw new CommentsError("NOT_ALLOWED", "You can read the comments here, not add or change them.", 403);
      }
      if (parts.length === 0) {
        if (reading) return sendJson(response, 200, await snapshot(viewer));
        if (method === "POST") return await create(request, response, viewer);
      } else if (parts.length === 1 && parts[0] === "events") {
        if (reading) return await events(request, response, viewer);
      } else if (parts.length === 2 && parts[0] === "agent" && parts[1] === "wait") {
        need(viewer.canSend, "Only the owner's agent waits for comments.");
        if (reading) return await wait(request, response, url);
      } else if (parts.length === 2 && parts[0] === "agent" && parts[1] === "send-all") {
        if (method === "POST") {
          request.resume();
          need(viewer.canSend, ONLY_OWNER_SENDS);
          const result = await updateComments(root, (doc) => {
            const open = doc.threads.filter((t) => t.status === "open" && t.messages.some((m) => m.author.kind === "user"));
            const sent = open.filter(canSendToAgent);
            const at = now();
            for (const t of sent) sendToAgent(t, at);
            return { threads: sent, sent: { count: sent.length, open: open.length } };
          });
          log(`comments: ${result.sent.count} of ${result.sent.open} open thread(s) sent to the agent`);
          sendJson(response, 200, result);
          return await afterChange(result.sent.count > 0);
        }
      } else if (parts.length === 2 && parts[0] === "agent" && parts[1] === "requests") {
        need(viewer.canSend, ONLY_OWNER_SENDS);
        if (method === "POST") return await ask(request, response);
      } else if (parts.length === 1) {
        const id = parts[0];
        if (method === "PATCH") {
          const status = readStatus(await readJson(request));
          if (!status) throw new CommentsError("INVALID_STATUS", 'expected {status: "open" | "resolved"}');
          const t = await updateComments(root, (doc) => {
            const found = findThread(doc, id);
            setStatus(found, status, now());
            return found;
          });
          log(`comments: ${id} ${status === "resolved" ? "resolved" : "reopened"}`);
          sendJson(response, 200, { thread: t });
          return await afterChange(false);
        }
        if (method === "DELETE") {
          await updateComments(root, (doc) => {
            const found = findThread(doc, id);
            const first = found.messages[0];
            need(viewer.admin || first !== void 0 && sameAuthor(first.author, viewer.author), "Only the person who started this comment, or the owner, can delete it.");
            doc.threads.splice(doc.threads.indexOf(found), 1);
          });
          log(`comments: ${id} deleted`);
          sendJson(response, 200, { deleted: id });
          return await afterChange(false);
        }
      } else if (parts.length === 2 && parts[1] === "messages") {
        if (method === "POST") {
          const id = parts[0];
          const input = readReply(await readJson(request));
          if (!input) throw new CommentsError("INVALID_COMMENT", "expected {text} with a non-empty text");
          need(!input.send || viewer.canSend, ONLY_OWNER_SENDS);
          const t = await updateComments(root, (doc) => {
            const found = findThread(doc, id);
            addMessage(found, viewer.author, input.text, now());
            if (input.send) sendToAgent(found, now());
            return found;
          });
          log(`comments: reply on ${id}${input.send ? ", sent to the agent" : ""}`);
          sendJson(response, 200, { thread: t });
          return await afterChange(Boolean(input.send));
        }
      } else if (parts.length === 3 && parts[1] === "messages") {
        if (method === "PATCH") {
          const id = parts[0];
          const messageId = parts[2];
          const input = readEdit(await readJson(request));
          if (!input) throw new CommentsError("INVALID_COMMENT", "expected {text} with a non-empty text");
          need(!input.send || viewer.canSend, ONLY_OWNER_SENDS);
          const t = await updateComments(root, (doc) => {
            const found = findThread(doc, id);
            editMessage(found, messageId, input.text, now(), viewer.author);
            if (input.send && canSendToAgent(found)) sendToAgent(found, now());
            return found;
          });
          log(`comments: message edited on ${id}${input.send ? ", sent to the agent" : ""}`);
          sendJson(response, 200, { thread: t });
          return await afterChange(Boolean(input.send));
        }
      } else if (parts.length === 2 && parts[1] === "send") {
        if (method === "POST") {
          const id = parts[0];
          request.resume();
          need(viewer.canSend, ONLY_OWNER_SENDS);
          const t = await updateComments(root, (doc) => {
            const found = findThread(doc, id);
            if (!canSendToAgent(found)) throw new CommentsError("NOTHING_TO_SEND", "the agent already has everything in this comment", 409);
            sendToAgent(found, now());
            return found;
          });
          log(`comments: ${id} sent to the agent`);
          sendJson(response, 200, { thread: t });
          return await afterChange(true);
        }
      } else {
        return sendJson(response, 404, { code: "NOT_FOUND", message: "no such comments route" });
      }
      request.resume();
      sendJson(response, 405, { code: "METHOD_NOT_ALLOWED", message: `${method} is not allowed here` });
    } catch (error) {
      if (response.headersSent) throw error;
      if (error instanceof CommentsError) return sendJson(response, error.status, { code: error.code, message: error.message });
      if (error instanceof TooLarge) return sendJson(response, 413, { code: "TOO_LARGE", message: "request body is too large" });
      throw error;
    }
  }
  function close() {
    closed = true;
    clearInterval(poll);
    clearTimeout(debounce);
    watcher?.close();
    for (const waiter of [...waiters]) finishWaiter(waiter, { threads: [], closed: true });
    for (const client of clients.keys()) client.end();
    clients.clear();
  }
  function requestStates() {
    const states = {};
    for (const [id, fate] of requestFates) if (fate.state !== "over") states[id] = fate.state;
    return states;
  }
  return { handle, presence, deliver, sendRequest, requestStates, close };
}

import { createHash } from "node:crypto";
import fs3 from "node:fs";
import net from "node:net";
import os2 from "node:os";
import path2 from "node:path";
var STATE_FILE = ".artifact-server.json";
var LOG_FILE = ".artifact-server.log";
var EXPORTS_DIR = "exports";
function readState(root) {
  try {
    return JSON.parse(fs3.readFileSync(path2.join(root, STATE_FILE), "utf8"));
  } catch {
    return null;
  }
}
function alive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return error.code === "EPERM";
  }
}
async function runningServer(root, kind, probe) {
  const state = readState(root);
  if (!state || state.kind !== kind || !alive(state.pid)) return null;
  try {
    const response = await fetch(new URL(probe, state.url), { signal: AbortSignal.timeout(2e3) });
    return response.ok ? state : null;
  } catch {
    return null;
  }
}
function writeState(state) {
  fs3.writeFileSync(path2.join(state.root, STATE_FILE), `${JSON.stringify(state, null, 2)}
`);
  writeGitignore(state.root, state.kind);
}
function removeState(root) {
  const state = readState(root);
  if (state && state.pid !== process.pid && alive(state.pid)) return;
  fs3.rmSync(path2.join(root, STATE_FILE), { force: true });
}
var GITIGNORE_FILE = ".gitignore";
var gitignoreLines = (kind) => [STATE_FILE, LOG_FILE, ".sharing.json", ...kind === "docs" ? [] : ["/history/"], `/${EXPORTS_DIR}/`, "*.tmp", ".tmp-*", ".DS_Store"];
function writeGitignore(root, kind) {
  const file = path2.join(root, GITIGNORE_FILE);
  try {
    fs3.writeFileSync(file, ["# Written by baoyu-design when the editor first opened this folder: what only this computer's editor server needs.", ...gitignoreLines(kind), ""].join("\n"), { flag: "wx" });
  } catch {
  }
}
async function listen(server2, port, host) {
  await new Promise((resolve, reject) => {
    server2.once("error", reject);
    server2.listen(port, host, () => resolve());
  });
  const address = server2.address();
  return typeof address === "object" && address ? address.port : port;
}
var DEFAULT_PORTS = { slides: 4710, design: 4720, "design-system": 4730, dashboard: 4740, docs: 4750, motion: 4760 };
var PORT_TRIES = 100;
var PortsTaken = class extends Error {
  code = "PORT_IN_USE";
  from;
  to;
  constructor(from, to) {
    super(`${from === to ? `Port ${from} is` : `Ports ${from} to ${to} are all`} in use; pick another --port (0 takes any free one).`);
    this.from = from;
    this.to = to;
  }
};
var NetworkBlocked = class extends Error {
  code = "NETWORK_BLOCKED";
  constructor() {
    super(
      "A sandbox blocks local network here (127.0.0.1), so the server cannot start and no port is really in use. Run this same command with local network allowed, outside the sandbox, asking for that permission if your commands need approval."
    );
  }
};
function loopbackBlocked(port) {
  return new Promise((resolve) => {
    const socket = net.connect({ host: "127.0.0.1", port });
    const done = (blocked) => {
      socket.destroy();
      resolve(blocked);
    };
    socket.setTimeout(300, () => done(false));
    socket.once("connect", () => done(false));
    socket.once("error", (error) => done(error.code === "EPERM" || error.code === "EACCES"));
  });
}
var isBenchmarkRange = (address) => /^198\.1[89]\./.test(address);
function isTailnet(address) {
  const m = /^100\.(\d{1,3})\./.exec(address);
  return !!m && Number(m[1]) >= 64 && Number(m[1]) <= 127;
}
var isTunnel = (name) => /^(utun|tun|tap|wg|ppp|ipsec)\d/i.test(name);
function lanAddresses() {
  const out = /* @__PURE__ */ new Map();
  for (const [name, list] of Object.entries(os2.networkInterfaces())) {
    for (const entry of list ?? []) {
      if (entry.family !== "IPv4" || entry.internal || entry.address.startsWith("169.254.") || isBenchmarkRange(entry.address)) continue;
      const rank = isTailnet(entry.address) ? 2 : isTunnel(name) ? 1 : 0;
      out.set(entry.address, Math.min(rank, out.get(entry.address) ?? rank));
    }
  }
  return [...out.keys()].sort((a, b) => out.get(a) - out.get(b));
}
function answers(host, port) {
  return new Promise((resolve) => {
    const socket = net.connect({ host, port });
    const done = (taken) => {
      socket.destroy();
      resolve(taken);
    };
    socket.setTimeout(300, () => done(false));
    socket.once("connect", () => done(true));
    socket.once("error", (error) => done(!["ECONNREFUSED", "EADDRNOTAVAIL", "EAFNOSUPPORT", "ENETUNREACH", "EHOSTUNREACH"].includes(error.code ?? "")));
  });
}
async function answersAnyPort(address) {
  const server2 = net.createServer();
  let port;
  try {
    port = await new Promise((resolve, reject) => {
      server2.once("error", reject);
      server2.listen(0, address, () => {
        const bound = server2.address();
        resolve(typeof bound === "object" && bound ? bound.port : 0);
      });
    });
  } catch {
    return false;
  } finally {
    await new Promise((resolve) => server2.close(() => resolve()));
  }
  return port > 0 && answers(address, port);
}
async function listenFree(server2, from, host, first) {
  if (await loopbackBlocked(first || from || 1)) throw new NetworkBlocked();
  if (from === 0) return listen(server2, 0, host);
  const to = Math.min(from + PORT_TRIES - 1, 65535);
  const range = Array.from({ length: to - from + 1 }, (_, i) => from + i);
  const ports = first ? [first, ...range.filter((port) => port !== first)] : range;
  const wildcard = host === "0.0.0.0" || host === "::";
  const lan = lanAddresses().filter((address) => address !== host);
  const answering = await Promise.all(lan.map(answersAnyPort));
  const probes = [.../* @__PURE__ */ new Set([...wildcard ? [] : [host], "127.0.0.1", "::1", ...lan.filter((_, i) => !answering[i])])];
  for (const port of ports) {
    if ((await Promise.all(probes.map((address) => answers(address, port)))).some(Boolean)) continue;
    try {
      return await listen(server2, port, host);
    } catch (error) {
      if (error.code !== "EADDRINUSE") throw error;
    }
  }
  throw new PortsTaken(from, to);
}
function serverOrigin(host, port) {
  const shown2 = host === "0.0.0.0" || host === "::" ? "127.0.0.1" : host.includes(":") ? `[${host}]` : host;
  return `http://${shown2}:${port}`;
}
var pagePathOf = (kind, root) => `/${kind}/${createHash("sha256").update(path2.resolve(root)).digest("base64url").slice(0, 22)}/`;
function sendJson2(response, status, body, headers = {}) {
  const text = JSON.stringify(body);
  response.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(text),
    "Cache-Control": "no-store",
    "X-Content-Type-Options": "nosniff",
    ...headers
  });
  response.end(text);
}
var TooLarge2 = class extends Error {
};
function readBody(request, limit) {
  return new Promise((resolve, reject) => {
    const declared = Number(request.headers["content-length"]);
    if (Number.isFinite(declared) && declared > limit) {
      reject(new TooLarge2());
      request.resume();
      return;
    }
    const chunks = [];
    let size = 0;
    request.on("data", (chunk) => {
      size += chunk.length;
      if (size > limit) {
        reject(new TooLarge2());
        request.destroy();
        return;
      }
      chunks.push(chunk);
    });
    request.on("end", () => resolve(Buffer.concat(chunks)));
    request.on("error", reject);
  });
}
function sameOrigin(request) {
  const origin = request.headers.origin;
  if (origin === void 0) return true;
  const host = request.headers.host;
  if (!host) return false;
  try {
    return new URL(origin).host === host;
  } catch {
    return false;
  }
}
function sendFile(request, response, file, headers) {
  response.writeHead(200, headers);
  if (request.method === "HEAD") {
    response.end();
    return;
  }
  const stream = fs3.createReadStream(file);
  stream.on("error", () => response.destroy());
  response.on("close", () => stream.destroy());
  stream.pipe(response);
}
var errorText = (error) => error instanceof Error ? error.message : String(error);

var MAX_EDIT = 2e3;
function editScript(a, b) {
  const n = a.length;
  const m = b.length;
  const limit = Math.min(n + m, MAX_EDIT);
  const offset = limit + 1;
  const v = new Int32Array(2 * limit + 3);
  const trace = [];
  let found = -1;
  for (let d = 0; d <= limit; d++) {
    trace.push(v.slice(offset - d - 1, offset + d + 2));
    for (let k = -d; k <= d; k += 2) {
      let x2 = k === -d || k !== d && v[offset + k - 1] < v[offset + k + 1] ? v[offset + k + 1] : v[offset + k - 1] + 1;
      let y2 = x2 - k;
      while (x2 < n && y2 < m && a[x2] === b[y2]) {
        x2++;
        y2++;
      }
      v[offset + k] = x2;
      if (x2 >= n && y2 >= m) {
        found = d;
        break;
      }
    }
    if (found >= 0) break;
  }
  if (found < 0) return null;
  const ops = [];
  let x = n;
  let y = m;
  for (let d = found; d > 0; d--) {
    const prev = trace[d];
    const at = (k2) => prev[k2 + d + 1];
    const k = x - y;
    const prevK = k === -d || k !== d && at(k - 1) < at(k + 1) ? k + 1 : k - 1;
    const prevX = at(prevK);
    const prevY = prevX - prevK;
    while (x > prevX && y > prevY) {
      ops.push("=");
      x--;
      y--;
    }
    ops.push(x === prevX ? "+" : "-");
    x = prevX;
    y = prevY;
  }
  while (x > 0 && y > 0) {
    ops.push("=");
    x--;
    y--;
  }
  return ops.reverse();
}
function hunksOf(base, side, mine) {
  const ops = editScript(base, side);
  if (!ops) return null;
  const out = [];
  let open = null;
  let i = 0;
  let j = 0;
  for (const op of ops) {
    if (op === "=") {
      open = null;
      i++;
      j++;
      continue;
    }
    if (!open) out.push(open = { start: i, end: i, lines: [], mine });
    if (op === "-") open.end = ++i;
    else open.lines.push(side[j++]);
  }
  return out.flatMap(
    (h) => h.end - h.start > 1 && h.lines.length === h.end - h.start ? h.lines.flatMap((line, k) => {
      const at = h.start + k;
      return line === base[at] ? [] : [{ start: at, end: at + 1, lines: [line], mine }];
    }) : [h]
  );
}
function applyHunks(base, start, end, hunks) {
  const out = [];
  let at = start;
  for (const h of hunks) {
    out.push(...base.slice(at, h.start), ...h.lines);
    at = h.end;
  }
  out.push(...base.slice(at, end));
  return out;
}
function mergeText(base, mine, current, at, conflicts) {
  const lines = base.split("\n");
  const ours = hunksOf(lines, mine.split("\n"), true);
  const theirs = ours && hunksOf(lines, current.split("\n"), false);
  if (!ours || !theirs) return null;
  const all = [...ours, ...theirs].sort((x, y) => x.start - y.start || x.end - y.end);
  const out = [];
  const lost = [];
  let done = 0;
  for (let i = 0; i < all.length; ) {
    const region = [all[i++]];
    const start = region[0].start;
    let end = region[0].end;
    for (; i < all.length; i++) {
      const h = all[i];
      if (!(h.start < end || h.start === h.end && start === end && h.start === start)) break;
      region.push(h);
      end = Math.max(end, h.end);
    }
    out.push(...lines.slice(done, start));
    const mineLines = applyHunks(lines, start, end, region.filter((h) => h.mine));
    if (region.every((h) => h.mine === region[0].mine)) out.push(...applyHunks(lines, start, end, region));
    else {
      const theirLines = applyHunks(lines, start, end, region.filter((h) => !h.mine));
      if (mineLines.join("\n") !== theirLines.join("\n")) lost.push(`${at}@${start + 1}`);
      out.push(...mineLines);
    }
    done = end;
  }
  out.push(...lines.slice(done));
  conflicts.push(...lost);
  return out.join("\n");
}

var utf8 = new TextDecoder("utf-8", { fatal: true });
function mergeTextBytes(base, mine, current) {
  const text = (bytes) => bytes.includes(0) ? null : utf8.decode(bytes);
  try {
    const [b, m, c] = [text(base), text(mine), text(current)];
    if (b === null || m === null || c === null) return null;
    const merged = mergeText(b, m, c, ".", []);
    return merged === null ? null : Buffer.from(merged, "utf8");
  } catch {
    return null;
  }
}

import path4 from "node:path";

import fs4 from "node:fs";
import path3 from "node:path";
var FOLDER = /^[a-z0-9][a-z0-9_-]{0,63}$/;
var COMPONENT = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/;
var NOT_COMPONENTS = /* @__PURE__ */ new Set(["lib", "src", "api", "archived"]);
var MAX_COMPONENTS = 400;
var MAX_TOKENS = 2e3;
var MAX_README = 24e3;
var MAX_COMPONENT_README = 8e3;
var MAX_TYPES = 4e3;
var MAX_FILE_BYTES = 2 * 1024 * 1024;
var str = (v) => typeof v === "string" ? v : typeof v === "number" ? String(v) : void 0;
function noLink(p) {
  try {
    return fs4.realpathSync(p) === path3.resolve(p);
  } catch {
    return false;
  }
}
function readText(file, max) {
  try {
    if (!noLink(file)) return void 0;
    const stat = fs4.lstatSync(file);
    if (!stat.isFile() || stat.size > MAX_FILE_BYTES) return void 0;
    const text = fs4.readFileSync(file, "utf8");
    return text.length > max ? `${text.slice(0, max)}
\u2026` : text;
  } catch {
    return void 0;
  }
}
function readable(file) {
  return noLink(file) && fs4.lstatSync(file).isFile();
}
function readJson2(file) {
  const text = readText(file, MAX_FILE_BYTES);
  if (text === void 0) return void 0;
  try {
    return JSON.parse(text);
  } catch {
    return void 0;
  }
}
function tokensOf(tokens) {
  const themes = [];
  const families = [];
  let count = 0;
  for (const [name, family] of Object.entries(tokens)) {
    if (name === "type" || !family || typeof family !== "object" || Array.isArray(family)) continue;
    const list = family.tokens;
    if (!Array.isArray(list)) continue;
    if (name === "color") {
      for (const theme of family.themes ?? []) {
        const t = theme;
        const id = str(t?.id);
        if (id) themes.push({ id, name: str(t.name) ?? id });
      }
    }
    const out = [];
    for (const raw of list) {
      if (count >= MAX_TOKENS) break;
      const t = raw;
      const tokenName = str(t?.name);
      if (!tokenName) continue;
      let value = str(t.value);
      if (value === void 0 && t.value && typeof t.value === "object" && !Array.isArray(t.value)) {
        const byTheme = {};
        for (const [k, v] of Object.entries(t.value)) if (str(v) !== void 0) byTheme[k] = str(v);
        value = byTheme;
      }
      if (value === void 0) continue;
      out.push({ name: tokenName, value, ...str(t.usage) ? { usage: str(t.usage) } : {} });
      count += 1;
    }
    if (out.length) families.push({ name, tokens: out });
  }
  families.sort((a, b) => Number(b.name === "color") - Number(a.name === "color"));
  const type = tokens.type ?? {};
  const fontFamilies = {};
  if (type.families && typeof type.families === "object") {
    for (const [k, v] of Object.entries(type.families)) if (str(v)) fontFamilies[k] = str(v);
  }
  const textStyles = [];
  for (const g of Array.isArray(type.groups) ? type.groups : []) {
    const group = g;
    for (const st of Array.isArray(group.styles) ? group.styles : []) {
      const s = st;
      const name = str(s?.name);
      if (!name || textStyles.length >= MAX_TOKENS) continue;
      textStyles.push({
        group: str(group.name) ?? "",
        name,
        ...str(s.family) ?? str(group.family) ? { family: str(s.family) ?? str(group.family) } : {},
        ...str(s.fontSize) ? { fontSize: str(s.fontSize) } : {},
        ...str(s.lineHeight) ? { lineHeight: str(s.lineHeight) } : {},
        ...str(s.fontWeight) ? { fontWeight: str(s.fontWeight) } : {}
      });
    }
  }
  return { themes, families, fontFamilies, textStyles };
}
function summaryOf(readme) {
  const body = readme.split("\n").filter((line) => !/^\s*#/.test(line)).join(" ").replace(/\s+/g, " ").trim();
  if (!body) return void 0;
  const sentence = /^(.+?[.!?。！？])(\s|$)/.exec(body)?.[1] ?? body;
  return sentence.length > 200 ? `${sentence.slice(0, 200)}\u2026` : sentence;
}
function typesOf(dts, name) {
  const lines = dts.split("\n");
  const named = new RegExp(`^\\s*(?:export\\s+)?(?:declare\\s+)?(?:function|const|let|class|interface|type|namespace)\\s+${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}(?:Props)?\\b`);
  const out = [];
  for (let i = 0; i < lines.length; i += 1) {
    if (!named.test(lines[i])) continue;
    let start = i;
    while (start > 0 && /^\s*(\*|\/\*\*|\*\/)/.test(lines[start - 1])) start -= 1;
    let end = i;
    let depth = 0;
    for (; end < lines.length; end += 1) {
      for (const ch of lines[end]) depth += ch === "{" ? 1 : ch === "}" ? -1 : 0;
      if (depth <= 0 && (/[;}]\s*$/.test(lines[end]) || end > i)) break;
    }
    out.push(lines.slice(start, end + 1).join("\n"));
    i = end;
    if (out.join("\n\n").length > MAX_TYPES) break;
  }
  const text = out.join("\n\n");
  return text ? text.length > MAX_TYPES ? `${text.slice(0, MAX_TYPES)}
\u2026` : text : void 0;
}
function componentsOf(dir) {
  const components = path3.join(dir, "components");
  if (!noLink(components)) return [];
  let entries;
  try {
    entries = fs4.readdirSync(components, { withFileTypes: true });
  } catch {
    return [];
  }
  const dts = readText(path3.join(components, "index.d.ts"), MAX_FILE_BYTES) ?? "";
  const out = [];
  for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
    if (out.length >= MAX_COMPONENTS) break;
    if (!entry.isDirectory() || !COMPONENT.test(entry.name) || NOT_COMPONENTS.has(entry.name)) continue;
    const own = path3.join(components, entry.name);
    const readme = readText(path3.join(own, "README.md"), MAX_COMPONENT_README);
    const types = dts ? typesOf(dts, entry.name) : void 0;
    out.push({
      name: entry.name,
      ...readme ? { readme } : {},
      ...readme && summaryOf(readme) ? { summary: summaryOf(readme) } : {},
      ...types ? { types } : {},
      preview: readable(path3.join(own, "preview.html"))
    });
  }
  return out;
}
function insideDs(projectDir, dir) {
  try {
    if (!noLink(dir)) return false;
    const real = fs4.realpathSync(dir);
    const ds = fs4.realpathSync(path3.join(projectDir, "ds"));
    return real === ds || real.startsWith(ds + path3.sep);
  } catch {
    return false;
  }
}
function designSystemOf(projectDir, indexFile, folder) {
  if (!FOLDER.test(folder)) return null;
  const dir = path3.join(projectDir, "ds", folder);
  if (!fs4.existsSync(dir) || !insideDs(projectDir, dir) || !fs4.lstatSync(dir).isDirectory()) return null;
  const index = readJson2(path3.join(projectDir, indexFile));
  const records = Array.isArray(index?.designSystems) ? index.designSystems : [];
  const record = records.find((r) => str(r?.namespace) === folder);
  const tokens = readJson2(path3.join(dir, "tokens.json"));
  const usable = tokens && typeof tokens === "object" && !Array.isArray(tokens) ? tokens : void 0;
  const readme = readText(path3.join(dir, "README.md"), MAX_README);
  return {
    folder,
    title: (str(record?.title) ?? str(usable?.name) ?? folder).replace(/\s+[-\u2013\u2014]\s+design system$/i, ""),
    ...str(record?.version) ? { version: str(record.version) } : {},
    ...str(record?.copiedAt) ? { copiedAt: str(record.copiedAt) } : {},
    ...readme ? { readme } : {},
    ...usable ? tokensOf(usable) : { themes: [], families: [], fontFamilies: {}, textStyles: [] },
    components: componentsOf(dir)
  };
}

var DS_PAGE = /^__ds\/([^/]+)\/$/;
var esc = (text) => text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");
var copyHref = (folder, rel) => `../../project/ds/${folder}/${rel}`;
function linkHref(href, folder, dir) {
  if (/^(https?:|mailto:)/i.test(href)) return href;
  if (/^[a-z][a-z0-9+.-]*:/i.test(href) || href.startsWith("/") || href.startsWith("#")) return null;
  const rel = path4.posix.normalize(path4.posix.join(dir, href));
  return rel.startsWith("..") ? null : copyHref(folder, rel);
}
var INLINE = /(`+)(.+?)\1|!?\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)|\*\*(.+?)\*\*|\*([^*\s][^*]*?)\*/g;
function inline(text, folder, dir) {
  let out = "";
  let at = 0;
  for (const m of text.matchAll(INLINE)) {
    out += esc(text.slice(at, m.index));
    at = m.index + m[0].length;
    if (m[2] !== void 0) out += `<code>${esc(m[2])}</code>`;
    else if (m[4] !== void 0) {
      const href = linkHref(m[4], folder, dir);
      const label = inline(m[3] || m[4], folder, dir);
      out += href === null ? label : `<a href="${esc(href)}" rel="noreferrer">${label}</a>`;
    } else if (m[5] !== void 0) out += `<strong>${inline(m[5], folder, dir)}</strong>`;
    else out += `<em>${inline(m[6], folder, dir)}</em>`;
  }
  return out + esc(text.slice(at));
}
var tableCells = (line) => line.trim().replace(/^\|/, "").replace(/\|$/, "").split("|").map((cell) => cell.trim());
function markdown(source, folder, dir) {
  const lines = source.replace(/\r\n/g, "\n").replace(/^---\n[\s\S]*?\n---\n/, "").split("\n");
  const out = [];
  let i = 0;
  const startsBlock = (line) => /^\s*(#{1,6}\s|```|>|[-*+]\s|\d+[.)]\s)/.test(line) || /^\s*([-*_])\1{2,}\s*$/.test(line);
  while (i < lines.length) {
    const line = lines[i];
    const fence = /^\s*(```+)/.exec(line);
    if (fence) {
      const code = [];
      for (i += 1; i < lines.length && !lines[i].trim().startsWith(fence[1]); i += 1) code.push(lines[i]);
      i += 1;
      out.push(`<pre><code>${esc(code.join("\n"))}</code></pre>`);
    } else if (!line.trim()) {
      i += 1;
    } else if (/^#{1,6}\s/.test(line)) {
      const hashes = /^#+/.exec(line)[0].length;
      const level = Math.min(hashes + 3, 6);
      out.push(`<h${level}>${inline(line.slice(hashes).trim().replace(/\s#+$/, ""), folder, dir)}</h${level}>`);
      i += 1;
    } else if (/^\s*([-*_])\1{2,}\s*$/.test(line)) {
      out.push("<hr>");
      i += 1;
    } else if (line.includes("|") && /^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/.test(lines[i + 1] ?? "")) {
      const rows = [`<tr>${tableCells(line).map((c) => `<th>${inline(c, folder, dir)}</th>`).join("")}</tr>`];
      for (i += 2; i < lines.length && lines[i].includes("|"); i += 1) {
        rows.push(`<tr>${tableCells(lines[i]).map((c) => `<td>${inline(c, folder, dir)}</td>`).join("")}</tr>`);
      }
      out.push(`<div class="scroll"><table>${rows.join("")}</table></div>`);
    } else if (/^\s*>/.test(line)) {
      const quote = [];
      for (; i < lines.length && /^\s*>/.test(lines[i]); i += 1) quote.push(lines[i].replace(/^\s*>\s?/, ""));
      out.push(`<blockquote>${markdown(quote.join("\n"), folder, dir)}</blockquote>`);
    } else if (/^\s*([-*+]|\d+[.)])\s/.test(line)) {
      const ordered = /^\s*\d/.test(line);
      const items = [];
      for (; i < lines.length && /^\s*([-*+]|\d+[.)])\s/.test(lines[i]); i += 1) {
        const item = /^(\s*)(?:[-*+]|\d+[.)])\s+(.*)$/.exec(lines[i]);
        const depth = Math.floor(item[1].length / 2);
        items.push(`<li${depth ? ` style="margin-left:${Math.min(depth, 4)}em"` : ""}>${inline(item[2], folder, dir)}</li>`);
      }
      out.push(`<${ordered ? "ol" : "ul"}>${items.join("")}</${ordered ? "ol" : "ul"}>`);
    } else {
      const text = [];
      for (; i < lines.length && lines[i].trim() && !startsBlock(lines[i]); i += 1) text.push(lines[i].trim());
      if (!text.length) text.push(lines[i++].trim());
      out.push(`<p>${inline(text.join(" "), folder, dir)}</p>`);
    }
  }
  return out.join("\n");
}
function colorOf(token2, theme, all, depth = 0) {
  const value = typeof token2.value === "string" ? token2.value : token2.value[theme ?? ""] ?? Object.values(token2.value)[0] ?? "";
  const ref = /^\{([^{}]+)\}$/.exec(value.trim());
  const target = ref ? all.get(ref[1]) : void 0;
  return target && depth < 8 ? colorOf(target, theme, all, depth + 1) : value;
}
function colorsSection(family, system) {
  const all = new Map(family.tokens.map((token2) => [token2.name, token2]));
  const columns = system.themes.length ? system.themes : [{ id: "", name: "" }];
  const head = system.themes.length ? `<tr><th></th>${columns.map((t) => `<th>${esc(t.name)}</th>`).join("")}<th></th></tr>` : "";
  const rows = family.tokens.map((token2) => {
    const cells = columns.map((theme) => {
      const value = colorOf(token2, theme.id || void 0, all);
      return `<td><span class="swatch" style="background:${esc(value)}"></span><code>${esc(value)}</code></td>`;
    });
    return `<tr><th scope="row">${esc(token2.name)}</th>${cells.join("")}<td class="muted">${esc(token2.usage ?? "")}</td></tr>`;
  });
  return `<h3>Colours</h3><div class="scroll"><table class="tokens">${head}${rows.join("")}</table></div>`;
}
var tokenValue = (token2) => typeof token2.value === "string" ? token2.value : Object.entries(token2.value).map(([theme, value]) => `${theme}: ${value}`).join(", ");
function familySection(family) {
  const rows = family.tokens.map(
    (token2) => `<tr><th scope="row">${esc(token2.name)}</th><td><code>${esc(tokenValue(token2))}</code></td><td class="muted">${esc(token2.usage ?? "")}</td></tr>`
  );
  const title = family.name.charAt(0).toUpperCase() + family.name.slice(1);
  return `<h3>${esc(title)}</h3><div class="scroll"><table class="tokens">${rows.join("")}</table></div>`;
}
function textSection(system) {
  if (!system.textStyles.length) return "";
  const rows = system.textStyles.map((style) => {
    const stack = style.family ? system.fontFamilies[style.family] ?? style.family : void 0;
    const css = [
      stack ? `font-family:${stack}` : "",
      style.fontSize ? `font-size:${style.fontSize}` : "",
      style.lineHeight ? `line-height:${style.lineHeight}` : "",
      style.fontWeight ? `font-weight:${style.fontWeight}` : ""
    ].filter(Boolean).join(";");
    const facts = [style.fontSize, style.lineHeight, style.fontWeight, style.family].filter(Boolean).join(" \xB7 ");
    return `<tr><td class="sample" style="${esc(css)}">${esc(style.name)}</td><td class="muted">${esc(style.group)}</td><td><code>${esc(facts)}</code></td></tr>`;
  });
  return `<h3>Text styles</h3><div class="scroll"><table class="tokens">${rows.join("")}</table></div>`;
}
function componentBlock(component, folder) {
  const dir = `components/${component.name}`;
  const preview = component.preview ? `<p><a href="${esc(copyHref(folder, `${dir}/preview.html`))}">Open its preview</a></p>` : "";
  const body = component.readme?.replace(/^\s*#\s+.*\n?/, "");
  const readme = body?.trim() ? `<div class="md">${markdown(body, folder, dir)}</div>` : "";
  const types = component.types ? `<h4>Types</h4><pre><code>${esc(component.types)}</code></pre>` : "";
  const empty = preview || readme || types ? "" : '<p class="muted">This component has no guidelines, types or preview.</p>';
  return `<details><summary><strong>${esc(component.name)}</strong>${component.summary ? `<span class="muted">${inline(component.summary, folder, dir)}</span>` : ""}</summary><div class="body">${preview}${readme}${types}${empty}</div></details>`;
}
var STYLE = `
:root{color-scheme:light dark;--ink:#1f1e1d;--muted:#6f6e69;--line:#e4e2dc;--ground:#faf9f7;--panel:#fff;--code:#f1efea}
@media (prefers-color-scheme:dark){:root{--ink:#ecebe7;--muted:#a3a29c;--line:#3a3936;--ground:#1f1e1d;--panel:#262624;--code:#32312e}}
*{box-sizing:border-box}
body{margin:0;background:var(--ground);color:var(--ink);font:14px/1.55 system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:960px;margin:0 auto;padding:40px 24px 80px}
h1{font-size:28px;line-height:1.2;margin:0 0 6px}
h2{font-size:20px;margin:44px 0 12px;padding-bottom:8px;border-bottom:1px solid var(--line)}
h3{font-size:15px;margin:24px 0 8px}
h4,h5,h6{font-size:14px;margin:18px 0 6px}
a{color:inherit;text-underline-offset:2px}
code{font:12px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}
p code,li code,td code{background:var(--code);padding:1px 4px;border-radius:4px}
pre{background:var(--code);padding:12px;border-radius:8px;overflow-x:auto;margin:10px 0}
nav{display:flex;flex-wrap:wrap;gap:16px;margin-top:14px}
hr{border:0;border-top:1px solid var(--line);margin:18px 0}
blockquote{margin:10px 0;padding-left:12px;border-left:3px solid var(--line);color:var(--muted)}
table{border-collapse:collapse;width:100%}
th,td{padding:7px 12px 7px 0;border-bottom:1px solid var(--line);text-align:left;vertical-align:middle;font-weight:400}
tr:first-child th:not([scope]){color:var(--muted);font-size:12px}
.md th{font-weight:600}
th[scope]{font-weight:500;white-space:nowrap}
.scroll{overflow-x:auto}
.muted{color:var(--muted)}
.swatch{display:inline-block;width:18px;height:18px;border-radius:5px;vertical-align:middle;margin-right:7px;box-shadow:inset 0 0 0 1px rgba(127,127,127,.35)}
.tokens td code{background:none;padding:0;white-space:nowrap}
.sample{white-space:nowrap}
details{background:var(--panel);border:1px solid var(--line);border-radius:8px;margin:8px 0}
summary{cursor:pointer;padding:10px 14px}
summary .muted{margin-left:10px}
.body{padding:2px 14px 12px;border-top:1px solid var(--line)}
@media (max-width:600px){main{padding:24px 16px 60px}summary .muted{display:block;margin:2px 0 0}}
`;
function designSystemPage(projectDir, indexFile, folder, noun) {
  const system = designSystemOf(projectDir, indexFile, folder);
  if (system === null) return null;
  const colors = system.families.find((family) => family.name === "color");
  const others = system.families.filter((family) => family !== colors);
  const hasTokens = system.families.length > 0 || system.textStyles.length > 0;
  const facts = [
    `Read-only copy this ${noun} carries, in <code>project/ds/${esc(folder)}/</code>`,
    ...system.version ? [`version ${esc(system.version)}`] : [],
    ...system.copiedAt ? [`copied ${esc(system.copiedAt.slice(0, 10))}`] : []
  ].join(" \xB7 ");
  const nav = [
    hasTokens ? '<a href="#tokens">Tokens</a>' : "",
    system.components.length ? `<a href="#components">Components (${system.components.length})</a>` : "",
    system.readme ? '<a href="#guide">Guide</a>' : ""
  ].filter(Boolean).join("");
  const sections = [
    hasTokens ? `<section id="tokens"><h2>Tokens</h2>${colors ? colorsSection(colors, system) : ""}${textSection(system)}${others.map(familySection).join("")}</section>` : "",
    system.components.length ? `<section id="components"><h2>Components</h2>${system.components.map((component) => componentBlock(component, folder)).join("")}</section>` : "",
    system.readme ? `<section id="guide"><h2>Guide</h2><div class="md">${markdown(system.readme, folder, "")}</div></section>` : ""
  ].filter(Boolean).join("");
  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${esc(system.title)}</title>
<style>${STYLE}</style>
</head>
<body>
<main>
<h1>${esc(system.title)}</h1>
<p class="muted">${facts}</p>
${nav ? `<nav>${nav}</nav>` : ""}
${sections || '<p class="muted">This copy has no tokens, components or guide.</p>'}
</main>
</body>
</html>
`;
}

import { EventEmitter } from "node:events";
import fs6 from "node:fs";
import http from "node:http";
import os3 from "node:os";
import path6 from "node:path";

import crypto from "node:crypto";
import fs5 from "node:fs";
import path5 from "node:path";
var GUEST_ROLES = ["editor", "commenter", "viewer"];
var isGuestRole = (value) => typeof value === "string" && GUEST_ROLES.includes(value);
var SHARING_FILE = ".sharing.json";
var MAX_GUESTS = 200;
var MAX_NAME = 40;
var token = (bytes = 18) => crypto.randomBytes(bytes).toString("base64url");
var isRecord2 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
function fresh() {
  return { lan: false, links: { editor: { token: token(), on: true }, commenter: { token: token(), on: true }, viewer: { token: token(), on: true } }, guests: [] };
}
function cleanName(raw) {
  const name = typeof raw === "string" ? raw.replace(/[\u0000-\u001f\u007f@<>]/g, "").replace(/\s+/g, " ").trim() : "";
  return [...name].slice(0, MAX_NAME).join("");
}
function guestUser(guest) {
  return { id: `guest-${guest.id}`, name: guest.name, email: `${guest.name}@${guest.id}.guest` };
}
function isLoopback(address) {
  if (!address) return false;
  const a = address.startsWith("::ffff:") ? address.slice(7) : address;
  return a === "::1" || a.startsWith("127.");
}
var Sharing = class _Sharing {
  file;
  state;
  constructor(root) {
    this.file = path5.join(root, SHARING_FILE);
    this.state = _Sharing.read(this.file);
  }
  static read(file) {
    let raw;
    try {
      raw = JSON.parse(fs5.readFileSync(file, "utf8"));
    } catch {
      return fresh();
    }
    const state = fresh();
    if (!isRecord2(raw)) return state;
    state.lan = raw.lan === true;
    if (typeof raw.port === "number" && Number.isInteger(raw.port) && raw.port > 0 && raw.port < 65536) state.port = raw.port;
    if (isRecord2(raw.links)) {
      for (const role of GUEST_ROLES) {
        const link = raw.links[role];
        if (isRecord2(link) && typeof link.token === "string" && link.token.length >= 16) state.links[role] = { token: link.token, on: link.on !== false };
      }
    }
    if (Array.isArray(raw.guests)) {
      for (const g of raw.guests) {
        if (!isRecord2(g) || typeof g.id !== "string" || typeof g.secret !== "string" || !isGuestRole(g.role) || typeof g.link !== "string") continue;
        state.guests.push({
          id: g.id,
          secret: g.secret,
          name: cleanName(g.name) || "Guest",
          role: g.role,
          link: g.link,
          joined: typeof g.joined === "string" ? g.joined : (/* @__PURE__ */ new Date(0)).toISOString(),
          ...typeof g.seen === "string" ? { seen: g.seen } : {}
        });
      }
    }
    return state;
  }
  save() {
    const tmp = `${this.file}.${process.pid}.tmp`;
    fs5.writeFileSync(tmp, `${JSON.stringify(this.state, null, 2)}
`, { mode: 384 });
    fs5.renameSync(tmp, this.file);
  }
  /** The role whose link has this token, while that link is on. */
  linkRole(tokenGiven) {
    for (const role of GUEST_ROLES) {
      const link = this.state.links[role];
      if (link.on && tokenGiven.length === link.token.length && crypto.timingSafeEqual(Buffer.from(tokenGiven), Buffer.from(link.token))) return role;
    }
    return null;
  }
  /** The guest a cookie names, if they still have access (their link on and not reset). */
  guest(secret) {
    if (!secret) return null;
    const guest = this.state.guests.find((g) => g.secret === secret);
    return guest && this.valid(guest) ? guest : null;
  }
  valid(guest) {
    const link = this.state.links[guest.role];
    return link.on && link.token === guest.link && this.state.guests.includes(guest);
  }
  join(role, name) {
    const guest = { id: token(4), secret: token(24), name: cleanName(name) || "Guest", role, link: this.state.links[role].token, joined: (/* @__PURE__ */ new Date()).toISOString() };
    this.state.guests.push(guest);
    if (this.state.guests.length > MAX_GUESTS) this.state.guests.splice(0, this.state.guests.length - MAX_GUESTS);
    this.save();
    return guest;
  }
  /** Marks a guest seen now (saved at most once a minute per guest). */
  seen(guest) {
    const now = /* @__PURE__ */ new Date();
    if (guest.seen && now.getTime() - Date.parse(guest.seen) < 6e4) return;
    guest.seen = now.toISOString();
    this.save();
  }
  setLan(on, port) {
    this.state.lan = on;
    if (on && port) this.state.port = port;
    this.save();
  }
  setLink(role, on) {
    this.state.links[role].on = on;
    this.save();
  }
  /** A new token for the link: the old link stops working and the guests who came through it are dropped. */
  resetLink(role) {
    const old = this.state.links[role].token;
    this.state.links[role] = { token: token(), on: this.state.links[role].on };
    this.state.guests = this.state.guests.filter((g) => g.link !== old);
    this.save();
  }
  /**
   * Gives a guest another role. They now hang on that role's link (turning it off or resetting it drops them), so a role
   * whose link is off would shut them out: false then, as for a guest who isn't there.
   */
  setRole(id, role) {
    const guest = this.state.guests.find((g) => g.id === id);
    if (!guest || !this.state.links[role].on) return false;
    guest.role = role;
    guest.link = this.state.links[role].token;
    this.save();
    return true;
  }
  removeGuest(id) {
    const before = this.state.guests.length;
    this.state.guests = this.state.guests.filter((g) => g.id !== id);
    if (this.state.guests.length === before) return false;
    this.save();
    return true;
  }
};

import { createHash as createHash2 } from "node:crypto";
var GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";
var MAX_MESSAGE = 64 * 1024 * 1024;
var WebSocketConnection = class {
  socket;
  buffer = Buffer.alloc(0);
  parts = [];
  partOpcode = 0;
  closed = false;
  handlers = {};
  constructor(socket, head) {
    this.socket = socket;
    socket.on("data", (chunk) => this.receive(chunk));
    socket.on("close", () => this.finish());
    socket.on("error", () => this.finish());
    if (head.length) this.receive(head);
  }
  get open() {
    return !this.closed;
  }
  sendText(text) {
    this.send(1, Buffer.from(text, "utf8"));
  }
  sendBinary(data) {
    this.send(2, data);
  }
  close(code = 1e3, reason = "") {
    if (this.closed) return;
    const body = Buffer.alloc(2 + Buffer.byteLength(reason));
    body.writeUInt16BE(code, 0);
    body.write(reason, 2);
    this.send(8, body);
    this.socket.end();
    this.finish();
  }
  finish() {
    if (this.closed) return;
    this.closed = true;
    this.socket.destroy();
    this.handlers.close?.();
  }
  send(opcode, payload) {
    if (this.closed) return;
    const length = payload.length;
    let header;
    if (length < 126) {
      header = Buffer.from([128 | opcode, length]);
    } else if (length < 65536) {
      header = Buffer.alloc(4);
      header[0] = 128 | opcode;
      header[1] = 126;
      header.writeUInt16BE(length, 2);
    } else {
      header = Buffer.alloc(10);
      header[0] = 128 | opcode;
      header[1] = 127;
      header.writeBigUInt64BE(BigInt(length), 2);
    }
    this.socket.write(Buffer.concat([header, payload]));
  }
  receive(chunk) {
    this.buffer = this.buffer.length ? Buffer.concat([this.buffer, chunk]) : chunk;
    for (; ; ) {
      if (this.buffer.length < 2) return;
      const first = this.buffer[0];
      const second = this.buffer[1];
      const fin = (first & 128) !== 0;
      const opcode = first & 15;
      const masked = (second & 128) !== 0;
      let length = second & 127;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      } else if (length === 127) {
        if (this.buffer.length < 10) return;
        const big = this.buffer.readBigUInt64BE(2);
        if (big > BigInt(MAX_MESSAGE)) return this.close(1009, "message too big");
        length = Number(big);
        offset = 10;
      }
      const maskLength = masked ? 4 : 0;
      if (this.buffer.length < offset + maskLength + length) return;
      const mask = masked ? this.buffer.subarray(offset, offset + 4) : null;
      const payload = Buffer.from(this.buffer.subarray(offset + maskLength, offset + maskLength + length));
      if (mask) for (let index = 0; index < payload.length; index++) payload[index] = payload[index] ^ mask[index & 3];
      this.buffer = this.buffer.subarray(offset + maskLength + length);
      this.frame(fin, opcode, payload);
      if (this.closed) return;
    }
  }
  frame(fin, opcode, payload) {
    switch (opcode) {
      case 8:
        this.close(payload.length >= 2 ? payload.readUInt16BE(0) : 1e3);
        return;
      case 9:
        this.send(10, payload);
        return;
      case 10:
        return;
      case 0:
      case 1:
      case 2: {
        if (opcode !== 0) {
          this.parts = [];
          this.partOpcode = opcode;
        }
        this.parts.push(payload);
        if (this.parts.reduce((total, part) => total + part.length, 0) > MAX_MESSAGE) return this.close(1009, "message too big");
        if (!fin) return;
        const message = Buffer.concat(this.parts);
        this.parts = [];
        if (this.partOpcode === 1) this.handlers.text?.(message.toString("utf8"));
        else this.handlers.binary?.(message);
        return;
      }
      default:
        this.close(1002, "unknown opcode");
    }
  }
};
function acceptWebSocket(request, socket, head, protocol) {
  const key = request.headers["sec-websocket-key"];
  if (typeof key !== "string" || String(request.headers.upgrade).toLowerCase() !== "websocket") {
    socket.end("HTTP/1.1 400 Bad Request\r\n\r\n");
    return null;
  }
  const accept = createHash2("sha1").update(key + GUID).digest("base64");
  socket.write(["HTTP/1.1 101 Switching Protocols", "Upgrade: websocket", "Connection: Upgrade", `Sec-WebSocket-Accept: ${accept}`, ...protocol ? [`Sec-WebSocket-Protocol: ${protocol}`] : [], "", ""].join("\r\n"));
  return new WebSocketConnection(socket, head);
}

var HUB_ROUTE = "/__hub";
var STREAM_PROTOCOL = "baoyu-sse";
var STREAM_REFUSED = 4e3;
function buildOf(script2) {
  const real = fs6.realpathSync(script2);
  const { mtimeMs, size } = fs6.statSync(real);
  return `${real}:${mtimeMs}:${size}`;
}
var SiteClash = class extends Error {
};
var StreamResponse = class extends EventEmitter {
  status = 200;
  headersSent = false;
  writableEnded = false;
  destroyed = false;
  ws;
  constructor(ws) {
    super();
    this.ws = ws;
  }
  writeHead(status) {
    this.status = status;
    this.headersSent = true;
    return this;
  }
  setHeader() {
    return this;
  }
  flushHeaders() {
  }
  write(chunk) {
    if (this.status === 200 && !this.writableEnded) this.ws.sendText(String(chunk));
    return true;
  }
  end(chunk) {
    if (this.writableEnded) return this;
    if (chunk !== void 0) this.write(chunk);
    this.writableEnded = true;
    this.ws.close(this.status === 200 ? 1e3 : STREAM_REFUSED + Math.min(this.status, 999));
    return this;
  }
  destroy() {
    this.destroyed = true;
    return this.end();
  }
  closed() {
    this.writableEnded = true;
    this.emit("close");
  }
};
var Hub = class {
  port = 0;
  kind;
  host;
  build;
  factory;
  /** Sites' logs go to its stderr too: it runs in someone's terminal, not in the background. */
  echo;
  server;
  /** page → site. */
  sites = /* @__PURE__ */ new Map();
  adding = /* @__PURE__ */ new Map();
  startedAt = /* @__PURE__ */ new Map();
  hosts = /* @__PURE__ */ new Set();
  /** One listener per LAN address while any site is open to the network. */
  lanServers = /* @__PURE__ */ new Map();
  network;
  constructor(options) {
    this.echo = options.echo;
    this.kind = options.kind;
    this.build = options.build;
    this.host = options.host;
    this.factory = options.site;
    this.server = this.createServer();
    const hub = this;
    this.network = {
      get port() {
        return hub.port;
      },
      addresses: () => [...this.lanServers.keys()],
      open: () => this.openLan(),
      release: () => this.releaseLan()
    };
  }
  createServer() {
    const server2 = http.createServer((request, response) => {
      this.onRequest(request, response).catch((error) => {
        process.stderr.write(`[${(/* @__PURE__ */ new Date()).toISOString()}] request failed: ${error instanceof Error ? error.stack ?? error.message : String(error)}
`);
        if (!response.headersSent) sendJson2(response, 500, { code: "internal", message: "internal error" });
        else response.destroy();
      });
    });
    server2.on("upgrade", (request, socket, head) => this.onUpgrade(request, socket, head));
    return server2;
  }
  /** Listens at the first free port from `port`; `first` is tried before them. */
  async listen(port, first) {
    this.port = await listenFree(this.server, port, this.host, first);
    const named = this.host === "0.0.0.0" || this.host === "::" ? [] : [this.host.includes(":") ? `[${this.host}]:${this.port}` : `${this.host}:${this.port}`];
    this.hosts = /* @__PURE__ */ new Set([`127.0.0.1:${this.port}`, `localhost:${this.port}`, `[::1]:${this.port}`, ...named]);
    return this.port;
  }
  /** Ends every site and the listeners; what the folders' state files say goes with them. */
  close() {
    for (const site of [...this.sites.values()]) this.drop(site);
    this.closeLan();
    this.server.close();
    this.server.closeAllConnections?.();
  }
  /** On a signal (or a stray error, logged): every site's record goes and the process exits. */
  ownProcess() {
    const shutdown = () => {
      this.close();
      process.exit(0);
    };
    for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) process.on(signal, shutdown);
    const stray = (error) => process.stderr.write(`[${(/* @__PURE__ */ new Date()).toISOString()}] uncaught: ${error instanceof Error ? error.stack ?? error.message : String(error)}
`);
    process.on("uncaughtException", stray);
    process.on("unhandledRejection", stray);
  }
  // ---- the sites -------------------------------------------------------------------------------------------------
  urlOf(site) {
    return `${serverOrigin(this.host, this.port)}${site.page}`;
  }
  stateOf(site) {
    const lan = site.share.lanUrls();
    return { pid: process.pid, url: this.urlOf(site), port: this.port, root: site.root, kind: this.kind, startedAt: this.startedAt.get(site.page) ?? (/* @__PURE__ */ new Date()).toISOString(), ...lan.length ? { lan } : {} };
  }
  /** Mounts a folder (already mounted: the same site), writing its <root>/.artifact-server.json. */
  add(root, options) {
    const mounted = [...this.sites.values()].find((site) => site.root === root);
    if (mounted) return (options.lan && !mounted.share.shared ? mounted.share.setLan(true) : Promise.resolve()).then(() => this.stateOf(mounted));
    const adding = this.adding.get(root);
    if (adding) return adding;
    const work = this.mount(root, options).finally(() => this.adding.delete(root));
    this.adding.set(root, work);
    return work;
  }
  async mount(root, { agentHook, lan = false }) {
    const file = path6.join(root, LOG_FILE);
    const log = (line) => {
      const text = `[${(/* @__PURE__ */ new Date()).toISOString()}] ${line}
`;
      try {
        fs6.appendFileSync(file, text);
      } catch {
      }
      if (this.echo) process.stderr.write(text);
    };
    const site = await this.factory(root, { log, network: this.network, ...agentHook ? { agentHook } : {} });
    const other = this.sites.get(site.page);
    if (other) {
      site.close();
      throw new SiteClash(`${other.root} is already open at ${site.page}.`);
    }
    this.sites.set(site.page, site);
    this.startedAt.set(site.page, (/* @__PURE__ */ new Date()).toISOString());
    try {
      await site.share.start(lan);
    } catch (error) {
      this.drop(site);
      throw error;
    }
    const state = this.stateOf(site);
    const { lan: _lan, ...record } = state;
    writeState(record);
    log(`serving ${root} at ${state.url}${agentHook ? `; each comment sent to the agent runs: ${agentHook}` : ""}`);
    return state;
  }
  drop(site) {
    this.sites.delete(site.page);
    this.startedAt.delete(site.page);
    removeState(site.root);
    site.close();
  }
  /** Ends one site; true when one was mounted there. The process ends with its last site. */
  remove(root) {
    const site = [...this.sites.values()].find((candidate) => candidate.root === root);
    if (site) this.drop(site);
    return site !== void 0;
  }
  // ---- the network side ------------------------------------------------------------------------------------------
  async openLan() {
    const addresses = lanAddresses().filter((address) => !this.lanServers.has(address));
    const answering = await Promise.all(addresses.map(answersAnyPort));
    for (const address of addresses.filter((_, i) => !answering[i])) {
      const server2 = this.createServer();
      try {
        await listen(server2, this.port, address);
        this.lanServers.set(address, server2);
      } catch (error) {
        server2.close();
        process.stderr.write(`[${(/* @__PURE__ */ new Date()).toISOString()}] not listening on ${address}:${this.port}: ${errorText(error)}
`);
      }
    }
    return [...this.lanServers.keys()];
  }
  /** A site stopped sharing: the listeners stay while another is shared. */
  releaseLan() {
    if (![...this.sites.values()].some((site) => site.share.shared)) this.closeLan();
  }
  closeLan() {
    for (const server2 of this.lanServers.values()) {
      server2.close();
      server2.closeAllConnections?.();
    }
    this.lanServers.clear();
  }
  /** A Host header this hub answers: its own names, and while something is shared, its LAN addresses and machine name. */
  hostAllowed(request) {
    const host = request.headers.host ?? "";
    if (this.hosts.has(host)) return true;
    if (!this.lanServers.size) return false;
    const suffix = `:${this.port}`;
    if (!host.endsWith(suffix)) return false;
    const name = host.slice(0, -suffix.length).toLowerCase();
    const machine = os3.hostname().toLowerCase();
    return this.lanServers.has(name) || name === machine || name === `${machine.replace(/\.local$/, "")}.local`;
  }
  // ---- requests --------------------------------------------------------------------------------------------------
  /** The site whose address `pathname` is under, with the path below it. */
  under(pathname) {
    const [, first, second] = pathname.split("/");
    const site = first && second ? this.sites.get(`/${first}/${second}/`) : void 0;
    if (site) return { site, rest: pathname.slice(site.page.length - 1) };
    for (const candidate of this.sites.values()) if (candidate.claims?.(pathname)) return { site: candidate, rest: pathname };
    return null;
  }
  /** The site a page at this request's Referer belongs to. */
  referred(request) {
    const referer = request.headers.referer;
    if (typeof referer !== "string") return null;
    try {
      const at = new URL(referer);
      return at.host === request.headers.host ? this.under(at.pathname)?.site ?? null : null;
    } catch {
      return null;
    }
  }
  /** The sites this request's person may see. */
  seenBy(request) {
    return [...this.sites.values()].filter((site) => site.share.viewerOf(request));
  }
  async onRequest(request, response) {
    if (!this.hostAllowed(request)) {
      request.resume();
      response.writeHead(421, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" }).end("Misdirected request");
      return;
    }
    const url = new URL(request.url ?? "/", "http://local");
    const pathname = url.pathname;
    if (pathname === HUB_ROUTE) return this.control(request, response);
    if (pathname === "/favicon.ico") {
      response.writeHead(204).end();
      return;
    }
    if (pathname === "/" || pathname === "/index.html") return this.landing(request, response, url.search);
    const at = this.under(pathname);
    if (at) {
      if (at.rest === "") {
        request.resume();
        response.writeHead(308, { Location: `${pathname}/${url.search}`, "Cache-Control": "no-store" }).end();
        return;
      }
      request.url = `${at.rest}${url.search}`;
      return at.site.handle(request, response);
    }
    const site = await this.answering(request, pathname);
    if (site) return site.handle(request, response);
    request.resume();
    response.writeHead(404, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" }).end("Not found");
  }
  /** The site that answers a path outside every site's address. */
  async answering(request, pathname) {
    const link = /^\/s\/([A-Za-z0-9_-]{16,64})$/.exec(pathname);
    if (link) return [...this.sites.values()].find((site) => site.share.sharing.linkRole(link[1])) ?? null;
    const blob = /^\/_blob\/([0-9a-f]{32})$/.exec(pathname);
    if (blob) {
      const referred = this.referred(request);
      const seen = this.seenBy(request);
      for (const site of referred && seen.includes(referred) ? [referred, ...seen] : seen) if (await site.hasBlob?.(blob[1])) return site;
      return null;
    }
    for (const site of this.sites.values()) if (site.common?.(pathname)) return site;
    return this.referred(request);
  }
  /** `/`: where this person goes. */
  landing(request, response, search) {
    const seen = this.seenBy(request);
    const only = seen[0];
    if (seen.length === 1 && only) {
      request.resume();
      response.writeHead(302, { Location: `${only.page}${search}`, "Cache-Control": "no-store" }).end();
      return;
    }
    const any = only ?? [...this.sites.values()][0];
    if (!any) {
      request.resume();
      response.writeHead(404, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" }).end("Nothing is open here.");
      return;
    }
    if (!only) return any.share.denied(request, response);
    any.share.sendList(response, seen.map((site) => ({ page: site.page, title: site.share.title() })));
  }
  /** The CLIs' route: over loopback, from a process (a browser's POST carries an Origin). */
  async control(request, response) {
    const info = { hub: 1, kind: this.kind, build: this.build, host: this.host, pid: process.pid };
    if (!isLoopback(request.socket.remoteAddress) || request.headers.origin !== void 0) {
      request.resume();
      return sendJson2(response, 403, { code: "not_granted", message: "Only a local process can do this." });
    }
    if (request.method === "GET") return sendJson2(response, 200, info);
    if (request.method !== "POST") {
      request.resume();
      return sendJson2(response, 405, { code: "method_not_allowed", message: "GET or POST." });
    }
    let body;
    try {
      body = JSON.parse((await readBody(request, 64 << 10)).toString("utf8"));
    } catch {
      return sendJson2(response, 400, { code: "invalid_argument", message: "The call is not JSON." });
    }
    if (typeof body.root !== "string" || !path6.isAbsolute(body.root)) return sendJson2(response, 400, { code: "invalid_argument", message: "root: an absolute path." });
    if (body.action === "mount") {
      try {
        const state = await this.add(body.root, { ...typeof body.agentHook === "string" && body.agentHook ? { agentHook: body.agentHook } : {}, lan: body.lan === true });
        return sendJson2(response, 200, { ...info, state });
      } catch (error) {
        sendJson2(response, error instanceof SiteClash ? 409 : 400, { ...info, code: error.code ?? "SERVER_FAILED", message: errorText(error) });
        return this.endIfEmpty();
      }
    }
    if (body.action === "unmount") {
      const removed = this.remove(body.root);
      const remaining = this.sites.size;
      sendJson2(response, 200, { ...info, removed, remaining });
      return this.endIfEmpty();
    }
    return sendJson2(response, 400, { code: "invalid_argument", message: "action: mount or unmount." });
  }
  /** The process ends with its last site. */
  endIfEmpty() {
    if (this.sites.size > 0 || this.adding.size > 0) return;
    this.close();
    setImmediate(() => process.exit(0));
  }
  onUpgrade(request, socket, head) {
    socket.on("error", () => {
    });
    const url = new URL(request.url ?? "/", "http://local");
    const at = this.hostAllowed(request) && this.originOk(request) ? this.under(url.pathname) : null;
    if (!at) {
      socket.end("HTTP/1.1 403 Forbidden\r\n\r\n");
      return;
    }
    request.url = `${at.rest}${url.search}`;
    const asked = String(request.headers["sec-websocket-protocol"] ?? "").split(",").map((p) => p.trim());
    if (asked.includes(STREAM_PROTOCOL)) return this.stream(at.site, request, socket, head, url);
    if (at.site.upgrade) return at.site.upgrade(request, socket, head);
    socket.end("HTTP/1.1 404 Not Found\r\n\r\n");
  }
  originOk(request) {
    const origin = request.headers.origin;
    if (origin === void 0) return true;
    try {
      return new URL(origin).host === request.headers.host;
    } catch {
      return false;
    }
  }
  /** An event stream over a WebSocket: the site's handler answers it as a GET. */
  stream(site, request, socket, head, url) {
    const ws = acceptWebSocket(request, socket, head, STREAM_PROTOCOL);
    if (!ws) return;
    const last = url.searchParams.get("__lastEventId");
    if (last !== null) {
      request.headers["last-event-id"] = last;
      url.searchParams.delete("__lastEventId");
      request.url = `${request.url?.split("?")[0]}${url.search}`;
    }
    request.method = "GET";
    const response = new StreamResponse(ws);
    ws.handlers = {
      close: () => {
        response.closed();
        request.emit("close");
      }
    };
    Promise.resolve(site.handle(request, response)).catch(() => ws.close(STREAM_REFUSED + 500));
  }
};
async function hubAt(origin) {
  try {
    const response = await fetch(`${origin}${HUB_ROUTE}`, { signal: AbortSignal.timeout(1e3) });
    if (!response.ok) return null;
    const info = await response.json();
    return info.hub === 1 ? info : null;
  } catch {
    return null;
  }
}
async function joinHub(call, below = Infinity) {
  if (call.port === 0) return null;
  const ports = call.portGiven ? [call.port] : Array.from({ length: PORT_TRIES }, (_, i) => call.port + i).filter((port) => port <= 65535 && port < below);
  const build = buildOf(call.script);
  const origins = ports.map((port) => serverOrigin(call.host, port));
  const infos = await Promise.all(origins.map(hubAt));
  for (const [i, info] of infos.entries()) {
    if (!info || info.kind !== call.kind || info.build !== build || info.host !== call.host) continue;
    const response = await fetch(`${origins[i]}${HUB_ROUTE}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "mount", root: call.root, ...call.agentHook ? { agentHook: call.agentHook } : {}, lan: call.lan === true }),
      signal: AbortSignal.timeout(2e4)
    }).catch(() => null);
    if (!response) continue;
    const body = await response.json().catch(() => ({}));
    if (response.ok && body.state) return body.state;
    if (response.status === 409) continue;
    throw Object.assign(new Error(body.message ?? `the server answered ${response.status}`), { code: body.code ?? "SERVER_FAILED" });
  }
  return null;
}
async function leaveHub(state) {
  let remaining = 0;
  const response = await fetch(`${new URL(state.url).origin}${HUB_ROUTE}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "unmount", root: state.root }),
    signal: AbortSignal.timeout(5e3)
  }).catch(() => null);
  const body = response ? await response.json().catch(() => null) : null;
  if (body?.hub === 1) remaining = Number(body.remaining ?? 0);
  else if (alive(state.pid)) process.kill(state.pid, "SIGTERM");
  const deadline = Date.now() + 5e3;
  while (remaining === 0 && alive(state.pid) && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 100));
  if (remaining === 0 && alive(state.pid)) return false;
  fs6.rmSync(path6.join(state.root, STATE_FILE), { force: true });
  return true;
}
function holdSite(state) {
  const leave = () => {
    void leaveHub(state).finally(() => process.exit(0));
  };
  for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) process.on(signal, leave);
  setInterval(() => {
    if (!alive(state.pid) || readState(state.root)?.pid !== state.pid) process.exit(0);
  }, 2e3);
}
async function serveHub(call) {
  const joined = await joinHub(call);
  const stayed = (state) => {
    if (!call.background) holdSite(state);
    return { ...state, joined: true };
  };
  if (joined) return stayed(joined);
  const hub = new Hub({ kind: call.kind, build: buildOf(call.script), host: call.host, site: call.site, echo: !call.background });
  const { state: sharing } = new Sharing(call.root);
  const linksPort = !call.portGiven && (call.lan || sharing.lan) ? sharing.port : void 0;
  const port = await hub.listen(call.port, linksPort);
  const lower = call.portGiven ? null : await joinHub(call, port);
  if (lower) {
    hub.close();
    return stayed(lower);
  }
  hub.ownProcess();
  try {
    return { ...await hub.add(call.root, { ...call.agentHook ? { agentHook: call.agentHook } : {}, lan: call.lan === true }), joined: false };
  } catch (error) {
    hub.close();
    throw error;
  }
}

import fs7 from "node:fs";
import path7 from "node:path";
var FOLDER2 = /^[a-z0-9][a-z0-9_-]{0,63}$/;
var MAX_FILES = 2e3;
var MAX_FILE_BYTES2 = 64 * 1024 * 1024;
var DESIGN_SYSTEM_FILE = "design-system.json";
function readRecords(project, indexFile) {
  let index;
  try {
    index = JSON.parse(fs7.readFileSync(path7.join(project, indexFile), "utf8"));
  } catch {
    return [];
  }
  const list = index?.designSystems;
  const records = Array.isArray(list) ? list : list && typeof list === "object" ? Object.values(list) : [];
  const out = [];
  for (const raw of records) {
    const r = raw;
    if (typeof r?.namespace !== "string" || !FOLDER2.test(r.namespace)) continue;
    out.push({
      namespace: r.namespace,
      title: typeof r.title === "string" ? r.title : r.namespace,
      artifact: typeof r.artifact === "string" ? r.artifact : ""
    });
  }
  return out;
}
function plain(p, want) {
  const stat = fs7.lstatSync(p, { throwIfNoEntry: false });
  return Boolean(stat && !stat.isSymbolicLink() && (want === "file" ? stat.isFile() : stat.isDirectory()));
}
function walk(dir, prefix = "", out = []) {
  for (const entry of fs7.readdirSync(path7.join(dir, prefix), { withFileTypes: true })) {
    if (out.length >= MAX_FILES) break;
    const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) walk(dir, rel, out);
    else if (entry.isFile()) out.push(rel);
  }
  return out;
}
var projectOf = (candidate) => path7.basename(candidate) === "project" ? candidate : path7.join(candidate, "project");
function sourceProject(artifact, root, from) {
  const given = from ?? artifact;
  if (!given) return { reason: "the record names no source folder", kind: "none" };
  if (!from && /^[a-z][a-z0-9+.-]*:\/\//i.test(given)) return { reason: `its source is ${given}, which is not a folder on this machine`, kind: "remote" };
  const candidates = path7.isAbsolute(given) ? [given] : [path7.resolve(given), path7.resolve(root, given), path7.resolve(root, "..", given)];
  for (const candidate of candidates) {
    const project = projectOf(candidate);
    if (plain(project, "dir")) return { project };
  }
  if (!from && !path7.isAbsolute(given)) {
    let dir = path7.resolve(root, "..");
    for (let up = path7.dirname(dir); up !== dir; up = path7.dirname(dir)) {
      dir = up;
      const project = projectOf(path7.resolve(dir, given));
      if (plain(path7.join(project, DESIGN_SYSTEM_FILE), "file")) return { project };
    }
  }
  return { reason: `its source folder ${given} was not found${from ? "" : ` (tried from ${process.cwd()} and the folders above the artifact); pass --from <folder>`}`, kind: "not-found" };
}
function compareSystem(project, root, record, from) {
  const diff = { namespace: record.namespace, title: record.title, changed: [], removed: [] };
  const found = sourceProject(record.artifact, root, from);
  if (!found.project) return { ...diff, skipped: found.reason, skippedKind: found.kind };
  diff.source = path7.dirname(found.project);
  const copy = path7.join(project, "ds", record.namespace);
  if (!plain(copy, "dir")) return { ...diff, missing: true };
  for (const rel of walk(copy)) {
    const mine = path7.join(copy, rel);
    const theirs = path7.join(found.project, rel);
    if (!plain(mine, "file")) continue;
    if (!plain(theirs, "file")) {
      diff.removed.push(rel);
      continue;
    }
    const a = fs7.statSync(mine);
    const b = fs7.statSync(theirs);
    if (a.size > MAX_FILE_BYTES2 || b.size > MAX_FILE_BYTES2) continue;
    if (a.size !== b.size || !fs7.readFileSync(mine).equals(fs7.readFileSync(theirs))) diff.changed.push(rel);
  }
  return diff;
}
function systemStatus(project, root, indexFile, namespace) {
  const record = readRecords(project, indexFile).find((r) => r.namespace === namespace);
  if (!record) return null;
  const diff = compareSystem(project, root, record);
  if (diff.skippedKind) return { state: "unknown", changed: 0, removed: 0, reason: diff.skippedKind };
  if (diff.missing) return { state: "unknown", changed: 0, removed: 0, reason: "missing" };
  return { state: diff.changed.length > 0 ? "behind" : "current", changed: diff.changed.length, removed: diff.removed.length };
}
var DS_SYNC = /^__ds-sync\/([a-z0-9][a-z0-9_-]{0,63})$/;
function pick(records, namespace) {
  if (namespace === void 0) return records;
  const one = records.filter((r) => r.namespace === namespace);
  if (one.length === 0) {
    const names = records.map((r) => r.namespace);
    throw new Error(`No designSystems record has namespace ${JSON.stringify(namespace)}${names.length ? ` (it has: ${names.join(", ")})` : ""}.`);
  }
  return one;
}
function compareSystems(project, root, indexFile, namespace, from) {
  return pick(readRecords(project, indexFile), namespace).map((r) => compareSystem(project, root, r, from));
}
function stamp(project, indexFile, namespaces, now) {
  const file = path7.join(project, indexFile);
  const text = fs7.readFileSync(file, "utf8");
  const index = JSON.parse(text);
  const list = index.designSystems;
  const records = Array.isArray(list) ? list : list && typeof list === "object" ? Object.values(list) : [];
  for (const r of records) {
    const record = r;
    if (record && typeof record.namespace === "string" && namespaces.has(record.namespace)) record.copiedAt = now;
  }
  const indent = /^\{\n/.test(text) ? /\n( +|\t)"/.exec(text)?.[1] ?? 2 : void 0;
  const out = JSON.stringify(index, null, indent);
  const tmp = `${file}.${process.pid}.tmp`;
  fs7.writeFileSync(tmp, text.endsWith("\n") ? `${out}
` : out);
  fs7.renameSync(tmp, file);
}
function syncSystems(project, root, indexFile, opts) {
  const diffs = compareSystems(project, root, indexFile, opts.namespace, opts.from);
  const results = [];
  const synced = /* @__PURE__ */ new Set();
  const now = (/* @__PURE__ */ new Date()).toISOString().replace(/\.\d{3}Z$/, "Z");
  for (const diff of diffs) {
    const result = { ...diff, copied: [] };
    if (!opts.check && diff.source && !diff.missing) {
      for (const rel of diff.changed) {
        const target = path7.join(project, "ds", diff.namespace, rel);
        const tmp = `${target}.${process.pid}.tmp`;
        fs7.copyFileSync(path7.join(diff.source, "project", rel), tmp);
        fs7.renameSync(tmp, target);
        result.copied.push(rel);
      }
      if (diff.changed.length > 0) synced.add(diff.namespace);
    }
    results.push(result);
  }
  if (synced.size > 0) {
    stamp(project, indexFile, synced, now);
    for (const r of results) if (synced.has(r.namespace)) r.copiedAt = now;
  }
  return results;
}

import { createHash as createHash3, randomBytes } from "node:crypto";
import fs8 from "node:fs";
import path8 from "node:path";
var HISTORY_DIR = "history";
var INDEX = "index.json";
var OBJECTS = "objects";
var IDLE_MS = 3e4;
var SPAN_MS = 5 * 6e4;
var MAX_VERSIONS = 500;
var MAX_NOTICES = 50;
var NOTICE_DAYS = 7;
var AGENT = { by: "agent", user: { id: "agent", name: "Agent" } };
var SYSTEM = { by: "system", user: { id: "system", name: "" } };
var sha256 = (bytes) => createHash3("sha256").update(bytes).digest("hex");
var SHA = /^[0-9a-f]{64}$/;
var isRecord3 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
function writeAtomic2(file, data) {
  fs8.mkdirSync(path8.dirname(file), { recursive: true });
  const tmp = path8.join(path8.dirname(file), `.${path8.basename(file)}.${process.pid}.${randomBytes(4).toString("hex")}.tmp`);
  try {
    fs8.writeFileSync(tmp, data);
    fs8.renameSync(tmp, file);
  } catch (error) {
    fs8.rmSync(tmp, { force: true });
    throw error;
  }
}
var emptyIndex = () => ({ format: 1, next: 1, base: {}, versions: [], notices: [] });
function readUser(value) {
  return isRecord3(value) && typeof value.id === "string" && typeof value.name === "string" ? { id: value.id, name: value.name } : null;
}
var isAuthor = (value) => value === "human" || value === "agent" || value === "system";
function parseIndex(raw) {
  const index = emptyIndex();
  if (!isRecord3(raw)) return index;
  if (isRecord3(raw.base)) {
    for (const [p, s] of Object.entries(raw.base)) if (typeof s === "string" && SHA.test(s)) index.base[p] = s;
  }
  if (Array.isArray(raw.versions)) {
    for (const v of raw.versions) {
      if (!isRecord3(v) || typeof v.n !== "number" || !isAuthor(v.by) || typeof v.at !== "string" || !isRecord3(v.changes)) continue;
      const user = readUser(v.user);
      if (!user) continue;
      const changes = {};
      for (const [p, s] of Object.entries(v.changes)) if (s === null || typeof s === "string" && SHA.test(s)) changes[p] = s;
      const version = { n: v.n, first: typeof v.first === "string" ? v.first : v.at, at: v.at, by: v.by, user, changes };
      if (typeof v.role === "string") version.role = v.role;
      if (typeof v.restoredFrom === "number") version.restoredFrom = v.restoredFrom;
      if (version.restoredFrom !== void 0 && v.restoredPart === true) version.restoredPart = true;
      if (Array.isArray(v.overwrote)) {
        const list = [];
        for (const o of v.overwrote) {
          const who = isRecord3(o) ? readUser(o.user) : null;
          if (!isRecord3(o) || !who || typeof o.path !== "string" || o.lost !== null && typeof o.lost !== "string" || typeof o.n !== "number" || !isAuthor(o.by)) continue;
          list.push({
            path: o.path,
            lost: o.lost,
            n: o.n,
            by: o.by,
            user: who,
            ...o.soft === true ? { soft: true } : {},
            ...o.chosen === true ? { chosen: true } : {},
            ...typeof o.base === "string" && SHA.test(o.base) ? { base: o.base } : {}
          });
        }
        if (list.length) version.overwrote = list;
      }
      index.versions.push(version);
    }
  }
  index.versions.sort((a, b) => a.n - b.n);
  const last = index.versions.at(-1);
  index.next = Math.max(typeof raw.next === "number" ? raw.next : 1, (last?.n ?? 0) + 1);
  if (Array.isArray(raw.notices)) {
    for (const n of raw.notices) {
      if (!isRecord3(n) || typeof n.id !== "string" || typeof n.to !== "string" || typeof n.path !== "string" || typeof n.n !== "number") continue;
      if (n.kind !== "overwritten" && n.kind !== "overwrote") continue;
      const by = readUser(n.by);
      const lost = isRecord3(n.lost) ? n.lost : null;
      const lostUser = lost ? readUser(lost.user) : null;
      if (!by || !lost || !lostUser || lost.sha !== null && typeof lost.sha !== "string" || typeof lost.n !== "number" || !isAuthor(lost.by)) continue;
      index.notices.push({
        id: n.id,
        to: n.to,
        at: typeof n.at === "string" ? n.at : (/* @__PURE__ */ new Date(0)).toISOString(),
        kind: n.kind,
        path: n.path,
        n: n.n,
        by,
        byAuthor: isAuthor(n.byAuthor) ? n.byAuthor : "human",
        lost: { sha: lost.sha, n: lost.n, by: lost.by, user: lostUser },
        ...n.soft === true ? { soft: true } : {},
        ...typeof n.base === "string" && SHA.test(n.base) ? { base: n.base } : {}
      });
    }
  }
  return index;
}
var History = class {
  dir;
  index;
  /** path → sha: what the files hold after the newest version. */
  headState;
  constructor(root) {
    this.dir = path8.join(root, HISTORY_DIR);
    let raw = null;
    try {
      raw = JSON.parse(fs8.readFileSync(path8.join(this.dir, INDEX), "utf8"));
    } catch {
    }
    this.index = parseIndex(raw);
    this.headState = this.fold(Infinity);
    this.index.notices = this.index.notices.filter((n) => (this.headState.get(n.path) ?? null) !== n.lost.sha);
  }
  get empty() {
    return this.index.versions.length === 0 && Object.keys(this.index.base).length === 0;
  }
  /** What the files hold now, as far as the history knows (path → sha). */
  head() {
    return this.headState;
  }
  versions() {
    return this.index.versions;
  }
  version(n) {
    return this.index.versions.find((v) => v.n === n);
  }
  /** The files after the versions numbered up to `upto` (the base and every version at or below it). */
  fold(upto) {
    const state = new Map(Object.entries(this.index.base));
    for (const version of this.index.versions) {
      if (version.n > upto) break;
      for (const [p, s] of Object.entries(version.changes)) {
        if (s === null) state.delete(p);
        else state.set(p, s);
      }
    }
    return state;
  }
  /** The files right after version `n`; null when it is not kept. */
  stateAt(n) {
    return this.version(n) ? this.fold(n) : null;
  }
  /** The files right before version `n`. */
  stateBefore(n) {
    return this.fold(n - 1);
  }
  /** The content with this sha, from the objects. */
  read(sha) {
    if (!SHA.test(sha)) return null;
    try {
      return fs8.readFileSync(path8.join(this.dir, OBJECTS, sha));
    } catch {
      return null;
    }
  }
  /** Keeps a content that no version holds (an overwrite's `base`); its sha. */
  put(bytes) {
    const sha = sha256(bytes);
    this.putObject(sha, bytes);
    return sha;
  }
  size(sha) {
    if (!SHA.test(sha)) return null;
    try {
      return fs8.statSync(path8.join(this.dir, OBJECTS, sha)).size;
    } catch {
      return null;
    }
  }
  /** The newest version that changed `path`, with what it left there. */
  lastChange(p) {
    for (let i = this.index.versions.length - 1; i >= 0; i--) {
      const version = this.index.versions[i];
      if (p in version.changes) return { version, sha: version.changes[p] };
    }
    return null;
  }
  /**
   * Records changes: path → its content now (null: deleted). Those equal to the head are left out; nothing left, no
   * version (null). The rest join the open version of the same person, or start a new one.
   */
  record(changes, who, extra = {}, now = /* @__PURE__ */ new Date()) {
    const changed = {};
    for (const [p, bytes] of changes) {
      const sha = bytes === null ? null : sha256(bytes);
      if ((this.headState.get(p) ?? null) === sha) continue;
      if (bytes !== null && sha !== null) this.putObject(sha, bytes);
      changed[p] = sha;
    }
    if (Object.keys(changed).length === 0) return null;
    const iso = now.toISOString();
    const last = this.index.versions.at(-1);
    const joins = !!last && who.by !== "system" && last.by === who.by && last.user.id === who.user.id && last.restoredFrom === void 0 && extra.restoredFrom === void 0 && now.getTime() - Date.parse(last.at) < IDLE_MS && now.getTime() - Date.parse(last.first) < SPAN_MS;
    let version;
    if (joins) {
      version = last;
      version.at = iso;
      Object.assign(version.changes, changed);
      if (who.user.name) version.user = { ...who.user };
    } else {
      version = { n: this.index.next++, first: iso, at: iso, by: who.by, user: { ...who.user }, changes: changed };
      if (who.role) version.role = who.role;
      if (extra.restoredFrom !== void 0) version.restoredFrom = extra.restoredFrom;
      if (extra.restoredFrom !== void 0 && extra.restoredPart) version.restoredPart = true;
      this.index.versions.push(version);
    }
    for (const [p, s] of Object.entries(changed)) {
      if (s === null) this.headState.delete(p);
      else this.headState.set(p, s);
    }
    this.index.notices = this.index.notices.filter(
      (n) => !(n.path in changed && (changed[n.path] === n.lost.sha || extra.restoredFrom !== void 0 && n.to === who.user.id))
    );
    if (extra.overwrote?.length) {
      version.overwrote = [...(version.overwrote ?? []).filter((o) => !extra.overwrote.some((x) => x.path === o.path)), ...extra.overwrote];
      this.notify(version, who, extra.overwrote, now);
    }
    this.prune(now);
    this.save();
    return version;
  }
  /** Notices for an overwrite: to whoever lost their change (the agent's go to the owner), and a hard one's saver unless they chose it. */
  notify(version, who, overwrote, now) {
    const at = now.toISOString();
    for (const o of overwrote) {
      const lost = { sha: o.lost, n: o.n, by: o.by, user: o.user };
      const victim = o.by === "agent" ? "local" : o.user.id;
      const fields = {
        at,
        path: o.path,
        n: version.n,
        by: { ...who.user },
        byAuthor: who.by,
        lost,
        ...o.soft ? { soft: true } : {},
        ...o.base ? { base: o.base } : {}
      };
      const same = (n) => n.path === o.path && n.n === version.n;
      this.index.notices = this.index.notices.filter((n) => !same(n));
      if (victim !== who.user.id) this.index.notices.push({ id: randomBytes(6).toString("hex"), to: victim, kind: "overwritten", ...fields });
      if (!o.soft && !o.chosen) this.index.notices.push({ id: randomBytes(6).toString("hex"), to: who.user.id, kind: "overwrote", ...fields });
    }
  }
  /** The notices for this person, newest first. */
  noticesFor(userId) {
    return this.index.notices.filter((n) => n.to === userId).reverse();
  }
  notice(id) {
    return this.index.notices.find((n) => n.id === id);
  }
  /** Dismisses one of this person's notices; false when they have none by that id. */
  dismiss(userId, id) {
    const before = this.index.notices.length;
    this.index.notices = this.index.notices.filter((n) => !(n.id === id && n.to === userId));
    if (this.index.notices.length === before) return false;
    this.save();
    return true;
  }
  /**
   * What to write to bring version `n` back: path → content (null: delete), for `paths` (default: every file it or the
   * head holds), only where the head differs. null when the version is not kept or a content is missing.
   */
  restoreTo(n, paths, tracked = () => true) {
    const target = this.stateAt(n);
    if (!target) return null;
    const out = /* @__PURE__ */ new Map();
    const all = paths ?? [.../* @__PURE__ */ new Set([...target.keys(), ...this.headState.keys()])];
    for (const p of all) {
      if (!tracked(p)) continue;
      const want = target.get(p) ?? null;
      if ((this.headState.get(p) ?? null) === want) continue;
      if (want === null) {
        out.set(p, null);
        continue;
      }
      const bytes = this.read(want);
      if (!bytes) return null;
      out.set(p, bytes);
    }
    return out;
  }
  /** Each version's changes as added, edited or deleted (what the file held before it decides). */
  fileChanges() {
    const out = /* @__PURE__ */ new Map();
    const state = new Map(Object.entries(this.index.base));
    for (const version of this.index.versions) {
      const list = [];
      for (const [p, s] of Object.entries(version.changes)) {
        const had = state.has(p);
        list.push({ path: p, op: s === null ? "deleted" : had ? "edited" : "added" });
        if (s === null) state.delete(p);
        else state.set(p, s);
      }
      out.set(version.n, list);
    }
    return out;
  }
  putObject(sha, bytes) {
    const file = path8.join(this.dir, OBJECTS, sha);
    if (fs8.existsSync(file)) return;
    writeAtomic2(file, bytes);
  }
  /** Past MAX_VERSIONS the oldest fold into the base; old notices go; contents no one holds are removed. */
  prune(now) {
    let folded = false;
    while (this.index.versions.length > MAX_VERSIONS) {
      const old = this.index.versions.shift();
      for (const [p, s] of Object.entries(old.changes)) {
        if (s === null) delete this.index.base[p];
        else this.index.base[p] = s;
      }
      folded = true;
    }
    const oldest = now.getTime() - NOTICE_DAYS * 864e5;
    const kept = this.index.versions[0]?.n ?? Infinity;
    const perPerson = /* @__PURE__ */ new Map();
    this.index.notices = this.index.notices.filter((n) => Date.parse(n.at) >= oldest && n.lost.n >= kept).reverse().filter((n) => {
      const count = (perPerson.get(n.to) ?? 0) + 1;
      perPerson.set(n.to, count);
      return count <= MAX_NOTICES;
    }).reverse();
    if (folded) this.collect();
  }
  /** Removes the objects that neither the base, a version nor a notice holds (with the base an overwrite was made on). */
  collect() {
    const keep = new Set(Object.values(this.index.base));
    for (const version of this.index.versions) {
      for (const s of Object.values(version.changes)) if (s) keep.add(s);
      for (const o of version.overwrote ?? []) if (o.base) keep.add(o.base);
    }
    for (const notice of this.index.notices) {
      if (notice.lost.sha) keep.add(notice.lost.sha);
      if (notice.base) keep.add(notice.base);
    }
    let names;
    try {
      names = fs8.readdirSync(path8.join(this.dir, OBJECTS));
    } catch {
      return;
    }
    for (const name of names) if (SHA.test(name) && !keep.has(name)) fs8.rmSync(path8.join(this.dir, OBJECTS, name), { force: true });
  }
  save() {
    writeAtomic2(path8.join(this.dir, INDEX), `${JSON.stringify(this.index)}
`);
  }
};
var MAX_DIFF_BYTES = 1024 * 1024;
var CONTEXT = 3;
var MAX_DIFF_LINES = 3e3;
var MAX_LINE = 2e3;
var decoder = new TextDecoder("utf-8", { fatal: true });
function textOf(bytes) {
  if (bytes === null) return null;
  if (bytes.includes(0)) return void 0;
  try {
    return decoder.decode(bytes);
  } catch {
    return void 0;
  }
}
var splitLines = (text) => text === null || text === "" ? [] : text.replace(/\r\n?/g, "\n").replace(/\n$/, "").split("\n");
function diffContents(before, after) {
  if ((before?.length ?? 0) > MAX_DIFF_BYTES || (after?.length ?? 0) > MAX_DIFF_BYTES) return { tooLarge: true, added: 0, removed: 0, lines: [] };
  const a = textOf(before);
  const b = textOf(after);
  if (a === void 0 || b === void 0) return { binary: true, added: 0, removed: 0, lines: [] };
  const left = splitLines(a);
  const right = splitLines(b);
  let head = 0;
  while (head < left.length && head < right.length && left[head] === right[head]) head++;
  let tail = 0;
  while (tail < left.length - head && tail < right.length - head && left[left.length - 1 - tail] === right[right.length - 1 - tail]) tail++;
  const middle = editScript(left.slice(head, left.length - tail), right.slice(head, right.length - tail));
  if (!middle) return { tooLarge: true, added: 0, removed: 0, lines: [] };
  const ops = [...Array(head).fill("="), ...middle, ...Array(tail).fill("=")];
  const all = [];
  let i = 0;
  let j = 0;
  let added = 0;
  let removed = 0;
  const cut = (s) => s.length > MAX_LINE ? `${s.slice(0, MAX_LINE)}\u2026` : s;
  for (const op of ops) {
    if (op === "=") all.push([" ", cut(right[j++])]), i++;
    else if (op === "-") all.push(["-", cut(left[i++])]), removed++;
    else all.push(["+", cut(right[j++])]), added++;
  }
  const keep = new Uint8Array(all.length);
  all.forEach(([kind], at) => {
    if (kind === " ") return;
    for (let x = Math.max(0, at - CONTEXT); x <= Math.min(all.length - 1, at + CONTEXT); x++) keep[x] = 1;
  });
  const lines = [];
  let skipped = 0;
  let truncated = false;
  for (let at = 0; at < all.length; at++) {
    if (!keep[at]) {
      skipped++;
      continue;
    }
    if (skipped) lines.push(["~", String(skipped)]);
    skipped = 0;
    if (lines.length >= MAX_DIFF_LINES) {
      truncated = true;
      break;
    }
    lines.push(all[at]);
  }
  if (skipped && !truncated) lines.push(["~", String(skipped)]);
  return { added, removed, lines, ...truncated ? { truncated: true } : {} };
}

import { createHash as createHash4 } from "node:crypto";
import fs11 from "node:fs";
import os5 from "node:os";
import path11 from "node:path";

import fs9 from "node:fs";
import fsp2 from "node:fs/promises";
import path9 from "node:path";
import { promisify } from "node:util";
import zlib from "node:zlib";
var LOCAL_FILES = /* @__PURE__ */ new Set([STATE_FILE, LOG_FILE, SHARING_FILE]);
var LOCAL_DIRS = /* @__PURE__ */ new Set([EXPORTS_DIR]);
var JUNK_FILES = /* @__PURE__ */ new Set([".DS_Store", "Thumbs.db", "desktop.ini", ".git"]);
var isTemp = (name) => name.endsWith(".tmp") || name.startsWith(".tmp-");
var STORED = /* @__PURE__ */ new Set([".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".heic", ".mp4", ".m4v", ".mov", ".webm", ".mp3", ".m4a", ".aac", ".ogg", ".opus", ".flac", ".woff", ".woff2", ".zip", ".gz", ".br", ".zst", ".7z"]);
var MAX_ENTRIES = 65535;
var MAX_BYTES = 4294967295;
var ArchiveTooLarge = class extends Error {
  code = "TOO_LARGE";
};
async function archiveEntries(root, skip = []) {
  const folder = path9.basename(path9.resolve(root)) || "artifact";
  const skipped = new Set(skip.map((file) => path9.resolve(file)));
  const entries = [];
  const walk2 = async (dir, rel) => {
    let names;
    try {
      names = await fsp2.readdir(dir, { withFileTypes: true });
    } catch (error) {
      if (rel && error.code === "ENOENT") return;
      throw error;
    }
    for (const dirent of names.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)) {
      const name = dirent.name;
      if (JUNK_FILES.has(name) || isTemp(name) || !rel && (LOCAL_FILES.has(name) || LOCAL_DIRS.has(name) && dirent.isDirectory())) continue;
      const file = path9.join(dir, name);
      if (skipped.has(file)) continue;
      const inner = rel ? `${rel}/${name}` : name;
      if (dirent.isDirectory()) await walk2(file, inner);
      else if (dirent.isFile()) {
        const stat = await fsp2.stat(file).catch(() => null);
        if (stat?.isFile()) entries.push({ name: `${folder}/${inner}`, file, size: stat.size, mode: stat.mode & 511, mtime: stat.mtime });
      }
    }
  };
  await walk2(path9.resolve(root), "");
  if (entries.length > MAX_ENTRIES) throw new ArchiveTooLarge(`The folder holds ${entries.length} files, more than a zip holds (${MAX_ENTRIES}); copy the folder itself instead.`);
  const bound = entries.reduce((sum, entry) => sum + entry.size + 2 * (46 + Buffer.byteLength(entry.name)), 22);
  if (bound > MAX_BYTES) throw new ArchiveTooLarge(`The folder holds ${(bound / 2 ** 30).toFixed(1)} GiB, more than a zip holds (4 GiB); copy the folder itself instead.`);
  return entries;
}
var CRC_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 3988292384 ^ c >>> 1 : c >>> 1;
    table[n] = c >>> 0;
  }
  return table;
})();
function crc32(bytes) {
  let crc = 4294967295;
  for (let i = 0; i < bytes.length; i++) crc = CRC_TABLE[(crc ^ bytes[i]) & 255] ^ crc >>> 8;
  return (crc ^ 4294967295) >>> 0;
}
function dosTime(date) {
  const year = date.getFullYear();
  if (year < 1980) return { time: 0, date: 1 << 5 | 1 };
  if (year > 2107) return { time: 23 << 11 | 59 << 5 | 29, date: 127 << 9 | 12 << 5 | 31 };
  return {
    time: date.getHours() << 11 | date.getMinutes() << 5 | date.getSeconds() >> 1,
    date: year - 1980 << 9 | date.getMonth() + 1 << 5 | date.getDate()
  };
}
var deflateRaw = promisify(zlib.deflateRaw);
var UTF8 = 1 << 11;
async function writeZip(entries, write) {
  const central = [];
  let offset = 0;
  const put = async (chunk) => {
    await write(chunk);
    offset += chunk.length;
  };
  for (const entry of entries) {
    let data;
    try {
      data = entry.data ?? await fsp2.readFile(entry.file);
    } catch (error) {
      if (error.code === "ENOENT") continue;
      throw error;
    }
    const crc = crc32(data);
    let method = 0;
    let body = data;
    if (data.length > 0 && !STORED.has(path9.extname(entry.name).toLowerCase())) {
      const deflated = await deflateRaw(data);
      if (deflated.length < data.length) {
        method = 8;
        body = deflated;
      }
    }
    if (offset + body.length > MAX_BYTES) throw new ArchiveTooLarge("The folder grew past what a zip holds (4 GiB) while it was being zipped.");
    const name = Buffer.from(entry.name, "utf8");
    const { time, date } = dosTime(entry.mtime);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(67324752, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(UTF8, 6);
    local.writeUInt16LE(method, 8);
    local.writeUInt16LE(time, 10);
    local.writeUInt16LE(date, 12);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(body.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(name.length, 26);
    local.writeUInt16LE(0, 28);
    const header = Buffer.alloc(46);
    header.writeUInt32LE(33639248, 0);
    header.writeUInt16LE(3 << 8 | 20, 4);
    header.writeUInt16LE(20, 6);
    header.writeUInt16LE(UTF8, 8);
    header.writeUInt16LE(method, 10);
    header.writeUInt16LE(time, 12);
    header.writeUInt16LE(date, 14);
    header.writeUInt32LE(crc, 16);
    header.writeUInt32LE(body.length, 20);
    header.writeUInt32LE(data.length, 24);
    header.writeUInt16LE(name.length, 28);
    header.writeUInt32LE((32768 | (entry.mode || 420) | 384) << 16 >>> 0, 38);
    header.writeUInt32LE(offset, 42);
    central.push(header, name);
    await put(Buffer.concat([local, name]));
    await put(body);
  }
  const start = offset;
  const directory = Buffer.concat(central);
  await put(directory);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(101010256, 0);
  end.writeUInt16LE(central.length / 2, 8);
  end.writeUInt16LE(central.length / 2, 10);
  end.writeUInt32LE(directory.length, 12);
  end.writeUInt32LE(start, 16);
  await put(end);
  return offset;
}
function archiveFileName(root, now = /* @__PURE__ */ new Date()) {
  const day = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
  return `${path9.basename(path9.resolve(root)) || "artifact"}-${day}.zip`;
}
async function writeArchive(root, out) {
  const file = path9.resolve(out);
  const entries = await archiveEntries(root, [file]);
  const tmp = path9.join(path9.dirname(file), `.${path9.basename(file)}.${process.pid}.tmp`);
  const handle = await fsp2.open(tmp, "w");
  let bytes;
  try {
    bytes = await writeZip(entries, async (chunk) => {
      await handle.write(chunk);
    });
    await handle.close();
  } catch (error) {
    await handle.close().catch(() => {
    });
    await fsp2.rm(tmp, { force: true });
    throw error;
  }
  await fsp2.rename(tmp, file);
  return { file, files: entries.length, bytes };
}
async function archiveCommand(root, out) {
  const where = out ? path9.resolve(out) : path9.dirname(path9.resolve(root));
  const target = fs9.statSync(where, { throwIfNoEntry: false })?.isDirectory() ? path9.join(where, archiveFileName(root)) : where;
  const { file, files, bytes } = await writeArchive(root, target);
  return { file, folder: path9.basename(path9.resolve(root)), files, bytes, leftOut: [...LOCAL_FILES] };
}
function attachment(name) {
  const ascii = name.replace(/[^\x20-\x7e]|["\\]/g, "_");
  return `attachment; filename="${ascii}"; filename*=UTF-8''${encodeURIComponent(name)}`;
}
async function sendArchive(request, response, root) {
  request.resume();
  let entries;
  try {
    entries = await archiveEntries(root);
  } catch (error) {
    const text = JSON.stringify({ code: error instanceof ArchiveTooLarge ? "too_large" : "upstream_error", message: error instanceof Error ? error.message : String(error) });
    response.writeHead(error instanceof ArchiveTooLarge ? 413 : 500, { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store" }).end(text);
    return;
  }
  response.writeHead(200, {
    "Content-Type": "application/zip",
    "Content-Disposition": attachment(archiveFileName(root)),
    "Cache-Control": "no-store",
    "X-Content-Type-Options": "nosniff"
  });
  if (request.method === "HEAD") {
    response.end();
    return;
  }
  const write = (chunk) => new Promise((resolve, reject) => {
    if (response.destroyed) return reject(new Error("The download was cancelled."));
    if (response.write(chunk)) return resolve();
    const drained = () => {
      response.off("close", closed);
      resolve();
    };
    const closed = () => {
      response.off("drain", drained);
      reject(new Error("The download was cancelled."));
    };
    response.once("drain", drained);
    response.once("close", closed);
  });
  try {
    await writeZip(entries, write);
    response.end();
  } catch {
    response.destroy();
  }
}

import fs10 from "node:fs";
import fsp3 from "node:fs/promises";
import os4 from "node:os";
import path10 from "node:path";
var MAX_BYTES2 = 4294967295;
function shown(file) {
  const home = os4.homedir();
  return home && (file === home || file.startsWith(home + path10.sep)) ? `~${file.slice(home.length)}` : file;
}
function saved(file, bytes) {
  return { path: file, name: path10.basename(file), folder: path10.dirname(file), shown: shown(file), bytes };
}
function safeFileName(raw) {
  let name = (raw.replace(/\\/g, "/").split("/").pop() ?? "").replace(/[\u0000-\u001f\u007f<>:"|?*]/g, "_").replace(/^[.\s]+|[.\s]+$/g, "");
  if (name.length > 200) {
    const ext = path10.extname(name).slice(0, 16);
    name = name.slice(0, 200 - ext.length) + ext;
  }
  return name || "download";
}
async function openFree(dir, name) {
  const ext = path10.extname(name);
  const stem = name.slice(0, name.length - ext.length);
  for (let n = 0; ; n++) {
    const file = path10.join(dir, n === 0 ? name : `${stem} (${n})${ext}`);
    try {
      return { file, handle: await fsp3.open(file, "wx") };
    } catch (error) {
      if (error.code !== "EEXIST" || n >= 999) throw error;
    }
  }
}
async function saveDownload(request, response, root) {
  const declared = Number(request.headers["content-length"]);
  if (Number.isFinite(declared) && declared > MAX_BYTES2) {
    request.resume();
    return sendJson2(response, 413, { code: "too_large", message: "The file is over 4 GiB." });
  }
  const name = safeFileName(new URL(request.url ?? "/", "http://local").searchParams.get("name") ?? "");
  let file = "";
  let handle = null;
  let bytes = 0;
  try {
    const dir = path10.join(root, EXPORTS_DIR);
    await fsp3.mkdir(dir, { recursive: true });
    ({ file, handle } = await openFree(dir, name));
    for await (const chunk of request) {
      bytes += chunk.length;
      if (bytes > MAX_BYTES2) throw Object.assign(new Error("The file is over 4 GiB."), { status: 413 });
      await handle.write(chunk);
    }
    await handle.close();
    handle = null;
  } catch (error) {
    await handle?.close().catch(() => {
    });
    if (file) await fsp3.rm(file, { force: true });
    request.resume();
    const status = error.status ?? 500;
    if (!response.headersSent) sendJson2(response, status, { code: status === 413 ? "too_large" : "upstream_error", message: error instanceof Error ? error.message : String(error) });
    return;
  }
  sendJson2(response, 200, saved(file, bytes));
}
async function saveArchive(request, response, root) {
  request.resume();
  try {
    const { file } = await archiveCommand(root, void 0);
    sendJson2(response, 200, saved(file, fs10.statSync(file).size));
  } catch (error) {
    const tooLarge = error instanceof ArchiveTooLarge;
    sendJson2(response, tooLarge ? 413 : 500, { code: tooLarge ? "too_large" : "upstream_error", message: error instanceof Error ? error.message : String(error) });
  }
}

var NOUNS = {
  docs: { en: "doc", zh: "\u6587\u6863", zhThis: "\u8FD9\u7BC7\u6587\u6863" },
  slides: { en: "deck", zh: "\u6F14\u793A\u6587\u7A3F", zhThis: "\u8FD9\u4EFD\u6F14\u793A\u6587\u7A3F" },
  design: { en: "design", zh: "\u8BBE\u8BA1\u7A3F", zhThis: "\u8FD9\u4EFD\u8BBE\u8BA1\u7A3F" },
  "design-system": { en: "design system", zh: "\u8BBE\u8BA1\u7CFB\u7EDF", zhThis: "\u8FD9\u4E2A\u8BBE\u8BA1\u7CFB\u7EDF" },
  dashboard: { en: "dashboard", zh: "\u4EEA\u8868\u76D8", zhThis: "\u8FD9\u4E2A\u4EEA\u8868\u76D8" },
  motion: { en: "animation", zh: "\u52A8\u753B", zhThis: "\u8FD9\u4E2A\u52A8\u753B" }
};
var ShareError = class extends Error {
};
var canEditAs = (role) => role === "owner" || role === "editor";
function commentsViewer(viewer) {
  return {
    author: viewer.guest ? { kind: "user", name: viewer.user.name, principal: viewer.user.email } : { kind: "user", name: viewer.user.name },
    canComment: viewer.role !== "viewer",
    canSend: viewer.role === "owner",
    admin: viewer.role === "owner"
  };
}
var isRecord4 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
var esc2 = (text) => text.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
var script = (value) => JSON.stringify(value).replace(/</g, "\\u003c");
function localOwner() {
  let name = "You";
  try {
    name = os5.userInfo().username.trim() || name;
  } catch {
  }
  return { id: "local", name, email: `${name}@localhost` };
}
var guestCookieName = (kind, root) => `${kind.replace(/-/g, "_")}_guest_${createHash4("sha256").update(path11.resolve(root)).digest("hex").slice(0, 12)}`;
var ShareHost = class {
  sharing;
  options;
  network;
  page;
  /** Open to the local network now (the hub's listeners are up, for this artifact too). */
  on = false;
  live = /* @__PURE__ */ new Set();
  constructor(options) {
    this.options = options;
    this.network = options.network;
    this.page = options.page;
    this.sharing = new Sharing(options.root);
  }
  get port() {
    return this.network.port;
  }
  /** The hub took the artifact: open to the network now with `lan`, or if it was left shared. */
  async start(lan = false) {
    const { state } = this.sharing;
    if (!lan && !state.lan) return;
    if (state.port && state.port !== this.port) this.options.log(`port ${state.port} (the shared links') is not this server's; it is on ${this.port}: give the new links out again`);
    await this.setLan(true);
  }
  /** The artifact at each LAN address it is shared on ([] while it is not). */
  lanUrls() {
    return this.on ? this.network.addresses().map((a) => `http://${a}:${this.port}${this.page}`) : [];
  }
  /** Open to the local network now. */
  get shared() {
    return this.on;
  }
  /** The artifact's name, for the join page and the list of what is open at the server's root. */
  title() {
    return this.options.title();
  }
  /**
   * Opens the artifact to the local network or closes it, and returns the addresses it is on. Guests already in keep
   * their place for when it opens again.
   */
  async setLan(on) {
    if (!on) {
      this.on = false;
      this.network.release();
      for (const live of [...this.live]) if (live.viewer.guest) live.close("unshared");
      if (this.sharing.state.lan) this.sharing.setLan(false);
      return [];
    }
    const addresses = await this.network.open();
    this.on = addresses.length > 0;
    if (!this.sharing.state.lan || this.sharing.state.port !== this.port) this.sharing.setLan(true, this.port);
    this.options.log(`shared on the local network: ${addresses.map((a) => `http://${a}:${this.port}${this.page}`).join(", ") || "(no LAN address)"}`);
    return addresses;
  }
  /** The artifact left the hub: it stays shared in .sharing.json, for the next time it opens. */
  close() {
    this.on = false;
    this.network.release();
  }
  // ---- who is asking ---------------------------------------------------------------------------------------------
  cookie(request) {
    for (const part of (request.headers.cookie ?? "").split(";")) {
      const eq = part.indexOf("=");
      if (eq > 0 && part.slice(0, eq).trim() === this.options.cookieName) return part.slice(eq + 1).trim();
    }
    return void 0;
  }
  /**
   * Who a request is: the owner over loopback, a guest by their cookie (while their link is on and the artifact is
   * shared), else nobody. Loopback is always the owner, so trying a link on this computer can't lock the owner out of
   * their own artifact; a guest comes over a LAN address (this computer's own included: that is how the owner sees
   * what a role sees).
   */
  isLocal(request) {
    return this.options.local ? this.options.local(request) : isLoopback(request.socket.remoteAddress);
  }
  viewerOf(request) {
    if (this.isLocal(request)) return { role: "owner", user: this.options.owner };
    const guest = this.on ? this.sharing.guest(this.cookie(request)) : null;
    if (!guest) return null;
    this.sharing.seen(guest);
    return { role: guest.role, user: guestUser(guest), guest };
  }
  /** Keeps track of a stream or socket `viewer` holds open; call the returned function when it closes. */
  attach(viewer, close) {
    const live = { viewer, close };
    this.live.add(live);
    return () => this.live.delete(live);
  }
  /** `attach` for an HTTP stream (server-sent events): it ends when its person loses access. */
  hold(viewer, response) {
    const detach = this.attach(viewer, () => response.end());
    response.on("close", detach);
  }
  /** Still allowed in: the owner, or a guest whose link is on and who was not removed. */
  stillIn(viewer) {
    return !viewer.guest || this.on && this.sharing.valid(viewer.guest);
  }
  /** Ends the streams of guests who lost access (a link turned off or reset, a guest removed). */
  dropRevoked() {
    for (const live of [...this.live]) if (live.viewer.guest && !this.sharing.valid(live.viewer.guest)) live.close("revoked");
  }
  me(viewer) {
    return { role: viewer.role, id: viewer.user.id, name: viewer.user.name, email: viewer.user.email };
  }
  // ---- routes ----------------------------------------------------------------------------------------------------
  /** Answers the share link and /__share paths; false for any other path. */
  async route(request, response) {
    const p = new URL(request.url ?? "/", "http://local").pathname;
    const method = request.method ?? "GET";
    const link = /^\/s\/([A-Za-z0-9_-]{16,64})$/.exec(p);
    const ours = link || p === "/__share" || p.startsWith("/__share/");
    if (!ours) return false;
    if (method === "POST" && !sameOrigin(request)) {
      request.resume();
      sendJson2(response, 403, { code: "not_granted", message: "Cross-origin write refused." });
      return true;
    }
    if (link) {
      if (method === "GET" || method === "POST") await this.join(request, response, method, link[1]);
      else response.writeHead(405, { Allow: "GET, POST" }).end();
      return true;
    }
    const viewer = this.viewerOf(request);
    if (!viewer) {
      this.denied(request, response);
      return true;
    }
    if (method === "GET" && p === "/__share/me") return sendJson2(response, 200, this.me(viewer)), true;
    if (method === "GET" && p === "/__share/boot.js") {
      const body = this.bootScript(viewer, new URL(request.url ?? "/", "http://local").searchParams);
      response.writeHead(200, { "Content-Type": "text/javascript; charset=utf-8", "Content-Length": Buffer.byteLength(body), "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" }).end(body);
      return true;
    }
    if (method === "GET" && p === "/__share/invite") return sendJson2(response, 200, this.invites(viewer, request.headers.host ?? "")), true;
    if (method === "GET" && p === "/__share/ui.js") {
      fs11.stat(this.options.uiFile, (error, stat) => {
        if (error || !stat.isFile()) return sendJson2(response, 404, { code: "not_found", message: "The Share panel is missing." });
        sendFile(request, response, this.options.uiFile, { "Content-Type": "text/javascript; charset=utf-8", "Content-Length": stat.size, "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
      });
      return true;
    }
    if (viewer.role !== "owner") return sendJson2(response, 403, { code: "not_granted", message: "Only the owner can do this." }), true;
    if ((method === "GET" || method === "HEAD") && p === "/__share/archive") return await sendArchive(request, response, this.options.root), true;
    if (method === "POST" && p === "/__share/archive") return await saveArchive(request, response, this.options.root), true;
    if (method === "POST" && p === "/__share/save") return await saveDownload(request, response, this.options.root), true;
    if (p === "/__share") return await this.shareApi(request, response, method), true;
    sendJson2(response, 404, { code: "not_found", message: "Not found" });
    return true;
  }
  /**
   * The tags a page puts first in its <head>: the boot script and the Share panel (the owner's, or a guest's). `query`
   * ("?…") goes to the boot script, for `globals`.
   */
  headTags(_viewer, query = "") {
    let tags = `<script src="${this.page}__share/boot.js${query.replace(/[<>"']/g, "").replace(/&/g, "&amp;")}"></script>`;
    if (fs11.existsSync(this.options.uiFile)) tags += `<script type="module" src="${this.page}__share/ui.js"></script>`;
    return tags;
  }
  /** `html` with the head tags first in its <head>; null when it has none. */
  injectHead(html, viewer, query = "") {
    const at = html.search(/<head(\s[^>]*)?>/i);
    if (at < 0) return null;
    const end = html.indexOf(">", at) + 1;
    return html.slice(0, end) + this.headTags(viewer, query) + html.slice(end);
  }
  /** What the page runs before anything else (see ROOT_SHIM, SECURE_CONTEXT_SHIM and GUEST_WATCH). */
  bootScript(viewer, query = new URLSearchParams()) {
    const share = { ...this.me(viewer), noun: this.options.noun, title: this.options.title(), ...this.options.reactions ? { reactions: true } : {} };
    let js = `(function(BASE){${ROOT_SHIM}})(${script(this.page)});${SECURE_CONTEXT_SHIM}window.__baoyuShare=${script(share)};`;
    for (const [name, value] of Object.entries(this.options.globals?.(viewer, query) ?? {})) js += `window[${script(name)}]=${script(value)};`;
    if (this.options.agentName) js += `window.__baoyuAgentName=${script(this.options.agentName)};`;
    if (viewer.guest) js += GUEST_WATCH;
    return js;
  }
  /** Nobody's request: a page for a browser's navigation, JSON for anything else. */
  denied(request, response) {
    request.resume();
    const navigation = (request.method === "GET" || request.method === "HEAD") && /\btext\/html\b/.test(request.headers.accept ?? "");
    if (!navigation) return sendJson2(response, 403, { code: "not_granted", message: `This ${this.options.noun.en} is not shared with you.` });
    const t = this.words(request);
    this.sendPage(response, 403, t.noAccessTitle, `<h1>${esc2(t.noAccessTitle)}</h1><p>${esc2(t.noAccess)}</p>${this.ownerHint(request, t)}`);
  }
  /** `/` on the server for someone who may see more than one artifact: a page of links. */
  sendList(response, sites) {
    const noun = this.options.noun.en;
    this.sendPage(response, 200, `Open ${noun}s`, `<h1>Open ${esc2(noun)}s</h1><ul>${sites.map((site) => `<li><a href="${esc2(site.page)}">${esc2(site.title)}</a></li>`).join("")}</ul>`);
  }
  /** A small page of our own (the join form, the refusal), in the reader's language. */
  sendPage(response, status, title, body, headers = {}) {
    const html = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${esc2(title)}</title><style>${PAGE_CSS}</style></head><body><main>${body}</main></body></html>`;
    response.writeHead(status, {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-store",
      "X-Content-Type-Options": "nosniff",
      "Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'",
      ...headers
    }).end(html);
  }
  /** On the computer that shares the artifact, over its LAN address: the way back to the owner's page. */
  ownerHint(request, t) {
    const from = (request.socket.remoteAddress ?? "").replace(/^::ffff:/, "");
    if (!this.network.addresses().includes(from)) return "";
    const url = `http://127.0.0.1:${this.port}${this.page}`;
    return `<p class="note">${esc2(t.ownerHere)} <a href="${esc2(url)}">${esc2(url)}</a></p>`;
  }
  /** GET /s/<token>: the name form (or straight in, for a guest already in through this link); POST: join. */
  async join(request, response, method, token2) {
    const t = this.words(request);
    const role = this.sharing.linkRole(token2);
    const local = this.isLocal(request);
    const form = method === "POST" ? await readBody(request, 8 << 10).then((body) => body.toString("utf8"), () => "") : "";
    const title = this.options.title();
    if (!role || !local && !this.on) return this.sendPage(response, 404, t.linkGoneTitle, `<h1>${esc2(t.linkGoneTitle)}</h1><p>${esc2(t.linkGone)}</p>`);
    if (local) {
      const lan = this.on ? this.network.addresses()[0] : void 0;
      const there = lan ? `http://${lan}:${this.port}${this.page}s/${token2}` : "";
      const tryIt = there ? `<p>${esc2(t.tryRole(t.roles[role]))} <a href="${esc2(there)}">${esc2(there)}</a></p>` : "";
      return this.sendPage(response, 200, title, `<h1>${esc2(title)}</h1><p>${esc2(t.youOwn)} <a href="${esc2(this.page)}">${esc2(t.openAsOwner)}</a></p>${tryIt}`);
    }
    const current = this.sharing.guest(this.cookie(request));
    if (method === "GET" && current && current.link === this.sharing.state.links[role].token) {
      response.writeHead(303, { Location: this.page, "Cache-Control": "no-store" }).end();
      return;
    }
    if (method === "GET") {
      return this.sendPage(
        response,
        200,
        title,
        `<h1>${esc2(title)}</h1><p>${esc2(t.invited(t.roles[role]))}</p><form method="post"><label for="n">${esc2(t.yourName)}</label><input id="n" name="name" maxlength="${MAX_NAME}" value="${esc2(current?.name ?? "")}" required autofocus autocomplete="name"><button type="submit">${esc2(t.open)}</button></form><p class="note">${esc2(t.nameNote)}</p>`
      );
    }
    const name = new URLSearchParams(form).get("name") ?? "";
    const guest = this.sharing.join(role, name);
    this.options.log(`share: ${guest.name} joined as ${role} (${request.socket.remoteAddress})`);
    const cookie = `${this.options.cookieName}=${guest.secret}; Path=/; HttpOnly; SameSite=Lax; Max-Age=31536000`;
    response.writeHead(303, { Location: this.page, "Set-Cookie": cookie, "Cache-Control": "no-store" }).end();
  }
  /**
   * The links this person may pass on: their own role's and the ones below it that are on (the owner: every one on), at
   * the address they came by (`host`, a Host header the hub let in). A guest could forward the link they joined
   * with anyway; a lower one gives no more than that. Who joins through one shows in the owner's panel like anyone else.
   */
  invites(viewer, host) {
    const { links } = this.sharing.state;
    const from = viewer.role === "owner" ? 0 : GUEST_ROLES.indexOf(viewer.role);
    return { links: GUEST_ROLES.slice(from).filter((role) => links[role].on).map((role) => ({ role, url: `http://${host}${this.page}s/${links[role].token}` })) };
  }
  /** Links, addresses and people, for the share panel and `artifact.mjs share`. */
  shareView() {
    const addresses = (this.on ? this.network.addresses() : []).sort((a, b) => Number(isTailnet(a)) - Number(isTailnet(b)));
    const local = addresses.filter((a) => !isTailnet(a));
    const base = local.length ? `http://${local[0]}:${this.port}` : addresses.length ? `http://${addresses[0]}:${this.port}` : `http://127.0.0.1:${this.port}`;
    const tailnet = addresses.find(isTailnet);
    const online = /* @__PURE__ */ new Set();
    for (const live of this.live) if (live.viewer.guest) online.add(live.viewer.guest.id);
    const { links, guests } = this.sharing.state;
    const lan = addresses.length ? addresses : [...lanAddresses()].sort((a, b) => Number(isTailnet(a)) - Number(isTailnet(b)));
    const network = lan.length ? `http://${lan[0]}:${this.port}` : base;
    return {
      title: this.options.title(),
      lan: this.on,
      port: this.port,
      page: this.page,
      addresses,
      local: `http://127.0.0.1:${this.port}${this.page}`,
      links: Object.fromEntries(GUEST_ROLES.map((role) => [role, { on: links[role].on, url: `${base}${this.page}s/${links[role].token}`, networkUrl: `${network}${this.page}s/${links[role].token}` }])),
      ...tailnet ? { tailnet: { address: tailnet, links: Object.fromEntries(GUEST_ROLES.map((role) => [role, { on: links[role].on, url: `http://${tailnet}:${this.port}${this.page}s/${links[role].token}` }])) } } : {},
      guests: guests.filter((g) => this.sharing.valid(g) || links[g.role].token === g.link).map((g) => ({ id: g.id, name: g.name, role: g.role, joined: g.joined, ...g.seen ? { seen: g.seen } : {}, online: online.has(g.id), active: this.sharing.valid(g) }))
    };
  }
  /**
   * The owner's sharing calls: `lan` {on}, `link` {role, on}, `reset` {role} (a new link: the old one and the guests
   * who came through it stop working), `role` {guest, role} (a guest's role, while that role's link is on), `remove`
   * {guest}. Answers the view after.
   */
  async share(action) {
    const role = action.role;
    switch (action.action) {
      case "lan": {
        const addresses = await this.setLan(action.on === true);
        if (action.on === true && !addresses.length) throw new ShareError("This computer has no local network address to share on.");
        break;
      }
      case "link":
        if (!isGuestRole(role)) throw new ShareError("role: editor, commenter or viewer");
        this.sharing.setLink(role, action.on === true);
        break;
      case "reset":
        if (!isGuestRole(role)) throw new ShareError("role: editor, commenter or viewer");
        this.sharing.resetLink(role);
        break;
      case "role": {
        if (!isGuestRole(role)) throw new ShareError("role: editor, commenter or viewer");
        if (typeof action.guest !== "string" || !this.sharing.state.guests.some((g) => g.id === action.guest)) throw new ShareError("no such guest");
        if (!this.sharing.setRole(action.guest, role)) throw new ShareError(`The ${role} link is off: turn it on first.`);
        for (const live of [...this.live]) if (live.viewer.guest?.id === action.guest) live.close("revoked");
        break;
      }
      case "remove":
        if (typeof action.guest !== "string" || !this.sharing.removeGuest(action.guest)) throw new ShareError("no such guest");
        break;
      default:
        throw new ShareError("action: lan, link, reset, role or remove");
    }
    this.options.log(`share: ${JSON.stringify(action)}`);
    this.dropRevoked();
    return this.shareView();
  }
  async shareApi(request, response, method) {
    if (method === "GET") return sendJson2(response, 200, this.shareView());
    if (method !== "POST") return sendJson2(response, 405, { code: "method_not_allowed", message: `${method} is not allowed here.` });
    let action;
    try {
      action = JSON.parse((await readBody(request, 64 << 10)).toString("utf8"));
    } catch {
      return sendJson2(response, 400, { code: "invalid_argument", message: "The call is not JSON." });
    }
    if (!isRecord4(action)) return sendJson2(response, 400, { code: "invalid_argument", message: "Expected {action, ...}." });
    try {
      return sendJson2(response, 200, await this.share(action));
    } catch (error) {
      if (error instanceof ShareError) return sendJson2(response, 400, { code: "invalid_argument", message: error.message });
      throw error;
    }
  }
  words(request) {
    const zh = /^zh\b/i.test(request.headers["accept-language"] ?? "");
    return (zh ? ZH : EN)(this.options.noun);
  }
};
var ROOT_SHIM = `
var P=location.protocol==='https:'?'wss:':'ws:';
function under(u){try{var a=new URL(String(u),location.href);if(a.host===location.host&&/^(https?|wss?):$/.test(a.protocol)&&a.pathname.indexOf(BASE)!==0)a.pathname=BASE+a.pathname.slice(1);return a.href}catch(e){return u}}
var F=window.fetch;
window.fetch=function(i,o){if(typeof Request==='function'&&i instanceof Request){var r=under(i.url);return F.call(this,r===i.url?i:new Request(r,i),o)}return F.call(this,under(i&&i.href||i),o)};
var X=XMLHttpRequest.prototype.open;
XMLHttpRequest.prototype.open=function(m,u){var a=[].slice.call(arguments);a[1]=under(u);return X.apply(this,a)};
var W=window.WebSocket;
function WS(u,p){return p===undefined?new W(under(u)):new W(under(u),p)}
WS.prototype=W.prototype;WS.CONNECTING=0;WS.OPEN=1;WS.CLOSING=2;WS.CLOSED=3;
window.WebSocket=WS;
if(navigator.sendBeacon){var B=navigator.sendBeacon;navigator.sendBeacon=function(u,d){return B.call(navigator,under(u),d)}}
class S extends EventTarget{
constructor(u){super();this.url=under(u);this.withCredentials=false;this.readyState=0;this.onopen=this.onmessage=this.onerror=null;this.l='';this.c()}
c(){var t=this,a=new URL(t.url);a.protocol=P;if(t.l)a.searchParams.set('__lastEventId',t.l);var w=t.w=new W(a.href,'baoyu-sse'),b='';
w.onopen=function(){t.readyState=1;t.f(new Event('open'))};
w.onmessage=function(e){b+=e.data;var i;while((i=b.indexOf('\\n\\n'))>=0){t.b(b.slice(0,i));b=b.slice(i+2)}};
w.onclose=function(e){if(t.readyState===2)return;if(e.code>=4000){t.readyState=2;t.f(new Event('error'));return}t.readyState=0;t.f(new Event('error'));t.r=setTimeout(function(){t.c()},1000)}}
b(s){var n='message',d=[],h=false;s.split('\\n').forEach(function(x){if(!x||x[0]===':')return;var i=x.indexOf(':'),k=i<0?x:x.slice(0,i),v=i<0?'':x.slice(i+1).replace(/^ /,'');if(k==='event')n=v;else if(k==='data'){d.push(v);h=true}else if(k==='id')this.l=v}.bind(this));if(h)this.f(new MessageEvent(n,{data:d.join('\\n'),origin:location.origin,lastEventId:this.l}))}
f(e){this.dispatchEvent(e);var h=this['on'+e.type];if(typeof h==='function')h.call(this,e)}
close(){this.readyState=2;clearTimeout(this.r);if(this.w)this.w.close()}
}
S.CONNECTING=0;S.OPEN=1;S.CLOSED=2;
window.EventSource=S;
`;
var SECURE_CONTEXT_SHIM = "if(window.crypto&&!crypto.randomUUID)crypto.randomUUID=function(){var b=crypto.getRandomValues(new Uint8Array(16)),h='',i;b[6]=b[6]&15|64;b[8]=b[8]&63|128;for(i=0;i<16;i++){h+=(b[i]+256).toString(16).slice(1);if(i==3||i==5||i==7||i==9)h+='-'}return h};";
var GUEST_WATCH = "setInterval(function(){fetch('/__share/me',{cache:'no-store'}).then(function(r){if(r.status===403)return location.reload();return r.json().then(function(m){if(m&&m.role&&m.role!==window.__baoyuShare.role)location.reload()})},function(){})},5000);";
var PAGE_CSS = `
:root { color-scheme: light dark; --bg: #f7f6f3; --card: #fff; --fg: #1f1e1d; --muted: #6b6a66; --line: #e3e1dc; --accent: #c96442; }
@media (prefers-color-scheme: dark) { :root { --bg: #1f1e1d; --card: #2a2927; --fg: #f2f1ee; --muted: #a8a6a0; --line: #3a3936; } }
* { box-sizing: border-box; }
body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: var(--bg); color: var(--fg); font: 15px/1.5 system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif; padding: 16px; }
main { width: min(420px, 100%); background: var(--card); border: 1px solid var(--line); border-radius: 14px; padding: 28px; }
h1 { font-size: 20px; margin: 0 0 8px; word-break: break-word; }
p { margin: 0 0 16px; color: var(--muted); }
label { display: block; font-size: 13px; margin: 0 0 6px; color: var(--muted); }
input { width: 100%; font: inherit; padding: 9px 11px; border: 1px solid var(--line); border-radius: 8px; background: transparent; color: inherit; margin: 0 0 14px; }
button { width: 100%; font: inherit; font-weight: 600; padding: 10px; border: 0; border-radius: 8px; background: var(--accent); color: #fff; cursor: pointer; }
.note { font-size: 12px; margin: 14px 0 0; }
a { color: var(--accent); overflow-wrap: anywhere; }
`;
var EN = (noun) => ({
  noAccessTitle: "No access",
  noAccess: `This ${noun.en} is not shared with you. Ask its owner for a link.`,
  linkGoneTitle: "This link does not work",
  linkGone: `The link was turned off or replaced, or the ${noun.en} is no longer shared. Ask its owner for a new one.`,
  invited: (what) => `You were invited to ${what} this ${noun.en}.`,
  yourName: "Your name (others see it on your comments and edits)",
  open: `Open the ${noun.en}`,
  nameNote: "This browser keeps your place; the owner can remove it at any time.",
  roles: { editor: "edit", commenter: "comment on", viewer: "view" },
  youOwn: `This is your ${noun.en}: on this computer you are its owner.`,
  openAsOwner: "Open it",
  tryRole: (what) => `To see it as someone invited to ${what} it does, open the link at the network address, here or on another device:`,
  ownerHere: `You are on the computer that shares this ${noun.en}. Open it as the owner at`
});
var ZH = (noun) => ({
  noAccessTitle: "\u65E0\u6743\u8BBF\u95EE",
  noAccess: `${noun.zhThis}\u6CA1\u6709\u5206\u4EAB\u7ED9\u4F60\u3002\u8BF7\u5411\u6240\u6709\u8005\u8981\u4E00\u4E2A\u94FE\u63A5\u3002`,
  linkGoneTitle: "\u94FE\u63A5\u5DF2\u5931\u6548",
  linkGone: `\u8FD9\u4E2A\u94FE\u63A5\u5DF2\u88AB\u5173\u95ED\u6216\u91CD\u7F6E\uFF0C\u6216\u8005${noun.zhThis}\u5DF2\u505C\u6B62\u5206\u4EAB\u3002\u8BF7\u5411\u6240\u6709\u8005\u8981\u4E00\u4E2A\u65B0\u94FE\u63A5\u3002`,
  invited: (what) => `\u4F60\u88AB\u9080\u8BF7${what}${noun.zhThis}\u3002`,
  yourName: "\u4F60\u7684\u540D\u5B57\uFF08\u522B\u4EBA\u4F1A\u5728\u4F60\u7684\u8BC4\u8BBA\u548C\u7F16\u8F91\u4E0A\u770B\u5230\u5B83\uFF09",
  open: `\u6253\u5F00${noun.zh}`,
  nameNote: "\u8FD9\u4E2A\u6D4F\u89C8\u5668\u4F1A\u8BB0\u4F4F\u4F60\u7684\u8EAB\u4EFD\uFF1B\u6240\u6709\u8005\u53EF\u4EE5\u968F\u65F6\u79FB\u9664\u3002",
  roles: { editor: "\u7F16\u8F91", commenter: "\u8BC4\u8BBA", viewer: "\u67E5\u770B" },
  youOwn: `\u8FD9\u662F\u4F60\u7684${noun.zh}\uFF1A\u5728\u8FD9\u53F0\u7535\u8111\u4E0A\u4F60\u662F\u6240\u6709\u8005\u3002`,
  openAsOwner: `\u6253\u5F00${noun.zh}`,
  tryRole: (what) => `\u60F3\u770B\u770B\u53D7\u9080${what}\u7684\u4EBA\u770B\u5230\u4EC0\u4E48\uFF0C\u7528\u5C40\u57DF\u7F51\u5730\u5740\u6253\u5F00\u8FD9\u4E2A\u94FE\u63A5\uFF08\u5728\u8FD9\u53F0\u7535\u8111\u6216\u522B\u7684\u8BBE\u5907\u4E0A\u90FD\u884C\uFF09\uFF1A`,
  ownerHere: `\u4F60\u5728\u5206\u4EAB${noun.zhThis}\u7684\u7535\u8111\u4E0A\u3002\u4EE5\u6240\u6709\u8005\u8EAB\u4EFD\u6253\u5F00\uFF1A`
});

var whoOf = (viewer) => ({ by: "human", user: { id: viewer.user.id, name: viewer.user.name }, role: viewer.role });
var MAX_BODY = 64 << 10;
var HistoryHost = class {
  history;
  options;
  constructor(options) {
    this.options = options;
    this.history = options.history;
  }
  /** What the page's boot script tells the panel (window.__baoyuHistory); `preview` on a version's preview page. */
  bootGlobal(viewer, preview) {
    const global = { base: this.options.base, canRestore: canEditAs(viewer.role), me: viewer.user.id };
    const version = preview === void 0 ? void 0 : this.history.version(preview);
    if (version) {
      const newest = this.history.versions().at(-1);
      global.preview = { n: version.n, at: version.at, by: version.by, name: version.user.name, current: version === newest, current_url: this.options.base };
    }
    return global;
  }
  notices(viewer, label) {
    const head = this.history.head();
    const gone = (n) => !head.has(n.path) && this.history.version(n.n)?.changes[n.path] === null;
    return this.history.noticesFor(viewer.user.id).map((n) => noticeView(n, label, this.history.version(n.lost.n)?.at, gone(n)));
  }
  list(viewer) {
    const label = this.options.labels();
    const versions = this.history.versions();
    const changes = this.history.fileChanges();
    const newest = versions.at(-1);
    const rows = [...versions].reverse().map((v) => ({
      n: v.n,
      first: v.first,
      at: v.at,
      by: v.by,
      name: v.user.name,
      userId: v.user.id,
      ...v.role ? { role: v.role } : {},
      ...v.restoredFrom !== void 0 ? { restoredFrom: v.restoredFrom } : {},
      ...v.restoredPart ? { restoredPart: true } : {},
      ...v === newest ? { current: true } : {},
      preview: this.options.previewUrl(v.n),
      files: (changes.get(v.n) ?? []).map((c) => ({ path: c.path, op: c.op, label: label(c.path) })),
      ...v.overwrote?.length ? { overwrote: v.overwrote.map((o) => ({ path: o.path, label: label(o.path), name: o.user.name, by: o.by, n: o.n, ...o.soft ? { soft: true } : {} })) } : {}
    }));
    return { me: viewer.user.id, canRestore: canEditAs(viewer.role), rows, notices: this.notices(viewer, label) };
  }
  /** Answers `__history…` (`sub` is what follows it); false for a path it doesn't know. */
  async handle(request, response, sub, viewer) {
    const method = request.method ?? "GET";
    const url = new URL(request.url ?? "/", "http://local");
    if (method === "POST" && !sameOrigin(request)) {
      request.resume();
      sendJson2(response, 403, { code: "not_granted", message: "Cross-origin write refused." });
      return true;
    }
    if (method === "GET" && (sub === "" || sub === "/")) return sendJson2(response, 200, this.list(viewer)), true;
    if (sub === "/notices") {
      if (method === "GET") return sendJson2(response, 200, { notices: this.notices(viewer, this.options.labels()) }), true;
      if (method !== "POST") return sendJson2(response, 405, { code: "method_not_allowed", message: "GET or POST" }), true;
      const body = await this.body(request, response);
      if (!body) return true;
      if (typeof body.dismiss !== "string") return sendJson2(response, 400, { code: "invalid_argument", message: "Expected {dismiss: id}." }), true;
      this.history.dismiss(viewer.user.id, body.dismiss);
      return sendJson2(response, 200, { notices: this.notices(viewer, this.options.labels()) }), true;
    }
    const versionRoute = /^\/(\d+)\/(diff|restore)$/.exec(sub);
    if (versionRoute) {
      const n = Number(versionRoute[1]);
      const version = this.history.version(n);
      if (!version) return sendJson2(response, 404, { code: "not_found", message: `There is no version ${n} (it may be too old to keep).` }), true;
      if (versionRoute[2] === "diff") {
        if (method !== "GET") return sendJson2(response, 405, { code: "method_not_allowed", message: "GET" }), true;
        const p = url.searchParams.get("path") ?? "";
        if (!(p in version.changes)) return sendJson2(response, 404, { code: "not_found", message: `Version ${n} did not change ${p}.` }), true;
        const before = this.history.stateBefore(n).get(p) ?? null;
        const after = version.changes[p] ?? null;
        return sendJson2(response, 200, this.diff(p, before, after)), true;
      }
      if (method !== "POST") return sendJson2(response, 405, { code: "method_not_allowed", message: "POST" }), true;
      if (!canEditAs(viewer.role)) return sendJson2(response, 403, { code: "not_writer", message: "Only the owner and editors can bring a version back." }), true;
      const body = await this.body(request, response);
      if (!body) return true;
      const paths = Array.isArray(body.paths) ? body.paths.filter((p) => typeof p === "string") : void 0;
      const changes = this.history.restoreTo(n, paths, this.options.tracked);
      if (!changes) return sendJson2(response, 409, { code: "unavailable", message: `Version ${n} can't be brought back: a file it holds is missing from the history.` }), true;
      const made = await this.options.apply(changes, whoOf(viewer), { restoredFrom: n });
      this.options.log(`history: ${viewer.user.name} brought back version ${n}${paths ? ` (${paths.join(", ")})` : ""}${made ? ` as version ${made.n}` : " (nothing to change)"}`);
      return sendJson2(response, 200, { version: made?.n ?? null, changed: [...changes.keys()] }), true;
    }
    const noticeRoute = /^\/notice\/([0-9a-f]{6,32})\/(diff|recover)$/.exec(sub);
    if (noticeRoute) {
      const notice = this.history.notice(noticeRoute[1]);
      if (!notice || notice.to !== viewer.user.id) return sendJson2(response, 404, { code: "not_found", message: "No such notice." }), true;
      if (noticeRoute[2] === "diff") {
        if (method !== "GET") return sendJson2(response, 405, { code: "method_not_allowed", message: "GET" }), true;
        const now2 = this.history.head().get(notice.path) ?? null;
        const nowBytes = now2 === null ? null : this.history.read(now2);
        const back2 = this.recovered(notice, nowBytes);
        if (now2 && !nowBytes || !back2) return sendJson2(response, 200, { path: notice.path, label: this.options.labels()(notice.path), missing: true, added: 0, removed: 0, lines: [], now: now2 }), true;
        return sendJson2(response, 200, { ...this.diffBytes(notice.path, nowBytes, back2.bytes), now: now2 }), true;
      }
      if (method !== "POST") return sendJson2(response, 405, { code: "method_not_allowed", message: "POST" }), true;
      if (!canEditAs(viewer.role)) return request.resume(), sendJson2(response, 403, { code: "not_writer", message: "Only the owner and editors can change the files." }), true;
      const body = await this.body(request, response);
      if (!body) return true;
      const now = this.history.head().get(notice.path) ?? null;
      if ("now" in body && body.now !== now) {
        return sendJson2(response, 409, { code: "changed", message: "This changed while you were looking. Compare again." }), true;
      }
      const back = this.recovered(notice, now === null ? null : this.history.read(now));
      if (!back) return sendJson2(response, 409, { code: "unavailable", message: "That content is no longer in the history." }), true;
      const made = await this.options.apply(/* @__PURE__ */ new Map([[notice.path, back.bytes]]), whoOf(viewer), { restoredFrom: notice.lost.n, ...back.part ? { restoredPart: true } : {} });
      this.history.dismiss(viewer.user.id, notice.id);
      this.options.log(`history: ${viewer.user.name} brought back ${notice.lost.user.name}'s ${back.bytes === null ? "deletion of " : ""}${notice.path} (version ${notice.lost.n}${back.part ? ", what was replaced" : ""})`);
      return sendJson2(response, 200, { version: made?.n ?? null }), true;
    }
    return false;
  }
  /** Before → after of one file, with the versions' names for the panel's header. */
  diff(p, before, after) {
    const read = (sha) => sha === null ? null : this.history.read(sha);
    const a = read(before);
    const b = read(after);
    if (before && !a || after && !b) return { path: p, label: this.options.labels()(p), missing: true, added: 0, removed: 0, lines: [] };
    return this.diffBytes(p, a, b);
  }
  diffBytes(p, a, b) {
    const [x, y] = this.options.diffSides ? this.options.diffSides(p, a, b) : [a, b];
    return { path: p, label: this.options.labels()(p), ...diffContents(x, y) };
  }
  /**
   * What bringing a notice's lost content back writes to its file (`now`: what the file holds): the lost content merged
   * into it when the notice has a base and the server can merge (`part` when that is not the whole lost content), else
   * the whole lost content; a deletion deletes it again (bytes null). null: not in the history.
   */
  recovered(notice, now) {
    if (notice.lost.sha === null) return { bytes: null, part: false };
    const lost = this.history.read(notice.lost.sha);
    if (!lost) return null;
    const base = notice.base ? this.history.read(notice.base) : null;
    const merged = base && this.options.recover ? this.options.recover(notice.path, base, lost, now) : null;
    return merged ? { bytes: merged, part: !merged.equals(lost) } : { bytes: lost, part: false };
  }
  async body(request, response) {
    try {
      const raw = (await readBody(request, MAX_BODY)).toString("utf8");
      const body = raw.trim() ? JSON.parse(raw) : {};
      if (typeof body === "object" && body !== null && !Array.isArray(body)) return body;
    } catch {
    }
    sendJson2(response, 400, { code: "invalid_argument", message: "Expected a JSON object." });
    return null;
  }
};
function noticeView(n, label, lostAt, gone) {
  return {
    ...lostAt ? { lostAt } : {},
    id: n.id,
    kind: n.kind,
    at: n.at,
    path: n.path,
    label: label(n.path),
    n: n.n,
    name: n.by.name,
    by: n.byAuthor,
    lostName: n.lost.user.name,
    lostBy: n.lost.by,
    lostN: n.lost.n,
    ...n.soft ? { soft: true } : {},
    // Bringing it back puts back only what was replaced (a write with a base); into a deleted file, all of it.
    ...n.base && !gone ? { part: true } : {},
    ...gone ? { removed: true } : {},
    // What was replaced is a deletion: bringing it back deletes the file again.
    ...n.lost.sha === null ? { deleted: true } : {}
  };
}
var plain2 = (text) => ({ en: text, zh: text });

var PEER_ID = /^[A-Za-z0-9_-]{8,64}$/;
function agentWhere(main2, count, label) {
  const where = main2 ? label(main2) : null;
  if (!where || count <= 1) return where;
  return { en: `${where.en} and ${count - 1} more`, zh: `${where.zh} \u7B49 ${count} \u5904` };
}
var AGENT_COLOR = "#d25f25";
var MAX_PEERS = 64;
var MAX_PATCH_BYTES = 16 << 10;
var MAX_FIELDS = 12;
var MAX_PRESENCE_BYTES = 6 << 10;
var BROADCAST_MS = 50;
var LEAVE_GRACE_MS = 4e3;
var AGENT_LINGER_MS = 2e4;
var AGENT_PEER = "agent-claude";
var PresenceHub = class {
  peers = /* @__PURE__ */ new Map();
  listeners = /* @__PURE__ */ new Set();
  agent = null;
  timer = null;
  /** Adds a page's stream; `send` writes one SSE frame to it. Returns what to call when the stream closes. */
  join(peer, viewer, send) {
    let entry = this.peers.get(peer);
    if (entry && entry.viewer.user.id !== viewer.user.id) {
      entry = void 0;
      peer = "";
    }
    if (peer && !entry && this.peers.size < MAX_PEERS) {
      entry = { viewer, presence: {}, updatedAt: Date.now(), streams: 0 };
      this.peers.set(peer, entry);
    }
    if (entry) {
      entry.streams += 1;
      if (entry.leaving) clearTimeout(entry.leaving);
      entry.leaving = void 0;
    }
    this.listeners.add(send);
    send(this.frame());
    this.schedule();
    let open = true;
    return () => {
      if (!open) return;
      open = false;
      this.listeners.delete(send);
      if (!entry) return;
      entry.streams -= 1;
      if (entry.streams > 0) return;
      const leaving = entry;
      leaving.leaving = setTimeout(() => {
        if (this.peers.get(peer) === leaving && leaving.streams === 0) {
          this.peers.delete(peer);
          this.schedule();
        }
      }, LEAVE_GRACE_MS);
      leaving.leaving.unref?.();
    };
  }
  /** Merges a page's patch into its presence; false when the peer isn't this person's open page or the patch is bad. */
  update(peer, viewer, patch) {
    const entry = this.peers.get(peer);
    if (!entry || entry.viewer.user.id !== viewer.user.id) return false;
    if (typeof patch !== "object" || patch === null || Array.isArray(patch)) return false;
    const next = { ...entry.presence };
    for (const [key, value] of Object.entries(patch)) {
      if (value === null || value === void 0) delete next[key];
      else next[key] = value;
    }
    if (Object.keys(next).length > MAX_FIELDS || Buffer.byteLength(JSON.stringify(next)) > MAX_PRESENCE_BYTES) return false;
    entry.presence = next;
    entry.updatedAt = Date.now();
    this.schedule();
    return true;
  }
  /** The agent changed the artifact: it shows on the list (with `context`, where it worked) for a while. */
  agentActive(agent = {}) {
    if (this.agent) clearTimeout(this.agent.timer);
    const presence = { who: { n: agent.name ?? "Agent", c: AGENT_COLOR } };
    if (agent.context) presence.context = agent.context;
    if (agent.where) presence.where = agent.where;
    const timer = setTimeout(() => {
      this.agent = null;
      this.schedule();
    }, AGENT_LINGER_MS);
    timer.unref?.();
    this.agent = { presence, updatedAt: Date.now(), timer };
    this.schedule();
  }
  /** Everyone on the list now. */
  list() {
    const views = [];
    for (const [peer, entry] of this.peers) {
      const who = entry.presence.who;
      const presence = { ...entry.presence, who: { ...typeof who === "object" && who !== null && !Array.isArray(who) ? who : {}, n: entry.viewer.user.name } };
      views.push({ peer, by: entry.viewer.user.id, name: entry.viewer.user.name, role: entry.viewer.role, presence, updatedAt: entry.updatedAt });
    }
    if (this.agent) {
      const name = String(this.agent.presence.who.n ?? "Agent");
      views.push({ peer: AGENT_PEER, by: "agent", name, kind: "agent", presence: this.agent.presence, updatedAt: this.agent.updatedAt });
    }
    return views;
  }
  /** Answers POST `__presence` ({peer, patch}); GET gives the list. */
  async handle(request, response, viewer) {
    const method = request.method ?? "GET";
    if (method === "GET") return sendJson2(response, 200, { peers: this.list() });
    if (method !== "POST") {
      request.resume();
      return sendJson2(response, 405, { code: "method_not_allowed", message: "GET or POST" });
    }
    if (!sameOrigin(request)) {
      request.resume();
      return sendJson2(response, 403, { code: "not_granted", message: "Cross-origin write refused." });
    }
    let body;
    try {
      body = JSON.parse((await readBody(request, MAX_PATCH_BYTES)).toString("utf8"));
    } catch {
      return sendJson2(response, 400, { code: "invalid_argument", message: `Expected {peer, patch} under ${MAX_PATCH_BYTES} bytes.` });
    }
    const { peer, patch } = typeof body === "object" && body !== null ? body : {};
    if (typeof peer !== "string" || !PEER_ID.test(peer)) return sendJson2(response, 400, { code: "invalid_argument", message: "Expected {peer, patch}." });
    if (!this.update(peer, viewer, patch)) return sendJson2(response, 409, { code: "not_joined", message: "That page is not open here (its event stream is closed)." });
    response.writeHead(204, { "Cache-Control": "no-store" }).end();
  }
  /** Ends the timers (the server is closing). */
  close() {
    if (this.timer) clearTimeout(this.timer);
    if (this.agent) clearTimeout(this.agent.timer);
    for (const entry of this.peers.values()) if (entry.leaving) clearTimeout(entry.leaving);
    this.peers.clear();
    this.listeners.clear();
  }
  frame() {
    return `event: peers
data: ${JSON.stringify({ peers: this.list() })}

`;
  }
  schedule() {
    if (this.timer) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      const frame = this.frame();
      for (const send of this.listeners) send(frame);
    }, BROADCAST_MS);
    this.timer.unref?.();
  }
};

var KIND = "";
var NOUN = "";
var INDEX_FILE = "";
var DESIGN_SYSTEMS = false;
var LAYOUT;
var pageScriptSrc;
var sessionExtras;
var commentLocation;
var kindLabels;
var kindAgentContext;
var SCRIPT = "";
var EDITOR_DIR = "";
var MIME = {
  html: "text/html",
  htm: "text/html",
  json: "application/json",
  css: "text/css",
  js: "text/javascript",
  mjs: "text/javascript",
  md: "text/markdown",
  txt: "text/plain",
  xml: "application/xml",
  svg: "image/svg+xml",
  png: "image/png",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  gif: "image/gif",
  webp: "image/webp",
  avif: "image/avif",
  ico: "image/x-icon",
  woff2: "font/woff2",
  woff: "font/woff",
  ttf: "font/ttf",
  otf: "font/otf",
  mp3: "audio/mpeg",
  wav: "audio/wav",
  mp4: "video/mp4",
  webm: "video/webm",
  mov: "video/quicktime",
  pdf: "application/pdf",
  wasm: "application/wasm"
};
var EXT_BY_MIME = {
  "image/svg+xml": "svg",
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
  "image/avif": "avif",
  "font/woff2": "woff2",
  "font/woff": "woff",
  "font/ttf": "ttf",
  "font/otf": "otf",
  "video/mp4": "mp4",
  "video/webm": "webm",
  "video/quicktime": "mov",
  "image/x-icon": "ico",
  "audio/mpeg": "mp3",
  "audio/wav": "wav",
  "application/pdf": "pdf"
};
var STATIC_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".woff2": "font/woff2",
  ".woff": "font/woff",
  ".ttf": "font/ttf",
  ".wasm": "application/wasm",
  ".map": "application/json; charset=utf-8"
};
var ROUTES = {
  listing: "_files.json",
  publish: "__deck/publish",
  assets: "__deck/assets",
  events: "__deck/events",
  session: "__deck/session"
};
var COMMENTS_ROUTE = "__comments";
var HISTORY_ROUTE = "__history";
var PRESENCE_ROUTE = "__presence";
var PREVIEW_PATH = /^v\/(\d{1,9})(\/.*)?$/;
var ASSET_INDEX = "index.json";
var BLOB_ID = /^[0-9a-f]{32}$/;
var BLOB_PATH = /^_blob\/([0-9a-f]{32})$/;
var DATA_CSP = "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:";
var MAX_PUBLISH_BYTES = 64 * 1024 * 1024;
var MAX_ASSET_BYTES = 256 * 1024 * 1024;
var SSE_KEEPALIVE_MS2 = 25e3;
var WATCH_DEBOUNCE_MS2 = 300;
var POLL_MS2 = 2e3;
var CHANGE_LOG_SIZE = 64;
var OVERWRITE_WINDOW_MS = 6e4;
var filesOf = (listing, paths) => Object.fromEntries(paths.map((p) => [p, listing.files[p]?.sha256 ?? null]));
var authorOf = (who) => ({ id: who.user.id, name: who.user.name, ...who.by === "agent" ? { agent: true } : {} });
var CLIENT_ID = /^[A-Za-z0-9_-]{8,64}$/;
var sha2562 = (bytes) => createHash5("sha256").update(bytes).digest("hex");
function contentTypeOf(file) {
  const ext = file.slice(file.lastIndexOf(".") + 1).toLowerCase();
  return MIME[ext] ?? "application/octet-stream";
}
var assetExtensionOf = (contentType) => EXT_BY_MIME[(contentType.split(";")[0] ?? "").trim().toLowerCase()] ?? "bin";
function isProjectPath(p) {
  if (typeof p !== "string" || !p.startsWith("project/") || p.includes("\\") || p.includes("\0")) return false;
  return p.split("/").every((part) => part !== "" && part !== "." && part !== "..");
}
var SHA256 = /^[0-9a-f]{64}$/;
var isWriteEntry = (entry) => entry !== null && !("delete" in entry);
function isWireEntry(value) {
  if (value === null) return true;
  if (typeof value !== "object" || Array.isArray(value)) return false;
  const entry = value;
  if (entry.delete === true) return entry.ifMatch === void 0 || typeof entry.ifMatch === "string";
  if (entry.ifMatch !== void 0 && entry.ifMatch !== null && typeof entry.ifMatch !== "string") return false;
  return typeof entry.base64 === "string" && typeof entry.contentType === "string";
}
function pageCsp(scriptSrc) {
  const fonts = "https://fonts.googleapis.com https://fonts.gstatic.com";
  return [
    "default-src 'self'",
    scriptSrc,
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "font-src 'self' data: https://fonts.gstatic.com",
    "img-src 'self' data: blob:",
    "media-src 'self' blob:",
    "worker-src 'self' blob:",
    `connect-src 'self' data: blob: ${fonts}`,
    "frame-src 'self' blob: data:",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'none'"
  ].join("; ");
}
function versionOf(files) {
  const digest = sha2562(
    Object.entries(files).map(([p, f]) => `${p}:${f.sha256}`).join("\n")
  );
  return "local-" + digest.slice(0, 16);
}
var isIgnoredPath = (rel) => rel.split("/").some((part) => part.startsWith("."));
async function collectFiles(root, limit) {
  const rels = [];
  const seen = /* @__PURE__ */ new Set();
  let truncated = false;
  const visit = async (relDir, descend) => {
    const dir = relDir === "" ? root : path12.join(root, ...relDir.split("/"));
    let entries;
    try {
      entries = await fsp4.readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }
    entries.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
    for (const entry of entries) {
      if (truncated) return;
      if (entry.name.startsWith(".")) continue;
      const rel = relDir === "" ? entry.name : `${relDir}/${entry.name}`;
      if (entry.isDirectory()) {
        if (descend && LAYOUT.descendInto(rel)) await visit(rel, true);
      } else if (entry.isFile() && LAYOUT.isFile(rel) && !seen.has(rel)) {
        if (rels.length >= limit) {
          truncated = true;
          return;
        }
        seen.add(rel);
        rels.push(rel);
      }
    }
  };
  for (const start of LAYOUT.roots) {
    if (truncated) break;
    await visit(start.rel, start.descend);
  }
  return { rels, truncated };
}
async function writeFileAtomic(file, bytes) {
  await fsp4.mkdir(path12.dirname(file), { recursive: true });
  const tmp = path12.join(path12.dirname(file), `.${path12.basename(file)}.${randomUUID()}.tmp`);
  try {
    await fsp4.writeFile(tmp, bytes);
    await fsp4.rename(tmp, file);
  } catch (error) {
    await fsp4.rm(tmp, { force: true }).catch(() => {
    });
    throw error;
  }
}
var Store = class {
  root;
  history;
  log;
  /** project/<path> → sha256 of the page's last save not yet seen by the watcher ('' for a delete). */
  ownWrites;
  /** "<user id>\0project/<path>" → when the server last refused that person's save of the file as a conflict. */
  refused;
  /**
   * "<page>\0project/<path>" → the sha256 of that page's last save of the file ('' for a delete). A page sends no
   * ifMatch for a file whose version it believes is still its own last save; the server checks that belief for it.
   */
  lastSaved;
  queue;
  seq;
  truncationReported;
  /** The last listing left files out (more than the format's limit): its missing files are not deletes. */
  truncated;
  constructor(root, history, log) {
    this.root = root;
    this.history = history;
    this.log = log;
    this.ownWrites = /* @__PURE__ */ new Map();
    this.refused = /* @__PURE__ */ new Map();
    this.lastSaved = /* @__PURE__ */ new Map();
    this.queue = Promise.resolve();
    this.seq = 0;
    this.truncationReported = false;
    this.truncated = false;
  }
  /** Run `work` after every save, upload and change check before it, and before the next. */
  exclusive(work) {
    const run = this.queue.then(work);
    this.queue = run.catch(() => void 0);
    return run;
  }
  /** project/<path> → its path relative to project/, or null when the editor may not see or write it. */
  fileOf(runtimePath) {
    if (!isProjectPath(runtimePath)) return null;
    const rel = runtimePath.slice("project/".length);
    return LAYOUT.isFile(rel) ? rel : null;
  }
  /** The file on disk (it may not exist yet); null when a directory on the way is a symlink or the name is not a file. */
  async diskPath(rel) {
    const realRoot = await fsp4.realpath(this.root).catch(() => null);
    if (realRoot === null) return null;
    const file = path12.join(realRoot, ...rel.split("/"));
    for (let dir = path12.dirname(file); ; dir = path12.dirname(dir)) {
      const real = await fsp4.realpath(dir).catch(() => null);
      if (real === null) {
        if (path12.dirname(dir) === dir) return null;
        continue;
      }
      if (real !== dir) return null;
      break;
    }
    const info = await fsp4.lstat(file).catch(() => null);
    return info && !info.isFile() ? null : file;
  }
  async listing() {
    const limit = LAYOUT.maxFiles;
    const { rels, truncated } = await collectFiles(this.root, limit);
    this.truncated = truncated;
    if (truncated && !this.truncationReported) {
      this.truncationReported = true;
      this.log(`The ${NOUN} folder has more than ${limit} files; the editor lists only the first ${limit}: ${this.root}`);
    }
    const files = {};
    for (const rel of rels.sort()) {
      const file = await this.diskPath(rel);
      if (file === null) continue;
      const bytes = await fsp4.readFile(file).catch(() => null);
      if (bytes === null) continue;
      files[`project/${rel}`] = { sha256: sha2562(bytes), size: bytes.length, contentType: contentTypeOf(rel) };
    }
    return { ver: versionOf(files), files };
  }
  async readProjectFile(p) {
    const rel = this.fileOf(p);
    const file = rel === null ? null : await this.diskPath(rel);
    if (file === null) return null;
    return fsp4.readFile(file).catch(() => null);
  }
  /**
   * A save from the page by `who`: all entries or none (ifMatch is checked for all before anything is written). It is
   * a version in the history under their name; what changed on disk before it is one of its own, Claude's.
   */
  publish(batch, who, over = [], merged = [], page = "") {
    return this.exclusive(() => this.publishNow(batch, who, over, merged, page));
  }
  async publishNow(batch, who, over, merged, page) {
    const saver = page ? `page:${page}` : `user:${who.user.id}`;
    const named = new Set([...over, ...merged].map((o) => o.path));
    const files = /* @__PURE__ */ new Map();
    let total = 0;
    for (const [p, entry] of Object.entries(batch)) {
      const rel = this.fileOf(p);
      const file = rel === null ? null : await this.diskPath(rel);
      if (file === null) return { status: 400, body: { code: "read_only_path", message: LAYOUT.readOnlyMessage(p) } };
      files.set(p, file);
      if (!isWriteEntry(entry)) continue;
      const bytes = Buffer.byteLength(entry.base64, "base64");
      if (bytes > LAYOUT.maxFileBytes) {
        return { status: 413, body: { code: "too_large", message: `${p}: a file can be at most ${LAYOUT.maxFileBytes} bytes` } };
      }
      total += bytes;
    }
    if (total > LAYOUT.maxPublishBytes) {
      return { status: 413, body: { code: "too_large", message: `a publish can write at most ${LAYOUT.maxPublishBytes} bytes` } };
    }
    const before = await this.listing();
    await this.syncDisk(before);
    const conflicts = [];
    for (const [p, entry] of Object.entries(batch)) {
      const expected = entry?.ifMatch;
      const actual = before.files[p]?.sha256 ?? null;
      if (expected === void 0) {
        const own = this.lastSaved.get(`${saver}\0${p}`);
        if (own !== void 0 && !named.has(p) && (actual ?? "") !== own) conflicts.push({ path: p, actual, expected: own || null });
        continue;
      }
      if (actual !== expected) conflicts.push({ path: p, actual, expected });
    }
    if (conflicts.length > 0) {
      const now = Date.now();
      for (const c of conflicts) this.refused.set(`${who.user.id}\0${c.path}`, now);
      return { status: 409, body: { code: "conflict", live: before.ver, paths: conflicts } };
    }
    for (const m of merged) {
      if (batch[m.path] !== void 0 && before.files[m.path]?.sha256 === m.sha) this.refused.delete(`${who.user.id}\0${m.path}`);
    }
    const chosen = /* @__PURE__ */ new Map();
    for (const o of over) {
      if (batch[o.path] === void 0 || before.files[o.path]?.sha256 !== o.sha) continue;
      this.refused.set(`${who.user.id}\0${o.path}`, Date.now());
      chosen.set(o.path, o.base);
    }
    const overwrote = this.overwrites(batch, before, who, chosen);
    const shas = {};
    const written = /* @__PURE__ */ new Map();
    for (const [p, entry] of Object.entries(batch)) {
      const file = files.get(p);
      if (!isWriteEntry(entry)) {
        this.ownWrites.set(p, "");
        this.rememberSave(saver, p, "");
        await fsp4.rm(file, { force: true });
        written.set(p, null);
        continue;
      }
      const bytes = Buffer.from(entry.base64, "base64");
      const sha = sha2562(bytes);
      this.ownWrites.set(p, sha);
      this.rememberSave(saver, p, sha);
      await writeFileAtomic(file, bytes);
      shas[p] = sha;
      written.set(p, bytes);
    }
    const version = this.history.record(written, who, overwrote.length ? { overwrote } : {});
    const after = await this.listing();
    this.seq += 1;
    return { status: 200, body: { version: after.ver, seq: this.seq, shas }, version };
  }
  /** A page's last save of a file (lastSaved), the oldest forgotten past a few thousand. */
  rememberSave(saver, p, sha) {
    const key = `${saver}\0${p}`;
    this.lastSaved.delete(key);
    this.lastSaved.set(key, sha);
    if (this.lastSaved.size > 4e3) this.lastSaved.delete(this.lastSaved.keys().next().value);
  }
  /**
   * The files this save replaces without its page having seen them: the server refused this person's save of the file
   * as a conflict moments ago (someone else had saved it since their page loaded it), and now the page saves on top of
   * what that someone saved. (The editors take the newer file and save their own over it.) Their own other tab's save,
   * and the content the history began with, are no one else's. `chosen`: the files the page says its person saw and
   * chose to save over (marked as refused above), whose overwrite is `chosen`, with the version the edits started
   * from as its `base` when the history holds that content (and it is not what was lost, which would bring back nothing).
   * A page that names no base (Keep mine from a bar or a banner, a save on top of a refused one) gets the file as it was
   * right before the lost version: what both started from when one saved while the other was editing, so Bring back
   * returns what that version changed and leaves the saver's other changes.
   */
  overwrites(batch, before, who, chosen) {
    const now = Date.now();
    const out = [];
    for (const [p, entry] of Object.entries(batch)) {
      const key = `${who.user.id}\0${p}`;
      const at = this.refused.get(key);
      if (at === void 0) continue;
      this.refused.delete(key);
      const lost = before.files[p]?.sha256;
      if (now - at > OVERWRITE_WINDOW_MS || !lost) continue;
      if ((isWriteEntry(entry) ? sha2562(Buffer.from(entry.base64, "base64")) : null) === lost) continue;
      const last = this.history.lastChange(p);
      if (!last || last.sha !== lost || last.version.by === "system" || last.version.user.id === who.user.id) continue;
      const base = chosen.get(p) ?? this.history.stateBefore(last.version.n).get(p);
      out.push({
        path: p,
        lost,
        n: last.version.n,
        by: last.version.by,
        user: { ...last.version.user },
        ...chosen.has(p) ? { chosen: true } : {},
        ...base && base !== lost && this.history.size(base) !== null ? { base } : {}
      });
    }
    for (const [key, at] of this.refused) if (now - at > OVERWRITE_WINDOW_MS) this.refused.delete(key);
    return out;
  }
  /**
   * Records what changed on disk since the history last looked, as Claude's (the agent's, or anything else on this
   * computer; the files as they were when the history began are a version of no one's). Runs inside `exclusive`.
   */
  async syncDisk(listing) {
    const head = this.history.head();
    const changes = /* @__PURE__ */ new Map();
    for (const [p, f] of Object.entries(listing.files)) {
      if (head.get(p) === f.sha256) continue;
      const bytes = await this.readProjectFile(p);
      if (bytes !== null) changes.set(p, bytes);
    }
    if (!this.truncated) {
      for (const p of head.keys()) if (!(p in listing.files) && this.fileOf(p) !== null) changes.set(p, null);
    }
    if (changes.size === 0) return null;
    const version = this.history.record(changes, this.history.empty ? SYSTEM : AGENT);
    if (version) this.log(`history: version ${version.n}${version.by === "agent" ? " (changed on disk)" : " (the files as they were)"}: ${[...changes.keys()].map((p) => p.replace(/^project\//, "")).join(", ")}`);
    return version;
  }
  /** Writes contents from the history (a restore) and records them as `who`'s version. */
  applyFromHistory(changes, who, extra) {
    return this.exclusive(async () => {
      await this.syncDisk(await this.listing());
      const written = /* @__PURE__ */ new Map();
      for (const [p, bytes] of changes) {
        const rel = this.fileOf(p);
        const file = rel === null ? null : await this.diskPath(rel);
        if (file === null) continue;
        if (bytes === null) {
          this.ownWrites.set(p, "");
          await fsp4.rm(file, { force: true });
        } else {
          this.ownWrites.set(p, sha2562(bytes));
          await writeFileAtomic(file, bytes);
        }
        written.set(p, bytes);
      }
      return { version: this.history.record(written, who, extra), paths: [...written.keys()] };
    });
  }
  async assetIndex() {
    try {
      const file = await this.diskPath(`${LAYOUT.blobDir}/${ASSET_INDEX}`);
      if (file === null) return {};
      const parsed = JSON.parse(await fsp4.readFile(file, "utf8"));
      return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
    } catch {
      return {};
    }
  }
  /** An upload, addressed by content: id = the first 32 hex digits of its sha256, stored once. */
  putBlob(bytes, contentType) {
    return this.exclusive(() => this.putBlobNow(bytes, contentType));
  }
  async putBlobNow(bytes, contentType) {
    const id = sha2562(bytes).slice(0, 32);
    const type = (contentType.split(";")[0] ?? "").trim().toLowerCase() || "application/octet-stream";
    if (await this.findBlobFile(id) === null) {
      const file = await this.diskPath(`${LAYOUT.blobDir}/${id}.${assetExtensionOf(type)}`);
      if (file === null) throw new Error(`${LAYOUT.blobDir} directory is not writable`);
      await writeFileAtomic(file, bytes);
    }
    const index = await this.assetIndex();
    const indexFile = await this.diskPath(`${LAYOUT.blobDir}/${ASSET_INDEX}`);
    if (index[id] !== type && indexFile !== null) {
      index[id] = type;
      await writeFileAtomic(indexFile, Buffer.from(JSON.stringify(index, null, 2) + "\n"));
    }
    return { id, url: `/_blob/${id}`, sizeBytes: bytes.byteLength };
  }
  async findBlobFile(id) {
    try {
      const names = await fsp4.readdir(path12.join(this.root, LAYOUT.blobDir));
      const name = names.find((n) => n.startsWith(`${id}.`) && !n.endsWith(".tmp"));
      if (name === void 0) return null;
      const file = await this.diskPath(`${LAYOUT.blobDir}/${name}`);
      return file !== null && (await fsp4.lstat(file).catch(() => null))?.isFile() ? file : null;
    } catch {
      return null;
    }
  }
  async blobFile(id) {
    if (!BLOB_ID.test(id)) return null;
    const file = await this.findBlobFile(id);
    if (file === null) return null;
    return { file, contentType: (await this.assetIndex())[id] ?? contentTypeOf(file) };
  }
  /**
   * Of these paths, the ones whose content on disk is neither the page's own last save nor what was already seen.
   * Runs inside `exclusive`, so no save is half done; afterwards `seen` holds what is on disk and the saves are settled.
   */
  async externalChanges(paths, seen) {
    const external = [];
    for (const p of paths) {
      const rel = this.fileOf(p);
      if (rel === null) continue;
      let sha = "";
      try {
        const file = await this.diskPath(rel);
        if (file !== null && (await fsp4.lstat(file)).isFile()) sha = sha2562(await fsp4.readFile(file));
      } catch {
        sha = "";
      }
      if (this.ownWrites.get(p) !== sha && seen.get(p) !== sha) external.push(p);
      this.ownWrites.delete(p);
      seen.set(p, sha);
    }
    return external;
  }
};
function watchArtifact(store, onChange, onError) {
  let timer;
  let stopped = false;
  const watchers = /* @__PURE__ */ new Map();
  let seen = null;
  let pollSignature = null;
  let polling = false;
  const maxDirectories = LAYOUT.maxFiles;
  const schedule = () => {
    if (stopped) return;
    clearTimeout(timer);
    timer = setTimeout(() => void store.exclusive(flush), WATCH_DEBOUNCE_MS2);
  };
  const refresh = () => {
    const directories = /* @__PURE__ */ new Map();
    const visit = (rel, descend) => {
      if (directories.size >= maxDirectories) return;
      const dir = rel === "" ? store.root : path12.join(store.root, ...rel.split("/"));
      if (directories.has(dir) || !fs12.existsSync(dir)) return;
      const info = fs12.lstatSync(dir);
      if (!info.isDirectory()) return;
      directories.set(dir, `${info.dev}:${info.ino}`);
      if (!descend) return;
      for (const entry of fs12.readdirSync(dir, { withFileTypes: true })) {
        if (!entry.isDirectory() || entry.name.startsWith(".")) continue;
        const child = rel === "" ? entry.name : `${rel}/${entry.name}`;
        if (LAYOUT.descendInto(child)) visit(child, true);
      }
    };
    for (const start of LAYOUT.roots) visit(start.rel, start.descend);
    for (const [dir, item] of watchers) {
      if (directories.get(dir) !== item.identity) {
        item.watcher.close();
        watchers.delete(dir);
      }
    }
    for (const [dir, identity] of directories) {
      if (watchers.has(dir)) continue;
      let watcher;
      try {
        watcher = fs12.watch(dir, (event, name) => {
          if (stopped) return;
          if (name) {
            const changed = path12.join(dir, String(name));
            if (event === "rename" && watchers.has(changed)) {
              for (const [watchedDir, item] of watchers) {
                if (watchedDir === changed || watchedDir.startsWith(changed + path12.sep)) {
                  item.watcher.close();
                  watchers.delete(watchedDir);
                }
              }
            }
            const rel = path12.relative(store.root, changed).split(path12.sep).join("/");
            if (isIgnoredPath(rel) || !LAYOUT.watches(rel)) return;
          }
          schedule();
        });
      } catch (error) {
        onError(error);
        continue;
      }
      watcher.on("error", onError);
      watchers.set(dir, { watcher, identity });
    }
  };
  const flush = async () => {
    if (stopped) return;
    try {
      refresh();
      const current = await store.listing();
      await store.syncDisk(current);
      if (seen === null) {
        seen = new Map(Object.entries(current.files).map(([p, f]) => [p, f.sha256]));
        pollSignature = await signature();
        return;
      }
      const external = await store.externalChanges(/* @__PURE__ */ new Set([...seen.keys(), ...Object.keys(current.files)]), seen);
      if (stopped || external.length === 0) return;
      const after = await store.listing();
      onChange({ type: "external-change", paths: external, ver: after.ver, files: filesOf(after, external) });
    } catch (error) {
      onError(error);
    }
  };
  const signature = async () => {
    const { rels } = await collectFiles(store.root, LAYOUT.maxFiles);
    const parts = [];
    for (const rel of rels) {
      const info = await fsp4.lstat(path12.join(store.root, ...rel.split("/"))).catch(() => null);
      parts.push(info ? `${rel}\0${info.size}\0${info.mtimeMs}\0${info.ino}` : `${rel}\0-`);
    }
    return parts.join("\n");
  };
  const poll = setInterval(async () => {
    if (stopped || polling) return;
    polling = true;
    try {
      const next = await signature();
      if (pollSignature !== null && next !== pollSignature) schedule();
      pollSignature = next;
    } catch (error) {
      onError(error);
    } finally {
      polling = false;
    }
  }, POLL_MS2);
  poll.unref?.();
  void store.exclusive(flush);
  return () => {
    stopped = true;
    clearTimeout(timer);
    clearInterval(poll);
    for (const item of watchers.values()) item.watcher.close();
    watchers.clear();
  };
}
var publishError = (response, status, code, message) => sendJson2(response, status, { code, message });
async function mountSite(root, { log: stamp2, network, agentHook, local, agentName }) {
  const project = path12.join(root, "project");
  const editorPath = pagePathOf(KIND, root);
  const projectDir = await fsp4.realpath(project);
  const history = new History(root);
  const store = new Store(projectDir, history, stamp2);
  const owner = localOwner();
  const share = new ShareHost({
    root,
    page: editorPath,
    network,
    cookieName: guestCookieName(KIND, root),
    owner,
    noun: NOUNS[KIND] ?? { en: NOUN, zh: NOUN, zhThis: NOUN },
    title: () => artifactTitle(root, projectDir),
    log: stamp2,
    uiFile: path12.join(EDITOR_DIR, "share-ui.js"),
    ...local ? { local } : {},
    ...agentName ? { agentName } : {},
    // The History panel's address and what this person may do there; on a version's page (?at=n), which version.
    globals: (viewer, query) => {
      const at = Number(query.get("at") ?? "");
      return { __baoyuHistory: historyHost.bootGlobal(viewer, query.has("at") && Number.isInteger(at) ? at : void 0) };
    }
  });
  const comments = createCommentsService({
    root,
    log: stamp2,
    locate: commentLocation ? (input) => commentLocation(input, projectDir) : void 0,
    agentHook,
    userName: owner.name,
    allowWrite: sameOrigin,
    viewer: (request) => {
      const viewer = share.viewerOf(request);
      return viewer ? commentsViewer(viewer) : null;
    }
  });
  const boot = randomBytes2(6).toString("hex");
  const clients = /* @__PURE__ */ new Map();
  const presence = new PresenceHub();
  const changes = [];
  let changeSeq = 0;
  const pageCache = /* @__PURE__ */ new Map();
  const eventId = () => `${boot}.${changeSeq}`;
  const frame = (event) => `id: ${eventId()}
event: ${event.type}
data: ${JSON.stringify(event)}

`;
  const changed = (event, from) => {
    changeSeq += 1;
    changes.push({ seq: changeSeq, paths: event.paths, ...from ? { from } : {}, ...event.by ? { by: event.by } : {} });
    if (changes.length > CHANGE_LOG_SIZE) changes.shift();
    for (const [client, id] of clients) if (!from || id !== from) client.write(frame(event));
  };
  const labels2 = () => {
    let named;
    try {
      named = kindLabels?.(projectDir);
    } catch {
      named = void 0;
    }
    return (p) => {
      const rel = p.replace(/^project\//, "");
      return named?.(rel) ?? plain2(rel);
    };
  };
  const agentWorked = (paths) => {
    const rels = paths.map((p) => p.replace(/^project\//, ""));
    const main2 = [...paths].reverse().find((p) => !p.endsWith(`/${INDEX_FILE}`) && p !== INDEX_FILE) ?? paths[0];
    let context = null;
    try {
      context = kindAgentContext?.(projectDir, rels) ?? null;
    } catch {
      context = null;
    }
    presence.agentActive({ context, where: agentWhere(main2, paths.length, labels2()) });
  };
  const historyHost = new HistoryHost({
    history,
    labels: labels2,
    base: editorPath,
    previewUrl: (n) => `${editorPath}v/${n}/`,
    apply: async (files, who, extra) => {
      const { version, paths } = await store.applyFromHistory(files, who, extra);
      if (paths.length) {
        const listing = await store.listing();
        changed({ type: "external-change", paths, ver: listing.ver, files: filesOf(listing, paths), by: authorOf(who) });
      }
      return version;
    },
    tracked: (p) => store.fileOf(p) !== null,
    // A notice whose overwrite named the version its saver's edits started from: what was lost goes back by line into
    // the file as it is now, the lost lines winning where both changed them; a deleted file gets all of it back.
    recover: (_path, base, lost, now) => now === null ? lost : mergeTextBytes(base, lost, now),
    log: stamp2
  });
  const stopWatch = watchArtifact(
    store,
    (event) => {
      stamp2(`changed on disk: ${event.paths.join(", ")}${clients.size ? "" : " (no page open)"}`);
      changed({ ...event, by: authorOf(AGENT) });
      agentWorked(event.paths);
    },
    (error) => stamp2(`watching failed: ${errorText(error)}`)
  );
  const events = async (request, response, viewer) => {
    const given = new URL(request.url ?? "/", "http://local").searchParams.get("client") ?? "";
    const client = CLIENT_ID.test(given) ? given : "";
    response.writeHead(200, {
      "Content-Type": "text/event-stream; charset=utf-8",
      "Cache-Control": "no-store",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no"
    });
    response.write(": connected\n\n");
    const last = /^([0-9a-f]+)\.(\d+)$/.exec(String(request.headers["last-event-id"] ?? ""));
    const missedSince = last && last[1] === boot ? Number(last[2]) : null;
    const truncated = missedSince !== null && changes.length > 0 && changes[0].seq > missedSince + 1;
    const missed = missedSince === null ? [] : truncated ? changes : changes.filter((c) => c.seq > missedSince && (!client || c.from !== client));
    if (missed.length) {
      const paths = [...new Set(missed.flatMap((c) => c.paths))];
      const listing = await store.listing();
      const by = missed.every((c) => c.by?.id === missed[0].by?.id) ? missed[0].by : void 0;
      response.write(frame({ type: "external-change", paths, ver: listing.ver, files: filesOf(listing, paths), ...by ? { by } : {} }));
    } else {
      response.write(`id: ${eventId()}

`);
    }
    clients.set(response, client);
    share.hold(viewer, response);
    const leave = presence.join(client, viewer, (text) => response.write(text));
    const keepAlive = setInterval(() => response.write(": ping\n\n"), SSE_KEEPALIVE_MS2);
    keepAlive.unref?.();
    const done = () => {
      clearInterval(keepAlive);
      clients.delete(response);
      leave();
    };
    request.on("close", done);
    response.on("close", done);
  };
  const serveBlob = async (request, response, id) => {
    if (request.method !== "GET" && request.method !== "HEAD") {
      response.writeHead(405, { Allow: "GET, HEAD" }).end();
      return;
    }
    const found = await store.blobFile(id);
    const info = found === null ? null : await fsp4.stat(found.file).catch(() => null);
    if (found === null || !info?.isFile()) {
      response.writeHead(404, { "Cache-Control": "no-store" }).end();
      return;
    }
    sendFile(request, response, found.file, {
      "Content-Type": found.contentType,
      "Content-Length": info.size,
      // Addressed by content: an id's bytes never change.
      "Cache-Control": "private, max-age=31536000, immutable",
      "X-Content-Type-Options": "nosniff",
      "Content-Security-Policy": DATA_CSP
    });
  };
  const serveEditor = async (request, response, rest, viewer, query = "") => {
    const parts = rest === "" ? ["index.html"] : rest.split("/");
    const invalid = parts.some((p) => p === "" || p === "." || p === ".." || p.startsWith(".") || p.includes("\\")) || parts[0] === "node";
    const realDist = invalid ? null : await fsp4.realpath(EDITOR_DIR).catch(() => null);
    const file = realDist === null ? null : path12.join(realDist, ...parts);
    const real = file === null ? null : await fsp4.realpath(file).catch(() => null);
    const info = real === null ? null : await fsp4.lstat(real).catch(() => null);
    if (file === null || real !== file || !info?.isFile()) {
      if (realDist === null && !invalid && parts[0] === "index.html") {
        response.writeHead(503, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" }).end("The editor page is missing; reinstall the skill.");
        return;
      }
      response.writeHead(404, { "Cache-Control": "no-store" }).end();
      return;
    }
    const ext = path12.extname(file).toLowerCase();
    const type = STATIC_TYPES[ext] ?? "application/octet-stream";
    if (ext === ".html") {
      const key = parts.join("/");
      let page = pageCache.get(key);
      if (!page || page.mtimeMs !== info.mtimeMs) {
        const html = await fsp4.readFile(file, "utf8");
        page = { mtimeMs: info.mtimeMs, html, csp: pageCsp(pageScriptSrc(html)) };
        pageCache.set(key, page);
      }
      const body = Buffer.from(share.injectHead(page.html, viewer, query) ?? page.html);
      response.writeHead(200, {
        "Content-Type": type,
        "Content-Length": body.length,
        "Cache-Control": "no-cache",
        "X-Content-Type-Options": "nosniff",
        // A root-relative <link> or <a href> finds its way to this artifact by the page's address (hub.ts).
        "Referrer-Policy": "same-origin",
        "Content-Security-Policy": page.csp
      });
      response.end(request.method === "HEAD" ? void 0 : body);
      return;
    }
    sendFile(request, response, file, {
      "Content-Type": type,
      "Content-Length": info.size,
      // The page's build names its assets by content hash.
      "Cache-Control": parts[0] === "assets" ? "private, max-age=31536000, immutable" : "no-cache",
      "X-Content-Type-Options": "nosniff"
    });
  };
  const publish = async (request, response, viewer) => {
    if (!/^application\/json\b/i.test(request.headers["content-type"] ?? "")) {
      publishError(response, 415, "invalid_content", "expected application/json");
      return;
    }
    const raw = await readBody(request, MAX_PUBLISH_BYTES);
    let files;
    let over = [];
    let merged = [];
    const named = (list) => Array.isArray(list) ? list.filter((o) => typeof o?.path === "string" && typeof o.sha === "string" && SHA256.test(o.sha)).map((o) => ({ path: o.path, sha: o.sha, ...typeof o.base === "string" && SHA256.test(o.base) ? { base: o.base } : {} })) : [];
    try {
      const body = JSON.parse(raw.toString("utf8"));
      if (typeof body?.files !== "object" || body.files === null || Array.isArray(body.files)) throw new Error("files");
      if (!Object.values(body.files).every(isWireEntry)) throw new Error("entry");
      files = body.files;
      over = named(body.over);
      merged = named(body.merged);
    } catch {
      publishError(response, 400, "invalid_content", "malformed publish request");
      return;
    }
    const page = String(request.headers["x-deck-client"] ?? "");
    const outcome = await store.publish(files, whoOf(viewer), over, merged, CLIENT_ID.test(page) ? page : "");
    const names = Object.keys(files).map((p) => p.replace(/^project\//, ""));
    const made = outcome.version ? ` as version ${outcome.version.n}` : "";
    stamp2(`saved from the page by ${viewer.user.name} (${outcome.status}${outcome.body.code ? ` ${outcome.body.code}` : ""})${made}: ${names.join(", ")}`);
    sendJson2(response, outcome.status, outcome.body);
    if (outcome.status === 200 && clients.size > 0) {
      const from = String(request.headers["x-deck-client"] ?? "");
      const shas = outcome.body.shas ?? {};
      const paths = Object.keys(files);
      changed(
        { type: "external-change", paths, ver: String(outcome.body.version ?? ""), files: Object.fromEntries(paths.map((p) => [p, shas[p] ?? null])), by: authorOf(whoOf(viewer)) },
        CLIENT_ID.test(from) ? from : void 0
      );
    }
  };
  const upload = async (request, response) => {
    const contentType = request.headers["content-type"] ?? "application/octet-stream";
    const limit = Math.min(MAX_ASSET_BYTES, LAYOUT.maxBlobBytes(contentType));
    const bytes = await readBody(request, limit);
    if (bytes.length === 0) {
      publishError(response, 400, "invalid_content", "empty upload");
      return;
    }
    const result = await store.putBlob(bytes, contentType);
    stamp2(`uploaded from the page: ${result.url} (${contentType}, ${bytes.length} bytes)`);
    sendJson2(response, 200, result);
  };
  const session = (viewer) => {
    const extras = sessionExtras();
    if (viewer.guest && "userName" in extras) extras.userName = viewer.user.name;
    return { canEdit: canEditAs(viewer.role), ...extras, role: viewer.role };
  };
  const previewRoute = async (request, response, n, rest, viewer) => {
    const method = request.method ?? "GET";
    const state = history.stateAt(n);
    if (!state) {
      request.resume();
      response.writeHead(404, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" }).end(`There is no version ${n} (it may be too old to keep).`);
      return;
    }
    if (rest === ROUTES.publish || rest === ROUTES.assets) {
      request.resume();
      publishError(response, 403, "not_writer", "This is an earlier version: open the current one to edit.");
      return;
    }
    if (rest === COMMENTS_ROUTE || rest.startsWith(`${COMMENTS_ROUTE}/`)) {
      if (method !== "GET") {
        request.resume();
        publishError(response, 403, "not_writer", "This is an earlier version: comment on the current one.");
        return;
      }
      return comments.handle(request, response, rest.slice(COMMENTS_ROUTE.length));
    }
    if (method !== "GET" && method !== "HEAD") {
      response.writeHead(405, { Allow: "GET, HEAD" }).end();
      return;
    }
    if (rest === ROUTES.listing) {
      const files = {};
      for (const [p, sha] of [...state].sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0)) {
        if (store.fileOf(p) === null) continue;
        files[p] = { sha256: sha, size: history.size(sha) ?? 0, contentType: contentTypeOf(p) };
      }
      return sendJson2(response, 200, { ver: `history-${n}`, files });
    }
    if (rest === ROUTES.session) return sendJson2(response, 200, { ...session(viewer), canEdit: false });
    if (rest === ROUTES.events) {
      response.writeHead(200, { "Content-Type": "text/event-stream; charset=utf-8", "Cache-Control": "no-store", Connection: "keep-alive", "X-Accel-Buffering": "no" });
      response.write(": connected\n\n");
      const keepAlive = setInterval(() => response.write(": ping\n\n"), SSE_KEEPALIVE_MS2);
      keepAlive.unref?.();
      request.on("close", () => clearInterval(keepAlive));
      return;
    }
    if (rest.startsWith("project/")) {
      const sha = store.fileOf(rest) === null ? void 0 : state.get(rest);
      const bytes = sha === void 0 ? null : history.read(sha);
      if (bytes === null) {
        response.writeHead(404, { "Cache-Control": "no-store" }).end();
        return;
      }
      response.writeHead(200, {
        "Content-Type": contentTypeOf(rest),
        "Content-Length": bytes.byteLength,
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
        "Content-Security-Policy": DATA_CSP
      });
      response.end(method === "HEAD" ? void 0 : bytes);
      return;
    }
    const blob = BLOB_PATH.exec(rest);
    if (blob) return serveBlob(request, response, blob[1]);
    return serveEditor(request, response, rest, viewer, `?at=${n}`);
  };
  const route = async (request, response, rest, viewer) => {
    const method = request.method ?? "GET";
    const preview = PREVIEW_PATH.exec(rest);
    if (preview) {
      if (preview[2] === void 0) {
        request.resume();
        response.writeHead(308, { Location: `${editorPath}${rest}/`, "Cache-Control": "no-store" }).end();
        return;
      }
      return previewRoute(request, response, Number(preview[1]), preview[2].slice(1), viewer);
    }
    if (rest === PRESENCE_ROUTE) return presence.handle(request, response, viewer);
    if (rest === HISTORY_ROUTE || rest.startsWith(`${HISTORY_ROUTE}/`)) {
      if (await historyHost.handle(request, response, rest.slice(HISTORY_ROUTE.length), viewer)) return;
      request.resume();
      response.writeHead(404, { "Cache-Control": "no-store" }).end();
      return;
    }
    if (rest === COMMENTS_ROUTE || rest.startsWith(`${COMMENTS_ROUTE}/`)) {
      const sub = rest.slice(COMMENTS_ROUTE.length);
      if (sub === "/events") share.hold(viewer, response);
      return comments.handle(request, response, sub);
    }
    if (rest === ROUTES.publish || rest === ROUTES.assets) {
      if (method !== "POST") {
        response.writeHead(405, { Allow: "POST" }).end();
        return;
      }
      if (!sameOrigin(request)) {
        request.resume();
        publishError(response, 403, "not_writer", "cross-origin request");
        return;
      }
      if (!canEditAs(viewer.role)) {
        request.resume();
        publishError(response, 403, "not_writer", `You can view this ${NOUN} but not edit it.`);
        return;
      }
      if (rest === ROUTES.publish) await publish(request, response, viewer);
      else await upload(request, response);
      return;
    }
    const dsSync = DESIGN_SYSTEMS ? DS_SYNC.exec(rest) : null;
    if (dsSync) {
      const folder = dsSync[1];
      const status = systemStatus(projectDir, root, INDEX_FILE, folder);
      if (method === "POST") {
        request.resume();
        if (!sameOrigin(request)) return publishError(response, 403, "not_writer", "cross-origin request");
        if (viewer.guest) return publishError(response, 403, "not_owner", "Only the owner of this computer can sync a design system from its source.");
        if (status === null) return sendJson2(response, 404, { error: "not_found" });
        const [result] = syncSystems(projectDir, root, INDEX_FILE, { namespace: folder });
        return sendJson2(response, 200, { copied: result.copied.length, ...systemStatus(projectDir, root, INDEX_FILE, folder), canSync: true });
      }
      if (method !== "GET" && method !== "HEAD") {
        response.writeHead(405, { Allow: "GET, HEAD, POST" }).end();
        return;
      }
      return status === null ? sendJson2(response, 404, { error: "not_found" }, { "Cache-Control": "no-store" }) : sendJson2(response, 200, { ...status, canSync: !viewer.guest }, { "Cache-Control": "no-store" });
    }
    if (method !== "GET" && method !== "HEAD") {
      response.writeHead(405, { Allow: "GET, HEAD" }).end();
      return;
    }
    if (rest === ROUTES.listing) return sendJson2(response, 200, await store.listing());
    if (rest === ROUTES.session) return sendJson2(response, 200, session(viewer));
    if (rest === ROUTES.events) return events(request, response, viewer);
    const dsPage = DESIGN_SYSTEMS ? DS_PAGE.exec(rest) : null;
    if (dsPage) {
      const html = designSystemPage(projectDir, INDEX_FILE, dsPage[1], NOUN);
      if (html === null) {
        response.writeHead(404, { "Cache-Control": "no-store" }).end();
        return;
      }
      const body = Buffer.from(html);
      response.writeHead(200, {
        "Content-Type": "text/html; charset=utf-8",
        "Content-Length": body.length,
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
        // A README may link anywhere, and the page (no script, its own origin) asks for nothing by its address.
        "Referrer-Policy": "no-referrer",
        "Content-Security-Policy": DATA_CSP
      });
      response.end(method === "HEAD" ? void 0 : body);
      return;
    }
    if (rest.startsWith("project/")) {
      const bytes = await store.readProjectFile(rest);
      if (bytes === null) {
        response.writeHead(404, { "Cache-Control": "no-store" }).end();
        return;
      }
      response.writeHead(200, {
        "Content-Type": contentTypeOf(rest),
        "Content-Length": bytes.byteLength,
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
        "Content-Security-Policy": DATA_CSP
      });
      response.end(method === "HEAD" ? void 0 : bytes);
      return;
    }
    const blob = BLOB_PATH.exec(rest);
    if (blob) return serveBlob(request, response, blob[1]);
    return serveEditor(request, response, rest, viewer);
  };
  const guard = async (response, work) => {
    try {
      await work();
    } catch (error) {
      if (error instanceof TooLarge2) {
        if (!response.headersSent) publishError(response, 413, "too_large", "request body is too large");
        return;
      }
      stamp2(`request failed: ${error instanceof Error ? error.stack ?? error.message : String(error)}`);
      if (!response.headersSent) publishError(response, 500, "upstream_error", "internal error");
      else response.destroy();
    }
  };
  async function handle(request, response) {
    if (await share.route(request, response)) return;
    const viewer = share.viewerOf(request);
    if (!viewer) return share.denied(request, response);
    let rest;
    try {
      rest = decodeURIComponent(new URL(request.url ?? "/", "http://localhost").pathname.slice(1));
    } catch {
      response.writeHead(400).end();
      return;
    }
    return route(request, response, rest, viewer);
  }
  const close = () => {
    stopWatch();
    comments.close();
    share.close();
    presence.close();
    for (const client of clients.keys()) client.end();
    clients.clear();
  };
  return {
    root,
    page: editorPath,
    share,
    handle: (request, response) => guard(response, () => handle(request, response)),
    hasBlob: async (id) => await store.findBlobFile(id) !== null,
    close
  };
}
function artifactTitle(root, projectDir) {
  try {
    const index = JSON.parse(fs12.readFileSync(path12.join(projectDir, INDEX_FILE), "utf8"));
    for (const value of [index.title, index.name]) if (typeof value === "string" && value.trim() && value.trim() !== "project") return value.trim();
  } catch {
  }
  return path12.basename(root);
}
var runningServer2 = (root) => runningServer(root, KIND, ROUTES.session);
function serveForeground(options) {
  return serveHub({ ...options, kind: KIND, script: SCRIPT, portGiven: options.portGiven === true, site: mountSite });
}
async function main(argv) {
  const positional = [];
  const flags = /* @__PURE__ */ new Map();
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (!arg.startsWith("--")) positional.push(arg);
    else if (arg.includes("=")) flags.set(arg.slice(2, arg.indexOf("=")), arg.slice(arg.indexOf("=") + 1));
    else if (arg === "--lan" || arg === "--background") flags.set(arg.slice(2), "true");
    else flags.set(arg.slice(2), argv[++i] ?? "");
  }
  if (positional.length !== 1 || flags.has("help")) {
    process.stderr.write(
      `Usage: node ${path12.basename(SCRIPT)} <root> [--port n] [--host h] [--lan] [--agent-hook "<command>"]
<root> holds project/${INDEX_FILE}.
`
    );
    return 2;
  }
  let root = path12.resolve(positional[0]);
  if (path12.basename(root) === "project" && fs12.existsSync(path12.join(root, INDEX_FILE)) && !fs12.existsSync(path12.join(root, "project"))) {
    root = path12.dirname(root);
  }
  if (!fs12.statSync(path12.join(root, "project", INDEX_FILE), { throwIfNoEntry: false })?.isFile()) {
    process.stderr.write(`${JSON.stringify({ error: "NOT_FOUND", message: `${path12.join(root, "project", INDEX_FILE)} does not exist.` })}
`);
    return 2;
  }
  const port = flags.has("port") ? Number(flags.get("port")) : DEFAULT_PORTS[KIND];
  if (!Number.isInteger(port) || port < 0 || port > 65535) {
    process.stderr.write(`${JSON.stringify({ error: "INVALID_PORT", message: "--port is a number from 0 to 65535." })}
`);
    return 2;
  }
  const existing = await runningServer2(root);
  if (existing) {
    process.stdout.write(`${JSON.stringify({ url: existing.url, root, pid: existing.pid, reused: true }, null, 2)}
`);
    return 0;
  }
  const agentHook = flags.get("agent-hook") || void 0;
  const state = await serveForeground({ root, port, host: flags.get("host") ?? "127.0.0.1", agentHook, lan: flags.has("lan"), portGiven: flags.has("port"), background: flags.has("background") });
  process.stdout.write(`${JSON.stringify({ url: state.url, root, pid: state.pid, ...state.lan ? { lan: state.lan } : {} }, null, 2)}
`);
  return state.joined && flags.has("background") ? 0 : -1;
}
function isMain(moduleUrl) {
  try {
    return Boolean(process.argv[1]) && pathToFileURL(fs12.realpathSync(process.argv[1])).href === moduleUrl;
  } catch {
    return false;
  }
}
function createArtifactServer(kind, moduleUrl) {
  KIND = kind.kind;
  NOUN = kind.noun;
  INDEX_FILE = kind.indexFile;
  DESIGN_SYSTEMS = kind.designSystems === true;
  LAYOUT = kind.layout;
  pageScriptSrc = kind.pageScriptSrc;
  sessionExtras = kind.sessionExtras;
  commentLocation = kind.commentLocation;
  kindLabels = kind.labels;
  kindAgentContext = kind.agentContext;
  SCRIPT = fileURLToPath(moduleUrl);
  EDITOR_DIR = path12.join(path12.dirname(SCRIPT), "editor");
  const runIfMain = () => {
    if (!isMain(moduleUrl)) return;
    main(process.argv.slice(2)).then(
      (code) => {
        if (code >= 0) process.exitCode = code;
      },
      (error) => {
        const given = error.code;
        const code = given === "PORT_IN_USE" || given === "NETWORK_BLOCKED" ? given : "SERVER_FAILED";
        process.stderr.write(`${JSON.stringify({ error: code, message: errorText(error) })}
`);
        process.exitCode = 2;
      }
    );
  };
  return { mountSite, runningServer: runningServer2, serveForeground, readState, runIfMain };
}

var KIND2 = "design-system";
var NOUN2 = "design system";
var INDEX_FILE2 = "design-system.json";
var BLOBS_DIR = "_blobs";
var MAX_PATH_LENGTH = 300;
var MAX_RUNTIME_PATH_UTF8_BYTES = 512;
var UNSAFE_PATH_CHARACTER = /[\u0000-\u001f\u007f-\u009f\u200b\u200e\u200f\u202a-\u202e\u2028\u2029\u2060-\u206f\u2215\u2044\ufeff\uff0f]/;
var RUNTIME_PATH_RESERVED_CHARACTERS = /[%;:\\]/;
var INVISIBLE_PATH_CHARACTERS = /[\u200b-\u200f\u202a-\u202e\u2060-\u206f\ufeff]/g;
var RESERVED_PROTOTYPE_NAMES = /* @__PURE__ */ new Set(["__proto__", "constructor", "prototype"]);
var REFUSED_TOOL_DIRECTORIES = /* @__PURE__ */ new Set(["node_modules"]);
var REFUSED_TOOL_FILENAMES = /* @__PURE__ */ new Set([
  "package.json",
  "package-lock.json",
  "yarn.lock",
  "pnpm-lock.yaml",
  "bun.lock",
  "bun.lockb",
  "bunfig.toml",
  "tsconfig.json",
  "jsconfig.json",
  "deno.json",
  "deno.jsonc",
  "claude.md",
  "claude.local.md",
  "agents.md",
  "agent.md",
  "gemini.md"
]);
var WINDOWS_SHORT_NAME = /~\d/;
var IMAGE_SLOT_STATE_FILENAME = ".image-slots.state.json";
function comparableSegment(segment) {
  let normalized = segment.replace(INVISIBLE_PATH_CHARACTERS, "");
  const colon = normalized.indexOf(":");
  if (colon >= 0) normalized = normalized.slice(0, colon);
  return normalized.normalize("NFC").toUpperCase().toLowerCase().replace(/[. ]+$/, "");
}
var comparablePath = (rel) => rel.split("/").map(comparableSegment).join("/");
function isImageSlotStatePath(rel) {
  const parts = rel.split("/");
  return parts[parts.length - 1] === IMAGE_SLOT_STATE_FILENAME && !parts.slice(0, -1).some((p) => comparableSegment(p).startsWith("."));
}
function explainToolConfigurationPath(rel) {
  const parts = rel.split("/").map(comparableSegment);
  if (parts.some((p) => p.startsWith(".")) && !isImageSlotStatePath(rel)) {
    return "is a dot-file or dot-directory (tool, editor, VCS or agent configuration); never part of a design system";
  }
  if (REFUSED_TOOL_FILENAMES.has(parts[parts.length - 1] ?? "") || parts.some((p) => REFUSED_TOOL_DIRECTORIES.has(p))) {
    return "is a package-manager, toolchain or agent-instruction file that tools auto-load from a working directory; never part of a design system";
  }
  if (parts.some((p) => WINDOWS_SHORT_NAME.test(p))) {
    return 'has a path segment shaped like a Windows 8.3 short name (NAME~1); rename it without the "~digit"';
  }
  return null;
}
function explainReservedSystemPath(rel) {
  const normalized = comparablePath(rel);
  const under = (name) => normalized === name || normalized.startsWith(`${name}/`);
  if (under(INDEX_FILE2)) return `is reserved: ${INDEX_FILE2} is the index; rename the file`;
  if (under("index.html")) return "is reserved: index.html is the page itself in the original format; rename the file";
  if (under("skill.md") || under("artifact-type")) {
    return "is reserved: SKILL.md and artifact-type/ are the design system type\u2019s own files in the original format; rename the file";
  }
  return null;
}
function explainDesignSystemPath(rel) {
  if (rel.length === 0 || rel.length > MAX_PATH_LENGTH || Buffer.byteLength(`project/${rel}`, "utf8") > MAX_RUNTIME_PATH_UTF8_BYTES) {
    return `is empty or longer than ${MAX_PATH_LENGTH} characters; rename it`;
  }
  if (rel.startsWith("/") || UNSAFE_PATH_CHARACTER.test(rel) || RUNTIME_PATH_RESERVED_CHARACTERS.test(rel)) {
    return "has a character a design system path cannot hold (a control or invisible character, a look-alike slash, % ; : or \\); rename it";
  }
  if (rel.endsWith(".cmekshadow")) return "ends in .cmekshadow, which is reserved; rename it";
  for (const part of rel.split("/")) {
    if (part === "" || part === "." || part === "..") return "has an empty path segment or one that is only dots; rename it";
    if (RESERVED_PROTOTYPE_NAMES.has(part)) return `uses "${part}" as a path segment, which is reserved; rename it`;
  }
  return explainToolConfigurationPath(rel) ?? explainReservedSystemPath(rel);
}
var isBlobsPath = (rel) => rel === BLOBS_DIR || rel.startsWith(`${BLOBS_DIR}/`);
function isArtifactFile(rel) {
  if (rel === INDEX_FILE2) return true;
  return !isBlobsPath(rel) && explainDesignSystemPath(rel) === null;
}
var descendInto = (rel) => !isBlobsPath(rel) && explainDesignSystemPath(rel) === null;
var LAYOUT2 = {
  blobDir: BLOBS_DIR,
  isFile: isArtifactFile,
  roots: [{ rel: "", descend: true }],
  descendInto,
  watches: (rel) => isArtifactFile(rel) || descendInto(rel),
  /** The format's limits: 4,096 files (4,080 of them the system's own; 1,024 before 2026-11-05), 15 MiB a file, 16 MiB a save; an upload 20 MiB, an SVG 2 MiB. */
  maxFiles: 4096,
  maxPublishBytes: 16 * 1024 * 1024,
  maxFileBytes: 15 * 1024 * 1024,
  maxBlobBytes: (contentType) => ((contentType.split(";")[0] ?? "").trim().toLowerCase() === "image/svg+xml" ? 2 : 20) * 1024 * 1024,
  readOnlyMessage: (p) => {
    const rel = p.startsWith("project/") ? p.slice("project/".length) : p;
    if (isBlobsPath(rel)) return `${p}: ${BLOBS_DIR}/ holds uploads and can't be written; upload the file instead`;
    return `${p}: ${explainDesignSystemPath(rel) ?? "is not a design system file"}`;
  }
};
function sessionExtras2() {
  try {
    const name = os6.userInfo().username.trim();
    return name ? { userName: name } : {};
  } catch {
    return {};
  }
}
function commentLocation2(input) {
  const detail = input.detail ?? "";
  const cut = detail.lastIndexOf(" \xB7 ");
  const named = /\(([^()]+)\)$/.exec(cut >= 0 ? detail.slice(0, cut) : detail)?.[1];
  return { file: projectFile(named) ?? `project/${INDEX_FILE2}` };
}
var labels = () => (rel) => rel === INDEX_FILE2 ? { en: "Design system settings", zh: "\u8BBE\u8BA1\u7CFB\u7EDF\u8BBE\u7F6E" } : null;
var server = createArtifactServer(
  {
    kind: KIND2,
    noun: NOUN2,
    indexFile: INDEX_FILE2,
    layout: LAYOUT2,
    /** The editor page and artifact-type/run.html: previews run the bundle and preview.html scripts. */
    pageScriptSrc: () => "script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:",
    sessionExtras: sessionExtras2,
    commentLocation: commentLocation2,
    labels
  },
  import.meta.url
);
var { mountSite: mountSite2, serveForeground: serveForeground2, runningServer: runningServer3, readState: readState2 } = server;
server.runIfMain();
export {
  INDEX_FILE2 as INDEX_FILE,
  KIND2 as KIND,
  LAYOUT2 as LAYOUT,
  NOUN2 as NOUN,
  mountSite2 as mountSite,
  readState2 as readState,
  runningServer3 as runningServer,
  serveForeground2 as serveForeground
};
