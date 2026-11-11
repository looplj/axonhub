#!/usr/bin/env node
// claude-design: local Artifact tool (prebuilt; do not edit).

import fs13 from "node:fs";
import fsp3 from "node:fs/promises";
import path15 from "node:path";
import { fileURLToPath as fileURLToPath2, pathToFileURL } from "node:url";

var BLOB_ID_PATTERN = /^[0-9a-f]{32}$/;
function isProjectPath(p) {
  if (!p.startsWith("project/") || p.includes("\\") || p.includes("\0")) return false;
  return p.split("/").every((part) => part !== "" && part !== "." && part !== "..");
}
function isWriteEntry(entry) {
  return entry !== null && !("delete" in entry);
}

import fs from "node:fs/promises";
import path from "node:path";
function deckReferenceProblems(deck, slideFiles) {
  const problems = [];
  const seen = /* @__PURE__ */ new Set();
  for (const id of deck.order) {
    if (seen.has(id)) problems.push(`order \u91CC ${id} \u91CD\u590D`);
    seen.add(id);
    if (!slideFiles.has(id)) problems.push(`order \u91CC\u7684 ${id} \u6CA1\u6709\u5BF9\u5E94\u7684 slides/${id}.html`);
  }
  for (const [key, section] of Object.entries(deck.sections)) {
    if (!seen.has(section.start)) problems.push(`\u7AE0\u8282 ${key} \u7684 start\u300C${section.start}\u300D\u4E0D\u5728 order \u91CC`);
  }
  if (deck.cover !== void 0 && !seen.has(deck.cover)) problems.push(`cover\u300C${deck.cover}\u300D\u4E0D\u5728 order \u91CC`);
  return problems;
}
var DECK_FILE = "deck.json";
function isDeckMetadata(value) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const deck = value;
  return deck.v === 4 && Array.isArray(deck.order);
}
async function readDeckSummary(dir) {
  const file = path.join(dir, DECK_FILE);
  try {
    const stat = await fs.lstat(file);
    if (!stat.isFile() || stat.size > 4 * 1024 * 1024) return null;
    const parsed = JSON.parse(await fs.readFile(file, "utf8"));
    if (!isDeckMetadata(parsed)) return null;
    const title = typeof parsed.title === "string" ? parsed.title.trim() : "";
    return { title, slideCount: parsed.order.filter((id) => typeof id === "string").length };
  } catch {
    return null;
  }
}

import { createHash, randomUUID } from "node:crypto";
import { lstat, mkdir, readdir, readFile, realpath, rename, rm, writeFile } from "node:fs/promises";
import path2 from "node:path";
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
var ASSETS_DIR = "assets";
var ASSET_INDEX = "index.json";
function contentTypeOf(file) {
  const ext = file.slice(file.lastIndexOf(".") + 1).toLowerCase();
  return MIME[ext] ?? "application/octet-stream";
}
function assetExtensionOf(contentType) {
  return EXT_BY_MIME[contentType.split(";")[0].trim().toLowerCase()] ?? "bin";
}
function sha256(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}
function isDeckFile(rel) {
  const parts = rel.split("/");
  if (parts.some((part) => part === "" || part === "." || part === ".." || part.startsWith("."))) return false;
  if (rel === "deck.json") return true;
  if (parts[0] === "slides") return parts.length === 2 && parts[1].endsWith(".html");
  return parts[0] === "ds" && parts.length > 1;
}
var DECK_LAYOUT = {
  isFile: isDeckFile,
  roots: [
    { rel: "", descend: false },
    { rel: "slides", descend: false },
    { rel: "ds", descend: true }
  ],
  watches: (rel) => rel === "slides" || rel === "ds" || isDeckFile(rel),
  readOnlyMessage: (p) => `${p}: only deck.json, slides/*.html and ds/** can be written`
};
async function collectFiles(root, layout, limit) {
  const rels = [];
  const seen = /* @__PURE__ */ new Set();
  let truncated = false;
  const visit = async (relDir, descend) => {
    const dir = relDir === "" ? root : path2.join(root, ...relDir.split("/"));
    let entries;
    try {
      entries = await readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }
    entries.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
    for (const entry of entries) {
      if (truncated) return;
      if (entry.name.startsWith(".")) continue;
      const rel = relDir === "" ? entry.name : `${relDir}/${entry.name}`;
      if (entry.isDirectory()) {
        if (descend && (layout.descendInto?.(rel) ?? true)) await visit(rel, true);
      } else if (entry.isFile() && layout.isFile(rel) && !seen.has(rel)) {
        if (rels.length >= limit) {
          truncated = true;
          return;
        }
        seen.add(rel);
        rels.push(rel);
      }
    }
  };
  for (const start of layout.roots) {
    if (truncated) break;
    await visit(start.rel, start.descend);
  }
  return { rels, truncated };
}
async function writeFileAtomic(file, bytes) {
  await mkdir(path2.dirname(file), { recursive: true });
  const tmp = path2.join(path2.dirname(file), `.${path2.basename(file)}.${randomUUID()}.tmp`);
  try {
    await writeFile(tmp, bytes);
    await rename(tmp, file);
  } catch (error) {
    await rm(tmp, { force: true }).catch(() => {
    });
    throw error;
  }
}
var DeckStore = class {
  /** deck 目录（含 deck.json 的那一层），或设计稿目录（含 canvas.json 的那一层）。 */
  root;
  layout;
  /** 本进程刚写过的文件（运行时路径）→ 写入时的 sha（删除记为 ""），用来区分「编辑器自己保存」和「外部改动」。 */
  ownWrites = /* @__PURE__ */ new Map();
  queue = Promise.resolve();
  seq = 0;
  onTruncated;
  truncationReported = false;
  constructor(root, layout = DECK_LAYOUT, options = {}) {
    this.root = root;
    this.layout = layout;
    this.onTruncated = options.onTruncated;
  }
  /** 上传素材所在的子目录。 */
  get blobDir() {
    return this.layout.blobDir ?? ASSETS_DIR;
  }
  /** 运行时路径 → 目录里的相对路径；不是这个布局可读写的文件时 null。 */
  fileOf(runtimePath) {
    if (!isProjectPath(runtimePath)) return null;
    const rel = runtimePath.slice("project/".length);
    return this.layout.isFile(rel) ? rel : null;
  }
  /**
   * deck 目录里一个文件的磁盘路径（文件本身可以还不存在）。路径上任何一段是符号链接、或这个名字不是普通文件时 null。
   */
  async diskPath(rel) {
    const realRoot = await realpath(this.root).catch(() => null);
    if (realRoot === null) return null;
    const file = path2.join(realRoot, ...rel.split("/"));
    for (let dir = path2.dirname(file); ; dir = path2.dirname(dir)) {
      const real = await realpath(dir).catch(() => null);
      if (real === null) continue;
      if (real !== dir) return null;
      break;
    }
    const info = await lstat(file).catch(() => null);
    return info && !info.isFile() ? null : file;
  }
  async listing() {
    const limit = this.layout.maxFiles ?? Number.POSITIVE_INFINITY;
    const { rels, truncated } = await collectFiles(this.root, this.layout, limit);
    if (truncated && !this.truncationReported) {
      this.truncationReported = true;
      this.onTruncated?.(limit);
    }
    const files = {};
    for (const rel of rels.sort()) {
      const file = await this.diskPath(rel);
      if (file === null) continue;
      const bytes = await readFile(file).catch(() => null);
      if (bytes === null) continue;
      files[`project/${rel}`] = { sha256: sha256(bytes), size: bytes.length, contentType: contentTypeOf(rel) };
    }
    return { ver: versionOf(files), files };
  }
  async readProjectFile(p) {
    const rel = this.fileOf(p);
    const file = rel === null ? null : await this.diskPath(rel);
    if (file === null) return null;
    try {
      return await readFile(file);
    } catch {
      return null;
    }
  }
  /** 串行执行，保证 ifMatch 检查和写入之间没有交错。 */
  publish(files) {
    const run = this.queue.then(() => this.publishNow(files));
    this.queue = run.catch(() => void 0);
    return run;
  }
  async publishNow(batch) {
    const files = /* @__PURE__ */ new Map();
    let total = 0;
    const maxFileBytes = this.layout.maxFileBytes;
    for (const [p, entry] of Object.entries(batch)) {
      const rel = this.fileOf(p);
      const file = rel === null ? null : await this.diskPath(rel);
      if (file === null) {
        return { status: 400, body: { code: "read_only_path", message: this.layout.readOnlyMessage(p) } };
      }
      files.set(p, file);
      if (!isWriteEntry(entry)) continue;
      const bytes = Buffer.byteLength(entry.base64, "base64");
      if (maxFileBytes !== void 0 && bytes > maxFileBytes) {
        return { status: 413, body: { code: "too_large", message: `${p}: a file can be at most ${maxFileBytes} bytes` } };
      }
      total += bytes;
    }
    const maxBytes = this.layout.maxPublishBytes;
    if (maxBytes !== void 0 && total > maxBytes) {
      return { status: 413, body: { code: "too_large", message: `a publish can write at most ${maxBytes} bytes` } };
    }
    const before = await this.listing();
    const conflicts = [];
    for (const [p, entry] of Object.entries(batch)) {
      const expected = entry?.ifMatch;
      if (expected === void 0) continue;
      const actual = before.files[p]?.sha256 ?? null;
      if (actual !== expected) conflicts.push({ path: p, actual, expected });
    }
    if (conflicts.length > 0) {
      return { status: 409, body: { code: "conflict", live: before.ver, paths: conflicts } };
    }
    const shas = {};
    for (const [p, entry] of Object.entries(batch)) {
      const file = files.get(p);
      if (!isWriteEntry(entry)) {
        this.ownWrites.set(p, "");
        await rm(file, { force: true });
        continue;
      }
      const bytes = Buffer.from(entry.base64, "base64");
      const sha = sha256(bytes);
      this.ownWrites.set(p, sha);
      await writeFileAtomic(file, bytes);
      shas[p] = sha;
    }
    const after = await this.listing();
    this.seq += 1;
    return { status: 200, body: { version: after.ver, seq: this.seq, shas } };
  }
  async assetIndex() {
    try {
      const file = await this.diskPath(`${this.blobDir}/${ASSET_INDEX}`);
      if (file === null) return {};
      const parsed = JSON.parse(await readFile(file, "utf8"));
      return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
    } catch {
      return {};
    }
  }
  /** 素材按内容寻址：id 是 sha256 的前 32 位，同一张图只存一份。 */
  putBlob(bytes, contentType) {
    const run = this.queue.then(() => this.putBlobNow(bytes, contentType));
    this.queue = run.catch(() => void 0);
    return run;
  }
  async putBlobNow(bytes, contentType) {
    const id = sha256(bytes).slice(0, 32);
    const type = contentType.split(";")[0].trim().toLowerCase() || "application/octet-stream";
    const existing = await this.findBlobFile(id);
    if (existing === null) {
      const file = await this.diskPath(`${this.blobDir}/${id}.${assetExtensionOf(type)}`);
      if (file === null) throw new Error(`${this.blobDir} directory is not writable`);
      await writeFileAtomic(file, bytes);
    }
    const index = await this.assetIndex();
    const indexFile = await this.diskPath(`${this.blobDir}/${ASSET_INDEX}`);
    if (index[id] !== type && indexFile !== null) {
      index[id] = type;
      await writeFileAtomic(indexFile, Buffer.from(JSON.stringify(index, null, 2) + "\n"));
    }
    return { id, url: `/_blob/${id}`, sizeBytes: bytes.byteLength };
  }
  /** 素材目录里以 `<id>.` 开头的那个文件。 */
  async findBlobFile(id) {
    try {
      const name = (await readdir(path2.join(this.root, this.blobDir))).find((n) => n.startsWith(`${id}.`) && !n.endsWith(".tmp"));
      if (name === void 0) return null;
      const file = await this.diskPath(`${this.blobDir}/${name}`);
      return file !== null && (await lstat(file).catch(() => null))?.isFile() ? file : null;
    } catch {
      return null;
    }
  }
  async hasBlob(id) {
    return BLOB_ID_PATTERN.test(id) && await this.findBlobFile(id) !== null;
  }
  /** 素材的磁盘文件与 Content-Type（路由按文件流式返回）。 */
  async blobFile(id) {
    if (!BLOB_ID_PATTERN.test(id)) return null;
    const file = await this.findBlobFile(id);
    if (file === null) return null;
    return { file, contentType: (await this.assetIndex())[id] ?? contentTypeOf(file) };
  }
  async getBlob(id) {
    const found = await this.blobFile(id);
    if (found === null) return null;
    try {
      return { bytes: await readFile(found.file), contentType: found.contentType };
    } catch {
      return null;
    }
  }
  /**
   * 这些运行时路径里，哪些的当前内容不是本进程写进去的。`seen` 是调用方已经知道的内容（开始监听时盘上的、报过的）：
   * 与它一致的也不算；查过的路径记成当前内容。
   */
  async externalChanges(paths, seen) {
    const external = [];
    for (const p of paths) {
      const rel = this.fileOf(p);
      if (rel === null) continue;
      let sha = "";
      try {
        const file = await this.diskPath(rel);
        if (file !== null && (await lstat(file)).isFile()) sha = sha256(await readFile(file));
      } catch {
        sha = "";
      }
      if (this.ownWrites.get(p) !== sha && seen?.get(p) !== sha) external.push(p);
      seen?.set(p, sha);
    }
    return external;
  }
};
function versionOf(files) {
  const digest = sha256(
    Object.entries(files).map(([p, f]) => `${p}:${f.sha256}`).join("\n")
  );
  return "local-" + digest.slice(0, 16);
}

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path3 from "node:path";
function problemCount(report) {
  return report.deckProblems.length + report.slides.reduce((n, s) => n + s.problems.length, 0);
}
var IMAGE_REF_ADVICE = /(?: — URLs are not fetched| — a relative path under the deck's folder| is a file reference with no data).*$/;
var IMPORT_ADVICE = " \u2014 \u56FE\u7247\u5148\u7528 slides_import_asset \u5BFC\u5165\uFF0Csrc \u5199\u5B83\u8FD4\u56DE\u7684 /_blob/<id>";
var fmt = (d) => `${d.line}:${d.col} ${d.message.replace(IMAGE_REF_ADVICE, IMPORT_ADVICE)}`;
var CJK = /[\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uac00-\ud7af\uf900-\ufaff]/;
function isCjkWordBreakLint(message) {
  if (!message.includes("break mid-word")) return false;
  const quoted = /"([^"]*)" needs ~/.exec(message)?.[1] ?? "";
  return CJK.test(quoted);
}
var FACE_FAMILY = /^[A-Za-z][A-Za-z0-9 _-]{0,39}$/;
var GOOGLE_FONTS_CSS = /^https:\/\/fonts\.googleapis\.com\/css2\?[A-Za-z0-9&=+,.:;@_-]+$/;
var MAX_FONT_HREF = 1024;
var BLOB_SRC = /^\/_blob\/([0-9a-f]{32})$/;
var DS_FONT_SRC = /^project\/(ds\/[a-z0-9][a-z0-9_-]{0,63}\/(?:[A-Za-z0-9][A-Za-z0-9._-]{0,99}\/){0,8}[A-Za-z0-9][A-Za-z0-9._-]{0,99}\.(?:[Ww][Oo][Ff][Ff]2?|[Tt][Tt][Ff]|[Oo][Tt][Ff]))$/;
var MAX_FACES = 4;
var DS_DIR = "ds";
var DS_FOLDER = /^[a-z0-9][a-z0-9_-]{0,63}$/;
var MAX_DESIGN_SYSTEMS = 4;
var isRecord = (v) => typeof v === "object" && v !== null && !Array.isArray(v);
function fontHead(deck) {
  const parts = [];
  for (const f of Object.values(deck.faces ?? {})) {
    if (typeof f?.href === "string") {
      parts.push(`<link rel="stylesheet" href="${f.href.replace(/&/g, "&amp;").replace(/"/g, "&quot;")}">`);
    } else if (typeof f?.src === "string" && typeof f.family === "string" && FACE_FAMILY.test(f.family) && (BLOB_SRC.test(f.src) || DS_FONT_SRC.test(f.src))) {
      parts.push(`<style>@font-face{font-family:'${f.family}';src:url(${f.src})}</style>`);
    }
  }
  return `<head>${parts.join("")}</head>
`;
}
var facesOf = (r) => Array.isArray(r.content.faces) ? r.content.faces.length : 0;
var headFaceCounts = /* @__PURE__ */ new Map();
function headFaces(rt, head) {
  let n = headFaceCounts.get(head);
  if (n === void 0) {
    n = facesOf(rt.compileSlideHtml(`${head}<section id="probe"></section>`));
    if (headFaceCounts.size > 64) headFaceCounts.clear();
    headFaceCounts.set(head, n);
  }
  return n;
}
function checkSlide(rt, id, html, options) {
  const report = { id, problems: [], notes: [], roundTripDrift: false };
  const r = rt.compileSlideHtml(options.head + html, { lists: options.lists });
  for (const e of r.errors) report.problems.push(`\u9519\u8BEF ${fmt(e)}`);
  if (r.hoisted > 0 || r.title !== void 0 || facesOf(r) > headFaces(rt, options.head)) {
    report.problems.push("\u9519\u8BEF \u9875\u9762\u6587\u4EF6\u91CC\u4E0D\u80FD\u6709 <title>\u3001<link> \u6216 <style>\uFF1A\u6837\u5F0F\u5199\u8FDB style \u5C5E\u6027\uFF0C\u7F51\u7EDC\u5B57\u4F53\u767B\u8BB0\u5728 deck.json \u7684 faces");
  }
  for (const u of r.unsupported) report.problems.push(`\u4E0D\u652F\u6301 ${fmt(u)}`);
  if (r.cellRemovals > 0) report.problems.push(`\u8868\u683C\u6709 ${r.cellRemovals} \u4E2A\u5355\u5143\u683C\u88AB\u5220\uFF08\u884C\u5217\u6570\u4E0D\u9F50\uFF09`);
  if (r.content.order.length !== 1) report.problems.push(`\u5E94\u8BE5\u6070\u597D\u4E00\u4E2A <section>\uFF0C\u5B9E\u9645 ${r.content.order.length} \u4E2A`);
  for (const w of r.warnings) report.notes.push(`\u8B66\u544A ${fmt(w)}`);
  if (options.verbose) {
    for (const l of r.lints) if (!isCjkWordBreakLint(l.message)) report.notes.push(`\u6392\u7248 ${fmt(l)}`);
  }
  if (options.blobExists) {
    for (const ref of html.matchAll(/\/_blob\/([0-9a-f]{32})/g)) {
      if (!options.blobExists(ref[1])) report.problems.push(`\u5F15\u7528\u7684 /_blob/${ref[1]} \u4E0D\u5B58\u5728`);
    }
  }
  const slide = r.content.slides[r.content.order[0] ?? ""];
  if (slide !== void 0) {
    const once = rt.serializeSlideHtml(r.content, slide);
    const again = rt.compileSlideHtml(options.head + once, { lists: options.lists });
    const againSlide = again.content.slides[again.content.order[0] ?? ""];
    report.roundTripDrift = againSlide === void 0 || rt.serializeSlideHtml(again.content, againSlide) !== once;
  }
  return report;
}
function checkDeck(rt, deckDir, options = {}) {
  const raw = JSON.parse(readFileSync(path3.join(deckDir, "deck.json"), "utf8"));
  const normalized = rt.normalizeDeckMetadata(raw);
  const slidesDir = path3.join(deckDir, "slides");
  const slideFiles = new Set(
    existsSync(slidesDir) ? readdirSync(slidesDir).filter((f) => f.endsWith(".html")).map((f) => f.slice(0, -".html".length)) : []
  );
  const deckProblems = deckReferenceProblems(raw, slideFiles);
  const dropped = (raw.order ?? []).length - normalized.order.length;
  if (dropped > 0) deckProblems.push(`order \u91CC\u6709 ${dropped} \u4E2A\u65E0\u6548\u6216\u91CD\u590D\u7684 id \u4F1A\u88AB\u8FD0\u884C\u65F6\u4E22\u6389`);
  const title = normalized.title.trim();
  if (title === "" || title.toLowerCase() === "untitled") deckProblems.push("title \u4E3A\u7A7A\u6216\u662F Untitled\uFF1A\u7ED9\u8FD9\u4EFD\u6F14\u793A\u8D77\u4E2A\u540D\u5B57");
  const head = fontHead(raw);
  const assetsDir = path3.join(deckDir, ASSETS_DIR);
  const assets = existsSync(assetsDir) ? readdirSync(assetsDir) : [];
  const slides = normalized.order.filter((id) => slideFiles.has(id)).map(
    (id) => checkSlide(rt, id, readFileSync(path3.join(slidesDir, `${id}.html`), "utf8"), {
      head,
      lists: raw.lists,
      blobExists: (blobId) => assets.some((name) => name.startsWith(`${blobId}.`)),
      verbose: options.verbose
    })
  );
  return {
    title: normalized.title,
    deckProblems: [...deckProblems.map((p) => `deck.json: ${p}`), ...deckFileProblems(raw, deckDir)],
    orphans: [...slideFiles].filter((id) => !normalized.order.includes(id)),
    slides
  };
}
function deckFileProblems(raw, deckDir) {
  const problems = [];
  const faces = raw.faces;
  const fileExists = (rel) => {
    try {
      return statSync(path3.join(deckDir, rel)).isFile();
    } catch {
      return false;
    }
  };
  const blobExists = (id) => {
    try {
      return readdirSync(path3.join(deckDir, ASSETS_DIR)).some((name) => name.startsWith(`${id}.`));
    } catch {
      return false;
    }
  };
  if (faces !== void 0 && !isRecord(faces)) problems.push('deck.json: faces \u8981\u662F\u5BF9\u8C61\uFF1A{"<\u952E>": {"family": \u2026, "href" \u6216 "src": \u2026}}');
  const entries = isRecord(faces) ? Object.entries(faces) : [];
  if (entries.length > MAX_FACES) problems.push(`deck.json: faces \u6709 ${entries.length} \u9879\uFF0C\u7F16\u8F91\u5668\u53EA\u8BA4\u524D ${MAX_FACES} \u4E2A`);
  const families = /* @__PURE__ */ new Set();
  for (const [key, face] of entries) {
    const where = `deck.json: faces.${key}`;
    if (!isRecord(face) || typeof face.family !== "string") {
      problems.push(`${where} \u8981\u5199\u6210 {"family": <\u9875\u9762 font-family \u91CC\u7684\u540D\u5B57>, "href": <Google Fonts \u94FE\u63A5>} \u6216 {"family": \u2026, "src": <slides_import_asset \u8FD4\u56DE\u7684 /_blob/<id>>}`);
      continue;
    }
    if (!FACE_FAMILY.test(face.family)) problems.push(`${where}.family\u300C${face.family}\u300D\u7F16\u8F91\u5668\u8BA4\u4E0D\u5F97\uFF1A\u4EE5\u82F1\u6587\u5B57\u6BCD\u5F00\u5934\uFF0C\u53EA\u7528\u5B57\u6BCD\u3001\u6570\u5B57\u3001\u7A7A\u683C\u3001- \u4E0E _\uFF0C\u81F3\u591A 40 \u4E2A\u5B57\u7B26`);
    else if (families.has(face.family.toLowerCase())) problems.push(`${where}.family\u300C${face.family}\u300D\u91CD\u590D\u4E86\uFF0C\u7F16\u8F91\u5668\u53EA\u8BA4\u7B2C\u4E00\u4E2A`);
    families.add(face.family.toLowerCase());
    const { href, src } = face;
    if (href !== void 0 && src !== void 0) problems.push(`${where} \u53EA\u5199 href \u6216 src \u4E2D\u7684\u4E00\u4E2A`);
    else if (href !== void 0) {
      if (typeof href !== "string" || !GOOGLE_FONTS_CSS.test(href)) problems.push(`${where}.href \u8981\u662F https://fonts.googleapis.com/css2?\u2026 \u7684\u94FE\u63A5`);
      else if (href.length > MAX_FONT_HREF) problems.push(`${where}.href \u6709 ${href.length} \u4E2A\u5B57\u7B26\uFF0C\u7F16\u8F91\u5668\u53EA\u8BA4 ${MAX_FONT_HREF} \u4E2A\u4EE5\u5185\uFF1A\u5C11\u5217\u51E0\u4E2A\u5B57\u91CD`);
    } else if (src !== void 0) {
      const blob = typeof src === "string" ? BLOB_SRC.exec(src) : null;
      const dsFile = typeof src === "string" ? DS_FONT_SRC.exec(src) : null;
      if (blob) {
        if (!blobExists(blob[1])) problems.push(`${where}.src \u5F15\u7528\u7684 /_blob/${blob[1]} \u4E0D\u5B58\u5728\uFF1A\u7528 slides_import_asset \u5BFC\u5165\u5B57\u4F53\u6587\u4EF6\uFF0C\u5199\u5B83\u8FD4\u56DE\u7684 src`);
      } else if (dsFile) {
        if (!fileExists(dsFile[1])) problems.push(`${where}.src \u6307\u5411\u7684 ${dsFile[1]} \u4E0D\u5728 deck \u91CC`);
      } else {
        problems.push(`${where}.src \u8981\u662F slides_import_asset \u8FD4\u56DE\u7684 /_blob/<id>\uFF08\u5B57\u4F53\u6587\u4EF6 woff2\u3001woff\u3001ttf\u3001otf \u5148\u5BFC\u5165\uFF09`);
      }
    } else {
      problems.push(`${where} \u8981\u6709 href\uFF08Google Fonts \u94FE\u63A5\uFF09\u6216 src\uFF08slides_import_asset \u8FD4\u56DE\u7684 /_blob/<id>\uFF09`);
    }
  }
  problems.push(...designSystemProblems(raw.designSystems, deckDir));
  return problems;
}
function designSystemProblems(value, deckDir) {
  const problems = [];
  const folders = [];
  let listing = [];
  try {
    listing = readdirSync(path3.join(deckDir, DS_DIR), { withFileTypes: true }).sort((x, y) => x.name < y.name ? -1 : x.name > y.name ? 1 : 0);
  } catch {
  }
  for (const entry of listing) {
    if (entry.name.startsWith(".")) continue;
    if (!entry.isDirectory()) problems.push(`${DS_DIR}/${entry.name}: ${DS_DIR}/ \u4E0B\u53EA\u653E\u76EE\u5F55\uFF0C\u4E00\u4E2A\u8BBE\u8BA1\u7CFB\u7EDF\u4E00\u4E2A ${DS_DIR}/<folder>/`);
    else if (!DS_FOLDER.test(entry.name)) problems.push(`${DS_DIR}/${entry.name}/: \u8BBE\u8BA1\u7CFB\u7EDF\u7684\u76EE\u5F55\u540D\u53EA\u7528\u5C0F\u5199\u5B57\u6BCD\u3001\u6570\u5B57\u3001- \u4E0E _\uFF0C\u4EE5\u5B57\u6BCD\u6216\u6570\u5B57\u5F00\u5934\uFF0C\u81F3\u591A 64 \u4E2A\u5B57\u7B26`);
    else folders.push(entry.name);
  }
  const records = Array.isArray(value) ? value : isRecord(value) ? Object.values(value) : value === void 0 ? [] : null;
  if (records === null) problems.push('deck.json: designSystems \u8981\u662F\u6570\u7EC4\uFF1A[{"title","namespace","artifact","version","copiedAt"}]');
  const recorded = /* @__PURE__ */ new Set();
  if (records && records.length > MAX_DESIGN_SYSTEMS) problems.push(`deck.json: designSystems \u6709 ${records.length} \u9879\uFF0C\u4E00\u4EFD deck \u81F3\u591A ${MAX_DESIGN_SYSTEMS} \u4E2A`);
  (records ?? []).forEach((record, i) => {
    const where = `deck.json: designSystems[${i}]`;
    if (!isRecord(record) || typeof record.title !== "string" || typeof record.namespace !== "string" || !DS_FOLDER.test(record.namespace) || typeof record.artifact !== "string" || !(record.version === void 0 || record.version === null || typeof record.version === "string") || !(record.copiedAt === void 0 || typeof record.copiedAt === "string")) {
      problems.push(`${where} \u8981\u5199\u6210 {"title": <\u540D\u5B57>, "namespace": <ds/ \u4E0B\u7684\u76EE\u5F55\u540D>, "artifact": <\u4ECE\u54EA\u91CC\u62F7\u6765>, "version": <\u5B57\u7B26\u4E32\u6216 null>, "copiedAt": <RFC 3339 \u65F6\u95F4>}`);
      return;
    }
    if (recorded.has(record.namespace)) problems.push(`${where}: namespace\u300C${record.namespace}\u300D\u51FA\u73B0\u4E86\u4E0D\u6B62\u4E00\u6B21`);
    recorded.add(record.namespace);
  });
  for (const folder of folders) {
    if (!recorded.has(folder)) problems.push(`${DS_DIR}/${folder}/ \u5728 designSystems \u91CC\u6CA1\u6709\u8BB0\u5F55\uFF1A\u7F16\u8F91\u5668\u8981\u4E24\u8005\u90FD\u5728\u624D\u63D0\u4F9B\u8FD9\u4E2A\u8BBE\u8BA1\u7CFB\u7EDF`);
  }
  return problems;
}

