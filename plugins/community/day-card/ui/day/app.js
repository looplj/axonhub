// 移植自 magpie day-card 的页面。小票的绘制、撕边、背景和分享按钮保持原样。
// 用量不在这里加总：analyticsOverview / analyticsDailyStats 给出当天和历史，
// analyticsDimensionStats 的 apiKey 维度是 Agent，每把要画出来的 Key 再按模型聚合一次。
const V = globalThis.DayView;
const B = globalThis.DayBackgrounds;
const $ = (id) => document.getElementById(id);
const HISTORY_DAYS = 400;
const SHOWN_KEYS = 8;

const bridge = window.axonhub;
let lang = "en";
let timeZone = "UTC";
let currency = "USD";
let canWriteKV = false;
const state = { date: "", today: "", day: null, icons: new Map(), loading: 0, drawn: 0, bg: B.norm(null) };
let history = [];
let historyKey = "";

function words() {
  const t = V.text(lang);
  document.querySelectorAll("[data-t]").forEach((e) => { e.textContent = t[e.dataset.t]; });
  document.querySelectorAll("[data-t-label]").forEach((e) => { e.setAttribute("aria-label", t[e.dataset.tLabel]); e.title = t[e.dataset.tLabel]; });
}
let statusTimer = 0;
function status(msg, bad, keep) {
  clearTimeout(statusTimer);
  $("status").textContent = msg || "";
  $("status").classList.toggle("bad", !!bad);
  if (msg && !keep) statusTimer = setTimeout(() => status(""), 6000);
}

const INK = "#1f1d20";
const MUTED = "#76736e";
const PAPER = "#f7f4ed";
const DASH = "#a8a49c";
const MONO = '"Day Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, "PingFang SC", "Hiragino Sans", "Microsoft YaHei", "Noto Sans CJK SC", monospace';

const fonts = (async () => {
  const F = globalThis.DayFonts || {};
  await Promise.all(Object.entries(F).map(async ([weight, b64]) => {
    try {
      const bin = atob(b64);
      const buf = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i);
      const face = new FontFace("Day Mono", buf.buffer, { weight });
      document.fonts.add(await face.load());
    } catch {
      // the system's monospace draws it instead
    }
  }));
})();

function picture(name) {
  const ic = state.day?.icons?.[name];
  if (!ic?.src) return Promise.resolve(null);
  if (!state.icons.has(ic.src)) {
    state.icons.set(ic.src, new Promise((ok) => {
      const img = new Image();
      img.onload = () => ok({ img, mono: !!ic.mono });
      img.onerror = () => ok(null);
      img.src = ic.src;
    }));
  }
  return state.icons.get(ic.src);
}

function drawIcon(ctx, ic, x, y, size, scale) {
  if (!ic) {
    ctx.fillStyle = INK;
    ctx.beginPath();
    ctx.arc(x + size / 2, y + size / 2, size * 0.22, 0, Math.PI * 2);
    ctx.fill();
    return;
  }
  const px = Math.max(1, Math.ceil(size * scale));
  const off = document.createElement("canvas");
  off.width = off.height = px;
  const o = off.getContext("2d");
  o.drawImage(ic.img, 0, 0, px, px);
  let clear = ic.mono;
  if (!clear) {
    try {
      const d = o.getImageData(0, 0, px, px);
      const p = d.data;
      const lum = (i) => (0.2126 * p[i] + 0.7152 * p[i + 1] + 0.0722 * p[i + 2]) / 255;
      let solid = 0, light = 0, dark = 0;
      for (let i = 0; i < p.length; i += 4) {
        if (p[i + 3] < 128) continue;
        solid++;
        const v = lum(i);
        if (v > 0.8) light++;
        else if (v < 0.5) dark++;
      }
      clear = !(solid && light / solid > 0.35 && dark / solid > 0.05) && solid < px * px * 0.8;
      if (!clear) {
        const ink = [0x1f, 0x1d, 0x20], paper = [0xf7, 0xf4, 0xed];
        for (let i = 0; i < p.length; i += 4) {
          const t = Math.min(1, Math.max(0, (lum(i) - 0.65) / 0.2));
          const l = t * t * (3 - 2 * t);
          for (let k = 0; k < 3; k++) p[i + k] = ink[k] + (paper[k] - ink[k]) * l;
        }
        o.putImageData(d, 0, 0);
      }
    } catch {
      clear = true;
    }
  }
  if (clear) {
    o.globalCompositeOperation = "source-in";
    o.fillStyle = INK;
    o.fillRect(0, 0, px, px);
  }
  ctx.drawImage(off, x, y, size, size);
}

