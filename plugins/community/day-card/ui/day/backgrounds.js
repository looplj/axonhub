// 移植自 magpie day-card。图案、照片和边距范围保持原样，小票衬底才和原卡片一样。
// The card's backdrops: patterns drawn on the canvas, photos from bg/, or a
// picture of the reader's own. A classic script: it sets
// globalThis.DayBackgrounds. What a setting may be (norm) needs no DOM, so a
// test can check it; paint draws on a canvas.
(() => {
  // shadow: the paper's shadow on that ground
  const LIST = [
    { id: "teal", shadow: "rgba(0, 60, 55, .28)" },
    { id: "gingham", shadow: "rgba(90, 30, 20, .22)" },
    { id: "grid", shadow: "rgba(30, 50, 90, .2)" },
    { id: "kraft", shadow: "rgba(60, 36, 12, .38)" },
    { id: "cyan", shadow: "rgba(0, 8, 30, .5)" },
    { id: "dots", shadow: "rgba(0, 0, 0, .6)" },
    { id: "deck", shadow: "rgba(10, 40, 32, .32)" },
    { id: "linen", shadow: "rgba(20, 26, 10, .45)" },
    { id: "seigaiha", shadow: "rgba(0, 16, 30, .5)" },
    { id: "xuan", shadow: "rgba(80, 64, 40, .22)" },
    { id: "zigzag", shadow: "rgba(70, 50, 20, .3)" },
    { id: "terracotta", shadow: "rgba(60, 16, 4, .4)" },
    { id: "lilac", shadow: "rgba(50, 36, 90, .25)" },
    { id: "velvet", shadow: "rgba(0, 0, 0, .55)" },
    { id: "polka", shadow: "rgba(120, 50, 20, .22)" },
    { id: "alpine", photo: true, shadow: "rgba(0, 10, 30, .45)" },
    { id: "sea", photo: true, shadow: "rgba(20, 30, 50, .35)" },
    { id: "dunes", photo: true, shadow: "rgba(70, 30, 0, .4)" },
    { id: "lake", photo: true, shadow: "rgba(0, 20, 12, .5)" },
  ].map((b) => ({ ...b, photo: b.photo === true })); // photo is always a boolean: classList.toggle(name, undefined) flips
  const BY = new Map(LIST.map((b) => [b.id, b]));
  const CUSTOM = { id: "custom", photo: true, shadow: "rgba(0, 0, 0, .45)" };

  const PAD = { min: 24, max: 160, step: 4, def: 76 };
  const BLUR = { min: 0, max: 20, def: 0 };
  const DIM = { min: 0, max: 60, def: 0 };
  const num = (v, r) => {
    const n = Math.round(Number(v));
    return Number.isFinite(n) ? Math.min(r.max, Math.max(r.min, n)) : r.def;
  };

  // the reader's saved options, made sound: an unknown backdrop is teal, a
  // custom one with no picture is teal, numbers are held to their range
  function norm(o) {
    o = o && typeof o === "object" ? o : {};
    const image = typeof o.image === "string" && /^data:image\/(jpeg|png|webp);base64,/.test(o.image) ? o.image : "";
    let bg = typeof o.bg === "string" ? o.bg : "teal";
    if (bg === "custom" ? !image : !BY.has(bg)) bg = "teal";
    return { bg, pad: num(o.pad, PAD), blur: num(o.blur, BLUR), dim: num(o.dim, DIM), image };
  }
  const meta = (id) => (id === "custom" ? CUSTOM : BY.get(id) || BY.get("teal"));

  // ---- patterns: a tile drawn once at the canvas's scale, then repeated ----
  function tile(scale, w, h, draw) {
    const c = typeof OffscreenCanvas === "function" ? new OffscreenCanvas(Math.ceil(w * scale), Math.ceil(h * scale)) : Object.assign(document.createElement("canvas"), { width: Math.ceil(w * scale), height: Math.ceil(h * scale) });
    const t = c.getContext("2d");
    t.scale(c.width / w, c.height / h);
    draw(t, w, h);
    return c;
  }
  // fill the card with a tile, rotated by deg if asked
  function layer(ctx, W, H, scale, w, h, draw, deg) {
    const p = ctx.createPattern(tile(scale, w, h, draw), "repeat");
    const m = new DOMMatrix().rotateSelf(deg || 0).scaleSelf(w / Math.ceil(w * scale), h / Math.ceil(h * scale));
    p.setTransform(m);
    ctx.fillStyle = p;
    ctx.fillRect(0, 0, W, H);
  }
  const dot = (t, x, y, r) => { t.beginPath(); t.arc(x, y, r, 0, Math.PI * 2); t.fill(); };
  const ground = (ctx, W, H, c) => { ctx.fillStyle = c; ctx.fillRect(0, 0, W, H); };

  const PATTERNS = {
    teal(ctx, W, H) {
      const g = ctx.createLinearGradient(0, 0, W * 0.35, H);
      g.addColorStop(0, "#1dd8d1");
      g.addColorStop(1, "#92ecc6");
      ground(ctx, W, H, g);
      const g2 = ctx.createLinearGradient(0, 0, W, 0);
      g2.addColorStop(0, "rgba(255, 255, 255, 0)");
      g2.addColorStop(1, "rgba(255, 255, 255, .14)");
      ground(ctx, W, H, g2);
    },
    gingham(ctx, W, H, s) {
      ground(ctx, W, H, "#fbf6ef");
      layer(ctx, W, H, s, 36, 36, (t) => {
        t.fillStyle = "rgba(214, 72, 56, .42)";
        t.fillRect(0, 0, 36, 18);
        t.fillRect(0, 0, 18, 36);
      });
    },
    grid(ctx, W, H, s) {
      ground(ctx, W, H, "#f3f0e6");
      layer(ctx, W, H, s, 22, 22, (t) => {
        t.fillStyle = "rgba(60, 110, 180, .2)";
        t.fillRect(0, 0, 22, 1);
        t.fillRect(0, 1, 1, 21);
      });
    },
    kraft(ctx, W, H, s) {
      ground(ctx, W, H, "#c4a274");
      // fibre specks, scattered by a fixed seed so every card is the same
      let seed = 7;
      const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647);
      layer(ctx, W, H, s, 96, 96, (t) => {
        for (let i = 0; i < 260; i++) {
          t.fillStyle = i % 3 ? "rgba(80, 52, 24, .22)" : "rgba(255, 255, 255, .16)";
          dot(t, rnd() * 96, rnd() * 96, 0.5 + rnd() * 0.8);
        }
      });
    },
    cyan(ctx, W, H, s) {
      ground(ctx, W, H, "#1b3a7a");
      layer(ctx, W, H, s, 9, 9, (t) => { t.fillStyle = "rgba(255, 255, 255, .12)"; dot(t, 4.5, 4.5, 1.3); });
    },
    dots(ctx, W, H, s) {
      ground(ctx, W, H, "#151515");
      layer(ctx, W, H, s, 12, 12, (t) => { t.fillStyle = "#3d3d3d"; dot(t, 6, 6, 1.5); });
    },
    deck(ctx, W, H, s) {
      ground(ctx, W, H, "#f2ead8");
      layer(ctx, W, H, s, 56, 8, (t) => { t.fillStyle = "#2f5d50"; t.fillRect(28, 0, 28, 8); });
    },
    linen(ctx, W, H, s) {
      ground(ctx, W, H, "#6f7a52");
      layer(ctx, W, H, s, 12, 12, (t) => {
        t.fillStyle = "rgba(255, 255, 255, .06)";
        for (let y = 0; y < 12; y += 3) t.fillRect(0, y, 12, 1);
        t.fillStyle = "rgba(0, 0, 0, .08)";
        for (let x = 0; x < 12; x += 4) t.fillRect(x, 0, 1, 12);
      });
    },
    seigaiha(ctx, W, H, s) {
      ground(ctx, W, H, "#24506a");
      // overlapping waves, each row over the one above it
      layer(ctx, W, H, s, 24, 12, (t) => {
        t.strokeStyle = "rgba(240, 232, 214, .4)";
        t.lineWidth = 0.9;
        for (let row = -2; row <= 4; row++) {
          const y = row * 6;
          for (const x of row % 2 ? [-12, 12, 36] : [0, 24]) {
            t.fillStyle = "#24506a";
            dot(t, x, y, 11.5);
            for (const r of [11, 7.5, 4]) { t.beginPath(); t.arc(x, y, r, Math.PI, 0); t.stroke(); }
          }
        }
      });
    },
    xuan(ctx, W, H, s) {
      ground(ctx, W, H, "#efe9db");
      layer(ctx, W, H, s, 6, 6, (t) => { t.fillStyle = "rgba(120, 98, 66, .06)"; t.fillRect(0, 0, 6, 1); }, 115);
      let seed = 11;
      const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647);
      layer(ctx, W, H, s, 80, 80, (t) => {
        t.fillStyle = "rgba(120, 98, 66, .16)";
        for (let i = 0; i < 70; i++) dot(t, rnd() * 80, rnd() * 80, 0.4 + rnd() * 0.7);
      });
    },
    zigzag(ctx, W, H, s) {
      ground(ctx, W, H, "#d6c4a2");
      layer(ctx, W, H, s, 32, 16, (t) => {
        t.fillStyle = "#ebdfc8";
        t.beginPath();
        t.moveTo(0, 8); t.lineTo(16, 0); t.lineTo(32, 8); t.lineTo(32, 16); t.lineTo(16, 8); t.lineTo(0, 16);
        t.closePath();
        t.fill();
      });
    },
    terracotta(ctx, W, H, s) {
      ground(ctx, W, H, "#bf6748");
      layer(ctx, W, H, s, 10, 10, (t) => { t.fillStyle = "rgba(255, 236, 214, .22)"; dot(t, 5, 5, 1.1); });
    },
    lilac(ctx, W, H, s) {
      ground(ctx, W, H, "#d9d3e6");
      layer(ctx, W, H, s, 10, 10, (t) => { t.fillStyle = "rgba(78, 60, 120, .14)"; t.fillRect(0, 0, 10, 1); }, 45);
    },
    velvet(ctx, W, H) {
      ground(ctx, W, H, "#17322a");
      const r = Math.max(W, H) * 0.9;
      const g = ctx.createRadialGradient(W * 0.3, H * 0.15, 0, W * 0.3, H * 0.15, r);
      g.addColorStop(0, "rgba(80, 140, 110, .45)");
      g.addColorStop(0.7, "rgba(80, 140, 110, 0)");
      ground(ctx, W, H, g);
    },
    polka(ctx, W, H, s) {
      ground(ctx, W, H, "#f4e4cf");
      layer(ctx, W, H, s, 22, 22, (t) => {
        t.fillStyle = "#e0795a";
        for (const [x, y] of [[0, 0], [22, 0], [0, 22], [22, 22], [11, 11]]) dot(t, x, y, 3);
      });
    },
  };

  // a picture covering the card, softened by blur and darkened by dim
  function photo(ctx, W, H, scale, img, blur, dim) {
    const iw = img.naturalWidth || img.width, ih = img.naturalHeight || img.height;
    const k = Math.max(W / iw, H / ih);
    const dw = iw * k, dh = ih * k;
    if (blur > 0) {
      // blur by drawing small and scaling back up: canvas filters are not in
      // every webview
      const f = 1 / (1 + blur * 0.55);
      const w = Math.max(2, Math.round(W * scale * f)), h = Math.max(2, Math.round(H * scale * f));
      const small = tile(1, w, h, (t) => {
        t.imageSmoothingQuality = "high";
        t.drawImage(img, (W - dw) / 2 * scale * f, (H - dh) / 2 * scale * f, dw * scale * f, dh * scale * f);
      });
      ctx.save();
      ctx.imageSmoothingQuality = "high";
      // a little past the edges, so the blur has no light rim
      const m = blur * 1.5;
      ctx.drawImage(small, -m, -m, W + m * 2, H + m * 2);
      ctx.restore();
    } else {
      ctx.drawImage(img, (W - dw) / 2, (H - dh) / 2, dw, dh);
    }
    if (dim > 0) ground(ctx, W, H, `rgba(0, 0, 0, ${dim / 100})`);
  }

  // Draw the backdrop for settings o on ctx (already at scale), W by H.
  // img: the photo's or the reader's picture, loaded; teal stands in for
  // one that isn't there.
  function paint(ctx, W, H, scale, o, img) {
    ctx.save();
    const m = meta(o.bg);
    if (m.photo && img) photo(ctx, W, H, scale, img, o.blur, o.dim);
    else (PATTERNS[o.bg] || PATTERNS.teal)(ctx, W, H, scale);
    ctx.restore();
  }

  globalThis.DayBackgrounds = { list: LIST, meta, norm, paint, PAD, BLUR, DIM };
})();