var MAX_PUBLISH_BYTES = 64 * 1024 * 1024;
var MAX_ASSET_BYTES = 256 * 1024 * 1024;

import fs2 from "node:fs";
import { createRequire } from "node:module";
import path4 from "node:path";
import { fileURLToPath } from "node:url";
var COMPILER_FILE = path4.join("node", "compiler.cjs");
function resolveSlidesEditorDist(env = process.env) {
  const given = env.BAOCUT_SLIDES_EDITOR_DIST;
  if (given) return fs2.existsSync(path4.join(given, "index.html")) ? given : null;
  const resources = process.resourcesPath;
  if (resources) {
    const bundled = path4.join(resources, "slides-editor");
    if (fs2.existsSync(path4.join(bundled, "index.html"))) return bundled;
  }
  let dir = path4.dirname(fileURLToPath(import.meta.url));
  for (; ; ) {
    const built = path4.join(dir, "apps", "slides-editor", "dist");
    if (fs2.existsSync(path4.join(built, "index.html"))) return built;
    const parent = path4.dirname(dir);
    if (parent === dir) return null;
    dir = parent;
  }
}
var require2 = createRequire(import.meta.url);
var loaded = /* @__PURE__ */ new Map();
function compilerAvailable(dist = resolveSlidesEditorDist()) {
  return dist !== null && fs2.existsSync(path4.join(dist, COMPILER_FILE));
}
function loadSlidesCompiler(dist = resolveSlidesEditorDist()) {
  if (!compilerAvailable(dist)) throw new Error("slides compiler is not built");
  const file = path4.join(dist, COMPILER_FILE);
  let runtime = loaded.get(file);
  if (!runtime) {
    runtime = require2(file).runtime;
    loaded.set(file, runtime);
  }
  return runtime;
}

import fs3 from "node:fs/promises";
import path5 from "node:path";
var CANVAS_FILE = "canvas.json";
var BOARD_SUFFIX = ".dc.html";
var DESIGN_SYSTEMS_DIR = "ds";
var CANVAS_LIMITS = {
  files: 512,
  totalBytes: 256 * 1024 * 1024,
  publishBytes: 16 * 1024 * 1024
};
var SEGMENT = /^[A-Za-z0-9_][A-Za-z0-9_.-]*$/;
function isCanvasSegment(segment) {
  return SEGMENT.test(segment);
}
function isCanvasFile(rel) {
  const parts = rel.split("/");
  if (parts[0] === ASSETS_DIR) return false;
  return parts.every(isCanvasSegment);
}
function isBoardPath(rel) {
  return rel.endsWith(BOARD_SUFFIX) && rel.split("/")[0] !== DESIGN_SYSTEMS_DIR;
}
var CANVAS_LAYOUT = {
  isFile: isCanvasFile,
  roots: [{ rel: "", descend: true }],
  descendInto: (rel) => isCanvasFile(rel),
  watches: (rel) => isCanvasFile(rel),
  maxFiles: CANVAS_LIMITS.files,
  maxPublishBytes: CANVAS_LIMITS.publishBytes,
  readOnlyMessage: (p) => `${p}: files under assets/, hidden paths and path segments outside [A-Za-z0-9_][A-Za-z0-9_.-]* can't be written`
};
function isCanvasMetadata(value) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const canvas = value;
  return canvas.v === 3 && typeof canvas.boards === "object" && canvas.boards !== null && !Array.isArray(canvas.boards);
}
var MAX_CANVAS_JSON_BYTES = 4 * 1024 * 1024;
async function readCanvasJson(dir) {
  const file = path5.join(dir, CANVAS_FILE);
  try {
    const stat = await fs3.lstat(file);
    if (!stat.isFile() || stat.size > MAX_CANVAS_JSON_BYTES) return null;
    return JSON.parse(await fs3.readFile(file, "utf8"));
  } catch {
    return null;
  }
}
function canvasTitle(canvas, dir) {
  const raw = canvas.title;
  const title = typeof raw === "string" ? raw.trim() : "";
  return title || path5.basename(dir);
}
async function readCanvasInfo(dir) {
  const parsed = await readCanvasJson(dir);
  if (!isCanvasMetadata(parsed)) return null;
  return { title: canvasTitle(parsed, dir), boardCount: Object.keys(parsed.boards).length };
}