const PW = 440;
const PAD = 30;
const TOOTH = 5;

const f = (w, px) => `${w} ${px}px ${MONO}`;
function fit(ctx, s, width) {
  if (ctx.measureText(s).width <= width) return s;
  let lo = 0, hi = s.length;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (ctx.measureText(s.slice(0, mid) + "…").width <= width) lo = mid; else hi = mid - 1;
  }
  return s.slice(0, lo) + "…";
}
function spaced(ctx, s, cx, y, em, max) {
  const chars = [...s];
  const bare = chars.reduce((n, ch) => n + ctx.measureText(ch).width, 0);
  const gap = Math.max(0, Math.min(em, (max - bare) / Math.max(1, chars.length - 1)));
  const w = bare + gap * (chars.length - 1);
  let x = cx - w / 2;
  ctx.textAlign = "left";
  for (const ch of chars) {
    ctx.fillText(ch, x, y);
    x += ctx.measureText(ch).width + gap;
  }
}
function sized(ctx, weight, px, s, width) {
  ctx.font = f(weight, px);
  const w = ctx.measureText(s).width;
  if (w > width) {
    px = Math.max(10, Math.floor(px * width / w));
    ctx.font = f(weight, px);
  }
  return px;
}

function slip(ctx, c, icons, scale) {
  const L = PAD, R = PW - PAD, inner = R - L;
  const rule = (y) => {
    ctx.save();
    ctx.strokeStyle = DASH;
    ctx.lineWidth = 1;
    ctx.setLineDash([5, 4]);
    ctx.beginPath();
    ctx.moveTo(L + 11, y + 0.5);
    ctx.lineTo(R - 11, y + 0.5);
    ctx.stroke();
    ctx.restore();
  };
  const row = (label, value, y, o = {}) => {
    ctx.font = f(o.weight || 400, o.px || 12.5);
    ctx.fillStyle = o.colour || INK;
    ctx.textAlign = "right";
    ctx.fillText(value, R, y);
    const vw = ctx.measureText(value).width;
    ctx.textAlign = "left";
    ctx.fillText(fit(ctx, label, inner - vw - 14), L, y);
  };
  const glow = (on) => {
    ctx.shadowColor = on ? "rgba(31, 29, 32, .16)" : "transparent";
    ctx.shadowBlur = on ? 14 : 0;
    ctx.shadowOffsetY = on ? 3 : 0;
  };
  ctx.textBaseline = "alphabetic";
  let y = 0;

  y += 54;
  ctx.fillStyle = INK;
  sized(ctx, 800, 30, c.title, inner);
  ctx.textAlign = "center";
  glow(true);
  ctx.fillText(c.title, PW / 2, y);
  glow(false);
  y += 25;
  ctx.font = f(400, 11.5);
  ctx.fillText(c.tagline, PW / 2, y);

  y += 27;
  c.head.forEach((h, i) => {
    if (i) y += 22;
    row(h.label, h.value, y);
  });
  y += 15;
  rule(y);

  y += 25;
  ctx.fillStyle = INK;
  ctx.font = f(400, 12.5);
  spaced(ctx, c.totalLabel, PW / 2, y, 3.2, inner);
  y += 57;
  sized(ctx, 800, 52, c.total, inner);
  ctx.textAlign = "center";
  glow(true);
  ctx.fillText(c.total, PW / 2, y);
  glow(false);
  y += 23;
  sized(ctx, 400, 10.5, c.totalNote, inner);
  spaced(ctx, c.totalNote, PW / 2, y, 1.6, inner);
  y += 16;
  rule(y);

  y += 25;
  c.stats.forEach((s, i) => {
    if (i) y += 21;
    row(s.label, s.value, y);
  });
  y += 15;
  rule(y);

  y += 25;
  row(c.agentHead[0], c.agentHead[1], y, { colour: MUTED });
  if (c.empty) {
    y += 27;
    ctx.font = f(400, 12.5);
    ctx.fillStyle = MUTED;
    ctx.textAlign = "left";
    ctx.fillText(c.empty, L, y);
  }
  for (const a of c.agents) {
    y += 27;
    drawIcon(ctx, icons.get(a.icon), L, y - 15, 18, scale);
    ctx.font = f(700, 14.5);
    ctx.fillStyle = INK;
    ctx.textAlign = "right";
    ctx.fillText(a.value, R, y);
    const vw = ctx.measureText(a.value).width;
    ctx.textAlign = "left";
    ctx.fillText(fit(ctx, a.name, inner - 26 - vw - 14), L + 26, y);
    y += 1;
    for (const m of a.models) {
      y += 27;
      drawIcon(ctx, icons.get(m.icon), L + 26, y - 10.5, 12, scale);
      ctx.font = f(400, 12.5);
      ctx.fillStyle = INK;
      ctx.textAlign = "right";
      ctx.fillText(m.value, R, y);
      const mw = ctx.measureText(m.value).width;
      ctx.textAlign = "left";
      ctx.fillText(fit(ctx, m.name, inner - 44 - mw - 14), L + 44, y);
    }
    if (a.more) {
      y += 24;
      ctx.font = f(400, 11.5);
      ctx.fillStyle = MUTED;
      ctx.textAlign = "left";
      ctx.fillText(a.more, L + 44, y);
    }
    y += 3;
  }
  if (c.moreAgents) {
    y += 24;
    ctx.font = f(400, 11.5);
    ctx.fillStyle = MUTED;
    ctx.textAlign = "left";
    ctx.fillText(c.moreAgents, L + 26, y);
  }
  y += 17;
  rule(y);

  y += 25;
  c.tokens.forEach((s, i) => {
    if (i) y += 21;
    row(s.label, s.value, y);
  });
  if (c.note) {
    y += 21;
    row(c.note, "", y, { colour: MUTED, px: 11 });
  }
  y += 18;
  rule(y);

  y += 22;
  for (const line of c.footer) {
    const parts = line.map((s, i) => ({ s, w: i === 1 ? 700 : 400 })).filter((p) => p.s);
    const width = parts.reduce((n, p) => { ctx.font = f(p.w, 10.5); return n + ctx.measureText(p.s).width; }, 0);
    let x = PW / 2 - width / 2;
    ctx.fillStyle = INK;
    ctx.textAlign = "left";
    for (const p of parts) {
      ctx.font = f(p.w, 10.5);
      ctx.fillText(p.s, x, y);
      x += ctx.measureText(p.s).width;
    }
    y += 20;
  }
  return Math.ceil(y + 4);
}

