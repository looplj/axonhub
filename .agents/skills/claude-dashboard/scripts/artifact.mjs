#!/usr/bin/env node
// claude-dashboard: local Artifact tool (prebuilt; do not edit).

import { createHash as createHash5 } from "node:crypto";
import fs10 from "node:fs";
import path9 from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

import { createHash } from "node:crypto";
import fs from "node:fs";
import net from "node:net";
import os from "node:os";
import path from "node:path";
var STATE_FILE = ".artifact-server.json";
var LOG_FILE = ".artifact-server.log";
var EXPORTS_DIR = "exports";
function readState(root) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, STATE_FILE), "utf8"));
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
  fs.writeFileSync(path.join(state.root, STATE_FILE), `${JSON.stringify(state, null, 2)}
`);
  writeGitignore(state.root, state.kind);
}
function removeState(root) {
  const state = readState(root);
  if (state && state.pid !== process.pid && alive(state.pid)) return;
  fs.rmSync(path.join(root, STATE_FILE), { force: true });
}
var GITIGNORE_FILE = ".gitignore";
var gitignoreLines = (kind) => [STATE_FILE, LOG_FILE, ".sharing.json", ...kind === "docs" ? [] : ["/history/"], `/${EXPORTS_DIR}/`, "*.tmp", ".tmp-*", ".DS_Store"];
function writeGitignore(root, kind) {
  const file = path.join(root, GITIGNORE_FILE);
  try {
    fs.writeFileSync(file, ["# Written by baoyu-design when the editor first opened this folder: what only this computer's editor server needs.", ...gitignoreLines(kind), ""].join("\n"), { flag: "wx" });
  } catch {
  }
}
async function listen(server, port, host) {
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, host, () => resolve());
  });
  const address = server.address();
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
  for (const [name, list] of Object.entries(os.networkInterfaces())) {
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
  const server = net.createServer();
  let port;
  try {
    port = await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, address, () => {
        const bound = server.address();
        resolve(typeof bound === "object" && bound ? bound.port : 0);
      });
    });
  } catch {
    return false;
  } finally {
    await new Promise((resolve) => server.close(() => resolve()));
  }
  return port > 0 && answers(address, port);
}
async function listenFree(server, from, host, first) {
  if (await loopbackBlocked(first || from || 1)) throw new NetworkBlocked();
  if (from === 0) return listen(server, 0, host);
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
      return await listen(server, port, host);
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
var pagePathOf = (kind, root) => `/${kind}/${createHash("sha256").update(path.resolve(root)).digest("base64url").slice(0, 22)}/`;
function sendJson(response, status, body, headers = {}) {
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
var TooLarge = class extends Error {
};
function readBody(request, limit) {
  return new Promise((resolve, reject) => {
    const declared = Number(request.headers["content-length"]);
    if (Number.isFinite(declared) && declared > limit) {
      reject(new TooLarge());
      request.resume();
      return;
    }
    const chunks = [];
    let size = 0;
    request.on("data", (chunk) => {
      size += chunk.length;
      if (size > limit) {
        reject(new TooLarge());
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
  const stream = fs.createReadStream(file);
  stream.on("error", () => response.destroy());
  response.on("close", () => stream.destroy());
  stream.pipe(response);
}
var errorText = (error) => error instanceof Error ? error.message : String(error);

import { spawn } from "node:child_process";
import { EventEmitter } from "node:events";
import fs3 from "node:fs";
import http from "node:http";
import os2 from "node:os";
import path3 from "node:path";

import crypto from "node:crypto";
import fs2 from "node:fs";
import path2 from "node:path";
var GUEST_ROLES = ["editor", "commenter", "viewer"];
var isGuestRole = (value) => typeof value === "string" && GUEST_ROLES.includes(value);
var SHARING_FILE = ".sharing.json";
var MAX_GUESTS = 200;
var MAX_NAME = 40;
var token = (bytes = 18) => crypto.randomBytes(bytes).toString("base64url");
var isRecord = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
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
    this.file = path2.join(root, SHARING_FILE);
    this.state = _Sharing.read(this.file);
  }
  static read(file) {
    let raw;
    try {
      raw = JSON.parse(fs2.readFileSync(file, "utf8"));
    } catch {
      return fresh();
    }
    const state = fresh();
    if (!isRecord(raw)) return state;
    state.lan = raw.lan === true;
    if (typeof raw.port === "number" && Number.isInteger(raw.port) && raw.port > 0 && raw.port < 65536) state.port = raw.port;
    if (isRecord(raw.links)) {
      for (const role of GUEST_ROLES) {
        const link = raw.links[role];
        if (isRecord(link) && typeof link.token === "string" && link.token.length >= 16) state.links[role] = { token: link.token, on: link.on !== false };
      }
    }
    if (Array.isArray(raw.guests)) {
      for (const g of raw.guests) {
        if (!isRecord(g) || typeof g.id !== "string" || typeof g.secret !== "string" || !isGuestRole(g.role) || typeof g.link !== "string") continue;
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
    fs2.writeFileSync(tmp, `${JSON.stringify(this.state, null, 2)}
`, { mode: 384 });
    fs2.renameSync(tmp, this.file);
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
  const real = fs3.realpathSync(script2);
  const { mtimeMs, size } = fs3.statSync(real);
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
    const server = http.createServer((request, response) => {
      this.onRequest(request, response).catch((error) => {
        process.stderr.write(`[${(/* @__PURE__ */ new Date()).toISOString()}] request failed: ${error instanceof Error ? error.stack ?? error.message : String(error)}
`);
        if (!response.headersSent) sendJson(response, 500, { code: "internal", message: "internal error" });
        else response.destroy();
      });
    });
    server.on("upgrade", (request, socket, head) => this.onUpgrade(request, socket, head));
    return server;
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
    const file = path3.join(root, LOG_FILE);
    const log = (line) => {
      const text = `[${(/* @__PURE__ */ new Date()).toISOString()}] ${line}
`;
      try {
        fs3.appendFileSync(file, text);
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
      const server = this.createServer();
      try {
        await listen(server, this.port, address);
        this.lanServers.set(address, server);
      } catch (error) {
        server.close();
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
    for (const server of this.lanServers.values()) {
      server.close();
      server.closeAllConnections?.();
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
    const machine = os2.hostname().toLowerCase();
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
      return sendJson(response, 403, { code: "not_granted", message: "Only a local process can do this." });
    }
    if (request.method === "GET") return sendJson(response, 200, info);
    if (request.method !== "POST") {
      request.resume();
      return sendJson(response, 405, { code: "method_not_allowed", message: "GET or POST." });
    }
    let body;
    try {
      body = JSON.parse((await readBody(request, 64 << 10)).toString("utf8"));
    } catch {
      return sendJson(response, 400, { code: "invalid_argument", message: "The call is not JSON." });
    }
    if (typeof body.root !== "string" || !path3.isAbsolute(body.root)) return sendJson(response, 400, { code: "invalid_argument", message: "root: an absolute path." });
    if (body.action === "mount") {
      try {
        const state = await this.add(body.root, { ...typeof body.agentHook === "string" && body.agentHook ? { agentHook: body.agentHook } : {}, lan: body.lan === true });
        return sendJson(response, 200, { ...info, state });
      } catch (error) {
        sendJson(response, error instanceof SiteClash ? 409 : 400, { ...info, code: error.code ?? "SERVER_FAILED", message: errorText(error) });
        return this.endIfEmpty();
      }
    }
    if (body.action === "unmount") {
      const removed = this.remove(body.root);
      const remaining = this.sites.size;
      sendJson(response, 200, { ...info, removed, remaining });
      return this.endIfEmpty();
    }
    return sendJson(response, 400, { code: "invalid_argument", message: "action: mount or unmount." });
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
  fs3.rmSync(path3.join(state.root, STATE_FILE), { force: true });
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
async function openOnHub(call) {
  if (await loopbackBlocked(call.port || 1)) throw new NetworkBlocked();
  const joined = await joinHub(call);
  if (joined) return { state: joined };
  const logFile = path3.join(call.root, LOG_FILE);
  const fd = fs3.openSync(logFile, "a");
  const child = spawn(process.execPath, [...call.command, "--background"], { detached: true, stdio: ["ignore", "ignore", fd] });
  child.unref();
  fs3.closeSync(fd);
  let exited = false;
  child.once("exit", () => exited = true);
  const deadline = Date.now() + 15e3;
  while (Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 120));
    const state = await call.running();
    if (state) return { state };
    if (exited && !readState(call.root)) break;
  }
  return { tail: fs3.existsSync(logFile) ? fs3.readFileSync(logFile, "utf8").split("\n").slice(-12).join("\n") : "" };
}

import { createHash as createHash3 } from "node:crypto";
import fs6 from "node:fs";
import os4 from "node:os";
import path6 from "node:path";

import fs4 from "node:fs";
import fsp from "node:fs/promises";
import path4 from "node:path";
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
  const folder = path4.basename(path4.resolve(root)) || "artifact";
  const skipped = new Set(skip.map((file) => path4.resolve(file)));
  const entries = [];
  const walk = async (dir, rel) => {
    let names;
    try {
      names = await fsp.readdir(dir, { withFileTypes: true });
    } catch (error) {
      if (rel && error.code === "ENOENT") return;
      throw error;
    }
    for (const dirent of names.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)) {
      const name = dirent.name;
      if (JUNK_FILES.has(name) || isTemp(name) || !rel && (LOCAL_FILES.has(name) || LOCAL_DIRS.has(name) && dirent.isDirectory())) continue;
      const file = path4.join(dir, name);
      if (skipped.has(file)) continue;
      const inner = rel ? `${rel}/${name}` : name;
      if (dirent.isDirectory()) await walk(file, inner);
      else if (dirent.isFile()) {
        const stat = await fsp.stat(file).catch(() => null);
        if (stat?.isFile()) entries.push({ name: `${folder}/${inner}`, file, size: stat.size, mode: stat.mode & 511, mtime: stat.mtime });
      }
    }
  };
  await walk(path4.resolve(root), "");
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
      data = entry.data ?? await fsp.readFile(entry.file);
    } catch (error) {
      if (error.code === "ENOENT") continue;
      throw error;
    }
    const crc = crc32(data);
    let method = 0;
    let body = data;
    if (data.length > 0 && !STORED.has(path4.extname(entry.name).toLowerCase())) {
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
  return `${path4.basename(path4.resolve(root)) || "artifact"}-${day}.zip`;
}
async function writeArchive(root, out) {
  const file = path4.resolve(out);
  const entries = await archiveEntries(root, [file]);
  const tmp = path4.join(path4.dirname(file), `.${path4.basename(file)}.${process.pid}.tmp`);
  const handle = await fsp.open(tmp, "w");
  let bytes;
  try {
    bytes = await writeZip(entries, async (chunk) => {
      await handle.write(chunk);
    });
    await handle.close();
  } catch (error) {
    await handle.close().catch(() => {
    });
    await fsp.rm(tmp, { force: true });
    throw error;
  }
  await fsp.rename(tmp, file);
  return { file, files: entries.length, bytes };
}
async function archiveCommand(root, out) {
  const where = out ? path4.resolve(out) : path4.dirname(path4.resolve(root));
  const target = fs4.statSync(where, { throwIfNoEntry: false })?.isDirectory() ? path4.join(where, archiveFileName(root)) : where;
  const { file, files, bytes } = await writeArchive(root, target);
  return { file, folder: path4.basename(path4.resolve(root)), files, bytes, leftOut: [...LOCAL_FILES] };
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

import fs5 from "node:fs";
import fsp2 from "node:fs/promises";
import os3 from "node:os";
import path5 from "node:path";
var MAX_BYTES2 = 4294967295;
function shown(file) {
  const home = os3.homedir();
  return home && (file === home || file.startsWith(home + path5.sep)) ? `~${file.slice(home.length)}` : file;
}
function saved(file, bytes) {
  return { path: file, name: path5.basename(file), folder: path5.dirname(file), shown: shown(file), bytes };
}
function safeFileName(raw) {
  let name = (raw.replace(/\\/g, "/").split("/").pop() ?? "").replace(/[\u0000-\u001f\u007f<>:"|?*]/g, "_").replace(/^[.\s]+|[.\s]+$/g, "");
  if (name.length > 200) {
    const ext = path5.extname(name).slice(0, 16);
    name = name.slice(0, 200 - ext.length) + ext;
  }
  return name || "download";
}
async function openFree(dir, name) {
  const ext = path5.extname(name);
  const stem = name.slice(0, name.length - ext.length);
  for (let n = 0; ; n++) {
    const file = path5.join(dir, n === 0 ? name : `${stem} (${n})${ext}`);
    try {
      return { file, handle: await fsp2.open(file, "wx") };
    } catch (error) {
      if (error.code !== "EEXIST" || n >= 999) throw error;
    }
  }
}
async function saveDownload(request, response, root) {
  const declared = Number(request.headers["content-length"]);
  if (Number.isFinite(declared) && declared > MAX_BYTES2) {
    request.resume();
    return sendJson(response, 413, { code: "too_large", message: "The file is over 4 GiB." });
  }
  const name = safeFileName(new URL(request.url ?? "/", "http://local").searchParams.get("name") ?? "");
  let file = "";
  let handle = null;
  let bytes = 0;
  try {
    const dir = path5.join(root, EXPORTS_DIR);
    await fsp2.mkdir(dir, { recursive: true });
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
    if (file) await fsp2.rm(file, { force: true });
    request.resume();
    const status = error.status ?? 500;
    if (!response.headersSent) sendJson(response, status, { code: status === 413 ? "too_large" : "upstream_error", message: error instanceof Error ? error.message : String(error) });
    return;
  }
  sendJson(response, 200, saved(file, bytes));
}
async function saveArchive(request, response, root) {
  request.resume();
  try {
    const { file } = await archiveCommand(root, void 0);
    sendJson(response, 200, saved(file, fs5.statSync(file).size));
  } catch (error) {
    const tooLarge = error instanceof ArchiveTooLarge;
    sendJson(response, tooLarge ? 413 : 500, { code: tooLarge ? "too_large" : "upstream_error", message: error instanceof Error ? error.message : String(error) });
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
var isRecord2 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
var esc = (text) => text.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
var script = (value) => JSON.stringify(value).replace(/</g, "\\u003c");
function localOwner() {
  let name = "You";
  try {
    name = os4.userInfo().username.trim() || name;
  } catch {
  }
  return { id: "local", name, email: `${name}@localhost` };
}
var guestCookieName = (kind, root) => `${kind.replace(/-/g, "_")}_guest_${createHash3("sha256").update(path6.resolve(root)).digest("hex").slice(0, 12)}`;
function dbAccess(viewer, segs, adminRead = /* @__PURE__ */ new Set()) {
  const admin = canEditAs(viewer.role);
  const read = segs[0] === "data" && segs[1] === "users" && segs.length > 2 ? segs[2] === viewer.user.id : admin || !adminRead.has(segs[0] ?? "");
  return { read, write: read && admin };
}
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
      sendJson(response, 403, { code: "not_granted", message: "Cross-origin write refused." });
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
    if (method === "GET" && p === "/__share/me") return sendJson(response, 200, this.me(viewer)), true;
    if (method === "GET" && p === "/__share/boot.js") {
      const body = this.bootScript(viewer, new URL(request.url ?? "/", "http://local").searchParams);
      response.writeHead(200, { "Content-Type": "text/javascript; charset=utf-8", "Content-Length": Buffer.byteLength(body), "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" }).end(body);
      return true;
    }
    if (method === "GET" && p === "/__share/invite") return sendJson(response, 200, this.invites(viewer, request.headers.host ?? "")), true;
    if (method === "GET" && p === "/__share/ui.js") {
      fs6.stat(this.options.uiFile, (error, stat) => {
        if (error || !stat.isFile()) return sendJson(response, 404, { code: "not_found", message: "The Share panel is missing." });
        sendFile(request, response, this.options.uiFile, { "Content-Type": "text/javascript; charset=utf-8", "Content-Length": stat.size, "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
      });
      return true;
    }
    if (viewer.role !== "owner") return sendJson(response, 403, { code: "not_granted", message: "Only the owner can do this." }), true;
    if ((method === "GET" || method === "HEAD") && p === "/__share/archive") return await sendArchive(request, response, this.options.root), true;
    if (method === "POST" && p === "/__share/archive") return await saveArchive(request, response, this.options.root), true;
    if (method === "POST" && p === "/__share/save") return await saveDownload(request, response, this.options.root), true;
    if (p === "/__share") return await this.shareApi(request, response, method), true;
    sendJson(response, 404, { code: "not_found", message: "Not found" });
    return true;
  }
  /**
   * The tags a page puts first in its <head>: the boot script and the Share panel (the owner's, or a guest's). `query`
   * ("?…") goes to the boot script, for `globals`.
   */
  headTags(_viewer, query = "") {
    let tags = `<script src="${this.page}__share/boot.js${query.replace(/[<>"']/g, "").replace(/&/g, "&amp;")}"></script>`;
    if (fs6.existsSync(this.options.uiFile)) tags += `<script type="module" src="${this.page}__share/ui.js"></script>`;
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
    const share2 = { ...this.me(viewer), noun: this.options.noun, title: this.options.title(), ...this.options.reactions ? { reactions: true } : {} };
    let js = `(function(BASE){${ROOT_SHIM}})(${script(this.page)});${SECURE_CONTEXT_SHIM}window.__baoyuShare=${script(share2)};`;
    for (const [name, value] of Object.entries(this.options.globals?.(viewer, query) ?? {})) js += `window[${script(name)}]=${script(value)};`;
    if (this.options.agentName) js += `window.__baoyuAgentName=${script(this.options.agentName)};`;
    if (viewer.guest) js += GUEST_WATCH;
    return js;
  }
  /** Nobody's request: a page for a browser's navigation, JSON for anything else. */
  denied(request, response) {
    request.resume();
    const navigation = (request.method === "GET" || request.method === "HEAD") && /\btext\/html\b/.test(request.headers.accept ?? "");
    if (!navigation) return sendJson(response, 403, { code: "not_granted", message: `This ${this.options.noun.en} is not shared with you.` });
    const t = this.words(request);
    this.sendPage(response, 403, t.noAccessTitle, `<h1>${esc(t.noAccessTitle)}</h1><p>${esc(t.noAccess)}</p>${this.ownerHint(request, t)}`);
  }
  /** `/` on the server for someone who may see more than one artifact: a page of links. */
  sendList(response, sites) {
    const noun = this.options.noun.en;
    this.sendPage(response, 200, `Open ${noun}s`, `<h1>Open ${esc(noun)}s</h1><ul>${sites.map((site) => `<li><a href="${esc(site.page)}">${esc(site.title)}</a></li>`).join("")}</ul>`);
  }
  /** A small page of our own (the join form, the refusal), in the reader's language. */
  sendPage(response, status, title, body, headers = {}) {
    const html = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${esc(title)}</title><style>${PAGE_CSS}</style></head><body><main>${body}</main></body></html>`;
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
    return `<p class="note">${esc(t.ownerHere)} <a href="${esc(url)}">${esc(url)}</a></p>`;
  }
  /** GET /s/<token>: the name form (or straight in, for a guest already in through this link); POST: join. */
  async join(request, response, method, token2) {
    const t = this.words(request);
    const role = this.sharing.linkRole(token2);
    const local = this.isLocal(request);
    const form = method === "POST" ? await readBody(request, 8 << 10).then((body) => body.toString("utf8"), () => "") : "";
    const title = this.options.title();
    if (!role || !local && !this.on) return this.sendPage(response, 404, t.linkGoneTitle, `<h1>${esc(t.linkGoneTitle)}</h1><p>${esc(t.linkGone)}</p>`);
    if (local) {
      const lan = this.on ? this.network.addresses()[0] : void 0;
      const there = lan ? `http://${lan}:${this.port}${this.page}s/${token2}` : "";
      const tryIt = there ? `<p>${esc(t.tryRole(t.roles[role]))} <a href="${esc(there)}">${esc(there)}</a></p>` : "";
      return this.sendPage(response, 200, title, `<h1>${esc(title)}</h1><p>${esc(t.youOwn)} <a href="${esc(this.page)}">${esc(t.openAsOwner)}</a></p>${tryIt}`);
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
        `<h1>${esc(title)}</h1><p>${esc(t.invited(t.roles[role]))}</p><form method="post"><label for="n">${esc(t.yourName)}</label><input id="n" name="name" maxlength="${MAX_NAME}" value="${esc(current?.name ?? "")}" required autofocus autocomplete="name"><button type="submit">${esc(t.open)}</button></form><p class="note">${esc(t.nameNote)}</p>`
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
    if (method === "GET") return sendJson(response, 200, this.shareView());
    if (method !== "POST") return sendJson(response, 405, { code: "method_not_allowed", message: `${method} is not allowed here.` });
    let action;
    try {
      action = JSON.parse((await readBody(request, 64 << 10)).toString("utf8"));
    } catch {
      return sendJson(response, 400, { code: "invalid_argument", message: "The call is not JSON." });
    }
    if (!isRecord2(action)) return sendJson(response, 400, { code: "invalid_argument", message: "Expected {action, ...}." });
    try {
      return sendJson(response, 200, await this.share(action));
    } catch (error) {
      if (error instanceof ShareError) return sendJson(response, 400, { code: "invalid_argument", message: error.message });
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
  let open2 = null;
  let i = 0;
  let j = 0;
  for (const op of ops) {
    if (op === "=") {
      open2 = null;
      i++;
      j++;
      continue;
    }
    if (!open2) out.push(open2 = { start: i, end: i, lines: [], mine });
    if (op === "-") open2.end = ++i;
    else open2.lines.push(side[j++]);
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

var isObject = (v) => typeof v === "object" && v !== null && !Array.isArray(v);
var canon = (v) => {
  if (v === void 0) return void 0;
  if (Array.isArray(v)) return `[${v.map((x) => canon(x) ?? "null").join(",")}]`;
  if (isObject(v)) {
    return `{${Object.keys(v).filter((k) => v[k] !== void 0).sort().map((k) => `${JSON.stringify(k)}:${canon(v[k])}`).join(",")}}`;
  }
  return JSON.stringify(v);
};
var same = (a, b) => canon(a) === canon(b);
var byId = (v) => {
  if (!Array.isArray(v)) return null;
  const out = /* @__PURE__ */ new Map();
  for (const item of v) {
    if (!isObject(item) || typeof item.id !== "string" || out.has(item.id)) return null;
    out.set(item.id, item);
  }
  return out;
};
var parseJson = (text) => {
  if (typeof text !== "string") return null;
  try {
    return { value: JSON.parse(text) };
  } catch {
    return null;
  }
};
var layoutLike = (value, like) => {
  if (!like.includes("\n")) return JSON.stringify(value);
  const indent = /\n([ \t]+)\S/.exec(like)?.[1] ?? "  ";
  return JSON.stringify(value, null, indent) + (like.endsWith("\n") ? "\n" : "");
};
function mergeDoc(base, mine, current) {
  if (current === null) return { result: mine, conflicts: [] };
  const conflicts = [];
  const result = mergeValue(base, mine, current, "", conflicts);
  return { result: isObject(result) ? result : mine, conflicts };
}
function mergeWrite(base, mine, current) {
  if (mine === null) return { result: null, conflicts: current === null || same(current, base) ? [] : ["."] };
  if (current === null) return same(mine, base) ? { result: null, conflicts: [] } : { result: mine, conflicts: ["."] };
  return mergeDoc(base, mine, current);
}
function mergeValue(base, mine, current, at, conflicts) {
  if (same(mine, base)) return current;
  if (same(current, base) || same(mine, current)) return mine;
  if (isObject(base) && isObject(mine) && isObject(current)) return mergeObject(base, mine, current, at, conflicts);
  const b = byId(base);
  const m = byId(mine);
  const c = byId(current);
  if (b && m && c) return mergeList(b, m, c, mine, current, at, conflicts);
  if (typeof base === "string" && typeof mine === "string" && typeof current === "string") {
    const text = mergeText(base, mine, current, at || ".", conflicts);
    if (text !== null) return text;
  }
  conflicts.push(at || ".");
  return mine;
}
function mergeObject(base, mine, current, at, conflicts) {
  const out = {};
  const keys = [.../* @__PURE__ */ new Set([...Object.keys(mine), ...Object.keys(current), ...Object.keys(base)])];
  const jsonText = mine.kind === "json" && current.kind === "json" && base.kind === "json";
  for (const k of keys) {
    if (k === "__proto__") continue;
    const where = at ? `${at}.${k}` : k;
    let v;
    const texts = jsonText && k === "text" ? [parseJson(base.text), parseJson(mine.text), parseJson(current.text)] : null;
    if (texts && texts[0] && texts[1] && texts[2] && !same(mine.text, base.text) && !same(current.text, base.text)) {
      const merged = mergeValue(texts[0].value, texts[1].value, texts[2].value, where, conflicts);
      v = same(merged, texts[1].value) ? mine.text : same(merged, texts[2].value) ? current.text : layoutLike(merged, mine.text);
    } else {
      v = mergeValue(base[k], mine[k], current[k], where, conflicts);
    }
    if (v !== void 0) out[k] = v;
  }
  return out;
}
function mergeList(base, mine, current, mineList, currentList, at, conflicts) {
  const merged = /* @__PURE__ */ new Map();
  for (const id of /* @__PURE__ */ new Set([...mine.keys(), ...current.keys(), ...base.keys()])) {
    const v = mergeValue(base.get(id), mine.get(id), current.get(id), `${at}[${id}]`, conflicts);
    if (v !== void 0) merged.set(id, v);
  }
  const order = mineList.map((item) => item.id).filter((id) => merged.has(id));
  currentList.forEach((item, i) => {
    const id = item.id;
    if (!merged.has(id) || order.includes(id)) return;
    let after = -1;
    for (let j = i - 1; j >= 0 && after < 0; j--) after = order.indexOf(currentList[j].id);
    order.splice(after + 1, 0, id);
  });
  return order.map((id) => merged.get(id));
}
var utf8 = new TextDecoder("utf-8", { fatal: true });

import { createHash as createHash4, randomBytes } from "node:crypto";
import fs7 from "node:fs";
import path7 from "node:path";
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
var sha256 = (bytes) => createHash4("sha256").update(bytes).digest("hex");
var SHA = /^[0-9a-f]{64}$/;
var isRecord3 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
function writeAtomic(file, data) {
  fs7.mkdirSync(path7.dirname(file), { recursive: true });
  const tmp = path7.join(path7.dirname(file), `.${path7.basename(file)}.${process.pid}.${randomBytes(4).toString("hex")}.tmp`);
  try {
    fs7.writeFileSync(tmp, data);
    fs7.renameSync(tmp, file);
  } catch (error) {
    fs7.rmSync(tmp, { force: true });
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
    this.dir = path7.join(root, HISTORY_DIR);
    let raw = null;
    try {
      raw = JSON.parse(fs7.readFileSync(path7.join(this.dir, INDEX), "utf8"));
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
      return fs7.readFileSync(path7.join(this.dir, OBJECTS, sha));
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
      return fs7.statSync(path7.join(this.dir, OBJECTS, sha)).size;
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
      const same3 = (n) => n.path === o.path && n.n === version.n;
      this.index.notices = this.index.notices.filter((n) => !same3(n));
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
    const file = path7.join(this.dir, OBJECTS, sha);
    if (fs7.existsSync(file)) return;
    writeAtomic(file, bytes);
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
      names = fs7.readdirSync(path7.join(this.dir, OBJECTS));
    } catch {
      return;
    }
    for (const name of names) if (SHA.test(name) && !keep.has(name)) fs7.rmSync(path7.join(this.dir, OBJECTS, name), { force: true });
  }
  save() {
    writeAtomic(path7.join(this.dir, INDEX), `${JSON.stringify(this.index)}
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
      sendJson(response, 403, { code: "not_granted", message: "Cross-origin write refused." });
      return true;
    }
    if (method === "GET" && (sub === "" || sub === "/")) return sendJson(response, 200, this.list(viewer)), true;
    if (sub === "/notices") {
      if (method === "GET") return sendJson(response, 200, { notices: this.notices(viewer, this.options.labels()) }), true;
      if (method !== "POST") return sendJson(response, 405, { code: "method_not_allowed", message: "GET or POST" }), true;
      const body = await this.body(request, response);
      if (!body) return true;
      if (typeof body.dismiss !== "string") return sendJson(response, 400, { code: "invalid_argument", message: "Expected {dismiss: id}." }), true;
      this.history.dismiss(viewer.user.id, body.dismiss);
      return sendJson(response, 200, { notices: this.notices(viewer, this.options.labels()) }), true;
    }
    const versionRoute = /^\/(\d+)\/(diff|restore)$/.exec(sub);
    if (versionRoute) {
      const n = Number(versionRoute[1]);
      const version = this.history.version(n);
      if (!version) return sendJson(response, 404, { code: "not_found", message: `There is no version ${n} (it may be too old to keep).` }), true;
      if (versionRoute[2] === "diff") {
        if (method !== "GET") return sendJson(response, 405, { code: "method_not_allowed", message: "GET" }), true;
        const p = url.searchParams.get("path") ?? "";
        if (!(p in version.changes)) return sendJson(response, 404, { code: "not_found", message: `Version ${n} did not change ${p}.` }), true;
        const before = this.history.stateBefore(n).get(p) ?? null;
        const after = version.changes[p] ?? null;
        return sendJson(response, 200, this.diff(p, before, after)), true;
      }
      if (method !== "POST") return sendJson(response, 405, { code: "method_not_allowed", message: "POST" }), true;
      if (!canEditAs(viewer.role)) return sendJson(response, 403, { code: "not_writer", message: "Only the owner and editors can bring a version back." }), true;
      const body = await this.body(request, response);
      if (!body) return true;
      const paths = Array.isArray(body.paths) ? body.paths.filter((p) => typeof p === "string") : void 0;
      const changes = this.history.restoreTo(n, paths, this.options.tracked);
      if (!changes) return sendJson(response, 409, { code: "unavailable", message: `Version ${n} can't be brought back: a file it holds is missing from the history.` }), true;
      const made = await this.options.apply(changes, whoOf(viewer), { restoredFrom: n });
      this.options.log(`history: ${viewer.user.name} brought back version ${n}${paths ? ` (${paths.join(", ")})` : ""}${made ? ` as version ${made.n}` : " (nothing to change)"}`);
      return sendJson(response, 200, { version: made?.n ?? null, changed: [...changes.keys()] }), true;
    }
    const noticeRoute = /^\/notice\/([0-9a-f]{6,32})\/(diff|recover)$/.exec(sub);
    if (noticeRoute) {
      const notice = this.history.notice(noticeRoute[1]);
      if (!notice || notice.to !== viewer.user.id) return sendJson(response, 404, { code: "not_found", message: "No such notice." }), true;
      if (noticeRoute[2] === "diff") {
        if (method !== "GET") return sendJson(response, 405, { code: "method_not_allowed", message: "GET" }), true;
        const now2 = this.history.head().get(notice.path) ?? null;
        const nowBytes = now2 === null ? null : this.history.read(now2);
        const back2 = this.recovered(notice, nowBytes);
        if (now2 && !nowBytes || !back2) return sendJson(response, 200, { path: notice.path, label: this.options.labels()(notice.path), missing: true, added: 0, removed: 0, lines: [], now: now2 }), true;
        return sendJson(response, 200, { ...this.diffBytes(notice.path, nowBytes, back2.bytes), now: now2 }), true;
      }
      if (method !== "POST") return sendJson(response, 405, { code: "method_not_allowed", message: "POST" }), true;
      if (!canEditAs(viewer.role)) return request.resume(), sendJson(response, 403, { code: "not_writer", message: "Only the owner and editors can change the files." }), true;
      const body = await this.body(request, response);
      if (!body) return true;
      const now = this.history.head().get(notice.path) ?? null;
      if ("now" in body && body.now !== now) {
        return sendJson(response, 409, { code: "changed", message: "This changed while you were looking. Compare again." }), true;
      }
      const back = this.recovered(notice, now === null ? null : this.history.read(now));
      if (!back) return sendJson(response, 409, { code: "unavailable", message: "That content is no longer in the history." }), true;
      const made = await this.options.apply(/* @__PURE__ */ new Map([[notice.path, back.bytes]]), whoOf(viewer), { restoredFrom: notice.lost.n, ...back.part ? { restoredPart: true } : {} });
      this.history.dismiss(viewer.user.id, notice.id);
      this.options.log(`history: ${viewer.user.name} brought back ${notice.lost.user.name}'s ${back.bytes === null ? "deletion of " : ""}${notice.path} (version ${notice.lost.n}${back.part ? ", what was replaced" : ""})`);
      return sendJson(response, 200, { version: made?.n ?? null }), true;
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
    sendJson(response, 400, { code: "invalid_argument", message: "Expected a JSON object." });
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
var plain = (text) => ({ en: text, zh: text });

var OVERLAP_MS = 6e4;
var docBytes = (data) => Buffer.from(`${JSON.stringify(data, null, 2)}
`);
var parse = (bytes) => {
  if (bytes === null) return null;
  try {
    const value = JSON.parse(bytes.toString("utf8"));
    return typeof value === "object" && value !== null && !Array.isArray(value) ? value : null;
  } catch {
    return null;
  }
};
var same2 = (a, b) => JSON.stringify(a) === JSON.stringify(b);
var DbHistory = class {
  history;
  host;
  options;
  /**
   * Document → who is writing it from the page, and what the write is (a restore; a merged write: the document it was
   * made on and the values in it that replaced someone else's), until the rescan after it.
   */
  pending;
  /** The first rescan also looks for documents deleted while no server ran. */
  first;
  constructor(options) {
    this.options = options;
    this.history = new History(options.root);
    this.pending = /* @__PURE__ */ new Map();
    this.first = true;
    this.host = new HistoryHost({
      history: this.history,
      labels: () => {
        const named = options.labels();
        return (p) => named(p) ?? plain(p);
      },
      base: options.base,
      previewUrl: (n) => `${options.base}?at=${n}`,
      apply: async (changes, who, extra) => this.apply(changes, who, extra),
      tracked: options.tracked,
      diffSides: textSides,
      recover: (_path, base, lost, now) => {
        const b = parse(base);
        const l = parse(lost);
        const n = parse(now);
        if (!b || !l) return null;
        return n ? docBytes(mergeDoc(b, l, n).result) : lost;
      },
      log: options.log
    });
  }
  /**
   * A page's write of `path` by `who`, until `settle`: the rescan in between records it as theirs. `merge`: the write was
   * merged (mergeDoc) from this base, and these are the values in it that replaced someone else's (empty: nothing lost).
   */
  attribute(path10, who, merge) {
    this.pending.set(path10, { who, ...merge ? { merge } : {} });
  }
  /** After the page's write and its rescan (the write may have changed nothing, or failed). */
  settle(path10) {
    this.pending.delete(path10);
  }
  /** Records the rescan's changes; `present` says which documents are on disk now (for the first rescan). */
  record(changes, present) {
    const groups = /* @__PURE__ */ new Map();
    const fallback = this.history.empty ? SYSTEM : AGENT;
    const add = (who, restore, p, bytes, merge) => {
      const key = `${who.by}\0${who.user.id}\0${restore.restoredFrom ?? ""}\0${restore.restoredPart ? "part" : ""}`;
      let group = groups.get(key);
      if (!group) groups.set(key, group = { who, restore, files: /* @__PURE__ */ new Map(), merged: /* @__PURE__ */ new Map() });
      group.files.set(p, bytes);
      if (merge) group.merged.set(p, merge);
    };
    for (const c of changes) {
      if (!this.options.tracked(c.path)) continue;
      const by = this.pending.get(c.path);
      const restore = by?.restoredFrom !== void 0 ? { restoredFrom: by.restoredFrom, ...by.restoredPart ? { restoredPart: true } : {} } : {};
      add(by?.who ?? fallback, restore, c.path, c.data === null ? null : docBytes(c.data), by?.merge);
    }
    if (this.first) {
      this.first = false;
      for (const p of this.history.head().keys()) if (this.options.tracked(p) && !present(p) && !changes.some((c) => c.path === p)) add(fallback, {}, p, null);
    }
    const made = [];
    for (const group of groups.values()) {
      const overwrote = group.who.by !== "system" && group.restore.restoredFrom === void 0 ? this.overlaps(group.files, group.who, group.merged) : [];
      const version = this.history.record(group.files, group.who, { ...group.restore, ...overwrote.length ? { overwrote } : {} });
      if (!version) continue;
      made.push(version);
      if (group.who.by !== "human") {
        const what = group.who.by === "agent" ? "changed outside the page" : "the documents as they were";
        this.options.log(`history: version ${version.n} (${what}): ${[...group.files.keys()].join(", ")}`);
      }
    }
    return made;
  }
  /**
   * Whose values on these documents this change replaced. A merged write says (`merged`: path → its base and the values
   * it replaced; the notice goes to the person behind the document's last version, when that is someone else, and keeps
   * the base so bringing the lost values back leaves the rest; chosen when its person saw that version and saved over
   * it). Otherwise a guess, for a change another person made in
   * the last minute: part of it is gone when merging it (the document before it → theirs) into this one changes this
   * one. The document before it is the notice's base. Their change a deletion: it is gone when the document is back.
   */
  overlaps(files, who, merged) {
    const now = Date.now();
    const out = [];
    for (const [p, bytes] of files) {
      const last = this.history.lastChange(p);
      if (!last) continue;
      const v = last.version;
      if (v.by !== "human" || v.user.id === who.user.id || v.restoredFrom !== void 0) continue;
      if ((this.history.head().get(p) ?? null) !== last.sha) continue;
      const lost = { path: p, lost: last.sha, n: v.n, by: "human", user: { ...v.user }, soft: true };
      const merge = merged.get(p);
      if (merge) {
        if (!merge.conflicts.length) continue;
        const { soft: _soft, ...hard } = lost;
        const over = merge.chosen ? { ...hard, chosen: true } : lost;
        out.push(last.sha === null ? over : { ...over, base: this.history.put(docBytes(merge.base)) });
        continue;
      }
      if (now - Date.parse(v.at) > OVERLAP_MS) continue;
      if (last.sha === null) {
        if (bytes !== null) out.push(lost);
        continue;
      }
      const theirs = parse(this.history.read(last.sha));
      const was = parse(this.history.read(this.history.stateBefore(v.n).get(p) ?? "") ?? null) ?? {};
      const next = parse(bytes) ?? {};
      if (!theirs || same(mergeDoc(was, theirs, next).result, next)) continue;
      out.push({ ...lost, base: this.history.stateBefore(v.n).get(p) ?? this.history.put(docBytes({})) });
    }
    return out;
  }
  /** A restore (or a recovered notice): writes the documents as `who`'s; the rescan after records them. */
  apply(changes, who, extra) {
    const paths = [];
    try {
      for (const [p, bytes] of changes) {
        const data = bytes === null ? null : parse(bytes);
        if (bytes !== null && data === null) continue;
        this.pending.set(p, { who, ...extra.restoredFrom !== void 0 ? { restoredFrom: extra.restoredFrom, ...extra.restoredPart ? { restoredPart: true } : {} } : {} });
        paths.push(p);
        this.options.write(p, data);
      }
      this.options.rescan();
      const versions = this.history.versions();
      const newest = versions.at(-1);
      return newest && newest.user.id === who.user.id && newest.restoredFrom === extra.restoredFrom && paths.some((p) => p in newest.changes) ? newest : null;
    } finally {
      for (const p of paths) this.pending.delete(p);
    }
  }
  /**
   * On a version's page (?at=n): the documents as that version left them in place of the kept ones now, the page's
   * own as they are. null: there is no such version.
   */
  docsAt(n, current) {
    const state = this.history.stateAt(n);
    if (!state) return null;
    const out = /* @__PURE__ */ new Map();
    for (const [p, data] of current) if (!this.options.tracked(p)) out.set(p, data);
    for (const [p, sha] of state) {
      if (!this.options.tracked(p)) continue;
      const data = parse(this.history.read(sha));
      if (data) out.set(p, data);
    }
    return out;
  }
  /** window.__baoyuHistory for this person; `query` is the page's (?at=n on a version's page). */
  bootGlobal(viewer, query) {
    return this.host.bootGlobal(viewer, previewOf(query));
  }
};
function previewOf(query) {
  const at = query.get("at");
  return at !== null && /^\d{1,9}$/.test(at) ? Number(at) : void 0;
}
function textSides(_path, before, after) {
  const a = parse(before);
  const b = parse(after);
  const textOf2 = (d) => d && typeof d.text === "string" ? d.text : null;
  const rest = (d) => {
    if (!d) return null;
    const { text: _text, ...others } = d;
    return others;
  };
  const ta = textOf2(a);
  const tb = textOf2(b);
  if ((ta !== null || a === null) && (tb !== null || b === null) && (a === null || b === null || same2(rest(a), rest(b)))) {
    return [ta === null ? null : Buffer.from(ta), tb === null ? null : Buffer.from(tb)];
  }
  return [before, after];
}

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
    let open2 = true;
    return () => {
      if (!open2) return;
      open2 = false;
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
    if (method === "GET") return sendJson(response, 200, { peers: this.list() });
    if (method !== "POST") {
      request.resume();
      return sendJson(response, 405, { code: "method_not_allowed", message: "GET or POST" });
    }
    if (!sameOrigin(request)) {
      request.resume();
      return sendJson(response, 403, { code: "not_granted", message: "Cross-origin write refused." });
    }
    let body;
    try {
      body = JSON.parse((await readBody(request, MAX_PATCH_BYTES)).toString("utf8"));
    } catch {
      return sendJson(response, 400, { code: "invalid_argument", message: `Expected {peer, patch} under ${MAX_PATCH_BYTES} bytes.` });
    }
    const { peer, patch } = typeof body === "object" && body !== null ? body : {};
    if (typeof peer !== "string" || !PEER_ID.test(peer)) return sendJson(response, 400, { code: "invalid_argument", message: "Expected {peer, patch}." });
    if (!this.update(peer, viewer, patch)) return sendJson(response, 409, { code: "not_joined", message: "That page is not open here (its event stream is closed)." });
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

async function shareCall(url, action, fail2) {
  const response = await fetch(new URL("__share", url), action ? { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(action) } : {});
  const body = await response.json().catch(() => ({}));
  if (!response.ok) fail2("SHARE_REFUSED", String(body.message ?? `the server answered ${response.status}`));
  return body;
}
function rolesOf(flags, key, fail2) {
  const given = flags.get(key);
  if (given === void 0) return [];
  const roles = given === "all" || given === "true" ? [...GUEST_ROLES] : given.split(",").map((r) => r.trim());
  for (const role of roles) if (!isGuestRole(role)) fail2("INVALID_ROLE", `--${key}: editor, commenter, viewer or all (got ${role}).`);
  return roles;
}
function guestRoles(flags, fail2) {
  const given = flags.get("role");
  if (given === void 0) return [];
  return given.split(",").map((pair) => {
    const [guest = "", role = ""] = pair.split("=").map((part) => part.trim());
    if (!guest || !isGuestRole(role)) fail2("INVALID_ROLE", `--role: <guest id>=editor|commenter|viewer, comma separated (got ${pair}).`);
    return { action: "role", guest, role };
  });
}
async function shareCommand(o) {
  const off = o.flags.has("off");
  const changes = [
    ...rolesOf(o.flags, "reset", o.fail).map((role) => ({ action: "reset", role })),
    ...rolesOf(o.flags, "enable", o.fail).map((role) => ({ action: "link", role, on: true })),
    ...rolesOf(o.flags, "disable", o.fail).map((role) => ({ action: "link", role, on: false })),
    ...guestRoles(o.flags, o.fail)
  ];
  const open2 = !off && changes.length === 0;
  let server = await o.running();
  if (!server) {
    const sharing = new Sharing(o.root);
    for (const change of changes) {
      if (change.action === "reset") sharing.resetLink(change.role);
      else if (change.action === "role") {
        if (!sharing.setRole(String(change.guest), change.role)) o.fail("SHARE_REFUSED", `No guest ${String(change.guest)}, or the ${String(change.role)} link is off.`);
      } else sharing.setLink(change.role, change.on === true);
    }
    if (off) {
      sharing.setLan(false);
      return { lan: false, message: "Not shared on the local network. Guests already in get their place back when it is shared again (unless their link was reset)." };
    }
    if (!open2 && !sharing.state.lan) {
      return {
        lan: false,
        links: Object.fromEntries(GUEST_ROLES.map((role) => [role, sharing.state.links[role].on ? "on" : "off"])),
        message: `Links changed. Not shared on the local network: ${NOT_SHARED_HINT}`
      };
    }
    server = await o.startShared();
    if (!server) o.fail("SERVER_FAILED", `The server did not start. Run ${o.serveCommand} in the foreground to see why.`);
  }
  let view = await shareCall(server.url, null, o.fail);
  for (const change of changes) view = await shareCall(server.url, change, o.fail);
  if (off || open2 && !view.lan) view = await shareCall(server.url, { action: "lan", on: !off }, o.fail);
  const links = view.links;
  const shown2 = (list) => Object.fromEntries(Object.entries(list).map(([role, link]) => [role, link.on ? link.url : "off"]));
  const tailnet = view.lan ? view.tailnet : void 0;
  return {
    lan: view.lan,
    ...view.lan ? { addresses: lanUrls(view) } : {},
    links: shown2(links),
    ...tailnet ? {
      tailscale: shown2(tailnet.links),
      tailscaleNote: `This computer has a Tailscale address (${tailnet.address}): the tailscale links open the same roles for people on your tailnet, from any network. Use the links above for people on this Wi-Fi or LAN.`
    } : {},
    guests: view.guests,
    ...view.lan ? {
      note: "Send the link for the role you want to give: editor edits, commenter comments, viewer reads. Anyone on this network with a link joins under a name they type. The owner (this computer) shares from the Share button too.",
      security: "It is plain http: others on the network can read what passes. A link is the permission: whoever has it gets its role, so send it only to the people meant to have it, and reset a role's link if it went further.",
      stop: o.offCommand
    } : { note: `Not shared on the network: the links work on this computer only. ${NOT_SHARED_HINT}` }
  };
}
var NOT_SHARED_HINT = "share <root> with no options shares it.";
var lanUrls = (view) => (Array.isArray(view.addresses) ? view.addresses : []).map((a) => `http://${String(a)}:${String(view.port)}${String(view.page ?? "/")}`);
async function lanOn(url, flags, fail2) {
  if (!flags.has("lan")) return {};
  return { lan: lanUrls(await shareCall(url, { action: "lan", on: true }, fail2)) };
}

var COMMENTS_FILE = "comments.json";
var COMMENTS_VERSION = 1;
var LIMITS = { text: 1e4, label: 1e3, detail: 4e3, anchor: 2048, section: 200, name: 100 };
function sameAuthor(a, b) {
  return a.kind === "user" && b.kind === "user" && (a.principal ?? "") === (b.principal ?? "");
}
var AGENT_AUTHOR = { kind: "agent", name: "agent" };
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
var isRecord4 = (v) => typeof v === "object" && v !== null && !Array.isArray(v);
var isText = (v, max) => typeof v === "string" && v.length <= max;
function cleanText(v) {
  if (typeof v !== "string") return null;
  const text = v.replace(/\r\n?/g, "\n").trim();
  return text !== "" && text.length <= LIMITS.text ? text : null;
}
function readNewThread(body) {
  if (!isRecord4(body)) return null;
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
  if (!isRecord4(body)) return null;
  const text = cleanText(body.text);
  if (text === null) return null;
  if (body.send !== void 0 && typeof body.send !== "boolean") return null;
  return { text, ...body.send ? { send: true } : {} };
}
var readEdit = readReply;
function readStatus(body) {
  return isRecord4(body) && (body.status === "open" || body.status === "resolved") ? body.status : null;
}
function readAgentRequest(body) {
  if (!isRecord4(body)) return null;
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
  if (!isRecord4(data) || data.version !== COMMENTS_VERSION || !Array.isArray(data.threads)) {
    throw new CommentsError("COMMENTS_FILE_INVALID", `not a version ${COMMENTS_VERSION} comments file (expected {"version": 1, "threads": [...]})`, 500);
  }
  const threads = data.threads.filter(
    (t) => isRecord4(t) && typeof t.id === "string" && typeof t.anchor === "string" && typeof t.label === "string" && (t.status === "open" || t.status === "resolved") && Array.isArray(t.messages) && t.messages.every((m) => isRecord4(m) && typeof m.text === "string" && isRecord4(m.author) && (m.author.kind === "user" || m.author.kind === "agent"))
  );
  for (const t of threads) {
    if (!isRecord4(t.location)) t.location = {};
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
  const open2 = openThreads(doc);
  return threads.map((t) => {
    const i = open2.indexOf(t);
    return agentView(t, i >= 0 ? i + 1 : void 0);
  });
}

import { spawn as spawn2 } from "node:child_process";
import fs9 from "node:fs";
import os5 from "node:os";

import fs8 from "node:fs";
import fsp3 from "node:fs/promises";
import path8 from "node:path";
var LOCK_STALE_MS = 1e4;
var LOCK_WAIT_MS = 5e3;
var LOCK_RETRY_MS = 20;
var commentsPath = (root) => path8.join(root, COMMENTS_FILE);
async function readComments(root) {
  let text;
  try {
    text = await fsp3.readFile(commentsPath(root), "utf8");
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
      const handle = await fsp3.open(file, "wx");
      await handle.writeFile(`${process.pid}
`);
      await handle.close();
      return () => fsp3.rm(file, { force: true });
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
    }
    const info = await fsp3.stat(file).catch(() => null);
    if (info && Date.now() - info.mtimeMs > LOCK_STALE_MS) {
      await fsp3.rm(file, { force: true });
      continue;
    }
    if (Date.now() > deadline) throw new CommentsError("COMMENTS_LOCKED", `${file} is held by another process; try again (or delete it if nothing is running).`, 503);
    await new Promise((resolve) => setTimeout(resolve, LOCK_RETRY_MS));
  }
}
async function writeAtomic2(file, text) {
  const temp = path8.join(path8.dirname(file), `.${path8.basename(file)}.${process.pid}.${Date.now().toString(36)}.tmp`);
  await fsp3.writeFile(temp, text);
  try {
    await fsp3.rename(temp, file);
  } catch (error) {
    await fsp3.rm(temp, { force: true });
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
    const exists = fs8.existsSync(commentsPath(root));
    if (after !== before && (exists || doc.threads.length > 0)) await writeAtomic2(commentsPath(root), after);
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
var TooLarge2 = class extends Error {
};
function sendJson2(response, status, body) {
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
        reject(new TooLarge2());
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
    return os5.userInfo().username.trim() || "you";
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
    if (!waiter.response.writableEnded) sendJson2(waiter.response, 200, body);
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
      const child = spawn2(command, {
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
      const child = spawn2(command, {
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
    watcher = fs9.watch(root, (_event, name) => {
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
    fs9.stat(commentsPath(root), (error, info) => {
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
    sendJson2(response, 201, { thread: created });
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
    sendJson2(response, 202, { request: queued });
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
        if (reading) return sendJson2(response, 200, await snapshot(viewer));
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
            const open2 = doc.threads.filter((t) => t.status === "open" && t.messages.some((m) => m.author.kind === "user"));
            const sent = open2.filter(canSendToAgent);
            const at = now();
            for (const t of sent) sendToAgent(t, at);
            return { threads: sent, sent: { count: sent.length, open: open2.length } };
          });
          log(`comments: ${result.sent.count} of ${result.sent.open} open thread(s) sent to the agent`);
          sendJson2(response, 200, result);
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
          sendJson2(response, 200, { thread: t });
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
          sendJson2(response, 200, { deleted: id });
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
          sendJson2(response, 200, { thread: t });
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
          sendJson2(response, 200, { thread: t });
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
          sendJson2(response, 200, { thread: t });
          return await afterChange(true);
        }
      } else {
        return sendJson2(response, 404, { code: "NOT_FOUND", message: "no such comments route" });
      }
      request.resume();
      sendJson2(response, 405, { code: "METHOD_NOT_ALLOWED", message: `${method} is not allowed here` });
    } catch (error) {
      if (response.headersSent) throw error;
      if (error instanceof CommentsError) return sendJson2(response, error.status, { code: error.code, message: error.message });
      if (error instanceof TooLarge2) return sendJson2(response, 413, { code: "TOO_LARGE", message: "request body is too large" });
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

var SCRIPT = fileURLToPath(import.meta.url);
var SKILL_DIR = path9.dirname(path9.dirname(SCRIPT));
var EDITOR_DIR = path9.join(path9.dirname(SCRIPT), "editor");
var LOCAL_USER_ID = "local";
var MAX_DOC_BYTES = 256 * 1024;
var MAX_FILE_CHARS = 262144;
var MAX_DATA_BYTES = 15 * 1024 * 1024;
var MAX_BATCH_WRITES = 50;
var MAX_BATCH_BYTES = 1024 * 1024;
var SEGMENT = /^(?!\.\.?$)[A-Za-z0-9_\-.~:@+]{1,200}$/;
var DATASET_ID = /^[A-Za-z0-9_-]{1,64}$/;
var FILE_NAME = /^[A-Za-z0-9_-][A-Za-z0-9_.-]{0,63}$/;
var PARAM_NAME = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/;
var RESERVED = /* @__PURE__ */ new Set(["__proto__", "constructor", "prototype"]);
var BLOB_URL = /^\/?_blob\/([0-9a-f]{32})$/;
var BLOB_ID = /^[0-9a-f]{32}$/;
var CONTENT_TYPES = {
  csv: "text/csv; charset=utf-8",
  tsv: "text/tab-separated-values; charset=utf-8",
  tab: "text/tab-separated-values; charset=utf-8",
  json: "application/json; charset=utf-8",
  txt: "text/plain; charset=utf-8",
  html: "text/html; charset=utf-8",
  js: "text/javascript; charset=utf-8",
  css: "text/css; charset=utf-8",
  svg: "image/svg+xml",
  png: "image/png",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  gif: "image/gif",
  webp: "image/webp",
  woff2: "font/woff2",
  pdf: "application/pdf"
};
var CliError = class extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
  }
};
var isObject2 = (v) => typeof v === "object" && v !== null && !Array.isArray(v);
var bytesOf = (value) => Buffer.byteLength(JSON.stringify(value), "utf8");
var kb = (n) => n >= 1024 * 1024 ? `${(n / 1024 / 1024).toFixed(1)} MB` : `${Math.ceil(n / 1024)} KB`;
var extOf = (name) => path9.extname(name).slice(1).toLowerCase();
var contentTypeOf = (name) => CONTENT_TYPES[extOf(name)] ?? "application/octet-stream";
var quote = (s) => /^[A-Za-z0-9_./:@%+=-]+$/.test(s) ? s : `'${s.replace(/'/g, `'\\''`)}'`;
var cmd = (name, ...args) => ["node", quote(SCRIPT), name, ...args.map(quote)].join(" ");
function print(value, compact = false) {
  process.stdout.write(`${compact ? JSON.stringify(value) : JSON.stringify(value, null, 2)}
`);
}
var SWITCHES = /* @__PURE__ */ new Set(["text", "json", "wait", "all", "resolve", "reopen", "lan", "off", "background"]);
function parseArgs(argv) {
  const positional = [];
  const flags = /* @__PURE__ */ new Map();
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg.startsWith("--")) {
      const eq = arg.indexOf("=");
      if (eq > 0) flags.set(arg.slice(2, eq), arg.slice(eq + 1));
      else if (i + 1 < argv.length && !argv[i + 1].startsWith("--") && !SWITCHES.has(arg.slice(2))) flags.set(arg.slice(2), argv[++i]);
      else flags.set(arg.slice(2), "true");
    } else positional.push(arg);
  }
  return { positional, flags };
}
function writeFileAtomic(file, data) {
  fs10.mkdirSync(path9.dirname(file), { recursive: true });
  const tmp = path9.join(path9.dirname(file), `.tmp-${process.pid}-${Date.now()}-${path9.basename(file)}`);
  fs10.writeFileSync(tmp, data);
  fs10.renameSync(tmp, file);
}
function resolveRoot(given, { create = false } = {}) {
  if (!given) throw new CliError("MISSING_ROOT", "Give the dashboard folder (for example designs/<slug>).");
  const root = path9.resolve(given);
  const stat = fs10.statSync(root, { throwIfNoEntry: false });
  if (stat && !stat.isDirectory()) throw new CliError("NOT_A_FOLDER", `${given} is a file; the dashboard is a folder.`);
  if (!stat) {
    if (!create) throw new CliError("NOT_FOUND", `${root} does not exist. Write the dashboard first: ${cmd("write-db", given, "batch.json")}.`);
    fs10.mkdirSync(root, { recursive: true });
  }
  return root;
}
function splitPath(p, what) {
  const clean = String(p ?? "").replace(/^\/+|\/+$/g, "");
  const segs = clean === "" ? [] : clean.split("/");
  if (segs.length === 0 || segs.length > 16 || !segs.every((s) => SEGMENT.test(s))) {
    throw new CliError("INVALID_PATH", `${JSON.stringify(p)} is not a ${what} path: "/"-separated segments of letters, digits and _ - . ~ : @ +.`);
  }
  if (segs[0] === "data" && segs[1] === "users" && segs[2] === "me") segs[2] = LOCAL_USER_ID;
  return segs;
}
function docSegments(p) {
  const segs = splitPath(p, "document");
  if (segs.length % 2 !== 0) throw new CliError("INVALID_PATH", `${p} names a collection; a document path is collection/doc_id (for example datasets/sales).`);
  return segs;
}
var docFile = (root, segs) => path9.join(root, "db", ...segs.slice(0, -1), `${segs.at(-1)}.json`);
function listDocFiles(root) {
  const out = /* @__PURE__ */ new Map();
  const base = path9.join(root, "db");
  const walk = (dir, rel) => {
    let entries;
    try {
      entries = fs10.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const e of entries) {
      if (e.name.startsWith(".")) continue;
      const full = path9.join(dir, e.name);
      if (e.isDirectory()) walk(full, [...rel, e.name]);
      else if (e.isFile() && e.name.endsWith(".json")) {
        const segs = [...rel, e.name.slice(0, -5)];
        if (segs.length % 2 !== 0 || !segs.every((s) => SEGMENT.test(s))) continue;
        out.set(segs.join("/"), { file: full });
      }
    }
  };
  walk(base, []);
  return out;
}
function readDoc(root, p) {
  const segs = docSegments(p);
  const file = docFile(root, segs);
  let text;
  try {
    text = fs10.readFileSync(file, "utf8");
  } catch {
    return { path: segs.join("/"), exists: false, file };
  }
  try {
    const data = JSON.parse(text);
    if (!isObject2(data)) throw new Error("not an object");
    return { path: segs.join("/"), exists: true, data, file, bytes: Buffer.byteLength(text) };
  } catch (error) {
    throw new CliError("DAMAGED_DOCUMENT", `${file} is not a JSON object (${error.message}). Rewrite it with set.`);
  }
}
function readAllDocs(root) {
  const docs = /* @__PURE__ */ new Map();
  const damaged = [];
  for (const [p, { file }] of listDocFiles(root)) {
    try {
      const data = JSON.parse(fs10.readFileSync(file, "utf8"));
      if (isObject2(data)) docs.set(p, data);
      else damaged.push(p);
    } catch {
      damaged.push(p);
    }
  }
  return { docs, damaged };
}
var isDeleteMarker = (v) => v === null || isObject2(v) && v.__delete__ === true && Object.keys(v).length === 1;
function mergePatch(target, patch) {
  const out = isObject2(target) ? { ...target } : {};
  for (const [k, v] of Object.entries(patch)) {
    if (k === "__proto__") continue;
    if (isDeleteMarker(v)) delete out[k];
    else if (isObject2(v)) out[k] = mergePatch(out[k], v);
    else out[k] = v;
  }
  return out;
}
function hasDeleteMarker(v) {
  if (Array.isArray(v)) return v.some(hasDeleteMarker);
  if (!isObject2(v)) return false;
  if (v.__delete__ === true) return true;
  return Object.values(v).some(hasDeleteMarker);
}
function documentProblem(segs, data) {
  const p = segs.join("/");
  const bytes = bytesOf(data);
  if (bytes > MAX_DOC_BYTES) return `${p} would be ${kb(bytes)}; the store refuses any document over 256 KB.`;
  const [collection, id] = segs;
  if (segs.length === 2 && collection === "datasets" && (!DATASET_ID.test(id) || RESERVED.has(id))) {
    return `datasets/${id}: a dataset id is 1\u201364 of A\u2013Z a\u2013z 0\u20139 _ - (the page ignores any other).`;
  }
  if (segs.length === 2 && collection === "files") {
    if (!FILE_NAME.test(id) || RESERVED.has(id)) return `files/${id}: a file name is letters, digits, _ - . (up to 64, no folders).`;
    if (typeof data.text !== "string") return `files/${id} needs {text}: the whole file as a string.`;
    if (data.text.length > MAX_FILE_CHARS) return `files/${id} is ${data.text.length} characters; a page file over 262,144 is refused. Split it: a <script src> or <link href> naming another files/<name> loads it.`;
  }
  return null;
}
function resolveFileRefs(value, where) {
  if (Array.isArray(value)) return value.map((v, i) => resolveFileRefs(v, `${where}[${i}]`));
  if (!isObject2(value)) return value;
  const keys = Object.keys(value);
  if (keys.length === 1 && keys[0] === "$file") {
    const file = path9.resolve(String(value.$file));
    try {
      return fs10.readFileSync(file, "utf8");
    } catch {
      throw new CliError("FILE_NOT_FOUND", `${where}: {"$file": ${JSON.stringify(value.$file)}} \u2014 no such file (paths are relative to ${process.cwd()}).`);
    }
  }
  return Object.fromEntries(keys.map((k) => [k, resolveFileRefs(value[k], `${where}.${k}`)]));
}
function applyWrites(root, writes, { agent = true } = {}) {
  if (!Array.isArray(writes) || writes.length === 0) throw new CliError("EMPTY_BATCH", "The batch has no writes.");
  if (writes.length > MAX_BATCH_WRITES) throw new CliError("BATCH_TOO_LARGE", `${writes.length} writes; a batch is at most ${MAX_BATCH_WRITES}. Split it.`);
  const planned = [];
  const seen = /* @__PURE__ */ new Set();
  let total = 0;
  writes.forEach((w, i) => {
    const where = `writes[${i}]`;
    if (!isObject2(w)) throw new CliError("INVALID_WRITE", `${where} is not an object.`);
    const op = w.op;
    if (!["set", "update", "delete"].includes(op)) throw new CliError("INVALID_WRITE", `${where}.op is ${JSON.stringify(op)}; use set, update or delete.`);
    const given = w.path ?? (w.collection !== void 0 && w.doc_id !== void 0 ? `${w.collection}/${w.doc_id}` : void 0);
    if (given === void 0) throw new CliError("INVALID_WRITE", `${where} names no document: give "path": "collection/doc_id" (or "collection" and "doc_id").`);
    let segs;
    try {
      segs = docSegments(given);
    } catch (error) {
      throw new CliError(error.code, `${where}: ${error.message}`);
    }
    const p = segs.join("/");
    if (agent && segs[0] === "data" && segs[1] === "users") throw new CliError("READ_ONLY_PATH", `${where}: ${p} is written by the open page; you only read it.`);
    if (seen.has(p)) throw new CliError("DUPLICATE_DOCUMENT", `${where}: ${p} appears twice; address each document once per batch.`);
    seen.add(p);
    let data;
    if (op !== "delete") {
      if (w.data === void 0 === (w.file_path === void 0)) throw new CliError("INVALID_WRITE", `${where} (${op}) needs exactly one of "data" or "file_path".`);
      if (w.file_path !== void 0) {
        try {
          data = JSON.parse(fs10.readFileSync(path9.resolve(String(w.file_path)), "utf8"));
        } catch (error) {
          throw new CliError("INVALID_WRITE", `${where}.file_path ${JSON.stringify(w.file_path)}: ${error.code === "ENOENT" ? "no such file" : `not JSON (${error.message})`}.`);
        }
      } else data = w.data;
      if (!isObject2(data)) throw new CliError("INVALID_WRITE", `${where}.data must be a JSON object (the document's fields).`);
      data = agent ? resolveFileRefs(data, `${where}.data`) : data;
      if (op === "set" && hasDeleteMarker(data)) throw new CliError("INVALID_WRITE", `${where}: {"__delete__": true} works only in an update; set writes the whole document.`);
    }
    const file = docFile(root, segs);
    const existing = fs10.existsSync(file) ? readDoc(root, p) : { exists: false };
    let next = null;
    if (op === "set") next = data;
    else if (op === "update") next = mergePatch(existing.exists ? existing.data : {}, data);
    if (next !== null) {
      const problem = documentProblem(segs, next);
      if (problem) throw new CliError("REFUSED", `${where}: ${problem}`);
      total += bytesOf(next);
    }
    planned.push({ op, path: p, file, next, created: !existing.exists && op !== "delete", existed: existing.exists });
  });
  if (total > MAX_BATCH_BYTES) throw new CliError("BATCH_TOO_LARGE", `The batch writes ${kb(total)}; a batch is at most 1 MB. Split it.`);
  for (const w of planned) {
    if (w.op === "delete") fs10.rmSync(w.file, { force: true });
    else writeFileAtomic(w.file, `${JSON.stringify(w.next, null, 2)}
`);
  }
  return planned.map((w) => ({
    op: w.op,
    path: w.path,
    ...w.next !== null ? { bytes: bytesOf(w.next) } : {},
    ...w.created ? { created: true } : {},
    ...w.op === "delete" && !w.existed ? { note: "there was no such document" } : {}
  }));
}
var blobFile = (root, id) => path9.join(root, "_blobs", id);
function readBlobMeta(root, id) {
  try {
    return JSON.parse(fs10.readFileSync(`${blobFile(root, id)}.json`, "utf8"));
  } catch {
    return null;
  }
}
function putBlob(root, bytes, name, contentType) {
  const id = createHash5("sha256").update(bytes).digest("hex").slice(0, 32);
  const file = blobFile(root, id);
  if (!fs10.existsSync(file)) writeFileAtomic(file, bytes);
  const meta = { name, contentType, bytes: bytes.length, uploadedAt: (/* @__PURE__ */ new Date()).toISOString() };
  writeFileAtomic(`${file}.json`, `${JSON.stringify(meta, null, 2)}
`);
  return { url: `/_blob/${id}`, id, name, bytes: bytes.length, contentType };
}
function upload(root, given, flags) {
  if (!given) throw new CliError("MISSING_FILE", "Give the file to attach (CSV, TSV or JSON).");
  const file = path9.resolve(given);
  const stat = fs10.statSync(file, { throwIfNoEntry: false });
  if (!stat) throw new CliError("FILE_NOT_FOUND", `No such file: ${given}`);
  if (!stat.isFile()) throw new CliError("NOT_A_FILE", `${given} is not a regular file.`);
  if (stat.size > MAX_DATA_BYTES) {
    throw new CliError("FILE_TOO_LARGE", `${given} is ${kb(stat.size)}; a file over 15 MiB is refused. Keep only the columns and rows the dashboard shows, or aggregate first.`);
  }
  const name = flags.get("name") ?? path9.basename(file);
  const type = flags.get("type") ?? contentTypeOf(name);
  const result = putBlob(root, fs10.readFileSync(file), name, type);
  const ext = extOf(name);
  return {
    ...result,
    next: `Write source {kind: "file", url: ${JSON.stringify(result.url)}, name: ${JSON.stringify(name)}${["csv", "tsv", "tab", "json"].includes(ext) ? "" : ', format: "csv" | "tsv" | "json"'}} and updated {at: <ISO time>, by: <who>} into the dataset (set it, or one update of source.url, source.name and updated), then run check.`
  };
}
function referencesTo(root, id) {
  const { docs } = readAllDocs(root);
  const users = [];
  for (const [p, data] of docs) {
    if (p.startsWith("data/")) continue;
    if (JSON.stringify(data).includes(id)) users.push(p);
  }
  return users;
}
function deleteAsset(root, given) {
  const id = BLOB_URL.exec(String(given ?? ""))?.[1] ?? (BLOB_ID.test(String(given ?? "")) ? given : null);
  if (!id) throw new CliError("INVALID_ASSET", "Give the attached file as its url (/_blob/<id>) or id (32 hex).");
  if (!fs10.existsSync(blobFile(root, id))) throw new CliError("NOT_FOUND", `No attached file ${id} in ${root}/_blobs.`);
  const users = referencesTo(root, id);
  if (users.length > 0) throw new CliError("IN_USE", `${id} is still named by ${users.join(", ")}; change or delete those first.`);
  fs10.rmSync(blobFile(root, id), { force: true });
  fs10.rmSync(`${blobFile(root, id)}.json`, { force: true });
  return { deleted: `/_blob/${id}` };
}
function readDb(root, given, flags) {
  if (!given) {
    const files = listDocFiles(root);
    return {
      root,
      documents: [...files.keys()].sort().map((p) => ({ path: p, bytes: fs10.statSync(files.get(p).file).size }))
    };
  }
  const segs = splitPath(given, "collection or document");
  if (segs.length % 2 === 1) {
    const prefix = `${segs.join("/")}/`;
    const docs = [];
    for (const p of [...listDocFiles(root).keys()].sort()) {
      if (!p.startsWith(prefix) || p.slice(prefix.length).includes("/")) continue;
      docs.push({ id: p.slice(prefix.length), data: readDoc(root, p).data });
    }
    return { collection: segs.join("/"), docs };
  }
  const doc = readDoc(root, segs.join("/"));
  if (flags.has("text")) {
    if (!doc.exists) throw new CliError("NOT_FOUND", `No document ${doc.path}.`);
    if (typeof doc.data.text !== "string") throw new CliError("NO_TEXT", `${doc.path} has no text field.`);
    return { raw: doc.data.text };
  }
  return doc.exists ? { path: doc.path, exists: true, data: doc.data } : { path: doc.path, exists: false };
}
function sanitizeText(s) {
  return typeof s === "string" ? s.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, " ") : "";
}
function normalizeColumns(header) {
  const out = [];
  header.forEach((h, i) => {
    let name = sanitizeText(typeof h === "number" || typeof h === "boolean" ? String(h) : h).replace(/\s+/g, " ").trim() || `column_${i + 1}`;
    while (out.includes(name)) name += "_";
    out.push(name);
  });
  return out;
}
function parseDelimited(text, sep) {
  const input = text.charCodeAt(0) === 65279 ? text.slice(1) : text;
  const rows = [];
  let row = [];
  let cell = "";
  let quoted = false;
  for (let i = 0; i < input.length; i++) {
    const c = input[i];
    if (quoted) {
      if (c !== '"') cell += c;
      else if (input[i + 1] === '"') {
        cell += '"';
        i++;
      } else quoted = false;
      continue;
    }
    if (c === '"' && cell === "") quoted = true;
    else if (c === sep) {
      row.push(cell);
      cell = "";
    } else if (c === "\n" || c === "\r") {
      if (c === "\r" && input[i + 1] === "\n") i++;
      row.push(cell);
      rows.push(row);
      row = [];
      cell = "";
    } else cell += c;
  }
  if (cell !== "" || row.length > 0) {
    row.push(cell);
    rows.push(row);
  }
  const kept = rows.filter((r) => !(r.length === 1 && r[0].trim() === ""));
  if (kept.length === 0) return null;
  const columns = normalizeColumns(kept[0]);
  const data = kept.slice(1).map((r) => Object.fromEntries(columns.map((c, i) => [c, sanitizeText((r[i] ?? "").trim()) || null])));
  return { columns, rows: data };
}
function parseJsonTable(text) {
  let value;
  try {
    value = JSON.parse(text);
  } catch {
    return null;
  }
  let rows = value;
  let declared = [];
  if (isObject2(value)) {
    rows = ["rows", "data", "results"].map((k) => Object.hasOwn(value, k) ? value[k] : void 0).find(Array.isArray);
    if (Array.isArray(value.columns)) declared = value.columns.filter((c) => typeof c === "string");
  }
  if (!Array.isArray(rows)) return null;
  const records = rows.filter(isObject2);
  const found = /* @__PURE__ */ new Set();
  for (const r of records) for (const k of Object.keys(r)) found.add(k);
  const columns = [.../* @__PURE__ */ new Set([...declared.filter((c) => found.has(c)), ...found])];
  return { columns, rows: records };
}
function inferFormat(name) {
  const n = String(name ?? "").toLowerCase();
  if (n.endsWith(".tsv") || n.endsWith(".tab")) return "tsv";
  if (n.endsWith(".json")) return "json";
  return "csv";
}
function cellText(v) {
  if (v === null || v === void 0) return "";
  return typeof v === "object" ? JSON.stringify(v) : String(v);
}
var VOID = /* @__PURE__ */ new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"]);
function decodeEntities(s) {
  return s.replace(/&(#x[0-9a-f]+|#\d+|amp|lt|gt|quot|apos|nbsp);/gi, (m, e) => {
    const k = e.toLowerCase();
    if (k === "amp") return "&";
    if (k === "lt") return "<";
    if (k === "gt") return ">";
    if (k === "quot") return '"';
    if (k === "apos") return "'";
    if (k === "nbsp") return "\xA0";
    const code = k.startsWith("#x") ? parseInt(k.slice(2), 16) : parseInt(k.slice(1), 10);
    return Number.isFinite(code) ? String.fromCodePoint(code) : m;
  });
}
function parseAttrs(src) {
  const attrs = {};
  const re = /([^\s"'<>\/=]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g;
  let m;
  while (m = re.exec(src)) attrs[m[1].toLowerCase()] = decodeEntities(m[2] ?? m[3] ?? m[4] ?? "");
  return attrs;
}
function scanHtml(html) {
  const elements = [];
  const scripts = [];
  const styles = [];
  const texts = [];
  const stack = [];
  const lineAt = (i2) => html.slice(0, i2).split("\n").length;
  let i = 0;
  while (i < html.length) {
    const lt = html.indexOf("<", i);
    const textEnd = lt === -1 ? html.length : lt;
    if (textEnd > i) {
      const t = html.slice(i, textEnd);
      if (t.trim()) texts.push({ text: decodeEntities(t), el: stack.at(-1) ?? -1, line: lineAt(i) });
    }
    if (lt === -1) break;
    if (html.startsWith("<!--", lt)) {
      const end = html.indexOf("-->", lt + 4);
      i = end === -1 ? html.length : end + 3;
      continue;
    }
    if (html[lt + 1] === "/") {
      const end = html.indexOf(">", lt);
      const name = html.slice(lt + 2, end === -1 ? html.length : end).trim().toLowerCase();
      for (let k = stack.length - 1; k >= 0; k--) {
        if (elements[stack[k]].tag === name) {
          stack.length = k;
          break;
        }
      }
      i = end === -1 ? html.length : end + 1;
      continue;
    }
    const m = /^<([A-Za-z][A-Za-z0-9:-]*)((?:[^>"']|"[^"]*"|'[^']*')*)>/.exec(html.slice(lt));
    if (!m) {
      i = lt + 1;
      continue;
    }
    const tag = m[1].toLowerCase();
    const rawAttrs = m[2];
    const selfClosing = /\/\s*$/.test(rawAttrs);
    const el = { tag, attrs: parseAttrs(rawAttrs.replace(/\/\s*$/, "")), line: lineAt(lt), parent: stack.at(-1) ?? -1 };
    elements.push(el);
    const index = elements.length - 1;
    i = lt + m[0].length;
    if (tag === "script" || tag === "style") {
      const close = html.toLowerCase().indexOf(`</${tag}`, i);
      const body = html.slice(i, close === -1 ? html.length : close);
      (tag === "script" ? scripts : styles).push({ el: index, text: body, line: el.line });
      const end = close === -1 ? html.length : html.indexOf(">", close);
      i = end === -1 ? html.length : end + 1;
      continue;
    }
    if (!selfClosing && !VOID.has(tag)) stack.push(index);
  }
  return { elements, scripts, styles, texts };
}
var runtimeTokens = null;
function definedRuntimeTokens() {
  if (runtimeTokens) return runtimeTokens;
  runtimeTokens = /* @__PURE__ */ new Set();
  try {
    for (const name of fs10.readdirSync(path9.join(EDITOR_DIR, "assets"))) {
      if (!name.endsWith(".css")) continue;
      const css = fs10.readFileSync(path9.join(EDITOR_DIR, "assets", name), "utf8");
      for (const m of css.matchAll(/(--[A-Za-z0-9_-]+)\s*:/g)) runtimeTokens.add(m[1]);
    }
  } catch {
  }
  return runtimeTokens;
}
var FIGURE = /(?<![\w.#-])(?:[$€£¥]\s?\d[\d,.]*|\d{1,3}(?:,\d{3})+(?:\.\d+)?|\d+(?:\.\d+)?\s?%|\d+\.\d+|\d{3,})(?![\w.-])/;
var YEAR_OR_DATE = /^(?:19|20)\d{2}$|\d{4}-\d{2}/;
var EMPTY_DRAWING = /^[\s–—−\-·.…]*$|^n\/?a$/i;
var CHECK_ABANDONED_MS = 3e4;
var CHECK_OUTPUT_WORDS = {
  not_a_number: "NaN",
  empty_chart: "a blank chart",
  labels_overlap: "labels over each other",
  labels_cut: "labels cut off"
};
var CHECK_DRAWING_WORDS = {
  not_empty: "a drawing stored holding more than a \u2014",
  nested: "a drawing inside another",
  no_function: "a data-draw with no dash.draw function",
  stray_write: "a script writing outside a drawing (there it sets only class, style, hidden, disabled, checked, value or aria-*)"
};
var LOOK_KEYS = /* @__PURE__ */ new Set(["system", "light", "dark", "corners", "sizes", "fonts"]);
var LOOK_SET_KEYS = /* @__PURE__ */ new Set(["ground", "card", "text", "muted", "border", "good", "warning", "bad", "charts"]);
var LOOK_STATUSES = ["good", "warning", "bad"];
var LOOK_SIZES = { caption: [9, 18], body: [11, 22], heading: [12, 32], title: [16, 64] };
var LOOK_MAX_CORNERS = 32;
var LOOK_MAX_CHARTS = 8;
var LOOK_MAX_FONTS = 12;
var LOOK_MAX_STACK = 200;
var LOOK_GROUND_LUMINANCE = { light: 0.82, dark: 0.04 };
var LOOK_DARK_LUMINANCE = 0.18;
var LOOK_BORDER_DISTANCE = 2;
var LOOK_CARD_LUMINANCE = { light: 0.6, dark: 0.08 };
var LOOK_TEXT_FALLBACK = { light: "#000000", dark: "#ffffff" };
var LOOK_TEXT_CONTRAST = 4.5;
var LOOK_MUTED_CONTRAST = 3;
var LOOK_CHART_CONTRAST = 1.5;
var LOOK_CHART_DISTANCE = 7;
var LOOK_LENGTH = /^(\d{1,4}(?:\.\d{1,4})?)(px|rem)$/i;
var LOOK_FONT_NAME = /^[\p{L}\p{N}][\p{L}\p{N} _.-]{0,59}$/u;
var GENERIC_FONT = /^(serif|sans-serif|monospace|cursive|fantasy|system-ui|ui-serif|ui-sans-serif|ui-monospace|ui-rounded|emoji|math|fangsong|-apple-system|blinkmacsystemfont)$/i;
var clampUnit = (x) => Math.max(0, Math.min(1, x));
var hexByte = (x) => Math.round(clampUnit(x) * 255).toString(16).padStart(2, "0");
var encodeSrgb = (x) => x <= 31308e-7 ? 12.92 * x : 1.055 * Math.pow(x, 1 / 2.4) - 0.055;
var decodeSrgb = (x) => x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
var rgbOfHex = (hex) => [parseInt(hex.slice(1, 3), 16) / 255, parseInt(hex.slice(3, 5), 16) / 255, parseInt(hex.slice(5, 7), 16) / 255];
function cssNumber(text, scale) {
  if (text === "none") return 0;
  const m = /^(-?\d*\.?\d+(?:e-?\d+)?)(%?)$/.exec(text);
  if (!m) return void 0;
  return m[2] === "%" ? Number(m[1]) / 100 * scale : Number(m[1]);
}
function cssAngle(text) {
  const m = /^(-?\d*\.?\d+)(deg|rad|grad|turn)?$/.exec(text === "none" ? "0" : text);
  if (!m) return void 0;
  const unit = m[2] === "rad" ? 180 / Math.PI : m[2] === "grad" ? 0.9 : m[2] === "turn" ? 360 : 1;
  return (Number(m[1]) * unit % 360 + 360) % 360;
}
function colorArguments(text, fn) {
  const m = new RegExp(`^${fn}\\((.*)\\)$`).exec(text);
  if (!m) return void 0;
  const parts = m[1].trim().split(/\s*[,/]\s*|\s+/);
  return parts.length === 3 || parts.length === 4 ? parts : void 0;
}
function oklabToRgb(l, a, b) {
  const lms = [(l + 0.3963377774 * a + 0.2158037573 * b) ** 3, (l - 0.1055613458 * a - 0.0638541728 * b) ** 3, (l - 0.0894841775 * a - 1.291485548 * b) ** 3];
  return [
    encodeSrgb(4.0767416621 * lms[0] - 3.3077115913 * lms[1] + 0.2309699292 * lms[2]),
    encodeSrgb(-1.2684380046 * lms[0] + 2.6097574011 * lms[1] - 0.3413193965 * lms[2]),
    encodeSrgb(-0.0041960863 * lms[0] - 0.7034186147 * lms[1] + 1.707614701 * lms[2])
  ];
}
function labToRgb(l, a, b) {
  const fy = (l + 16) / 116;
  const fx = fy + a / 500;
  const fz = fy - b / 200;
  const inverse = (t) => t ** 3 > 0.008856451679035631 ? t ** 3 : (116 * t - 16) / 903.2962962962963;
  const [x, y, z] = [inverse(fx) * 0.96422, l > 8 ? fy ** 3 : l / 903.2962962962963, inverse(fz) * 0.82521];
  const [dx, dy, dz] = [0.9555766 * x - 0.0230393 * y + 0.0631636 * z, -0.0282895 * x + 1.0099416 * y + 0.0210077 * z, 0.0122982 * x - 0.020483 * y + 1.3299098 * z];
  return [encodeSrgb(3.2404542 * dx - 1.5371385 * dy - 0.4985314 * dz), encodeSrgb(-0.969266 * dx + 1.8760108 * dy + 0.041556 * dz), encodeSrgb(0.0556434 * dx - 0.2040259 * dy + 1.0572252 * dz)];
}
function parseLookColor(value) {
  const text = value.trim().toLowerCase();
  if (text.length > 96) return void 0;
  const hex = /^#([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/.exec(text)?.[1];
  if (hex !== void 0) {
    const full = hex.length <= 4 ? [...hex].map((c) => c + c).join("") : hex;
    return `#${full.endsWith("ff") && full.length === 8 ? full.slice(0, 6) : full}`;
  }
  const fn = /^(rgba?|hsla?|oklch|oklab|lab|lch)\(/.exec(text)?.[1];
  const args = fn !== void 0 ? colorArguments(text, fn) : void 0;
  if (fn === void 0 || args === void 0) return void 0;
  const alpha = args[3] !== void 0 ? cssNumber(args[3], 1) : 1;
  let rgb;
  if (fn === "rgb" || fn === "rgba") {
    const [r, g, b] = [cssNumber(args[0], 255), cssNumber(args[1], 255), cssNumber(args[2], 255)];
    if (r !== void 0 && g !== void 0 && b !== void 0) rgb = [r / 255, g / 255, b / 255];
  } else if (fn === "hsl" || fn === "hsla") {
    const [h, s, l] = [cssAngle(args[0]), cssNumber(args[1].replace(/%$/, ""), 100), cssNumber(args[2].replace(/%$/, ""), 100)];
    if (h !== void 0 && s !== void 0 && l !== void 0) {
      const [sat, light] = [clampUnit(s / 100), clampUnit(l / 100)];
      const channel = (n) => {
        const k = (n + h / 30) % 12;
        return light - sat * Math.min(light, 1 - light) * Math.max(-1, Math.min(k - 3, 9 - k, 1));
      };
      rgb = [channel(0), channel(8), channel(4)];
    }
  } else if (fn === "oklab" || fn === "oklch") {
    const l = cssNumber(args[0], 1);
    let a = cssNumber(args[1], 0.4);
    let b = fn === "oklab" ? cssNumber(args[2], 0.4) : void 0;
    if (fn === "oklch") {
      const h = cssAngle(args[2]);
      if (a !== void 0 && h !== void 0) [a, b] = [Math.max(0, a) * Math.cos(h * Math.PI / 180), Math.max(0, a) * Math.sin(h * Math.PI / 180)];
    }
    if (l !== void 0 && a !== void 0 && b !== void 0) rgb = oklabToRgb(clampUnit(l), a, b);
  } else {
    const l = cssNumber(args[0], 100);
    let a = cssNumber(args[1], fn === "lab" ? 125 : 150);
    let b = fn === "lab" ? cssNumber(args[2], 125) : void 0;
    if (fn === "lch") {
      const h = cssAngle(args[2]);
      if (a !== void 0 && h !== void 0) [a, b] = [Math.max(0, a) * Math.cos(h * Math.PI / 180), Math.max(0, a) * Math.sin(h * Math.PI / 180)];
    }
    if (l !== void 0 && a !== void 0 && b !== void 0) rgb = labToRgb(Math.max(0, Math.min(100, l)), a, b);
  }
  if (rgb === void 0 || alpha === void 0 || rgb.some((x) => !Number.isFinite(x))) return void 0;
  const color = `#${hexByte(rgb[0])}${hexByte(rgb[1])}${hexByte(rgb[2])}`;
  return alpha >= 1 ? color : color + hexByte(alpha);
}
var luminance = (hex) => {
  const [r, g, b] = rgbOfHex(hex).map(decodeSrgb);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};
var contrast = (a, b) => {
  const [x, y] = [luminance(a), luminance(b)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
};
var mixHex = (a, b, t) => {
  const [x, y] = [rgbOfHex(a), rgbOfHex(b)];
  return `#${[0, 1, 2].map((i) => Math.round((x[i] * (1 - t) + y[i] * t) * 255).toString(16).padStart(2, "0")).join("")}`;
};
function oklabOfHex(hex) {
  const [r, g, b] = rgbOfHex(hex).map(decodeSrgb);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s, 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s, 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s];
}
var colorDistance = (a, b) => {
  const [x, y] = [oklabOfHex(a), oklabOfHex(b)];
  return 100 * Math.hypot(x[0] - y[0], x[1] - y[1], x[2] - y[2]);
};
var inLuminanceBand = (hex, theme, band) => theme === "light" ? luminance(hex) >= band.light : luminance(hex) <= band.dark;
function readLookColor(value, under) {
  const color = typeof value === "string" ? parseLookColor(value) : void 0;
  if (color === void 0 || color.length === 7) return color;
  if (under === void 0) return void 0;
  const alpha = color.slice(7);
  return alpha === "ff" ? color.slice(0, 7) : mixHex(under, color.slice(0, 7), parseInt(alpha, 16) / 255);
}
function readLookLength(value, min, max) {
  const m = typeof value === "string" ? LOOK_LENGTH.exec(value.trim()) : null;
  const px = typeof value === "number" ? value : m ? Number(m[1]) * (m[2].toLowerCase() === "rem" ? 16 : 1) : NaN;
  if (!Number.isFinite(px)) return void 0;
  const kept = Math.round(Math.min(max, Math.max(min, px)) * 100) / 100;
  return { px: kept, held: kept !== Math.round(px * 100) / 100 };
}
function isLookFontStack(value) {
  if (typeof value !== "string" || value.length > LOOK_MAX_STACK) return false;
  const names = value.split(",").map((n) => n.trim().replace(/^(["'])(.*)\1$/, "$2"));
  return names.length <= LOOK_MAX_FONTS && names.every((n) => GENERIC_FONT.test(n) || LOOK_FONT_NAME.test(n));
}
function checkLookSet(value, theme, issue, warn) {
  if (value === void 0) return false;
  if (!isObject2(value)) {
    issue(`${theme} is an object of colors ({ground, card, text, \u2026}); the ${theme} set is dropped.`);
    return false;
  }
  for (const key of Object.keys(value)) if (!LOOK_SET_KEYS.has(key)) warn(`${theme}.${key} isn\u2019t read (a set has ${[...LOOK_SET_KEYS].join(", ")}).`);
  const form = "a hex, or rgb(), hsl(), oklch(), oklab(), lab() or lch(): no name, var() or gradient";
  const givenGround = readLookColor(value.ground);
  if (givenGround === void 0) {
    issue(value.ground === void 0 ? `${theme}.ground is missing: a set needs its ground, so the ${theme} set is dropped.` : `${theme}.ground ${JSON.stringify(value.ground)} isn\u2019t a color the dashboard reads (${form}; no alpha on a ground): the ${theme} set is dropped.`);
    return false;
  }
  if (luminance(givenGround) < LOOK_DARK_LUMINANCE !== (theme === "dark")) {
    issue(`${theme}.ground ${value.ground} is on the wrong side for a ${theme} set (${theme === "light" ? "#767676 or lighter" : "#757575 or darker"}): the whole ${theme} set is dropped and its readers see the dashboard\u2019s own colors.`);
    return false;
  }
  const pole = theme === "light" ? "#ffffff" : "#000000";
  const ground = inLuminanceBand(givenGround, theme, LOOK_GROUND_LUMINANCE) ? givenGround : [0, 0.2, 0.4, 0.6, 0.8].map((t) => mixHex(givenGround, pole, t)).find((c) => inLuminanceBand(c, theme, LOOK_GROUND_LUMINANCE)) ?? pole;
  if (ground !== givenGround) warn(`${theme}.ground ${value.ground} is too ${theme === "light" ? "dark" : "light"} for the dashboard\u2019s toolbar (a ${theme} ground is as ${theme} as ${theme === "light" ? "#eaeaea or lighter" : "#383838 or darker"}, as that gray): it is taken toward ${theme === "light" ? "white" : "black"} (${ground}) and the other colors are checked against that. Give one of the system\u2019s neutrals that fits.`);
  const unread = (key) => value[key] !== void 0 && readLookColor(value[key], ground) === void 0;
  for (const key of ["card", "text", "muted", "border", ...LOOK_STATUSES]) if (unread(key)) issue(`${theme}.${key} ${JSON.stringify(value[key])} isn\u2019t a color the dashboard reads (${form}); it is dropped.`);
  const cardColor = readLookColor(value.card, ground);
  const card = cardColor !== void 0 && inLuminanceBand(cardColor, theme, LOOK_CARD_LUMINANCE) ? cardColor : ground;
  if (cardColor !== void 0 && card !== cardColor) warn(`${theme}.card ${value.card} is ${theme === "light" ? "darker than #cccccc" : "lighter than #4f4f4f"}; the ground stands in.`);
  const reads = (color2, ratio) => color2 !== void 0 && contrast(color2, ground) >= ratio && contrast(color2, card) >= ratio;
  const color = (key) => readLookColor(value[key], card);
  const given = color("text");
  const text = [given, LOOK_TEXT_FALLBACK[theme]].find((c) => reads(c, LOOK_TEXT_CONTRAST)) ?? LOOK_TEXT_FALLBACK[theme];
  if (given !== void 0 && given !== text) warn(`${theme}.text ${value.text} is under 4.5:1 on the ground or the card; ${theme === "light" ? "black" : "white"} stands in.`);
  const borderGiven = color("border");
  if (borderGiven !== void 0 && ground !== givenGround && Math.min(colorDistance(borderGiven, ground), colorDistance(borderGiven, card)) < LOOK_BORDER_DISTANCE) warn(`${theme}.border ${value.border} is too close to the adjusted ground or the card; one is mixed from text and ground instead.`);
  const muted = color("muted");
  if (muted !== void 0 && !reads(muted, LOOK_MUTED_CONTRAST)) warn(`${theme}.muted ${value.muted} is under 3:1 on the ground or the card; one is mixed from text and ground instead.`);
  for (const key of LOOK_STATUSES) {
    const status = color(key);
    if (status !== void 0 && !reads(status, LOOK_MUTED_CONTRAST)) warn(`${theme}.${key} ${value[key]} is under 3:1 on the ground or the card; the dashboard\u2019s own shows.`);
  }
  if (value.charts !== void 0) {
    if (!Array.isArray(value.charts)) issue(`${theme}.charts is a list of colors, the brand color first; it is dropped.`);
    else {
      if (value.charts.length > LOOK_MAX_CHARTS) warn(`${theme}.charts has ${value.charts.length} colors; only the first ${LOOK_MAX_CHARTS} are read.`);
      const kept = [];
      value.charts.slice(0, LOOK_MAX_CHARTS).forEach((entry, i) => {
        const chart = readLookColor(entry, card);
        const near = chart === void 0 ? -1 : kept.findIndex((k) => k !== void 0 && colorDistance(k, chart) < LOOK_CHART_DISTANCE);
        const ok = reads(chart, LOOK_CHART_CONTRAST) && near === -1;
        kept.push(ok ? chart : void 0);
        if (chart === void 0) issue(`${theme}.charts[${i}] ${JSON.stringify(entry)} isn\u2019t a color the dashboard reads (${form}); that slot keeps the dashboard\u2019s color.`);
        else if (!reads(chart, LOOK_CHART_CONTRAST)) warn(`${theme}.charts[${i}] ${entry} is under 1.5:1 on the ground or the card; that slot keeps the dashboard\u2019s color.`);
        else if (near !== -1) warn(`${theme}.charts[${i}] ${entry} is too close to charts[${near}]; that slot keeps the dashboard\u2019s color.`);
      });
    }
  }
  return true;
}
function checkLook(look, issue, warn) {
  const doc = "dash/look";
  const lookIssue = (message) => issue(doc, message);
  const lookWarn = (message) => warn(doc, message);
  for (const key of Object.keys(look)) if (!LOOK_KEYS.has(key)) lookWarn(`${key} isn\u2019t read (dash/look has ${[...LOOK_KEYS].join(", ")}).`);
  const light = checkLookSet(look.light, "light", lookIssue, lookWarn);
  const dark = checkLookSet(look.dark, "dark", lookIssue, lookWarn);
  if (!light && !dark) lookIssue("has no set the dashboard can use (light or dark, each with its ground), so nothing of the look applies.");
  else if (!light || !dark) lookWarn(`only the ${light ? "light" : "dark"} set is used: readers in the other theme${light ? "" : ", and print,"} see the dashboard\u2019s own colors. Fill both sets.`);
  const length = (where, value, min, max) => {
    if (value === void 0) return;
    const read = readLookLength(value, min, max);
    if (read === void 0) lookIssue(`${where} ${JSON.stringify(value)} isn\u2019t a length the dashboard reads (px or rem, as "12px" or "0.75rem"); it is dropped.`);
    else if (read.held) lookWarn(`${where} ${value} is held to ${read.px}px (${min} to ${max}px).`);
  };
  length("corners", look.corners, 0, LOOK_MAX_CORNERS);
  if (look.sizes !== void 0) {
    if (!isObject2(look.sizes)) lookIssue("sizes is an object: {caption, body, heading, title}.");
    else {
      for (const key of Object.keys(look.sizes)) if (!(key in LOOK_SIZES)) lookWarn(`sizes.${key} isn\u2019t read (sizes are ${Object.keys(LOOK_SIZES).join(", ")}).`);
      for (const [key, [min, max]] of Object.entries(LOOK_SIZES)) length(`sizes.${key}`, look.sizes[key], min, max);
    }
  }
  if (look.fonts !== void 0) {
    if (!isObject2(look.fonts)) lookIssue("fonts is an object: {text, display}.");
    else {
      for (const key of Object.keys(look.fonts)) if (key !== "text" && key !== "display") lookWarn(`fonts.${key} isn\u2019t read (fonts are text and display).`);
      for (const key of ["text", "display"]) {
        const stack = look.fonts[key];
        if (stack !== void 0 && !isLookFontStack(stack)) lookIssue(`fonts.${key} ${JSON.stringify(stack)} is dropped: up to ${LOOK_MAX_FONTS} names, ${LOOK_MAX_STACK} characters in all, each a generic or up to 60 letters, digits, spaces, _ . -.`);
      }
    }
  }
}
function editStampProblem(edit) {
  if (!isObject2(edit)) return 'edit must be {via: "claude", at: <UTC ISO>} (the page ignores anything else).';
  const time = (t) => typeof t === "number" ? Number.isFinite(t) : typeof t === "string" && !Number.isNaN(Date.parse(t));
  const writer = typeof edit.via === "string" && time(edit.at);
  const person = typeof edit.by === "string" && time(edit.byAt);
  return writer || person ? null : 'edit must be {via: "claude", at: <UTC ISO>} (the page ignores anything else).';
}
function check(root) {
  const issues = [];
  const warnings = [];
  const issue = (doc, message) => issues.push({ doc, message });
  const warn = (doc, message) => warnings.push({ doc, message });
  const { docs, damaged } = readAllDocs(root);
  for (const p of damaged) issue(p, "is not a JSON object; the page can\u2019t read it. Rewrite it with set.");
  for (const [p, data] of docs) {
    const bytes = bytesOf(data);
    if (bytes > MAX_DOC_BYTES) issue(p, `is ${kb(bytes)}; the store refuses any document over 256 KB.`);
    const top = p.split("/")[0];
    if (top === "drafts") warn(p, "is a Remix draft: Remix and its drafts need the claude.ai page, so the local editor never shows it. Change the dashboard\u2019s own documents instead.");
    else if (!["dash", "datasets", "files", "data", "versions", "version-docs"].includes(top)) warn(p, "is in a collection the dashboard doesn\u2019t read.");
  }
  const meta = docs.get("dash/meta");
  let title = null;
  if (!meta) issue("dash/meta", "is missing: write {title} first, what the user calls it.");
  else if (typeof meta.title !== "string" || meta.title.trim() === "") issue("dash/meta", "needs a title (what the user calls it).");
  else {
    title = meta.title;
    if (/^dashboard$/i.test(meta.title.trim())) warn("dash/meta", 'title is "Dashboard"; use what the user calls it.');
  }
  const params = docs.get("dash/params");
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      const scalar = (x) => x === null || typeof x === "boolean" || typeof x === "number" && Number.isFinite(x) || typeof x === "string" && !/[\x00-\x1f\x7f\u2028\u2029]/.test(x);
      if (!PARAM_NAME.test(k) || RESERVED.has(k)) issue("dash/params", `${JSON.stringify(k)} is not a parameter name (letters, digits, _; not starting with a digit).`);
      else if (!(scalar(v) || Array.isArray(v) && v.every((x) => x !== null && scalar(x)))) issue("dash/params", `${k}: a default is text on one line, a number, a boolean, null, or a list of them.`);
    }
  }
  const look = docs.get("dash/look");
  if (look) checkLook(look, issue, warn);
  const datasets = /* @__PURE__ */ new Map();
  for (const [p, data] of docs) {
    const m = /^datasets\/([^/]+)$/.exec(p);
    if (!m) continue;
    const id = m[1];
    if (!DATASET_ID.test(id) || RESERVED.has(id)) {
      issue(p, "has an id the page ignores: 1\u201364 of A\u2013Z a\u2013z 0\u20139 _ -.");
      continue;
    }
    const info = { id, kind: null, table: null };
    datasets.set(id, info);
    if (typeof data.title !== "string" || !data.title.trim()) issue(p, "needs a title: a short name that stands alone in a list of sources.");
    else if (data.title.length > 40) warn(p, `title is ${data.title.length} characters; about 30 reads best in Sources (the detail goes in description).`);
    if (data.description !== void 0 && typeof data.description !== "string") issue(p, "description must be text.");
    if (data.edit !== void 0) {
      const problem = editStampProblem(data.edit);
      if (problem) warn(p, problem);
    }
    const source = isObject2(data.source) ? data.source : null;
    if (!source) {
      issue(p, 'needs source: {kind: "file", url, name, format?}.');
      continue;
    }
    const kind = typeof source.kind === "string" ? source.kind : typeof source.tool === "string" ? "query" : typeof source.url === "string" ? "file" : Array.isArray(source.rows) ? "rows" : null;
    info.kind = kind;
    if (kind === "query" || kind === "call") {
      issue(p, `is a live query (kind "${kind}"): there are no connectors in the local page, so it never loads. Run the query yourself, attach its rows as a file (upload), and set the dataset with a file source.`);
      continue;
    }
    if (kind === "rows") {
      issue(p, 'is an old rows dataset (it shows "Didn\u2019t load"): make its rows a JSON file, upload it, then set the document with a file source and updated.');
      continue;
    }
    if (kind !== "file") {
      issue(p, `source.kind ${JSON.stringify(source.kind)} isn\u2019t one the page loads; use "file".`);
      continue;
    }
    if (data.fixed !== void 0 && data.fixed !== true) issue(p, `fixed is true or left out; the page ignores ${JSON.stringify(data.fixed)}.`);
    const leftovers = ["lastAttempt", "pausedBy"].filter((k) => data[k] !== void 0);
    if (leftovers.length) warn(p, `${leftovers.join(" and ")} ${leftovers.length > 1 ? "are" : "is"} left from a paused query, which the page no longer has; remove ${leftovers.length > 1 ? "them" : "it"}.`);
    const blob = BLOB_URL.exec(String(source.url ?? ""))?.[1];
    if (!blob) {
      issue(p, `source.url ${JSON.stringify(source.url)} isn\u2019t an attached file: give the url upload printed (/_blob/<id>), as it came.`);
      continue;
    }
    const file = blobFile(root, blob);
    const stat = fs10.statSync(file, { throwIfNoEntry: false });
    if (!stat) {
      issue(p, `source.url ${source.url}: no such attached file in _blobs/. Upload the file again and use the url it prints.`);
      continue;
    }
    if (stat.size > MAX_DATA_BYTES) {
      issue(p, `its file is ${kb(stat.size)}; a file over 15 MiB is refused.`);
      continue;
    }
    if (typeof source.name !== "string" || !source.name.trim()) issue(p, "source.name: the file\u2019s own name (it sets the format when format is left out).");
    if (source.format !== void 0 && !["csv", "tsv", "json"].includes(source.format)) issue(p, 'source.format is "csv", "tsv" or "json" (only when the name\u2019s extension doesn\u2019t say).');
    const format = ["csv", "tsv", "json"].includes(source.format) ? source.format : inferFormat(source.name);
    const text = fs10.readFileSync(file, "utf8");
    const table = format === "json" ? parseJsonTable(text) : parseDelimited(text, format === "tsv" ? "	" : ",");
    if (!table || table.columns.length === 0) {
      issue(p, `the file has no rows the page can read as ${format}${format === "json" ? " (an array of row objects, or one under rows/data/results)" : ""}.`);
      continue;
    }
    info.table = table;
    info.format = format;
    const updated = isObject2(data.updated) ? data.updated : null;
    if (!updated || typeof updated.at !== "string" || Number.isNaN(Date.parse(updated.at))) issue(p, "needs updated: {at: <ISO time>, by: <who>} on every file write; without it the file shows a warning.");
    else if (typeof updated.by !== "string" || !updated.by.trim()) warn(p, "updated.by is empty: say who wrote it.");
    if (data.staleAfterHours !== void 0 && !(Number.isInteger(data.staleAfterHours) && data.staleAfterHours >= 1)) issue(p, "staleAfterHours is a whole number of hours, 1 or more.");
    if (data.schedule !== void 0 && data.staleAfterHours === void 0) warn(p, "has a schedule but no staleAfterHours (about twice the cadence).");
  }
  const files = /* @__PURE__ */ new Map();
  for (const [p, data] of docs) {
    const m = /^files\/([^/]+)$/.exec(p);
    if (!m) continue;
    if (!FILE_NAME.test(m[1]) || RESERVED.has(m[1])) issue(p, "has a name the page ignores: letters, digits, _ - . (up to 64, no folders).");
    else if (typeof data.text !== "string") issue(p, "needs {text}: the whole file.");
    else if (data.text.length > MAX_FILE_CHARS) issue(p, `is ${data.text.length} characters; a page file over 262,144 is refused.`);
    else files.set(m[1], data.text);
    if (data.edit !== void 0) {
      const problem = editStampProblem(data.edit);
      if (problem) warn(p, problem);
    }
  }
  const index = files.get("index.html");
  if (index === void 0 && !issues.some((x) => x.doc === "files/index.html")) issue("files/index.html", "is missing: the page is files/index.html {text}.");
  const calcs = /* @__PURE__ */ new Map();
  const loaders = /* @__PURE__ */ new Set();
  const reads = /* @__PURE__ */ new Map();
  const marked = /* @__PURE__ */ new Set();
  const js = [];
  const css = [];
  if (index !== void 0) {
    const page = scanHtml(index);
    const doc = "files/index.html";
    const isOneWriterMeta = (el) => el.tag === "meta" && el.attrs.name === "dash-one-writer";
    const oneWriter = page.elements.some(isOneWriterMeta);
    for (const el of page.elements) {
      if (["meta", "base", "form", "iframe", "frame", "frameset", "object", "embed", "portal", "fencedframe"].includes(el.tag) && !isOneWriterMeta(el)) {
        warn(doc, `line ${el.line}: <${el.tag}> is dropped${el.tag === "form" ? " with everything in it" : ""}.`);
      }
      const priority = el.attrs["data-fetchpriority"];
      if (priority !== void 0 && !["high", "low"].includes(priority.trim().toLowerCase())) warn(doc, `line ${el.line}: data-fetchpriority="${priority}" reads as auto; it is high or low.`);
      for (const name of Object.keys(el.attrs)) if (name.startsWith("on") && name.length > 2) warn(doc, `line ${el.line}: ${name}= doesn\u2019t run; add a listener in a script.`);
      if (el.tag === "link" && /(^|\s)stylesheet(\s|$)/i.test(el.attrs.rel ?? "")) {
        const name = (el.attrs.href ?? "").trim().replace(/^\.\//, "");
        if (files.has(name)) css.push({ doc: `files/${name}`, text: files.get(name) });
        else warn(doc, `line ${el.line}: <link href="${el.attrs.href ?? ""}"> names no files/<name>, so it is dropped.`);
      }
    }
    for (const s of page.scripts) {
      const el = page.elements[s.el];
      const type = (el.attrs.type ?? "").trim().toLowerCase();
      if (!["", "text/javascript", "application/javascript"].includes(type)) {
        if (type === "module") warn(doc, `line ${el.line}: <script type="module"> doesn\u2019t run here; no modules.`);
        continue;
      }
      if (el.attrs.src !== void 0) {
        const name = el.attrs.src.trim().replace(/^\.\//, "");
        if (files.has(name)) js.push({ doc: `files/${name}`, text: files.get(name), line: 1 });
        else warn(doc, `line ${el.line}: <script src="${el.attrs.src}"> names no files/<name>, so it is dropped (d3 is already the global d3).`);
      } else js.push({ doc, text: s.text, line: s.line });
    }
    for (const s of page.styles) css.push({ doc, text: s.text });
    for (const s of js) {
      try {
        new vm.Script(s.text, { filename: s.doc, lineOffset: s.line - 1 });
      } catch (error) {
        let ok = false;
        try {
          new Function(s.text);
          ok = true;
        } catch {
        }
        if (!ok) issue(s.doc, `a script doesn\u2019t parse (from line ${s.line}): ${error.message}`);
      }
      for (const m of s.text.matchAll(/dash\.calc\(\s*(['"`])([^'"`]+)\1/g)) {
        const after = s.text.slice(m.index, m.index + 600);
        const inputs = /inputs\s*:\s*\[([^\]]*)\]/.exec(after)?.[1];
        calcs.set(m[2], inputs ? [...inputs.matchAll(/(['"`])([^'"`]+)\1/g)].map((x) => x[2]) : []);
      }
      for (const m of s.text.matchAll(/dash\.loader\(\s*(['"`])([^'"`]+)\1/g)) loaders.add(m[2]);
      for (const m of s.text.matchAll(/dash\.(?:data|refresh)\(\s*(['"`])([^'"`]+)\1/g)) if (!reads.has(m[2])) reads.set(m[2], s.doc);
      for (const m of s.text.matchAll(/var\(--(?:cds|color)-[A-Za-z0-9_-]*\$\{/g)) warn(s.doc, `${m[0]}\u2026: write token names in full; a built name hides a typo.`);
      if (/\.innerHTML\s*=|\.html\(/.test(s.text)) warn(s.doc, "uses innerHTML / .html(): data values are untrusted text; show them with textContent / .text().");
      if (/\b(?:localStorage|sessionStorage|document\.cookie|indexedDB)\b/.test(s.text)) warn(s.doc, "uses storage or cookies, which the dashboard doesn\u2019t have.");
      if (/\bfetch\(|XMLHttpRequest|new WebSocket|import\(/.test(s.text)) warn(s.doc, "reaches the network or imports a module; the page has no network: data comes from dash.data.");
    }
    for (const [id, inputs] of calcs) {
      if (datasets.has(id)) issue("files", `dash.calc("${id}") uses a dataset\u2019s id; a calc takes an id no dataset has.`);
      for (const input of inputs) if (!datasets.has(input) && !calcs.has(input)) issue("files", `dash.calc("${id}") reads input "${input}", which is no dataset or calc.`);
    }
    for (const id of loaders) warn("files", `dash.loader("${id}") is for a live query\u2019s answer; file datasets load as rows without one.`);
    const draws = /* @__PURE__ */ new Set();
    let drawsByName = true;
    for (const s of js) {
      for (const m of s.text.matchAll(/dash\.draw\(\s*(['"`])([^'"`]+)\1/g)) draws.add(m[2]);
      if (/dash\.draw\(\s*[^'"`\s)]/.test(s.text)) drawsByName = false;
    }
    const insideDrawing = (k) => {
      for (k = page.elements[k].parent; k !== -1; k = page.elements[k].parent) if (page.elements[k].attrs["data-draw"] !== void 0) return true;
      return false;
    };
    const drawn = /* @__PURE__ */ new Set();
    page.elements.forEach((el, k) => {
      const name = el.attrs["data-draw"];
      if (name === void 0) return;
      drawn.add(name);
      const where = `line ${el.line} <${el.tag} data-draw="${name}">`;
      if (drawsByName && !draws.has(name)) warn(doc, `${where}: no dash.draw("${name}", fn) fills it, so it keeps its \u2014.`);
      if (!oneWriter) return;
      if (insideDrawing(k)) warn(doc, `${where} sits inside another drawing; the outer one\u2019s function draws all of it.`);
      else {
        const holds = page.elements.some((e) => e.parent === k) || !EMPTY_DRAWING.test(page.texts.filter((t) => t.el === k).map((t) => t.text).join(""));
        if (holds) warn(doc, `${where} is stored holding more than a \u2014; a drawing\u2019s function writes all of it.`);
      }
    });
    for (const name of draws) if (!drawn.has(name)) warn("files", `dash.draw("${name}") has no element marked data-draw="${name}" to draw.`);
    const defined = definedRuntimeTokens();
    const own = /* @__PURE__ */ new Set();
    for (const c of [...css, ...js]) for (const m of c.text.matchAll(/(--[A-Za-z0-9_-]+)\s*:/g)) own.add(m[1]);
    for (const el of page.elements) for (const m of (el.attrs.style ?? "").matchAll(/(--[A-Za-z0-9_-]+)\s*:/g)) own.add(m[1]);
    const unknown = /* @__PURE__ */ new Set();
    const scan = (text) => {
      for (const m of text.matchAll(/var\(\s*(--(?:cds|color|font-anthropic)-[A-Za-z0-9_-]+)/g)) if (defined.size > 0 && !defined.has(m[1]) && !own.has(m[1])) unknown.add(m[1]);
    };
    for (const c of [...css, ...js]) scan(c.text);
    for (const el of page.elements) scan(el.attrs.style ?? "");
    for (const t of unknown) issue("files", `var(${t}) is no token of the dashboard (a typo?): use the names in artifact-type/reference/design-defaults.md.`);
    const sourceOf = (id) => datasets.has(id) ? "dataset" : calcs.has(id) ? "calc" : null;
    const closestTable = (el) => {
      for (let k = el.parent; k !== -1; k = page.elements[k].parent) {
        const t = page.elements[k];
        if (t.tag === "table" && t.attrs["data-source"] !== void 0) return t;
      }
      return null;
    };
    const columnCheck = (where, ds, field) => {
      if (ds?.table && !ds.table.columns.includes(field)) issue(doc, `${where}: data-field="${field}" is no column of ${ds.id} (columns: ${ds.table.columns.join(", ")}).`);
    };
    for (const el of page.elements) {
      const where = `line ${el.line} <${el.tag}${el.attrs.id ? ` id="${el.attrs.id}"` : ""}>`;
      const src = el.attrs["data-source"];
      if (src === void 0) {
        if (el.tag === "th" && el.attrs["data-field"] !== void 0) {
          const table = closestTable(el);
          if (!table) warn(doc, `${where}: a <th data-field> outside a marked <table data-source data-row-key>.`);
          else {
            const ids2 = table.attrs["data-source"].trim().split(/\s+/);
            if (ids2.length === 1) columnCheck(where, datasets.get(ids2[0]), el.attrs["data-field"]);
          }
        } else if (["data-field", "data-where", "data-row"].some((a) => el.attrs[a] !== void 0)) {
          warn(doc, `${where}: data-field/data-where/data-row without data-source on the same element.`);
        }
        continue;
      }
      const ids = src.trim().split(/\s+/).filter(Boolean);
      if (ids.length === 0) {
        issue(doc, `${where}: data-source is empty.`);
        continue;
      }
      for (const id of ids) {
        marked.add(id);
        if (!sourceOf(id)) issue(doc, `${where}: data-source="${id}" names no dataset or dash.calc (datasets: ${[...datasets.keys()].join(", ") || "none"}${calcs.size ? `; calcs: ${[...calcs.keys()].join(", ")}` : ""}).`);
      }
      if (el.tag === "svg") warn(doc, `${where}: mark a chart on its container, not the <svg> itself.`);
      const one = ids.length === 1 ? datasets.get(ids[0]) : null;
      const field = el.attrs["data-field"];
      if (field !== void 0) {
        if (ids.length > 1) issue(doc, `${where}: data-field with several data-source ids; a cell comes from one source.`);
        else if (el.tag === "th" && datasets.has(ids[0])) {
          const table = closestTable(el);
          if (table) warn(doc, `${where}: a computed column\u2019s <th> names a calc, not a dataset.`);
          columnCheck(where, one, field);
        } else columnCheck(where, one, field);
      }
      const whereAttr = el.attrs["data-where"];
      let picked = null;
      if (whereAttr !== void 0 && one?.table) {
        const pairs = whereAttr.split("&").filter(Boolean).map((part) => {
          const eq = part.indexOf("=");
          const dec = (s) => {
            try {
              return decodeURIComponent(s);
            } catch {
              return s;
            }
          };
          return eq < 0 ? [dec(part), ""] : [dec(part.slice(0, eq)), dec(part.slice(eq + 1))];
        });
        const missing = pairs.filter(([c]) => !one.table.columns.includes(c));
        if (missing.length) issue(doc, `${where}: data-where names ${missing.map(([c]) => `"${c}"`).join(", ")}, no column of ${one.id}.`);
        else {
          picked = one.table.rows.filter((r) => pairs.every(([c, v]) => cellText(r[c]) === v));
          if (picked.length === 0) issue(doc, `${where}: data-where="${whereAttr}" matches no row of ${one.id}.`);
        }
      }
      const rowAttr = el.attrs["data-row"];
      if (rowAttr !== void 0 && one?.table) {
        const n = Number(rowAttr);
        if (!Number.isInteger(n) || n < 0 || n >= one.table.rows.length) issue(doc, `${where}: data-row="${rowAttr}" is outside ${one.id}\u2019s ${one.table.rows.length} rows (0-based).`);
      }
      const key = el.attrs["data-row-key"];
      if (key !== void 0) {
        if (el.tag !== "table") warn(doc, `${where}: data-row-key goes on the <table>.`);
        if (ids.length > 1) issue(doc, `${where}: a table is marked with one data-source.`);
        else if (one?.table) {
          const cols = key.split(",").map((c) => c.trim()).filter(Boolean);
          const missing = cols.filter((c) => !one.table.columns.includes(c));
          if (missing.length) issue(doc, `${where}: data-row-key names ${missing.join(", ")}, no column of ${one.id}.`);
          else {
            const keys = one.table.rows.map((r) => cols.map((c) => cellText(r[c])).join("\0"));
            if (new Set(keys).size !== keys.length) issue(doc, `${where}: data-row-key="${key}" doesn\u2019t tell ${one.id}\u2019s rows apart (repeated values); add a column ("a,b").`);
          }
        }
      }
    }
    const insideMarkOrScript = (k) => {
      for (; k !== -1; k = page.elements[k].parent) {
        const e = page.elements[k];
        if (["script", "style", "template"].includes(e.tag) || e.attrs["data-source"] !== void 0 || e.attrs["data-literal"] !== void 0 || e.attrs["data-draw"] !== void 0) return true;
      }
      return false;
    };
    let figures = 0;
    for (const t of page.texts) {
      if (insideMarkOrScript(t.el)) continue;
      const m = FIGURE.exec(t.text);
      if (!m || YEAR_OR_DATE.test(m[0].trim())) continue;
      if (++figures <= 8) warn(doc, `line ${t.line}: "${t.text.trim().slice(0, 60)}" holds a figure (${m[0].trim()}) outside any mark. Every figure comes from a dataset; a fixed reference the user chose is fine.`);
    }
  }
  for (const [id, info] of datasets) {
    if (!marked.has(id) && !reads.has(id)) warn(`datasets/${id}`, 'nothing on the page names it; Sources labels it "Unused".');
    else if (!marked.has(id) && reads.has(id)) issue(`datasets/${id}`, `${reads.get(id)} reads it, but no element is marked data-source="${id}". Mark every dataset your scripts read.`);
    void info;
  }
  for (const [id, doc] of reads) if (!datasets.has(id) && !calcs.has(id)) issue(doc, `dash.data("${id}") reads no dataset or calc.`);
  let pageReport;
  const runs = docs.get(`data/users/${LOCAL_USER_ID}/runs`);
  if (runs) {
    const runsFile = docFile(root, ["data", "users", LOCAL_USER_ID, "runs"]);
    const writtenAt = fs10.statSync(runsFile).mtime;
    const pageChanged = [...docs.keys()].some((p) => /^(files|datasets)\//.test(p) && fs10.statSync(docFile(root, p.split("/"))).mtime > writtenAt);
    const failedSources = Object.entries(isObject2(runs.datasets) ? runs.datasets : {}).filter(([, d]) => isObject2(d) && d.status !== "ok");
    pageReport = {
      writtenAt: writtenAt.toISOString(),
      ...pageChanged ? { stale: "files or datasets changed after the page wrote this; it is rewritten (or removed) when the page next runs them" } : {},
      ...failedSources.length ? { sources: Object.fromEntries(failedSources) } : {},
      ...Array.isArray(runs.marks) && runs.marks.length ? { marks: runs.marks, flagged: runs.flagged } : {},
      ...Array.isArray(runs.errors) && runs.errors.length ? { errors: runs.errors } : {}
    };
  }
  if (pageReport) {
    const report = pageReport.stale ? warn : issue;
    const doc = "data/users/me/runs";
    for (const [id, d] of Object.entries(pageReport.sources ?? {})) report(doc, `the open page says ${id} ${d.status}${d.message ? `: ${String(d.message).slice(0, 160)}` : ""}.`);
    for (const m of pageReport.marks ?? []) {
      const at = `${m.mark} (${[m.source, m.field, m.where ?? (m.row !== void 0 ? `row ${m.row}` : void 0)].filter(Boolean).join(" \xB7 ")})`;
      report(doc, m.reason ? `the open page flagged ${at}: ${String(m.reason).slice(0, 120)}.` : `the open page flagged ${at}: it shows ${JSON.stringify(m.shown)}, the data holds ${m.inData}.`);
    }
    for (const e of pageReport.errors ?? []) report(doc, `the page's code threw (${e.kind}) ${e.name}: ${String(e.message).slice(0, 160)}${e.file ? ` in ${e.file}` : ""}${e.count > 1 ? ` (${e.count} times)` : ""}.`);
  }
  const server = readState(root);
  const running = server && server.kind === "dashboard" && alive(server.pid) ? server.url : null;
  let pageCheck;
  const checkDoc = docs.get(`data/users/${LOCAL_USER_ID}/check`);
  if (checkDoc) {
    const writtenAt = fs10.statSync(docFile(root, ["data", "users", LOCAL_USER_ID, "check"])).mtime;
    const changedSince = (re) => [...docs.keys()].some((p) => re.test(p) && fs10.statSync(docFile(root, p.split("/"))).mtime > writtenAt);
    const stale = changedSince(/^files\//) ? "a page file changed after the page wrote this; the open page checks again when it picks the change up, then rewrites or removes it" : changedSince(/^(datasets|dash)\//) ? "a dataset or dash document changed after the page wrote this; only a page-file write (or opening the page again) checks again" : checkDoc.status === "checking" && Date.now() - writtenAt.getTime() > CHECK_ABANDONED_MS ? "it has said checking for over 30 seconds: the page was closed while it checked; it checks again when it is opened" : void 0;
    pageCheck = {
      status: checkDoc.status,
      ...checkDoc.at !== void 0 ? { at: checkDoc.at } : {},
      writtenAt: writtenAt.toISOString(),
      ...stale ? { stale } : {},
      ...Array.isArray(checkDoc.page) ? { page: checkDoc.page.filter(isObject2) } : {}
    };
    if (checkDoc.status === "failed") {
      const doc = "data/users/me/check";
      const report = stale ? warn : issue;
      for (const e of pageCheck.page ?? []) {
        const times = typeof e.count === "number" && e.count > 1 ? ` (${e.count})` : "";
        if (e.kind === "output") {
          warn(doc, `the open page shows ${CHECK_OUTPUT_WORDS[String(e.what)] ?? String(e.what)}${times}: fix it unless a filter or the data explains it.`);
        } else if (e.kind === "drawing") {
          report(doc, `the open page found ${CHECK_DRAWING_WORDS[String(e.what)] ?? String(e.what)}${times}.`);
        } else {
          const did = e.kind === "stopped" ? "stopped" : e.kind === "rejection" ? "left a promise rejected" : "threw";
          report(doc, `the page's code ${did}: ${e.name} (${e.what})${e.file ? ` in ${e.file}` : ""}${times}; errors in data/users/me/runs hold its words.`);
        }
      }
    }
  } else if (running) {
    pageCheck = {
      status: null,
      note: "No check document. Open in a browser, the page checks itself when it opens and after each page-file write (it takes about 10 seconds), writes what failed here, and removes the document when it passes. None after one more check: it passed, its data came too late, or no browser has it open."
    };
  }
  const problems = issues.length;
  const checking = pageCheck?.status === "checking" && !pageCheck.stale;
  return {
    problems,
    result: {
      root,
      title,
      problems,
      ...issues.length ? { issues } : {},
      ...warnings.length ? { warnings } : {},
      datasets: [...datasets.values()].map((d) => ({
        id: d.id,
        kind: d.kind,
        ...d.table ? { format: d.format, rows: d.table.rows.length, columns: d.table.columns } : {}
      })),
      ...calcs.size ? { calcs: [...calcs.keys()] } : {},
      ...pageReport ? { page: pageReport } : {},
      ...pageCheck ? { pageCheck } : {},
      notChecked: "Nothing is rendered here: whether scripts run, values shown match the data, and the layout are checked by the page itself while a browser has it open (pageCheck after a page write; page for what its readers ran into).",
      next: problems > 0 ? "Fix each issue (doc names the document; files/index.html lines are lines of its text), write it with write-db, and run check again until problems is 0. Warnings are worth a look; they don\u2019t block." : checking ? "No problems so far. The open page is checking itself: run check again in a few seconds." : running ? `No problems. The editor is open at ${running} and shows each change as it is written; where a browser has it open, the page checks itself after each page write: run check one more time in about 10 seconds to read pageCheck (still none then: it passed, or no browser has it open).` : `No problems. Start the editor (it keeps running in the background): ${cmd("open", root)}, then give the person the url it prints and the folder.`
    }
  };
}
var runningServer2 = (root) => runningServer(root, "dashboard", "_api/session");
function serverFlags(flags) {
  const out = [];
  if (flags.has("port")) out.push("--port", flags.get("port"));
  if (flags.has("host")) out.push("--host", flags.get("host"));
  if (flags.has("agent-hook")) out.push("--agent-hook", flags.get("agent-hook"));
  if (flags.has("lan")) out.push("--lan");
  return out;
}
var COMMENTS_ROUTE = "/__comments";
var COMMENT_FILE = "files/index.html";
var ADMIN_READ = /* @__PURE__ */ new Set(["versions", "version-docs", "drafts"]);
var access = (viewer, p) => dbAccess(viewer, p.split("/"), ADMIN_READ);
var ownPath = (p, viewer) => String(p ?? "").replace(/^\/*data\/users\/me(?=\/|$)/, `data/users/${viewer.user.id}`);
var historyTracks = (p) => {
  const top = p.split("/")[0] ?? "";
  return top !== "data" && !ADMIN_READ.has(top) && p !== "dash/settings";
};
function historyLabel(p, docs) {
  if (p === "dash/meta") return { en: "Title and description", zh: "\u6807\u9898\u548C\u8BF4\u660E" };
  if (p === "dash/params") return { en: "Parameters", zh: "\u53C2\u6570" };
  if (p === "dash/look") return { en: "Look", zh: "\u5916\u89C2" };
  const [collection, id] = p.split("/");
  if (collection === "datasets" && id) {
    let title = id;
    try {
      const data = JSON.parse(docs.get(p)?.text ?? "{}");
      if (typeof data.title === "string" && data.title.trim()) title = data.title.trim();
    } catch {
    }
    return { en: `Dataset \xB7 ${title}`, zh: `\u6570\u636E\u96C6 \xB7 ${title}` };
  }
  if (collection === "files" && id) return id === "index.html" ? { en: "Page \xB7 index.html", zh: "\u9875\u9762 \xB7 index.html" } : { en: `Page file \xB7 ${id}`, zh: `\u9875\u9762\u6587\u4EF6 \xB7 ${id}` };
  return null;
}
async function serve(root, flags) {
  if (!["index.html", "local-host.js", "comments.js"].every((name) => fs10.existsSync(path9.join(EDITOR_DIR, name)))) {
    throw new CliError("EDITOR_MISSING", `The editor page is missing (${EDITOR_DIR}); reinstall the skill.`);
  }
  const existing = await runningServer2(root);
  if (existing && existing.pid !== process.pid) {
    print({ url: existing.url, root, pid: existing.pid, reused: true, ...await lanOn2(existing.url, flags) }, true);
    return false;
  }
  const host = flags.get("host") ?? "127.0.0.1";
  const port = flags.has("port") ? Number(flags.get("port")) : DEFAULT_PORTS.dashboard;
  const agentHook = flags.get("agent-hook");
  if (agentHook === "true") throw new CliError("INVALID_AGENT_HOOK", '--agent-hook takes a command: --agent-hook "<command>".');
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new CliError("INVALID_PORT", "--port is a number from 0 to 65535.");
  const background = flags.has("background");
  const state = await serveHub({ kind: "dashboard", script: SCRIPT, root, port, portGiven: flags.has("port"), host, ...agentHook ? { agentHook } : {}, lan: flags.has("lan"), background, site: mountSite });
  print({ url: state.url, root, pid: state.pid, ...state.lan?.length ? { lan: state.lan } : {} }, true);
  return !(state.joined && background);
}
function mountSite(root, { log, network, agentHook, local, agentName }) {
  const dbDir = path9.join(root, "db");
  fs10.mkdirSync(dbDir, { recursive: true });
  const known = /* @__PURE__ */ new Map();
  const clients = /* @__PURE__ */ new Map();
  const presence = new PresenceHub();
  const send = (res, event, data) => res.write(`event: ${event}
data: ${JSON.stringify(data)}

`);
  const snapshot = (viewer) => Object.fromEntries([...known].filter(([p]) => access(viewer, p).read).map(([p, k]) => [p, JSON.parse(k.text)]));
  const page = pagePathOf("dashboard", root);
  const history = new DbHistory({
    root,
    tracked: historyTracks,
    labels: () => (p) => historyLabel(p, known),
    base: page,
    write: (p, data) => void applyWrites(root, [data === null ? { op: "delete", path: p } : { op: "set", path: p, data }], { agent: false }),
    rescan: () => rescan(),
    log
  });
  const docsAt = (n) => history.docsAt(n, new Map([...known].map(([p, k]) => [p, JSON.parse(k.text)])));
  function rescan() {
    const files = listDocFiles(root);
    const changes = [];
    for (const [p, { file }] of files) {
      let stat;
      try {
        stat = fs10.statSync(file);
      } catch {
        continue;
      }
      const prev = known.get(p);
      if (prev && prev.mtimeMs === stat.mtimeMs && prev.size === stat.size) continue;
      let data;
      try {
        data = JSON.parse(fs10.readFileSync(file, "utf8"));
        if (!isObject2(data)) throw new Error("not an object");
      } catch {
        continue;
      }
      const text = JSON.stringify(data);
      known.set(p, { text, mtimeMs: stat.mtimeMs, size: stat.size });
      if (!prev || prev.text !== text) changes.push({ path: p, data });
    }
    for (const p of [...known.keys()]) {
      if (!files.has(p)) {
        known.delete(p);
        changes.push({ path: p, data: null });
      }
    }
    const made = history.record(changes, (p) => files.has(p));
    const byAgent = made.filter((v) => v.by === "agent").flatMap((v) => Object.keys(v.changes));
    if (byAgent.length) presence.agentActive({ where: agentWhere(byAgent.at(-1), byAgent.length, (p) => historyLabel(p, known) ?? { en: p, zh: p }) });
    if (changes.length) {
      for (const [res, viewer] of clients) {
        const seen = changes.filter((c) => access(viewer, c.path).read);
        if (seen.length) send(res, "change", { changes: seen });
      }
      log(`changed: ${changes.map((c) => c.path).join(", ")}`);
    }
  }
  rescan();
  let timer = null;
  const soon = () => {
    clearTimeout(timer);
    timer = setTimeout(rescan, 40);
  };
  let watcher = null;
  try {
    watcher = fs10.watch(dbDir, { recursive: true }, soon);
    watcher.on("error", () => {
    });
  } catch (error) {
    log(`file watching unavailable (${error.message}); polling instead`);
  }
  const poll = setInterval(rescan, watcher ? 2e3 : 700);
  const owner = localOwner();
  const share2 = new ShareHost({
    root,
    page,
    network,
    cookieName: guestCookieName("dashboard", root),
    owner,
    noun: NOUNS.dashboard,
    title: () => dashboardTitle(root),
    log,
    uiFile: path9.join(EDITOR_DIR, "share-ui.js"),
    ...local ? { local } : {},
    ...agentName ? { agentName } : {},
    globals: (viewer, query) => ({ __baoyuHistory: history.bootGlobal(viewer, query) })
  });
  const comments = createCommentsService({
    root,
    log,
    userName: owner.name,
    locate: () => ({ file: COMMENT_FILE }),
    ...agentHook ? { agentHook } : {},
    allowWrite: sameOrigin,
    viewer: (req) => {
      const viewer = share2.viewerOf(req);
      return viewer ? commentsViewer(viewer) : null;
    }
  });
  const json = (res, status, body) => sendJson(res, status, body);
  const serveFile = (req, res, file, type, cache) => {
    fs10.stat(file, (error, stat) => {
      if (error || !stat.isFile()) return json(res, 404, { code: "not_found", message: "Not found" });
      sendFile(req, res, file, { "Content-Type": type, "Content-Length": stat.size, "Cache-Control": cache, "X-Content-Type-Options": "nosniff" });
    });
  };
  const servePage = (res, viewer, at) => {
    let html;
    try {
      html = fs10.readFileSync(path9.join(EDITOR_DIR, "index.html"), "utf8");
    } catch {
      return json(res, 404, { code: "not_found", message: "The editor page is missing." });
    }
    const body = Buffer.from(share2.injectHead(html, viewer, at === void 0 ? "" : `?at=${at}`) ?? html);
    res.writeHead(200, { "Content-Type": "text/html; charset=utf-8", "Content-Length": body.length, "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" }).end(body);
  };
  async function handle(req, res) {
    try {
      const url = new URL(req.url ?? "/", "http://local");
      if (await share2.route(req, res)) return;
      const viewer = share2.viewerOf(req);
      if (!viewer) return share2.denied(req, res);
      const p = url.pathname;
      if (req.method === "POST" && !sameOrigin(req)) return json(res, 403, { code: "not_granted", message: "Cross-origin write refused." });
      const preview = previewOf(url.searchParams);
      const editing = canEditAs(viewer.role) && preview === void 0;
      if (req.method === "GET" && (p === "/" || p === "/index.html")) return servePage(res, viewer, preview);
      if (p === "/__presence" && preview === void 0) return presence.handle(req, res, viewer);
      if (p === "/__history" || p.startsWith("/__history/")) {
        if (await history.host.handle(req, res, p.slice("/__history".length), viewer)) return;
        req.resume();
        return json(res, 404, { code: "not_found", message: "Not found" });
      }
      if (preview !== void 0 && p.startsWith("/_api/")) {
        const docs = docsAt(preview);
        if (!docs) return json(res, 404, { code: "not_found", message: `There is no version ${preview} (it may be too old to keep).` });
        if (req.method === "GET" && p === "/_api/events") {
          res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-store", Connection: "keep-alive" });
          res.write("retry: 1500\n\n");
          send(res, "snapshot", { docs: Object.fromEntries([...docs].filter(([dp]) => access(viewer, dp).read)) });
          const ping = setInterval(() => res.write(": ping\n\n"), 25e3);
          req.on("close", () => clearInterval(ping));
          return;
        }
        if (req.method === "GET" && p === "/_api/doc") {
          let wanted;
          try {
            wanted = docSegments(ownPath(url.searchParams.get("path"), viewer)).join("/");
          } catch (error) {
            return json(res, 400, { code: "invalid_argument", message: error.message });
          }
          const data = docs.get(wanted);
          return json(res, 200, data && access(viewer, wanted).read ? { exists: true, data } : { exists: false });
        }
        if (req.method === "POST") {
          req.resume();
          return json(res, 403, { code: "invalid_argument", message: "This is an earlier version: open the current one to change it." });
        }
      }
      if (req.method === "GET" && (p === "/local-host.js" || p === "/comments.js")) return serveFile(req, res, path9.join(EDITOR_DIR, p.slice(1)), "text/javascript; charset=utf-8", "no-store");
      if (p === COMMENTS_ROUTE || p.startsWith(`${COMMENTS_ROUTE}/`)) {
        const sub = p.slice(COMMENTS_ROUTE.length);
        if (sub === "/events") share2.hold(viewer, res);
        return await comments.handle(req, res, sub);
      }
      if (req.method === "GET" && /^\/assets\/[A-Za-z0-9_.-]+$/.test(p)) return serveFile(req, res, path9.join(EDITOR_DIR, p), contentTypeOf(p), "public, max-age=31536000, immutable");
      const blob = /^\/_blob\/([0-9a-f]{32})$/.exec(p);
      if (req.method === "GET" && blob) {
        const meta = readBlobMeta(root, blob[1]);
        return serveFile(req, res, blobFile(root, blob[1]), meta?.contentType ?? "application/octet-stream", "private, max-age=31536000, immutable");
      }
      if (req.method === "GET" && p === "/_api/session") {
        const session = { ok: true, kind: "dashboard", ...viewer.guest ? {} : { root }, userId: viewer.user.id, userName: viewer.user.name, role: viewer.role, canEdit: editing };
        return json(res, 200, session);
      }
      if (req.method === "GET" && p === "/_api/events") {
        res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-store", Connection: "keep-alive" });
        res.write("retry: 1500\n\n");
        rescan();
        send(res, "snapshot", { docs: snapshot(viewer) });
        clients.set(res, viewer);
        share2.hold(viewer, res);
        const peer = url.searchParams.get("peer") ?? "";
        const leave = presence.join(PEER_ID.test(peer) ? peer : "", viewer, (text) => res.write(text));
        const ping = setInterval(() => res.write(": ping\n\n"), 25e3);
        req.on("close", () => {
          clearInterval(ping);
          clients.delete(res);
          leave();
        });
        return;
      }
      if (req.method === "GET" && p === "/_api/doc") {
        let doc;
        try {
          doc = readDoc(root, ownPath(url.searchParams.get("path"), viewer));
        } catch (error) {
          return json(res, 400, { code: "invalid_argument", message: error.message });
        }
        return json(res, 200, doc.exists && access(viewer, doc.path).read ? { exists: true, data: doc.data } : { exists: false });
      }
      if (req.method === "POST" && p === "/_api/write") {
        let body;
        try {
          body = JSON.parse((await readBody(req, MAX_BATCH_BYTES + 4096)).toString("utf8"));
        } catch (error) {
          const tooLarge = error instanceof TooLarge;
          return json(res, tooLarge ? 413 : 400, { code: "invalid_argument", message: tooLarge ? "The write is over 1 MB." : "The write is not JSON." });
        }
        const given = ownPath(body.path, viewer);
        let segs;
        try {
          segs = docSegments(given);
        } catch (error) {
          return json(res, 400, { code: "invalid_argument", message: error.message });
        }
        if (!editing) return json(res, 403, { code: "invalid_argument", message: `You can view this dashboard but not change it.` });
        if (!access(viewer, segs.join("/")).write) return json(res, 403, { code: "invalid_argument", message: `${segs.join("/")} is another person's own: only they read and write it.` });
        let write = { op: body.op, path: given, ...body.op === "delete" ? {} : { data: body.data } };
        let merge;
        const base = isObject2(body.base) ? body.base : body.base === null && body.op !== "delete" ? {} : null;
        if ((body.op === "set" || body.op === "update" || body.op === "delete") && base && (body.op === "delete" || isObject2(body.data))) {
          let now = null;
          try {
            now = readDoc(root, segs.join("/"));
          } catch {
          }
          if (now && (now.exists || body.base !== null)) {
            const mine = body.op === "delete" ? null : body.op === "update" ? mergePatch(base, body.data) : body.data;
            const merged = mergeWrite(base, mine, now.exists ? now.data : null);
            write = merged.result === null ? { op: "delete", path: given } : { op: "set", path: given, data: merged.result };
            merge = { base, conflicts: merged.conflicts, ...body.chosen === true ? { chosen: true } : {} };
          }
        }
        history.attribute(segs.join("/"), whoOf(viewer), merge);
        try {
          const done = applyWrites(root, [write], { agent: false });
          rescan();
          return json(res, 200, { ok: true, writes: done });
        } catch (error) {
          return json(res, 400, { code: "invalid_argument", message: error.message });
        } finally {
          history.settle(segs.join("/"));
        }
      }
      if (req.method === "POST" && p === "/_api/upload") {
        if (!editing) {
          req.resume();
          return json(res, 403, { code: "not_granted", message: "You can view this dashboard but not add files to it." });
        }
        let bytes;
        try {
          bytes = await readBody(req, MAX_DATA_BYTES);
        } catch (error) {
          const tooLarge = error instanceof TooLarge;
          return json(res, tooLarge ? 413 : 400, { code: tooLarge ? "too_large" : "invalid_argument", message: tooLarge ? "The file is over 15 MiB." : error.message });
        }
        let name = "upload";
        try {
          name = decodeURIComponent(String(req.headers["x-filename"] ?? "upload")).replace(/[\\/]/g, "_").slice(0, 200) || "upload";
        } catch {
        }
        const type = String(req.headers["content-type"] ?? "") || contentTypeOf(name);
        const result = putBlob(root, bytes, name, type);
        log(`uploaded ${name} as ${result.url}${viewer.guest ? ` (${viewer.user.name})` : ""}`);
        return json(res, 200, result);
      }
      return json(res, 404, { code: "not_found", message: "Not found" });
    } catch (error) {
      log(`error: ${error.stack ?? error}`);
      if (!res.headersSent) json(res, 500, { code: "unavailable", message: String(error.message ?? error) });
    }
  }
  const close = () => {
    clearInterval(poll);
    watcher?.close();
    comments.close();
    for (const c of clients.keys()) c.end();
    share2.close();
    presence.close();
  };
  return {
    root,
    page,
    share: share2,
    handle,
    // The page's assets, for a frame (an opaque origin) that asks by a root-relative address.
    common: (pathname) => /^\/(assets|artifact-type)\//.test(pathname),
    hasBlob: (id) => fs10.existsSync(blobFile(root, id)),
    close
  };
}
var fail = (code, message) => {
  throw new CliError(code, message);
};
var shareCall2 = (url, action) => shareCall(url, action, fail);
var lanOn2 = (url, flags) => lanOn(url, flags, fail);
async function open(root, flags) {
  const existing = await runningServer2(root);
  if (existing) {
    print({ url: existing.url, root, pid: existing.pid, reused: true, ...await lanOn2(existing.url, flags), log: path9.join(root, LOG_FILE), stop: cmd("stop", root) }, true);
    return;
  }
  const state = await startBackground(root, flags);
  const empty = !fs10.existsSync(docFile(root, ["files", "index.html"]));
  print({ url: state.url, root, pid: state.pid, ...state.lan?.length ? { lan: state.lan } : {}, log: path9.join(root, LOG_FILE), stop: cmd("stop", root), ...empty ? { note: "files/index.html is not written yet: the page is empty until it is." } : {} }, true);
}
async function startBackground(root, flags) {
  const port = flags.has("port") ? Number(flags.get("port")) : DEFAULT_PORTS.dashboard;
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new CliError("INVALID_PORT", "--port is a number from 0 to 65535.");
  const agentHook = flags.get("agent-hook");
  const opened = await openOnHub({
    kind: "dashboard",
    script: SCRIPT,
    root,
    port,
    portGiven: flags.has("port"),
    host: flags.get("host") ?? "127.0.0.1",
    ...agentHook && agentHook !== "true" ? { agentHook } : {},
    lan: flags.has("lan"),
    command: [SCRIPT, "serve", root, ...serverFlags(flags)],
    running: () => runningServer2(root)
  });
  if ("state" in opened) {
    const view = await shareCall2(opened.state.url, null).catch(() => null);
    return { ...opened.state, ...view?.lan ? { lan: lanUrls(view) } : {} };
  }
  throw new CliError("SERVER_FAILED", `The editor server did not start. Run ${cmd("serve", root)} in the foreground to see why.${opened.tail ? `
${opened.tail}` : ""}`);
}
async function stop(root) {
  const state = readState(root);
  if (!state || !alive(state.pid)) {
    fs10.rmSync(path9.join(root, STATE_FILE), { force: true });
    print({ stopped: false, message: "No editor server is running for this folder." }, true);
    return;
  }
  if (!await leaveHub(state)) {
    print({ stopped: false, pid: state.pid, message: `The server did not exit within 5 s; end it with: kill ${state.pid}` }, true);
    return;
  }
  print({ stopped: true, pid: state.pid }, true);
}
async function share(root, flags) {
  print(
    await shareCommand({
      root,
      flags,
      running: () => runningServer2(root),
      startShared: () => startBackground(root, new Map([...flags, ["lan", "true"]])),
      offCommand: cmd("share", root, "--off"),
      serveCommand: cmd("serve", root, "--lan"),
      fail
    })
  );
}
var WAIT_CHUNK_S = 300;
function commentHints(root) {
  return { reply: cmd("comment-reply", root, "<id>", "<text>"), resolve: cmd("comment-resolve", root, "<id>") };
}
async function listComments(root, all) {
  const doc = await readComments(root);
  const threads = all ? [...doc.threads].sort((x, y) => x.createdAt.localeCompare(y.createdAt)) : openThreads(doc);
  const resolved = doc.threads.filter((t) => t.status === "resolved").length;
  return {
    root,
    file: commentsPath(root),
    threads: agentViews(doc, threads),
    ...all || !resolved ? {} : { resolved: `${resolved} resolved, not shown (--all shows them)` },
    ...threads.length ? commentHints(root) : {}
  };
}
function dashboardTitle(root) {
  let title;
  try {
    const meta = readDoc(root, "dash/meta");
    title = meta.exists ? meta.data?.title : void 0;
  } catch {
  }
  return typeof title === "string" && title.trim() ? title.trim() : path9.basename(root);
}
async function waitComments(root, flags) {
  const timeout = flags.has("timeout") ? Number(flags.get("timeout")) : Infinity;
  if (!(timeout > 0)) throw new CliError("INVALID_TIMEOUT", "--timeout is a number of seconds above 0.");
  const deadline = Date.now() + timeout * 1e3;
  let failures = 0;
  for (; ; ) {
    const state = await runningServer2(root);
    if (!state) throw new CliError("NO_SERVER", `No editor server is running for this folder: start it with ${cmd("open", root)}, then wait again.`);
    const left = Math.min(WAIT_CHUNK_S, (deadline - Date.now()) / 1e3);
    if (left <= 0) return { root, threads: [], timedOut: true, next: cmd("comments", root, "--wait") };
    let body;
    try {
      const response = await fetch(new URL(`__comments/agent/wait?timeout=${Math.max(1, Math.round(left))}`, state.url), {
        // A whole number of ms: AbortSignal.timeout refuses a fraction (and --timeout leaves one in `left`).
        signal: AbortSignal.timeout(Math.ceil((left + 30) * 1e3))
      });
      if (response.status === 404) {
        throw new CliError(
          "SERVER_OUTDATED",
          `The running editor server has no comments (an older version started it). Restart it: ${cmd("stop", root)}, then ${cmd("open", root)}.`
        );
      }
      if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);
      body = await response.json();
      failures = 0;
    } catch (error) {
      if (error instanceof CliError) throw error;
      if (++failures >= 3) throw new CliError("SERVER_GONE", `Lost the editor server: ${error.message}`);
      await new Promise((resolve) => setTimeout(resolve, 1e3));
      continue;
    }
    if (body.closed) continue;
    const threads = body.threads ?? [];
    if (threads.length) {
      return {
        root,
        url: state.url,
        title: dashboardTitle(root),
        file: commentsPath(root),
        threads,
        ...commentHints(root),
        next: cmd("comments", root, "--wait")
      };
    }
  }
}
async function readStdin() {
  if (process.stdin.isTTY) return "";
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  return Buffer.concat(chunks).toString("utf8");
}
async function replyComment(root, id, words, resolve) {
  if (!id) throw new CliError("MISSING_ID", `Give the comment id: ${cmd("comment-reply", root, "<id>", "<text>")}`);
  const given = words.join(" ");
  const text = (given === "" || given === "-" ? await readStdin() : given).replace(/\r\n?/g, "\n").trim();
  if (!text) throw new CliError("EMPTY_REPLY", "The reply is empty: pass it as an argument, or as - with the text on stdin.");
  if (text.length > LIMITS.text) throw new CliError("REPLY_TOO_LONG", `A reply is at most ${LIMITS.text} characters.`);
  const thread = await updateComments(root, (doc) => {
    const found = findThread(doc, id);
    const now = (/* @__PURE__ */ new Date()).toISOString();
    addMessage(found, AGENT_AUTHOR, text, now);
    if (resolve) setStatus(found, "resolved", now);
    return found;
  });
  return { replied: thread.id, status: thread.status, file: commentsPath(root) };
}
async function resolveComment(root, id, reopen) {
  if (!id) throw new CliError("MISSING_ID", `Give the comment id: ${cmd("comment-resolve", root, "<id>")}`);
  const thread = await updateComments(root, (doc) => {
    const found = findThread(doc, id);
    setStatus(found, reopen ? "open" : "resolved", (/* @__PURE__ */ new Date()).toISOString());
    return found;
  });
  return { id: thread.id, status: thread.status, file: commentsPath(root) };
}
var USAGE = `claude-dashboard \u2014 local Artifact tool for the Dashboard type

Usage:
  node scripts/artifact.mjs write-db <root> <batch.json | ->     write documents (set / update / delete), all or nothing
  node scripts/artifact.mjs read-db  <root> [<collection>[/<doc_id>]] [--text]
                                                                  read the store; --text prints a document's text raw
  node scripts/artifact.mjs upload   <root> <file> [--name n]    attach a CSV/TSV/JSON file; prints its /_blob/<id> url
  node scripts/artifact.mjs delete-asset <root> <url|id>         remove an attached file nothing names any more
  node scripts/artifact.mjs check    <root>                      check documents, files and marks; exit 1 on problems
  node scripts/artifact.mjs open     <root> [--port n] [--lan]   start the editor in the background; prints {"url"}
  node scripts/artifact.mjs serve    <root> [--port n] [--lan]   the same server, in the foreground
                                     [--agent-hook "<command>"]  open/serve: run this for each comment sent to the agent
                                                                  (the comment as JSON on stdin); --lan: shared too
  node scripts/artifact.mjs stop     <root>                      stop the background editor
  node scripts/artifact.mjs share    <root>                      share on the local network; prints a link per role
                                     [--off] [--reset R] [--enable R] [--disable R]
                                                                  R: editor, commenter, viewer, a comma list, or all
                                     [--role G=R,...]            give guest G (an id it prints) role R
                                                                  (link options only: shared or not stays as it was)
  node scripts/artifact.mjs comments <root> [--all]              the open comments people left in the page (--all: resolved too)
  node scripts/artifact.mjs comments <root> --wait [--timeout s] wait for a comment sent to the agent from the open page
  node scripts/artifact.mjs comment-reply <root> <id> <text|->   reply as the agent (-: text on stdin) [--resolve]
  node scripts/artifact.mjs comment-resolve <root> <id> [--reopen]  mark a comment resolved (or open again)
  node scripts/artifact.mjs archive  <root> [--out file.zip|dir] the whole folder as one zip (backup, another computer,
                                                                  someone else); beside the folder unless --out says

<root> is the dashboard's folder: documents at <root>/db/<collection>/<doc_id>.json, attached files in <root>/_blobs/,
comments in <root>/comments.json.
A batch is a JSON array (or {"writes": [...]}) of {op, path: "collection/doc_id" (or collection + doc_id), data | file_path};
in data, {"$file": "path"} is replaced by that file's text. update merges fields; null or {"__delete__": true} removes one.
Paths in a batch are relative to the current directory. Skill: ${SKILL_DIR}`;
async function main() {
  const [command, ...rest] = process.argv.slice(2);
  const { positional, flags } = parseArgs(rest);
  switch (command) {
    case "write-db": {
      const root = resolveRoot(positional[0], { create: true });
      const from = positional[1];
      if (!from) throw new CliError("MISSING_BATCH", "Give the batch: a JSON file, or - to read it from stdin.");
      let text;
      try {
        text = from === "-" ? fs10.readFileSync(0, "utf8") : fs10.readFileSync(path9.resolve(from), "utf8");
      } catch {
        throw new CliError("FILE_NOT_FOUND", `No such batch file: ${from}`);
      }
      let batch;
      try {
        batch = JSON.parse(text);
      } catch (error) {
        throw new CliError("INVALID_BATCH", `The batch is not JSON: ${error.message}`);
      }
      const writes = Array.isArray(batch) ? batch : Array.isArray(batch?.writes) ? batch.writes : isObject2(batch) && batch.op ? [batch] : null;
      if (!writes) throw new CliError("INVALID_BATCH", 'A batch is an array of writes, or {"writes": [...]}.');
      const written = applyWrites(root, writes);
      const server = readState(root);
      print({ root, written, next: `Run ${cmd("check", root)}.${server && alive(server.pid) ? " The open editor picks the change up by itself." : ""}` });
      return 0;
    }
    case "read-db": {
      const result = readDb(resolveRoot(positional[0]), positional[1], flags);
      if ("raw" in result) process.stdout.write(result.raw.endsWith("\n") ? result.raw : `${result.raw}
`);
      else print(result);
      return 0;
    }
    case "upload":
      print(upload(resolveRoot(positional[0], { create: true }), positional[1], flags));
      return 0;
    case "delete-asset":
      print(deleteAsset(resolveRoot(positional[0]), positional[1]));
      return 0;
    case "check": {
      const { result, problems } = check(resolveRoot(positional[0]));
      print(result);
      return problems > 0 ? 1 : 0;
    }
    case "open":
      await open(resolveRoot(positional[0]), flags);
      return 0;
    case "serve":
      return await serve(resolveRoot(positional[0]), flags) ? -1 : 0;
    case "stop":
      await stop(resolveRoot(positional[0]));
      return 0;
    case "share":
      await share(resolveRoot(positional[0]), flags);
      return 0;
    case "comments": {
      const root = resolveRoot(positional[0]);
      print(flags.has("wait") ? await waitComments(root, flags) : await listComments(root, flags.has("all")));
      return 0;
    }
    case "comment-reply":
      print(await replyComment(resolveRoot(positional[0]), positional[1], positional.slice(2), flags.has("resolve")), true);
      return 0;
    case "comment-resolve":
      print(await resolveComment(resolveRoot(positional[0]), positional[1], flags.has("reopen")), true);
      return 0;
    case "archive": {
      const made = await archiveCommand(resolveRoot(positional[0]), flags.get("out"));
      print({ ...made, open: `unzip it anywhere, then ${cmd("open", `<there>/${made.folder}`)}` }, true);
      return 0;
    }
    case void 0:
    case "help":
    case "--help":
    case "-h":
      process.stdout.write(`${USAGE}
`);
      return command === void 0 ? 2 : 0;
    default:
      process.stderr.write(`Unknown command: ${command}

${USAGE}
`);
      return 2;
  }
}
if (typeof __BAOBOT_LIB__ === "undefined") main().then(
  (code) => {
    if (code >= 0) process.exitCode = code;
  },
  (error) => {
    process.stderr.write(`${JSON.stringify({ error: error.code ?? "ERROR", message: error.message })}
`);
    process.exitCode = 2;
  }
);
export {
  applyWrites,
  check,
  mountSite,
  readDb,
  upload
};
