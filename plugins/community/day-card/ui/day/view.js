// 移植自 magpie @magpie-community/page-day-card 的小票文案和排版数据。
// 数字来自 AxonHub 已经聚合好的用量分析：API Key 当作 Agent，模型挂在 Key 下面。
// 不在页面里把 usage log 加总。经典脚本，挂到 globalThis.DayView，测试可以直接加载。
(() => {
  const TEXT = {
    en: {
      title: "DAILY AI STATEMENT", tagline: "Routed through AxonHub.",
      date: "DATE", issued: "ISSUED", at: "AT",
      total: "TOTAL API COST", totalNote: "API-EQUIVALENT COST · NOT AN INVOICE",
      streak: "STREAK", days: (n) => (n === 1 ? "1 DAY" : `${n} DAYS`), requests: "REQUESTS", errors: "ERRORS", vsAverage: "VS AVERAGE DAY",
      agentModel: "API KEY / MODEL", cost: "COST",
      totalTokens: "TOTAL TOKENS", input: "INPUT", output: "OUTPUT", hit: "CACHE HIT RATE",
      footer: [["Routed through ", "AxonHub", ""], ["Each API key, then the models it called"]],
      noCalls: "No calls on this day.", more: (n) => `+${n} MORE`, unpriced: (n) => (n === 1 ? "1 request without a price" : `${n} requests without a price`),
      noAgent: "NO API KEY",
      today: "Today", prev: "Previous day", next: "Next day", copy: "Copy image", copied: "Copied", save: "Save PNG", saved: "Saved",
      failed: "Couldn't read usage", shareFailed: "Couldn't share the picture", bridge: "AxonHub bridge did not load.",
      loading: "Reading usage…",
      background: "Background", padding: "Padding", blur: "Blur", dim: "Dim", upload: "Add your own picture", imageFailed: "Couldn't use that picture", keepFailed: "Couldn't keep the background",
      "bg-teal": "Teal", "bg-gingham": "Gingham", "bg-grid": "Graph paper", "bg-kraft": "Kraft paper", "bg-cyan": "Cyanotype", "bg-dots": "Night dots", "bg-deck": "Deckchair stripes", "bg-linen": "Olive linen", "bg-seigaiha": "Seigaiha waves", "bg-xuan": "Xuan paper", "bg-zigzag": "Zigzag", "bg-terracotta": "Terracotta", "bg-lilac": "Lilac lines", "bg-velvet": "Green velvet", "bg-polka": "Apricot polka", "bg-alpine": "Alpine dawn", "bg-sea": "Misty shore", "bg-dunes": "Dunes", "bg-lake": "Forest lake", "bg-custom": "Your picture",
    },
    zh: {
      title: "每日 AI 账单", tagline: "经 AxonHub 路由",
      date: "日期", issued: "开具", at: "",
      total: "API 总费用", totalNote: "按 API 价格折算 · 非发票",
      streak: "连续使用", days: (n) => `${n} 天`, requests: "请求数", errors: "错误", vsAverage: "对比日均",
      agentModel: "API KEY / 模型", cost: "费用",
      totalTokens: "TOKEN 总量", input: "输入", output: "输出", hit: "缓存命中率",
      footer: [["经 ", "AxonHub", " 路由"], ["每把 API Key，以及它调用的模型"]],
      noCalls: "这一天没有调用。", more: (n) => `还有 ${n} 个`, unpriced: (n) => `${n} 次请求没有价格`,
      noAgent: "未关联 API KEY",
      today: "今天", prev: "前一天", next: "后一天", copy: "复制图片", copied: "已复制", save: "保存 PNG", saved: "已保存",
      failed: "读取用量失败", shareFailed: "分享图片失败", bridge: "AxonHub bridge 没有加载。",
      loading: "正在读取用量…",
      background: "背景", padding: "边距", blur: "模糊", dim: "压暗", upload: "用自己的图片", imageFailed: "这张图片用不了", keepFailed: "背景没能保存",
      "bg-teal": "青绿", "bg-gingham": "野餐格", "bg-grid": "方格纸", "bg-kraft": "牛皮纸", "bg-cyan": "蓝晒", "bg-dots": "夜色网点", "bg-deck": "躺椅条纹", "bg-linen": "橄榄亚麻", "bg-seigaiha": "青海波", "bg-xuan": "宣纸", "bg-zigzag": "人字纹", "bg-terracotta": "赤陶", "bg-lilac": "雾紫细线", "bg-velvet": "墨绿丝绒", "bg-polka": "杏色波点", "bg-alpine": "雪山晨光", "bg-sea": "雾中海岸", "bg-dunes": "沙丘", "bg-lake": "林间湖", "bg-custom": "自己的图片",
    },
    "zh-TW": {
      title: "每日 AI 帳單", tagline: "經 AxonHub 路由",
      date: "日期", issued: "開立", at: "",
      total: "API 總費用", totalNote: "按 API 價格折算 · 非發票",
      streak: "連續使用", days: (n) => `${n} 天`, requests: "請求數", errors: "錯誤", vsAverage: "對比日均",
      agentModel: "API KEY / 模型", cost: "費用",
      totalTokens: "TOKEN 總量", input: "輸入", output: "輸出", hit: "快取命中率",
      footer: [["經 ", "AxonHub", " 路由"], ["每把 API Key，以及它呼叫的模型"]],
      noCalls: "這一天沒有呼叫。", more: (n) => `還有 ${n} 個`, unpriced: (n) => `${n} 次請求沒有價格`,
      noAgent: "未關聯 API KEY",
      today: "今天", prev: "前一天", next: "後一天", copy: "複製圖片", copied: "已複製", save: "儲存 PNG", saved: "已儲存",
      failed: "讀取用量失敗", shareFailed: "分享圖片失敗", bridge: "AxonHub bridge 沒有載入。",
      loading: "正在讀取用量…",
      background: "背景", padding: "邊距", blur: "模糊", dim: "壓暗", upload: "用自己的圖片", imageFailed: "這張圖片用不了", keepFailed: "背景沒能儲存",
      "bg-teal": "青綠", "bg-gingham": "野餐格", "bg-grid": "方格紙", "bg-kraft": "牛皮紙", "bg-cyan": "藍曬", "bg-dots": "夜色網點", "bg-deck": "躺椅條紋", "bg-linen": "橄欖亞麻", "bg-seigaiha": "青海波", "bg-xuan": "宣紙", "bg-zigzag": "人字紋", "bg-terracotta": "赤陶", "bg-lilac": "霧紫細線", "bg-velvet": "墨綠絲絨", "bg-polka": "杏色波點", "bg-alpine": "雪山晨光", "bg-sea": "霧中海岸", "bg-dunes": "沙丘", "bg-lake": "林間湖", "bg-custom": "自己的圖片",
    },
    ja: {
      title: "1日の AI 明細", tagline: "AxonHub 経由",
      date: "日付", issued: "発行", at: "",
      total: "API 合計コスト", totalNote: "API 料金換算 · 請求書ではありません",
      streak: "連続日数", days: (n) => `${n} 日`, requests: "リクエスト", errors: "エラー", vsAverage: "平均日との比較",
      agentModel: "API キー / モデル", cost: "コスト",
      totalTokens: "合計トークン", input: "入力", output: "出力", hit: "キャッシュヒット率",
      footer: [["", "AxonHub", " 経由"], ["各 API キーと、そのモデル"]],
      noCalls: "この日の呼び出しはありません。", more: (n) => `ほか ${n} 件`, unpriced: (n) => `価格のないリクエスト ${n} 件`,
      noAgent: "API キーなし",
      today: "今日", prev: "前の日", next: "次の日", copy: "画像をコピー", copied: "コピーしました", save: "PNG を保存", saved: "保存しました",
      failed: "使用量を読み込めませんでした", shareFailed: "画像を共有できませんでした", bridge: "AxonHub bridge が読み込まれていません。",
      loading: "使用量を読み込んでいます…",
      background: "背景", padding: "余白", blur: "ぼかし", dim: "暗さ", upload: "自分の画像を使う", imageFailed: "この画像は使えません", keepFailed: "背景を保存できませんでした",
      "bg-teal": "ティール", "bg-gingham": "ギンガム", "bg-grid": "方眼紙", "bg-kraft": "クラフト紙", "bg-cyan": "青写真", "bg-dots": "夜のドット", "bg-deck": "デッキチェア", "bg-linen": "オリーブリネン", "bg-seigaiha": "青海波", "bg-xuan": "宣紙", "bg-zigzag": "ジグザグ", "bg-terracotta": "テラコッタ", "bg-lilac": "ライラック", "bg-velvet": "深緑ベルベット", "bg-polka": "アプリコット水玉", "bg-alpine": "雪山の朝", "bg-sea": "霧の海辺", "bg-dunes": "砂丘", "bg-lake": "森の湖", "bg-custom": "自分の画像",
    },
    de: {
      title: "TÄGLICHE KI-ABRECHNUNG", tagline: "Geroutet über AxonHub.",
      date: "DATUM", issued: "AUSGESTELLT", at: "UM",
      total: "API-GESAMTKOSTEN", totalNote: "API-ÄQUIVALENT · KEINE RECHNUNG",
      streak: "SERIE", days: (n) => (n === 1 ? "1 TAG" : `${n} TAGE`), requests: "ANFRAGEN", errors: "FEHLER", vsAverage: "VS. DURCHSCHNITTSTAG",
      agentModel: "API-SCHLÜSSEL / MODELL", cost: "KOSTEN",
      totalTokens: "TOKENS GESAMT", input: "EINGABE", output: "AUSGABE", hit: "CACHE-TREFFERQUOTE",
      footer: [["Geroutet über ", "AxonHub", ""], ["Jeder API-Schlüssel, dann seine Modelle"]],
      noCalls: "An diesem Tag keine Aufrufe.", more: (n) => `+${n} WEITERE`, unpriced: (n) => (n === 1 ? "1 Anfrage ohne Preis" : `${n} Anfragen ohne Preis`),
      noAgent: "OHNE API-SCHLÜSSEL",
      today: "Heute", prev: "Vorheriger Tag", next: "Nächster Tag", copy: "Bild kopieren", copied: "Kopiert", save: "PNG sichern", saved: "Gesichert",
      failed: "Nutzung nicht lesbar", shareFailed: "Bild nicht teilbar", bridge: "AxonHub-Bridge wurde nicht geladen.",
      loading: "Nutzung wird gelesen…",
      background: "Hintergrund", padding: "Rand", blur: "Weichzeichnen", dim: "Abdunkeln", upload: "Eigenes Bild wählen", imageFailed: "Dieses Bild geht nicht", keepFailed: "Hintergrund nicht gesichert",
      "bg-teal": "Türkis", "bg-gingham": "Vichy-Karo", "bg-grid": "Millimeterpapier", "bg-kraft": "Packpapier", "bg-cyan": "Cyanotypie", "bg-dots": "Nachtpunkte", "bg-deck": "Liegestuhlstreifen", "bg-linen": "Olivleinen", "bg-seigaiha": "Seigaiha", "bg-xuan": "Xuan-Papier", "bg-zigzag": "Zickzack", "bg-terracotta": "Terrakotta", "bg-lilac": "Fliederlinien", "bg-velvet": "Grüner Samt", "bg-polka": "Aprikosenpunkte", "bg-alpine": "Alpenmorgen", "bg-sea": "Nebelküste", "bg-dunes": "Dünen", "bg-lake": "Waldsee", "bg-custom": "Eigenes Bild",
    },
  };

  // AxonHub locales; anything else reads English. zh-CN stays 简体.
  function langOf(lang) {
    const l = String(lang || "").toLowerCase();
    if (l.startsWith("zh")) return /^zh([-_](tw|hk|mo|hant)|-hant)/.test(l) ? "zh-TW" : "zh";
    if (l.startsWith("ja")) return "ja";
    if (l.startsWith("de")) return "de";
    return "en";
  }
  const text = (lang) => TEXT[langOf(lang)];

  function count(n) {
    n = Number(n) || 0;
    const a = Math.abs(n);
    if (a >= 1e9) return (n / 1e9).toFixed(a >= 1e10 ? 1 : 2) + "B";
    if (a >= 1e6) return (n / 1e6).toFixed(a >= 1e8 ? 0 : a >= 1e7 ? 1 : 2) + "M";
    if (a >= 1e5) return (n / 1e3).toFixed(0) + "K";
    return Math.round(n).toLocaleString("en-US");
  }

  // Cost is already in the AxonHub system currency. There is no FX rate.
  function money(amount, currency) {
    const code = String(currency || "USD").trim().toUpperCase() || "USD";
    const n = Number(amount) || 0;
    const sign = code === "CNY" ? "¥" : code === "USD" ? "$" : "";
    const body = (value) => value.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    if (!sign) {
      try {
        if (n > 0 && n < 0.01) {
          const tiny = new Intl.NumberFormat("en-US", { style: "currency", currency: code, minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(0.01);
          return "<" + tiny;
        }
        return new Intl.NumberFormat("en-US", { style: "currency", currency: code, minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(n);
      } catch {
        // unknown code: fall through to a prefixed number
      }
    }
    if (n === 0) return (sign || code + " ") + "0.00";
    if (n > 0 && n < 0.01) return "<" + (sign || code + " ") + "0.01";
    return (sign || code + " ") + body(n);
  }

  const tokensOf = (t) => (Number(t?.input) || 0) + (Number(t?.output) || 0) + (Number(t?.cacheRead) || 0) + (Number(t?.cacheWrite) || 0);

  function hitRate(t) {
    const read = Number(t?.cacheRead) || 0;
    const prompt = (Number(t?.input) || 0) + read + (Number(t?.cacheWrite) || 0) - (Number(t?.cacheUnknown) || 0);
    if (prompt <= 0) return null;
    return Math.min(1, read / prompt);
  }

  function percent(r) {
    if (r == null) return "–";
    const p = r * 100;
    return (p > 0 && p < 1 ? p.toFixed(1) : Math.round(p)) + "%";
  }

  function shift(date, n) {
    const [y, m, d] = String(date).split("-").map(Number);
    return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
  }

  function dateLabel(date, lang, short) {
    const [y, m, d] = String(date).split("-").map(Number);
    if (!y || !m || !d) return String(date || "");
    const at = new Date(Date.UTC(y, m - 1, d, 12));
    const o = short ? { month: "short", day: "numeric", timeZone: "UTC" } : { weekday: "short", year: "numeric", month: "short", day: "numeric", timeZone: "UTC" };
    try {
      return at.toLocaleDateString(langOf(lang) === "zh-TW" ? "zh-TW" : langOf(lang), o);
    } catch {
      return date;
    }
  }

  function receiptDate(date, lang) {
    const [y, m, d] = String(date).split("-").map(Number);
    if (!y || !m || !d) return String(date || "");
    try {
      return new Date(Date.UTC(y, m - 1, d, 12)).toLocaleDateString(langOf(lang) === "zh-TW" ? "zh-TW" : langOf(lang), { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" }).toLocaleUpperCase();
    } catch {
      return date;
    }
  }

  function issued(now, lang) {
    const p = (n) => String(n).padStart(2, "0");
    const day = `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
    const at = text(lang).at;
    return [receiptDate(day, lang), at, `${p(now.getHours())}:${p(now.getMinutes())}`].filter(Boolean).join(" ");
  }

  function streak(days, date, today) {
    const had = new Set((days || []).filter((d) => Number(d.calls) > 0).map((d) => d.date));
    let at = date;
    if (!had.has(at) && at === today) at = shift(at, -1);
    let n = 0;
    while (had.has(at)) {
      n++;
      at = shift(at, -1);
    }
    return n;
  }

  function average(days, date) {
    const list = (days || []).filter((d) => d.date < date && Number(d.calls) > 0);
    if (!list.length) return null;
    const sum = list.reduce((s, d) => ({ tokens: s.tokens + (Number(d.tokens) || 0), cost: s.cost + (Number(d.cost) || 0) }), { tokens: 0, cost: 0 });
    return { days: list.length, tokens: sum.tokens / list.length, cost: sum.cost / list.length };
  }

  function versus(day, avg) {
    if (!avg) return "–";
    let r = 0;
    if (avg.cost > 0 && Number(day.cost) > 0) r = Number(day.cost) / avg.cost;
    else if (avg.tokens > 0) r = tokensOf(day) / avg.tokens;
    else return "–";
    if (r > 0 && r < 0.1) return "<0.1×";
    return (r >= 100 ? Math.round(r).toLocaleString("en-US") : r.toFixed(1)) + "×";
  }

  const byCost = (a, b) => (Number(b.cost) || 0) - (Number(a.cost) || 0) || tokensOf(b) - tokensOf(a);
  const upper = (s) => String(s).toLocaleUpperCase();

  function card(day, lang, limits = {}, now = new Date()) {
    const t = text(lang);
    const maxAgents = limits.agents ?? 8;
    const maxModels = limits.models ?? 5;
    const cur = day.currency;
    const calls = Number(day.calls) || 0;
    const errors = Number(day.errors) || 0;
    const agents = (Array.isArray(day.agents) ? day.agents : []).slice().sort(byCost);
    const shown = agents.slice(0, maxAgents).map((a) => {
      const models = (Array.isArray(a.models) ? a.models : []).slice().sort(byCost);
      const hidden = Math.max(0, models.length - maxModels) + (Number(a.moreModels) || 0);
      return {
        name: upper(a.id ? a.name || a.id : t.noAgent),
        icon: a.icon || "",
        value: money(a.cost, cur),
        models: models.slice(0, maxModels).map((m) => ({ name: upper(m.model || "?"), icon: m.icon || "", value: money(m.cost, cur) })),
        more: hidden ? t.more(hidden) : "",
      };
    });
    const hiddenAgents = Math.max(0, agents.length - maxAgents) + (Number(day.moreAgents) || 0);
    const stats = [
      { label: t.streak, value: t.days(streak(day.days, day.date, day.today)) },
      { label: t.requests, value: calls.toLocaleString("en-US") },
    ];
    if (errors) stats.push({ label: t.errors, value: errors.toLocaleString("en-US") });
    stats.push({ label: t.vsAverage, value: versus(day, average(day.days, day.date)) });
    return {
      title: t.title,
      tagline: t.tagline,
      head: [
        { label: t.date, value: receiptDate(day.date, lang) },
        { label: t.issued, value: issued(now, lang) },
      ],
      totalLabel: t.total,
      total: money(day.cost, cur),
      totalNote: t.totalNote,
      stats,
      agentHead: [t.agentModel, t.cost],
      agents: shown,
      moreAgents: hiddenAgents ? t.more(hiddenAgents) : "",
      tokens: [
        { label: t.totalTokens, value: count(tokensOf(day)) },
        { label: t.input, value: count(day.input) },
        { label: t.output, value: count(day.output) },
        { label: t.hit, value: percent(hitRate(day)) },
      ],
      note: Number(day.unpriced) > 0 ? t.unpriced(Number(day.unpriced)) : "",
      empty: calls === 0 ? t.noCalls : "",
      footer: t.footer,
    };
  }

  const fileName = (date) => `axonhub ${date}`;

  // analyticsDimensionStats returns a raw integer id. The filter wants a GUID.
  function apiKeyGUID(id) {
    const raw = String(id ?? "").trim();
    if (!raw) return "";
    if (raw.startsWith("gid://")) return raw;
    if (!/^\d+$/.test(raw)) return "";
    return `gid://axonhub/APIKey/${raw}`;
  }

  function zonedParts(date, timeZone) {
    const fmt = new Intl.DateTimeFormat("en-US", {
      timeZone: timeZone || "UTC",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    });
    const out = {};
    for (const part of fmt.formatToParts(date)) out[part.type] = part.value;
    if (out.hour === "24") out.hour = "00";
    return out;
  }

  function calendarDate(date, timeZone) {
    const p = zonedParts(date, timeZone);
    return `${p.year}-${p.month}-${p.day}`;
  }

  function zoneOffsetMs(date, timeZone) {
    const p = zonedParts(date, timeZone);
    const asUTC = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second);
    return asUTC - date.getTime();
  }

  // Midnight and the last millisecond of a calendar day in the analytics timezone.
  function startOfDay(date, timeZone) {
    const guess = new Date(`${date}T00:00:00Z`);
    const off = zoneOffsetMs(guess, timeZone);
    let start = new Date(guess.getTime() - off);
    const off2 = zoneOffsetMs(start, timeZone);
    if (off2 !== off) start = new Date(guess.getTime() - off2);
    return start;
  }

  function endOfDay(date, timeZone) {
    return new Date(startOfDay(shift(date, 1), timeZone).getTime() - 1);
  }

  // Prompt tokens from analytics include the cached portion. Split them so the
  // receipt can add cache read without counting it twice.
  function asUsage(stat) {
    const statIn = stat || {};
    const cacheRead = Number(statIn.cachedInputTokens ?? statIn.totalCachedInputTokens) || 0;
    const output = Number(statIn.outputTokens ?? statIn.totalOutputTokens) || 0;
    const input = statIn.totalUncachedInputTokens != null
      ? Number(statIn.totalUncachedInputTokens) || 0
      : Math.max(0, (Number(statIn.inputTokens ?? statIn.totalInputTokens) || 0) - cacheRead);
    return {
      input,
      output,
      cacheRead,
      cacheWrite: 0,
      cost: Number(statIn.cost ?? statIn.totalCost) || 0,
      calls: Number(statIn.requestCount ?? statIn.totalRequests) || 0,
    };
  }

  function withRemainder(keys, overview) {
    const list = (keys || []).slice();
    const all = asUsage(overview);
    const sum = list.reduce((s, key) => {
      const u = asUsage(key);
      s.calls += u.calls;
      s.cost += u.cost;
      s.input += u.input;
      s.output += u.output;
      s.cacheRead += u.cacheRead;
      return s;
    }, { calls: 0, cost: 0, input: 0, output: 0, cacheRead: 0 });
    const restCalls = all.calls - sum.calls;
    const restCost = all.cost - sum.cost;
    if (restCalls > 0 || restCost > 0.000001) {
      const input = Math.max(0, all.input - sum.input);
      const cacheRead = Math.max(0, all.cacheRead - sum.cacheRead);
      list.push({
        id: "",
        name: "",
        requestCount: Math.max(0, restCalls),
        cost: Math.max(0, restCost),
        inputTokens: input + cacheRead,
        cachedInputTokens: cacheRead,
        outputTokens: Math.max(0, all.output - sum.output),
      });
    }
    return list;
  }

  function rankedKeys(keys, overview, limit = 8) {
    const ranked = withRemainder(keys, overview)
      .map((raw) => ({ raw, usage: asUsage(raw) }))
      .sort((a, b) => byCost(a.usage, b.usage));
    return { shown: ranked.slice(0, limit), hidden: Math.max(0, ranked.length - limit) };
  }

  function toModel(row) {
    const t = asUsage(row);
    return { model: row.name || row.id || "?", icon: "", cost: t.cost, input: t.input, output: t.output, cacheRead: t.cacheRead, cacheWrite: 0 };
  }

  function toAgent(key, models) {
    const t = asUsage(key);
    return {
      id: key.id || "",
      name: key.name || "",
      icon: "",
      calls: t.calls,
      cost: t.cost,
      input: t.input,
      output: t.output,
      cacheRead: t.cacheRead,
      cacheWrite: 0,
      models: (models || []).map(toModel),
    };
  }

  // One day, in the shape card() already draws. keys and models are server aggregates.
  function assemble(input) {
    const overview = asUsage(input.overview);
    const ranked = rankedKeys(input.keys, input.overview, input.limit ?? 8);
    const modelsByKey = input.modelsByKey || {};
    const days = (input.days || []).filter((d) => Number(d.requestCount) > 0).map((d) => ({
      date: d.date,
      calls: Number(d.requestCount) || 0,
      tokens: (Number(d.inputTokens) || 0) + (Number(d.outputTokens) || 0),
      cost: Number(d.cost) || 0,
    }));
    return {
      date: input.date,
      today: input.today,
      calls: overview.calls,
      errors: Number(input.errors) || 0,
      input: overview.input,
      output: overview.output,
      cacheRead: overview.cacheRead,
      cacheWrite: 0,
      cacheUnknown: 0,
      cost: overview.cost,
      unpriced: Number(input.unpriced) || 0,
      agents: ranked.shown.map((item) => toAgent(item.raw, modelsByKey[item.raw.id] || [])),
      moreAgents: ranked.hidden,
      days,
      currency: input.currency || "USD",
    };
  }

  const HISTORY_QUERY = `query DayHistory($filter: AnalyticsFilter!) {
    analyticsDailyStats(filter: $filter) {
      date inputTokens cachedInputTokens outputTokens totalTokens requestCount cost
    }
  }`;

  const DAY_QUERY = `query DayCard($filter: AnalyticsFilter!, $failed: RequestWhereInput!, $unpriced: UsageLogWhereInput!) {
    analyticsOverview(filter: $filter) {
      totalTokens totalInputTokens totalCachedInputTokens totalUncachedInputTokens totalOutputTokens totalRequests totalCost
    }
    analyticsDimensionStats(filter: $filter, dimension: "apiKey") {
      id name requestCount inputTokens cachedInputTokens outputTokens totalTokens cost
    }
    failed: requests(first: 1, where: $failed) { totalCount }
    unpriced: usageLogs(first: 1, where: $unpriced) { totalCount }
  }`;

  const SETTINGS_QUERY = `query DaySettings { systemGeneralSettings { currencyCode timezone } }`;

  const KV_READ = `query DayBackground { pluginKV(key: "background") { value } }`;

  const KV_WRITE = `mutation DayBackground($value: String!) { setPluginKV(key: "background", value: $value) { key } }`;

  function modelsQuery(count) {
    const vars = [];
    const fields = [];
    for (let i = 0; i < count; i++) {
      vars.push(`$f${i}: AnalyticsFilter!`);
      fields.push(`m${i}: analyticsDimensionStats(filter: $f${i}, dimension: "model") { id name cost requestCount inputTokens cachedInputTokens outputTokens totalTokens }`);
    }
    return `query DayModels(${vars.join(", ")}) { ${fields.join(" ")} }`;
  }

  globalThis.DayView = {
    TEXT, langOf, text, count, money, tokensOf, hitRate, percent, shift, dateLabel, receiptDate, issued, streak, average, versus, card, fileName,
    apiKeyGUID, calendarDate, startOfDay, endOfDay, asUsage, rankedKeys, assemble, modelsQuery,
    HISTORY_QUERY, DAY_QUERY, SETTINGS_QUERY, KV_READ, KV_WRITE,
  };
})();