async function draw() {
  const day = state.day;
  if (!day) return;
  const n = ++state.drawn;
  const bg = state.bg;
  await fonts;
  const c = V.card(day, lang);
  const icons = new Map();
  const [img] = await Promise.all([
    backdrop(bg),
    ...[...new Set(c.agents.flatMap((a) => [a.icon, ...a.models.map((m) => m.icon)]))].map(async (name) => icons.set(name, await picture(name))),
  ]);
  if (state.day !== day || n !== state.drawn) return;
  const scale = Math.max(2, Math.min(3, Math.ceil(window.devicePixelRatio || 1)));

  const ph = slip(document.createElement("canvas").getContext("2d"), c, new Map(), 1) + TOOTH;
  const side = bg.pad;
  const W = PW + side * 2;
  const H = ph + side * 2;
  const top = side;
  const cv = $("card");
  cv.width = W * scale;
  cv.height = H * scale;
  cv.style.aspectRatio = `${W} / ${H}`;
  const ctx = cv.getContext("2d");
  ctx.setTransform(scale, 0, 0, scale, 0, 0);

  B.paint(ctx, W, H, scale, bg, img);

  ctx.save();
  ctx.translate(side, top);
  const paper = new Path2D();
  paper.moveTo(0, 0);
  paper.lineTo(PW, 0);
  paper.lineTo(PW, ph - TOOTH);
  const teeth = Math.round(PW / (TOOTH * 2));
  const tw = PW / teeth;
  for (let i = teeth - 1; i >= 0; i--) {
    paper.lineTo(i * tw + tw / 2, ph);
    paper.lineTo(i * tw, ph - TOOTH);
  }
  paper.closePath();
  ctx.shadowColor = B.meta(bg.bg).shadow;
  ctx.shadowBlur = 36;
  ctx.shadowOffsetY = 12;
  ctx.fillStyle = PAPER;
  ctx.fill(paper);
  ctx.shadowColor = "transparent";
  slip(ctx, c, icons, scale);
  ctx.restore();
  $("copy").disabled = $("save").disabled = false;
}