import fs4 from "node:fs/promises";
import path6 from "node:path";
var BOARD_SIZE = { min: 40, max: 8e3 };
var MAX_PAGES = 40;
var MAX_NOTES = 999;
var MAX_GUIDES = 6;
var ID_PATTERN = /^[A-Za-z0-9_-]{1,40}$/;
var MAX_BOARD_BYTES = 8 * 1024 * 1024;
var MAX_SCAN_ENTRIES = 2e4;
var SUPPORT_LINE = '<script src="./support.js"></script>';
var TEXT_EXTENSIONS = /* @__PURE__ */ new Set([".html", ".css", ".js", ".mjs", ".json", ".svg", ".md", ".txt"]);
async function scanTree(dir, issues) {
  const tree = { files: /* @__PURE__ */ new Map(), assets: /* @__PURE__ */ new Set(), totalBytes: 0, truncated: false };
  let seen = 0;
  const visit = async (rel) => {
    const full = rel === "" ? dir : path6.join(dir, ...rel.split("/"));
    const entries = await fs4.readdir(full, { withFileTypes: true }).catch(() => []);
    entries.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
    for (const entry of entries) {
      if (++seen > MAX_SCAN_ENTRIES) {
        tree.truncated = true;
        return;
      }
      if (entry.name.startsWith(".")) continue;
      const child = rel === "" ? entry.name : `${rel}/${entry.name}`;
      if (rel === "" && entry.name === ASSETS_DIR && entry.isDirectory()) {
        await scanAssets(path6.join(dir, ASSETS_DIR), tree);
        continue;
      }
      if (entry.isSymbolicLink()) {
        issues.push({ file: child, message: "is a symbolic link; the editor ignores symbolic links. Replace it with a regular file or folder" });
        continue;
      }
      if (!isCanvasSegment(entry.name)) {
        issues.push({
          file: child,
          message: `"${entry.name}" is not a valid path segment: start with a letter, digit or _, then use only letters, digits, _, . and - (no spaces). The editor can't see it`
        });
        continue;
      }
      if (entry.isDirectory()) await visit(child);
      else if (entry.isFile()) {
        const stat = await fs4.lstat(path6.join(full, entry.name)).catch(() => null);
        const size = stat?.size ?? 0;
        tree.files.set(child, size);
        tree.totalBytes += size;
      }
    }
  };
  await visit("");
  return tree;
}
async function scanAssets(dir, tree) {
  const entries = await fs4.readdir(dir, { withFileTypes: true }).catch(() => []);
  for (const entry of entries) {
    if (!entry.isFile() || entry.name.startsWith(".")) continue;
    const id = entry.name.split(".")[0];
    if (BLOB_ID_PATTERN.test(id)) tree.assets.add(id);
    const stat = await fs4.lstat(path6.join(dir, entry.name)).catch(() => null);
    tree.totalBytes += stat?.size ?? 0;
  }
}
var isRecord2 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
var isFiniteNumber = (value) => typeof value === "number" && Number.isFinite(value);
function decodeEntities(text) {
  return text.replace(/&(#x[0-9a-f]+|#\d+|amp|apos|quot|lt|gt);/gi, (whole, name) => {
    const lower = name.toLowerCase();
    if (lower.startsWith("#x")) return String.fromCodePoint(Number.parseInt(lower.slice(2), 16));
    if (lower.startsWith("#")) return String.fromCodePoint(Number.parseInt(lower.slice(1), 10));
    return { amp: "&", apos: "'", quot: '"', lt: "<", gt: ">" }[lower] ?? whole;
  });
}
var isRemote = (url) => /^(?:[a-z][a-z0-9+.-]*:)?\/\//i.test(url.trim()) && !/^data:/i.test(url.trim());
var isGoogleFonts = (url) => /^(?:https?:)?\/\/fonts\.googleapis\.com\//i.test(url.trim());
function attr(attrs, name) {
  const match = new RegExp(`\\b${name}\\s*=\\s*(?:'([^']*)'|"([^"]*)"|([^\\s>]+))`, "i").exec(attrs);
  if (!match) return null;
  if (match[1] !== void 0) return { value: match[1], quote: "'" };
  if (match[2] !== void 0) return { value: match[2], quote: '"' };
  return { value: match[3], quote: "" };
}
function checkBlobRefs(file, text, assets, issues) {
  const reported = /* @__PURE__ */ new Set();
  for (const match of text.matchAll(/\/_blob\/([^\s"'()<>?#]*)/g)) {
    const id = match[1];
    if (reported.has(id)) continue;
    reported.add(id);
    if (!BLOB_ID_PATTERN.test(id)) {
      issues.push({ file, message: `/_blob/${id} is not a valid asset reference: use the /_blob/<id> that design_import_asset returned (32 lowercase hex digits)` });
    } else if (!assets.has(id)) {
      issues.push({ file, message: `/_blob/${id} is not in assets/: import the file with design_import_asset and use the src it returns` });
    }
  }
}
function checkCssImports(file, css, issues) {
  for (const match of css.matchAll(/@import\s+(?:url\(\s*)?["']?([^"')\s;]+)/gi)) {
    const url = match[1];
    if (isRemote(url) && !isGoogleFonts(url)) {
      issues.push({ file, message: `@import ${url} loads a stylesheet from the network; only Google Fonts (fonts.googleapis.com) is allowed. Keep styles inline or import the file with design_import_asset` });
    }
  }
}
var ATTRS = String.raw`((?:[^>"']|"[^"]*"|'[^']*')*)`;
var TAG = new RegExp(String.raw`<(\/?)([A-Za-z][A-Za-z0-9:-]*)` + ATTRS + ">", "g");
var VOID_ELEMENTS = /* @__PURE__ */ new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"]);
var RAW_TEXT = /* @__PURE__ */ new Set(["script", "style", "textarea", "title"]);
var PROP_EDITORS = /* @__PURE__ */ new Set(["text", "color", "int", "float", "range", "boolean", "enum"]);
var HOLE = /^\s*(?:[A-Za-z_$][\w$]*(?:\.[\w$]+)*|true|false|null|-?\d+(?:\.\d+)?|"[^"{}]*"|'[^'{}]*')\s*$/;
function attrList(attrs) {
  const list = [];
  for (const match of attrs.matchAll(/([^\s=/"']+)(?:\s*=\s*("[^"]*"|'[^']*'|[^\s"'>]+))?/g)) {
    const raw = match[2];
    if (raw === void 0) list.push({ name: match[1], value: null, quoted: false });
    else {
      const quoted = raw.startsWith('"') || raw.startsWith("'");
      list.push({ name: match[1], value: quoted ? raw.slice(1, -1) : raw, quoted });
    }
  }
  return list;
}
function resolveRelative(file, ref) {
  const clean = ref.split(/[?#]/)[0];
  const parts = clean.startsWith("/") ? [] : file.split("/").slice(0, -1);
  for (const part of clean.split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") {
      if (parts.length === 0) return null;
      parts.pop();
    } else parts.push(part);
  }
  return parts.join("/");
}
var isBlobUrl = (url) => /^(?:\.{1,2}\/)*\/?_blob\//.test(url.trim());
function checkAssetUrl(ctx, url, where) {
  const value = url.trim();
  if (value === "" || value.startsWith("#") || /^\{\{[\s\S]*\}\}$/.test(value) || isBlobUrl(value)) return;
  const add = (message) => ctx.issues.push({ file: ctx.file, message });
  if (/^data:/i.test(value)) add(`${where} uses a data: URI; upload the file with design_import_asset and use its /_blob/<id>`);
  else if (/^blob:/i.test(value)) add(`${where} uses a blob: URL, which only lives in one browser tab; upload the file with design_import_asset`);
  else if (isRemote(value) || /^[a-z][a-z0-9+.-]*:/i.test(value)) {
    if (!/^(?:https?:)?\/\/fonts\.gstatic\.com\//i.test(value)) add(`${where} loads ${value} from the network; upload the file with design_import_asset and use its /_blob/<id>`);
  } else add(`${where} names the file ${value}; upload it with design_import_asset and use the /_blob/<id> it returns`);
}
function checkMarkup(ctx, markup) {
  const add = (message) => ctx.issues.push({ file: ctx.file, message });
  const text = markup.replace(/<!--[\s\S]*?-->/g, (comment) => " ".repeat(comment.length));
  const stack = [];
  const once = /* @__PURE__ */ new Set();
  const addOnce = (key, message) => {
    if (once.has(key)) return;
    once.add(key);
    add(message);
  };
  let svgDepth = 0;
  TAG.lastIndex = 0;
  for (let match = TAG.exec(text); match; match = TAG.exec(text)) {
    const closing = match[1] === "/";
    const name = match[2];
    const lower = name.toLowerCase();
    const attrs = match[3] ?? "";
    if (closing) {
      const at = stack.lastIndexOf(lower);
      if (at < 0) {
        addOnce(`stray:${lower}`, `</${name}> has no matching <${name}>`);
        continue;
      }
      for (const unclosed of stack.splice(at).slice(1).reverse()) addOnce(`open:${unclosed}`, `<${unclosed}> is not closed before </${name}>; close every non-void element`);
      if (lower === "svg") svgDepth = Math.max(0, svgDepth - 1);
      continue;
    }
    if (svgDepth === 0 && /[A-Z]/.test(name)) {
      addOnce(`case:${name}`, `<${name}>: tag names are lower case; mount another artboard with <dc-import name="${name}"></dc-import>`);
    }
    const list = attrList(attrs.replace(/\/\s*$/, ""));
    for (const item of list) {
      if (item.value !== null && !item.quoted) addOnce(`quote:${item.name}`, `the attribute ${item.name}=${item.value} is not quoted; quote every attribute`);
      if (item.value !== null && /\{\{/.test(item.value)) {
        for (const hole of item.value.matchAll(/\{\{([\s\S]*?)\}\}/g)) {
          if (!HOLE.test(hole[1])) addOnce(`hole:${hole[1]}`, `{{${hole[1]}}} is an expression; a hole is a dotted lookup only: compute it in renderVals() and use its name`);
        }
      }
    }
    const style = list.find((item) => item.name.toLowerCase() === "style")?.value ?? "";
    if (/url\(/i.test(style)) addOnce("inline-url", 'url(\u2026) in an inline style="\u2026"; put it in a <helmet><style> rule');
    if (lower === "img" || lower === "source" && svgDepth === 0) {
      const src = list.find((item) => item.name.toLowerCase() === "src")?.value;
      if (src !== void 0 && src !== null) checkAssetUrl(ctx, src, `<${lower} src>`);
    }
    if (lower === "dc-import") {
      const ref = list.find((item) => item.name.toLowerCase() === "name")?.value ?? list.find((item) => item.name.toLowerCase() === "component")?.value;
      if (!ref) add('<dc-import> needs name="<artboard file name without .dc.html>"');
      else if (!/\{\{/.test(ref) && !ctx.stems.has(ref) && !ctx.tree.files.has(resolveRelative(ctx.file, `${ref}${BOARD_SUFFIX}`) ?? "")) {
        add(`<dc-import name="${ref}"> has no ${ref}${BOARD_SUFFIX} to mount`);
      }
    }
    if (lower === "a") {
      const href = list.find((item) => item.name.toLowerCase() === "href")?.value;
      if (href && !/\{\{/.test(href) && !isRemote(href) && !/^[a-z][a-z0-9+.-]*:/i.test(href) && href.split(/[?#]/)[0].endsWith(BOARD_SUFFIX)) {
        const target = resolveRelative(ctx.file, href);
        if (target === null || !ctx.tree.files.has(target)) add(`<a href="${href}"> links to an artboard that does not exist; the href is the target's path relative to this artboard (a leading / is the canvas root)`);
      }
    }
    const selfClosed = /\/\s*$/.test(attrs);
    if (RAW_TEXT.has(lower) && !selfClosed) {
      const end = new RegExp(`</${lower}\\s*>`, "ig");
      end.lastIndex = TAG.lastIndex;
      const found = end.exec(text);
      if (!found) {
        addOnce(`open:${lower}`, `<${lower}> is never closed`);
        return;
      }
      if (lower === "style") checkStyle(ctx, text.slice(TAG.lastIndex, found.index));
      TAG.lastIndex = end.lastIndex;
      continue;
    }
    if (VOID_ELEMENTS.has(lower)) continue;
    if (selfClosed) {
      if (svgDepth === 0) addOnce(`self:${lower}`, `<${name} /> is self-closed; write <${name}></${name}>`);
      continue;
    }
    stack.push(lower);
    if (lower === "svg") svgDepth += 1;
  }
  for (const unclosed of stack.reverse()) addOnce(`open:${unclosed}`, `<${unclosed}> is never closed; close every non-void element`);
  for (const hole of text.replace(/<[^>]*>/g, " ").matchAll(/\{\{([\s\S]*?)\}\}/g)) {
    if (!HOLE.test(hole[1])) addOnce(`hole:${hole[1]}`, `{{${hole[1]}}} is an expression; a hole is a dotted lookup only: compute it in renderVals() and use its name`);
  }
}
function checkStyle(ctx, css) {
  checkCssImports(ctx.file, css, ctx.issues);
  for (const match of css.matchAll(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*))\s*\)/gi)) {
    const url = match[1] ?? match[2] ?? match[3] ?? "";
    if (/^(?:https?:)?\/\/fonts\.googleapis\.com\//i.test(url.trim())) continue;
    checkAssetUrl(ctx, url, "url(\u2026)");
  }
}
function checkScript(ctx, code, logic) {
  const add = (message) => ctx.issues.push({ file: ctx.file, message });
  if (logic) {
    if (!/\bclass\s+Component\s+extends\s+DCLogic\b/.test(code)) add("the data-dc-script block must define class Component extends DCLogic");
    if (/^\s*(?:import|export)\b/m.test(code)) add("the data-dc-script block is classic JS: no import or export");
  }
  if (/\.(?:inner|outer)HTML\s*=(?!=)|\binsertAdjacentHTML\s*\(|\.appendChild\s*\(|\bdocument\.write(?:ln)?\s*\(/.test(code)) {
    add("builds UI from script (innerHTML / appendChild / document.write); all UI is <x-dc> markup");
  }
  if (/\bwindow\.[A-Za-z_$][\w$]*\s*=(?!=)/.test(code)) add("sets a window.X global; keep state in the component (state / setState)");
  if (/\b(?:window|document|globalThis)\s*\.\s*(?:addEventListener\s*\(\s*['"`]key(?:down|up|press)['"`]|onkey(?:down|up|press)\s*=)/.test(code)) {
    add("installs a global keyboard handler; artboards must not handle keys on window or document");
  }
  for (const match of code.matchAll(/\bfetch\s*\(\s*['"`]([^'"`]+)['"`]/g)) {
    if (isRemote(match[1])) add(`fetch("${match[1]}") goes to the network; upload the data with design_import_asset and fetch its /_blob/<id>`);
  }
  if (/\bnew\s+(?:WebSocket|EventSource)\s*\(|\bXMLHttpRequest\b/.test(code)) add("opens a network connection (WebSocket / EventSource / XMLHttpRequest); artboards stay offline");
}
function checkBoardFile(ctx, html) {
  const { file, size } = ctx;
  const add = (message) => ctx.issues.push({ file, message });
  const support = html.indexOf(SUPPORT_LINE);
  const headEnd = html.search(/<\/head\s*>/i);
  if (support < 0) add(`the <head> must contain the exact line ${SUPPORT_LINE}`);
  else if (headEnd >= 0 && support > headEnd) add(`${SUPPORT_LINE} must be inside <head>`);
  const open2 = /<x-dc\s*>/i.exec(html);
  const close = /<\/x-dc\s*>/i.exec(html);
  if (!open2 || !close || close.index < open2.index) add("the markup must be wrapped in <x-dc>\u2026</x-dc>");
  else checkMarkup(ctx, html.slice(open2.index + open2[0].length, close.index));
  const scripts = [...html.matchAll(new RegExp(String.raw`<script\b` + ATTRS + String.raw`>([\s\S]*?)<\/script\s*>`, "gi"))];
  const blocks = scripts.filter((m) => attrList(m[1] ?? "").some((a) => a.name.toLowerCase() === "data-dc-script"));
  if (blocks.length === 0) add(`missing the <script type="text/x-dc" data-dc-script data-props='\u2026'> block (class Component extends DCLogic)`);
  else if (blocks.length > 1) add(`has ${blocks.length} data-dc-script blocks; an artboard has exactly one`);
  const block = blocks[0];
  if (block) {
    const attrs = block[1] ?? "";
    if (attr(attrs, "type")?.value.toLowerCase() !== "text/x-dc") add('the data-dc-script block must have type="text/x-dc"');
    const props = attr(attrs, "data-props");
    if (props === null) add(`the data-dc-script block needs data-props='{"$preview":{"width":\u2026,"height":\u2026}}'`);
    else if (props.quote !== "'") add("data-props must be single-quoted: data-props='{\u2026}'");
    else {
      let parsed;
      try {
        parsed = JSON.parse(decodeEntities(props.value));
      } catch (error) {
        add(`data-props is not valid JSON (${error.message}); escape & as &amp; and ' as &#39;`);
      }
      if (parsed !== void 0) {
        const preview = isRecord2(parsed) ? parsed.$preview : void 0;
        if (!isRecord2(preview) || !isFiniteNumber(preview.width) || !isFiniteNumber(preview.height)) {
          add(`data-props needs "$preview": {"width": <number>, "height": <number>} equal to the board's w and h`);
        } else if (size && (preview.width !== size.w || preview.height !== size.h)) {
          add(`$preview is ${preview.width}\xD7${preview.height} but canvas.json boards gives ${size.w}\xD7${size.h}; make them equal`);
        }
        if (isRecord2(parsed)) {
          for (const [key, spec] of Object.entries(parsed)) {
            if (key === "$preview" || !isRecord2(spec) || !("editor" in spec)) continue;
            if (spec.editor !== null && !(typeof spec.editor === "string" && PROP_EDITORS.has(spec.editor))) {
              add(`data-props "${key}": editor must be one of text, color, int, float, range, boolean, enum or null`);
            }
          }
        }
      }
    }
    checkScript(ctx, block[2] ?? "", true);
  }
  for (const script2 of scripts) {
    if (script2 === block) continue;
    const list = attrList(script2[1] ?? "");
    const src = list.find((a) => a.name.toLowerCase() === "src")?.value;
    if (typeof src === "string") {
      if (src === "./support.js") continue;
      if (isRemote(src)) add(`<script src="${src}"> loads a script from the network; import it with design_import_asset and use its /_blob/<id>`);
      else if (!isBlobUrl(src)) checkRelativeFile(ctx, src, "<script src>");
    } else checkScript(ctx, script2[2] ?? "", false);
  }
  const embedded = new Set([...html.matchAll(/<(iframe|object|embed)\b/gi)].map((m) => m[1].toLowerCase()));
  for (const tag of embedded) add(`<${tag}> is not allowed in an artboard`);
  for (const match of html.matchAll(new RegExp(String.raw`<link\b` + ATTRS + ">", "gi"))) {
    const list = attrList(match[1] ?? "");
    const rel = list.find((a) => a.name.toLowerCase() === "rel")?.value?.toLowerCase() ?? "";
    const href = list.find((a) => a.name.toLowerCase() === "href")?.value;
    if (typeof href !== "string" || !rel.split(/\s+/).some((r) => r === "stylesheet" || r === "preload" || r === "modulepreload")) continue;
    if (isRemote(href)) {
      if (!isGoogleFonts(href)) add(`<link href="${href}"> loads from the network; only Google Fonts (fonts.googleapis.com) is allowed. Import the file with design_import_asset instead`);
    } else if (!isBlobUrl(href)) checkRelativeFile(ctx, href, "<link href>");
  }
  checkBlobRefs(file, html, ctx.tree.assets, ctx.issues);
}
function checkRelativeFile(ctx, ref, where) {
  const target = resolveRelative(ctx.file, ref);
  if (target === null || target === "" || !ctx.tree.files.has(target)) {
    ctx.issues.push({ file: ctx.file, message: `${where} "${ref}" is not a file in this design; write it there or upload it with design_import_asset` });
  }
}
var GUIDE_KINDS = /* @__PURE__ */ new Set(["columns", "rows", "grid"]);
var NOTE_KINDS = /* @__PURE__ */ new Set(["title1", "rect", "oval", "pen", "line", "arrow", "image"]);
var DS_FOLDER2 = /^[a-z0-9][a-z0-9_-]{0,63}$/;
var MAX_DESIGN_SYSTEMS2 = 4;
function checkDesignSystems(value, tree, add) {
  const folders = /* @__PURE__ */ new Set();
  for (const file of tree.files.keys()) {
    const parts = file.split("/");
    if (parts[0] === DESIGN_SYSTEMS_DIR && parts.length > 2) folders.add(parts[1]);
    else if (parts[0] === DESIGN_SYSTEMS_DIR && parts.length === 2) add(`${file}: files under ${DESIGN_SYSTEMS_DIR}/ belong in ${DESIGN_SYSTEMS_DIR}/<folder>/, one folder per design system`);
  }
  for (const folder of folders) {
    if (!DS_FOLDER2.test(folder)) add(`${DESIGN_SYSTEMS_DIR}/${folder}/: a design system folder is lower case letters, digits, - and _, starting with a letter or digit, 64 at most`);
  }
  const recorded = /* @__PURE__ */ new Set();
  if (value !== void 0) {
    if (!Array.isArray(value)) add('designSystems must be an array of {"title","namespace","artifact","version","copiedAt"}');
    else {
      if (value.length > MAX_DESIGN_SYSTEMS2) add(`designSystems has ${value.length} entries; a design uses at most ${MAX_DESIGN_SYSTEMS2}`);
      value.forEach((record, i) => {
        const where = `designSystems[${i}]`;
        if (!isRecord2(record) || typeof record.title !== "string" || typeof record.namespace !== "string" || typeof record.artifact !== "string" || !(record.version === null || typeof record.version === "string") || typeof record.copiedAt !== "string") {
          add(`${where} must be {"title": <string>, "namespace": <folder>, "artifact": <where it was copied from>, "version": <string or null>, "copiedAt": <RFC 3339 time>}`);
          return;
        }
        if (recorded.has(record.namespace)) add(`${where}: designSystems has the namespace "${record.namespace}" more than once`);
        recorded.add(record.namespace);
      });
    }
  }
  for (const folder of folders) {
    if (!recorded.has(folder)) add(`${DESIGN_SYSTEMS_DIR}/${folder}/ has no record in designSystems; the editor offers a design system only when both are there`);
  }
}
function checkIndex(canvas, tree, issues) {
  const add = (message) => issues.push({ file: CANVAS_FILE, message });
  if (canvas.title !== void 0 && typeof canvas.title !== "string") add("title must be a string");
  if (canvas.createdOnFiles !== void 0) {
    const created = canvas.createdOnFiles;
    if (!isRecord2(created) || created.v !== 1 || typeof created.at !== "string") add('createdOnFiles must be {"v":1,"at":"<ISO time>"}');
  }
  checkDesignSystems(canvas.designSystems, tree, add);
  const pageIds = /* @__PURE__ */ new Set();
  if (canvas.pages !== void 0) {
    if (!Array.isArray(canvas.pages)) add('pages must be an array of {"id","name"}');
    else {
      if (canvas.pages.length > MAX_PAGES) add(`pages has ${canvas.pages.length} entries; at most ${MAX_PAGES}`);
      canvas.pages.forEach((page, i) => {
        if (!isRecord2(page) || typeof page.id !== "string" || !ID_PATTERN.test(page.id) || typeof page.name !== "string") {
          add(`pages[${i}] must be {"id": [A-Za-z0-9_-]{1,40}, "name": <string>}`);
          return;
        }
        if (pageIds.has(page.id)) add(`pages has the id "${page.id}" more than once`);
        pageIds.add(page.id);
      });
    }
  }
  const boards = /* @__PURE__ */ new Map();
  for (const [key, value] of Object.entries(canvas.boards)) {
    const where = `boards["${key}"]`;
    if (!isCanvasFile(key) || !key.endsWith(BOARD_SUFFIX) || key.split("/")[0] === DESIGN_SYSTEMS_DIR) {
      add(`${where}: the key must be the artboard file's path, ending in ${BOARD_SUFFIX}, outside ${ASSETS_DIR}/ and ${DESIGN_SYSTEMS_DIR}/, with valid path segments`);
    } else if (!tree.files.has(key)) {
      add(`${where}: the file ${key} does not exist; write it or remove the entry`);
    }
    if (!isRecord2(value)) {
      add(`${where} must be an object with x, y, w and h`);
      boards.set(key, null);
      continue;
    }
    if (!isFiniteNumber(value.x) || !isFiniteNumber(value.y)) add(`${where}: x and y must be numbers`);
    const sizeOk = (n) => isFiniteNumber(n) && n >= BOARD_SIZE.min && n <= BOARD_SIZE.max;
    if (!sizeOk(value.w) || !sizeOk(value.h)) add(`${where}: w and h must be numbers from ${BOARD_SIZE.min} to ${BOARD_SIZE.max}`);
    boards.set(key, sizeOk(value.w) && sizeOk(value.h) ? { w: value.w, h: value.h } : null);
    if (value.title !== void 0 && typeof value.title !== "string") add(`${where}: title must be a string`);
    if (value.page !== void 0 && (typeof value.page !== "string" || !pageIds.has(value.page))) add(`${where}: page must be the id of an entry in pages`);
    if (value.expand !== void 0 && value.expand !== "fill") add(`${where}: expand can only be "fill"`);
    if (value.print !== void 0 && value.print !== "flow") add(`${where}: print can only be "flow"`);
    if (value.paper !== void 0 && value.paper !== "letter" && value.paper !== "a4") add(`${where}: paper must be "letter" or "a4"`);
    if (value.is_interactive !== void 0 && typeof value.is_interactive !== "boolean") add(`${where}: is_interactive must be true or false`);
    if (value.frameless !== void 0 && typeof value.frameless !== "boolean") add(`${where}: frameless must be true or false`);
    if (value.radius !== void 0 && !(isFiniteNumber(value.radius) && value.radius >= 0)) add(`${where}: radius must be a number of px, 0 or more`);
    if (value.guides !== void 0) {
      if (!Array.isArray(value.guides) || !value.guides.every(isRecord2)) add(`${where}: guides must be an array of guide objects`);
      else {
        if (value.guides.length > MAX_GUIDES) add(`${where}: guides has ${value.guides.length} entries; at most ${MAX_GUIDES}`);
        value.guides.forEach((guide, i) => {
          if (!GUIDE_KINDS.has(guide.kind)) add(`${where}: guides[${i}].kind must be "columns", "rows" or "grid"`);
        });
      }
    }
  }
  if (!Array.isArray(canvas.order)) add("order must be an array of the board paths, back to front");
  else {
    const listed = /* @__PURE__ */ new Set();
    canvas.order.forEach((item, i) => {
      if (typeof item !== "string" || !boards.has(item)) {
        add(`order[${i}] (${JSON.stringify(item)}) is not a key of boards`);
        return;
      }
      if (listed.has(item)) add(`order lists ${item} more than once`);
      listed.add(item);
    });
    for (const key of boards.keys()) if (!listed.has(key)) add(`${key} is in boards but not in order`);
  }
  if (canvas.notes !== void 0) {
    if (!isRecord2(canvas.notes)) add('notes must be an object {"<id>": {"x","y","text",\u2026}}');
    else {
      const entries = Object.entries(canvas.notes);
      if (entries.length > MAX_NOTES) add(`notes has ${entries.length} entries; it must stay under ${MAX_NOTES + 1}`);
      for (const [id, note] of entries) {
        if (!ID_PATTERN.test(id)) add(`notes: the id "${id}" must match [A-Za-z0-9_-]{1,40}`);
        if (!isRecord2(note) || !isFiniteNumber(note.x) || !isFiniteNumber(note.y) || typeof note.text !== "string") {
          add(`notes["${id}"] needs numbers x and y and a string text`);
          continue;
        }
        if (note.kind !== void 0 && !NOTE_KINDS.has(note.kind)) {
          add(`notes["${id}"].kind must be "title1" (a title over several artboards), left out (a sticky), or one of the user's rect, oval, pen, line, arrow, image`);
        }
        if (note.page !== void 0 && (typeof note.page !== "string" || !pageIds.has(note.page))) add(`notes["${id}"].page must be the id of an entry in pages`);
      }
    }
  }
  if (canvas.launch !== void 0) {
    const launch = canvas.launch;
    if (!isRecord2(launch)) add('launch must be {"view":"canvas"} or {"view":"focused","file":"<board path>"}');
    else if (launch.view === "canvas") {
      if (launch.page !== void 0 && (typeof launch.page !== "string" || !pageIds.has(launch.page))) add("launch.page must be the id of an entry in pages");
    } else if (launch.view === "focused") {
      if (typeof launch.file !== "string" || !boards.has(launch.file)) add("launch.file must be a key of boards");
    } else add('launch.view must be "canvas" or "focused"');
  }
  return boards;
}
async function checkCanvas(dir) {
  const issues = [];
  const tree = await scanTree(dir, issues);
  const fileCount = tree.files.size;
  if (tree.truncated || fileCount > CANVAS_LIMITS.files) {
    issues.push({
      file: ".",
      message: `${tree.truncated ? "more than" : "has"} ${tree.truncated ? MAX_SCAN_ENTRIES : fileCount} files outside assets/; a design holds at most ${CANVAS_LIMITS.files} (the editor lists only the first ${CANVAS_LIMITS.files})`
    });
  }
  if (tree.totalBytes > CANVAS_LIMITS.totalBytes) {
    issues.push({ file: ".", message: `files and assets add up to ${tree.totalBytes} bytes; a design holds at most ${CANVAS_LIMITS.totalBytes} (256 MB)` });
  }
  let raw = null;
  const indexFile = path6.join(dir, CANVAS_FILE);
  const indexStat = await fs4.lstat(indexFile).catch(() => null);
  if (indexStat?.isFile()) raw = await fs4.readFile(indexFile, "utf8").catch(() => null);
  let parsed;
  if (raw === null) issues.push({ file: CANVAS_FILE, message: 'missing: a design needs canvas.json with "v": 3 and "boards"' });
  else {
    try {
      parsed = JSON.parse(raw);
    } catch (error) {
      issues.push({ file: CANVAS_FILE, message: `is not valid JSON: ${error.message}` });
    }
  }
  if (raw !== null && parsed !== void 0 && !isCanvasMetadata(parsed)) {
    issues.push({ file: CANVAS_FILE, message: 'must be an object with "v": 3 and "boards" as an object keyed by artboard path' });
  }
  const canvas = isCanvasMetadata(parsed) ? parsed : null;
  const boards = canvas ? checkIndex(canvas, tree, issues) : /* @__PURE__ */ new Map();
  const stems = /* @__PURE__ */ new Map();
  for (const file of tree.files.keys()) {
    if (!isBoardPath(file)) continue;
    const stem = path6.basename(file).slice(0, -BOARD_SUFFIX.length);
    const other = stems.get(stem);
    if (other !== void 0) issues.push({ file, message: `has the same name as ${other}; artboard file names (without ${BOARD_SUFFIX}) must be unique` });
    else stems.set(stem, file);
  }
  for (const [file, bytes] of tree.files) {
    const ext = path6.extname(file).toLowerCase();
    const board = isBoardPath(file);
    if (file.split("/")[0] === DESIGN_SYSTEMS_DIR) continue;
    if (board) {
      if (canvas && !boards.has(file)) {
        issues.push({
          file,
          message: `is not in canvas.json boards; every ${BOARD_SUFFIX} outside ${DESIGN_SYSTEMS_DIR}/ shows on the canvas, so add a boards entry (w and h = its $preview) and its path in order`
        });
      }
    }
    if (!board && !TEXT_EXTENSIONS.has(ext)) continue;
    if (bytes > MAX_BOARD_BYTES) {
      if (board) issues.push({ file, message: `is ${bytes} bytes; an artboard file should stay well under ${MAX_BOARD_BYTES}` });
      continue;
    }
    const text = await fs4.readFile(path6.join(dir, ...file.split("/")), "utf8").catch(() => null);
    if (text === null) continue;
    if (board) checkBoardFile({ file, size: boards.get(file) ?? null, tree, stems, issues }, text);
    else {
      checkBlobRefs(file, text, tree.assets, issues);
      if (ext === ".css") checkCssImports(file, text, issues);
    }
  }
  return {
    title: canvas ? canvasTitle(canvas, dir) : path6.basename(dir),
    boardCount: canvas ? Object.keys(canvas.boards).length : 0,
    issues
  };
}

import fs5 from "node:fs/promises";
import path7 from "node:path";
var DESIGN_SYSTEM_FILE = "design-system.json";
var BLOBS_DIR = "_blobs";
var DESIGN_SYSTEM_LIMITS = {
  files: 4096,
  ownFiles: 4080,
  totalBytes: 256 * 1024 * 1024,
  fileBytes: 15 * 1024 * 1024,
  publishBytes: 16 * 1024 * 1024,
  uploadBytes: 20 * 1024 * 1024,
  svgUploadBytes: 2 * 1024 * 1024
};
var DESIGN_SYSTEM_PATHS = {
  readme: "README.md",
  tokens: "tokens.json",
  manifest: "manifest.json",
  componentsDir: "components/",
  libDir: "components/lib/",
  bundleJs: "components/bundle.js",
  bundleCss: "components/bundle.css",
  indexDts: "components/index.d.ts",
  fontsDir: "fonts/",
  assetsDir: "assets/",
  archivedDir: "archived/"
};
var GENERATED_TOKENS_CSS_PATH = "tokens.css";
var GENERATED_API_PATH_PATTERN = /^api\/(?:icons\.md|tokens\.md|(?:tokens|components|assets)\/[^/]+\.md)$/;
var EDITOR_OWN_FILES = ["migration.json", "design-system.before-files.json", "publish-progress.json", "migration-map.json"];
var WINDOW_NAMESPACE_PATTERN = /^(?!(?:__proto__|constructor|prototype)$)[A-Za-z_$][A-Za-z0-9_$]{0,63}$/;
var COMPONENT_NAME_PATTERN = WINDOW_NAMESPACE_PATTERN;
var MAX_DESIGN_SYSTEM_PATH_LENGTH = 300;
var MAX_RUNTIME_PATH_UTF8_BYTES = 512;
var UNSAFE_PATH_CHARACTER_PATTERN = /[\u0000-\u001f\u007f-\u009f\u200b\u200e\u200f\u202a-\u202e\u2028\u2029\u2060-\u206f\u2215\u2044\ufeff\uff0f]/;
var RUNTIME_PATH_RESERVED_CHARACTERS = /[%;:\\]/;
var INVISIBLE_PATH_CHARACTER_PATTERN = /[\u200b-\u200f\u202a-\u202e\u2060-\u206f\ufeff]/g;
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
var WINDOWS_SHORT_NAME_PATTERN = /~\d/;
var IMAGE_SLOT_STATE_FILENAME = ".image-slots.state.json";
function comparableSegment(segment) {
  let normalized = segment.replace(INVISIBLE_PATH_CHARACTER_PATTERN, "");
  const colon = normalized.indexOf(":");
  if (colon >= 0) normalized = normalized.slice(0, colon);
  return normalized.normalize("NFC").toUpperCase().toLowerCase().replace(/[. ]+$/, "");
}
function comparablePath(rel) {
  return rel.split("/").map(comparableSegment).join("/");
}
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
  if (parts.some((p) => WINDOWS_SHORT_NAME_PATTERN.test(p))) {
    return 'has a path segment shaped like a Windows 8.3 short name (NAME~1); rename it without the "~digit"';
  }
  return null;
}
function explainReservedSystemPath(rel) {
  const normalized = comparablePath(rel);
  const under = (name) => normalized === name || normalized.startsWith(`${name}/`);
  if (under(DESIGN_SYSTEM_FILE)) return `is reserved: ${DESIGN_SYSTEM_FILE} is the index; rename the file`;
  if (under("index.html")) return "is reserved: index.html is the page itself in the original format; rename the file";
  if (under("skill.md") || under("artifact-type")) {
    return "is reserved: SKILL.md and artifact-type/ are the design system type\u2019s own files in the original format; rename the file";
  }
  return null;
}
function explainDesignSystemPath(rel) {
  if (rel.length === 0 || rel.length > MAX_DESIGN_SYSTEM_PATH_LENGTH || Buffer.byteLength(`project/${rel}`, "utf8") > MAX_RUNTIME_PATH_UTF8_BYTES) {
    return `is empty or longer than ${MAX_DESIGN_SYSTEM_PATH_LENGTH} characters; rename it`;
  }
  if (rel.startsWith("/") || UNSAFE_PATH_CHARACTER_PATTERN.test(rel) || RUNTIME_PATH_RESERVED_CHARACTERS.test(rel)) {
    return "has a character a design system path cannot hold (a control or invisible character, a look-alike slash, % ; : or \\); rename it";
  }
  if (rel.endsWith(".cmekshadow")) return "ends in .cmekshadow, which is reserved; rename it";
  for (const part of rel.split("/")) {
    if (part === "" || part === "." || part === "..") return "has an empty path segment or one that is only dots; rename it";
    if (RESERVED_PROTOTYPE_NAMES.has(part)) return `uses "${part}" as a path segment, which is reserved; rename it`;
  }
  return explainToolConfigurationPath(rel) ?? explainReservedSystemPath(rel);
}
function isBlobsPath(rel) {
  return rel === BLOBS_DIR || rel.startsWith(`${BLOBS_DIR}/`);
}
function isDesignSystemFile(rel) {
  if (rel === DESIGN_SYSTEM_FILE) return true;
  return !isBlobsPath(rel) && explainDesignSystemPath(rel) === null;
}
function descendInto(rel) {
  return !isBlobsPath(rel) && explainDesignSystemPath(rel) === null;
}
function designSystemUploadLimit(contentType) {
  const type = contentType.split(";")[0].trim().toLowerCase();
  return type === "image/svg+xml" ? DESIGN_SYSTEM_LIMITS.svgUploadBytes : DESIGN_SYSTEM_LIMITS.uploadBytes;
}
var DESIGN_SYSTEM_LAYOUT = {
  blobDir: BLOBS_DIR,
  isFile: isDesignSystemFile,
  roots: [{ rel: "", descend: true }],
  descendInto,
  watches: (rel) => isDesignSystemFile(rel) || descendInto(rel),
  maxFiles: DESIGN_SYSTEM_LIMITS.files,
  maxPublishBytes: DESIGN_SYSTEM_LIMITS.publishBytes,
  maxFileBytes: DESIGN_SYSTEM_LIMITS.fileBytes,
  maxBlobBytes: designSystemUploadLimit,
  readOnlyMessage: (p) => {
    const rel = p.startsWith("project/") ? p.slice("project/".length) : p;
    if (isBlobsPath(rel)) return `${p}: ${BLOBS_DIR}/ holds uploads and can't be written; upload the file instead`;
    return `${p}: ${explainDesignSystemPath(rel) ?? "is not a design system file"}`;
  }
};
var MAX_INDEX_BYTES = 8 * 1024 * 1024;
var MAX_DESIGN_SYSTEM_INDEX_CHARS = 2e6;
var isRecord3 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
function isDesignSystemIndex(value) {
  return isRecord3(value) && !isRecord3(value.content) && (hasDesignSystemMarker(value) || typeof value.v === "number" && value.v >= 3 && value.layout === "files");
}
function hasDesignSystemMarker(index) {
  return isRecord3(index.createdOnFiles) || isRecord3(index.convertedFrom);
}
async function readDesignSystemJson(dir) {
  const file = path7.join(dir, DESIGN_SYSTEM_FILE);
  try {
    const stat = await fs5.lstat(file);
    if (!stat.isFile() || stat.size > MAX_INDEX_BYTES) return null;
    return JSON.parse(await fs5.readFile(file, "utf8"));
  } catch {
    return null;
  }
}
function designSystemTitle(index, dir) {
  const raw = index.title;
  const title = typeof raw === "string" ? raw.trim() : "";
  return title || path7.basename(dir);
}
function designSystemNamespace(index) {
  const raw = index.namespace;
  return typeof raw === "string" && WINDOW_NAMESPACE_PATTERN.test(raw) ? raw : null;
}
async function readDesignSystemInfo(dir) {
  const parsed = await readDesignSystemJson(dir);
  if (!isDesignSystemIndex(parsed)) return null;
  return { title: designSystemTitle(parsed, dir), namespace: designSystemNamespace(parsed) };
}

var TOKENS_LIMITS = {
  bytes: 512 * 1024,
  themes: 8,
  colors: 600,
  fontFaces: 4096,
  fontFamilies: 12,
  typographyGroups: 12,
  typographyStyles: 80,
  tokensPerFamily: 60,
  extraFamilies: 12,
  colorAliasDepth: 16
};
var TOKEN_NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/;
var HEX_COLOR_PATTERN = /^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/;
var COLOR_ALIAS_PATTERN = /^\{([A-Za-z0-9][A-Za-z0-9_.-]{0,63})\}$/;
var EXTRA_TOKEN_FAMILY_PATTERN = /^(?!(?:__proto__|constructor|prototype)$)[A-Za-z][A-Za-z0-9_-]{0,31}$/;
var BUILTIN_TOKEN_DOCUMENT_KEYS = ["version", "color", "type", "spacing", "radius", "shadow"];
var COLOR_FUNCTION_PATTERN = /^(?:rgb|rgba|hsl|hsla|oklch|oklab|lab|lch|color)\([0-9a-z.,%/ \-]{1,60}\)$/;
var CSS_LENGTH_PATTERN = /^-?\d{1,4}(?:\.\d{1,6})?(?:px|rem|em|%)$|^0$/;
var FONT_STACK_PATTERN = /^[^;{}<>()\\\r\n\f]{1,200}$/;
var BOX_SHADOW_PATTERN = /^[a-z0-9 #%(),./+-]{1,400}$/;
var MOTION_DURATION_PATTERN = /^(?:\d{1,6}(?:\.\d{1,3})?|\.\d{1,3})m?s$/;
var MOTION_EASING_PATTERN = /^(?:linear|ease|ease-in|ease-out|ease-in-out|step-start|step-end|cubic-bezier\([-\d., ]{1,60}\)|steps\([a-z0-9, -]{1,40}\))$/;
var EXTRA_TOKEN_VALUE_PATTERN = /^[A-Za-z0-9 #%(),./+_-]{1,200}$/;
var isObject = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
var asArray = (value) => Array.isArray(value) ? value : [];
function isSafeFontStack(stack) {
  if (!FONT_STACK_PATTERN.test(stack) || stack.includes("/*") || stack.includes("*/")) return false;
  let quote2 = null;
  for (const ch of stack) {
    if (quote2 === null) {
      if (ch === '"' || ch === "'") quote2 = ch;
    } else if (ch === quote2) quote2 = null;
  }
  return quote2 === null;
}
function isColorLiteral(value) {
  return typeof value === "string" && (HEX_COLOR_PATTERN.test(value) || COLOR_FUNCTION_PATTERN.test(value) && !/url|var|expression/.test(value));
}
function balanced(value) {
  let depth = 0;
  for (const ch of value) {
    if (ch === "(") depth++;
    else if (ch === ")" && --depth < 0) return false;
  }
  return depth === 0;
}
function normalizeBoxShadow(value) {
  if (typeof value !== "string") return void 0;
  const normalized = value.trim().toLowerCase().replace(/\s+/g, " ");
  if (!BOX_SHADOW_PATTERN.test(normalized) || /(?:url|var|expression|image|image-set|src)\s*\(/.test(normalized) || normalized === "") return void 0;
  return balanced(normalized) ? normalized : void 0;
}
function normalizeMotionValue(value) {
  if (typeof value === "number" && Number.isFinite(value) && value >= 0 && value < 1e6) return `${Math.round(value)}ms`;
  if (typeof value !== "string") return void 0;
  let normalized = value.trim().toLowerCase().replace(/\s+/g, " ");
  if (normalized.startsWith(".")) normalized = "0" + normalized;
  return MOTION_DURATION_PATTERN.test(normalized) || MOTION_EASING_PATTERN.test(normalized) ? normalized : void 0;
}
function normalizeExtraTokenValue(value) {
  if (typeof value === "number" && Number.isFinite(value) && Math.abs(value) < 1e6) return String(value);
  if (typeof value !== "string") return void 0;
  const normalized = value.trim().replace(/\s+/g, " ");
  if (!EXTRA_TOKEN_VALUE_PATTERN.test(normalized) || /(?:url|var|expression|image|image-set|src)\s*\(/i.test(normalized)) return void 0;
  return balanced(normalized) ? normalized : void 0;
}
function normalizeColorOrAlias(value) {
  if (typeof value !== "string") return void 0;
  const literal = value.trim().toLowerCase();
  if (isColorLiteral(literal)) return literal;
  const alias = value.trim().replace(/^\{--/, "{");
  return COLOR_ALIAS_PATTERN.test(alias) ? alias : void 0;
}
function normalizeCssLength(value, allowFullRadius) {
  if (typeof value === "number" && Number.isFinite(value)) {
    if (Math.abs(value) >= 1e4) return void 0;
    return value === 0 ? "0" : `${Math.round(value * 1e3) / 1e3}px`;
  }
  if (typeof value !== "string") return void 0;
  const normalized = value.trim().toLowerCase();
  if (allowFullRadius && normalized === "full") return "9999px";
  return CSS_LENGTH_PATTERN.test(normalized) ? normalized : void 0;
}
var colorAliasTarget = (value) => value === void 0 ? void 0 : COLOR_ALIAS_PATTERN.exec(value)?.[1];
function explainColorAliasError(token2, themeId, baseThemeId, byName) {
  const seen = /* @__PURE__ */ new Set([token2.name]);
  let current = token2;
  for (let index = 0; index <= TOKENS_LIMITS.colorAliasDepth; index++) {
    const target = colorAliasTarget(current.value[themeId] ?? current.value[baseThemeId]);
    if (target === void 0) return void 0;
    const entry = byName.get(target);
    if (entry === void 0) return { why: `${index === 0 ? "" : `reaches {${target}}, which `}names no color token`, own: index === 0 };
    if (seen.has(target)) return { why: "is a cycle", own: target === token2.name };
    seen.add(target);
    current = entry;
  }
  return { why: `chains deeper than ${TOKENS_LIMITS.colorAliasDepth}`, own: true };
}
function removeInvalidColorAliases(themes, tokens, dropped) {
  const baseThemeId = themes[0].id;
  for (; ; ) {
    const byName = new Map(tokens.map((t) => [t.name, t]));
    const items = [];
    for (const token2 of tokens) {
      for (const theme of themes) {
        if (colorAliasTarget(token2.value[theme.id]) === void 0) continue;
        const error = explainColorAliasError(token2, theme.id, baseThemeId, byName);
        if (error !== void 0) items.push([token2, theme.id, error.why, error.own]);
      }
    }
    if (items.length === 0) return;
    const anyOwn = items.some((item) => item[3]);
    for (const [token2, themeId, why, own] of items) {
      if (anyOwn && !own) continue;
      dropped.push(`color ${token2.name} [${themeId}]: ${String(token2.value[themeId])} ${why}`);
      delete token2.value[themeId];
    }
    for (let index = 0; index < tokens.length; index++) {
      const token2 = tokens[index];
      if (Object.hasOwn(token2.value, baseThemeId)) continue;
      const theme = themes.find((t) => Object.hasOwn(token2.value, t.id));
      if (theme === void 0) {
        dropped.push(`color ${token2.name}: no valid value`);
        tokens.splice(index--, 1);
      } else {
        token2.value[baseThemeId] = token2.value[theme.id];
        delete token2.value[theme.id];
      }
    }
  }
}
function shapeProblem(key, value) {
  if (!isObject(value)) return `"${key}" must be an object {"tokens":[{"name","value","usage"}\u2026]}`;
  if (!Array.isArray(value.tokens)) {
    return `"${key}" is not shaped {"tokens":[{"name","value","usage"}\u2026]}: a name-to-value map is valid JSON the editor cannot read (the family shows empty and its entries leave the file at the first token edit); make it a list`;
  }
  return null;
}
function checkTokensText(text) {
  const problems = [];
  const notes = [];
  const dropped = [];
  let source;
  try {
    const parsed = JSON.parse(text);
    if (!isObject(parsed)) return { problems: ["is not a JSON object"], notes };
    source = parsed;
  } catch (error) {
    return { problems: [`is not valid JSON: ${String(error.message).slice(0, 120)}`], notes };
  }
  if (Object.hasOwn(source, "color") && !isObject(source.color)) problems.push('"color" must be an object {"themes":[\u2026],"tokens":[\u2026]}');
  const sourceColor = isObject(source.color) ? source.color : {};
  if (isObject(source.color) && !Array.isArray(sourceColor.tokens) && !Array.isArray(sourceColor.groups) && Object.hasOwn(sourceColor, "tokens")) {
    problems.push('"color.tokens" is not a list: a name-to-value map is valid JSON the editor cannot read; make it [{"name","value","usage"}\u2026]');
  }
  if (isObject(source.color) && !Object.hasOwn(sourceColor, "tokens") && !Object.hasOwn(sourceColor, "groups")) {
    const keys = Object.keys(sourceColor).filter((k) => k !== "themes" && k !== "note");
    if (keys.length > 0) problems.push('"color" has no "tokens" list: a name-to-value map is valid JSON the editor cannot read; make it {"themes":[\u2026],"tokens":[{"name","value","usage"}\u2026]}');
  }
  const sourceThemes = asArray(sourceColor.themes);
  if (sourceThemes.length > TOKENS_LIMITS.themes) problems.push(`color has ${sourceThemes.length} themes; at most ${TOKENS_LIMITS.themes} are read`);
  const themes = [];
  const themeIds = /* @__PURE__ */ new Set();
  for (const theme of sourceThemes) {
    if (themes.length >= TOKENS_LIMITS.themes) break;
    const id = isObject(theme) && typeof theme.id === "string" ? theme.id.trim().toLowerCase() : typeof theme === "string" ? theme.trim().toLowerCase() : "";
    if (!TOKEN_NAME_PATTERN.test(id) || themeIds.has(id) || id === "constructor" || id === "prototype" || Object.hasOwn(Object.prototype, id)) {
      dropped.push(`theme ${JSON.stringify(id)}: bad or duplicate id`);
      continue;
    }
    themeIds.add(id);
    themes.push({ id });
  }
  if (themes.length === 0) themes.push({ id: "light" });
  const baseThemeId = themes[0].id;
  const colorNames = /* @__PURE__ */ new Set();
  const colorTokens = [];
  const sourceColorTokens = Array.isArray(sourceColor.tokens) ? asArray(sourceColor.tokens) : asArray(sourceColor.groups).flatMap((group) => isObject(group) ? asArray(group.tokens) : []);
  if (Array.isArray(sourceColor.tokens) && Array.isArray(sourceColor.groups)) {
    problems.push('color has both "tokens" and a legacy "groups"; only "tokens" is read');
  }
  if (sourceColorTokens.length > TOKENS_LIMITS.colors) problems.push(`color tokens capped at ${TOKENS_LIMITS.colors}; the rest are ignored`);
  for (const token2 of sourceColorTokens) {
    if (colorTokens.length >= TOKENS_LIMITS.colors) break;
    if (!isObject(token2)) continue;
    const name = typeof token2.name === "string" ? token2.name.trim().replace(/^--/, "") : "";
    if (!TOKEN_NAME_PATTERN.test(name) || colorNames.has(name)) {
      dropped.push(`color ${JSON.stringify(name)}: bad or duplicate name`);
      continue;
    }
    const value = {};
    if (isObject(token2.value)) {
      for (const theme of themes) {
        const normalized = Object.hasOwn(token2.value, theme.id) ? normalizeColorOrAlias(token2.value[theme.id]) : void 0;
        if (normalized !== void 0) value[theme.id] = normalized;
        else if (Object.hasOwn(token2.value, theme.id)) dropped.push(`color ${name} [${theme.id}]: not a color value`);
      }
    } else {
      const normalized = normalizeColorOrAlias(token2.value);
      if (normalized !== void 0) value[baseThemeId] = normalized;
    }
    if (!Object.hasOwn(value, baseThemeId)) {
      const borrowed = themes.map((t) => value[t.id]).find((v) => v !== void 0);
      if (borrowed === void 0) {
        dropped.push(`color ${name}: no valid value`);
        continue;
      }
      value[baseThemeId] = borrowed;
    }
    colorNames.add(name);
    colorTokens.push({ name, value });
  }
  removeInvalidColorAliases(themes, colorTokens, dropped);
  if (Object.hasOwn(source, "type") && !isObject(source.type)) problems.push('"type" must be an object {"fonts":[\u2026],"families":{\u2026},"groups":[\u2026]}');
  const sourceType = isObject(source.type) ? source.type : {};
  for (const key of ["fonts", "groups"]) {
    if (Object.hasOwn(sourceType, key) && !Array.isArray(sourceType[key])) problems.push(`"type.${key}" must be a list`);
  }
  if (Object.hasOwn(sourceType, "families") && !isObject(sourceType.families)) problems.push('"type.families" must be an object {"<key>":"<font stack>"}');
  const sourceFonts = asArray(sourceType.fonts);
  if (sourceFonts.length > TOKENS_LIMITS.fontFaces) {
    notes.push(`type.fonts: ${sourceFonts.length - TOKENS_LIMITS.fontFaces} entries after the first ${TOKENS_LIMITS.fontFaces} fonts are kept in the file but not read`);
  }
  for (const font of sourceFonts.slice(0, TOKENS_LIMITS.fontFaces)) {
    if (!isObject(font)) continue;
    const family = typeof font.family === "string" ? font.family.trim() : "";
    const file = typeof font.file === "string" ? font.file.trim() : "";
    if (family === "" || !isSafeFontStack(family) || /["']/.test(family)) {
      dropped.push(`font ${JSON.stringify(file)}: bad family name`);
      continue;
    }
    if (file === "" || /^[a-z][a-z0-9+.-]*:/i.test(file)) {
      dropped.push(`font ${JSON.stringify(family)}: bad file path (a relative path into the system, e.g. fonts/Name.woff2)`);
    }
  }
  const families = {};
  if (isObject(sourceType.families)) {
    const keys = Object.keys(sourceType.families);
    if (keys.length > TOKENS_LIMITS.fontFamilies) problems.push(`type.families has ${keys.length} families; at most ${TOKENS_LIMITS.fontFamilies} are read`);
    let index = 0;
    for (const key of keys) {
      if (index >= TOKENS_LIMITS.fontFamilies) break;
      const id = key.trim().toLowerCase();
      const stack = sourceType.families[key];
      if (!TOKEN_NAME_PATTERN.test(id) || typeof stack !== "string" || !isSafeFontStack(stack.trim())) {
        dropped.push(`font family ${JSON.stringify(key)}: bad key or stack`);
        continue;
      }
      families[id] = stack.trim();
      index++;
    }
  }
  const sourceGroups = asArray(sourceType.groups);
  if (sourceGroups.length > TOKENS_LIMITS.typographyGroups) problems.push(`type.groups has ${sourceGroups.length} groups; at most ${TOKENS_LIMITS.typographyGroups} are read`);
  const styleNames = /* @__PURE__ */ new Set();
  let styleCount = 0;
  let stylesSeen = 0;
  for (const group of sourceGroups.slice(0, TOKENS_LIMITS.typographyGroups)) {
    if (!isObject(group)) continue;
    if (Object.hasOwn(group, "styles") && !Array.isArray(group.styles)) problems.push(`type group ${JSON.stringify(group.name ?? "")}: "styles" must be a list`);
    for (const style of asArray(group.styles)) {
      stylesSeen++;
      if (styleCount >= TOKENS_LIMITS.typographyStyles) continue;
      if (!isObject(style)) continue;
      const name = typeof style.name === "string" ? style.name.trim().replace(/^\./, "") : "";
      if (!TOKEN_NAME_PATTERN.test(name) || styleNames.has(name)) {
        dropped.push(`type style ${JSON.stringify(name)}: bad or duplicate name`);
        continue;
      }
      if (normalizeCssLength(style.fontSize, false) === void 0) {
        dropped.push(`type style ${name}: bad fontSize`);
        continue;
      }
      styleNames.add(name);
      styleCount++;
    }
  }
  if (stylesSeen > TOKENS_LIMITS.typographyStyles) problems.push(`type has ${stylesSeen} styles; at most ${TOKENS_LIMITS.typographyStyles} are read`);
  const familyOf = new Map(colorTokens.map((t) => [t.name, "color"]));
  const scalarFamily = (input, family, normalize) => {
    const tokens = asArray(isObject(input) ? input.tokens : void 0);
    if (tokens.length > TOKENS_LIMITS.tokensPerFamily) problems.push(`${family} has ${tokens.length} tokens; at most ${TOKENS_LIMITS.tokensPerFamily} are read`);
    let count = 0;
    for (const token2 of tokens) {
      if (count >= TOKENS_LIMITS.tokensPerFamily) break;
      if (!isObject(token2)) continue;
      const name = typeof token2.name === "string" ? token2.name.trim().replace(/^--/, "") : "";
      const other = familyOf.get(name);
      if (!TOKEN_NAME_PATTERN.test(name) || other === family) {
        dropped.push(`${family} ${JSON.stringify(name)}: bad or duplicate name`);
        continue;
      }
      if (other !== void 0) {
        dropped.push(`${family} ${name}: a ${other} token already has that name (one --${name} across every family)`);
        continue;
      }
      if (normalize(token2.value, name) === void 0) {
        dropped.push(`${family} ${name}: bad value`);
        continue;
      }
      familyOf.set(name, family);
      count++;
    }
  };
  const shadowValue = (input, name) => {
    const value = {};
    if (isObject(input)) {
      for (const theme of themes) {
        if (!Object.hasOwn(input, theme.id)) continue;
        const normalized = normalizeBoxShadow(input[theme.id]);
        if (normalized !== void 0) value[theme.id] = normalized;
        else dropped.push(`shadow ${name} [${theme.id}]: not a box-shadow value`);
      }
    } else {
      const normalized = normalizeBoxShadow(input);
      if (normalized !== void 0) value[baseThemeId] = normalized;
    }
    if (!Object.hasOwn(value, baseThemeId)) {
      const theme = themes.find((t) => Object.hasOwn(value, t.id));
      if (theme === void 0) return void 0;
      value[baseThemeId] = value[theme.id];
    }
    return value;
  };
  for (const key of ["spacing", "radius"]) {
    if (Object.hasOwn(source, key)) {
      const shape = shapeProblem(key, source[key]);
      if (shape) problems.push(shape);
    }
  }
  scalarFamily(source.spacing, "spacing", (value) => normalizeCssLength(value, false));
  scalarFamily(source.radius, "radius", (value) => normalizeCssLength(value, true));
  scalarFamily(source.shadow, "shadow", shadowValue);
  if (Object.hasOwn(source, "shadow") && !(isObject(source.shadow) && Array.isArray(source.shadow.tokens))) {
    notes.push('"shadow" is not shaped {"tokens":[{name,value,usage}]}: kept in the file, not shown');
  }
  let extraCount = 0;
  for (const key of Object.keys(source)) {
    if (BUILTIN_TOKEN_DOCUMENT_KEYS.includes(key)) continue;
    const value = source[key];
    if (key === "name" && typeof value === "string") continue;
    if (!EXTRA_TOKEN_FAMILY_PATTERN.test(key) || !isObject(value) || !Array.isArray(value.tokens)) {
      notes.push(`${JSON.stringify(key.slice(0, 40))} is not a token family ({"tokens":[{name,value,usage}]}): kept in the file, not shown`);
      continue;
    }
    if (extraCount >= TOKENS_LIMITS.extraFamilies) {
      notes.push(`token families beyond the built-in ones are capped at ${TOKENS_LIMITS.extraFamilies}: ${JSON.stringify(key)} is kept in the file, not shown`);
      continue;
    }
    extraCount++;
    scalarFamily(value, key, key === "motion" ? (v) => normalizeMotionValue(v) ?? normalizeExtraTokenValue(v) : normalizeExtraTokenValue);
  }
  return { problems: [...problems, ...dropped], notes };
}
function tokenFontFiles(text) {
  try {
    const parsed = JSON.parse(text);
    if (!isObject(parsed) || !isObject(parsed.type)) return [];
    const files = [];
    for (const font of asArray(parsed.type.fonts)) {
      if (!isObject(font) || typeof font.file !== "string") continue;
      const file = font.file.trim();
      if (file === "" || /^[a-z][a-z0-9+.-]*:/i.test(file)) continue;
      files.push(file.includes("/") ? file : `fonts/${file}`);
    }
    return files;
  } catch {
    return [];
  }
}

import fs6 from "node:fs/promises";
import path8 from "node:path";
var DESIGN_SYSTEM_NOT_CHECKED = [DESIGN_SYSTEM_PATHS.manifest, GENERATED_TOKENS_CSS_PATH, "api/"];
var KIB = 1024;
var MIB = 1024 * 1024;
var DESIGN_SYSTEM_FILE_LIMITS = {
  readme: 200 * KIB,
  tokens: 512 * KIB,
  bundleJs: 6 * MIB,
  bundleCss: 2 * MIB,
  dtsTotal: 1.5 * MIB,
  preview: 256 * KIB,
  componentReadme: 64 * KIB,
  componentLib: 2 * MIB,
  asset: 12 * MIB,
  other: 512 * KIB
};
var MAX_SECTION_DOCUMENTS = 24;
var MAX_SCAN_ENTRIES2 = 2e4;
var MAX_READ_BYTES = 8 * MIB;
var FONT_EXTENSIONS = {
  woff2: "WOFF2",
  woff: "WOFF",
  ttf: "TrueType",
  otf: "OpenType",
  ttc: "TrueType Collection"
};
var FORBIDDEN_PREVIEW_ELEMENTS = ["iframe", "frame", "object", "embed", "portal", "noscript"];
var CARD_HEADER_PATTERN = /^[ \t]*<!--\s*@dsCard\b((?:(?!--)[^\r\n])*)(?:[\r\n]\s*)?-->/;
var ASSET_TILE_SIZES = /* @__PURE__ */ new Set(["xs", "s", "m", "l", "xl"]);
var MAX_ASSET_GROUP_NAME_LENGTH = 80;
var RESERVED_PROTOTYPE_NAMES2 = /* @__PURE__ */ new Set(["__proto__", "constructor", "prototype"]);
var TEXT_EXTENSIONS2 = /* @__PURE__ */ new Set(["md", "mdx", "markdown", "txt", "json", "js", "mjs", "cjs", "jsx", "ts", "tsx", "css", "html", "htm", "svg", "xml", "csv", "yml", "yaml", "toml"]);
var BLOB_ID = BLOB_ID_PATTERN;
function extensionOf(rel) {
  const base = rel.slice(rel.lastIndexOf("/") + 1).toLowerCase();
  if (base.endsWith(".d.ts")) return "d.ts";
  const dot = base.lastIndexOf(".");
  return dot <= 0 ? "" : base.slice(dot + 1);
}
function isSectionDocument(rel) {
  return rel !== DESIGN_SYSTEM_PATHS.readme && !rel.startsWith(DESIGN_SYSTEM_PATHS.componentsDir) && !rel.startsWith(DESIGN_SYSTEM_PATHS.assetsDir) && !rel.startsWith(DESIGN_SYSTEM_PATHS.fontsDir) && !rel.startsWith(DESIGN_SYSTEM_PATHS.archivedDir) && !GENERATED_API_PATH_PATTERN.test(rel) && /\.(?:mdx?|markdown)$/i.test(rel);
}
function classify(rel) {
  if (rel === DESIGN_SYSTEM_FILE) return "index";
  if (EDITOR_OWN_FILES.includes(rel)) return "own";
  if (rel === DESIGN_SYSTEM_PATHS.manifest || rel === GENERATED_TOKENS_CSS_PATH || rel.startsWith("api/")) return "generated";
  if (rel === DESIGN_SYSTEM_PATHS.readme) return "readme";
  if (rel === DESIGN_SYSTEM_PATHS.tokens) return "tokens";
  if (rel.startsWith(DESIGN_SYSTEM_PATHS.archivedDir)) return "archived";
  if (isSectionDocument(rel)) return "doc";
  if (rel === DESIGN_SYSTEM_PATHS.bundleJs) return "bundleJs";
  if (rel === DESIGN_SYSTEM_PATHS.bundleCss) return "bundleCss";
  if (rel.startsWith(DESIGN_SYSTEM_PATHS.componentsDir)) {
    if (rel.startsWith(DESIGN_SYSTEM_PATHS.libDir)) return "componentLib";
    if (extensionOf(rel) === "d.ts") return "dts";
    const parts = rel.slice(DESIGN_SYSTEM_PATHS.componentsDir.length).split("/");
    if (parts.length === 2 && parts[1] === "preview.html") return "preview";
    if (parts.length === 2 && parts[1] === "README.md") return "componentReadme";
    if (parts[0] === "src" && parts.length >= 2) return "componentSource";
    return "componentOther";
  }
  if (rel.startsWith(DESIGN_SYSTEM_PATHS.fontsDir)) return "font";
  if (rel.startsWith(DESIGN_SYSTEM_PATHS.assetsDir)) return "asset";
  if (extensionOf(rel) === "d.ts") return "dts";
  return "other";
}
function limitOf(kind) {
  switch (kind) {
    case "readme":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.readme, label: "README.md" };
    case "tokens":
      return { bytes: TOKENS_LIMITS.bytes, label: "tokens.json" };
    case "doc":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.other, label: "a section document" };
    case "bundleJs":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.bundleJs, label: "bundle.js" };
    case "bundleCss":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.bundleCss, label: "bundle.css" };
    case "preview":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.preview, label: "a preview.html" };
    case "componentReadme":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.componentReadme, label: "a component's README.md" };
    case "componentLib":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.componentLib, label: "a components/lib/ file" };
    case "componentSource":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.other, label: "a components/src/ file" };
    case "componentOther":
    case "other":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.other, label: "a file the format has no place for" };
    case "asset":
      return { bytes: DESIGN_SYSTEM_FILE_LIMITS.asset, label: "an asset" };
    default:
      return null;
  }
}
async function scanTree2(dir, issues) {
  const tree = { files: /* @__PURE__ */ new Map(), blobs: /* @__PURE__ */ new Set(), totalBytes: 0, truncated: false };
  let seen = 0;
  const visit = async (rel) => {
    const full = rel === "" ? dir : path8.join(dir, ...rel.split("/"));
    const entries = await fs6.readdir(full, { withFileTypes: true }).catch(() => []);
    entries.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
    for (const entry of entries) {
      if (++seen > MAX_SCAN_ENTRIES2) {
        tree.truncated = true;
        return;
      }
      if (entry.name.startsWith(".")) continue;
      const child = rel === "" ? entry.name : `${rel}/${entry.name}`;
      if (rel === "" && entry.name === BLOBS_DIR && entry.isDirectory()) {
        await scanBlobs(path8.join(dir, BLOBS_DIR), tree);
        continue;
      }
      if (entry.isSymbolicLink()) {
        issues.push({ file: child, message: "is a symbolic link; the editor ignores symbolic links. Replace it with a regular file or folder" });
        continue;
      }
      if (child !== DESIGN_SYSTEM_FILE) {
        const reason = explainDesignSystemPath(child);
        if (reason !== null) {
          issues.push({ file: child, message: `${reason}. The editor can't see it` });
          continue;
        }
      }
      if (entry.isDirectory()) await visit(child);
      else if (entry.isFile()) {
        const stat = await fs6.lstat(path8.join(full, entry.name)).catch(() => null);
        const size = stat?.size ?? 0;
        tree.files.set(child, size);
        tree.totalBytes += size;
      }
    }
  };
  await visit("");
  return tree;
}
async function scanBlobs(dir, tree) {
  const entries = await fs6.readdir(dir, { withFileTypes: true }).catch(() => []);
  for (const entry of entries) {
    if (!entry.isFile() || entry.name.startsWith(".")) continue;
    const id = entry.name.split(".")[0];
    if (BLOB_ID.test(id)) tree.blobs.add(id);
  }
}
var isRemote2 = (url) => /^(?:[a-z][a-z0-9+.-]*:)?\/\//i.test(url.trim()) && !/^data:/i.test(url.trim());
var isGoogleFonts2 = (url) => /^(?:https?:)?\/\/fonts\.(?:googleapis|gstatic)\.com\//i.test(url.trim());
function attr2(attrs, name) {
  const match = new RegExp(`(?:^|\\s)${name}\\s*=\\s*(?:'([^']*)'|"([^"]*)"|([^\\s>]+))`, "i").exec(attrs);
  if (!match) return null;
  return match[1] ?? match[2] ?? match[3] ?? null;
}
function checkBlobRefs2(file, text, blobs, issues) {
  const reported = /* @__PURE__ */ new Set();
  for (const match of text.matchAll(/\/_blob\/([^\s"'()<>?#`\\]*)/g)) {
    const id = match[1];
    if (reported.has(id)) continue;
    reported.add(id);
    if (!BLOB_ID.test(id)) {
      issues.push({ file, message: `/_blob/${id} is not a valid upload reference: use the /_blob/<id> that design_system_import_asset returned (32 lowercase hex digits)` });
    } else if (!blobs.has(id)) {
      issues.push({ file, message: `/_blob/${id} is not in ${BLOBS_DIR}/: import the file with design_system_import_asset and use the src it returns` });
    }
  }
}
var NO_CDN = "BaoCut previews load nothing from the network but Google Fonts (no CDN in the page CSP)";
function remoteScriptLoads(code) {
  const urls = [];
  const patterns = [
    /\bimport\s*\(\s*['"`]([^'"`]+)['"`]/g,
    /\bimportScripts\s*\(\s*['"`]([^'"`]+)['"`]/g,
    /\bfetch\s*\(\s*['"`]([^'"`]+)['"`]/g,
    /\.src\s*=\s*['"`]([^'"`]+)['"`]/g,
    /<script\b[^>]*\bsrc\s*=\s*\\?['"]([^'"\\]+)/gi,
    /^\s*import\s+(?:[\w$*{},\s]+\s+from\s+)?['"]([^'"]+)['"]/gm
  ];
  for (const pattern of patterns) {
    for (const match of code.matchAll(pattern)) {
      const url = match[1];
      if (isRemote2(url) && !isGoogleFonts2(url) && !urls.includes(url)) urls.push(url);
    }
  }
  return urls;
}
function remoteCssLoads(css) {
  const urls = [];
  for (const match of css.matchAll(/@import\s+(?:url\(\s*)?["']?([^"')\s;]+)/gi)) urls.push(match[1]);
  for (const match of css.matchAll(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*))\s*\)/gi)) urls.push((match[1] ?? match[2] ?? match[3] ?? "").trim());
  return [...new Set(urls.filter((url) => isRemote2(url) && !isGoogleFonts2(url)))];
}
function checkPreview(file, html, issues) {
  const add = (message) => issues.push({ file, message });
  const text = html.replace(/^\uFEFF/, "");
  if (!CARD_HEADER_PATTERN.test(text)) {
    add('line 1 must be the marker <!-- @dsCard group="\u2026" height=N \u2026 --> (height 40\u20134000); without it the card has no group or height');
  }
  const markup = text.replace(/<!--[\s\S]*?-->/g, (comment) => " ".repeat(comment.length));
  const forbidden = /* @__PURE__ */ new Set();
  for (const match of markup.matchAll(/<([A-Za-z][A-Za-z0-9-]*)\b/g)) {
    const name = match[1].toLowerCase();
    if (FORBIDDEN_PREVIEW_ELEMENTS.includes(name)) forbidden.add(name);
  }
  for (const name of forbidden) add(`uses <${name}>; a preview may not contain <iframe>, <frame>, <object>, <embed>, <portal> or <noscript>`);
  for (const match of markup.matchAll(/<script\b((?:[^>"']|"[^"]*"|'[^']*')*)>([\s\S]*?)<\/script\s*>/gi)) {
    const src = attr2(match[1] ?? "", "src");
    if (src !== null) {
      const clean = src.trim();
      if (isRemote2(clean)) add(`loads the script ${clean} from the network; ${NO_CDN}. Copy the library into components/lib/ and list it in the index's libraries`);
      else if (/(?:^|\/)(?:bundle\.js|lib\/[^/]+\.js|_vendor\/)/.test(clean.split(/[?#]/)[0])) {
        add(`links ${clean}; bundle.js and the libraries are preloaded in every preview, and a preview that links them doesn't load. Remove the <script src>`);
      }
    }
    for (const url of remoteScriptLoads(match[2] ?? "")) add(`its script loads ${url} from the network; ${NO_CDN}`);
  }
  for (const match of markup.matchAll(/<link\b((?:[^>"']|"[^"]*"|'[^']*')*)>/gi)) {
    const href = attr2(match[1] ?? "", "href");
    if (href !== null && isRemote2(href) && !isGoogleFonts2(href)) add(`links ${href.trim()} from the network; ${NO_CDN}`);
  }
  for (const match of markup.matchAll(/<(?:img|source|video|audio)\b((?:[^>"']|"[^"]*"|'[^']*')*)>/gi)) {
    const src = attr2(match[1] ?? "", "src");
    if (src !== null && isRemote2(src)) add(`loads ${src.trim()} from the network; ${NO_CDN}. Put the file under assets/ and use a relative path, or import it with design_system_import_asset`);
  }
  for (const match of markup.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style\s*>/gi)) {
    for (const url of remoteCssLoads(match[1] ?? "")) add(`its style loads ${url} from the network; ${NO_CDN}`);
  }
}
function bundleHeader(script2) {
  const prefix = script2.slice(0, 262144);
  const offset = prefix.indexOf("@ds-bundle:");
  if (offset < 0) return null;
  const start = prefix.lastIndexOf("/*", offset);
  const end = prefix.indexOf("*/", offset);
  if (start < 0 || end < 0 || prefix.slice(0, start).trim() !== "") return "invalid";
  try {
    const header = JSON.parse(prefix.slice(offset + 11, end).trim());
    if (!isRecord3(header)) return "invalid";
    const ns = header.namespace;
    return { namespace: typeof ns === "string" && WINDOW_NAMESPACE_PATTERN.test(ns) ? ns : null };
  } catch {
    return "invalid";
  }
}
function escapeRegExp(text) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
function checkBundle(file, code, namespace, issues) {
  const add = (message) => issues.push({ file, message });
  if (/^\s*import\s*(?:[\w$*{},\s]+\s*from\s*)?['"]|^\s*export\s+(?:default\b|\{|\*|const\b|let\b|var\b|function\b|class\b|async\b)|\bimport\.meta\b|\bimport\s*\(/m.test(code)) {
    add("is not a classic script: it uses import / export; bundle it into ONE classic script (an IIFE) that reads window.React and window.ReactDOM");
  }
  if (/<\/script/i.test(code)) add('contains a literal </script, which ends the inline <script> previews and consumers put it in; write it "<\\/script"');
  if (code.includes("<!--")) add('contains a literal <!--, which can escape the inline <script> previews and consumers put it in; write it "\\x3C!--"');
  if (/\bfetch\s*\(|\bXMLHttpRequest\b|\bnew\s+(?:WebSocket|EventSource)\s*\(|\bsendBeacon\s*\(|\bimportScripts\s*\(/.test(code)) {
    add("makes network requests (fetch / XMLHttpRequest / WebSocket / EventSource / sendBeacon); the bundle fetches nothing");
  }
  const header = bundleHeader(code);
  if (header === "invalid") add("line 1 has an @ds-bundle header that is not /* @ds-bundle: {JSON} */ at the very start; fix it or remove it");
  const declared = header !== null && header !== "invalid" ? header.namespace : null;
  if (declared !== null && namespace !== null && declared !== namespace) {
    add(`its @ds-bundle header names the namespace ${declared}, but design-system.json says ${namespace}; make them the same`);
  }
  const ns = namespace ?? declared;
  if (ns === null) return;
  const body = header !== null && header !== "invalid" ? code.slice(code.indexOf("*/") + 2) : code;
  const name = escapeRegExp(ns);
  if (!new RegExp(`\\.\\s*${name}\\b|\\[\\s*['"\`]${name}['"\`]\\s*\\]|\\bvar\\s+${name}\\b`).test(body)) {
    add(`never assigns window.${ns}: the bundle must set window.${ns} = { \u2026components } (the index's namespace)`);
  }
}
async function checkFont(dir, file, issues) {
  const ext = extensionOf(file);
  const format = FONT_EXTENSIONS[ext];
  if (format === void 0) {
    issues.push({ file, message: "fonts/ holds font files only (woff2, woff, ttf, otf, ttc); move it elsewhere" });
    return;
  }
  const handle = await fs6.open(path8.join(dir, ...file.split("/")), "r").catch(() => null);
  if (handle === null) return;
  try {
    const head = Buffer.alloc(4);
    const { bytesRead } = await handle.read(head, 0, 4, 0);
    const magic = head.subarray(0, bytesRead).toString("latin1");
    const sfnt = magic === "\0\0\0" || magic === "OTTO" || magic === "true" || magic === "typ1";
    const ok = ext === "woff2" ? magic === "wOF2" : ext === "woff" ? magic === "wOFF" : ext === "ttc" ? magic === "ttcf" : sfnt;
    if (!ok) issues.push({ file, message: `is not a ${format} font: its first bytes don't match a .${ext} file. Use the real font file` });
  } finally {
    await handle.close();
  }
}
function isAssetGroupName(name) {
  return typeof name === "string" && name.length > 0 && name.length <= MAX_ASSET_GROUP_NAME_LENGTH && !/[/\\\u0000-\u001f\u007f]/.test(name) && name !== "." && name !== ".." && !RESERVED_PROTOTYPE_NAMES2.has(name) && name.trim() === name;
}
function encodeIndexKey(key) {
  let encoded = "";
  for (const byte of Buffer.from(key, "utf8")) {
    const ch = String.fromCharCode(byte);
    encoded += byte < 128 && /[A-Za-z0-9_./-]/.test(ch) ? ch : `~${byte.toString(16).padStart(2, "0")}`;
  }
  return encoded;
}
function decodeIndexKey(key) {
  if (!key.includes("~")) return key;
  const bytes = [];
  for (let i = 0; i < key.length; i++) {
    const hex = key[i] === "~" ? key.slice(i + 1, i + 3) : "";
    if (/^[0-9a-f]{2}$/.test(hex)) {
      bytes.push(parseInt(hex, 16));
      i += 2;
    } else {
      for (const byte of Buffer.from(key[i], "utf8")) bytes.push(byte);
    }
  }
  return Buffer.from(bytes).toString("utf8");
}
function checkRecord(ctx, where, record, filePath) {
  const { add, shadowed, restored, tree } = ctx;
  if (!isRecord3(record)) {
    add(`${where}: each record is {"name","blob","size","type"}`);
    return;
  }
  const { name, blob, size, type } = record;
  if (typeof name !== "string" || name === "") {
    add(`${where}: "name" must be the file's path (below the group folder for a group's files)`);
    return;
  }
  const target = filePath(name);
  const reason = explainDesignSystemPath(target);
  if (reason !== null) add(`${where}: ${target} ${reason}; the editor skips the record`);
  if (typeof size !== "number" || !Number.isFinite(size) || size < 0) add(`${where}: "size" must be the file's size in bytes`);
  if (typeof type !== "string" || !/^[a-z0-9][a-z0-9!#$&^_.+-]*\/[a-z0-9][a-z0-9!#$&^_.+-]*$/.test(type)) add(`${where}: "type" must be the file's media type`);
  if (reason === null && tree.files.has(target)) {
    shadowed.push(target);
    return;
  }
  if (typeof blob !== "string" || !BLOB_ID.test(blob)) {
    add(`${where}: "blob" must be the upload's id (the 32 hex digits after /_blob/ that design_system_import_asset returned)`);
  } else if (tree.blobs.has(blob)) {
    if (reason === null) restored.add(target);
  } else {
    add(
      `${where}: the upload ${blob} is not in ${BLOBS_DIR}/ and there is no file at ${target}, so the asset can't be shown. Put the file at ${target} (and delete the record), or import it with design_system_import_asset`,
      "ASSET_RECORD_MISSING_UPLOAD"
    );
  }
}
function checkIndex2(index, ctx, note) {
  checkIndexFields(index, ctx, note);
  const { shadowed } = ctx;
  if (shadowed.length > 0) {
    note(
      `${shadowed.length} asset ${shadowed.length === 1 ? "record points" : "records point"} at a path that is a file (${shadowed.slice(0, 3).join(", ")}${shadowed.length > 3 ? ", \u2026" : ""}): the files are used and the records ignored, as in an exported system; the editor drops the records when it next rewrites the index`
    );
  }
}
function checkIndexFields(index, ctx, note) {
  const { add, tree } = ctx;
  if (isRecord3(index.content)) {
    add('has a "content" object: that is the old one-file envelope, not a files index. Move the files out of it into the folder');
  }
  if (!hasDesignSystemMarker(index)) {
    add('has no createdOnFiles or convertedFrom marker: the editor opens it read-only. A system you start gets "createdOnFiles": {"v": 1, "at": "<ISO time>"}');
  }
  if (Object.hasOwn(index, "title") && typeof index.title !== "string") add(`"title" must be a string (the system's name)`);
  else if (typeof index.title !== "string" || index.title.trim() === "") note(`has no "title": the system is shown under its folder name; the title is the system's name`);
  if (Object.hasOwn(index, "namespace") && (typeof index.namespace !== "string" || !WINDOW_NAMESPACE_PATTERN.test(index.namespace))) {
    add(`"namespace" must be the bundle's global: a JavaScript identifier of at most 64 characters (not __proto__, constructor or prototype)`);
  }
  if (Object.hasOwn(index, "libraries") && !Array.isArray(index.libraries)) add('"libraries" must be a list ([] when there is no bundle)');
  else if (Array.isArray(index.libraries)) {
    index.libraries.forEach((library, i) => {
      const where = `libraries[${i}]`;
      if (!isRecord3(library) || typeof library.name !== "string" || typeof library.version !== "string") {
        add(`${where}: each library is {"name","version","global"?,"file"?}`);
        return;
      }
      const file = library.file;
      if (file !== void 0) {
        if (typeof file !== "string" || !file.startsWith(DESIGN_SYSTEM_PATHS.libDir)) add(`${where}: "file" must be a path under ${DESIGN_SYSTEM_PATHS.libDir}`);
        else if (!tree.files.has(file)) add(`${where}: ${file} is not in the system`);
        if (library.global !== void 0 && (typeof library.global !== "string" || !WINDOW_NAMESPACE_PATTERN.test(library.global))) {
          add(`${where}: "global" must be the window global the library defines`);
        }
      } else if (library.name !== "react" && library.name !== "react-dom") {
        add(`${where}: ${library.name} has no "file", so previews can't load it (${NO_CDN}); copy it into ${DESIGN_SYSTEM_PATHS.libDir} and set "file" and "global"`);
      }
    });
  }
  if (Object.hasOwn(index, "sections") && (!isRecord3(index.sections) || Object.values(index.sections).some((v) => typeof v !== "string"))) {
    add('"sections" must be an object of card titles ({} in a new index)');
  }
  if (Object.hasOwn(index, "groups") && (!Array.isArray(index.groups) || index.groups.some((g) => typeof g !== "string"))) {
    add('"groups" must be a list of asset group keys');
  }
  if (Object.hasOwn(index, "assetGroups") && !isRecord3(index.assetGroups)) add('"assetGroups" must be an object keyed by group');
  for (const [key, group] of Object.entries(isRecord3(index.assetGroups) ? index.assetGroups : {})) {
    const where = `assetGroups.${key}`;
    if (!isRecord3(group)) {
      add(`${where}: each group is {"name","order","files","tile"?}`);
      continue;
    }
    const name = group.name === void 0 ? decodeIndexKey(key) : group.name;
    if (!isAssetGroupName(name)) {
      add(`${where}: "name" must be the group folder's name (at most ${MAX_ASSET_GROUP_NAME_LENGTH} characters, no slashes or control characters)`);
      continue;
    }
    if (group.order !== void 0 && (!Array.isArray(group.order) || group.order.some((n) => typeof n !== "string"))) add(`${where}: "order" must be a list of file names`);
    if (group.tile !== void 0 && !ASSET_TILE_SIZES.has(group.tile)) add(`${where}: "tile" is one of "xs", "s", "m", "l", "xl"`);
    if (group.files === void 0) continue;
    if (!isRecord3(group.files)) {
      add(`${where}: "files" must be an object of upload records ({} when the group's files are files)`);
      continue;
    }
    for (const [fileKey, record] of Object.entries(group.files)) {
      checkRecord(ctx, `${where}.files.${fileKey}`, record, (n) => `${DESIGN_SYSTEM_PATHS.assetsDir}${name}/${n}`);
    }
  }
  if (Object.hasOwn(index, "blobs") && !isRecord3(index.blobs)) add('"blobs" must be an object ({} in a new index)');
  for (const [key, record] of Object.entries(isRecord3(index.blobs) ? index.blobs : {})) checkRecord(ctx, `blobs.${key}`, record, (n) => n);
  const docs = index.docs;
  if (docs !== void 0 && (!isRecord3(docs) || docs.sections !== void 0 && (!Array.isArray(docs.sections) || docs.sections.some((s) => typeof s !== "string")) || docs.readme !== void 0 && typeof docs.readme !== "string")) {
    add('"docs" must be {"readme"?, "sections": [\u2026]} ({"sections": []} in a new index)');
  }
}
async function readText(dir, rel) {
  return fs6.readFile(path8.join(dir, ...rel.split("/")), "utf8").catch(() => null);
}
async function checkDesignSystem(dir) {
  const issues = [];
  const notes = [];
  const tree = await scanTree2(dir, issues);
  const notChecked = [...DESIGN_SYSTEM_NOT_CHECKED];
  let index = null;
  const indexText = tree.files.has(DESIGN_SYSTEM_FILE) ? await readText(dir, DESIGN_SYSTEM_FILE) : null;
  const addIndex = (message, code) => issues.push({ file: DESIGN_SYSTEM_FILE, message, ...code ? { code } : {} });
  const noteIndex = (message) => notes.push({ file: DESIGN_SYSTEM_FILE, message });
  if (indexText === null) {
    addIndex('missing: a design system needs design-system.json with "v": 3, "layout": "files" and a createdOnFiles marker, written last');
  } else {
    let parsed;
    try {
      parsed = JSON.parse(indexText);
    } catch (error) {
      addIndex(`is not valid JSON: ${error.message}`);
    }
    const envelope = isRecord3(parsed) && isRecord3(parsed.content);
    const fields = isRecord3(parsed) && (envelope || isDesignSystemIndex(parsed)) ? parsed : null;
    if (parsed !== void 0 && (!fields || typeof fields.v !== "number" || fields.v < 3 || fields.layout !== "files")) {
      addIndex('must be an object with "v": 3 and "layout": "files"');
    }
    if (fields) index = fields;
    if (indexText.length > MAX_DESIGN_SYSTEM_INDEX_CHARS) addIndex(`has ${indexText.length} characters; the editor reads at most ${MAX_DESIGN_SYSTEM_INDEX_CHARS}`);
  }
  const restored = /* @__PURE__ */ new Set();
  if (index) checkIndex2(index, { tree, add: addIndex, shadowed: [], restored }, noteIndex);
  const namespace = index && typeof index.namespace === "string" && WINDOW_NAMESPACE_PATTERN.test(index.namespace) ? index.namespace : null;
  let ownFiles = 0;
  let dtsTotal = 0;
  const sections = [];
  for (const [file, bytes] of tree.files) {
    const kind = classify(file);
    ownFiles++;
    if (bytes > DESIGN_SYSTEM_LIMITS.fileBytes) {
      issues.push({ file, message: `is ${bytes} bytes; a file can be at most ${DESIGN_SYSTEM_LIMITS.fileBytes} (15 MiB)` });
      continue;
    }
    if (kind === "dts") dtsTotal += bytes;
    if (kind === "doc") sections.push(file);
    const limit = limitOf(kind);
    if (limit !== null && bytes > limit.bytes) issues.push({ file, message: `is ${bytes} bytes; ${limit.label} can be at most ${limit.bytes}` });
  }
  if (tree.truncated || ownFiles > DESIGN_SYSTEM_LIMITS.ownFiles) {
    issues.push({
      file: ".",
      message: `${tree.truncated ? `has more than ${MAX_SCAN_ENTRIES2}` : `has ${ownFiles}`} files outside ${BLOBS_DIR}/; a design system holds at most ${DESIGN_SYSTEM_LIMITS.ownFiles} (the editor lists only the first ${DESIGN_SYSTEM_LIMITS.files})`
    });
  }
  if (tree.totalBytes > DESIGN_SYSTEM_LIMITS.totalBytes) {
    issues.push({ file: ".", message: `files outside ${BLOBS_DIR}/ add up to ${tree.totalBytes} bytes; a design system holds at most ${DESIGN_SYSTEM_LIMITS.totalBytes} (256 MiB)` });
  }
  if (dtsTotal > DESIGN_SYSTEM_FILE_LIMITS.dtsTotal) {
    issues.push({ file: DESIGN_SYSTEM_PATHS.componentsDir.slice(0, -1), message: `the .d.ts files add up to ${dtsTotal} bytes; at most ${DESIGN_SYSTEM_FILE_LIMITS.dtsTotal} (1.5 MB)` });
  }
  if (sections.length > MAX_SECTION_DOCUMENTS) {
    issues.push({ file: ".", message: `has ${sections.length} section documents (*.md outside components/, assets/, fonts/ and archived/); the editor shows at most ${MAX_SECTION_DOCUMENTS}, the rest stay files` });
  }
  for (const file of tree.files.keys()) {
    if (!file.startsWith(DESIGN_SYSTEM_PATHS.componentsDir)) continue;
    const parts = file.slice(DESIGN_SYSTEM_PATHS.componentsDir.length).split("/");
    if (parts.length === 2 && (parts[1] === "preview.html" || parts[1] === "README.md") && !COMPONENT_NAME_PATTERN.test(parts[0])) {
      issues.push({ file, message: `components/${parts[0]}/ is not a component name: an ASCII JavaScript identifier of at most 64 characters` });
    }
  }
  const tokensText = tree.files.has(DESIGN_SYSTEM_PATHS.tokens) ? await readText(dir, DESIGN_SYSTEM_PATHS.tokens) : null;
  if (tokensText !== null && (tree.files.get(DESIGN_SYSTEM_PATHS.tokens) ?? 0) <= TOKENS_LIMITS.bytes) {
    const tokens = checkTokensText(tokensText);
    for (const message of tokens.problems) issues.push({ file: DESIGN_SYSTEM_PATHS.tokens, message });
    for (const message of tokens.notes) notes.push({ file: DESIGN_SYSTEM_PATHS.tokens, message });
  }
  const listedFonts = new Set(tokensText === null ? [] : tokenFontFiles(tokensText));
  for (const font of listedFonts) {
    if (!tree.files.has(font) && !restored.has(font)) issues.push({ file: DESIGN_SYSTEM_PATHS.tokens, message: `type.fonts names ${font}, which is not in the system` });
  }
  for (const file of tree.files.keys()) {
    if (classify(file) !== "font") continue;
    await checkFont(dir, file, issues);
    if (FONT_EXTENSIONS[extensionOf(file)] !== void 0 && !listedFonts.has(file)) {
      issues.push({ file, message: 'is not listed in tokens.json type.fonts, so it gets no @font-face; list it ({"family","file","weight","style"})' });
    }
  }
  for (const file of tree.files.keys()) {
    if (!file.startsWith(DESIGN_SYSTEM_PATHS.assetsDir)) continue;
    const rest = file.slice(DESIGN_SYSTEM_PATHS.assetsDir.length);
    const slash = rest.indexOf("/");
    if (slash > 0 && !isAssetGroupName(rest.slice(0, slash))) {
      issues.push({ file, message: `assets/${rest.slice(0, slash)}/ is not a valid group name (at most ${MAX_ASSET_GROUP_NAME_LENGTH} characters, no leading or trailing spaces)` });
    }
  }
  for (const [file, bytes] of tree.files) {
    const kind = classify(file);
    if (kind === "generated" || kind === "own" || kind === "archived" || bytes > MAX_READ_BYTES || bytes > DESIGN_SYSTEM_LIMITS.fileBytes) continue;
    if (!TEXT_EXTENSIONS2.has(extensionOf(file)) && kind !== "componentLib") continue;
    const text = await readText(dir, file);
    if (text === null) continue;
    if (kind !== "componentLib" && kind !== "bundleJs") checkBlobRefs2(file, text, tree.blobs, issues);
    if (kind === "preview") checkPreview(file, text, issues);
    else if (kind === "bundleJs") checkBundle(file, text, namespace, issues);
    else if (kind === "bundleCss") {
      if (/<\/style/i.test(text)) issues.push({ file, message: "contains a literal </style, which ends the inline <style> previews put it in" });
      for (const url of remoteCssLoads(text)) issues.push({ file, message: `loads ${url} from the network; ${NO_CDN}` });
    } else if (kind === "componentLib" && /\.m?js$/i.test(file)) {
      if (/<\/script/i.test(text)) issues.push({ file, message: "contains a literal </script, which ends the inline <script> previews put it in" });
      for (const url of remoteScriptLoads(text)) issues.push({ file, message: `loads ${url} from the network; ${NO_CDN}. Copy what it loads into components/lib/` });
    }
  }
  if (tree.files.has(DESIGN_SYSTEM_PATHS.bundleJs) && namespace === null && index !== null) {
    const bundle = await readText(dir, DESIGN_SYSTEM_PATHS.bundleJs);
    const header = bundle === null ? null : bundleHeader(bundle);
    if (header === null || header === "invalid" || header.namespace === null) {
      addIndex(`has no "namespace", and bundle.js has no @ds-bundle header naming one: set "namespace" to the bundle's window global`);
    }
  }
  return { title: index ? designSystemTitle(index, dir) : path8.basename(dir), namespace, issues, notes, notChecked };
}

var COMMENTS_FILE = "comments.json";
var COMMENTS_VERSION = 1;
var LIMITS = { text: 1e4, label: 1e3, detail: 4e3, anchor: 2048, section: 200, name: 100 };
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

import fs7 from "node:fs";
import fsp from "node:fs/promises";
import path9 from "node:path";
var LOCK_STALE_MS = 1e4;
var LOCK_WAIT_MS = 5e3;
var LOCK_RETRY_MS = 20;
var commentsPath = (root) => path9.join(root, COMMENTS_FILE);
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
  const temp = path9.join(path9.dirname(file), `.${path9.basename(file)}.${process.pid}.${Date.now().toString(36)}.tmp`);
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
    const exists = fs7.existsSync(commentsPath(root));
    if (after !== before && (exists || doc.threads.length > 0)) await writeAtomic(commentsPath(root), after);
    return result;
  } finally {
    await unlock();
  }
}

import fs8 from "node:fs";
import net from "node:net";
import path10 from "node:path";
var STATE_FILE = ".artifact-server.json";
var LOG_FILE = ".artifact-server.log";
var EXPORTS_DIR = "exports";
function readState(root) {
  try {
    return JSON.parse(fs8.readFileSync(path10.join(root, STATE_FILE), "utf8"));
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
var DEFAULT_PORTS = { slides: 4710, design: 4720, "design-system": 4730, dashboard: 4740, docs: 4750, motion: 4760 };
var PORT_TRIES = 100;
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
function serverOrigin(host, port) {
  const shown = host === "0.0.0.0" || host === "::" ? "127.0.0.1" : host.includes(":") ? `[${host}]` : host;
  return `http://${shown}:${port}`;
}

import { spawn } from "node:child_process";
import fs10 from "node:fs";
import path12 from "node:path";

import crypto from "node:crypto";
import fs9 from "node:fs";
import path11 from "node:path";
var GUEST_ROLES = ["editor", "commenter", "viewer"];
var isGuestRole = (value) => typeof value === "string" && GUEST_ROLES.includes(value);
var SHARING_FILE = ".sharing.json";
var MAX_GUESTS = 200;
var MAX_NAME = 40;
var token = (bytes = 18) => crypto.randomBytes(bytes).toString("base64url");
var isRecord5 = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
function fresh() {
  return { lan: false, links: { editor: { token: token(), on: true }, commenter: { token: token(), on: true }, viewer: { token: token(), on: true } }, guests: [] };
}
function cleanName(raw) {
  const name = typeof raw === "string" ? raw.replace(/[\u0000-\u001f\u007f@<>]/g, "").replace(/\s+/g, " ").trim() : "";
  return [...name].slice(0, MAX_NAME).join("");
}
var Sharing = class _Sharing {
  file;
  state;
  constructor(root) {
    this.file = path11.join(root, SHARING_FILE);
    this.state = _Sharing.read(this.file);
  }
  static read(file) {
    let raw;
    try {
      raw = JSON.parse(fs9.readFileSync(file, "utf8"));
    } catch {
      return fresh();
    }
    const state = fresh();
    if (!isRecord5(raw)) return state;
    state.lan = raw.lan === true;
    if (typeof raw.port === "number" && Number.isInteger(raw.port) && raw.port > 0 && raw.port < 65536) state.port = raw.port;
    if (isRecord5(raw.links)) {
      for (const role of GUEST_ROLES) {
        const link = raw.links[role];
        if (isRecord5(link) && typeof link.token === "string" && link.token.length >= 16) state.links[role] = { token: link.token, on: link.on !== false };
      }
    }
    if (Array.isArray(raw.guests)) {
      for (const g of raw.guests) {
        if (!isRecord5(g) || typeof g.id !== "string" || typeof g.secret !== "string" || !isGuestRole(g.role) || typeof g.link !== "string") continue;
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
    fs9.writeFileSync(tmp, `${JSON.stringify(this.state, null, 2)}
`, { mode: 384 });
    fs9.renameSync(tmp, this.file);
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

var MAX_MESSAGE = 64 * 1024 * 1024;

var HUB_ROUTE = "/__hub";
function buildOf(script2) {
  const real = fs10.realpathSync(script2);
  const { mtimeMs, size } = fs10.statSync(real);
  return `${real}:${mtimeMs}:${size}`;
}
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
  fs10.rmSync(path12.join(state.root, STATE_FILE), { force: true });
  return true;
}
async function openOnHub(call) {
  if (await loopbackBlocked(call.port || 1)) throw new NetworkBlocked();
  const joined = await joinHub(call);
  if (joined) return { state: joined };
  const logFile = path12.join(call.root, LOG_FILE);
  const fd = fs10.openSync(logFile, "a");
  const child = spawn(process.execPath, [...call.command, "--background"], { detached: true, stdio: ["ignore", "ignore", fd] });
  child.unref();
  fs10.closeSync(fd);
  let exited = false;
  child.once("exit", () => exited = true);
  const deadline = Date.now() + 15e3;
  while (Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 120));
    const state = await call.running();
    if (state) return { state };
    if (exited && !readState(call.root)) break;
  }
  return { tail: fs10.existsSync(logFile) ? fs10.readFileSync(logFile, "utf8").split("\n").slice(-12).join("\n") : "" };
}

import fs11 from "node:fs";
import fsp2 from "node:fs/promises";
import path13 from "node:path";
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
  const folder = path13.basename(path13.resolve(root)) || "artifact";
  const skipped = new Set(skip.map((file) => path13.resolve(file)));
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
      const file = path13.join(dir, name);
      if (skipped.has(file)) continue;
      const inner = rel ? `${rel}/${name}` : name;
      if (dirent.isDirectory()) await walk2(file, inner);
      else if (dirent.isFile()) {
        const stat = await fsp2.stat(file).catch(() => null);
        if (stat?.isFile()) entries.push({ name: `${folder}/${inner}`, file, size: stat.size, mode: stat.mode & 511, mtime: stat.mtime });
      }
    }
  };
  await walk2(path13.resolve(root), "");
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
    if (data.length > 0 && !STORED.has(path13.extname(entry.name).toLowerCase())) {
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
  return `${path13.basename(path13.resolve(root)) || "artifact"}-${day}.zip`;
}
async function writeArchive(root, out) {
  const file = path13.resolve(out);
  const entries = await archiveEntries(root, [file]);
  const tmp = path13.join(path13.dirname(file), `.${path13.basename(file)}.${process.pid}.tmp`);
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
  const where = out ? path13.resolve(out) : path13.dirname(path13.resolve(root));
  const target = fs11.statSync(where, { throwIfNoEntry: false })?.isDirectory() ? path13.join(where, archiveFileName(root)) : where;
  const { file, files, bytes } = await writeArchive(root, target);
  return { file, folder: path13.basename(path13.resolve(root)), files, bytes, leftOut: [...LOCAL_FILES] };
}

import fs12 from "node:fs";
import path14 from "node:path";
var FOLDER = /^[a-z0-9][a-z0-9_-]{0,63}$/;
var MAX_FILES = 2e3;
var MAX_FILE_BYTES = 64 * 1024 * 1024;
var DESIGN_SYSTEM_FILE2 = "design-system.json";
function readRecords(project, indexFile) {
  let index;
  try {
    index = JSON.parse(fs12.readFileSync(path14.join(project, indexFile), "utf8"));
  } catch {
    return [];
  }
  const list = index?.designSystems;
  const records = Array.isArray(list) ? list : list && typeof list === "object" ? Object.values(list) : [];
  const out = [];
  for (const raw of records) {
    const r = raw;
    if (typeof r?.namespace !== "string" || !FOLDER.test(r.namespace)) continue;
    out.push({
      namespace: r.namespace,
      title: typeof r.title === "string" ? r.title : r.namespace,
      artifact: typeof r.artifact === "string" ? r.artifact : ""
    });
  }
  return out;
}
function plain(p, want) {
  const stat = fs12.lstatSync(p, { throwIfNoEntry: false });
  return Boolean(stat && !stat.isSymbolicLink() && (want === "file" ? stat.isFile() : stat.isDirectory()));
}
function walk(dir, prefix = "", out = []) {
  for (const entry of fs12.readdirSync(path14.join(dir, prefix), { withFileTypes: true })) {
    if (out.length >= MAX_FILES) break;
    const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) walk(dir, rel, out);
    else if (entry.isFile()) out.push(rel);
  }
  return out;
}
var projectOf = (candidate) => path14.basename(candidate) === "project" ? candidate : path14.join(candidate, "project");
function sourceProject(artifact, root, from) {
  const given = from ?? artifact;
  if (!given) return { reason: "the record names no source folder", kind: "none" };
  if (!from && /^[a-z][a-z0-9+.-]*:\/\//i.test(given)) return { reason: `its source is ${given}, which is not a folder on this machine`, kind: "remote" };
  const candidates = path14.isAbsolute(given) ? [given] : [path14.resolve(given), path14.resolve(root, given), path14.resolve(root, "..", given)];
  for (const candidate of candidates) {
    const project = projectOf(candidate);
    if (plain(project, "dir")) return { project };
  }
  if (!from && !path14.isAbsolute(given)) {
    let dir = path14.resolve(root, "..");
    for (let up = path14.dirname(dir); up !== dir; up = path14.dirname(dir)) {
      dir = up;
      const project = projectOf(path14.resolve(dir, given));
      if (plain(path14.join(project, DESIGN_SYSTEM_FILE2), "file")) return { project };
    }
  }
  return { reason: `its source folder ${given} was not found${from ? "" : ` (tried from ${process.cwd()} and the folders above the artifact); pass --from <folder>`}`, kind: "not-found" };
}
function compareSystem(project, root, record, from) {
  const diff = { namespace: record.namespace, title: record.title, changed: [], removed: [] };
  const found = sourceProject(record.artifact, root, from);
  if (!found.project) return { ...diff, skipped: found.reason, skippedKind: found.kind };
  diff.source = path14.dirname(found.project);
  const copy = path14.join(project, "ds", record.namespace);
  if (!plain(copy, "dir")) return { ...diff, missing: true };
  for (const rel of walk(copy)) {
    const mine = path14.join(copy, rel);
    const theirs = path14.join(found.project, rel);
    if (!plain(mine, "file")) continue;
    if (!plain(theirs, "file")) {
      diff.removed.push(rel);
      continue;
    }
    const a = fs12.statSync(mine);
    const b = fs12.statSync(theirs);
    if (a.size > MAX_FILE_BYTES || b.size > MAX_FILE_BYTES) continue;
    if (a.size !== b.size || !fs12.readFileSync(mine).equals(fs12.readFileSync(theirs))) diff.changed.push(rel);
  }
  return diff;
}
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
function staleNotes(project, root, indexFile, command) {
  const notes = [];
  for (const diff of compareSystems(project, root, indexFile)) {
    if (diff.changed.length === 0 && diff.removed.length === 0) continue;
    const parts = [];
    if (diff.changed.length > 0) parts.push(`${diff.changed.length} changed (${diff.changed.slice(0, 3).join(", ")}${diff.changed.length > 3 ? ", \u2026" : ""})`);
    if (diff.removed.length > 0) parts.push(`${diff.removed.length} gone from the source`);
    notes.push(
      `project/ds/${diff.namespace}/ is behind its source system ${diff.source}: ${parts.join(", ")}. A copy never changes by itself: run \`${command(diff.namespace, false)}\` to copy the changed files again (\`${command(diff.namespace, true)}\` lists them).`
    );
  }
  return notes;
}
function stamp(project, indexFile, namespaces, now) {
  const file = path14.join(project, indexFile);
  const text = fs12.readFileSync(file, "utf8");
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
  fs12.writeFileSync(tmp, text.endsWith("\n") ? `${out}
` : out);
  fs12.renameSync(tmp, file);
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
        const target = path14.join(project, "ds", diff.namespace, rel);
        const tmp = `${target}.${process.pid}.tmp`;
        fs12.copyFileSync(path14.join(diff.source, "project", rel), tmp);
        fs12.renameSync(tmp, target);
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
  const shown = (list) => Object.fromEntries(Object.entries(list).map(([role, link]) => [role, link.on ? link.url : "off"]));
  const tailnet = view.lan ? view.tailnet : void 0;
  return {
    lan: view.lan,
    ...view.lan ? { addresses: lanUrls(view) } : {},
    links: shown(links),
    ...tailnet ? {
      tailscale: shown(tailnet.links),
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

var emitWarning = process.emitWarning.bind(process);
process.emitWarning = ((warning, ...rest) => {
  if (String(typeof warning === "string" ? warning : warning.message).includes("localStorage")) return;
  emitWarning(warning, ...rest);
});
var KIND = "design";
var SCRIPT = fileURLToPath2(import.meta.url);
var EDITOR_DIR = path15.join(path15.dirname(SCRIPT), "editor");
var SERVER_SCRIPT = path15.join(path15.dirname(SCRIPT), "server.mjs");
var MAX_LISTED = 200;
var UPLOAD_MAX_BYTES = 20 * 1024 * 1024;
var INDEX_FILE = KIND === "slides" ? "deck.json" : KIND === "design" ? CANVAS_FILE : DESIGN_SYSTEM_FILE;
var LAYOUT = KIND === "slides" ? DECK_LAYOUT : KIND === "design" ? CANVAS_LAYOUT : DESIGN_SYSTEM_LAYOUT;
var NOUN = KIND === "slides" ? "deck" : KIND === "design" ? "design" : "design system";
var CliError = class extends Error {
  code;
  constructor(code, message) {
    super(message);
    this.code = code;
  }
};
var SWITCHES = /* @__PURE__ */ new Set(["all", "wait", "resolve", "reopen", "lan", "off", "check"]);
function parseArgs(argv) {
  const positional = [];
  const flags = /* @__PURE__ */ new Map();
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg.startsWith("--")) {
      const eq = arg.indexOf("=");
      if (eq > 0) flags.set(arg.slice(2, eq), arg.slice(eq + 1));
      else if (SWITCHES.has(arg.slice(2))) flags.set(arg.slice(2), "true");
      else if (i + 1 < argv.length && !argv[i + 1].startsWith("--")) flags.set(arg.slice(2), argv[++i]);
      else flags.set(arg.slice(2), "true");
    } else positional.push(arg);
  }
  return { positional, flags };
}
function print(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}
`);
}
var script = (cmd, ...args) => ["node", quote(SCRIPT), cmd, ...args.map(quote)].join(" ");
function quote(s) {
  return /^[A-Za-z0-9_./:@%+=-]+$/.test(s) ? s : `'${s.replace(/'/g, `'\\''`)}'`;
}
function resolveArtifact(given, mustExist = true) {
  if (!given) throw new CliError("MISSING_ROOT", `Give the artifact folder: the one that holds project/${INDEX_FILE}.`);
  let root = path15.resolve(given);
  if (path15.basename(root) === "project" && fs13.existsSync(path15.join(root, INDEX_FILE)) && !fs13.existsSync(path15.join(root, "project"))) {
    root = path15.dirname(root);
  }
  const project = path15.join(root, "project");
  if (mustExist) {
    const stat = fs13.statSync(project, { throwIfNoEntry: false });
    if (!stat?.isDirectory()) {
      throw new CliError("NOT_FOUND", `${project} does not exist. Write project/${INDEX_FILE} and the other files under ${root} first.`);
    }
  }
  return { root, project };
}
async function readInfo(project) {
  if (KIND === "slides") return await readDeckSummary(project);
  if (KIND === "design") return await readCanvasInfo(project);
  return await readDesignSystemInfo(project);
}
function localize(value) {
  const text = JSON.stringify(value).replace(/\b(?:design_system|design|slides)_import_asset\b/g, "artifact.mjs upload").replace(/\b(?:design_system|design|slides)_check\b/g, "artifact.mjs check").replace(
    /BaoCut previews load nothing from the network but Google Fonts \(no CDN in the page CSP\)/g,
    "previews here load nothing from the network but Google Fonts: make the library a file in components/lib/ with a libraries entry (format.md); React 18 comes with the page"
  );
  return JSON.parse(text);
}
function designSystemNotes(project, indexFile) {
  let index;
  try {
    index = JSON.parse(fs13.readFileSync(path15.join(project, indexFile), "utf8"));
  } catch {
    return [];
  }
  const list = index?.designSystems;
  const records = Array.isArray(list) ? list : list && typeof list === "object" ? Object.values(list) : [];
  const notes = [];
  for (const record of records) {
    const ns = record?.namespace;
    if (typeof ns !== "string") continue;
    if (!/^[a-z0-9][a-z0-9_-]{0,63}$/.test(ns)) {
      notes.push(`designSystems namespace ${JSON.stringify(ns)} is not a folder name ([a-z0-9][a-z0-9_-]{0,63}): the page skips it. Use the lower-case folder (list prints it as folder), the same in the record and in project/ds/<folder>/.`);
      continue;
    }
    const folder = path15.join(project, "ds", ns);
    if (!fs13.existsSync(folder)) {
      notes.push(`designSystems names "${ns}" but project/ds/${ns}/ does not exist: copy the system's project/tokens.json (and the files you use) there, or take the record out.`);
    } else if (!fs13.existsSync(path15.join(folder, "tokens.json"))) {
      notes.push(`project/ds/${ns}/ has no tokens.json, so the Theme and color menus show bare hexes: copy the system's project/tokens.json there (a system with none: nothing to do).`);
    }
  }
  const root = path15.dirname(project);
  return [...notes, ...staleNotes(project, root, indexFile, (ns, listOnly) => script("ds-sync", root, ns, ...listOnly ? ["--check"] : []))];
}
async function check(a) {
  if (KIND === "slides") {
    let rt;
    try {
      rt = loadSlidesCompiler(EDITOR_DIR);
    } catch {
      throw new CliError("COMPILER_MISSING", `The slides compiler is missing (${path15.join(EDITOR_DIR, "node", "compiler.cjs")}); reinstall the skill.`);
    }
    if (!fs13.existsSync(path15.join(a.project, "deck.json"))) {
      return {
        problems: 1,
        result: {
          root: a.root,
          slideCount: 0,
          problems: 1,
          deckProblems: ['deck.json: missing: a deck needs project/deck.json with "v": 4, "title" and "order" (Creating, step 4)'],
          next: "Write project/deck.json and the slides, then run check again until problems is 0."
        }
      };
    }
    let report;
    try {
      report = checkDeck(rt, a.project, { verbose: true });
    } catch (error) {
      throw new CliError("CHECK_FAILED", `The check did not finish: ${error.message}. Is project/deck.json valid JSON?`);
    }
    const problems2 = problemCount(report);
    const slides = report.slides.filter((s) => s.problems.length > 0 || s.notes.length > 0 || s.roundTripDrift).map((s) => ({
      id: s.id,
      ...s.problems.length > 0 ? { problems: s.problems } : {},
      ...s.notes.length > 0 ? { notes: s.notes } : {},
      ...s.roundTripDrift ? { roundTripDrift: true } : {}
    }));
    const layoutNotes = slides.some((s) => s.notes?.some((n) => n.startsWith("\u6392\u7248") || n.startsWith("\u8B66\u544A")));
    const dsNotes = designSystemNotes(a.project, "deck.json");
    return {
      problems: problems2,
      result: {
        root: a.root,
        title: report.title,
        slideCount: report.slides.length,
        problems: problems2,
        ...report.deckProblems.length > 0 ? { deckProblems: report.deckProblems } : {},
        ...report.orphans.length > 0 ? { orphans: report.orphans } : {},
        ...slides.length > 0 ? { slides } : {},
        ...dsNotes.length > 0 ? { designSystemNotes: dsNotes } : {},
        notChecked: "Text sizes, contrast, whether a section id matches its file name and whether an image path exists are not checked (nothing is rendered).",
        next: problems2 > 0 ? `Fix the files named in deckProblems and in each slide's problems (paths are under project/), then run check again until problems is 0. notes are hints and lose no content; fix the layout ones (starting "\u6392\u7248") and the warnings (starting "\u8B66\u544A": a font or image falls back) too.` : `No problems. Show it to the user: ${script("open", a.root)} prints the editor URL.` + (layoutNotes ? ' Some slides carry layout notes ("\u6392\u7248": content may overflow or overlap) or warnings ("\u8B66\u544A": a font or image falls back); fix them and check again.' : "") + (report.orphans.length > 0 ? " orphans are slide files missing from order: they do not show; delete the unneeded ones." : "") + (dsNotes.length > 0 ? " Read designSystemNotes: a design system install looks incomplete or behind its source." : "")
      }
    };
  }
  if (KIND === "design") {
    const r2 = await checkCanvas(a.project);
    const problems2 = r2.issues.length;
    const dsNotes = designSystemNotes(a.project, CANVAS_FILE);
    return {
      problems: problems2,
      result: {
        root: a.root,
        title: r2.title,
        boardCount: r2.boardCount,
        problems: problems2,
        ...problems2 > 0 ? { issues: r2.issues.slice(0, MAX_LISTED) } : {},
        ...problems2 > MAX_LISTED ? { more: `${problems2 - MAX_LISTED} more not listed: fix these, then check again` } : {},
        ...dsNotes.length > 0 ? { designSystemNotes: dsNotes } : {},
        notChecked: "Whether the artboard scripts run, and how the layout looks, is not checked (nothing is rendered).",
        next: problems2 > 0 ? 'Fix the files per issues (file is a path under project/; "." is the whole folder), then run check again until problems is 0.' : `No problems. Show it to the user: ${script("open", a.root)} prints the editor URL.` + (dsNotes.length > 0 ? " Read designSystemNotes: a design system install looks incomplete or behind its source." : "")
      }
    };
  }
  const r = await checkDesignSystem(a.project);
  const problems = r.issues.length;
  if (fs13.existsSync(path15.join(a.project, DESIGN_SYSTEM_FILE))) {
    if (!fs13.existsSync(path15.join(a.project, "README.md"))) {
      r.notes.push({ file: "README.md", message: "missing: every system includes project/README.md; every reader starts there (Creating, step 1)" });
    }
    if (!fs13.existsSync(path15.join(a.project, "components", "Cover", "preview.html"))) {
      r.notes.push({ file: "components/Cover/preview.html", message: "missing: every system gets a cover, written last before the index (cover.md)" });
    }
  }
  return {
    problems,
    result: {
      root: a.root,
      title: r.title,
      namespace: r.namespace,
      problems,
      ...problems > 0 ? { issues: r.issues.slice(0, MAX_LISTED) } : {},
      ...r.notes.length > 0 ? { notes: r.notes.slice(0, 50) } : {},
      ...problems > MAX_LISTED ? { more: `${problems - MAX_LISTED} more not listed: fix these, then check again` } : {},
      notChecked: `${r.notChecked.join(", ")} are generated by the editor when it saves and are not checked; whether components render is not checked (nothing is rendered).`,
      next: problems > 0 ? 'Fix the files per issues (file is a path under project/; "." is the whole folder), then run check again until problems is 0. Read the notes too: most need no change, but one naming a missing README or cover, or a file that hides an asset record, means a file to write or remove.' : `No problems. ${r.notes.length > 0 ? "Read the notes: most need no change, but one naming a missing README or cover, or a file that hides an asset record, means a file to write or remove. " : ""}Show it to the user: ${script("open", a.root)} prints the editor URL.`
    }
  };
}
var DESIGN_SYSTEM_UPLOAD_EXTENSIONS = /* @__PURE__ */ new Set(["png", "jpg", "jpeg", "gif", "webp", "svg", "mp4", "webm", "pdf", "woff2", "woff", "ttf", "otf"]);
var CANVAS_TEXT_UPLOADS = /* @__PURE__ */ new Set(["css", "js", "mjs", "json"]);
function formatBytes(n) {
  return n >= 1024 * 1024 ? `${(n / 1024 / 1024).toFixed(1)} MB` : `${Math.ceil(n / 1024)} KB`;
}
async function upload(a, given, group) {
  if (!given) throw new CliError("MISSING_FILE", "Give the file to upload.");
  const file = path15.resolve(given);
  const stat = await fsp3.stat(file).catch(() => null);
  if (!stat) throw new CliError("FILE_NOT_FOUND", `No such file: ${given}`);
  if (!stat.isFile()) throw new CliError("NOT_A_FILE", `${given} is not a regular file.`);
  const ext = path15.extname(file).slice(1).toLowerCase();
  const type = contentTypeOf(file);
  const isMedia = type.startsWith("image/") || type.startsWith("font/");
  let limit = UPLOAD_MAX_BYTES;
  if (KIND === "slides") {
    if (!isMedia) throw new CliError("UNSUPPORTED_TYPE", "Only images (png, jpg, gif, webp, avif, svg) and fonts (woff2, woff, ttf, otf) can be uploaded, judged by extension. Convert other images to png first.");
  } else if (KIND === "design") {
    if (!isMedia && !CANVAS_TEXT_UPLOADS.has(ext)) {
      throw new CliError("UNSUPPORTED_TYPE", "Only images (png, jpg, gif, webp, avif, svg), fonts (woff2, woff, ttf, otf) and .css .js .json files can be uploaded, judged by extension.");
    }
  } else {
    if (!DESIGN_SYSTEM_UPLOAD_EXTENSIONS.has(ext)) {
      throw new CliError("UNSUPPORTED_TYPE", `Only ${[...DESIGN_SYSTEM_UPLOAD_EXTENSIONS].join(", ")} files can be uploaded, judged by extension. Text files (README.md, .json, .css, .js) are files, not uploads.`);
    }
    if (group === void 0) {
      throw new CliError("MISSING_GROUP", "Give --group <Group>: a design system's uploads sit in an asset group (assets/<Group>/<file>). Fonts are files: copy them to project/fonts/ instead.");
    }
    if (!isAssetGroupName(group)) {
      throw new CliError("INVALID_GROUP", "--group is an asset group folder name: at most 80 characters, no / \\ or control characters, no surrounding spaces, not . or ..");
    }
    limit = designSystemUploadLimit(type);
  }
  if (stat.size > limit) throw new CliError("FILE_TOO_LARGE", `${given} is ${formatBytes(stat.size)}, over the ${formatBytes(limit)} limit: compress or shrink it first.`);
  const bytes = await fsp3.readFile(file);
  const uploaded = await new DeckStore(a.project, LAYOUT).putBlob(bytes, type);
  const base = path15.basename(file);
  if (KIND !== "design-system") {
    return {
      url: uploaded.url,
      id: uploaded.id,
      bytes: uploaded.sizeBytes,
      contentType: type,
      next: KIND === "slides" ? type.startsWith("font/") ? 'Put url verbatim as the src of its face in project/deck.json "faces", then run check once the deck is written.' : 'Put url verbatim in <img src="\u2026">, then run check once project/deck.json is written.' : type.startsWith("font/") ? "Use url verbatim in an @font-face src: url(\u2026) in the <helmet><style> of the artboards that use it, then run check once project/canvas.json is written." : "Use url verbatim where this file is named (<img src>, url(\u2026) in a <helmet><style> rule, fetch(\u2026), <link href>, <script src>), never its filename, a path to it or a data: URI; then run check once project/canvas.json is written. (A design system's files are not uploads: they are copied under project/ds/<folder>/ and linked relatively.)"
    };
  }
  const groupName = group;
  const key = encodeIndexKey(base);
  const groupKey = encodeIndexKey(groupName);
  const record = { name: base, blob: uploaded.id, size: uploaded.sizeBytes, type };
  const shownAs = `${DESIGN_SYSTEM_PATHS.assetsDir}${groupName}/${base}`;
  return {
    url: uploaded.url,
    id: uploaded.id,
    bytes: uploaded.sizeBytes,
    contentType: type,
    record,
    indexPath: ["assetGroups", groupKey, "files", key],
    shownAs,
    next: `Write record into project/design-system.json at assetGroups.${groupKey}.files.${key} and add ${JSON.stringify(base)} to that group's order (no such group yet: add {"name": ${JSON.stringify(groupName)}, "tile": "m", "order": [], "files": {}} and put ${JSON.stringify(groupName)} in groups). The page shows it as ${shownAs}; write no file at that path (a file wins over the record). Previews and other files reference it as url, verbatim. Then run check.`
  };
}
function folderName(name) {
  const folder = name.toLowerCase().replace(/[^a-z0-9_-]+/g, "-").replace(/^[-_]+/, "").slice(0, 64).replace(/-+$/, "");
  return /^[a-z0-9][a-z0-9_-]{0,63}$/.test(folder) ? folder : "ds";
}
var SKIP_DIRS = /* @__PURE__ */ new Set(["node_modules", "dist", "build", "out", "vendor", "target"]);
async function listSystems(dirs) {
  const extra = (process.env.CLAUDE_DESIGN_SYSTEMS ?? "").split(path15.delimiter).filter(Boolean);
  const starts = [...dirs.length > 0 ? dirs : [process.cwd()], ...extra].map((d) => path15.resolve(d));
  const found = /* @__PURE__ */ new Map();
  const visit = async (dir, depth) => {
    const project = path15.join(dir, "project");
    if (fs13.existsSync(path15.join(project, DESIGN_SYSTEM_FILE)) && !found.has(dir)) {
      const info = await readDesignSystemInfo(project).catch(() => null);
      if (info) {
        const namespace = info.namespace;
        found.set(dir, {
          title: info.title,
          namespace: namespace ?? null,
          folder: folderName(typeof namespace === "string" && namespace.trim() ? namespace : String(info.title ?? "")),
          root: dir,
          readme: fs13.existsSync(path15.join(project, "README.md")) ? path15.join(project, "README.md") : null,
          tokens: fs13.existsSync(path15.join(project, "tokens.json")) ? path15.join(project, "tokens.json") : null
        });
        return;
      }
    }
    if (depth <= 0) return;
    const entries = await fsp3.readdir(dir, { withFileTypes: true }).catch(() => []);
    for (const entry of entries) {
      if (!entry.isDirectory() || entry.name.startsWith(".") || SKIP_DIRS.has(entry.name) || entry.name === "project") continue;
      await visit(path15.join(dir, entry.name), depth - 1);
    }
  };
  for (const start of starts) await visit(start, 4);
  return [...found.values()];
}
var runningServer2 = (root) => runningServer(root, KIND, "__deck/session");
function serverArgs(flags) {
  const out = [];
  if (flags.has("port")) out.push("--port", flags.get("port"));
  if (flags.has("host")) out.push("--host", flags.get("host"));
  if (flags.get("agent-hook")) out.push("--agent-hook", flags.get("agent-hook"));
  if (flags.has("lan")) out.push("--lan");
  return out;
}
var fail = (code, message) => {
  throw new CliError(code, message);
};
var shareCall2 = (url, action) => shareCall(url, action, fail);
var lanOn2 = (url, flags) => lanOn(url, flags, fail);
function reusedNote(a, flags) {
  return flags.get("agent-hook") ? { note: `--agent-hook was not applied: the server was already running. To use it: ${script("stop", a.root)}, then open again.` } : {};
}
function assertServerFiles() {
  if (!fs13.existsSync(path15.join(EDITOR_DIR, "index.html"))) {
    throw new CliError("EDITOR_MISSING", `The editor page is missing (${EDITOR_DIR}); reinstall the skill.`);
  }
  if (!fs13.existsSync(SERVER_SCRIPT)) {
    throw new CliError("EDITOR_MISSING", `The editor server is missing (${SERVER_SCRIPT}); reinstall the skill.`);
  }
}
async function serve(a, flags) {
  if (!await readInfo(a.project)) {
    throw new CliError("NOT_A_" + KIND.toUpperCase().replace("-", "_"), `project/${INDEX_FILE} is missing or not a valid ${NOUN} index; write it, then run ${script("check", a.root)}.`);
  }
  assertServerFiles();
  const existing = await runningServer2(a.root);
  if (existing && existing.pid !== process.pid) {
    print({ url: existing.url, root: a.root, pid: existing.pid, reused: true, ...await lanOn2(existing.url, flags), ...reusedNote(a, flags) });
    return;
  }
  const host = flags.get("host") ?? "127.0.0.1";
  const port = flags.has("port") ? Number(flags.get("port")) : DEFAULT_PORTS[KIND];
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new CliError("INVALID_PORT", "--port is a number from 0 to 65535.");
  const server = await import(pathToFileURL(SERVER_SCRIPT).href);
  const state = await server.serveForeground({ root: a.root, port, host, agentHook: flags.get("agent-hook") || void 0, lan: flags.has("lan"), portGiven: flags.has("port") });
  print({ url: state.url, root: a.root, pid: state.pid, ...state.lan ? { lan: state.lan } : {} });
}
async function open(a, flags) {
  if (!await readInfo(a.project)) {
    throw new CliError("NOT_A_" + KIND.toUpperCase().replace("-", "_"), `project/${INDEX_FILE} is missing or not a valid ${NOUN} index; write it, then run ${script("check", a.root)}.`);
  }
  assertServerFiles();
  const existing = await runningServer2(a.root);
  if (existing) {
    print({ url: existing.url, root: a.root, pid: existing.pid, reused: true, ...await lanOn2(existing.url, flags), log: path15.join(a.root, LOG_FILE), stop: script("stop", a.root), ...reusedNote(a, flags) });
    return;
  }
  const state = await startBackground(a, flags);
  print({ url: state.url, root: a.root, pid: state.pid, ...state.lan?.length ? { lan: state.lan } : {}, log: path15.join(a.root, LOG_FILE), stop: script("stop", a.root) });
}
async function startBackground(a, flags) {
  const host = flags.get("host") ?? "127.0.0.1";
  const port = flags.has("port") ? Number(flags.get("port")) : DEFAULT_PORTS[KIND];
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new CliError("INVALID_PORT", "--port is a number from 0 to 65535.");
  const hook = flags.get("agent-hook") || void 0;
  const opened = await openOnHub({
    kind: KIND,
    script: SERVER_SCRIPT,
    root: a.root,
    port,
    portGiven: flags.has("port"),
    host,
    ...hook ? { agentHook: hook } : {},
    lan: flags.has("lan"),
    command: [SERVER_SCRIPT, a.root, ...serverArgs(flags)],
    running: () => runningServer2(a.root)
  });
  if ("state" in opened) {
    const view = await shareCall2(opened.state.url, null).catch(() => null);
    return { ...opened.state, ...view?.lan ? { lan: lanUrls(view) } : {} };
  }
  throw new CliError("SERVER_FAILED", `The editor server did not start. Run ${script("serve", a.root)} in the foreground to see why.${opened.tail ? `
${opened.tail}` : ""}`);
}
async function stop(a) {
  const state = readState(a.root);
  if (!state || !alive(state.pid)) {
    fs13.rmSync(path15.join(a.root, STATE_FILE), { force: true });
    print({ stopped: false, message: "No editor server is running for this folder." });
    return;
  }
  if (!await leaveHub(state)) {
    print({ stopped: false, pid: state.pid, message: `The server did not exit within 5 s; end it with: kill ${state.pid}` });
    return;
  }
  fs13.rmSync(path15.join(a.root, STATE_FILE), { force: true });
  print({ stopped: true, pid: state.pid });
}
async function share(a, flags) {
  if (!await readInfo(a.project)) {
    throw new CliError("NOT_A_" + KIND.toUpperCase().replace("-", "_"), `project/${INDEX_FILE} is missing or not a valid ${NOUN} index; write it, then run ${script("check", a.root)}.`);
  }
  print(
    await shareCommand({
      root: a.root,
      flags,
      running: () => runningServer2(a.root),
      startShared: async () => {
        assertServerFiles();
        return startBackground(a, new Map([...flags, ["lan", "true"]]));
      },
      offCommand: script("share", a.root, "--off"),
      serveCommand: script("serve", a.root, "--lan"),
      fail
    })
  );
}
var WAIT_CHUNK_S = 300;
function commentHints(a) {
  return {
    reply: script("comment-reply", a.root, "<id>", "<text>"),
    resolve: script("comment-resolve", a.root, "<id>")
  };
}
async function listComments(a, all) {
  const doc = await readComments(a.root);
  const threads = all ? [...doc.threads].sort((x, y) => x.createdAt.localeCompare(y.createdAt)) : openThreads(doc);
  const resolved = doc.threads.filter((t) => t.status === "resolved").length;
  return {
    root: a.root,
    file: commentsPath(a.root),
    threads: agentViews(doc, threads),
    ...all || !resolved ? {} : { resolved: `${resolved} resolved, not shown (--all shows them)` },
    ...threads.length ? commentHints(a) : {}
  };
}
async function artifactTitle(a) {
  const info = await readInfo(a.project).catch(() => null);
  const title = typeof info?.title === "string" ? info.title.trim() : "";
  return title && title !== "project" ? title : path15.basename(a.root);
}
async function waitComments(a, flags) {
  const timeout = flags.has("timeout") ? Number(flags.get("timeout")) : Infinity;
  if (!(timeout > 0)) throw new CliError("INVALID_TIMEOUT", "--timeout is a number of seconds above 0.");
  const deadline = Date.now() + timeout * 1e3;
  let failures = 0;
  for (; ; ) {
    const state = await runningServer2(a.root);
    if (!state) {
      throw new CliError("NO_SERVER", `No editor server is running for this folder: start it with ${script("open", a.root)}, then wait again.`);
    }
    const left = Math.min(WAIT_CHUNK_S, (deadline - Date.now()) / 1e3);
    if (left <= 0) return { root: a.root, threads: [], timedOut: true, next: script("comments", a.root, "--wait") };
    let body;
    try {
      const response = await fetch(new URL(`__comments/agent/wait?timeout=${Math.max(1, Math.round(left))}`, state.url), {
        // A whole number of ms: AbortSignal.timeout refuses a fraction (and --timeout leaves one in `left`).
        signal: AbortSignal.timeout(Math.ceil((left + 30) * 1e3))
      });
      if (response.status === 404) {
        throw new CliError(
          "SERVER_OUTDATED",
          `The running editor server has no comments (an older version started it). Restart it: ${script("stop", a.root)}, then ${script("open", a.root)}.`
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
        root: a.root,
        url: state.url,
        title: await artifactTitle(a),
        file: commentsPath(a.root),
        threads,
        ...commentHints(a),
        next: script("comments", a.root, "--wait")
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
async function replyComment(a, id, words, resolve) {
  if (!id) throw new CliError("MISSING_ID", `Give the comment id: ${script("comment-reply", a.root, "<id>", "<text>")}`);
  const given = words.join(" ");
  const text = (given === "" || given === "-" ? await readStdin() : given).replace(/\r\n?/g, "\n").trim();
  if (!text) throw new CliError("EMPTY_REPLY", "The reply is empty: pass it as an argument, or as - with the text on stdin.");
  if (text.length > LIMITS.text) throw new CliError("REPLY_TOO_LONG", `A reply is at most ${LIMITS.text} characters.`);
  const thread = await updateComments(a.root, (doc) => {
    const found = findThread(doc, id);
    const now = (/* @__PURE__ */ new Date()).toISOString();
    addMessage(found, AGENT_AUTHOR, text, now);
    if (resolve) setStatus(found, "resolved", now);
    return found;
  });
  return { replied: thread.id, status: thread.status, file: commentsPath(a.root) };
}
async function resolveComment(a, id, reopen) {
  if (!id) throw new CliError("MISSING_ID", `Give the comment id: ${script("comment-resolve", a.root, "<id>")}`);
  const thread = await updateComments(a.root, (doc) => {
    const found = findThread(doc, id);
    setStatus(found, reopen ? "open" : "resolved", (/* @__PURE__ */ new Date()).toISOString());
    return found;
  });
  return { id: thread.id, status: thread.status, file: commentsPath(a.root) };
}
var USAGE = `${"claude-design"} \u2014 local Artifact tool for the ${NOUN} type

Usage:
  node scripts/artifact.mjs open   <root> [--port n] [--host h]   start the editor in the background; prints {"url"}
  node scripts/artifact.mjs serve  <root> [--port n] [--host h]   run the editor server in the foreground
                                   [--agent-hook "<command>"]     open/serve: run this for each comment sent to the agent
                                                                  (the comment as JSON on stdin)
                                   [--lan]                        open/serve: also share on the local network, as share does
  node scripts/artifact.mjs stop   <root>                         stop the background editor server
  node scripts/artifact.mjs share  <root>                         share on the local network; prints a link per role
                                   [--reset R] [--enable R] [--disable R]   R: editor, commenter, viewer (comma list) or all
                                   [--role G=R,...]               give guest G (an id it prints) role R
                                                                  (these change links only: shared or not stays as it was)
                                   [--off]                        stop sharing (guests keep their place for next time)
  node scripts/artifact.mjs check  <root>                         validate project/; exit 1 while problems remain
  node scripts/artifact.mjs upload <root> <file>${KIND === "design-system" ? " --group G  " : "            "}      store an upload; prints its /_blob/<id> url
  node scripts/artifact.mjs list   [dir \u2026]                        find design systems (folders holding project/design-system.json)
${KIND === "design-system" ? "" : `  node scripts/artifact.mjs ds-sync <root> [namespace] [--check]  an installed design system is a copy: copy its changed files again
                                                                  from the system's folder and stamp copiedAt (--check: list only;
                                                                  --from <folder>: the source, when the record's path is not found)
`}  node scripts/artifact.mjs comments <root> [--all]               the open comments people left in the editor (--all: resolved too)
  node scripts/artifact.mjs comments <root> --wait [--timeout s]  wait for a comment sent to the agent from the open editor
  node scripts/artifact.mjs comment-reply <root> <id> <text|->    reply as the agent (-: text on stdin) [--resolve]
  node scripts/artifact.mjs comment-resolve <root> <id> [--reopen]   mark a comment resolved (or open again)
  node scripts/artifact.mjs archive <root> [--out file.zip|dir]   the whole folder as one zip (backup, another computer,
                                                                  someone else); beside the folder unless --out says

<root> is the artifact folder: its files live at <root>/project/<path> (project/${INDEX_FILE}, \u2026).`;
async function main() {
  const [command, ...rest] = process.argv.slice(2);
  const { positional, flags } = parseArgs(rest);
  switch (command) {
    case "open":
      await open(resolveArtifact(positional[0]), flags);
      return 0;
    case "serve":
      await serve(resolveArtifact(positional[0]), flags);
      return -1;
    case "stop":
      await stop(resolveArtifact(positional[0], false));
      return 0;
    case "share":
      await share(resolveArtifact(positional[0]), flags);
      return 0;
    case "check": {
      const { result, problems } = await check(resolveArtifact(positional[0]));
      print(localize(result));
      return problems > 0 ? 1 : 0;
    }
    case "upload": {
      const a = resolveArtifact(positional[0], false);
      if (!fs13.statSync(a.root, { throwIfNoEntry: false })?.isDirectory()) {
        throw new CliError("NOT_FOUND", `${a.root} does not exist. Make the artifact folder first.`);
      }
      fs13.mkdirSync(a.project, { recursive: true });
      print(await upload(a, positional[1], flags.get("group")));
      return 0;
    }
    case "list":
      print(await listSystems(positional));
      return 0;
    case "ds-sync": {
      if (KIND === "design-system") throw new CliError("NOT_APPLICABLE", "A design system has no installed design systems: ds-sync is for decks and canvases.");
      const a = resolveArtifact(positional[0]);
      const check2 = flags.has("check");
      let results;
      try {
        results = syncSystems(a.project, a.root, INDEX_FILE, { namespace: positional[1], from: flags.get("from"), check: check2 });
      } catch (error) {
        throw new CliError("NO_SUCH_SYSTEM", error.message);
      }
      const behind = results.filter((r) => r.changed.length > 0 || r.removed.length > 0);
      print({
        root: a.root,
        systems: results.map((r) => ({
          namespace: r.namespace,
          title: r.title,
          ...r.skipped ? { skipped: r.skipped } : {},
          ...r.missing ? { skipped: `project/ds/${r.namespace}/ does not exist: install it first (SKILL.md, Creating, step 4)` } : {},
          ...r.source ? { source: r.source } : {},
          changed: r.changed,
          ...r.removed.length > 0 ? { goneFromSource: r.removed } : {},
          ...check2 ? {} : { copied: r.copied, ...r.copiedAt ? { copiedAt: r.copiedAt } : {} }
        })),
        next: check2 ? behind.length > 0 ? `Behind its source. Run ${script("ds-sync", a.root)} to copy the changed files again.` : "Every copy matches its source (or its source was not found: see skipped)." : results.some((r) => r.copied.length > 0) ? "Copied. The open editor shows the new files at once (a file over 2 MB, or a types file, after a reload). Files the source dropped stay in the copy: remove one, and change the artboards that load it, if the source no longer has it. Files the source gained are not copied: compare its README with project/ds/<folder>/." : "Nothing to copy."
      });
      return check2 && behind.some((r) => r.changed.length > 0) ? 1 : 0;
    }
    case "comments": {
      const a = resolveArtifact(positional[0]);
      print(flags.has("wait") ? await waitComments(a, flags) : await listComments(a, flags.has("all")));
      return 0;
    }
    case "comment-reply":
      print(await replyComment(resolveArtifact(positional[0]), positional[1], positional.slice(2), flags.has("resolve")));
      return 0;
    case "comment-resolve":
      print(await resolveComment(resolveArtifact(positional[0]), positional[1], flags.has("reopen")));
      return 0;
    case "archive": {
      const a = resolveArtifact(positional[0]);
      const made = await archiveCommand(a.root, flags.get("out"));
      print({ ...made, open: `unzip it anywhere, then ${script("open", `<there>/${made.folder}`)}` });
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
main().then(
  (code) => {
    if (code >= 0) process.exitCode = code;
  },
  (error) => {
    const e = error;
    process.stderr.write(`${JSON.stringify({ error: e.code ?? "ERROR", message: e.message })}
`);
    process.exitCode = 2;
  }
);
