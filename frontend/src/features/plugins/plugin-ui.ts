export const PLUGIN_UI_PROTOCOL = 'axonhub.plugin-ui';
export const PLUGIN_UI_VERSION = 1;

const BRIDGE_MESSAGES = {
  INVALID_ARGUMENT: 'Invalid argument',
  NOT_READY: 'Not ready',
  UNAUTHENTICATED: 'Unauthenticated',
  FORBIDDEN: 'Forbidden',
  NOT_FOUND: 'Not found',
  RATE_LIMITED: 'Rate limited',
  TIMEOUT: 'Timed out',
  DISPOSED: 'Disposed',
  NETWORK_ERROR: 'Network error',
  INTERNAL: 'Internal error',
} as const;

export type PluginBridgeCode = keyof typeof BRIDGE_MESSAGES;

export interface PluginBridgeError {
  code: PluginBridgeCode;
  message: string;
  status?: number;
}

export type PluginBridgeMatch =
  | { type: 'hello' }
  | { type: 'call'; id: string; method: string; params: Record<string, unknown> | null }
  | { type: 'pageError'; message: string };

function isRecord(value: unknown): value is Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false;
  }
  const proto = Object.getPrototypeOf(value);
  return proto === Object.prototype || proto === null;
}

export function bridgeError(code: PluginBridgeCode, status?: number): PluginBridgeError {
  const error: PluginBridgeError = { code, message: BRIDGE_MESSAGES[code] };
  if (typeof status === 'number') {
    error.status = status;
  }
  return error;
}

// A string title is shown unchanged. An object title uses the current language,
// then en, then the page key. zh and zh-CN satisfy each other because the app
// language code is zh and manifests use zh-CN.
export function resolvePluginTitle(title: unknown, language: string, pageKey: string): string {
  if (typeof title === 'string') {
    return title.trim() === '' ? pageKey : title;
  }
  if (!isRecord(title)) {
    return pageKey;
  }
  const candidates: string[] = [];
  const push = (value: string) => {
    if (value && !candidates.includes(value)) {
      candidates.push(value);
    }
  };
  push(language);
  const normalized = language.toLowerCase();
  if (normalized === 'zh' || normalized.startsWith('zh-')) {
    push('zh-CN');
    push('zh');
  }
  push('en');
  for (const key of candidates) {
    const value = title[key];
    if (typeof value === 'string' && value.trim() !== '') {
      return value;
    }
  }
  return pageKey;
}

// Accepts one site path: a single leading slash, optional query and hash.
// Rejects a protocol-relative path, a backslash, control characters, and a URL
// that parses to a different origin.
export function isPluginSitePath(path: unknown, origin: string): path is string {
  if (typeof path !== 'string' || path.length === 0 || !path.startsWith('/')) {
    return false;
  }
  if (path.includes('//') || path.includes('\\')) {
    return false;
  }
  for (let i = 0; i < path.length; i += 1) {
    const code = path.charCodeAt(i);
    if (code <= 31 || code === 127) {
      return false;
    }
  }
  let base: string;
  try {
    base = new URL(origin).origin;
  } catch {
    return false;
  }
  if (!base || base === 'null') {
    return false;
  }
  let url: URL;
  try {
    url = new URL(path, `${base}/`);
  } catch {
    return false;
  }
  return url.origin === base && url.pathname.startsWith('/');
}

export function resolvePluginTheme(theme: string, prefersDark: boolean): 'light' | 'dark' {
  if (theme === 'dark') {
    return 'dark';
  }
  if (theme === 'light') {
    return 'light';
  }
  return prefersDark ? 'dark' : 'light';
}

// Keep this list identical to TOKEN_NAMES in internal/server/plugin/uisdk/bridge.js.
export const PLUGIN_UI_TOKEN_NAMES = [
  '--background',
  '--foreground',
  '--card',
  '--card-foreground',
  '--popover',
  '--popover-foreground',
  '--primary',
  '--primary-foreground',
  '--secondary',
  '--secondary-foreground',
  '--muted',
  '--muted-foreground',
  '--accent',
  '--accent-foreground',
  '--destructive',
  '--destructive-foreground',
  '--border',
  '--input',
  '--ring',
  '--radius',
  '--font-sans',
  '--font-mono',
  '--shadow-2xs',
  '--shadow-xs',
  '--shadow-sm',
  '--shadow',
  '--shadow-md',
  '--shadow-lg',
  '--shadow-xl',
  '--shadow-2xl',
] as const;

const PLUGIN_UI_TOKEN_VALUE_MAX = 200;

// Computed custom properties are colors, lengths, and font stacks.
// Reject values that could terminate a declaration if they were ever interpolated.
export function isPluginUITokenValue(value: string): boolean {
  if (value.length === 0 || value.length > PLUGIN_UI_TOKEN_VALUE_MAX) {
    return false;
  }
  for (let i = 0; i < value.length; i += 1) {
    const code = value.charCodeAt(i);
    if (code <= 31 || code === 127) {
      return false;
    }
  }
  return !/[;{}<>\\]/.test(value);
}

export function readPluginUITokens(style: { getPropertyValue(name: string): string }): Record<string, string> {
  const tokens: Record<string, string> = {};
  for (const name of PLUGIN_UI_TOKEN_NAMES) {
    const value = style.getPropertyValue(name).trim();
    if (isPluginUITokenValue(value)) {
      tokens[name] = value;
    }
  }
  return tokens;
}

export function pluginProjectId(selected: string | null | undefined): number | null {
  if (!selected) {
    return null;
  }
  const slash = selected.lastIndexOf('/');
  const raw = slash >= 0 ? selected.slice(slash + 1) : selected;
  if (!/^[0-9]+$/.test(raw)) {
    return null;
  }
  const id = Number(raw);
  if (!Number.isSafeInteger(id) || id <= 0) {
    return null;
  }
  return id;
}

export function pluginHTTPBridgeError(status: number): PluginBridgeError {
  if (status === 401) {
    return bridgeError('UNAUTHENTICATED', status);
  }
  if (status === 403) {
    return bridgeError('FORBIDDEN', status);
  }
  if (status === 404) {
    return bridgeError('NOT_FOUND', status);
  }
  if (status === 429) {
    return bridgeError('RATE_LIMITED', status);
  }
  return bridgeError('INTERNAL', status);
}

// Matches hello, call, and pageError for the current session.
// Hello has no session id. Call and pageError must carry the host session id.
// A call with a non-object params value is returned with params null.
export function matchPluginBridgeMessage(data: unknown, sessionId: string | null): PluginBridgeMatch | null {
  if (!isRecord(data)) {
    return null;
  }
  if (data.protocol !== PLUGIN_UI_PROTOCOL || data.version !== PLUGIN_UI_VERSION) {
    return null;
  }
  if (data.type === 'hello') {
    return { type: 'hello' };
  }
  if (!sessionId || data.sessionId !== sessionId) {
    return null;
  }
  if (data.type === 'pageError') {
    const payload = isRecord(data.payload) ? data.payload : {};
    const message = typeof payload.message === 'string' && payload.message ? payload.message : 'Error';
    return { type: 'pageError', message };
  }
  if (data.type !== 'call') {
    return null;
  }
  if (typeof data.id !== 'string' || data.id === '') {
    return null;
  }
  const method = typeof data.method === 'string' ? data.method : '';
  return {
    type: 'call',
    id: data.id,
    method,
    params: isRecord(data.params) ? data.params : null,
  };
}