function problem(result) {
  const err = (result && result.errors && result.errors[0]) || null;
  return err ? err.message : V.text(lang).failed;
}

async function gql(query, variables) {
  return bridge.graphql(query, variables || {});
}

async function ensureHistory(today, force) {
  const key = `${today}|${timeZone}`;
  if (!force && historyKey === key && history.length) return history;
  const start = V.startOfDay(V.shift(today, -(HISTORY_DAYS - 1)), timeZone);
  const end = V.endOfDay(today, timeZone);
  const result = await gql(V.HISTORY_QUERY, { filter: { startTime: start.toISOString(), endTime: end.toISOString() } });
  if (!result || !result.data || !result.data.analyticsDailyStats) throw new Error(problem(result));
  history = result.data.analyticsDailyStats;
  historyKey = key;
  return history;
}

async function loadModels(ids, from, to) {
  const real = ids.filter((id) => V.apiKeyGUID(id));
  if (!real.length) return {};
  const variables = {};
  real.forEach((id, i) => {
    variables[`f${i}`] = { startTime: from, endTime: to, apiKeyIDs: [V.apiKeyGUID(id)] };
  });
  const result = await gql(V.modelsQuery(real.length), variables);
  const out = {};
  real.forEach((id, i) => { out[id] = (result && result.data && result.data[`m${i}`]) || []; });
  return out;
}

async function load(date) {
  const n = ++state.loading;
  if (!state.day) status(V.text(lang).loading, false, true);
  try {
    const today = V.calendarDate(new Date(), timeZone);
    state.today = today;
    const picked = date || today;
    const from = V.startOfDay(picked, timeZone).toISOString();
    const to = V.endOfDay(picked, timeZone).toISOString();
    const failed = { createdAtGTE: from, createdAtLTE: to, statusIn: ["failed"] };
    const unpriced = { createdAtGTE: from, createdAtLTE: to, totalCostIsNil: true };
    const [days, result] = await Promise.all([
      ensureHistory(today, picked === today),
      gql(V.DAY_QUERY, { filter: { startTime: from, endTime: to }, failed, unpriced }),
    ]);
    if (n !== state.loading) return;
    const data = (result && result.data) || {};
    if (!data.analyticsOverview) throw new Error(problem(result));
    const keys = data.analyticsDimensionStats || [];
    const ranked = V.rankedKeys(keys, data.analyticsOverview, SHOWN_KEYS);
    const modelsByKey = await loadModels(ranked.shown.map((item) => item.raw.id), from, to);
    if (n !== state.loading) return;
    state.day = V.assemble({
      date: picked,
      today,
      currency,
      errors: data.failed ? data.failed.totalCount : 0,
      unpriced: data.unpriced ? data.unpriced.totalCount : 0,
      overview: data.analyticsOverview,
      keys,
      modelsByKey,
      days,
      limit: SHOWN_KEYS,
    });
    state.date = picked;
    status("");
  } catch (e) {
    if (n !== state.loading) return;
    status(`${V.text(lang).failed}: ${e?.message || e}`, true, true);
  }
  paintBar();
  await draw();
}

function paintBar() {
  const d = state.day;
  $("date-text").textContent = d ? V.dateLabel(d.date, lang) : "…";
  $("next").disabled = !d || d.date >= d.today;
  $("today").disabled = !d || d.date === d.today;
  $("prev").disabled = !d;
}

