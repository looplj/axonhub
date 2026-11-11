import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { ensureFreshAccessToken } from '@/lib/auth-session';
import { useTheme } from '@/context/theme-context';
import { Button } from '@/components/ui/button';
import { createPluginUISession, pluginFrameSrc, type PluginUISession } from '../data/plugin-ui';
import {
  PLUGIN_UI_PROTOCOL,
  PLUGIN_UI_VERSION,
  bridgeError,
  isPluginSitePath,
  matchPluginBridgeMessage,
  pluginFetchCallParams,
  pluginFetchHTTPError,
  pluginFetchResult,
  pluginHTTPBridgeError,
  pluginProjectId,
  readPluginUITokens,
  resolvePluginTheme,
  type PluginBridgeMatch,
} from '../plugin-ui';

const HELLO_TIMEOUT_MS = 10_000;
const CALL_TIMEOUT_MS = 15_000;

const TOASTS = {
  info: toast.info,
  success: toast.success,
  warning: toast.warning,
  error: toast.error,
} as const;

type PluginFrameProps = {
  pluginId: string;
  entry: string;
  title?: string;
  onNavigate?: () => void;
} & ({ pageKey: string; actionKey?: never } | { pageKey?: never; actionKey: string });

interface FrameSession {
  id: string;
  ready: boolean;
  disposed: boolean;
  abort: AbortController;
  helloTimer: number;
}

export function PluginFrame(props: PluginFrameProps) {
  const [attempt, setAttempt] = useState(0);
  return (
    <PluginFrameAttempt
      key={`${props.pluginId}:${props.actionKey ? 'action' : 'page'}:${props.actionKey ?? props.pageKey}:${props.entry}:${attempt}`}
      {...props}
      onRetry={() => setAttempt((value) => value + 1)}
    />
  );
}

