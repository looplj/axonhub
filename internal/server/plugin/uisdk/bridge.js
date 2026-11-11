(function () {
  "use strict";

  var PROTOCOL = "axonhub.plugin-ui";
  var VERSION = 1;
  var READY_TIMEOUT_MS = 10000;
  var CALL_TIMEOUT_MS = 15000;
  var MESSAGES = {
    INVALID_ARGUMENT: "Invalid argument",
    NOT_READY: "Not ready",
    UNAUTHENTICATED: "Unauthenticated",
    FORBIDDEN: "Forbidden",
    NOT_FOUND: "Not found",
    RATE_LIMITED: "Rate limited",
    TIMEOUT: "Timed out",
    DISPOSED: "Disposed",
    NETWORK_ERROR: "Network error",
    INVALID_STATE: "Invalid state",
    INTERNAL: "Internal error"
  };

  var parentOrigin = null;
  var sessionId = null;
  var initPayload = null;
  var currentTheme = null;
  var readyPromise = null;
  var readyResolve = null;
  var readyReject = null;
  var readyTimer = null;
  var readySettled = false;
  var helloSent = false;
  var disposed = false;
  var callSeq = 0;
  var pending = new Map();
  var initListeners = new Set();
  var themeListeners = new Set();
  // Keep this list identical to PLUGIN_UI_TOKEN_NAMES in frontend/src/features/plugins/plugin-ui.ts.
  var TOKEN_NAMES = [
    "--background",
    "--foreground",
    "--card",
    "--card-foreground",
    "--popover",
    "--popover-foreground",
    "--primary",
    "--primary-foreground",
    "--secondary",
    "--secondary-foreground",
    "--muted",
    "--muted-foreground",
    "--accent",
    "--accent-foreground",
    "--destructive",
    "--destructive-foreground",
    "--border",
    "--input",
    "--ring",
    "--radius",
    "--font-sans",
    "--font-mono",
    "--shadow-2xs",
    "--shadow-xs",
    "--shadow-sm",
    "--shadow",
    "--shadow-md",
    "--shadow-lg",
    "--shadow-xl",
    "--shadow-2xl"
  ];
  var TOKEN_VALUE_MAX = 200;

  function bridgeError(code, status) {
    var mapped = Object.prototype.hasOwnProperty.call(MESSAGES, code) ? code : "INTERNAL";
    var error = new Error(MESSAGES[mapped]);
    error.name = "AxonhubBridgeError";
    error.code = mapped;
    if (typeof status === "number") {
      error.status = status;
    }
    return error;
  }

  function errorFromHost(error) {
    var status = error && typeof error.status === "number" ? error.status : undefined;
    var code = error && typeof error.code === "string" ? error.code : "";
    var mapped = "INTERNAL";
    if (status === 401) {
      mapped = "UNAUTHENTICATED";
    } else if (status === 403) {
      mapped = "FORBIDDEN";
    } else if (status === 404) {
      mapped = "NOT_FOUND";
    } else if (status === 429) {
      mapped = "RATE_LIMITED";
    } else if (code === "TIMEOUT" || code === "NETWORK_ERROR" || code === "INVALID_STATE") {
      mapped = code;
    } else if (typeof status === "number") {
      mapped = "INTERNAL";
    } else if (Object.prototype.hasOwnProperty.call(MESSAGES, code)) {
      mapped = code;
    }
    return bridgeError(mapped, status);
  }

  function isPlainObject(value) {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      return false;
    }
    var proto = Object.getPrototypeOf(value);
    if (proto === Object.prototype || proto === null) {
      return true;
    }
    // A structured clone from the parent window is an ordinary object whose
    // prototype belongs to that realm, so it is not this realm's Object.prototype.
    return proto !== null && Object.getPrototypeOf(proto) === null;
  }

  function isJSONValue(value) {
    if (value === null) {
      return true;
    }
    var kind = typeof value;
    if (kind === "string" || kind === "boolean") {
      return true;
    }
    if (kind === "number") {
      return Number.isFinite(value);
    }
    if (Array.isArray(value)) {
      for (var i = 0; i < value.length; i++) {
        if (!isJSONValue(value[i])) {
          return false;
        }
      }
      return true;
    }
    if (!isPlainObject(value)) {
      return false;
    }
    var keys = Object.keys(value);
    for (var j = 0; j < keys.length; j++) {
      if (!isJSONValue(value[keys[j]])) {
        return false;
      }
    }
    return true;
  }

  function cloneJSON(value) {
    return JSON.parse(JSON.stringify(value));
  }

  function freezeDeep(value) {
    if (!value || typeof value !== "object") {
      return value;
    }
    Object.freeze(value);
    if (Array.isArray(value)) {
      for (var i = 0; i < value.length; i++) {
        freezeDeep(value[i]);
      }
      return value;
    }
    var keys = Object.keys(value);
    for (var j = 0; j < keys.length; j++) {
      freezeDeep(value[keys[j]]);
    }
    return value;
  }

  function isTokenValue(value) {
    if (typeof value !== "string" || value.length === 0 || value.length > TOKEN_VALUE_MAX) {
      return false;
    }
    for (var i = 0; i < value.length; i++) {
      var code = value.charCodeAt(i);
      if (code <= 31 || code === 127) {
        return false;
      }
    }
    return value.indexOf(";") === -1 && value.indexOf("{") === -1 && value.indexOf("}") === -1 &&
      value.indexOf("<") === -1 && value.indexOf(">") === -1 && value.indexOf("\\") === -1;
  }

  function sanitizeTokens(raw) {
    var tokens = {};
    if (!isPlainObject(raw)) {
      return tokens;
    }
    for (var i = 0; i < TOKEN_NAMES.length; i++) {
      var name = TOKEN_NAMES[i];
      if (isTokenValue(raw[name])) {
        tokens[name] = raw[name];
      }
    }
    return tokens;
  }

  function sameTokens(left, right) {
    for (var i = 0; i < TOKEN_NAMES.length; i++) {
      var name = TOKEN_NAMES[i];
      var a = left && Object.prototype.hasOwnProperty.call(left, name) ? left[name] : undefined;
      var b = right && Object.prototype.hasOwnProperty.call(right, name) ? right[name] : undefined;
      if (a !== b) {
        return false;
      }
    }
    return true;
  }

  function applyThemeStyle() {
    var root = typeof document !== "undefined" ? document.documentElement : null;
    if (!root || !root.style || typeof root.style.setProperty !== "function") {
      return;
    }
    var tokens = currentTheme && currentTheme.tokens ? currentTheme.tokens : {};
    for (var i = 0; i < TOKEN_NAMES.length; i++) {
      var name = TOKEN_NAMES[i];
      if (Object.prototype.hasOwnProperty.call(tokens, name)) {
        root.style.setProperty(name, tokens[name]);
      } else if (typeof root.style.removeProperty === "function") {
        root.style.removeProperty(name);
      }
    }
    var scheme = currentTheme && currentTheme.theme === "dark" ? "dark" : "light";
    root.style.setProperty("color-scheme", scheme);
  }

  function themeSnapshot() {
    return freezeDeep({
      theme: currentTheme.theme,
      colorScheme: currentTheme.colorScheme,
      tokens: cloneJSON(currentTheme.tokens || {})
    });
  }

  function ensureOrigin() {
    if (parentOrigin) {
      return parentOrigin;
    }
    var href = window.location && window.location.href;
    if (typeof href !== "string" || href === "") {
      return null;
    }
    var url;
    try {
      url = new URL(href);
    } catch (err) {
      return null;
    }
    if ((url.protocol !== "http:" && url.protocol !== "https:") || !url.origin || url.origin === "null") {
      return null;
    }
    parentOrigin = url.origin;
    return parentOrigin;
  }

  function post(message) {
    if (!parentOrigin || !window.parent || typeof window.parent.postMessage !== "function") {
      return false;
    }
    try {
      window.parent.postMessage(message, parentOrigin);
    } catch (err) {
      return false;
    }
    return true;
  }

  function reportPageError(message) {
    if (disposed || !sessionId || !parentOrigin) {
      return;
    }
    var text = typeof message === "string" && message ? message : "Error";
    post({
      protocol: PROTOCOL,
      version: VERSION,
      type: "pageError",
      sessionId: sessionId,
      payload: { message: text }
    });
  }

  function safeCall(handler, snapshot) {
    try {
      handler(snapshot);
    } catch (err) {
      reportPageError(err && typeof err.message === "string" && err.message ? err.message : "Error");
    }
  }

  function clearReadyTimer() {
    if (readyTimer != null) {
      window.clearTimeout(readyTimer);
      readyTimer = null;
    }
  }

  function failReady(error) {
    if (readySettled) {
      return;
    }
    readySettled = true;
    clearReadyTimer();
    if (readyReject) {
      readyReject(error);
    }
  }

  function notifyInit(snapshot) {
    var handlers = Array.from(initListeners);
    for (var i = 0; i < handlers.length; i++) {
      safeCall(handlers[i], snapshot);
    }
  }

  function notifyTheme() {
    var snapshot = themeSnapshot();
    var handlers = Array.from(themeListeners);
    for (var i = 0; i < handlers.length; i++) {
      safeCall(handlers[i], snapshot);
    }
  }

  function acceptInit(data) {
    if (readySettled || disposed) {
      return;
    }
    if (typeof data.sessionId !== "string" || data.sessionId === "") {
      return;
    }
    if (!isPlainObject(data.payload)) {
      return;
    }
    sessionId = data.sessionId;
    var payload = cloneJSON(data.payload);
    payload.tokens = sanitizeTokens(payload.tokens);
    initPayload = freezeDeep(payload);
    currentTheme = {
      theme: initPayload.theme,
      colorScheme: initPayload.colorScheme,
      tokens: initPayload.tokens
    };
    applyThemeStyle();
    readySettled = true;
    clearReadyTimer();
    if (readyResolve) {
      readyResolve(initPayload);
    }
    notifyInit(initPayload);
    notifyTheme();
  }

  function acceptReply(data) {
    if (!sessionId || data.sessionId !== sessionId) {
      return;
    }
    if (typeof data.id !== "string" && typeof data.id !== "number") {
      return;
    }
    var item = pending.get(String(data.id));
    if (!item) {
      return;
    }
    pending.delete(String(data.id));
    window.clearTimeout(item.timer);
    if (data.ok === true) {
      item.resolve(data.result == null ? undefined : data.result);
      return;
    }
    item.reject(errorFromHost(data.error));
  }

  function acceptTheme(data) {
    if (disposed || !sessionId || data.sessionId !== sessionId || !isPlainObject(data.payload)) {
      return;
    }
    var tokens = sanitizeTokens(data.payload.tokens);
    if (
      currentTheme &&
      currentTheme.theme === data.payload.theme &&
      currentTheme.colorScheme === data.payload.colorScheme &&
      sameTokens(currentTheme.tokens, tokens)
    ) {
      return;
    }
    currentTheme = {
      theme: data.payload.theme,
      colorScheme: data.payload.colorScheme,
      tokens: tokens
    };
    applyThemeStyle();
    notifyTheme();
  }

  function onMessage(event) {
    if (disposed || !event || event.source !== window.parent) {
      return;
    }
    if (!parentOrigin || event.origin !== parentOrigin) {
      return;
    }
    var data = event.data;
    if (!data || typeof data !== "object") {
      return;
    }
    if (data.protocol !== PROTOCOL || data.version !== VERSION) {
      return;
    }
    if (data.type === "init") {
      acceptInit(data);
    } else if (data.type === "reply") {
      acceptReply(data);
    } else if (data.type === "theme") {
      acceptTheme(data);
    }
  }

  function onError(event) {
    reportPageError(event && typeof event.message === "string" ? event.message : "Error");
  }

  function onRejection(event) {
    var reason = event && event.reason;
    var message = "Unhandled rejection";
    if (typeof reason === "string") {
      message = reason;
    } else if (reason && typeof reason.message === "string" && reason.message) {
      message = reason.message;
    }
    reportPageError(message);
  }

  function dispose() {
    if (disposed) {
      return;
    }
    disposed = true;
    window.removeEventListener("pagehide", dispose);
    window.removeEventListener("message", onMessage);
    window.removeEventListener("error", onError);
    window.removeEventListener("unhandledrejection", onRejection);
    clearReadyTimer();
    if (!readySettled && readyReject) {
      readySettled = true;
      readyReject(bridgeError("DISPOSED"));
    }
    pending.forEach(function (item) {
      window.clearTimeout(item.timer);
      item.reject(bridgeError("DISPOSED"));
    });
    pending.clear();
    initListeners.clear();
    themeListeners.clear();
  }

  function ready() {
    if (readyPromise) {
      return readyPromise;
    }
    if (disposed) {
      readySettled = true;
      readyPromise = Promise.reject(bridgeError("DISPOSED"));
      return readyPromise;
    }
    if (!ensureOrigin()) {
      readySettled = true;
      readyPromise = Promise.reject(bridgeError("INTERNAL"));
      return readyPromise;
    }
    readyPromise = new Promise(function (resolve, reject) {
      readyResolve = resolve;
      readyReject = reject;
    });
    readyTimer = window.setTimeout(function () {
      failReady(bridgeError("TIMEOUT"));
    }, READY_TIMEOUT_MS);
    if (!helloSent) {
      helloSent = true;
      post({
        protocol: PROTOCOL,
        version: VERSION,
        type: "hello"
      });
    }
    return readyPromise;
  }

  function onInit(handler) {
    if (typeof handler !== "function") {
      return function () {};
    }
    initListeners.add(handler);
    if (readySettled && initPayload) {
      var snapshot = initPayload;
      window.queueMicrotask(function () {
        if (disposed || !initListeners.has(handler)) {
          return;
        }
        safeCall(handler, snapshot);
      });
    }
    return function unsubscribe() {
      initListeners.delete(handler);
    };
  }

  function onTheme(handler) {
    if (typeof handler !== "function") {
      return function () {};
    }
    themeListeners.add(handler);
    if (readySettled && currentTheme) {
      window.queueMicrotask(function () {
        if (disposed || !themeListeners.has(handler)) {
          return;
        }
        safeCall(handler, themeSnapshot());
      });
    }
    return function unsubscribe() {
      themeListeners.delete(handler);
    };
  }

  function declaredOperations(query) {
    // Strings and comments are tokens so braces/keywords inside them are ignored.
    var tokens = String(query).match(/#[^\n]*|"""(?:\\[\s\S]|[^\\])*?"""|"(?:\\.|[^"\\])*"|[A-Za-z_][A-Za-z0-9_]*|[{}()]/g) || [];
    var count = 0;
    var braces = 0;
    var parens = 0;
    var definitionStart = true;
    for (var i = 0; i < tokens.length; i++) {
      var token = tokens[i];
      if (token[0] === '"' || token[0] === '#') continue;
      if (definitionStart) {
        if (token === "query" || token === "mutation" || token === "subscription" || token === "{") count++;
        definitionStart = false;
      }
      if (token === "(") parens++;
      if (token === ")") parens--;
      if (token === "{") braces++;
      if (token === "}") {
        braces--;
        if (braces === 0 && parens === 0) definitionStart = true;
      }
    }
    return count;
  }

  function sendCall(method, params) {
    callSeq += 1;
    var id = "c" + callSeq;
    return new Promise(function (resolve, reject) {
      var timer = window.setTimeout(function () {
        if (!pending.has(id)) {
          return;
        }
        pending.delete(id);
        reject(bridgeError("TIMEOUT"));
      }, CALL_TIMEOUT_MS);
      pending.set(id, { resolve: resolve, reject: reject, timer: timer });
      post({
        protocol: PROTOCOL,
        version: VERSION,
        type: "call",
        sessionId: sessionId,
        id: id,
        method: method,
        params: params
      });
    });
  }

  function call(method, params) {
    if (disposed) {
      return Promise.reject(bridgeError("DISPOSED"));
    }
    if (readySettled && sessionId) {
      return sendCall(method, params);
    }
    return ready().then(function () {
      if (disposed) {
        return Promise.reject(bridgeError("DISPOSED"));
      }
      if (!sessionId) {
        return Promise.reject(bridgeError("NOT_READY"));
      }
      return sendCall(method, params);
    });
  }

  function graphql(query, variables, options) {
    if (typeof query !== "string" || query.trim() === "") {
      return Promise.reject(bridgeError("INVALID_ARGUMENT"));
    }
    var vars = variables === undefined ? {} : variables;
    if (!isPlainObject(vars) || !isJSONValue(vars)) {
      return Promise.reject(bridgeError("INVALID_ARGUMENT"));
    }
    var params = { query: query, variables: vars };
    var operationName = "";
    if (options !== undefined && options !== null) {
      if (!isPlainObject(options)) {
        return Promise.reject(bridgeError("INVALID_ARGUMENT"));
      }
      if (options.operationName !== undefined) {
        if (typeof options.operationName !== "string" || options.operationName.trim() === "") {
          return Promise.reject(bridgeError("INVALID_ARGUMENT"));
        }
        operationName = options.operationName;
        params.operationName = operationName;
      }
    }
    if (declaredOperations(query) > 1 && operationName === "") {
      return Promise.reject(bridgeError("INVALID_ARGUMENT"));
    }
    return call("graphql", params);
  }

  function isSitePath(path) {
    if (typeof path !== "string" || path.length === 0 || path.charAt(0) !== "/") {
      return false;
    }
    if (path.indexOf("//") !== -1 || path.indexOf("\\") !== -1) {
      return false;
    }
    for (var i = 0; i < path.length; i++) {
      var code = path.charCodeAt(i);
      if (code <= 31 || code === 127) {
        return false;
      }
    }
    var origin = ensureOrigin();
    if (!origin) {
      return false;
    }
    var url;
    try {
      url = new URL(path, origin + "/");
    } catch (err) {
      return false;
    }
    return url.origin === origin && url.pathname.charAt(0) === "/";
  }

  function navigate(path) {
    if (!isSitePath(path)) {
      return Promise.reject(bridgeError("INVALID_ARGUMENT"));
    }
    return call("navigate", { path: path });
  }

  function toast(message, options) {
    if (typeof message !== "string" || message.trim() === "") {
      return Promise.reject(bridgeError("INVALID_ARGUMENT"));
    }
    var kind = "info";
    if (options !== undefined && options !== null) {
      if (!isPlainObject(options)) {
        return Promise.reject(bridgeError("INVALID_ARGUMENT"));
      }
      if (options.kind !== undefined) {
        if (options.kind !== "info" && options.kind !== "success" && options.kind !== "warning" && options.kind !== "error") {
          return Promise.reject(bridgeError("INVALID_ARGUMENT"));
        }
        kind = options.kind;
      }
    }
    return call("toast", { message: message, kind: kind });
  }

  function isFetchMethod(method) {
    return method === "GET" || method === "POST" || method === "PUT" || method === "PATCH" || method === "DELETE";
  }

  function fetch(url, init) {
    if (typeof url !== "string" || url === "") {
      return Promise.reject(bridgeError("INVALID_ARGUMENT"));
    }
    var params = { url: url };
    if (init !== undefined) {
      if (!isPlainObject(init)) {
        return Promise.reject(bridgeError("INVALID_ARGUMENT"));
      }
      if (init.method !== undefined) {
        if (!isFetchMethod(init.method)) {
          return Promise.reject(bridgeError("INVALID_ARGUMENT"));
        }
        params.method = init.method;
      }
      if (init.headers !== undefined) {
        if (!isPlainObject(init.headers)) {
          return Promise.reject(bridgeError("INVALID_ARGUMENT"));
        }
        var keys = Object.keys(init.headers);
        var headers = {};
        for (var i = 0; i < keys.length; i++) {
          if (typeof init.headers[keys[i]] !== "string") {
            return Promise.reject(bridgeError("INVALID_ARGUMENT"));
          }
          headers[keys[i]] = init.headers[keys[i]];
        }
        params.headers = headers;
      }
      if (init.body !== undefined) {
        if (typeof init.body !== "string") {
          return Promise.reject(bridgeError("INVALID_ARGUMENT"));
        }
        params.body = init.body;
      }
    }
    return call("fetch", params);
  }

  window.addEventListener("message", onMessage);
  window.addEventListener("error", onError);
  window.addEventListener("unhandledrejection", onRejection);
  window.addEventListener("pagehide", dispose);

  window.axonhub = {
    ready: ready,
    onInit: onInit,
    onTheme: onTheme,
    graphql: graphql,
    navigate: navigate,
    toast: toast,
    fetch: fetch
  };
  window.__axonhubDispose = dispose;
})();