const menu = $("menu");
function openMenu() {
  const d = state.day;
  if (!d) return;
  const days = (d.days || []).slice().reverse().slice(0, 120);
  if (!days.some((x) => x.date === d.today)) days.unshift({ date: d.today, tokens: 0 });
  menu.replaceChildren(...days.map((x) => {
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    li.dataset.date = x.date;
    li.setAttribute("aria-selected", String(x.date === d.date));
    const a = document.createElement("span");
    a.textContent = V.dateLabel(x.date, lang);
    const b = document.createElement("span");
    b.textContent = x.tokens ? V.count(x.tokens) : "";
    li.append(a, b);
    return li;
  }));
  menu.hidden = false;
  menu.classList.remove("closing");
  menu.classList.add("opening");
  $("date").setAttribute("aria-expanded", "true");
  requestAnimationFrame(() => menu.classList.remove("opening"));
  menu.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
}
function closeMenu() {
  if (menu.hidden) return;
  $("date").setAttribute("aria-expanded", "false");
  menu.classList.add("closing");
  setTimeout(() => { if (menu.classList.contains("closing")) { menu.hidden = true; menu.classList.remove("closing"); } }, 110);
}
$("date").addEventListener("click", (e) => {
  e.stopPropagation();
  if (menu.hidden || menu.classList.contains("closing")) openMenu(); else closeMenu();
});
menu.addEventListener("click", (e) => {
  const li = e.target.closest("li");
  if (!li) return;
  closeMenu();
  if (li.dataset.date !== state.date) load(li.dataset.date);
});
document.addEventListener("click", (e) => { if (!e.target.closest(".pick")) closeMenu(); });
document.addEventListener("keydown", (e) => { if (e.key === "Escape") { closeMenu(); closePop(); } });

$("prev").addEventListener("click", () => state.date && load(V.shift(state.date, -1)));
$("next").addEventListener("click", () => state.date && state.day && state.date < state.day.today && load(V.shift(state.date, 1)));
$("today").addEventListener("click", () => load(""));

const images = new Map();
function image(src) {
  if (!images.has(src)) {
    images.set(src, new Promise((ok) => {
      const img = new Image();
      img.onload = () => ok(img);
      img.onerror = () => ok(null);
      img.src = src;
    }));
  }
  return images.get(src);
}
const scripts = new Map();
function script(file) {
  if (!scripts.has(file)) {
    scripts.set(file, new Promise((ok) => {
      const s = document.createElement("script");
      s.src = file;
      s.onload = () => ok(true);
      s.onerror = () => { scripts.delete(file); ok(false); };
      document.head.append(s);
    }));
  }
  return scripts.get(file);
}
async function photoSrc(id) {
  if (!globalThis.DayPhotos?.[id]) await script(`bg/${id}.js`);
  return globalThis.DayPhotos?.[id] || "";
}
async function backdrop(bg) {
  if (!B.meta(bg.bg).photo) return null;
  const src = bg.bg === "custom" ? bg.image : await photoSrc(bg.bg);
  return src ? image(src) : null;
}

let saveTimer = 0;
function keep() {
  clearTimeout(saveTimer);
  if (!canWriteKV) return;
  saveTimer = setTimeout(async () => {
    try {
      const result = await gql(V.KV_WRITE, { value: JSON.stringify(state.bg) });
      if (result && result.errors && result.errors.length) throw new Error(result.errors[0].message);
    } catch (e) {
      status(`${V.text(lang).keepFailed}: ${e?.message || e}`, true);
    }
  }, 1100);
}
function pick(o) {
  state.bg = B.norm({ ...state.bg, ...o });
  paintPicker();
  keep();
  draw();
}