function PluginFrameAttempt({
  pluginId,
  pageKey,
  actionKey,
  entry,
  title,
  onNavigate,
  onRetry,
}: PluginFrameProps & { onRetry: () => void }) {
  const { t, i18n } = useTranslation();
  const { theme, colorScheme } = useTheme();
  const currentProjectId = useSelectedProjectId();
  const selectedProjectId = actionKey === undefined ? currentProjectId : null;
  const [instanceId] = useState(() => crypto.randomUUID());
  const router = useRouter();
  const [prefersDark, setPrefersDark] = useState(() => window.matchMedia('(prefers-color-scheme: dark)').matches);
  const [failed, setFailed] = useState(false);
  const sessionQuery = useQuery({
    queryKey: ['plugin-ui-session', pluginId, entry, instanceId],
    queryFn: ({ signal }) => createPluginUISession(pluginId, entry, signal),
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });

  const iframeRef = useRef<HTMLIFrameElement>(null);
  const sessionRef = useRef<FrameSession | null>(null);
  const pendingHelloRef = useRef(false);
  const themeRef = useRef({ theme: 'light' as 'light' | 'dark', colorScheme });
  const localeRef = useRef(i18n.language);
  const projectRef = useRef(selectedProjectId);
  const sessionDataRef = useRef<PluginUISession | null>(null);
  const routerRef = useRef(router);
  const pageKeyRef = useRef(pageKey);
  const lastThemeRef = useRef<{ theme: 'light' | 'dark'; colorScheme: string } | null>(null);

  const resolvedTheme = resolvePluginTheme(theme, prefersDark);
  themeRef.current = { theme: resolvedTheme, colorScheme };
  localeRef.current = i18n.language;
  projectRef.current = selectedProjectId;
  sessionDataRef.current = sessionQuery.data ?? null;
  routerRef.current = router;
  pageKeyRef.current = pageKey;

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => setPrefersDark(media.matches);
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, []);

  function themePayload() {
    return {
      theme: themeRef.current.theme,
      colorScheme: themeRef.current.colorScheme,
      tokens: readPluginUITokens(window.getComputedStyle(document.documentElement)),
    };
  }

  function initPayload() {
    const data = sessionDataRef.current;
    return {
      ...themePayload(),
      locale: localeRef.current,
      projectId: pluginProjectId(projectRef.current),
      page: pageKeyRef.current ?? null,
      action: actionKey ?? null,
      context: {},
      panel: null,
      slot: actionKey === undefined ? null : 'app.header.actions',
      config: data?.config ?? {},
      canWriteKV: Boolean(data?.canWriteKV),
    };
  }

  function postToFrame(message: object) {
    const frame = iframeRef.current?.contentWindow;
    if (!frame) {
      return;
    }
    frame.postMessage(message, '*');
  }

  function failPage() {
    const session = sessionRef.current;
    if (session && !session.disposed) {
      session.disposed = true;
      session.abort.abort();
      window.clearTimeout(session.helloTimer);
    }
    setFailed(true);
  }

  function acceptHello() {
    const session = sessionRef.current;
    if (!session || session.disposed || session.ready || !iframeRef.current?.contentWindow) {
      return;
    }
    session.ready = true;
    window.clearTimeout(session.helloTimer);
    const payload = initPayload();
    lastThemeRef.current = { theme: payload.theme, colorScheme: payload.colorScheme };
    postToFrame({
      protocol: PLUGIN_UI_PROTOCOL,
      version: PLUGIN_UI_VERSION,
      type: 'init',
      sessionId: session.id,
      payload,
    });
  }

  function beginSession() {
    const previous = sessionRef.current;
    if (previous && !previous.disposed) {
      previous.disposed = true;
      previous.abort.abort();
      window.clearTimeout(previous.helloTimer);
    }
    const id = crypto.randomUUID();
    const abort = new AbortController();
    const helloTimer = window.setTimeout(() => {
      const current = sessionRef.current;
      if (!current || current.id !== id || current.disposed || current.ready) {
        return;
      }
      failPage();
    }, HELLO_TIMEOUT_MS);
    sessionRef.current = { id, ready: false, disposed: false, abort, helloTimer };
    if (pendingHelloRef.current) {
      pendingHelloRef.current = false;
      acceptHello();
    }
  }

  async function handleCall(session: FrameSession, call: Extract<PluginBridgeMatch, { type: 'call' }>) {
    const reply = (body: { ok: true; result: unknown } | { ok: false; error: ReturnType<typeof bridgeError> }) => {
      const current = sessionRef.current;
      if (!current || current.disposed || current.id !== session.id) {
        return;
      }
      postToFrame({
        protocol: PLUGIN_UI_PROTOCOL,
        version: PLUGIN_UI_VERSION,
        type: 'reply',
        sessionId: session.id,
        id: call.id,
        ...body,
      });
    };
    if (!session.ready) {
      reply({ ok: false, error: bridgeError('NOT_READY') });
      return;
    }
    if (call.method !== 'graphql' && call.method !== 'navigate' && call.method !== 'toast' && call.method !== 'fetch') {
      reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
      return;
    }
    if (!call.params) {
      reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
      return;
    }
    if (call.method === 'navigate') {
      if (!isPluginSitePath(call.params.path, window.location.origin)) {
        reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
        return;
      }
      try {
        routerRef.current.history.push(call.params.path);
      } catch {
        reply({ ok: false, error: bridgeError('INTERNAL') });
        return;
      }
      reply({ ok: true, result: null });
      onNavigate?.();
      return;
    }
    if (call.method === 'toast') {
      const message = call.params.message;
      const kind = call.params.kind === undefined ? 'info' : call.params.kind;
      if (typeof message !== 'string' || message.trim() === '') {
        reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
        return;
      }
      if (kind !== 'info' && kind !== 'success' && kind !== 'warning' && kind !== 'error') {
        reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
        return;
      }
      TOASTS[kind](message);
      reply({ ok: true, result: null });
      return;
    }
    if (call.method === 'fetch') {
      const outbound = pluginFetchCallParams(call.params);
      if (!outbound) {
        reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
        return;
      }
      const requestPluginId = pluginId;
      const requestProjectId = selectedProjectId;
      const timeoutSignal = AbortSignal.timeout(CALL_TIMEOUT_MS);
      const signal = AbortSignal.any([session.abort.signal, timeoutSignal]);
      try {
        const token = await ensureFreshAccessToken();
        if (session.disposed || sessionRef.current?.id !== session.id || signal.aborted) {
          return;
        }
        const headers: Record<string, string> = { 'Content-Type': 'application/json' };
        if (token) {
          headers.Authorization = `Bearer ${token}`;
        }
        if (requestProjectId) {
          headers['X-Project-ID'] = requestProjectId;
        }
        const response = await fetch(`/admin/plugins/${encodeURIComponent(requestPluginId)}/fetch`, {
          method: 'POST',
          headers,
          body: JSON.stringify(outbound),
          signal,
        });
        if (session.disposed || sessionRef.current?.id !== session.id) {
          return;
        }
        if (!response.ok) {
          let payload: unknown = null;
          try {
            payload = await response.json();
          } catch {
            payload = null;
          }
          if (session.disposed || sessionRef.current?.id !== session.id) {
            return;
          }
          reply({ ok: false, error: pluginFetchHTTPError(response.status, payload) });
          return;
        }
        let payload: unknown;
        try {
          payload = await response.json();
        } catch {
          reply({ ok: false, error: bridgeError('INTERNAL', response.status) });
          return;
        }
        if (session.disposed || sessionRef.current?.id !== session.id) {
          return;
        }
        const result = pluginFetchResult(payload);
        if (!result) {
          reply({ ok: false, error: bridgeError('INTERNAL', response.status) });
          return;
        }
        reply({ ok: true, result });
      } catch {
        if (session.disposed || sessionRef.current?.id !== session.id) {
          return;
        }
        if (timeoutSignal.aborted) {
          reply({ ok: false, error: bridgeError('TIMEOUT') });
          return;
        }
        reply({ ok: false, error: bridgeError('NETWORK_ERROR') });
      }
      return;
    }

    const query = call.params.query;
    if (typeof query !== 'string' || query.trim() === '') {
      reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
      return;
    }
    const variables = call.params.variables === undefined ? {} : call.params.variables;
    if (!variables || typeof variables !== 'object' || Array.isArray(variables)) {
      reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
      return;
    }
    let operationName: string | undefined;
    if (call.params.operationName !== undefined) {
      if (typeof call.params.operationName !== 'string' || call.params.operationName.trim() === '') {
        reply({ ok: false, error: bridgeError('INVALID_ARGUMENT') });
        return;
      }
      operationName = call.params.operationName;
    }

    const requestPluginId = pluginId;
    const requestProjectId = selectedProjectId;
    const timeoutSignal = AbortSignal.timeout(CALL_TIMEOUT_MS);
    const signal = AbortSignal.any([session.abort.signal, timeoutSignal]);
    try {
      const token = await ensureFreshAccessToken();
      if (session.disposed || sessionRef.current?.id !== session.id || signal.aborted) {
        return;
      }
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) {
        headers.Authorization = `Bearer ${token}`;
      }
      if (requestProjectId) {
        headers['X-Project-ID'] = requestProjectId;
      }
      const response = await fetch(`/admin/plugins/${encodeURIComponent(requestPluginId)}/graphql`, {
        method: 'POST',
        headers,
        body: JSON.stringify({ query, variables, operationName }),
        signal,
      });
      if (session.disposed || sessionRef.current?.id !== session.id) {
        return;
      }
      if (!response.ok) {
        reply({ ok: false, error: pluginHTTPBridgeError(response.status) });
        return;
      }
      let body: unknown;
      try {
        body = await response.json();
      } catch {
        reply({ ok: false, error: bridgeError('INTERNAL', response.status) });
        return;
      }
      reply({ ok: true, result: body });
    } catch {
      if (session.disposed || sessionRef.current?.id !== session.id) {
        return;
      }
      if (timeoutSignal.aborted) {
        reply({ ok: false, error: bridgeError('TIMEOUT') });
        return;
      }
      reply({ ok: false, error: bridgeError('NETWORK_ERROR') });
    }
  }

  const handlersRef = useRef({ acceptHello, failPage, handleCall });
  handlersRef.current = { acceptHello, failPage, handleCall };

  useLayoutEffect(() => {
    function onMessage(event: MessageEvent) {
      const frame = iframeRef.current;
      if (!frame || event.source !== frame.contentWindow) {
        return;
      }
      const session = sessionRef.current;
      const matched = matchPluginBridgeMessage(event.data, session?.id ?? null);
      if (!matched) {
        return;
      }
      if (matched.type === 'hello') {
        if (session && !session.disposed && !session.ready) {
          handlersRef.current.acceptHello();
          return;
        }
        pendingHelloRef.current = true;
        return;
      }
      if (!session || session.disposed) {
        return;
      }
      if (matched.type === 'pageError') {
        handlersRef.current.failPage();
        return;
      }
      void handlersRef.current.handleCall(session, matched);
    }
    window.addEventListener('message', onMessage);
    return () => {
      window.removeEventListener('message', onMessage);
      const session = sessionRef.current;
      if (session && !session.disposed) {
        session.disposed = true;
        session.abort.abort();
        window.clearTimeout(session.helloTimer);
      }
    };
    // The listener reads the latest session and handlers through refs.
  }, []);

  useEffect(() => {
    // ThemeProvider writes the theme class in its own effect, which runs after
    // this child effect. Read the computed values in a microtask so the class
    // is already on <html>.
    let cancelled = false;
    queueMicrotask(() => {
      if (cancelled) {
        return;
      }
      const session = sessionRef.current;
      const frame = iframeRef.current?.contentWindow;
      if (!session?.ready || session.disposed || !frame) {
        return;
      }
      const next = themeRef.current;
      const previous = lastThemeRef.current;
      if (previous && previous.theme === next.theme && previous.colorScheme === next.colorScheme) {
        return;
      }
      const payload = themePayload();
      lastThemeRef.current = { theme: payload.theme, colorScheme: payload.colorScheme };
      frame.postMessage(
        {
          protocol: PLUGIN_UI_PROTOCOL,
          version: PLUGIN_UI_VERSION,
          type: 'theme',
          sessionId: session.id,
          payload,
        },
        '*'
      );
    });
    return () => {
      cancelled = true;
    };
  }, [resolvedTheme, colorScheme]);

  if (sessionQuery.isError || failed) {
    return (
      <div
        data-testid='plugin-frame-error'
        role='alert'
        className='flex h-full min-h-0 flex-1 flex-col items-center justify-center gap-4 p-6'
      >
        <p className='text-muted-foreground text-center text-lg'>
          {t(actionKey === undefined ? 'plugins.page.error' : 'plugins.actions.error')}
        </p>
        <Button data-testid='plugin-frame-retry' onClick={onRetry}>
          {t('plugins.page.retry')}
        </Button>
      </div>
    );
  }

  if (!sessionQuery.data) {
    return <div className='text-muted-foreground flex h-full min-h-0 flex-1 items-center justify-center p-6'>{t('plugins.loading')}</div>;
  }

  return (
    <iframe
      ref={iframeRef}
      data-testid='plugin-frame'
      title={title ?? pageKey ?? actionKey}
      src={pluginFrameSrc(sessionQuery.data.token, entry)}
      sandbox='allow-scripts allow-forms allow-downloads'
      allow='clipboard-write'
      referrerPolicy='no-referrer'
      className='h-full min-h-0 w-full flex-1 border-0'
      onLoad={beginSession}
    />
  );
}
