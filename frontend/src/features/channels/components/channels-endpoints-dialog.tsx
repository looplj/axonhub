'use client';

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { Google } from '@lobehub/icons';
import { AlertCircle, Check, ChevronDown, Loader2, Pencil, Plus, Radar, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { cn } from '@/lib/utils';
import { useDetectChannelEndpoints, useUpdateChannelSettings } from '../data/channels';
import {
  buildCodexResponsesEndpoints,
  getAvailableProtocolFormats,
  getModelProtocolValidationFormats,
  getCodexResponsesWebSocketState,
  getCodexResponsesWebSocketURL,
  hasCodexResponsesWebSocketOverrideReplacement,
  getConfigurableApiFormatsForChannelType,
  isResponsesFamilyFormat,
  toHttpBaseURL,
  translateLegacyCodexModelProtocols,
} from '../data/protocol-options';
import {
  Channel,
  ChannelEndpoint,
  DetectedChannelEndpoint,
  ModelProtocol,
  channelEndpointSchema,
  configurableChannelEndpointApiFormats,
  configurableChannelEndpointApiFormatSchema,
} from '../data/schema';
import {
  RELAY_PROTOCOL_LABEL_KEYS,
  getChannelRelayProtocols,
  isRelayProtocol,
  type RelayProtocol,
} from '../data/relay-protocols';
import {
  QWEN_REALTIME_API_FORMAT,
  QWEN_REALTIME_REGIONS,
  buildQwenRealtimeBaseURL,
  buildQwenRealtimeEndpoint,
  isQwenRealtimeEndpointAllowed,
  isValidQwenRealtimeRegion,
  isValidQwenRealtimeWorkspace,
  parseQwenRealtimeBaseURL,
  type QwenRealtimeRegion,
} from '../data/qwen-realtime';
import { getRealtimeDefaultEndpoint, hasLegacyRealtimeEndpoints, isRealtimeProtocol } from '../data/channel-realtime';
import { ChatProtocolIcon, MessagesProtocolIcon, ResponsesProtocolIcon } from './endpoint-protocol-icons';

const RELAY_PROTOCOL_ICONS: Record<RelayProtocol, React.ComponentType<{ size?: number | string; className?: string }>> = {
  'openai/chat_completions': ChatProtocolIcon,
  'openai/responses': ResponsesProtocolIcon,
  'anthropic/messages': MessagesProtocolIcon,
  'gemini/contents': Google,
};

const RELAY_PROTOCOL_ACTIVE_CLASSES: Record<RelayProtocol, string> = {
  'openai/chat_completions': 'border-sky-500/30 bg-sky-500/15 text-sky-600 dark:text-sky-400',
  'openai/responses': 'border-violet-500/30 bg-violet-500/15 text-violet-600 dark:text-violet-400',
  'anthropic/messages': 'border-amber-500/30 bg-amber-500/15 text-amber-600 dark:text-amber-400',
  'gemini/contents': 'border-emerald-500/30 bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
};

const DETECTION_REASON_KEYS: Record<string, string> = {
  supported: 'channels.endpoints.detect.reasons.supported',
  not_found: 'channels.endpoints.detect.reasons.notFound',
  auth_error: 'channels.endpoints.detect.reasons.authError',
  rate_limited: 'channels.endpoints.detect.reasons.rateLimited',
  server_error: 'channels.endpoints.detect.reasons.serverError',
  unreachable: 'channels.endpoints.detect.reasons.unreachable',
  invalid_request: 'channels.endpoints.detect.reasons.invalidRequest',
};

function DetectionResultCard({ result }: { result: DetectedChannelEndpoint }) {
  const { t } = useTranslation();

  if (!isRelayProtocol(result.apiFormat)) {
    return null;
  }

  const Icon = RELAY_PROTOCOL_ICONS[result.apiFormat];
  const label = t(RELAY_PROTOCOL_LABEL_KEYS[result.apiFormat]);
  const reasonKey = DETECTION_REASON_KEYS[result.reason] ?? DETECTION_REASON_KEYS.unreachable;

  return (
    <div
      className={cn(
        'flex min-w-0 items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs',
        result.supported ? RELAY_PROTOCOL_ACTIVE_CLASSES[result.apiFormat] : 'border-border/60 bg-muted/40 text-muted-foreground'
      )}
    >
      <Icon size={14} />
      <span className='truncate font-medium'>{label}</span>
      <span className='text-muted-foreground shrink-0'>
        {result.supported ? t('channels.endpoints.detect.supported') : t(reasonKey)}
        {!result.supported && result.statusCode ? ` (${result.statusCode})` : ''}
      </span>
    </div>
  );
}

interface Props {
  channel: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const ALLOWED_API_FORMATS = configurableChannelEndpointApiFormatSchema.options;
const OPENAI_RESPONSES = 'openai/responses';
const OPENAI_RESPONSES_WEBSOCKET = 'openai/responses-ws';
const OPENAI_REALTIME_WEBSOCKET = 'openai/realtime-ws';
// The official OpenAI Realtime upstream is fixed: the channel credentials and
// model selection carry the rest of the configuration.
const OPENAI_REALTIME_BASE_URL = 'wss://api.openai.com';
const OPENAI_REALTIME_PATH = '/v1/realtime';
// WebSocket Responses endpoints are valid persisted endpoints, but they are not
// offered as custom endpoints: Codex manages them with its WebSocket checkbox.
const RECOGNIZED_API_FORMATS: readonly string[] = [...ALLOWED_API_FORMATS, OPENAI_RESPONSES_WEBSOCKET];

// Codex saves canonical Responses endpoints, so legacy WebSocket-only restrictions
// are edited in their WebSocket identity (mirrors the backend legacy mapping).
function getInitialModelProtocols(channel: Pick<Channel, 'type' | 'baseURL' | 'endpoints' | 'settings'>): ModelProtocol[] {
  const protocols = channel.settings?.modelProtocols ?? [];
  return (channel.type === 'codex' && translateLegacyCodexModelProtocols(channel, protocols)) || protocols;
}

function ProtocolFormatMultiSelect({
  formats,
  selectedFormats,
  onToggle,
  placeholder,
  portalContainer,
}: {
  formats: string[];
  selectedFormats: string[];
  onToggle: (format: string) => void;
  placeholder: string;
  portalContainer?: HTMLElement | null;
}) {
  const [open, setOpen] = useState(false);
  const displayText = selectedFormats.length > 0 ? selectedFormats.join(', ') : placeholder;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type='button' variant='outline' className='h-10 w-full min-w-0 justify-between font-normal'>
          <span className={selectedFormats.length > 0 ? 'truncate font-mono text-xs' : 'text-muted-foreground truncate text-xs'}>
            {displayText}
          </span>
          <ChevronDown className='ml-2 h-4 w-4 shrink-0 opacity-50' />
        </Button>
      </PopoverTrigger>
      <PopoverContent container={portalContainer} className='w-[var(--radix-popover-trigger-width)] min-w-64 p-1' align='start'>
        {formats.length === 0 ? (
          <div className='text-muted-foreground px-2 py-3 text-center text-xs'>{placeholder}</div>
        ) : (
          <div className='max-h-56 overflow-y-auto'>
            {formats.map((format) => {
              const checked = selectedFormats.includes(format);
              return (
                <label
                  key={format}
                  className='hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-2 text-left text-xs'
                >
                  <Checkbox checked={checked} onCheckedChange={() => onToggle(format)} />
                  <span className='truncate font-mono'>{format}</span>
                </label>
              );
            })}
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}

function EndpointTable({
  endpoints,
  readOnly,
  hideBaseURL,
  children,
}: {
  endpoints: ChannelEndpoint[];
  readOnly?: boolean;
  hideBaseURL?: boolean;
  children?: (ep: ChannelEndpoint, index: number) => React.ReactNode;
}) {
  const { t } = useTranslation();
  const gridCols = hideBaseURL ? 'grid-cols-[1fr_1fr_auto]' : 'grid-cols-[1fr_1fr_1fr_auto]';
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className={`bg-muted/50 text-muted-foreground ${gridCols} gap-2 border-b px-3 py-2 text-xs font-medium`}>
        <span>{t('channels.endpoints.apiFormat')}</span>
        {!hideBaseURL && <span>{t('channels.endpoints.baseURL')}</span>}
        <span>{t('channels.endpoints.path')}</span>
        <span className='w-8' />
      </div>
      <div className='divide-y'>
        {endpoints.map((ep, index) => (
          <div
            key={`${ep.apiFormat}-${index}`}
            className={`hover:bg-muted/30 ${gridCols} items-center gap-2 px-3 py-2.5 text-sm transition-colors`}
          >
            <div className='flex items-center gap-2'>
              <Badge variant='secondary' className='w-fit font-mono text-xs'>
                {ep.apiFormat}
              </Badge>
              {readOnly && index === 0 && (
                <Badge variant='outline' className='text-[10px]'>
                  {t('channels.endpoints.primaryBadge')}
                </Badge>
              )}
              {!readOnly && !RECOGNIZED_API_FORMATS.includes(ep.apiFormat) && (
                <Badge variant='destructive' className='text-[10px]'>
                  {t('channels.endpoints.unsupportedBadge')}
                </Badge>
              )}
            </div>
            {!hideBaseURL && <span className='text-muted-foreground truncate font-mono text-xs'>{ep.baseURL || '-'}</span>}
            <span className='text-muted-foreground truncate font-mono text-xs'>{ep.path || '-'}</span>
            {readOnly ? (
              <span className='text-muted-foreground text-right text-[10px]'>{t('channels.endpoints.readOnly')}</span>
            ) : (
              children?.(ep, index)
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function ChannelsEndpointsDialog({ channel, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const updateChannelSettings = useUpdateChannelSettings();
  const detectEndpoints = useDetectChannelEndpoints();
  const [dialogContentElement, setDialogContentElement] = useState<HTMLDivElement | null>(null);
  const wasOpenRef = useRef(false);

  const isCodexChannel = channel.type === 'codex';
  const realtimeDefaultEndpoint = useMemo(() => getRealtimeDefaultEndpoint(channel.settings?.realtime), [channel.settings?.realtime]);
  // Codex Responses defaults are projected by the WebSocket checkbox, so only the
  // remaining built-in defaults are listed and validated as defaults.
  const defaultEndpoints = useMemo(() => {
    if (realtimeDefaultEndpoint) return [realtimeDefaultEndpoint];
    const defaults = channel.defaultEndpoints ?? [];
    return channel.type === 'codex' ? defaults.filter((ep) => !isResponsesFamilyFormat(ep.apiFormat)) : defaults;
  }, [channel.defaultEndpoints, channel.type, realtimeDefaultEndpoint]);

  const [endpoints, setEndpoints] = useState<ChannelEndpoint[]>(() => channel.endpoints ?? []);
  const legacyRealtimeEndpoints = hasLegacyRealtimeEndpoints(channel.endpoints ?? []);
  const [newApiFormat, setNewApiFormat] = useState('');
  const [newPath, setNewPath] = useState('');
  const [newBaseURL, setNewBaseURL] = useState('');
  const [newHandshakeTimeout, setNewHandshakeTimeout] = useState('');
  const [newQwenRegion, setNewQwenRegion] = useState<QwenRealtimeRegion>(QWEN_REALTIME_REGIONS[0]);
  const [newQwenWorkspace, setNewQwenWorkspace] = useState('');
  const [editingQwenIndex, setEditingQwenIndex] = useState<number | null>(null);
  const [modelProtocols, setModelProtocols] = useState<ModelProtocol[]>(() => getInitialModelProtocols(channel));
  const [newProtocolModel, setNewProtocolModel] = useState('');
  const [newProtocolFormats, setNewProtocolFormats] = useState<string[]>([]);
  const [editingProtocolModel, setEditingProtocolModel] = useState<string | null>(null);
  const [blockedEndpointRemoval, setBlockedEndpointRemoval] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [detectionResults, setDetectionResults] = useState<DetectedChannelEndpoint[] | null>(null);
  const initialCodexResponses = useMemo(() => getCodexResponsesWebSocketState(channel), [channel]);
  const [codexResponsesWebSocket, setCodexResponsesWebSocket] = useState(initialCodexResponses.enabled);
  const [preferResponsesWebSocket, setPreferResponsesWebSocket] = useState<boolean>();

  const supportedModelOptions = useMemo(
    () => Array.from(new Set([...channel.supportedModels, ...modelProtocols.map((mp) => mp.model)])).map((model) => ({ value: model, label: model })),
    [channel.supportedModels, modelProtocols]
  );

  const availableModelOptions = useMemo(() => {
    const configuredModels = new Set(modelProtocols.filter((mp) => mp.model !== editingProtocolModel).map((mp) => mp.model));
    return supportedModelOptions.filter((option) => !configuredModels.has(option.value));
  }, [editingProtocolModel, modelProtocols, supportedModelOptions]);

  useEffect(() => {
    const justOpened = open && !wasOpenRef.current;
    wasOpenRef.current = open;
    if (!justOpened) {
      return;
    }

    setEndpoints(channel.endpoints ?? []);
    setCodexResponsesWebSocket(initialCodexResponses.enabled);
    setPreferResponsesWebSocket(undefined);
    setNewApiFormat('');
    setNewPath('');
    setNewBaseURL('');
    setNewHandshakeTimeout('');
    setNewQwenRegion(QWEN_REALTIME_REGIONS[0]);
    setNewQwenWorkspace('');
    setEditingQwenIndex(null);
    setModelProtocols(getInitialModelProtocols(channel));
    setNewProtocolModel('');
    setNewProtocolFormats([]);
    setEditingProtocolModel(null);
    setBlockedEndpointRemoval(null);
    setError(null);
    setDetectionResults(null);
  }, [open, channel, initialCodexResponses.enabled]);

  const usedApiFormats = useMemo(() => new Set(endpoints.map((ep) => ep.apiFormat)), [endpoints]);

  const availableApiFormats = useMemo(
    () =>
      getConfigurableApiFormatsForChannelType(channel.type, configurableChannelEndpointApiFormats).filter(
        (f) => !usedApiFormats.has(f) || f === newApiFormat
      ),
    [usedApiFormats, channel.type, newApiFormat]
  );

  // Codex saves its Responses family from the checkbox through the same routine as
  // the channel form; every other channel saves the endpoint list as edited.
  const savedEndpoints = useMemo(
    () => (isCodexChannel ? buildCodexResponsesEndpoints(endpoints, codexResponsesWebSocket) : endpoints),
    [codexResponsesWebSocket, endpoints, isCodexChannel]
  );
  const codexWebSocketURL =
    isCodexChannel && codexResponsesWebSocket ? getCodexResponsesWebSocketURL(toHttpBaseURL(channel.baseURL), endpoints) : undefined;

  const isQwenRealtimeFormat = newApiFormat === QWEN_REALTIME_API_FORMAT;
  const isOpenAIRealtimeFormat = newApiFormat === OPENAI_REALTIME_WEBSOCKET;

  const qwenRealtimePreview = useMemo(() => {
    if (!isQwenRealtimeFormat) return undefined;
    if (!isValidQwenRealtimeWorkspace(newQwenWorkspace) || !isValidQwenRealtimeRegion(newQwenRegion)) return undefined;
    try {
      return buildQwenRealtimeBaseURL(newQwenWorkspace, newQwenRegion);
    } catch {
      return undefined;
    }
  }, [isQwenRealtimeFormat, newQwenRegion, newQwenWorkspace]);

  const handleNewApiFormatChange = useCallback((format: string) => {
    if (isRealtimeProtocol(format)) { setError(t('channels.realtime.legacyEndpoint')); return; }
    setNewApiFormat(format);
    setError(null);
    if (format === OPENAI_REALTIME_WEBSOCKET) {
      setNewBaseURL(OPENAI_REALTIME_BASE_URL);
      setNewPath(OPENAI_REALTIME_PATH);
    } else {
      setNewBaseURL('');
      setNewPath('');
    }
    if (format !== QWEN_REALTIME_API_FORMAT && format !== OPENAI_REALTIME_WEBSOCKET) {
      setEditingQwenIndex(null);
    }
  }, [t]);

  const handleAddEndpoint = useCallback(() => {
    setError(null);
    if (!newApiFormat) return;
    if (isRealtimeProtocol(newApiFormat)) { setError(t('channels.realtime.legacyEndpoint')); return; }

    if (endpoints.some((ep, index) => ep.apiFormat === newApiFormat && index !== editingQwenIndex)) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    const handshakeTimeoutMs = newHandshakeTimeout.trim() ? Number(newHandshakeTimeout) : undefined;
    if ((newApiFormat === QWEN_REALTIME_API_FORMAT || newApiFormat === OPENAI_REALTIME_WEBSOCKET) && handshakeTimeoutMs !== undefined && (!Number.isInteger(handshakeTimeoutMs) || handshakeTimeoutMs < 1000 || handshakeTimeoutMs > 60000)) {
      setError(t('channels.endpoints.realtime.invalidTimeout'));
      return;
    }
    if (newApiFormat === QWEN_REALTIME_API_FORMAT) {
      if (!isQwenRealtimeEndpointAllowed(channel.type)) {
        setError(t('channels.endpoints.qwenRealtime.notAllowed'));
        return;
      }
      const workspace = newQwenWorkspace.trim();
      if (!isValidQwenRealtimeWorkspace(workspace)) {
        setError(t('channels.endpoints.qwenRealtime.invalidWorkspace'));
        return;
      }
      const endpoint = { ...buildQwenRealtimeEndpoint(workspace, newQwenRegion), ...(handshakeTimeoutMs !== undefined ? { handshakeTimeoutMs } : {}) };
      if (editingQwenIndex !== null) {
        setEndpoints((prev) => prev.map((ep, index) => (index === editingQwenIndex ? endpoint : ep)));
      } else {
        setEndpoints((prev) => [...prev, endpoint]);
      }
      setNewApiFormat('');
      setNewHandshakeTimeout('');
      setNewQwenWorkspace('');
      setNewQwenRegion(QWEN_REALTIME_REGIONS[0]);
      setEditingQwenIndex(null);
      return;
    }

    if (newApiFormat === OPENAI_REALTIME_WEBSOCKET) {
      if (channel.type !== 'openai') {
        setError(t('channels.endpoints.openAIRealtime.notAllowed'));
        return;
      }
      const parsed = channelEndpointSchema.safeParse({
        apiFormat: newApiFormat,
        baseURL: OPENAI_REALTIME_BASE_URL,
        path: OPENAI_REALTIME_PATH,
        handshakeTimeoutMs,
        transport: 'websocket',
      });
      if (!parsed.success) {
        setError(t('channels.endpoints.openAIRealtime.invalid'));
        return;
      }
      setEndpoints((prev) => (editingQwenIndex !== null ? prev.map((ep, index) => (index === editingQwenIndex ? parsed.data : ep)) : [...prev, parsed.data]));
      setNewApiFormat('');
      setNewHandshakeTimeout('');
      setEditingQwenIndex(null);
      return;
    }

    if (!ALLOWED_API_FORMATS.includes(newApiFormat as (typeof ALLOWED_API_FORMATS)[number])) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    const parsed = channelEndpointSchema.safeParse({
      apiFormat: newApiFormat,
      path: newPath || undefined,
      baseURL: newBaseURL || undefined,
    });
    if (!parsed.success) {
      const firstIssue = parsed.error.issues[0];
      if (firstIssue?.path[0] === 'baseURL') {
        setError(t('channels.endpoints.invalidBaseURL'));
      } else {
        setError(firstIssue?.message || 'Invalid endpoint');
      }
      return;
    }

    if (editingQwenIndex !== null) {
      setEndpoints((prev) => prev.map((ep, index) => (index === editingQwenIndex ? parsed.data : ep)));
    } else {
      setEndpoints((prev) => [...prev, parsed.data]);
    }
    setNewApiFormat('');
    setNewPath('');
    setNewBaseURL('');
    setEditingQwenIndex(null);
  }, [newApiFormat, newPath, newBaseURL, newHandshakeTimeout, t, channel.type, newQwenRegion, newQwenWorkspace, editingQwenIndex, endpoints]);

  const handleEditQwenEndpoint = useCallback((index: number, apiFormat: string) => {
    if (isRealtimeProtocol(apiFormat)) { setError(t('channels.realtime.legacyEndpoint')); return; }
    setError(null);
    setEditingQwenIndex(index);
    setNewHandshakeTimeout(String(endpoints[index]?.handshakeTimeoutMs ?? ''));
    if (apiFormat === QWEN_REALTIME_API_FORMAT) {
      // Only the generated wss://{workspace}.{region}.maas.aliyuncs.com host is
      // echoed back; any other override is hidden until a valid one is set.
      setNewApiFormat(QWEN_REALTIME_API_FORMAT);
      const parsed = (endpoints[index] as ChannelEndpoint | undefined)?.baseURL
        ? parseQwenRealtimeBaseURL((endpoints[index] as ChannelEndpoint).baseURL)
        : null;
      setNewQwenWorkspace(parsed?.workspace ?? '');
      setNewQwenRegion(parsed?.region ?? QWEN_REALTIME_REGIONS[0]);
    } else if (apiFormat === OPENAI_REALTIME_WEBSOCKET) {
      setNewApiFormat(OPENAI_REALTIME_WEBSOCKET);
    }
  }, [endpoints, t]);

  const handleRemoveEndpoint = useCallback((apiFormat: string) => {
    setEndpoints((prev) => prev.filter((ep) => ep.apiFormat !== apiFormat));
    // Removing the endpoint being edited closes the editor instead of leaving a
    // dangling override target.
    setEditingQwenIndex((index) => (index === null ? null : null));
    setNewApiFormat((format) => (format === apiFormat ? '' : format));
    setNewQwenWorkspace('');
    setNewQwenRegion(QWEN_REALTIME_REGIONS[0]);
    setBlockedEndpointRemoval(null);
    setError(null);
  }, []);

  const handleRequestRemoveEndpoint = useCallback(
    (apiFormat: string) => {
      const remainsAvailable = defaultEndpoints.some((ep) => ep.apiFormat === apiFormat);
      if (!remainsAvailable && modelProtocols.some((mp) => mp.enabled !== false && mp.apiFormats.includes(apiFormat))) {
        setBlockedEndpointRemoval(apiFormat);
        return;
      }
      handleRemoveEndpoint(apiFormat);
    },
    [defaultEndpoints, handleRemoveEndpoint, modelProtocols]
  );

  // Disabling WebSocket follows the endpoint removal rule: enabled restrictions
  // that still use it must be edited first.
  const handleCodexWebSocketChange = useCallback(
    (checked: boolean) => {
      if (!checked && modelProtocols.some((mp) => mp.enabled !== false && mp.apiFormats.includes(OPENAI_RESPONSES_WEBSOCKET))) {
        setBlockedEndpointRemoval(OPENAI_RESPONSES_WEBSOCKET);
        return;
      }
      setCodexResponsesWebSocket(checked);
    },
    [modelProtocols]
  );

  const handleDetectEndpoints = useCallback(async () => {
    setError(null);

    try {
      const results = await detectEndpoints.mutateAsync({ channelID: channel.id });
      setDetectionResults(results);

      // Add the supported relay protocols that the channel does not expose yet.
      // Default endpoints are built-in and read-only, so only the override list
      // is updated. The user still has to save to persist the change.
      const available = getChannelRelayProtocols(defaultEndpoints, savedEndpoints);
      const additions = results
        .filter((result) => result.supported && isRelayProtocol(result.apiFormat) && !available.has(result.apiFormat))
        .map((result) => channelEndpointSchema.parse({ apiFormat: result.apiFormat }));

      if (additions.length > 0) {
        setEndpoints((prev) => {
          const existing = new Set(prev.map((ep) => ep.apiFormat));
          return [...prev, ...additions.filter((ep) => !existing.has(ep.apiFormat))];
        });
      }
    } catch {
      // error handled by the hook
    }
  }, [channel.id, defaultEndpoints, savedEndpoints, detectEndpoints]);

  const availableProtocolFormats = useMemo(() => {
    return getAvailableProtocolFormats(defaultEndpoints, savedEndpoints);
  }, [defaultEndpoints, savedEndpoints]);
  const canPreferResponsesWebSocket =
    availableProtocolFormats.includes(OPENAI_RESPONSES) && availableProtocolFormats.includes(OPENAI_RESPONSES_WEBSOCKET);

  const affectedProtocolModels = useMemo(
    () =>
      blockedEndpointRemoval && !defaultEndpoints.some((ep) => ep.apiFormat === blockedEndpointRemoval)
        ? modelProtocols.filter((mp) => mp.apiFormats.includes(blockedEndpointRemoval)).map((mp) => mp.model)
        : [],
    [blockedEndpointRemoval, defaultEndpoints, modelProtocols]
  );

  useEffect(() => {
    setNewProtocolFormats((prev) => prev.filter((format) => availableProtocolFormats.includes(format)));
  }, [availableProtocolFormats]);

  const handleToggleProtocolFormat = useCallback((format: string) => {
    setNewProtocolFormats((prev) => (prev.includes(format) ? prev.filter((f) => f !== format) : [...prev, format]));
  }, []);

  const handleSaveModelProtocol = useCallback(() => {
    setError(null);

    const model = newProtocolModel.trim();
    if (!model) {
      setError(t('channels.endpoints.modelProtocols.modelRequired'));
      return;
    }

    if (modelProtocols.some((mp) => mp.model === model && mp.model !== editingProtocolModel)) {
      setError(t('channels.endpoints.modelProtocols.duplicateModel'));
      return;
    }

    if (newProtocolFormats.length === 0) {
      setError(t('channels.endpoints.modelProtocols.formatsRequired'));
      return;
    }

    if (editingProtocolModel) {
      setModelProtocols((prev) =>
        prev.map((mp) => (mp.model === editingProtocolModel ? { ...mp, model, apiFormats: [...newProtocolFormats] } : mp))
      );
    } else {
      setModelProtocols((prev) => [...prev, { model, apiFormats: [...newProtocolFormats], enabled: true }]);
    }
    setNewProtocolModel('');
    setNewProtocolFormats([]);
    setEditingProtocolModel(null);
  }, [editingProtocolModel, newProtocolModel, newProtocolFormats, modelProtocols, t]);

  const handleStartEditModelProtocol = useCallback((protocol: ModelProtocol) => {
    setNewProtocolModel(protocol.model);
    setNewProtocolFormats([...protocol.apiFormats]);
    setEditingProtocolModel(protocol.model);
    setError(null);
  }, []);

  const handleCancelEditModelProtocol = useCallback(() => {
    setNewProtocolModel('');
    setNewProtocolFormats([]);
    setEditingProtocolModel(null);
    setError(null);
  }, []);

  const handleRemoveModelProtocol = useCallback((model: string) => {
    setModelProtocols((prev) => prev.filter((mp) => mp.model !== model));
    if (editingProtocolModel === model) {
      setEditingProtocolModel(null);
      setNewProtocolModel('');
      setNewProtocolFormats([]);
    }
    setError(null);
  }, [editingProtocolModel]);

  const handleToggleModelProtocol = useCallback((model: string, enabled: boolean) => {
    setModelProtocols((prev) => prev.map((mp) => (mp.model === model ? { ...mp, enabled } : mp)));
    setError(null);
  }, []);

  const handleSave = useCallback(async () => {
    setError(null);
    if (legacyRealtimeEndpoints || hasLegacyRealtimeEndpoints(savedEndpoints)) {
      setError(t('channels.realtime.legacyEndpoint'));
      return;
    }

    const apiFormats = savedEndpoints.map((ep) => ep.apiFormat);
    const duplicates = apiFormats.filter((f, i) => apiFormats.indexOf(f) !== i);
    if (duplicates.length > 0) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    const invalidApiFormat = apiFormats.find((format) => !RECOGNIZED_API_FORMATS.includes(format));
    if (invalidApiFormat) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    // Upstream realtime endpoints are channel-type specific: the Qwen endpoint
    // only exists on Bailian, and the official OpenAI endpoint only on OpenAI.
    const qwenEndpoint = savedEndpoints.find((ep) => ep.apiFormat === QWEN_REALTIME_API_FORMAT);
    if (qwenEndpoint && !isQwenRealtimeEndpointAllowed(channel.type)) {
      setError(t('channels.endpoints.qwenRealtime.notAllowed'));
      return;
    }
    if (qwenEndpoint && !parseQwenRealtimeBaseURL(qwenEndpoint.baseURL)) {
      setError(t('channels.endpoints.qwenRealtime.invalidWorkspace'));
      return;
    }
    if (savedEndpoints.some((ep) => ep.apiFormat === OPENAI_REALTIME_WEBSOCKET) && channel.type !== 'openai') {
      setError(t('channels.endpoints.openAIRealtime.notAllowed'));
      return;
    }

    // Every model protocol must reference an endpoint that exists after this save.
    // The backend validates against resolved endpoints (type defaults + custom), so
    // the check here must include the default endpoints too.
    const modelProtocolValidationDefaults = realtimeDefaultEndpoint && !isCodexChannel ? channel.defaultEndpoints ?? [] : defaultEndpoints;
    const savedFormats = getModelProtocolValidationFormats(modelProtocolValidationDefaults, savedEndpoints, channel.settings?.realtime);
    const invalidProtocolFormat = modelProtocols.find((mp) => mp.enabled !== false && mp.apiFormats.some((f) => !savedFormats.has(f)));
    if (invalidProtocolFormat) {
      setError(t('channels.endpoints.modelProtocols.formatsRequired'));
      return;
    }

    // Canonical Codex Responses endpoints inherit the channel Base URL, so a legacy
    // WS(S) Base URL is saved as its HTTP(S) equivalent.
    const baseURL = isCodexChannel ? toHttpBaseURL(channel.baseURL) : channel.baseURL;

    try {
      // Endpoints and model protocol overrides must be committed together in one
      // update: saving them separately validates each half against the other's
      // stale state (e.g. deleting an endpoint together with the override that
      // references it would be rejected) and a later failure would leave the
      // earlier change half-applied.
      await updateChannelSettings.mutateAsync({
        id: channel.id,
        input: {
          ...(baseURL !== channel.baseURL ? { baseURL } : {}),
          endpoints: savedEndpoints.map((ep) => ({
            apiFormat: ep.apiFormat,
            path: ep.path || undefined,
            baseURL: ep.baseURL || undefined,
            transport: ep.transport || undefined,
            handshakeTimeoutMs: ep.handshakeTimeoutMs ?? undefined,
          })),
        },
        patch: { modelProtocols, preferResponsesWebSocket },
      });

      toast.success(t('channels.messages.updateSuccess'));
      onOpenChange(false);
    } catch {
      // error handled by hook
    }
  }, [
    channel,
    defaultEndpoints,
    isCodexChannel,
    legacyRealtimeEndpoints,
    savedEndpoints,
    modelProtocols,
    preferResponsesWebSocket,
    onOpenChange,
    updateChannelSettings,
    t,
  ]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && newApiFormat) {
        e.preventDefault();
        handleAddEndpoint();
      }
    },
    [newApiFormat, handleAddEndpoint]
  );

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent ref={setDialogContentElement} className='flex h-[90vh] max-h-[700px] w-full max-w-full flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader className='shrink-0'>
          <DialogTitle>{t('channels.endpoints.title')}</DialogTitle>
          <DialogDescription>{channel.name}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-6 overflow-y-auto py-4'>
          {realtimeDefaultEndpoint && <p className='text-muted-foreground rounded-md bg-muted/50 px-3 py-2 text-xs'>{t('channels.realtime.endpointsInactive')}</p>}
          {legacyRealtimeEndpoints && <div role='alert' className='text-destructive bg-destructive/10 rounded-md p-3 text-sm'>{t('channels.realtime.legacyEndpoint')}</div>}
          {isCodexChannel && (
            <div className='space-y-3'>
              <label className='text-sm font-medium'>{t('channels.dialogs.fields.apiFormat.label')}</label>
              <div className='space-y-1 rounded-lg border px-3 py-2.5'>
                <div className='flex min-h-8 items-center justify-between gap-4'>
                  <span className='text-sm'>{t('channels.dialogs.fields.apiFormat.formats.openai/responses')}</span>
                  <label className='flex cursor-pointer items-center gap-2 text-sm'>
                    <Checkbox
                      checked={codexResponsesWebSocket}
                      onCheckedChange={(checked) => handleCodexWebSocketChange(checked === true)}
                      data-testid='codex-endpoints-websocket-checkbox'
                    />
                    <span>{t('channels.dialogs.fields.apiFormat.codexWebSocket.label')}</span>
                  </label>
                </div>
                {codexWebSocketURL && (
                  <p className='text-muted-foreground text-xs break-all' data-testid='codex-endpoints-websocket-url'>
                    {t('channels.dialogs.fields.apiFormat.codexWebSocket.derivedURL', { url: codexWebSocketURL })}
                  </p>
                )}
                {codexResponsesWebSocket && (initialCodexResponses.webSocketOnly || hasCodexResponsesWebSocketOverrideReplacement(channel)) && (
                  <p className='text-xs text-amber-600 dark:text-amber-400' data-testid='codex-endpoints-websocket-upgrade-note'>
                    {t(initialCodexResponses.webSocketOnly
                      ? 'channels.dialogs.fields.apiFormat.codexWebSocket.legacyUpgrade'
                      : 'channels.dialogs.fields.apiFormat.codexWebSocket.overrideReplacement')}
                  </p>
                )}
              </div>
            </div>
          )}

          {canPreferResponsesWebSocket && (
            <label className='flex cursor-pointer items-center justify-between gap-4 rounded-lg border px-3 py-2.5'>
              <span className='space-y-1'>
                <span className='block text-sm font-medium'>{t('channels.endpoints.preferResponsesWebSocket.label')}</span>
                <span className='text-muted-foreground block text-xs'>{t('channels.endpoints.preferResponsesWebSocket.description')}</span>
              </span>
              <Switch
                checked={preferResponsesWebSocket ?? channel.settings?.preferResponsesWebSocket ?? false}
                onCheckedChange={setPreferResponsesWebSocket}
              />
            </label>
          )}

          {/* Default endpoints */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between'>
              <label className='text-sm font-medium'>{t('channels.endpoints.defaultEndpoints', 'Default endpoints')}</label>
              {defaultEndpoints.length > 0 && (
                <span className='text-muted-foreground text-xs'>
                  {t('channels.endpoints.resolvedCount', { count: defaultEndpoints.length })}
                </span>
              )}
            </div>
            {defaultEndpoints.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
                {t('channels.endpoints.noDefaultEndpoints', 'No default endpoints resolved for this channel type.')}
              </div>
            ) : (
              <EndpointTable endpoints={defaultEndpoints} readOnly hideBaseURL />
            )}
          </div>

          {/* Current configured endpoints */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between gap-2'>
              <label className='text-sm font-medium'>{t('channels.endpoints.currentEndpoints')}</label>
              <div className='flex items-center gap-2'>
                {savedEndpoints.length > 0 && (
                  <span className='text-muted-foreground text-xs'>
                    {t('channels.endpoints.configuredCount', { count: savedEndpoints.length })}
                  </span>
                )}
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='h-8'
                  onClick={handleDetectEndpoints}
                  disabled={detectEndpoints.isPending}
                >
                  {detectEndpoints.isPending ? (
                    <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                  ) : (
                    <Radar className='mr-1.5 h-3.5 w-3.5' />
                  )}
                  {t(detectEndpoints.isPending ? 'channels.endpoints.detect.detecting' : 'channels.endpoints.detect.button')}
                </Button>
              </div>
            </div>
            <p className='text-muted-foreground text-xs'>{t('channels.endpoints.detect.description')}</p>
            {savedEndpoints.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
                {t('channels.endpoints.noOverridesHint', 'No custom endpoint overrides configured.')}
              </div>
            ) : (
              <EndpointTable endpoints={savedEndpoints}>
                {(ep, index) =>
                  isCodexChannel && isResponsesFamilyFormat(ep.apiFormat) ? (
                    <span className='text-muted-foreground text-right text-[10px]'>{t('channels.endpoints.readOnly')}</span>
                  ) : (
                    <div className='flex items-center justify-end gap-1'>
                      {(ep.apiFormat === QWEN_REALTIME_API_FORMAT || ep.apiFormat === OPENAI_REALTIME_WEBSOCKET) && (
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          className='hover:bg-accent h-7 w-7 p-0'
                          onClick={() => handleEditQwenEndpoint(index, ep.apiFormat)}
                          aria-label={t('channels.endpoints.editEndpoint', { apiFormat: ep.apiFormat })}
                        >
                          <Pencil className='h-3.5 w-3.5' />
                        </Button>
                      )}
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0'
                        onClick={() => handleRequestRemoveEndpoint(ep.apiFormat)}
                        aria-label={t('channels.endpoints.removeEndpoint', { apiFormat: ep.apiFormat })}
                      >
                        <X className='h-3.5 w-3.5' />
                      </Button>
                    </div>
                  )
                }
              </EndpointTable>
            )}
            {detectionResults && (
              <div className='bg-muted/20 space-y-2 rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('channels.endpoints.detect.resultTitle')}</p>
                <div className='flex flex-wrap gap-2'>
                  {detectionResults.map((result) => (
                    <DetectionResultCard key={result.apiFormat} result={result} />
                  ))}
                </div>
              </div>
            )}
          </div>

          {(isQwenRealtimeFormat || isOpenAIRealtimeFormat) && (
            <div className='space-y-1'>
              <label htmlFor='endpoint-handshake-timeout' className='text-sm font-medium'>{t('channels.endpoints.realtime.handshakeTimeout')}</label>
              <Input id='endpoint-handshake-timeout' type='number' min={1000} max={60000} value={newHandshakeTimeout} onChange={(event) => setNewHandshakeTimeout(event.target.value)} placeholder={t('channels.endpoints.realtime.handshakeTimeoutHint')} />
            </div>
          )}
          {/* Add new endpoint */}
          <div className='space-y-3'>
            <label className='text-sm font-medium'>{t('channels.endpoints.addEndpoint')}</label>
            <div className='grid items-start gap-3 md:grid-cols-[minmax(10rem,1fr)_minmax(14rem,1.2fr)_minmax(14rem,1.2fr)_auto]'>
              <div className='min-w-0 space-y-1'>
                <label htmlFor='endpoint-api-format' className='text-muted-foreground block text-xs'>
                  {t('channels.endpoints.apiFormat')}
                </label>
                <Select value={newApiFormat} onValueChange={handleNewApiFormatChange}>
                  <SelectTrigger id='endpoint-api-format' className='w-full'>
                    <SelectValue placeholder={t('channels.endpoints.apiFormat')} />
                  </SelectTrigger>
                  <SelectContent>
                    {availableApiFormats.length === 0 ? (
                      <div className='text-muted-foreground px-2 py-4 text-center text-sm'>{t('channels.endpoints.allFormatsUsed')}</div>
                    ) : (
                      availableApiFormats.map((format) => (
                        <SelectItem key={format} value={format}>
                          {format}
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </div>
              {isQwenRealtimeFormat ? (
                <>
                  <div className='min-w-0 space-y-1'>
                    <label htmlFor='endpoint-qwen-region' className='text-muted-foreground block text-xs'>
                      {t('channels.endpoints.qwenRealtime.region')}
                    </label>
                    <Select
                      value={newQwenRegion}
                      onValueChange={(value) => {
                        if (isValidQwenRealtimeRegion(value)) setNewQwenRegion(value);
                      }}
                    >
                      <SelectTrigger id='endpoint-qwen-region' className='w-full'>
                        <SelectValue placeholder={t('channels.endpoints.qwenRealtime.region')} />
                      </SelectTrigger>
                      <SelectContent>
                        {QWEN_REALTIME_REGIONS.map((region) => (
                          <SelectItem key={region} value={region}>
                            {region}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className='min-w-0 space-y-1'>
                    <label htmlFor='endpoint-qwen-workspace' className='text-muted-foreground block text-xs'>
                      {t('channels.endpoints.qwenRealtime.workspace')}
                    </label>
                    <Input
                      id='endpoint-qwen-workspace'
                      placeholder={t('channels.endpoints.qwenRealtime.workspacePlaceholder')}
                      value={newQwenWorkspace}
                      onChange={(e) => setNewQwenWorkspace(e.target.value)}
                      onKeyDown={handleKeyDown}
                      aria-describedby='endpoint-qwen-workspace-hint'
                    />
                    <p id='endpoint-qwen-workspace-hint' className='text-muted-foreground text-[11px] leading-4 break-words'>
                      {qwenRealtimePreview
                        ? t('channels.endpoints.qwenRealtime.derivedURL', { url: qwenRealtimePreview })
                        : t('channels.endpoints.qwenRealtime.workspaceHint')}
                    </p>
                  </div>
                </>
              ) : (
                <>
                  <div className='min-w-0 space-y-1'>
                    <label htmlFor='endpoint-base-url' className='text-muted-foreground block text-xs'>
                      {t('channels.endpoints.baseURL')}
                    </label>
                    <Input
                      id='endpoint-base-url'
                      placeholder={newApiFormat ? t('channels.endpoints.baseURLPlaceholder') : t('channels.endpoints.selectFormatFirst')}
                      value={newBaseURL}
                      onChange={(e) => setNewBaseURL(e.target.value)}
                      onKeyDown={handleKeyDown}
                      disabled={!newApiFormat || isOpenAIRealtimeFormat}
                      readOnly={isOpenAIRealtimeFormat}
                      aria-describedby='endpoint-base-url-hint'
                      className='disabled:opacity-70'
                    />
                    <p id='endpoint-base-url-hint' className='text-muted-foreground text-[11px] leading-4 break-words'>
                      {isOpenAIRealtimeFormat ? t('channels.endpoints.openAIRealtime.fixedHint') : t('channels.endpoints.baseURLHint')}
                    </p>
                  </div>
                  <div className='min-w-0 space-y-1'>
                    <label htmlFor='endpoint-path' className='text-muted-foreground block text-xs'>
                      {t('channels.endpoints.path')}
                    </label>
                    <Input
                      id='endpoint-path'
                      placeholder={newApiFormat ? t('channels.endpoints.pathPlaceholder') : t('channels.endpoints.selectFormatFirst')}
                      value={newPath}
                      onChange={(e) => setNewPath(e.target.value)}
                      onKeyDown={handleKeyDown}
                      disabled={!newApiFormat || isOpenAIRealtimeFormat}
                      readOnly={isOpenAIRealtimeFormat}
                      aria-describedby='endpoint-path-hint'
                      className='disabled:opacity-70'
                    />
                    <p id='endpoint-path-hint' className='text-muted-foreground text-[11px] leading-4 break-words'>
                      {isOpenAIRealtimeFormat ? t('channels.endpoints.openAIRealtime.pathHint') : t('channels.endpoints.pathHint')}
                    </p>
                  </div>
                </>
              )}
              <Button
                type='button'
                variant='default'
                size='icon'
                onClick={handleAddEndpoint}
                disabled={!newApiFormat}
                className='shrink-0 justify-self-end md:mt-5 md:justify-self-auto'
                aria-label={t('channels.endpoints.addEndpoint')}
              >
                <Plus className='h-4 w-4' />
              </Button>
            </div>
          </div>

          {/* Model protocol overrides */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between'>
              <label className='text-sm font-medium'>{t('channels.endpoints.modelProtocols.title')}</label>
              {modelProtocols.length > 0 && (
                <span className='text-muted-foreground text-xs'>
                  {t('channels.endpoints.modelProtocols.count', { count: modelProtocols.length })}
                </span>
              )}
            </div>
            <p className='text-muted-foreground text-xs'>{t('channels.endpoints.modelProtocols.description')}</p>

            <div className='grid items-start gap-3 md:grid-cols-[minmax(10rem,1fr)_minmax(14rem,1.2fr)_minmax(14rem,1.2fr)_auto]'>
              <div className='min-w-0'>
                <AutoCompleteSelect
                  selectedValue={newProtocolModel}
                  onSelectedValueChange={setNewProtocolModel}
                  items={availableModelOptions}
                  placeholder={t('channels.endpoints.modelProtocols.modelPlaceholder')}
                  emptyMessage={t(
                    availableModelOptions.length === 0 && supportedModelOptions.length > 0
                      ? 'channels.endpoints.modelProtocols.allModelsConfigured'
                      : 'channels.endpoints.modelProtocols.noModelsAvailable'
                  )}
                  portalContainer={dialogContentElement}
                />
              </div>
              <div className='min-w-0 md:col-span-2'>
                <ProtocolFormatMultiSelect
                  formats={availableProtocolFormats}
                  selectedFormats={newProtocolFormats}
                  onToggle={handleToggleProtocolFormat}
                  placeholder={t('channels.endpoints.modelProtocols.formatsPlaceholder')}
                  portalContainer={dialogContentElement}
                />
              </div>
              <div className='flex items-center gap-2 justify-self-end md:col-start-4 md:justify-self-auto'>
                <Button
                  type='button'
                  variant='default'
                  size='icon'
                  onClick={handleSaveModelProtocol}
                  disabled={!newProtocolModel || newProtocolFormats.length === 0 || availableModelOptions.length === 0}
                  className='shrink-0'
                  aria-label={t(editingProtocolModel ? 'channels.endpoints.modelProtocols.save' : 'channels.endpoints.modelProtocols.add')}
                >
                  {editingProtocolModel ? <Check className='h-4 w-4' /> : <Plus className='h-4 w-4' />}
                </Button>
                {editingProtocolModel && (
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    onClick={handleCancelEditModelProtocol}
                    className='shrink-0'
                    aria-label={t('channels.endpoints.modelProtocols.cancelEdit')}
                  >
                    <X className='h-4 w-4' />
                  </Button>
                )}
              </div>
            </div>

            {modelProtocols.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border border-dashed p-3 text-center text-xs'>
                {t('channels.endpoints.modelProtocols.empty')}
              </div>
            ) : (
              <div className='divide-y rounded-lg border'>
                {modelProtocols.map((mp) => (
                  <div
                    key={mp.model}
                    className={`flex items-center justify-between gap-2 px-3 py-2 text-sm ${mp.enabled !== false ? '' : 'bg-muted/30 opacity-70'}`}
                  >
                    <div className='flex min-w-0 items-center gap-2'>
                      <Switch
                        disabled={mp.apiFormats.some(isRealtimeProtocol)}
                        checked={mp.enabled !== false}
                        onCheckedChange={(checked) => handleToggleModelProtocol(mp.model, checked)}
                        aria-label={t('channels.endpoints.modelProtocols.toggle', { model: mp.model })}
                      />
                      <span className='truncate font-mono text-xs'>{mp.model}</span>
                    </div>
                    <div className='flex flex-1 flex-wrap items-center justify-end gap-1.5'>
                      {mp.apiFormats.map((format) => (
                        <Badge key={format} variant='secondary' className='font-mono text-xs'>
                          {format}
                        </Badge>
                      ))}
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='hover:bg-accent h-7 w-7 p-0'
                        onClick={() => handleStartEditModelProtocol(mp)}
                        disabled={mp.apiFormats.some(isRealtimeProtocol)}
                        aria-label={t('channels.endpoints.modelProtocols.edit', { model: mp.model })}
                      >
                        <Pencil className='h-3.5 w-3.5' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0'
                        onClick={() => handleRemoveModelProtocol(mp.model)}
                        disabled={mp.apiFormats.some(isRealtimeProtocol)}
                        aria-label={t('channels.endpoints.modelProtocols.remove', { model: mp.model })}
                      >
                        <X className='h-3.5 w-3.5' />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {error && (
            <div className='text-destructive bg-destructive/10 flex items-center gap-2 rounded-md px-3 py-2 text-sm'>
              <AlertCircle className='h-4 w-4 shrink-0' />
              <span>{error}</span>
            </div>
          )}
        </div>

        <DialogFooter pinned divided>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={updateChannelSettings.isPending || legacyRealtimeEndpoints}>
            {updateChannelSettings.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
      </Dialog>
      <AlertDialog
        open={blockedEndpointRemoval !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) setBlockedEndpointRemoval(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader className='text-left'>
            <AlertDialogTitle>{t('channels.endpoints.removeEndpointBlockedTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('channels.endpoints.removeEndpointBlockedDescription', {
                apiFormat: blockedEndpointRemoval ?? '',
                count: affectedProtocolModels.length,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {affectedProtocolModels.length > 0 && (
            <div className='flex flex-wrap gap-1 rounded-md border px-3 py-2'>
              {affectedProtocolModels.map((model) => (
                <Badge key={model} variant='secondary' className='font-mono text-xs'>
                  {model}
                </Badge>
              ))}
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.buttons.close')}</AlertDialogCancel>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