const pop = $("bg-pop");
const grid = $("bg-grid");
function swatch(id) {
  const b = document.createElement("button");
  b.type = "button";
  b.className = "sw";
  b.dataset.bg = id;
  b.setAttribute("role", "radio");
  return b;
}
function patternCanvas(id, size) {
  const cv = document.createElement("canvas");
  const s = Math.max(1, Math.min(3, Math.ceil(window.devicePixelRatio || 1)));
  cv.width = cv.height = size * s;
  const ctx = cv.getContext("2d");
  ctx.setTransform(s, 0, 0, s, 0, 0);
  B.paint(ctx, size, size, s, { ...state.bg, bg: id }, null);
  return cv;
}
async function buildGrid() {
  await script("bg/thumbs.js");
  const t = V.text(lang);
  const list = B.list.map((m) => {
    const b = swatch(m.id);
    if (m.photo) {
      const img = document.createElement("img");
      img.alt = "";
      img.src = globalThis.DayThumbs?.[m.id] || "";
      b.append(img);
    } else {
      b.append(patternCanvas(m.id, 60));
    }
    return b;
  });
  if (state.bg.image) {
    const b = swatch("custom");
    const img = document.createElement("img");
    img.alt = "";
    img.src = state.bg.image;
    b.append(img);
    list.push(b);
  }
  const add = document.createElement("button");
  add.type = "button";
  add.className = "sw add";
  add.id = "bg-add";
  add.innerHTML = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 3v10M3 8h10"/></svg>';
  add.setAttribute("aria-label", t.upload);
  add.title = t.upload;
  list.push(add);
  grid.replaceChildren(...list);
  paintPicker();
}
function paintPicker() {
  const t = V.text(lang);
  const bg = state.bg;
  grid.querySelectorAll(".sw[data-bg]").forEach((b) => {
    const name = t[`bg-${b.dataset.bg}`];
    b.setAttribute("aria-label", name);
    b.title = name;
    b.setAttribute("aria-checked", String(b.dataset.bg === bg.bg));
  });
  $("pad").value = bg.pad;
  $("blur").value = bg.blur;
  $("dim").value = bg.dim;
  pop.classList.toggle("photo", B.meta(bg.bg).photo === true);
  paintChip();
}
async function paintChip() {
  const cv = $("bg-chip");
  const s = Math.max(1, Math.min(3, Math.ceil(window.devicePixelRatio || 1)));
  cv.width = cv.height = 14 * s;
  const ctx = cv.getContext("2d");
  ctx.setTransform(s, 0, 0, s, 0, 0);
  const bg = state.bg;
  const img = await backdrop(bg);
  if (bg !== state.bg) return;
  B.paint(ctx, 14, 14, s, bg, img);
}

function openPop() {
  if (!grid.childElementCount) buildGrid();
  pop.hidden = false;
  pop.classList.remove("closing");
  pop.classList.add("opening");
  $("bg").setAttribute("aria-expanded", "true");
  requestAnimationFrame(() => pop.classList.remove("opening"));
}
function closePop() {
  if (pop.hidden) return;
  $("bg").setAttribute("aria-expanded", "false");
  pop.classList.add("closing");
  setTimeout(() => { if (pop.classList.contains("closing")) { pop.hidden = true; pop.classList.remove("closing"); } }, 120);
}
$("bg").addEventListener("click", (e) => {
  e.stopPropagation();
  if (pop.hidden || pop.classList.contains("closing")) openPop(); else closePop();
});
grid.addEventListener("click", (e) => {
  const b = e.target.closest(".sw");
  if (!b) return;
  if (b.id === "bg-add") $("file").click();
  else pick({ bg: b.dataset.bg });
});
for (const id of ["pad", "blur", "dim"]) $(id).addEventListener("input", (e) => pick({ [id]: e.target.value }));
document.addEventListener("click", (e) => { if (!e.target.closest(".bgpick")) closePop(); });

const readFile = (file) => new Promise((ok, no) => {
  const r = new FileReader();
  r.onload = () => ok(String(r.result));
  r.onerror = () => no(r.error);
  r.readAsDataURL(file);
});
async function usePicture(file) {
  if (!file || !/^image\//.test(file.type)) return;
  try {
    const img = await image(await readFile(file));
    if (!img) throw new Error(file.name || "?");
    const iw = img.naturalWidth, ih = img.naturalHeight;
    let out = "";
    for (const edge of [960, 800, 640, 520, 400]) {
      const k = Math.min(1, edge / Math.max(iw, ih));
      const cv = document.createElement("canvas");
      cv.width = Math.max(1, Math.round(iw * k));
      cv.height = Math.max(1, Math.round(ih * k));
      const ctx = cv.getContext("2d");
      ctx.imageSmoothingQuality = "high";
      ctx.drawImage(img, 0, 0, cv.width, cv.height);
      for (const q of [0.82, 0.72, 0.62]) {
        out = cv.toDataURL("image/jpeg", q);
        if (out.length <= 44000) break;
      }
      if (out.length <= 44000) break;
    }
    if (out.length > 44000) throw new Error(file.name || "?");
    pick({ bg: "custom", image: out });
    buildGrid();
  } catch (e) {
    status(`${V.text(lang).imageFailed}: ${e?.message || e}`, true);
  }
}
$("file").addEventListener("change", (e) => {
  usePicture(e.target.files?.[0]);
  e.target.value = "";
});
const main = document.querySelector("main");
document.addEventListener("dragover", (e) => {
  if (![...(e.dataTransfer?.items || [])].some((i) => i.kind === "file")) return;
  e.preventDefault();
  main.classList.add("drop");
});
document.addEventListener("dragleave", (e) => { if (!e.relatedTarget) main.classList.remove("drop"); });
document.addEventListener("drop", (e) => {
  main.classList.remove("drop");
  const file = [...(e.dataTransfer?.files || [])].find((item) => /^image\//.test(item.type));
  if (!file) return;
  e.preventDefault();
  usePicture(file);
});
document.addEventListener("paste", (e) => {
  const file = [...(e.clipboardData?.files || [])].find((item) => /^image\//.test(item.type));
  if (file) usePicture(file);
});

const png = () => new Promise((ok, no) => $("card").toBlob((b) => (b ? ok(b) : no(new Error("no picture"))), "image/png"));
$("copy").addEventListener("click", async () => {
  const b = $("copy");
  b.disabled = true;
  try {
    const blob = await png();
    if (!navigator.clipboard || typeof ClipboardItem === "undefined") throw new Error("clipboard");
    await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
    b.textContent = V.text(lang).copied;
    setTimeout(() => { b.textContent = V.text(lang).copy; }, 1500);
  } catch (e) {
    status(`${V.text(lang).shareFailed}: ${e?.message || e}`, true);
  } finally {
    b.disabled = false;
  }
});
$("save").addEventListener("click", async () => {
  const b = $("save");
  b.disabled = true;
  try {
    const blob = await png();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${V.fileName(state.date)}.png`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1500);
    status(V.text(lang).saved);
  } catch (e) {
    status(`${V.text(lang).shareFailed}: ${e?.message || e}`, true);
  } finally {
    b.disabled = false;
  }
});

function browserZone() {
  try { return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"; } catch { return "UTC"; }
}
function useZone(zone) {
  if (!zone) return "UTC";
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: zone }).format(0);
    return zone;
  } catch {
    return browserZone();
  }
}

async function readSettings() {
  try {
    const result = await gql(V.SETTINGS_QUERY);
    const settings = result && result.data && result.data.systemGeneralSettings;
    if (!settings) return;
    currency = settings.currencyCode || "USD";
    timeZone = useZone(settings.timezone);
  } catch {
    timeZone = browserZone();
  }
}

async function readBackground() {
  try {
    const result = await gql(V.KV_READ);
    const raw = result && result.data && result.data.pluginKV && result.data.pluginKV.value;
    if (!raw) return;
    state.bg = B.norm(JSON.parse(raw));
  } catch {
    // the default backdrop, until the reader picks one
  }
}

words();
paintBar();
paintPicker();

if (!bridge || typeof bridge.ready !== "function" || typeof bridge.graphql !== "function") {
  status(V.text(lang).bridge, true, true);
} else {
  const boot = async () => {
    const init = await bridge.ready();
    lang = V.langOf(init && init.locale);
    canWriteKV = !!(init && init.canWriteKV);
    timeZone = browserZone();
    words();
    if (typeof bridge.onTheme === "function") {
      bridge.onTheme((theme) => {
        const next = V.langOf(theme && theme.locale ? theme.locale : lang);
        if (next === lang) return;
        lang = next;
        words();
        paintBar();
        if (grid.childElementCount) paintPicker();
        draw();
      });
    }
    await readSettings();
    await readBackground();
    paintPicker();
    await load("");
  };
  boot().catch((e) => status(`${V.text(lang).failed}: ${e?.message || e}`, true, true));
  setInterval(() => {
    if (state.day && state.date === state.day.today && document.visibilityState !== "hidden") load(state.date);
  }, 60_000);
}
