'use client';

import { useCallback, useEffect, useMemo, useRef, useState, memo } from 'react';
import { z } from 'zod';
import { useFieldArray, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, RefreshCw, Search, PanelLeft, Plus, Trash2, Eye, EyeOff, Play, Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useHorizontalScroll } from '@/hooks/use-horizontal-scroll';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { SelectDropdown } from '@/components/select-dropdown';
import {
  usePassThroughSettings,
  useProxyPresets,
  useQuotaRoutingSettings,
  useSaveProxyPreset,
  useUserAgentPassThroughSettings,
} from '@/features/system/data/system';
import { antigravityOAuthExchange, antigravityOAuthStart } from '../data/antigravity';
import {
  useCreateChannel,
  useDuplicateChannel,
  useUpdateChannelSettings,
  useFetchModels,
  useAllChannelNames,
  useAllChannelTags,
  useSyncChannelModels,
} from '../data/channels';
import { claudecodeOAuthExchange, claudecodeOAuthStart } from '../data/claudecode';
import { codexDecodeAuthJSON, codexOAuthExchange, codexOAuthStart } from '../data/codex';
import {
  getDefaultBaseURL,
  CHANNEL_CONFIGS,
  OPENAI_CHAT_COMPLETIONS,
  OPENAI_RESPONSES,
  ANTHROPIC_MESSAGES,
  GEMINI_CONTENTS,
} from '../data/config_channels';
import {
  PROVIDER_CONFIGS,
  getProviderFromChannelType,
  getApiFormatsForProvider,
  getChannelTypeForApiFormat,
} from '../data/config_providers';
import {
  buildCodexResponsesEndpoints,
  getCodexResponsesWebSocketState,
  getCodexResponsesWebSocketURL,
  hasCodexResponsesWebSocketOverrideReplacement,
  getInitialApiFormatForChannel,
  getModelProtocolsForApiFormat,
  toHttpBaseURL,
  translateLegacyCodexModelProtocols,
} from '../data/protocol-options';
import {
  Channel,
  ChannelType,
  ApiFormat,
  ChannelSettings,
  ChannelQuotaRoutingMode,
  RetryableErrorPattern,
  createChannelInputSchema,
  updateChannelInputSchema,
} from '../data/schema';
import { xaiDecodeSSO, xaiOAuthExchange, xaiOAuthStart } from '../data/xai';
import { ProxyConfig, useOAuthFlow } from '../hooks/use-oauth-flow';
import { mergeChannelSettingsForUpdate } from '../utils/merge';
import { isValidModelPattern } from '../utils/pattern';
import { ProxyType } from './channels-proxy-dialog';
import { CopilotDeviceFlow } from './copilot-device-flow';
import { ManualModelBadge } from './manual-model-badge';

interface Props {
  currentRow?: Channel;
  duplicateFromRow?: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  showModelsPanel?: boolean;
}

const duplicateNameRegex = /^(.*) \((\d+)\)$/;

type ApiFormatOption = ApiFormat;
type ResponsesTransport = 'http' | 'websocket';

const OPENAI_RESPONSES_WEBSOCKET: ApiFormatOption = 'openai/responses-ws';
// A single trailing # suppresses automatic version suffix appending while still
// allowing the Responses transformer to append /responses. Do not replace these
// defaults with ## unless the upstream URL should be used fully raw.
const OPENAI_RESPONSES_WEBSOCKET_BASE_URL = 'wss://api.openai.com/v1#';

function getResponsesTransportFromBaseURL(baseURL?: string): ResponsesTransport {
  return baseURL?.trim().toLowerCase().startsWith('ws') ? 'websocket' : 'http';
}

function baseURLMatchesResponsesTransport(baseURL: string | undefined, transport: ResponsesTransport): boolean {
  const normalized = baseURL?.trim().toLowerCase() ?? '';
  if (transport === 'websocket') {
    return normalized.startsWith('ws://') || normalized.startsWith('wss://');
  }
  return normalized.startsWith('http://') || normalized.startsWith('https://');
}

function getResponsesTransportBaseURLError(transport: ResponsesTransport): string {
  return transport === 'websocket'
    ? 'channels.dialogs.fields.baseURL.errors.websocketScheme'
    : 'channels.dialogs.fields.baseURL.errors.httpScheme';
}

// Dialog-init recall for the per-channel quota routing mode: an absent
// settings field displays as INHERIT (the backend stores "" for inherit).
export function recallQuotaRoutingMode(settings: ChannelSettings | null | undefined): ChannelQuotaRoutingMode {
  return settings?.quotaRoutingMode ?? 'INHERIT';
}

// Single dialog-state -> GraphQL-input mapping: every wire value passes
// through unchanged. Explicit INHERIT is required to override the merge
// whitelist's stored-value fallback; the backend maps INHERIT to empty storage.
export function quotaRoutingModeSettingsPatch(mode: ChannelQuotaRoutingMode): Partial<ChannelSettings> {
  return mode === 'INHERIT' ? { quotaRoutingMode: 'INHERIT' } : { quotaRoutingMode: mode };
}

function formatRetryableStatusCodes(codes: number[] | null | undefined): string {
  return (codes ?? []).join(', ');
}

function parseRetryableStatusCodesInput(value: string): number[] | null {
  const tokens = value.split(/[,\s]+/).filter(Boolean);

  if (tokens.length === 0) {
    return [];
  }

  const codes: number[] = [];
  for (const token of tokens) {
    if (!/^\d+$/.test(token)) {
      return null;
    }

    const code = Number(token);
    if (code < 400 || code > 599) {
      return null;
    }

    codes.push(code);
  }

  return Array.from(new Set(codes)).sort((a, b) => a - b);
}

function formatRetryableErrorPatterns(patterns: RetryableErrorPattern[] | null | undefined): string {
  return (patterns ?? []).map(({ pattern, regex }) => (regex ? `regex:${pattern}` : pattern)).join('\n');
}

function parseRetryableErrorPatternsInput(value: string): RetryableErrorPattern[] | null {
  const patterns: RetryableErrorPattern[] = [];
  const seen = new Set<string>();

  for (const rawLine of value.split(/\r?\n/)) {
    let line = rawLine.trim();
    if (!line) {
      continue;
    }

    const regex = line.toLowerCase().startsWith('regex:');
    if (regex) {
      line = line.slice('regex:'.length).trim();
    }

    if (!line) {
      return null;
    }

    const key = `${regex}\0${line}`;
    if (!seen.has(key)) {
      seen.add(key);
      patterns.push({ pattern: line, regex });
    }
  }

  return patterns;
}

function getResponsesTransportFromChannel(channel?: Pick<Channel, 'type' | 'baseURL' | 'endpoints'>): ResponsesTransport {
  // Codex always edits an HTTP(S) Base URL; WebSocket is its separate checkbox.
  if (channel?.type === 'codex') return 'http';
  const responsesEndpoint = channel?.endpoints?.find((endpoint) => endpoint.apiFormat === OPENAI_RESPONSES);
  if (responsesEndpoint?.transport === 'http' || responsesEndpoint?.transport === 'websocket') return responsesEndpoint.transport;
  return getResponsesTransportFromBaseURL(channel?.baseURL);
}

// Vertex credentials are project-bound triples; the form keeps the same shape as
// the backend contract so project/region stay attached to their key.
type VertexAPIKeyCredential = NonNullable<NonNullable<Channel['credentials']>['vertexApiKeys']>[number];

function isVertexChannelType(channelType: ChannelType | undefined): boolean {
  return channelType === 'gemini_vertex';
}

function emptyVertexAPIKey(): VertexAPIKeyCredential {
  return { projectID: '', apiKey: '', region: 'global' };
}

function getResponsesWebSocketBaseURL(channelType: ChannelType): string | undefined {
  if (channelType === 'openai_responses') return OPENAI_RESPONSES_WEBSOCKET_BASE_URL;
  return undefined;
}

// Custom hook for debounced value
// Memoized SupportedModelItem component
const SupportedModelItem = memo(({ model, isManual, onRemove }: { model: string; isManual: boolean; onRemove: () => void }) => (
  <div className='hover:bg-accent flex items-center gap-2 rounded-md p-2 text-sm'>
    <Tooltip>
      <TooltipTrigger asChild>
        <span className='w-0 flex-1 cursor-help truncate'>{model}</span>
      </TooltipTrigger>
      <TooltipContent>
        <p className='max-w-xs break-all'>{model}</p>
      </TooltipContent>
    </Tooltip>
    <ManualModelBadge isManual={isManual} />
    <Button type='button' variant='ghost' size='sm' className='hover:text-destructive h-6 w-6 shrink-0 p-0' onClick={onRemove}>
      <X className='h-3 w-3' />
    </Button>
  </div>
));
SupportedModelItem.displayName = 'SupportedModelItem';

function getDuplicateBaseName(name: string) {
  const match = name.match(duplicateNameRegex);
  if (match?.[1]) {
    return match[1];
  }
  return name;
}

function getNextDuplicateName(name: string, existingNames: Set<string>) {
  const baseName = getDuplicateBaseName(name);
  let i = 1;
  for (;;) {
    const candidate = `${baseName} (${i})`;
    if (!existingNames.has(candidate)) {
      return candidate;
    }
    i++;
  }
}

// Providers that are always OAuth (no third-party API key mode)
const alwaysOAuthProviderKeys = ['antigravity', 'github_copilot', 'xai_subscription'];

function isOfficialCodexChannel(channel: { credentials?: { apiKey?: string | null } | null }): boolean {
  try {
    const apiKey = channel.credentials?.apiKey || '';
    const json = JSON.parse(apiKey);
    return !!((json.access_token && json.refresh_token) || (json.tokens?.access_token && json.tokens?.refresh_token));
  } catch {
    return false;
  }
}

function isOfficialClaudeCodeChannel(channel: { credentials?: { apiKey?: string | null } | null; baseURL: string }): boolean {
  const apiKey = channel.credentials?.apiKey || '';
  const defaultURL = getDefaultBaseURL('claudecode');
  return apiKey.includes('sk-ant-oat') || apiKey.includes('sk-ant-api03') || channel.baseURL === defaultURL;
}

function extractCodexAuthJSONText(apiKey: string | undefined): string | undefined {
  if (!apiKey) return apiKey;

  try {
    const parsed = JSON.parse(apiKey);
    return parsed?.tokens?.access_token ? apiKey : undefined;
  } catch {
    return undefined;
  }
}

// Codex forms edit only an HTTP(S) Base URL: official and auth.json rows show the
// canonical Base URL and legacy WS(S) third-party rows their HTTP(S) equivalent.
// Opening the form never changes the stored channel.
function getInitialBaseURL(row: Pick<Channel, 'type' | 'baseURL' | 'credentials'>): string {
  if (row.type !== 'codex') return row.baseURL;
  return isOfficialCodexChannel(row) ? getDefaultBaseURL('codex') : toHttpBaseURL(row.baseURL);
}

// Used by both the submit path and fetch-models: reads the access_token out of
// an OAuth/auth.json credential, falling back to the raw string.

// A third-party Codex channel authenticates with a plain API key only. An OAuth
// or auth.json row that is copied/edited into third-party mode still carries its
// old JSON in `credentials.apiKey`; the backend replaces credentials wholesale,
// so the inherited value must be dropped before submit or it silently revives.
// One group in the grouped-key editor. A group is a required remark (name) plus
// one or more keys: several keys rotate round-robin inside the group.
type KeyGroupFormRow = {
  name?: string;
  apiKeys?: string[];
  models?: string[];
  enabled?: boolean;
};

// Read side: echo stored groups into form rows. Legacy channels have no groups,
// so this returns an empty list and the plain apiKeys editor stays in use. Groups
// stored before a group could hold several keys carry a single apiKey, which is
// folded into the key list here.
function normalizeKeyGroupsFromCredentials(
  credentials?: {
    apiKey?: string | null;
    apiKeys?: string[] | null;
    keyGroups?:
      | { name?: string | null; apiKeys?: string[] | null; apiKey?: string | null; models?: string[] | null; enabled?: boolean }[]
      | null;
  } | null
): KeyGroupFormRow[] {
  const groups = (credentials?.keyGroups || []).map((group) => {
    const keys = (group.apiKeys || []).map((key) => (key || '').trim()).filter((key) => key.length > 0);

    if (keys.length === 0 && (group.apiKey || '').trim()) {
      keys.push((group.apiKey || '').trim());
    }

    return {
      name: group.name || '',
      apiKeys: keys,
      models: group.models || [],
      enabled: group.enabled ?? true,
    };
  });

  // Legacy flat credentials: fold them into a single default group so the
  // grouped editor shows the stored keys instead of hiding them. A plain
  // OAuth/auth.json apiKey is NOT a key and must not migrate.
  const existingKeys = new Set(groups.flatMap((group) => group.apiKeys || []));
  const legacyKeys: string[] = [];
  for (const key of credentials?.apiKeys || []) {
    const trimmed = (key || '').trim();
    if (trimmed && !existingKeys.has(trimmed) && !isOAuthJSONString(trimmed)) {
      legacyKeys.push(trimmed);
      existingKeys.add(trimmed);
    }
  }
  const flatKey = (credentials?.apiKey || '').trim();
  if (flatKey && !existingKeys.has(flatKey) && !isOAuthJSONString(flatKey)) {
    legacyKeys.push(flatKey);
    existingKeys.add(flatKey);
  }
  if (legacyKeys.length > 0) {
    groups.unshift({
      name: '',
      apiKeys: legacyKeys,
      models: [],
      enabled: true,
    });
  }

  return groups;
}

// isOAuthJSONString mirrors the backend's isOAuthJSON: a JSON object carrying an
// access_token is an OAuth credential blob, not a plain API key.
function isOAuthJSONString(value: string): boolean {
  const trimmed = value.trim();
  return trimmed.startsWith('{') && trimmed.includes('access_token');
}

// Display label for a group. The remark is required by the form, so persisted
// groups always have one; the positional fallback only protects legacy data.
function groupModelLabel(
  group: { name?: string | null; apiKeys?: (string | null)[] | null; apiKey?: string | null } | undefined,
  index: number,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  const name = (group?.name || '').trim();
  if (name) return name;

  return t('channels.dialogs.fields.keyGroup.unnamedGroup', { index: index + 1 });
}

// Turn raw textarea content into a key list: one key per line, blanks dropped,
// duplicates removed. This is what makes several keys in one group rotate.
function parseKeyList(text: string): string[] {
  const seen = new Set<string>();
  const keys: string[] = [];

  for (const line of text.split('\n')) {
    const key = line.trim();

    if (!key || seen.has(key)) continue;

    seen.add(key);
    keys.push(key);
  }

  return keys;
}

const keyListToText = (keys?: string[] | null): string => (keys || []).join('\n');

function createEmptyKeyGroup(index: number): KeyGroupFormRow {
  return {
    name: `分组${index}`,
    apiKeys: [],
    models: [],
    enabled: true,
  };
}

// Grouped API key editor.
//
// 分组不是独立模块，只是给 API Key 打的备注。每一行就是"备注 + 该组的 Key 填写框"，
// 和上方默认输入框完全同款：一个框，每行一个 Key，多个 Key 在该分组内轮询。
// 想加几个分组就点几次"添加分组"。这里没有任何模型控件——获取/勾选模型统一在
// 下方"支持的模型"栏里按分组进行。
interface KeyGroupsEditorProps {
  rows: KeyGroupFormRow[];
  onAdd: () => void;
  onRemove: (index: number) => void;
  onChangeName: (index: number, name: string) => void;
  onChangeKeys: (index: number, keys: string[], options?: { validate?: boolean }) => void;
  onViewKeys: (index: number) => void;
}

// Masking placeholder. Masked text must never be parsed back into the key list:
// a blur used to re-parse the dots, replace every key with the placeholder, and
// then trip the duplicate-key validation.
const MASKED_KEY_PLACEHOLDER = '••••••••';

function KeyGroupsEditor({ rows, onAdd, onRemove, onChangeName, onChangeKeys, onViewKeys }: KeyGroupsEditorProps) {
  const { t } = useTranslation();
  const [visibleKeys, setVisibleKeys] = useState<Record<number, boolean>>({});

  return (
    <div className='space-y-3'>
      {rows.map((row, index) => {
        const keys = row.apiKeys || [];
        // A group without keys has nothing to mask: start revealed so the very
        // first key can be typed without hunting for the eye icon. Groups that
        // already carry keys keep the masked state until revealed explicitly.
        const isKeysVisible = visibleKeys[index] ?? keys.length === 0;

        return (
          <div key={index} className='bg-muted/20 space-y-3 rounded-lg border p-3 shadow-xs' data-testid={`channel-key-group-row-${index}`}>
            <div className='flex items-center gap-2'>
              <Badge variant='secondary' className='shrink-0 font-normal'>
                {t('channels.dialogs.fields.keyGroup.groupNumber', { index: index + 1 })}
              </Badge>
              <Input
                value={row.name || ''}
                onChange={(e) => onChangeName(index, e.target.value)}
                placeholder={t('channels.dialogs.fields.keyGroup.namePlaceholder')}
                className='h-8 min-w-0 flex-1'
                aria-label={t('channels.dialogs.fields.keyGroup.namePlaceholder')}
                data-testid={`channel-key-group-name-${index}`}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon'
                onClick={() => onRemove(index)}
                className='text-muted-foreground hover:text-destructive h-8 w-8 shrink-0'
                aria-label={t('channels.dialogs.fields.keyGroup.removeGroup')}
              >
                <Trash2 className='h-4 w-4' />
              </Button>
            </div>

            <div className='relative'>
              <Textarea
                value={isKeysVisible ? keyListToText(keys) : keys.map(() => MASKED_KEY_PLACEHOLDER).join('\n')}
                onChange={(e) => {
                  if (!isKeysVisible) return;
                  onChangeKeys(index, parseKeyList(e.target.value), { validate: false });
                }}
                onBlur={(e) => {
                  if (!isKeysVisible) return;
                  onChangeKeys(index, parseKeyList(e.target.value), { validate: true });
                }}
                placeholder={t('channels.dialogs.fields.keyGroup.keysPlaceholder')}
                className='bg-background min-h-[76px] resize-y pr-16 font-mono text-sm'
                autoComplete='new-password'
                data-form-type='other'
                spellCheck={false}
                readOnly={!isKeysVisible}
                data-testid={`channel-key-group-keys-${index}`}
              />
              <div className='absolute top-1.5 right-1.5 flex gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='h-7 w-7'
                  onClick={() => setVisibleKeys((prev) => ({ ...prev, [index]: !prev[index] }))}
                  aria-label={t('channels.dialogs.fields.apiKey.revealToEditHint')}
                >
                  {isKeysVisible ? <EyeOff className='h-3.5 w-3.5' /> : <Eye className='h-3.5 w-3.5' />}
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='h-7 w-7'
                  onClick={() => onViewKeys(index)}
                  aria-label={t('channels.dialogs.fields.keyGroup.viewKeys')}
                >
                  <PanelLeft className='h-3.5 w-3.5' />
                </Button>
              </div>
            </div>

            <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.keyGroup.groupHint', { count: keys.length })}</p>
          </div>
        );
      })}

      <Button type='button' variant='outline' size='sm' onClick={onAdd} data-testid='channel-add-key-group'>
        <Plus className='mr-1.5 h-3.5 w-3.5' />
        {t('channels.dialogs.fields.keyGroup.addGroup')}
      </Button>
    </div>
  );
}

export function ChannelsActionDialog({ currentRow, duplicateFromRow, open, onOpenChange, showModelsPanel = false }: Props) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;
  const isDuplicate = !!duplicateFromRow && !isEdit;
  const initialRow: Channel | undefined = currentRow || duplicateFromRow;
  const createChannel = useCreateChannel();
  const duplicateChannel = useDuplicateChannel();
  const updateChannelSettings = useUpdateChannelSettings();
  const fetchModels = useFetchModels();
  const syncChannelModels = useSyncChannelModels();
  const { data: allChannelNames = [], isSuccess: allChannelNamesLoaded } = useAllChannelNames({ enabled: open && isDuplicate });
  const { data: allTags = [], isLoading: isLoadingTags } = useAllChannelTags();
  const { data: proxyPresets = [] } = useProxyPresets();
  const saveProxyPreset = useSaveProxyPreset();
  const { hasSystemScope } = usePermissions();
  const { data: quotaRoutingSettings } = useQuotaRoutingSettings();
  const canReadSystemSettings = hasSystemScope('read_settings');
  const { data: userAgentPassThroughSettings } = useUserAgentPassThroughSettings({ enabled: canReadSystemSettings });
  const { data: passThroughSettings } = usePassThroughSettings({ enabled: canReadSystemSettings });
  const [supportedModels, setSupportedModels] = useState<string[]>(() => initialRow?.supportedModels || []);
  const [manualModels, setManualModels] = useState<string[]>(() => initialRow?.manualModels || []);
  const [newModel, setNewModel] = useState('');
  const providerRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const providerListRef = useRef<HTMLDivElement | null>(null);
  const providerHorizontalScrollRef = useHorizontalScroll<HTMLDivElement>();

  // Expandable panel states
  const hasAutoSetDuplicateNameRef = useRef(false);
  const codexThirdPartyBaseURLRef = useRef('');
  const [showCommandCodeAuthCookie, setShowCommandCodeAuthCookie] = useState(false);
  const [showOllamaAuthCookie, setShowOllamaAuthCookie] = useState(false);
  const [authMode, setAuthMode] = useState<'official' | 'auth-json' | 'third-party'>('official');
  const [codexAuthJSONText, setCodexAuthJSONText] = useState('');
  const [xaiSSOToken, setXaiSSOToken] = useState('');
  const [isImportingXAISSO, setIsImportingXAISSO] = useState(false);
  const [patternError, setPatternError] = useState<string | null>(null);

  // Refs for virtual scrolling

  const [proxyType, setProxyType] = useState<ProxyType>(() => {
    if (initialRow?.settings?.proxy?.type) {
      return initialRow.settings.proxy.type as ProxyType;
    }
    return ProxyType.ENVIRONMENT;
  });
  const [proxyUrl, setProxyUrl] = useState(() => initialRow?.settings?.proxy?.url || '');
  const [proxyUsername, setProxyUsername] = useState(() => initialRow?.settings?.proxy?.username || '');
  const [proxyPassword, setProxyPassword] = useState(() => initialRow?.settings?.proxy?.password || '');
  const [passThroughUserAgent, setPassThroughUserAgent] = useState<boolean | null>(() => {
    return initialRow?.settings?.passThroughUserAgent ?? null;
  });
  const [passThroughBody, setPassThroughBody] = useState<boolean | null>(() => {
    return initialRow?.settings?.passThroughBody ?? null;
  });
  const [quotaRoutingMode, setQuotaRoutingMode] = useState<ChannelQuotaRoutingMode>(() => recallQuotaRoutingMode(initialRow?.settings));
  const [retryableStatusCodesText, setRetryableStatusCodesText] = useState(() =>
    formatRetryableStatusCodes(initialRow?.settings?.retryableStatusCodes)
  );
  const [retryableErrorPatternsText, setRetryableErrorPatternsText] = useState(() =>
    formatRetryableErrorPatterns(initialRow?.settings?.retryableErrorPatterns)
  );
  const userAgentInheritLabel = userAgentPassThroughSettings
    ? t('channels.dialogs.userAgentPassThrough.inheritWithValue', {
        value: t(
          userAgentPassThroughSettings.enabled
            ? 'channels.dialogs.userAgentPassThrough.enabled'
            : 'channels.dialogs.userAgentPassThrough.disabled'
        ),
      })
    : t('channels.dialogs.userAgentPassThrough.inherit');
  const passThroughInheritLabel = passThroughSettings
    ? t('channels.dialogs.bodyPassThrough.inheritWithValue', {
        value: t(passThroughSettings.enabled ? 'channels.dialogs.bodyPassThrough.enabled' : 'channels.dialogs.bodyPassThrough.disabled'),
      })
    : t('channels.dialogs.bodyPassThrough.inherit');

  // Memoized proxy config for OAuth exchange
  const proxyConfig: ProxyConfig | undefined = useMemo(() => {
    if (proxyType === ProxyType.URL && proxyUrl) {
      return {
        type: proxyType,
        url: proxyUrl,
        ...(proxyUsername && { username: proxyUsername }),
        ...(proxyPassword && { password: proxyPassword }),
      };
    }
    return undefined;
  }, [proxyType, proxyUrl, proxyUsername, proxyPassword]);

  const handleProxyPresetSelect = (presetUrl: string) => {
    const preset = proxyPresets.find((p) => p.url === presetUrl);
    if (preset) {
      setProxyType(ProxyType.URL);
      setProxyUrl(preset.url);
      setProxyUsername(preset.username || '');
      setProxyPassword(preset.password || '');
    }
  };

  // OAuth flows using the reusable hook
  // OAuth credentials are stored in apiKey field as JSON string, not in apiKeys array
  const codexOAuth = useOAuthFlow({
    startFn: codexOAuthStart,
    exchangeFn: codexOAuthExchange,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });

  const claudecodeOAuth = useOAuthFlow({
    startFn: claudecodeOAuthStart,
    exchangeFn: claudecodeOAuthExchange,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });

  const xaiOAuth = useOAuthFlow({
    startFn: xaiOAuthStart,
    exchangeFn: xaiOAuthExchange,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });

  const antigravityOAuth = useOAuthFlow({
    startFn: antigravityOAuthStart,
    exchangeFn: antigravityOAuthExchange,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });
  const { reset: resetCodexOAuth } = codexOAuth;
  const { reset: resetClaudecodeOAuth } = claudecodeOAuth;
  const { reset: resetXaiOAuth } = xaiOAuth;
  const { reset: resetAntigravityOAuth } = antigravityOAuth;

  // Provider-based selection state
  const [selectedProvider, setSelectedProvider] = useState<string>(() => {
    if (initialRow) {
      return getProviderFromChannelType(initialRow.type) || 'openai';
    }
    return 'openai';
  });
  const [selectedApiFormat, setSelectedApiFormat] = useState<ApiFormat>(() => {
    if (initialRow) {
      return getInitialApiFormatForChannel(
        initialRow.type,
        CHANNEL_CONFIGS[initialRow.type as ChannelType]?.apiFormat || OPENAI_CHAT_COMPLETIONS,
        initialRow.settings?.modelProtocols
      );
    }
    return OPENAI_CHAT_COMPLETIONS;
  });
  const [responsesTransport, setResponsesTransport] = useState<ResponsesTransport>(() => getResponsesTransportFromChannel(initialRow));
  const initialCodexResponses = useMemo(() => getCodexResponsesWebSocketState(initialRow), [initialRow]);
  const [codexResponsesWebSocket, setCodexResponsesWebSocket] = useState(initialCodexResponses.enabled);
  const [useGeminiVertex, setUseGeminiVertex] = useState(() => {
    if (initialRow) {
      return initialRow.type === 'gemini_vertex';
    }
    return false;
  });
  const [useAnthropicAws, setUseAnthropicAws] = useState(() => {
    if (initialRow) {
      return initialRow.type === 'anthropic_aws';
    }
    return false;
  });
  const [useKimiCoding, setUseKimiCoding] = useState(() => {
    if (initialRow) {
      return initialRow.type === 'moonshot_coding';
    }
    return false;
  });

  useEffect(() => {
    if (!initialRow) return;

    const provider = getProviderFromChannelType(initialRow.type) || 'openai';
    setSelectedProvider(provider);
    const apiFormat = getInitialApiFormatForChannel(
      initialRow.type,
      CHANNEL_CONFIGS[initialRow.type as ChannelType]?.apiFormat || OPENAI_CHAT_COMPLETIONS,
      initialRow.settings?.modelProtocols
    );
    setSelectedApiFormat(apiFormat);
    setResponsesTransport(getResponsesTransportFromChannel(initialRow));
    setCodexResponsesWebSocket(getCodexResponsesWebSocketState(initialRow).enabled);
    setUseGeminiVertex(initialRow.type === 'gemini_vertex');
    setUseAnthropicAws(initialRow.type === 'anthropic_aws');
    setUseKimiCoding(initialRow.type === 'moonshot_coding');

    // Detect authMode for codex and claudecode
    if (initialRow.type === 'codex') {
      setAuthMode(isOfficialCodexChannel(initialRow) ? 'official' : 'third-party');
      setCodexAuthJSONText(extractCodexAuthJSONText(initialRow.credentials?.apiKey ?? undefined) || '');
    } else if (initialRow.type === 'claudecode') {
      setAuthMode(isOfficialClaudeCodeChannel(initialRow) ? 'official' : 'third-party');
    }
  }, [initialRow]);

  useEffect(() => {
    if (!open) {
      hasAutoSetDuplicateNameRef.current = false;
      resetCodexOAuth();
      resetClaudecodeOAuth();
      resetAntigravityOAuth();
      resetXaiOAuth();
      setCodexAuthJSONText('');
      codexThirdPartyBaseURLRef.current = '';
      setCodexResponsesWebSocket(initialCodexResponses.enabled);
      setXaiSSOToken('');
    }
  }, [open, resetCodexOAuth, resetClaudecodeOAuth, resetAntigravityOAuth, resetXaiOAuth, initialCodexResponses.enabled]);

  useEffect(() => {
    if (!open) {
      setShowCommandCodeAuthCookie(false);
      setShowOllamaAuthCookie(false);
      setPatternError(null);
      setShowGroupModelsPanel(false);
      setShowKeyPanel(false);
      setActiveKeyGroupIndex(null);
      setActiveGroupModelIndex(null);
      setGroupModelSelection([]);
      setGroupModelsSearch('');
      setUnifiedFetchedModels([]);
      setModelSourceGroups({});
      setFetchingAllGroups(false);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;

    const timer = setTimeout(() => {
      const target = providerRefs.current[selectedProvider];
      const container = providerListRef.current;
      if (target && container) {
        const isHorizontal =
          (window.getComputedStyle(container).overflowX === 'auto' || window.getComputedStyle(container).overflowX === 'scroll') &&
          container.scrollWidth > container.clientWidth;
        if (isHorizontal) {
          const targetCenter = target.offsetLeft + target.clientWidth / 2;
          container.scrollLeft = Math.max(0, targetCenter - container.clientWidth / 2);
        } else {
          const targetCenter = target.offsetTop + target.clientHeight / 2;
          container.scrollTop = Math.max(0, targetCenter - container.clientHeight / 2);
        }
      }
    }, 100);

    return () => clearTimeout(timer);
  }, [open, isEdit, selectedProvider]);

  const setProviderListRef = useCallback(
    (el: HTMLDivElement | null) => {
      providerListRef.current = el;
      providerHorizontalScrollRef(el);
    },
    [providerHorizontalScrollRef]
  );

  // Get available providers (excluding fake types)
  const availableProviders = useMemo(
    () =>
      Object.entries(PROVIDER_CONFIGS)
        .filter(([, config]) => {
          // Filter out providers that only have fake types
          const nonFakeTypes = config.channelTypes.filter((t) => !t.endsWith('_fake'));
          return nonFakeTypes.length > 0;
        })
        .map(([key, config]) => ({
          key,
          label: t(`channels.providers.${key}`),
          icon: config.icon,
          channelTypes: config.channelTypes.filter((t) => !t.endsWith('_fake')),
        })),
    [t]
  );

  // Get available API formats for selected provider
  const availableApiFormats = useMemo(() => {
    return getApiFormatsForProvider(selectedProvider);
  }, [selectedProvider]);

  const availableApiFormatOptions = useMemo<ApiFormatOption[]>(() => {
    if (selectedProvider !== 'openai') return availableApiFormats;

    const options: ApiFormatOption[] = [];
    for (const format of availableApiFormats) {
      options.push(format);
      if (format === OPENAI_RESPONSES) {
        options.push(OPENAI_RESPONSES_WEBSOCKET);
      }
    }
    return options;
  }, [availableApiFormats, selectedProvider]);

  const selectedApiFormatOption: ApiFormatOption =
    selectedApiFormat === OPENAI_RESPONSES && responsesTransport === 'websocket' ? OPENAI_RESPONSES_WEBSOCKET : selectedApiFormat;

  const getApiFormatLabel = useCallback(
    (format: ApiFormat) => {
      return t(`channels.dialogs.fields.apiFormat.formats.${format}`);
    },
    [t]
  );

  const getApiFormatOptionLabel = useCallback(
    (format: ApiFormatOption) => {
      if (format === OPENAI_RESPONSES_WEBSOCKET) {
        return t('channels.dialogs.fields.apiFormat.formats.openai/responses_websocket');
      }
      return getApiFormatLabel(format);
    },
    [getApiFormatLabel, t]
  );

  // Determine the actual channel type based on provider and API format
  const derivedChannelType = useMemo(() => {
    // If gemini/contents is selected and vertex checkbox is checked, use gemini_vertex
    if (selectedApiFormat === 'gemini/contents' && useGeminiVertex) {
      return 'gemini_vertex';
    }

    // If anthropic/messages is selected, check which variant is selected
    if (selectedApiFormat === 'anthropic/messages') {
      if (useAnthropicAws) return 'anthropic_aws';
      if (useKimiCoding) return 'moonshot_coding';
    }

    return getChannelTypeForApiFormat(selectedProvider, selectedApiFormat) || 'openai';
  }, [selectedProvider, selectedApiFormat, useGeminiVertex, useAnthropicAws, useKimiCoding]);

  const formSchema = isEdit ? updateChannelInputSchema : createChannelInputSchema;

  type ChannelFormValues = z.infer<typeof createChannelInputSchema> & z.infer<typeof updateChannelInputSchema>;
  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(formSchema) as any,
    defaultValues:
      isEdit && currentRow
        ? {
            type: currentRow.type,
            baseURL: getInitialBaseURL(currentRow),
            name: currentRow.name,
            policies: currentRow.policies ?? { stream: 'unlimited' },
            supportedModels: currentRow.supportedModels,
            autoSyncSupportedModels: currentRow.autoSyncSupportedModels,
            autoSyncModelPattern: currentRow.autoSyncModelPattern || '',
            defaultTestModel: currentRow.defaultTestModel,
            tags: currentRow.tags || [],
            remark: currentRow.remark || '',
            credentials: {
              // OAuth 类型的凭据存储在 apiKey 字段，不放入 apiKeys
              apiKey: currentRow.credentials?.apiKey ?? undefined,
              apiKeys: [],
              keyGroups: normalizeKeyGroupsFromCredentials(currentRow.credentials),
              vertexApiKeys: currentRow.type === 'gemini_vertex' ? (currentRow.credentials?.vertexApiKeys ?? undefined) : [],
              managementApiKey: currentRow.credentials?.managementApiKey || undefined,
              gcp: {
                region: currentRow.credentials?.gcp?.region || '',
                projectID: currentRow.credentials?.gcp?.projectID || '',
                jsonData: currentRow.credentials?.gcp?.jsonData || '',
              },
            },
            settings: currentRow.settings ?? undefined,
          }
        : duplicateFromRow
          ? {
              type: duplicateFromRow.type,
              baseURL: getInitialBaseURL(duplicateFromRow),
              name: duplicateFromRow.name,
              policies: duplicateFromRow.policies ?? { stream: 'unlimited' },
              supportedModels: duplicateFromRow.supportedModels,
              autoSyncSupportedModels: duplicateFromRow.autoSyncSupportedModels,
              autoSyncModelPattern: duplicateFromRow.autoSyncModelPattern || '',
              defaultTestModel: duplicateFromRow.defaultTestModel,
              tags: duplicateFromRow.tags || [],
              remark: duplicateFromRow.remark || '',
              settings: duplicateFromRow.settings ?? undefined,
              credentials: {
                // OAuth 类型的凭据存储在 apiKey 字段，不放入 apiKeys
                apiKey: duplicateFromRow.credentials?.apiKey ?? undefined,
                apiKeys: [],
                keyGroups: normalizeKeyGroupsFromCredentials(duplicateFromRow.credentials),
                vertexApiKeys: duplicateFromRow.type === 'gemini_vertex' ? (duplicateFromRow.credentials?.vertexApiKeys ?? undefined) : [],
                managementApiKey: duplicateFromRow.credentials?.managementApiKey || undefined,
                gcp: {
                  region: duplicateFromRow.credentials?.gcp?.region || '',
                  projectID: duplicateFromRow.credentials?.gcp?.projectID || '',
                  jsonData: duplicateFromRow.credentials?.gcp?.jsonData || '',
                },
              },
            }
          : {
              type: derivedChannelType,
              baseURL: getDefaultBaseURL(derivedChannelType),
              name: '',
              policies: { stream: 'unlimited' },
              credentials: {
                apiKeys: [],
                keyGroups: [createEmptyKeyGroup(1)],
                vertexApiKeys: isVertexChannelType(derivedChannelType) ? [emptyVertexAPIKey()] : [],
                managementApiKey: undefined,
                gcp: {
                  region: '',
                  projectID: '',
                  jsonData: '',
                },
              },
              supportedModels: [],
              defaultTestModel: '',
              tags: [],
              remark: '',
              settings: undefined,
            },
  });

  const vertexApiKeyFields = useFieldArray({ control: form.control, name: 'credentials.vertexApiKeys' });
  const keyGroups = form.watch('credentials.keyGroups');
  // Per-row fetch state: which group row is currently probing its upstream /models.
  const [fetchingGroupIndex, setFetchingGroupIndex] = useState<number | null>(null);
  // Which group row's model allowlist is open in the side panel, and the fetched
  // models that came back from a unified "fetch all groups" run.
  const [activeGroupModelIndex, setActiveGroupModelIndex] = useState<number | null>(null);
  const [showGroupModelsPanel, setShowGroupModelsPanel] = useState(false);
  const [showKeyPanel, setShowKeyPanel] = useState(false);
  const [activeKeyGroupIndex, setActiveKeyGroupIndex] = useState<number | null>(null);
  const [groupModelSelection, setGroupModelSelection] = useState<string[]>([]);
  const [groupModelsSearch, setGroupModelsSearch] = useState('');
  // Source label per model id, so the panel can show which group a model came from.
  const [modelSourceGroups, setModelSourceGroups] = useState<Record<string, string[]>>({});
  const [fetchingAllGroups, setFetchingAllGroups] = useState(false);
  // Flat list of every model gathered by "fetch all", tagged with its group.
  const [unifiedFetchedModels, setUnifiedFetchedModels] = useState<{ model: string; group: string; groupIndex: number }[]>([]);
  const isSubmitting = createChannel.isPending || duplicateChannel.isPending || updateChannelSettings.isPending;

  useEffect(() => {
    if (!open || !isDuplicate || !duplicateFromRow) return;
    if (!allChannelNamesLoaded) return;
    if (hasAutoSetDuplicateNameRef.current) return;

    const currentName = form.getValues('name');
    if (currentName !== duplicateFromRow.name) {
      return;
    }

    const nextName = getNextDuplicateName(duplicateFromRow.name, new Set(allChannelNames));
    form.setValue('name', nextName);
    hasAutoSetDuplicateNameRef.current = true;
  }, [open, isDuplicate, duplicateFromRow, allChannelNamesLoaded, allChannelNames, form]);
  const selectedType = form.watch('type') as ChannelType | undefined;
  const isGeminiVertex = (selectedType || derivedChannelType) === 'gemini_vertex';

  const activeChannelType = selectedType || derivedChannelType;
  const isCodexType = activeChannelType === 'codex';
  const isClaudeCodeType = activeChannelType === 'claudecode';
  const isCopilotType = activeChannelType === 'github_copilot';
  const isXAISubscriptionType = activeChannelType === 'xai_subscription';
  const isZenmuxType = ['zenmux', 'zenmux_responses', 'zenmux_anthropic', 'zenmux_gemini', 'zenmux_video'].includes(activeChannelType);
  const isCommandCodeType = activeChannelType === 'commandcode' || activeChannelType === 'commandcode_anthropic';
  const isOllamaType = activeChannelType === 'ollama' || activeChannelType === 'ollama_anthropic';

  // OAuth providers cannot have their provider/API format changed during edit.
  // Derived from currentRow credentials so it stays stable across re-renders
  // and is not affected by mutable authMode state.
  const isOAuthChannel = useMemo(() => {
    if (!isEdit || !currentRow) return false;
    if (alwaysOAuthProviderKeys.includes(currentRow.type)) return true;
    if (currentRow.type === 'codex') return isOfficialCodexChannel(currentRow);
    if (currentRow.type === 'claudecode') return isOfficialClaudeCodeChannel(currentRow);
    return false;
  }, [isEdit, currentRow]);

  const wrapUnsupported = useCallback(
    (enabled: boolean, children: React.ReactNode, wrapperClassName: string) => {
      if (!enabled) return children;
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className={wrapperClassName}>{children}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('channels.dialogs.fields.unsupported')}</p>
          </TooltipContent>
        </Tooltip>
      );
    },
    [t]
  );

  const baseURLPlaceholder = useMemo(() => {
    const currentType = selectedType || derivedChannelType;
    if (selectedApiFormat === OPENAI_RESPONSES && responsesTransport === 'websocket') {
      const websocketBaseURL = getResponsesWebSocketBaseURL(currentType);
      if (websocketBaseURL) {
        return websocketBaseURL;
      }
    }
    const defaultURL = getDefaultBaseURL(currentType);
    if (defaultURL) {
      return defaultURL;
    }
    return t('channels.dialogs.fields.baseURL.placeholder');
  }, [selectedType, derivedChannelType, selectedApiFormat, responsesTransport, t]);

  const watchedBaseURL = form.watch('baseURL');
  const codexResponsesWebSocketURL =
    isCodexType && codexResponsesWebSocket ? getCodexResponsesWebSocketURL(watchedBaseURL, initialRow?.endpoints) : undefined;

  // Sync form type when provider or API format changes
  const handleProviderChange = useCallback(
    (provider: string) => {
      if (isOAuthChannel) return;
      if (isEdit && alwaysOAuthProviderKeys.includes(provider)) return;
      setSelectedProvider(provider);
      setAuthMode(isEdit && ['codex', 'claudecode'].includes(provider) ? 'third-party' : 'official');

      if (provider !== 'gemini') {
        setUseGeminiVertex(false);
      }
      if (provider !== 'anthropic') {
        setUseAnthropicAws(false);
      }
      if (provider !== 'moonshot') {
        setUseKimiCoding(false);
      }

      if (provider === 'codex') {
        setSelectedApiFormat(OPENAI_RESPONSES);
        form.setValue('type', 'codex');
        if (!isEdit) {
          const baseURL = getDefaultBaseURL('codex');
          if (baseURL) {
            form.setValue('baseURL', baseURL, { shouldDirty: true });
          }
        }
        return;
      }

      if (provider === 'xai_subscription') {
        setSelectedApiFormat(OPENAI_RESPONSES);
        setResponsesTransport('http');
        form.setValue('type', 'xai_subscription');
        if (!isEdit) {
          const baseURL = getDefaultBaseURL('xai_subscription');
          if (baseURL) {
            form.setValue('baseURL', baseURL, { shouldDirty: true });
          }
        }
        return;
      }

      if (provider === 'antigravity') {
        setSelectedApiFormat(GEMINI_CONTENTS);
        form.setValue('type', 'antigravity');
        if (!isEdit) {
          // Set default Base URL only if empty
          const baseURL = getDefaultBaseURL('antigravity');
          const currentURL = form.getValues('baseURL');
          if (baseURL && !isDuplicate && (!currentURL || currentURL === '')) {
            form.setValue('baseURL', baseURL);
          }
        }
        return;
      }

      const formats = getApiFormatsForProvider(provider);
      const currentFormat = selectedApiFormat;
      let newFormat = currentFormat;

      if (!formats.includes(currentFormat)) {
        newFormat = formats[0] || 'openai/chat_completions';
      }

      setSelectedApiFormat(newFormat);
      const newChannelType =
        provider === 'gemini' && newFormat === 'gemini/contents' && useGeminiVertex
          ? 'gemini_vertex'
          : provider === 'anthropic' && newFormat === 'anthropic/messages' && useAnthropicAws
            ? 'anthropic_aws'
            : provider === 'moonshot' && newFormat === 'anthropic/messages' && useKimiCoding
              ? 'moonshot_coding'
              : getChannelTypeForApiFormat(provider, newFormat);
      if (newChannelType) {
        form.setValue('type', newChannelType);
        if (!isEdit) {
          if (!isDuplicate) {
            const baseURL = getDefaultBaseURL(newChannelType);
            if (baseURL) {
              form.resetField('baseURL', { defaultValue: baseURL });
            }
          }
        }
      }
    },
    [form, useGeminiVertex, useAnthropicAws, useKimiCoding, isDuplicate, isEdit, selectedApiFormat, isOAuthChannel]
  );

  const handleApiFormatChange = useCallback(
    (formatOption: ApiFormatOption) => {
      if (isOAuthChannel) return;
      if (selectedProvider === 'codex' || selectedProvider === 'antigravity') return;

      const format = formatOption === OPENAI_RESPONSES_WEBSOCKET ? OPENAI_RESPONSES : formatOption;
      const nextResponsesTransport = formatOption === OPENAI_RESPONSES_WEBSOCKET ? 'websocket' : 'http';

      setSelectedApiFormat(format);
      setResponsesTransport(nextResponsesTransport);

      // Reset vertex checkbox if not gemini/contents
      if (format !== 'gemini/contents') {
        setUseGeminiVertex(false);
      }
      // Reset anthropic/kimi checkboxes if not anthropic/messages
      if (format !== 'anthropic/messages') {
        setUseAnthropicAws(false);
        setUseKimiCoding(false);
      }

      const channelTypeFromFormat = getChannelTypeForApiFormat(selectedProvider, format);
      const newChannelType =
        format === 'gemini/contents' && useGeminiVertex
          ? 'gemini_vertex'
          : format === 'anthropic/messages' && useAnthropicAws
            ? 'anthropic_aws'
            : format === 'anthropic/messages' && useKimiCoding
              ? 'moonshot_coding'
              : channelTypeFromFormat;
      if (newChannelType) {
        form.setValue('type', newChannelType);

        if (!isEdit) {
          const baseURLFieldState = form.getFieldState('baseURL', form.formState);
          if (nextResponsesTransport === 'websocket') {
            const baseURL = getResponsesWebSocketBaseURL(newChannelType);
            if (baseURL) {
              form.resetField('baseURL', { defaultValue: baseURL });
            }
          } else if (!baseURLFieldState.isDirty && !isDuplicate) {
            const baseURL = getDefaultBaseURL(newChannelType);
            if (baseURL) {
              form.resetField('baseURL', { defaultValue: baseURL });
            }
          }
        }
      }
    },
    [selectedProvider, form, useGeminiVertex, useAnthropicAws, useKimiCoding, isDuplicate, isEdit, isOAuthChannel]
  );

  const handleGeminiVertexChange = useCallback(
    (checked: boolean) => {
      if (isOAuthChannel) return;
      setUseGeminiVertex(checked);

      if (selectedApiFormat === 'gemini/contents') {
        const newChannelType = checked ? 'gemini_vertex' : 'gemini';
        form.setValue('type', newChannelType);

        // Switching into Vertex mode needs a row to edit: an empty list would
        // reject the submit with no visible field to fill in.
        if (checked && !(form.getValues('credentials.vertexApiKeys') ?? []).length) {
          form.setValue('credentials.vertexApiKeys', [emptyVertexAPIKey()]);
        }

        if (!isEdit) {
          const baseURLFieldState = form.getFieldState('baseURL', form.formState);
          if (!baseURLFieldState.isDirty && !isDuplicate) {
            const baseURL = getDefaultBaseURL(newChannelType);
            if (baseURL) {
              form.resetField('baseURL', { defaultValue: baseURL });
            }
          }
        }
      }
    },
    [selectedApiFormat, form, isDuplicate, isEdit, isOAuthChannel]
  );

  const handleAnthropicAwsChange = useCallback(
    (checked: boolean) => {
      if (isOAuthChannel) return;
      setUseAnthropicAws(checked);

      if (selectedApiFormat === 'anthropic/messages') {
        const newChannelType = checked ? 'anthropic_aws' : 'anthropic';
        form.setValue('type', newChannelType);

        if (!isEdit) {
          const baseURLFieldState = form.getFieldState('baseURL', form.formState);
          if (!baseURLFieldState.isDirty && !isDuplicate) {
            const baseURL = getDefaultBaseURL(newChannelType);
            if (baseURL) {
              form.resetField('baseURL', { defaultValue: baseURL });
            }
          }
        }
      }
    },
    [selectedApiFormat, form, isDuplicate, isEdit, isOAuthChannel]
  );

  const handleKimiCodingChange = useCallback(
    (checked: boolean) => {
      if (isOAuthChannel) return;
      setUseKimiCoding(checked);

      if (selectedApiFormat === 'anthropic/messages') {
        const newChannelType = checked ? 'moonshot_coding' : 'moonshot_anthropic';
        form.setValue('type', newChannelType);

        if (!isEdit) {
          const baseURLFieldState = form.getFieldState('baseURL', form.formState);
          if (!baseURLFieldState.isDirty && !isDuplicate) {
            const baseURL = getDefaultBaseURL(newChannelType);
            if (baseURL) {
              form.resetField('baseURL', { defaultValue: baseURL });
            }
          }
        }
      }
    },
    [selectedApiFormat, form, isDuplicate, isEdit, isOAuthChannel]
  );

  // The Command Code quota cookie only exists on Command Code channel types.
  // Reset the reveal state as soon as the active type leaves the two types.
  useEffect(() => {
    if (!isCommandCodeType) {
      setShowCommandCodeAuthCookie(false);
    }
    if (!isOllamaType) {
      setShowOllamaAuthCookie(false);
    }
  }, [isCommandCodeType, isOllamaType]);

  useEffect(() => {
    if (isEdit || isDuplicate) return;

    if (!isCodexType) {
      resetCodexOAuth();
    }
    if (selectedProvider !== 'claudecode') {
      resetClaudecodeOAuth();
    }
    if (selectedProvider !== 'antigravity') {
      resetAntigravityOAuth();
    }
    if (selectedProvider !== 'xai_subscription') {
      resetXaiOAuth();
      setXaiSSOToken('');
    }

    const providerToChannelType: Partial<Record<string, ChannelType>> = {
      claudecode: authMode === 'official' ? 'claudecode' : undefined,
      codex: authMode === 'official' || authMode === 'auth-json' ? 'codex' : undefined,
      antigravity: 'antigravity',
      xai_subscription: 'xai_subscription',
    };

    const channelTypeForURL: ChannelType | undefined = providerToChannelType[selectedProvider];

    if (channelTypeForURL) {
      const baseURL = getDefaultBaseURL(channelTypeForURL);
      if (baseURL) {
        // Use setValue instead of resetField to avoid infinite loop
        const currentURL = form.getValues('baseURL');
        if (!currentURL || currentURL !== baseURL) {
          form.setValue('baseURL', baseURL);
        }
      }
    }
  }, [
    isEdit,
    isDuplicate,
    isCodexType,
    selectedProvider,
    authMode,
    form,
    resetCodexOAuth,
    resetClaudecodeOAuth,
    resetAntigravityOAuth,
    resetXaiOAuth,
  ]);

  const watchedDefaultTestModel = form.watch('defaultTestModel');
  useEffect(() => {
    if (supportedModels.length === 0) return;
    if (!isEdit && !isDuplicate) {
      if (!watchedDefaultTestModel || !supportedModels.includes(watchedDefaultTestModel)) {
        form.setValue('defaultTestModel', supportedModels[0]);
      }
      return;
    }
    if (watchedDefaultTestModel && !supportedModels.includes(watchedDefaultTestModel)) {
      form.setValue('defaultTestModel', supportedModels[0]);
    }
  }, [supportedModels, watchedDefaultTestModel, isEdit, isDuplicate, form]);

  const renderOAuthSection = useCallback(
    (oauth: ReturnType<typeof useOAuthFlow>, description: string) => (
      <div className='mt-3 space-y-2'>
        <div className='rounded-md border p-3'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button type='button' variant='secondary' onClick={oauth.start} disabled={oauth.isStarting}>
              {oauth.isStarting ? t('channels.dialogs.oauth.buttons.starting') : t('channels.dialogs.oauth.buttons.startOAuth')}
            </Button>
            {oauth.authUrl && (
              <Button type='button' variant='ghost' onClick={() => window.open(oauth.authUrl || '', '_blank', 'noopener,noreferrer')}>
                {t('channels.dialogs.oauth.buttons.openOAuthLink')}
              </Button>
            )}
          </div>

          {oauth.authUrl && (
            <div className='mt-3 space-y-2'>
              <FormLabel className='text-sm font-medium'>{t('channels.dialogs.oauth.labels.authorizationUrl')}</FormLabel>
              <Textarea
                value={oauth.authUrl}
                readOnly
                className='min-h-[60px] resize-none font-mono text-xs'
                placeholder={t('channels.dialogs.oauth.placeholders.authorizationUrl')}
              />
            </div>
          )}

          <div className='mt-3 space-y-2'>
            <FormLabel className='text-sm font-medium'>{t('channels.dialogs.oauth.labels.callbackUrl')}</FormLabel>
            <Textarea
              value={oauth.callbackUrl}
              onChange={(e) => oauth.setCallbackUrl(e.target.value)}
              placeholder={t('channels.dialogs.oauth.placeholders.callbackUrl')}
              className='min-h-[80px] resize-y font-mono text-xs'
            />
            <Button type='button' onClick={oauth.exchange} disabled={oauth.isExchanging || !oauth.sessionId}>
              {oauth.isExchanging
                ? t('channels.dialogs.oauth.buttons.exchanging')
                : t('channels.dialogs.oauth.buttons.exchangeAndFillApiKey')}
            </Button>
          </div>

          <p className='mt-2 text-xs text-amber-600 dark:text-amber-400'>{t('channels.dialogs.proxy.oauthHint')}</p>
          <p className='text-muted-foreground mt-2 text-xs'>{description}</p>
        </div>
      </div>
    ),
    [t]
  );

  const applyXAISSO = useCallback(async () => {
    const token = xaiSSOToken.trim();
    if (!token) {
      toast.error(t('channels.dialogs.xaiSso.errors.required'));
      return;
    }

    setIsImportingXAISSO(true);
    try {
      const result = await xaiDecodeSSO({
        sso_token: token,
        ...(proxyConfig ? { proxy: proxyConfig } : {}),
      });
      form.setValue('credentials.apiKey', result.credentials, {
        shouldDirty: true,
        shouldTouch: true,
        shouldValidate: true,
      });
      setXaiSSOToken('');
      toast.success(t('channels.dialogs.xaiSso.messages.imported'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setIsImportingXAISSO(false);
    }
  }, [form, proxyConfig, t, xaiSSOToken]);

  const applyCodexAuthJSON = useCallback(async () => {
    try {
      const result = await codexDecodeAuthJSON({ auth_json: codexAuthJSONText });
      form.setValue('credentials.apiKey', result.credentials, {
        shouldDirty: true,
        shouldTouch: true,
        shouldValidate: true,
      });
      toast.success(t('channels.dialogs.codexAuthJson.messages.applied'));
    } catch {
      toast.error(t('channels.dialogs.codexAuthJson.messages.invalid'));
    }
  }, [codexAuthJSONText, form, t]);

  const onSubmit = async (values: ChannelFormValues) => {
    try {
      // Normalize grouped keys: trim values, drop fully-empty rows, and drop any
      // group that has no key. A non-empty group list is authoritative, so the
      // flat apiKeys list is cleared to keep the backend's uniqueness check happy
      // and to avoid routing the same key twice.
      const normalizedKeyGroups = (values.credentials?.keyGroups || [])
        .map((group) => ({
          name: (group.name || '').trim(),
          apiKeys: [...new Set((group.apiKeys || []).map((key) => (key || '').trim()).filter((key) => key.length > 0))],
          models: [...new Set((group.models || []).map((model) => model.trim()).filter((model) => model.length > 0))],
          enabled: group.enabled ?? true,
        }))
        .filter((group) => group.apiKeys.length > 0);

      if (values.credentials) {
        values.credentials.keyGroups = normalizedKeyGroups;
        values.credentials.apiKeys = [];
      }

      const retryableStatusCodes = parseRetryableStatusCodesInput(retryableStatusCodesText);
      if (retryableStatusCodes === null) {
        toast.error(t('channels.dialogs.retryableStatusCodes.validation'));
        return;
      }

      const retryableErrorPatterns = parseRetryableErrorPatternsInput(retryableErrorPatternsText);
      if (retryableErrorPatterns === null) {
        toast.error(t('channels.dialogs.retryableErrorPatterns.validation'));
        return;
      }

      const valuesForSubmit = isEdit
        ? values
        : {
            ...values,
            type: derivedChannelType,
          };

      // Vertex credentials are project-bound triples and are the only credential for
      // that channel type: a leftover singular apiKey would still be selected at
      // runtime and would not resolve to a project/region pair. Ordinary channels keep
      // their flat apiKeys and never carry the Vertex field.
      const submittedCredentials = valuesForSubmit.credentials;
      const vertexApiKeys = submittedCredentials?.vertexApiKeys;
      const credentials =
        valuesForSubmit.type === 'gemini_vertex'
          ? vertexApiKeys?.length
            ? {
                ...submittedCredentials,
                apiKey: undefined,
                apiKeys: undefined,
                vertexApiKeys: vertexApiKeys.map((credential) => ({
                  projectID: credential.projectID.trim(),
                  apiKey: credential.apiKey.trim(),
                  region: credential.region.trim() || 'global',
                })),
              }
            : { ...submittedCredentials, vertexApiKeys: undefined }
          : { ...submittedCredentials, vertexApiKeys: undefined };

      const dataWithModels = {
        ...valuesForSubmit,
        supportedModels,
        manualModels,
        credentials,
      };
      // The Command Code / Ollama quota cookie is a browser-session credential
      // that only belongs on its own channel type. Never let a
      // duplicate/type-switch flow attach it to an unrelated channel type.
      // Clearing it explicitly sends providerQuota: null so the backend
      // removes the stored cookie.
      const isCommandCodeSubmit = valuesForSubmit.type === 'commandcode' || valuesForSubmit.type === 'commandcode_anthropic';
      const commandCodeAuthCookie = isCommandCodeSubmit ? values.settings?.providerQuota?.commandCode?.authCookie?.trim() : undefined;
      const isOllamaSubmit = valuesForSubmit.type === 'ollama' || valuesForSubmit.type === 'ollama_anthropic';
      const ollamaAuthCookie = isOllamaSubmit ? values.settings?.providerQuota?.ollama?.authCookie?.trim() : undefined;
      const settingsForSubmit = values.settings
        ? {
            ...values.settings,
            ...((isCommandCodeSubmit && commandCodeAuthCookie) || (isOllamaSubmit && ollamaAuthCookie) ? {} : { providerQuota: null }),
          }
        : undefined;

      const shouldUseProtocolDefaultBaseURL =
        (isCodexType && (authMode === 'official' || authMode === 'auth-json')) ||
        (isClaudeCodeType && authMode === 'official' && !isDuplicate) ||
        isXAISubscriptionType;
      if (shouldUseProtocolDefaultBaseURL) {
        const currentType = selectedType || derivedChannelType;
        const baseURL = getDefaultBaseURL(currentType);
        if (baseURL) {
          dataWithModels.baseURL = baseURL;
        }
      }

      // Codex always saves an HTTP(S) Base URL; its WebSocket endpoint is derived from it.
      const requiredResponsesTransport: ResponsesTransport = isCodexType ? 'http' : responsesTransport;
      if (
        (isCodexType || selectedApiFormat === OPENAI_RESPONSES) &&
        !baseURLMatchesResponsesTransport(dataWithModels.baseURL, requiredResponsesTransport)
      ) {
        form.setError('baseURL', {
          type: 'manual',
          message: t(getResponsesTransportBaseURLError(requiredResponsesTransport)),
        });
        return;
      }

      if (isCodexType) {
        dataWithModels.endpoints = buildCodexResponsesEndpoints(initialRow?.endpoints, codexResponsesWebSocket);
      }
      const codexModelProtocols =
        isCodexType && initialRow ? translateLegacyCodexModelProtocols(initialRow, initialRow.settings?.modelProtocols) : undefined;

      if (isEdit && currentRow) {
        const existingModelProtocols = currentRow.settings?.modelProtocols;
        const shouldUpdateModelProtocols =
          selectedApiFormat === 'zenmux/video' ||
          existingModelProtocols?.some((protocol) => protocol.apiFormats.includes('zenmux/video')) === true;
        const settingsPatch: Partial<ChannelSettings> = {
          passThroughUserAgent,
          passThroughBody,
          retryableStatusCodes,
          retryableErrorPatterns,
          // Cookie edits (including clearing the saved cookie) travel through
          // the settings patch; mergeChannelSettingsForUpdate preserves the
          // field when the patch omits it and carries the null clear through.
          providerQuota: settingsForSubmit?.providerQuota,
          ...quotaRoutingModeSettingsPatch(quotaRoutingMode),
          ...(shouldUpdateModelProtocols
            ? { modelProtocols: getModelProtocolsForApiFormat(selectedApiFormat, supportedModels, existingModelProtocols) }
            : {}),
          ...(codexModelProtocols ? { modelProtocols: codexModelProtocols } : {}),
        };

        const updateInput = {
          ...dataWithModels,
          ...(isOAuthChannel ? { type: currentRow.type } : {}),
        } as z.infer<typeof updateChannelInputSchema>;
        delete updateInput.settings;

        const finalChannelType = updateInput.type || currentRow.type;
        const keepsManagementApiKey = ['zenmux', 'zenmux_responses', 'zenmux_anthropic', 'zenmux_gemini', 'zenmux_video'].includes(
          finalChannelType
        );
        if (!keepsManagementApiKey && updateInput.credentials) {
          delete updateInput.credentials.managementApiKey;
        }
        if (updateInput.credentials) {
          updateInput.credentials.apiKeys = [];
        }

        const apiKey = values.credentials?.apiKey || '';
        const hasApiKey = apiKey.trim().length > 0;
        const hasApiKeys = false;
        const hasManagementApiKey = (values.credentials?.managementApiKey || '').trim().length > 0;
        const hasGcpCredentials =
          values.credentials?.gcp?.region &&
          values.credentials.gcp.region.trim() !== '' &&
          values.credentials?.gcp?.projectID &&
          values.credentials.gcp.projectID.trim() !== '' &&
          values.credentials?.gcp?.jsonData &&
          values.credentials.gcp.jsonData.trim() !== '';
        const hasVertexApiKeys =
          values.credentials?.vertexApiKeys && values.credentials.vertexApiKeys.some((binding) => binding.apiKey.trim() !== '');
        const hasKeyGroups =
          values.credentials?.keyGroups &&
          values.credentials.keyGroups.some((group) => (group.apiKeys || []).some((key) => (key || '').trim() !== ''));

        if (!hasApiKey && !hasApiKeys && !hasKeyGroups && !hasManagementApiKey && !hasGcpCredentials && !hasVertexApiKeys) {
          delete updateInput.credentials;
        }

        await updateChannelSettings.mutateAsync({
          id: currentRow.id,
          input: updateInput,
          patch: settingsPatch,
        });
        toast.success(t('channels.messages.updateSuccess'));
      } else {
        const proxyConfig = {
          type: proxyType as 'disabled' | 'environment' | 'url',
          ...(proxyType === ProxyType.URL && {
            url: proxyUrl,
            username: proxyUsername || undefined,
            password: proxyPassword || undefined,
          }),
        };

        const nextSettings = mergeChannelSettingsForUpdate(settingsForSubmit, {
          proxy: proxyConfig,
          passThroughUserAgent,
          passThroughBody,
          retryableStatusCodes,
          retryableErrorPatterns,
          ...quotaRoutingModeSettingsPatch(quotaRoutingMode),
          ...(selectedApiFormat === 'zenmux/video' ||
          settingsForSubmit?.modelProtocols?.some((protocol) => protocol.apiFormats.includes('zenmux/video'))
            ? {
                modelProtocols: getModelProtocolsForApiFormat(selectedApiFormat, supportedModels, settingsForSubmit?.modelProtocols),
              }
            : {}),
          ...(codexModelProtocols ? { modelProtocols: codexModelProtocols } : {}),
        });

        const createInput = {
          ...(dataWithModels as unknown as z.infer<typeof createChannelInputSchema>),
          settings: nextSettings,
        } as z.infer<typeof createChannelInputSchema>;

        if (isDuplicate && duplicateFromRow) {
          await duplicateChannel.mutateAsync({
            sourceID: duplicateFromRow.id,
            input: createInput,
          });
        } else {
          await createChannel.mutateAsync(createInput);
        }

        // Auto-save proxy preset (preserve existing name if available)
        if (hasSystemScope('write_settings') && proxyType === ProxyType.URL && proxyUrl) {
          const existingPreset = proxyPresets.find((p) => p.url === proxyUrl);
          saveProxyPreset.mutate({
            name: existingPreset?.name,
            url: proxyUrl,
            username: proxyUsername || undefined,
            password: proxyPassword || undefined,
          });
        }
      }

      form.reset();
      setSupportedModels([]);
      setManualModels([]);
      onOpenChange(false);
    } catch (_error) {
      void _error;
    }
  };

  const addModel = () => {
    if (newModel.trim() && !supportedModels.includes(newModel.trim())) {
      setSupportedModels([...supportedModels, newModel.trim()]);
      setManualModels([...manualModels, newModel.trim()]);
      setNewModel('');
    }
  };

  const batchAddModels = useCallback(() => {
    const raw = newModel.trim();
    if (!raw) return;

    const models = raw
      .split(/[,，]+/)
      .map((m) => m.trim())
      .filter((m) => m.length > 0);

    if (models.length === 0) {
      setNewModel('');
      return;
    }

    setSupportedModels((prev) => {
      const combinedModels = new Set([...prev, ...models]);
      if (combinedModels.size === prev.length) return prev;
      return [...combinedModels];
    });
    setManualModels((prev) => {
      // Only add models that are NOT already in supportedModels
      const newModels = models.filter((m) => !supportedModels.includes(m));
      const combinedModels = new Set([...prev, ...newModels]);
      if (combinedModels.size === prev.length) return prev;
      return [...combinedModels];
    });
    setNewModel('');
  }, [newModel, supportedModels]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addModel();
    }
  };

  const openGroupModelsPanel = useCallback(
    (index: number) => {
      const own = form.getValues(`credentials.keyGroups.${index}.models`) || [];
      setActiveGroupModelIndex(index);
      // Only models already saved for this group are checked; fetched candidates
      // are available to select but must not be auto-selected.
      setGroupModelSelection([...own]);
      setGroupModelsSearch('');
      setShowGroupModelsPanel(true);
    },
    [form]
  );

  // Probe the upstream /models endpoint using one group's own key and write the
  // result into that group's model list only. Each row is independent, so two
  // groups of the same channel can carry different model sets.
  const handleFetchModelsForGroup = useCallback(
    async (index: number) => {
      const channelType = form.getValues('type');
      const baseURL = form.getValues('baseURL');
      const group = form.getValues(`credentials.keyGroups.${index}`);
      // Probe with the group's first key: every key of a group reaches the same
      // upstream, so the first one is representative.
      const apiKey = (group?.apiKeys || []).find((key) => (key || '').trim())?.trim() || '';

      if (!channelType || !baseURL) {
        return;
      }

      if (!apiKey) {
        toast.error(t('channels.dialogs.fields.keyGroup.apiKeyRequired'));
        return;
      }

      setFetchingGroupIndex(index);

      try {
        const result = await fetchModels.mutateAsync({
          channelType,
          baseURL,
          apiKey,
          channelID: isEdit ? currentRow?.id : undefined,
        });

        if (result.error) {
          toast.error(result.error);
          return;
        }

        const models = [...new Set(result.models.map((m) => m.id))];
        if (models.length === 0) {
          return;
        }

        // The fetch only refreshes the candidate list. Selection remains the
        // group's current model allowlist, so freshly fetched models are not
        // implicitly checked.
        const groupLabel = groupModelLabel(group, index, t);
        setUnifiedFetchedModels((prev) => [
          ...prev.filter((entry) => entry.groupIndex !== index),
          ...models.map((model) => ({ model, group: groupLabel, groupIndex: index })),
        ]);
        setModelSourceGroups((prev) => {
          const next = { ...prev };
          for (const model of models) {
            next[model] = [...new Set([...(next[model] || []), groupLabel])];
          }
          return next;
        });
        openGroupModelsPanel(index);
      } catch (_error) {
        // Error is already handled by the mutation
      } finally {
        setFetchingGroupIndex(null);
      }
    },
    [fetchModels, form, isEdit, currentRow, t, openGroupModelsPanel]
  );

  // Unified fetch: probe every group's own key once, merge the results into a
  // single selectable list, and remember which group each model came from. This
  // is the "统一获取全部" action — nothing is written to any group until the
  // operator picks the models and confirms.
  const handleFetchModelsForAllGroups = useCallback(async () => {
    const channelType = form.getValues('type');
    const baseURL = form.getValues('baseURL');
    const groups = form.getValues('credentials.keyGroups') || [];
    const usable = groups
      .map((group, index) => ({ group, index }))
      .filter((entry) => (entry.group?.apiKeys || []).some((key) => (key || '').trim().length > 0));

    if (!channelType || !baseURL) {
      return;
    }

    if (usable.length === 0) {
      toast.error(t('channels.dialogs.fields.keyGroup.noKeysToFetch'));
      return;
    }

    setFetchingAllGroups(true);

    const collected: { model: string; group: string; groupIndex: number }[] = [];
    const sources: Record<string, string[]> = {};
    let failed = 0;

    // Sequential so a failing upstream cannot drown the others in parallel errors.
    for (const { group, index } of usable) {
      const apiKey = (group.apiKeys || []).find((key) => (key || '').trim())?.trim() || '';

      try {
        const result = await fetchModels.mutateAsync({
          channelType,
          baseURL,
          apiKey,
          channelID: isEdit ? currentRow?.id : undefined,
        });

        if (result.error) {
          failed += 1;
          continue;
        }

        const groupLabel = groupModelLabel(group, index, t);
        for (const id of [...new Set(result.models.map((m) => m.id))]) {
          collected.push({ model: id, group: groupLabel, groupIndex: index });
          sources[id] = [...new Set([...(sources[id] || []), groupLabel])];
        }
      } catch {
        failed += 1;
      }
    }

    setFetchingAllGroups(false);

    if (collected.length === 0) {
      toast.error(t('channels.dialogs.fields.keyGroup.fetchAllEmpty'));
      return;
    }

    setUnifiedFetchedModels(collected);
    setModelSourceGroups((prev) => ({ ...prev, ...sources }));
    const selected = new Set<string>();
    for (const group of groups) {
      for (const model of group.models || []) {
        selected.add(model);
      }
    }
    setGroupModelSelection([...selected]);
    setGroupModelsSearch('');
    setActiveGroupModelIndex(null);
    setShowGroupModelsPanel(true);

    if (failed > 0) {
      toast.warning(t('channels.dialogs.fields.keyGroup.fetchAllPartial', { failed }));
    }
  }, [fetchModels, form, isEdit, currentRow, t]);

  const handleAddManualModelToAllGroups = useCallback(() => {
    const model = newModel.trim();
    if (!model) return;

    const groups = form.getValues('credentials.keyGroups') || [];
    if (groups.length === 0) {
      toast.error(t('channels.dialogs.fields.keyGroup.apiKeyRequired'));
      return;
    }

    groups.forEach((group, index) => {
      const models = group.models || [];
      if (!models.includes(model)) {
        form.setValue(`credentials.keyGroups.${index}.models`, [...models, model], {
          shouldDirty: true,
          shouldValidate: true,
        });
      }
    });

    setSupportedModels((prev) => (prev.includes(model) ? prev : [...prev, model]));
    setNewModel('');
  }, [form, newModel, t]);

  const handleSyncNow = useCallback(async () => {
    if (!currentRow) return [];
    if (patternError) {
      toast.error(patternError);
      return [];
    }

    const channelId = currentRow.id;
    const formPattern = form.getValues('autoSyncModelPattern') || '';
    const result = await syncChannelModels.mutateAsync({
      channelID: channelId,
      pattern: formPattern.trim() ? formPattern : undefined,
    });

    // Sync is authoritative for both lists; refreshing only supportedModels
    // leaves manualModels stale and makes the header count drift from the badges.
    setSupportedModels(result.supportedModels || []);
    setManualModels(result.manualModels || []);
    return result.supportedModels || [];
  }, [currentRow, form, patternError, syncChannelModels]);

  // ---- Group-aware model panel -------------------------------------------
  // There is no default key: every model operation starts from a named group.
  // "Fetch all" probes each group and tags every returned model with its source.

  const groupModelCandidates = useMemo(() => {
    if (activeGroupModelIndex === null) {
      return unifiedFetchedModels.map((entry) => entry.model);
    }

    const own = form.getValues(`credentials.keyGroups.${activeGroupModelIndex}.models`) || [];
    const fetchedForGroup = unifiedFetchedModels.filter((entry) => entry.groupIndex === activeGroupModelIndex).map((entry) => entry.model);
    return [...new Set([...fetchedForGroup, ...own])];
  }, [activeGroupModelIndex, unifiedFetchedModels, form]);

  const filteredGroupModelCandidates = useMemo(() => {
    const search = groupModelsSearch.trim().toLowerCase();
    if (!search) return groupModelCandidates;
    return groupModelCandidates.filter((model) => model.toLowerCase().includes(search));
  }, [groupModelCandidates, groupModelsSearch]);

  const closeGroupModelsPanel = useCallback(() => {
    setShowGroupModelsPanel(false);
    setActiveGroupModelIndex(null);
    setGroupModelSelection([]);
    setGroupModelsSearch('');
  }, []);

  // Auto-open the grouped model panel when the parent asks to show models.
  useEffect(() => {
    if (open && showModelsPanel && keyGroups && keyGroups.length > 0) {
      const firstGroupIndex = keyGroups.findIndex((group) => (group.apiKeys || []).some((key) => (key || '').trim().length > 0));
      if (firstGroupIndex >= 0) {
        openGroupModelsPanel(firstGroupIndex);
      }
    }
  }, [open, showModelsPanel, keyGroups, openGroupModelsPanel]);

  const toggleGroupModelSelection = useCallback((model: string) => {
    setGroupModelSelection((prev) => (prev.includes(model) ? prev.filter((item) => item !== model) : [...prev, model]));
  }, []);

  const selectAllGroupModels = useCallback(() => {
    setGroupModelSelection(filteredGroupModelCandidates);
  }, [filteredGroupModelCandidates]);

  const deselectAllGroupModels = useCallback(() => {
    setGroupModelSelection([]);
  }, []);

  const applyGroupModelSelection = useCallback(() => {
    if (activeGroupModelIndex !== null) {
      form.setValue(`credentials.keyGroups.${activeGroupModelIndex}.models`, [...groupModelSelection], {
        shouldDirty: true,
        shouldValidate: true,
      });
      setSupportedModels((prev) => [...new Set([...prev, ...groupModelSelection])]);
      closeGroupModelsPanel();
      return;
    }

    const byGroup = new Map<number, Set<string>>();
    for (const entry of unifiedFetchedModels) {
      if (!groupModelSelection.includes(entry.model)) continue;
      if (!byGroup.has(entry.groupIndex)) byGroup.set(entry.groupIndex, new Set());
      byGroup.get(entry.groupIndex)!.add(entry.model);
    }

    const nextSupported = new Set(supportedModels);
    for (const [index, models] of byGroup) {
      const current = form.getValues(`credentials.keyGroups.${index}.models`) || [];
      const merged = [...new Set([...current, ...models])];
      form.setValue(`credentials.keyGroups.${index}.models`, merged, {
        shouldDirty: true,
        shouldValidate: true,
      });
      models.forEach((model) => nextSupported.add(model));
    }

    setSupportedModels([...nextSupported]);
    closeGroupModelsPanel();
  }, [activeGroupModelIndex, closeGroupModelsPanel, form, groupModelSelection, supportedModels, unifiedFetchedModels]);

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(state) => {
          if (!state) {
            form.reset();
            setSupportedModels(initialRow?.supportedModels || []);
            setManualModels(initialRow?.manualModels || []);
            // Reset expandable panel states
            // Reset proxy state
            if (initialRow?.settings?.proxy?.type) {
              setProxyType(initialRow.settings.proxy.type as ProxyType);
            } else {
              setProxyType(ProxyType.ENVIRONMENT);
            }
            setProxyUrl(initialRow?.settings?.proxy?.url || '');
            setProxyUsername(initialRow?.settings?.proxy?.username || '');
            setProxyPassword(initialRow?.settings?.proxy?.password || '');
            setPassThroughUserAgent(initialRow?.settings?.passThroughUserAgent ?? null);
            setPassThroughBody(initialRow?.settings?.passThroughBody ?? null);
            setQuotaRoutingMode(recallQuotaRoutingMode(initialRow?.settings));
            setRetryableStatusCodesText(formatRetryableStatusCodes(initialRow?.settings?.retryableStatusCodes));
            setRetryableErrorPatternsText(formatRetryableErrorPatterns(initialRow?.settings?.retryableErrorPatterns));
            // Reset provider and API format state
            if (initialRow) {
              setSelectedProvider(getProviderFromChannelType(initialRow.type) || 'openai');
              setSelectedApiFormat(
                getInitialApiFormatForChannel(
                  initialRow.type,
                  CHANNEL_CONFIGS[initialRow.type as ChannelType]?.apiFormat || OPENAI_CHAT_COMPLETIONS,
                  initialRow.settings?.modelProtocols
                )
              );
              setResponsesTransport(getResponsesTransportFromChannel(initialRow));
              setUseGeminiVertex(initialRow.type === 'gemini_vertex');
              setUseAnthropicAws(initialRow.type === 'anthropic_aws');
              setUseKimiCoding(initialRow.type === 'moonshot_coding');
            } else {
              setSelectedProvider('openai');
              setSelectedApiFormat(OPENAI_CHAT_COMPLETIONS);
              setResponsesTransport('http');
              setUseGeminiVertex(false);
              setUseAnthropicAws(false);
              setUseKimiCoding(false);
            }
          }
          onOpenChange(state);
        }}
      >
        <DialogContent
          className={`flex max-h-[90vh] flex-col overflow-hidden transition-all duration-300 ${showGroupModelsPanel || showKeyPanel ? 'sm:max-w-6xl' : 'sm:max-w-4xl'}`}
        >
          <DialogHeader className='flex-shrink-0 text-left'>
            <DialogTitle>{isEdit ? t('channels.dialogs.edit.title') : t('channels.dialogs.create.title')}</DialogTitle>
            <DialogDescription>
              {isEdit ? t('channels.dialogs.edit.description') : t('channels.dialogs.create.description')}
            </DialogDescription>
          </DialogHeader>
          <div className='flex min-h-0 flex-1 overflow-hidden md:gap-4'>
            {/* Main Form Section */}
            <div
              className={`flex min-h-0 flex-1 flex-col overflow-hidden py-1 transition-all duration-300 ${showGroupModelsPanel || showKeyPanel ? 'pr-2' : 'pr-0'}`}
            >
              <Form {...form}>
                <form id='channel-form' onSubmit={form.handleSubmit(onSubmit)} className='flex min-h-0 flex-1 flex-col space-y-6 p-0.5'>
                  {/* Provider Selection - Left Side */}
                  <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-hidden md:flex-row md:gap-6'>
                    <div className='flex max-h-48 min-h-0 w-full flex-shrink-0 flex-col md:max-h-none md:w-60'>
                      <FormItem className='flex min-h-0 flex-1 flex-col space-y-2'>
                        <FormLabel className='text-base font-semibold'>{t('channels.dialogs.fields.provider.label')}</FormLabel>
                        <div
                          ref={setProviderListRef}
                          className={`flex-1 overflow-x-auto overflow-y-hidden pb-2 md:overflow-x-hidden md:overflow-y-auto md:pr-2 md:pb-0 ${isOAuthChannel ? 'cursor-not-allowed opacity-60' : ''}`}
                        >
                          <RadioGroup
                            value={selectedProvider}
                            onValueChange={handleProviderChange}
                            disabled={!!isOAuthChannel}
                            className='flex flex-row gap-2 md:flex-col md:space-y-2'
                          >
                            {availableProviders.map((provider) => {
                              const Icon = provider.icon;
                              const isSelected = provider.key === selectedProvider;
                              const isProviderDisabled =
                                isOAuthChannel || (isEdit && !isOAuthChannel && alwaysOAuthProviderKeys.includes(provider.key));
                              return (
                                <div
                                  key={provider.key}
                                  ref={(el) => {
                                    providerRefs.current[provider.key] = el;
                                  }}
                                  className={`flex shrink-0 items-center space-x-3 rounded-lg border p-3 transition-colors md:w-full ${
                                    isProviderDisabled
                                      ? isSelected
                                        ? 'border-primary bg-muted/80 cursor-not-allowed shadow-sm'
                                        : 'cursor-not-allowed opacity-60'
                                      : (isSelected ? 'border-primary bg-accent/40 shadow-sm' : '') + ' hover:bg-accent/50'
                                  }`}
                                >
                                  <RadioGroupItem
                                    value={provider.key}
                                    id={`provider-${provider.key}`}
                                    disabled={!!isProviderDisabled}
                                    data-testid={`provider-${provider.key}`}
                                  />
                                  {Icon && <Icon size={20} className='flex-shrink-0' />}
                                  <FormLabel
                                    htmlFor={`provider-${provider.key}`}
                                    className='flex-1 cursor-pointer font-normal whitespace-nowrap'
                                  >
                                    {provider.label}
                                  </FormLabel>
                                </div>
                              );
                            })}
                          </RadioGroup>
                        </div>
                      </FormItem>
                      {/* Hidden field to keep form type in sync */}
                      <FormField control={form.control} name='type' render={() => <input type='hidden' />} />
                    </div>

                    {/* Right Side - Form Fields */}
                    <div className='flex-1 space-y-6 overflow-y-auto md:pr-4'>
                      {selectedProvider !== 'jina' &&
                        selectedProvider !== 'codex' &&
                        selectedProvider !== 'claudecode' &&
                        selectedProvider !== 'typesafe' && (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.apiFormat.label')}
                            </FormLabel>
                            <div className='max-w-64 space-y-1 md:col-span-6 md:max-w-none'>
                              <SelectDropdown
                                key={selectedProvider}
                                defaultValue={selectedApiFormatOption}
                                onValueChange={(value) => handleApiFormatChange(value as ApiFormatOption)}
                                disabled={!!isOAuthChannel}
                                placeholder={t('channels.dialogs.fields.apiFormat.placeholder')}
                                data-testid='api-format-select'
                                isControlled={true}
                                items={availableApiFormatOptions.map((format) => ({
                                  value: format,
                                  label: getApiFormatOptionLabel(format),
                                }))}
                              />
                              {selectedApiFormat === 'gemini/contents' && (
                                <div className='mt-3'>
                                  <label
                                    className={`flex items-center gap-2 text-sm ${isOAuthChannel ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}
                                  >
                                    <Checkbox
                                      checked={useGeminiVertex}
                                      onCheckedChange={(checked) => handleGeminiVertexChange(checked === true)}
                                      disabled={!!isOAuthChannel}
                                    />
                                    <span>{t('channels.dialogs.fields.apiFormat.geminiVertex.label')}</span>
                                  </label>
                                </div>
                              )}
                              {selectedApiFormat === 'anthropic/messages' && selectedProvider === 'anthropic' && (
                                <div className='mt-3 space-y-2'>
                                  <label
                                    className={`flex items-center gap-2 text-sm ${isOAuthChannel ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}
                                  >
                                    <Checkbox
                                      checked={useAnthropicAws}
                                      onCheckedChange={(checked) => handleAnthropicAwsChange(checked === true)}
                                      disabled={!!isOAuthChannel}
                                    />
                                    <span>{t('channels.dialogs.fields.apiFormat.anthropicAWS.label')}</span>
                                  </label>
                                </div>
                              )}
                              {selectedApiFormat === 'anthropic/messages' && selectedProvider === 'moonshot' && (
                                <div className='mt-3 space-y-2'>
                                  <label
                                    className={`flex items-center gap-2 text-sm ${isOAuthChannel ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}
                                  >
                                    <Checkbox
                                      checked={useKimiCoding}
                                      onCheckedChange={(checked) => handleKimiCodingChange(checked === true)}
                                      disabled={!!isOAuthChannel}
                                    />
                                    <span>{t('channels.dialogs.fields.apiFormat.kimiCoding.label')}</span>
                                  </label>
                                </div>
                              )}
                            </div>
                          </FormItem>
                        )}
                      {selectedProvider === 'codex' && (
                        <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                            {t('channels.dialogs.fields.apiFormat.label')}
                          </FormLabel>
                          <div className='space-y-1 md:col-span-6'>
                            <div className='flex min-h-9 items-center justify-between gap-4'>
                              <span className='text-sm'>{getApiFormatLabel(OPENAI_RESPONSES)}</span>
                              <label className='flex cursor-pointer items-center gap-2 text-sm'>
                                <Checkbox
                                  checked={codexResponsesWebSocket}
                                  onCheckedChange={(checked) => setCodexResponsesWebSocket(checked === true)}
                                  data-testid='codex-responses-websocket-checkbox'
                                />
                                <span>{t('channels.dialogs.fields.apiFormat.codexWebSocket.label')}</span>
                              </label>
                            </div>
                            {codexResponsesWebSocketURL && (
                              <p className='text-muted-foreground text-xs break-all' data-testid='codex-responses-websocket-url'>
                                {t('channels.dialogs.fields.apiFormat.codexWebSocket.derivedURL', { url: codexResponsesWebSocketURL })}
                              </p>
                            )}
                            {codexResponsesWebSocket &&
                              (initialCodexResponses.webSocketOnly || hasCodexResponsesWebSocketOverrideReplacement(initialRow)) && (
                                <p
                                  className='text-xs text-amber-600 dark:text-amber-400'
                                  data-testid='codex-responses-websocket-upgrade-note'
                                >
                                  {t(
                                    initialCodexResponses.webSocketOnly
                                      ? 'channels.dialogs.fields.apiFormat.codexWebSocket.legacyUpgrade'
                                      : 'channels.dialogs.fields.apiFormat.codexWebSocket.overrideReplacement'
                                  )}
                                </p>
                              )}
                          </div>
                        </FormItem>
                      )}

                      {selectedProvider === 'claudecode' && (
                        <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                            {t('channels.dialogs.fields.apiFormat.label')}
                          </FormLabel>
                          <div className='space-y-1 md:col-span-6'>
                            <div className='text-sm'>{getApiFormatLabel(ANTHROPIC_MESSAGES)}</div>
                            <p className='text-muted-foreground mt-1 text-xs'>{t('channels.dialogs.fields.apiFormat.editDisabled')}</p>
                          </div>
                        </FormItem>
                      )}

                      {selectedProvider === 'antigravity' && (
                        <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                            {t('channels.dialogs.fields.apiFormat.label')}
                          </FormLabel>
                          <div className='space-y-1 md:col-span-6'>
                            <div className='text-sm'>{getApiFormatLabel(GEMINI_CONTENTS)}</div>
                            <p className='text-muted-foreground mt-1 text-xs'>{t('channels.dialogs.fields.apiFormat.editDisabled')}</p>

                            <div className='mt-3 space-y-2'>
                              <div className='rounded-md border p-3'>
                                <div className='flex flex-wrap items-center gap-2'>
                                  <Button
                                    type='button'
                                    variant='secondary'
                                    onClick={() => antigravityOAuth.start()}
                                    disabled={antigravityOAuth.isStarting}
                                  >
                                    {antigravityOAuth.isStarting
                                      ? t('channels.dialogs.antigravity.buttons.starting')
                                      : t('channels.dialogs.antigravity.buttons.startOAuth')}
                                  </Button>
                                  {antigravityOAuth.authUrl && (
                                    <Button
                                      type='button'
                                      variant='ghost'
                                      onClick={() => window.open(antigravityOAuth.authUrl || '', '_blank', 'noopener,noreferrer')}
                                    >
                                      {t('channels.dialogs.antigravity.buttons.openOAuthLink')}
                                    </Button>
                                  )}
                                </div>

                                {antigravityOAuth.authUrl && (
                                  <div className='mt-3 space-y-2'>
                                    <FormLabel className='text-sm font-medium'>
                                      {t('channels.dialogs.antigravity.labels.authorizationUrl')}
                                    </FormLabel>
                                    <Textarea
                                      value={antigravityOAuth.authUrl}
                                      readOnly
                                      className='min-h-[60px] resize-none font-mono text-xs'
                                      placeholder={t('channels.dialogs.antigravity.placeholders.authorizationUrl')}
                                    />
                                  </div>
                                )}

                                <div className='mt-3 space-y-2'>
                                  <FormLabel className='text-sm font-medium'>
                                    {t('channels.dialogs.antigravity.labels.callbackUrl')}
                                  </FormLabel>
                                  <Textarea
                                    value={antigravityOAuth.callbackUrl}
                                    onChange={(e) => antigravityOAuth.setCallbackUrl(e.target.value)}
                                    placeholder={t('channels.dialogs.antigravity.placeholders.callbackUrl')}
                                    className='min-h-[80px] resize-y font-mono text-xs'
                                  />
                                  <Button
                                    type='button'
                                    onClick={() => antigravityOAuth.exchange()}
                                    disabled={antigravityOAuth.isExchanging || !antigravityOAuth.sessionId}
                                  >
                                    {antigravityOAuth.isExchanging
                                      ? t('channels.dialogs.antigravity.buttons.exchanging')
                                      : t('channels.dialogs.antigravity.buttons.exchangeAndFillApiKey')}
                                  </Button>
                                </div>

                                <p className='mt-2 text-xs text-amber-600 dark:text-amber-400'>{t('channels.dialogs.proxy.oauthHint')}</p>
                                <p className='text-muted-foreground mt-2 text-xs'>
                                  {t('channels.dialogs.fields.apiFormat.antigravity.description')}
                                </p>
                              </div>
                            </div>
                          </div>
                        </FormItem>
                      )}

                      {isCopilotType && (
                        <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <div className='col-span-2' />
                          <div className='space-y-4 md:col-span-6'>
                            <CopilotDeviceFlow
                              existingCredentials={form.watch('credentials.apiKey')}
                              onSuccess={(token) => {
                                // Store as OAuth JSON format expected by backend
                                const oauthCredentials = JSON.stringify({
                                  access_token: token,
                                  token_type: 'bearer',
                                });
                                form.setValue('credentials.apiKey', oauthCredentials, { shouldDirty: true, shouldValidate: true });
                              }}
                              onError={(error) => {
                                toast.error(error);
                              }}
                            />
                          </div>
                        </div>
                      )}
                      <FormField
                        control={form.control}
                        name='name'
                        render={({ field, fieldState }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.name.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              <Input
                                placeholder={t('channels.dialogs.fields.name.placeholder')}
                                autoComplete='off'
                                aria-invalid={!!fieldState.error}
                                data-testid='channel-name-input'
                                {...field}
                              />
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />

                      {!isEdit && (
                        <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                            {t('channels.dialogs.proxy.fields.type.label')}
                          </FormLabel>
                          <div className='space-y-3 md:col-span-6'>
                            <Select value={proxyType} onValueChange={(value) => setProxyType(value as ProxyType)}>
                              <FormControl>
                                <SelectTrigger>
                                  <SelectValue placeholder={t('channels.dialogs.proxy.fields.type.placeholder')} />
                                </SelectTrigger>
                              </FormControl>
                              <SelectContent>
                                <SelectItem value={ProxyType.DISABLED}>{t('channels.dialogs.proxy.types.disabled')}</SelectItem>
                                <SelectItem value={ProxyType.ENVIRONMENT}>{t('channels.dialogs.proxy.types.environment')}</SelectItem>
                                <SelectItem value={ProxyType.URL}>{t('channels.dialogs.proxy.types.url')}</SelectItem>
                              </SelectContent>
                            </Select>

                            {proxyType === ProxyType.URL && proxyPresets.length > 0 && (
                              <div className='space-y-1'>
                                <FormLabel className='text-sm'>{t('channels.dialogs.proxy.presets.label')}</FormLabel>
                                <Select onValueChange={handleProxyPresetSelect}>
                                  <FormControl>
                                    <SelectTrigger>
                                      <SelectValue placeholder={t('channels.dialogs.proxy.presets.placeholder')} />
                                    </SelectTrigger>
                                  </FormControl>
                                  <SelectContent>
                                    {proxyPresets.map((preset) => (
                                      <SelectItem key={preset.url} value={preset.url}>
                                        {preset.name || preset.url}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                              </div>
                            )}

                            {proxyType === ProxyType.URL && (
                              <>
                                <div className='space-y-1'>
                                  <FormLabel className='text-sm'>{t('channels.dialogs.proxy.fields.url.label')}</FormLabel>
                                  <Input
                                    placeholder={t('channels.dialogs.proxy.fields.url.placeholder')}
                                    value={proxyUrl}
                                    onChange={(e) => setProxyUrl(e.target.value)}
                                  />
                                </div>

                                <div className='space-y-1'>
                                  <FormLabel className='text-sm'>{t('channels.dialogs.proxy.fields.username.label')}</FormLabel>
                                  <Input
                                    placeholder={t('channels.dialogs.proxy.fields.username.placeholder')}
                                    value={proxyUsername}
                                    onChange={(e) => setProxyUsername(e.target.value)}
                                  />
                                </div>

                                <div className='space-y-1'>
                                  <FormLabel className='text-sm'>{t('channels.dialogs.proxy.fields.password.label')}</FormLabel>
                                  <Input
                                    type='password'
                                    placeholder={t('channels.dialogs.proxy.fields.password.placeholder')}
                                    value={proxyPassword}
                                    onChange={(e) => setProxyPassword(e.target.value)}
                                  />
                                </div>
                              </>
                            )}
                          </div>
                        </FormItem>
                      )}

                      {(isCodexType || isClaudeCodeType) && (
                        <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <div className='col-span-2' />
                          <div className='space-y-4 md:col-span-6'>
                            <div className='space-y-3'>
                              <Tabs
                                value={authMode}
                                onValueChange={(value) => {
                                  const mode = value as 'official' | 'auth-json' | 'third-party';
                                  // Codex official and auth.json always use the canonical HTTPS Base URL;
                                  // returning to third-party restores the URL entered there.
                                  if (isCodexType && authMode === 'third-party') {
                                    codexThirdPartyBaseURLRef.current = form.getValues('baseURL') ?? '';
                                  }
                                  setAuthMode(mode);
                                  if (mode !== 'third-party') {
                                    const defaultURL = getDefaultBaseURL(selectedType || derivedChannelType);
                                    if (defaultURL) {
                                      form.setValue('baseURL', defaultURL);
                                    }
                                  } else if (isCodexType && codexThirdPartyBaseURLRef.current) {
                                    form.setValue('baseURL', codexThirdPartyBaseURLRef.current);
                                  }
                                }}
                              >
                                <TabsList className={`grid w-full ${isCodexType ? 'grid-cols-3' : 'grid-cols-2'}`}>
                                  <TabsTrigger value='official'>{t('channels.dialogs.authMode.official')}</TabsTrigger>
                                  {isCodexType && <TabsTrigger value='auth-json'>{t('channels.dialogs.authMode.authJson')}</TabsTrigger>}
                                  <TabsTrigger value='third-party'>{t('channels.dialogs.authMode.thirdParty')}</TabsTrigger>
                                </TabsList>
                              </Tabs>

                              {isCodexType && authMode === 'auth-json' && (
                                <div className='rounded-md border p-3'>
                                  <div className='space-y-2'>
                                    <FormLabel className='text-sm font-medium'>{t('channels.dialogs.codexAuthJson.label')}</FormLabel>
                                    <Textarea
                                      value={codexAuthJSONText}
                                      onChange={(e) => setCodexAuthJSONText(e.target.value)}
                                      placeholder={t('channels.dialogs.codexAuthJson.placeholder')}
                                      className='min-h-[160px] resize-y font-mono text-xs'
                                      data-testid='codex-auth-json-input'
                                    />
                                    <Button
                                      type='button'
                                      variant='secondary'
                                      onClick={applyCodexAuthJSON}
                                      data-testid='codex-auth-json-apply'
                                    >
                                      {t('channels.dialogs.codexAuthJson.applyButton')}
                                    </Button>
                                    <p className='text-muted-foreground text-xs'>{t('channels.dialogs.codexAuthJson.description')}</p>
                                  </div>
                                </div>
                              )}
                            </div>

                            {isCodexType && (
                              <div className='space-y-2'>
                                {authMode === 'official' &&
                                  renderOAuthSection(codexOAuth, t('channels.dialogs.fields.apiFormat.codex.description'))}
                              </div>
                            )}

                            {isClaudeCodeType && (
                              <div className='space-y-2'>
                                {authMode === 'official' &&
                                  renderOAuthSection(claudecodeOAuth, t('channels.dialogs.fields.apiFormat.claudecode.description'))}
                              </div>
                            )}
                          </div>
                        </div>
                      )}

                      {isXAISubscriptionType && (
                        <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <div className='col-span-2' />
                          <div className='space-y-4 md:col-span-6'>
                            {renderOAuthSection(xaiOAuth, t('channels.dialogs.fields.apiFormat.xaiSubscription.description'))}
                            <div className='rounded-md border p-3'>
                              <div className='space-y-2'>
                                <FormLabel htmlFor='xai-sso-token' className='text-sm font-medium'>
                                  {t('channels.dialogs.xaiSso.label')}
                                </FormLabel>
                                <Textarea
                                  id='xai-sso-token'
                                  value={xaiSSOToken}
                                  onChange={(event) => setXaiSSOToken(event.target.value)}
                                  spellCheck={false}
                                  autoComplete='off'
                                  placeholder={t('channels.dialogs.xaiSso.placeholder')}
                                  className='min-h-[96px] resize-y font-mono text-xs'
                                />
                                <Button
                                  type='button'
                                  variant='secondary'
                                  onClick={applyXAISSO}
                                  disabled={isImportingXAISSO || !xaiSSOToken.trim()}
                                >
                                  {isImportingXAISSO
                                    ? t('channels.dialogs.xaiSso.buttons.importing')
                                    : t('channels.dialogs.xaiSso.buttons.import')}
                                </Button>
                                <p className='text-muted-foreground text-xs'>{t('channels.dialogs.xaiSso.description')}</p>
                              </div>
                            </div>
                          </div>
                        </div>
                      )}

                      <FormField
                        control={form.control}
                        name='baseURL'
                        render={({ field, fieldState }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.baseURL.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              <Input
                                placeholder={baseURLPlaceholder}
                                autoComplete='new-password'
                                data-form-type='other'
                                aria-invalid={!!fieldState.error}
                                data-testid='channel-base-url-input'
                                disabled={
                                  (isCodexType && authMode !== 'third-party') ||
                                  (isClaudeCodeType && authMode === 'official') ||
                                  isXAISubscriptionType ||
                                  selectedProvider === 'antigravity'
                                }
                                {...field}
                              />
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />

                      {isGeminiVertex && (
                        <FormField
                          control={form.control}
                          name='credentials.vertexApiKeys'
                          render={({ field }) => {
                            const credentials = field.value || [];
                            return (
                              <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                                <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                                  {t('channels.dialogs.fields.vertexApiKey.label')}
                                </FormLabel>
                                <div className='space-y-3 md:col-span-6'>
                                  {credentials.map((_, index) => (
                                    <div className='grid gap-3 rounded-md border p-3' key={vertexApiKeyFields.fields[index]?.id ?? index}>
                                      <div className='grid gap-3 sm:grid-cols-[minmax(0,1fr)_11rem_auto] sm:items-end'>
                                        <FormField
                                          control={form.control}
                                          name={`credentials.vertexApiKeys.${index}.projectID`}
                                          render={({ field: rowField }) => (
                                            <FormItem className='grid gap-1.5'>
                                              <FormLabel className='text-muted-foreground text-xs font-medium'>
                                                {t('channels.dialogs.fields.vertexApiKey.projectIDLabel')}
                                              </FormLabel>
                                              <FormControl>
                                                <Input
                                                  value={rowField.value}
                                                  onChange={(event) => rowField.onChange(event.target.value)}
                                                  placeholder={t('channels.dialogs.fields.vertexApiKey.projectIDPlaceholder')}
                                                />
                                              </FormControl>
                                              <FormMessage />
                                            </FormItem>
                                          )}
                                        />
                                        <FormField
                                          control={form.control}
                                          name={`credentials.vertexApiKeys.${index}.region`}
                                          render={({ field: rowField }) => (
                                            <FormItem className='grid gap-1.5'>
                                              <FormLabel className='text-muted-foreground text-xs font-medium'>
                                                {t('channels.dialogs.fields.vertexApiKey.regionLabel')}
                                              </FormLabel>
                                              <FormControl>
                                                <Input
                                                  value={rowField.value}
                                                  onChange={(event) => rowField.onChange(event.target.value)}
                                                  placeholder='global'
                                                />
                                              </FormControl>
                                              <FormMessage />
                                            </FormItem>
                                          )}
                                        />
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='icon'
                                          onClick={() => vertexApiKeyFields.remove(index)}
                                          disabled={credentials.length <= 1}
                                          aria-label={t('channels.dialogs.fields.vertexApiKey.remove')}
                                        >
                                          <Trash2 className='h-4 w-4' />
                                        </Button>
                                      </div>
                                      <FormField
                                        control={form.control}
                                        name={`credentials.vertexApiKeys.${index}.apiKey`}
                                        render={({ field: rowField }) => (
                                          <FormItem className='grid gap-1.5'>
                                            <FormLabel className='text-muted-foreground text-xs font-medium'>
                                              {t('channels.dialogs.fields.vertexApiKey.apiKeyLabel')}
                                            </FormLabel>
                                            <FormControl>
                                              <Input
                                                type='password'
                                                value={rowField.value}
                                                onChange={(event) => rowField.onChange(event.target.value)}
                                                placeholder={t('channels.dialogs.fields.vertexApiKey.apiKeyPlaceholder')}
                                              />
                                            </FormControl>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    </div>
                                  ))}
                                  <Button
                                    type='button'
                                    variant='secondary'
                                    onClick={() => vertexApiKeyFields.append({ projectID: '', apiKey: '', region: 'global' })}
                                  >
                                    <Plus className='mr-2 h-4 w-4' />
                                    {t('channels.dialogs.fields.vertexApiKey.add')}
                                  </Button>
                                  <FormDescription>{t('channels.dialogs.fields.vertexApiKey.description')}</FormDescription>
                                  <FormMessage />
                                </div>
                              </FormItem>
                            );
                          }}
                        />
                      )}

                      {!isGeminiVertex &&
                        (!(isCodexType || isClaudeCodeType || isCopilotType || isXAISubscriptionType) || authMode === 'third-party') &&
                        selectedProvider !== 'antigravity' &&
                        selectedType !== 'anthropic_gcp' && (
                          <div className='grid grid-cols-1 items-start gap-x-6 gap-y-3 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.keyGroup.label')}
                            </FormLabel>
                            <div className='space-y-3 md:col-span-6'>
                              <FormDescription>{t('channels.dialogs.fields.keyGroup.hint')}</FormDescription>
                              <KeyGroupsEditor
                                rows={keyGroups || []}
                                onAdd={() => {
                                  const current: KeyGroupFormRow[] = form.getValues('credentials.keyGroups') || [];
                                  form.setValue('credentials.keyGroups' as any, [...current, createEmptyKeyGroup(current.length + 1)], {
                                    shouldDirty: true,
                                    shouldValidate: true,
                                  });
                                }}
                                onRemove={(index) => {
                                  const current = form.getValues('credentials.keyGroups') || [];
                                  form.setValue(
                                    'credentials.keyGroups',
                                    current.filter((_, i) => i !== index),
                                    { shouldDirty: true, shouldValidate: true }
                                  );
                                }}
                                onChangeName={(index, name) => {
                                  const current = [...(form.getValues('credentials.keyGroups') || [])];
                                  current[index] = { ...current[index], name };
                                  // Typing must stay cheap: the channel schema validates
                                  // the whole form, so validation waits for blur/submit.
                                  form.setValue('credentials.keyGroups', current, { shouldDirty: true, shouldValidate: false });
                                }}
                                onChangeKeys={(index, keys, options) => {
                                  const current = [...(form.getValues('credentials.keyGroups') || [])];
                                  current[index] = { ...current[index], apiKeys: keys };
                                  form.setValue('credentials.keyGroups', current, {
                                    shouldDirty: true,
                                    shouldValidate: options?.validate === true,
                                  });
                                }}
                                onViewKeys={(index) => {
                                  // Toggle: clicking the same group again closes the
                                  // panel instead of keeping it stuck open.
                                  if (showKeyPanel && activeKeyGroupIndex === index) {
                                    setShowKeyPanel(false);
                                    setActiveKeyGroupIndex(null);
                                    return;
                                  }
                                  setActiveKeyGroupIndex(index);
                                  setShowKeyPanel(true);
                                  setShowGroupModelsPanel(false);
                                }}
                              />
                              {(form.formState.errors.credentials as any)?.keyGroups?.message && (
                                <p className='text-destructive text-sm'>{(form.formState.errors.credentials as any).keyGroups.message}</p>
                              )}
                              {(keyGroups || []).map((_, index) => (
                                <div key={index} className='space-y-1'>
                                  {(form.formState.errors.credentials as any)?.keyGroups?.[index]?.name?.message && (
                                    <p className='text-destructive text-xs'>
                                      {(form.formState.errors.credentials as any).keyGroups[index].name.message}
                                    </p>
                                  )}
                                  {(form.formState.errors.credentials as any)?.keyGroups?.[index]?.apiKeys?.message && (
                                    <p className='text-destructive text-xs'>
                                      {(form.formState.errors.credentials as any).keyGroups[index].apiKeys.message}
                                    </p>
                                  )}
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                      {isZenmuxType && (
                        <FormField
                          control={form.control}
                          name='credentials.managementApiKey'
                          render={({ field }) => (
                            <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                              <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                                {t('channels.dialogs.fields.managementApiKey.label')}
                              </FormLabel>
                              <div className='space-y-1 md:col-span-6'>
                                <Input
                                  type='password'
                                  placeholder={t('channels.dialogs.fields.managementApiKey.placeholder')}
                                  autoComplete='new-password'
                                  data-form-type='other'
                                  spellCheck={false}
                                  {...field}
                                  value={field.value ?? ''}
                                />
                                <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.managementApiKey.hint')}</p>
                              </div>
                            </FormItem>
                          )}
                        />
                      )}

                      {isCommandCodeType && (
                        <FormField
                          control={form.control}
                          name='settings.providerQuota.commandCode.authCookie'
                          render={({ field, fieldState }) => (
                            <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                              <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                                {t('channels.dialogs.fields.commandCodeQuota.authCookie.label')}
                              </FormLabel>
                              <div className='space-y-1 md:col-span-6'>
                                <div className='relative'>
                                  <Input
                                    type={showCommandCodeAuthCookie ? 'text' : 'password'}
                                    value={field.value ?? ''}
                                    onChange={field.onChange}
                                    onBlur={field.onBlur}
                                    placeholder={t('channels.dialogs.fields.commandCodeQuota.authCookie.placeholder')}
                                    autoComplete='new-password'
                                    data-form-type='other'
                                    spellCheck={false}
                                    aria-invalid={!!fieldState.error}
                                    data-testid='channel-commandcode-auth-cookie-input'
                                    className='pr-10 font-mono text-xs'
                                  />
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='sm'
                                    className='absolute top-0 right-0 h-full px-3'
                                    onClick={() => setShowCommandCodeAuthCookie((visible) => !visible)}
                                  >
                                    {showCommandCodeAuthCookie ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                                  </Button>
                                </div>
                                <FormDescription className='text-xs'>
                                  {t('channels.dialogs.fields.commandCodeQuota.authCookie.description')}
                                </FormDescription>
                                <FormMessage />
                              </div>
                            </FormItem>
                          )}
                        />
                      )}

                      {isOllamaType && (
                        <FormField
                          control={form.control}
                          name='settings.providerQuota.ollama.authCookie'
                          render={({ field, fieldState }) => (
                            <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                              <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                                {t('channels.dialogs.fields.ollamaQuota.authCookie.label')}
                              </FormLabel>
                              <div className='space-y-1 md:col-span-6'>
                                <div className='relative'>
                                  <Input
                                    type={showOllamaAuthCookie ? 'text' : 'password'}
                                    value={field.value ?? ''}
                                    onChange={field.onChange}
                                    onBlur={field.onBlur}
                                    placeholder={t('channels.dialogs.fields.ollamaQuota.authCookie.placeholder')}
                                    autoComplete='new-password'
                                    data-form-type='other'
                                    spellCheck={false}
                                    aria-invalid={!!fieldState.error}
                                    data-testid='channel-ollama-auth-cookie-input'
                                    className='pr-10 font-mono text-xs'
                                  />
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='sm'
                                    className='absolute top-0 right-0 h-full px-3'
                                    onClick={() => setShowOllamaAuthCookie((visible) => !visible)}
                                  >
                                    {showOllamaAuthCookie ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                                  </Button>
                                </div>
                                <FormDescription className='text-xs'>
                                  {t('channels.dialogs.fields.ollamaQuota.authCookie.description')}
                                </FormDescription>
                                <FormMessage />
                              </div>
                            </FormItem>
                          )}
                        />
                      )}

                      <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                        <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                          {t('channels.dialogs.fields.quotaRoutingMode.label')}
                        </FormLabel>
                        <div className='space-y-1 md:col-span-6'>
                          <Select value={quotaRoutingMode} onValueChange={(value) => setQuotaRoutingMode(value as ChannelQuotaRoutingMode)}>
                            <SelectTrigger>
                              <SelectValue placeholder={t('channels.dialogs.fields.quotaRoutingMode.options.INHERIT')} />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='INHERIT'>
                                {quotaRoutingSettings?.defaultMode
                                  ? t('channels.dialogs.fields.quotaRoutingMode.options.INHERIT_WITH_MODE', {
                                      mode: t(`channels.dialogs.fields.quotaRoutingMode.options.${quotaRoutingSettings.defaultMode}`),
                                    })
                                  : t('channels.dialogs.fields.quotaRoutingMode.options.INHERIT')}
                              </SelectItem>
                              <SelectItem value='IGNORE_QUOTA'>
                                {t('channels.dialogs.fields.quotaRoutingMode.options.IGNORE_QUOTA')}
                              </SelectItem>
                              <SelectItem value='REMOVE_ON_EXHAUSTED'>
                                {t('channels.dialogs.fields.quotaRoutingMode.options.REMOVE_ON_EXHAUSTED')}
                              </SelectItem>
                              <SelectItem value='BACKPRESSURE'>
                                {t('channels.dialogs.fields.quotaRoutingMode.options.BACKPRESSURE')}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                          <FormDescription className='text-xs'>
                            {t(`channels.dialogs.fields.quotaRoutingMode.descriptions.${quotaRoutingMode}`)}
                          </FormDescription>
                        </div>
                      </FormItem>

                      <FormField
                        control={form.control}
                        name='policies.stream'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.streamPolicy.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              <SelectDropdown
                                defaultValue={(field.value as string) || 'unlimited'}
                                onValueChange={(value) => field.onChange(value)}
                                placeholder={t('channels.dialogs.fields.streamPolicy.placeholder')}
                                data-testid='channel-stream-policy-select'
                                isControlled={true}
                                items={[
                                  { value: 'unlimited', label: t('channels.dialogs.fields.streamPolicy.options.unlimited') },
                                  { value: 'require', label: t('channels.dialogs.fields.streamPolicy.options.require') },
                                  { value: 'forbid', label: t('channels.dialogs.fields.streamPolicy.options.forbid') },
                                ]}
                              />
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />

                      <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                        <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                          {t('channels.dialogs.fields.supportedModels.label')}
                        </FormLabel>
                        <div className='space-y-2 md:col-span-6'>
                          <div className='flex gap-2'>
                            <Input
                              placeholder={t('channels.dialogs.fields.supportedModels.description')}
                              value={newModel}
                              onChange={(e) => setNewModel(e.target.value)}
                              onKeyDown={handleKeyDown}
                              className='flex-1'
                            />
                            <Button type='button' onClick={() => handleAddManualModelToAllGroups()} size='sm'>
                              {t('channels.dialogs.buttons.add')}
                            </Button>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button type='button' onClick={batchAddModels} size='sm' variant='outline'>
                                  {t('channels.dialogs.buttons.batchAdd')}
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>
                                <p>{t('channels.dialogs.buttons.batchAddTooltip')}</p>
                              </TooltipContent>
                            </Tooltip>
                          </div>

                          {supportedModels.length === 0 && (
                            <p className='text-destructive text-sm'>{t('channels.dialogs.fields.supportedModels.required')}</p>
                          )}

                          {/* Auto sync checkbox */}
                          <div className='pt-3'>
                            <FormField
                              control={form.control}
                              name='autoSyncSupportedModels'
                              render={({ field }) => (
                                <FormItem
                                  className={`flex items-center gap-2 ${isCodexType || isClaudeCodeType || isCopilotType ? 'opacity-60' : ''}`}
                                >
                                  {wrapUnsupported(
                                    isCodexType || isClaudeCodeType || isCopilotType,
                                    <Checkbox
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                      data-testid='auto-sync-supported-models-checkbox'
                                      disabled={isCodexType || isClaudeCodeType || isCopilotType}
                                      className={isCodexType || isClaudeCodeType || isCopilotType ? 'pointer-events-none' : undefined}
                                    />,
                                    'inline-flex items-center'
                                  )}
                                  <div className='flex flex-1 items-center justify-between'>
                                    <div className='space-y-0.5'>
                                      <div className='flex items-center gap-1.5'>
                                        <FormLabel className='cursor-pointer text-sm font-normal'>
                                          {t('channels.dialogs.fields.autoSyncSupportedModels.label')}
                                        </FormLabel>
                                        <Tooltip>
                                          <TooltipTrigger asChild>
                                            <button
                                              type='button'
                                              className='text-muted-foreground hover:text-foreground inline-flex items-center'
                                              aria-label={t('channels.dialogs.fields.autoSyncSupportedModels.description')}
                                              data-testid='auto-sync-supported-models-tip'
                                            >
                                              <Info className='h-3.5 w-3.5' />
                                            </button>
                                          </TooltipTrigger>
                                          <TooltipContent>
                                            <p>{t('channels.dialogs.fields.autoSyncSupportedModels.description')}</p>
                                          </TooltipContent>
                                        </Tooltip>
                                      </div>
                                    </div>
                                    {isEdit && field.value && (
                                      <Button
                                        type='button'
                                        size='sm'
                                        variant='outline'
                                        onClick={handleSyncNow}
                                        disabled={syncChannelModels.isPending || updateChannelSettings.isPending}
                                      >
                                        <Play className={`mr-1 h-3 w-3 ${syncChannelModels.isPending ? 'animate-spin' : ''}`} />
                                        {syncChannelModels.isPending
                                          ? t('channels.dialogs.buttons.syncingNow')
                                          : t('channels.dialogs.buttons.syncNow')}
                                      </Button>
                                    )}
                                  </div>
                                </FormItem>
                              )}
                            />

                            {/* Auto sync model pattern */}
                            {form.watch('autoSyncSupportedModels') && (
                              <FormField
                                control={form.control}
                                name='autoSyncModelPattern'
                                render={({ field }) => (
                                  <FormItem className='mt-2 pl-6'>
                                    <FormLabel className='text-sm font-normal'>
                                      {t('channels.dialogs.fields.autoSyncModelPattern.label')}
                                    </FormLabel>
                                    <FormControl>
                                      <Input
                                        placeholder={t('channels.dialogs.fields.autoSyncModelPattern.placeholder')}
                                        {...field}
                                        value={field.value || ''}
                                        onChange={(e) => {
                                          const val = e.target.value;
                                          field.onChange(val);
                                          if (val === '') {
                                            setPatternError(null);
                                          } else if (isValidModelPattern(val)) {
                                            setPatternError(null);
                                          } else {
                                            setPatternError(t('channels.dialogs.fields.autoSyncModelPattern.invalid'));
                                          }
                                        }}
                                        className='font-mono text-sm'
                                      />
                                    </FormControl>
                                    <p className='text-muted-foreground text-xs'>
                                      {t('channels.dialogs.fields.autoSyncModelPattern.description')}
                                    </p>
                                    {patternError && <p className='text-destructive text-xs'>{patternError}</p>}
                                  </FormItem>
                                )}
                              />
                            )}
                          </div>

                          {/* Per-group model fetching and selection. */}
                          {/* 按分组获取/选取模型：每一行是一个分组备注，可单独拉取该分组
                              Key 的模型并只写回该分组；"全部"则逐个分组拉取后统一勾选。
                              分组在此只是标签，获取模型不再散落在 Key 输入框旁。 */}
                          {(keyGroups || []).some((group) => (group.apiKeys || []).some((key) => (key || '').trim().length > 0)) && (
                            <div className='mt-3 rounded-md border p-3' data-testid='supported-models-by-group'>
                              <div className='mb-2 flex items-center justify-between gap-2'>
                                <span className='text-sm font-medium'>{t('channels.dialogs.fields.supportedModels.byGroupLabel')}</span>
                                <Button
                                  type='button'
                                  variant='outline'
                                  size='sm'
                                  className='h-7 shrink-0 px-2 text-xs'
                                  onClick={handleFetchModelsForAllGroups}
                                  disabled={fetchingAllGroups || fetchModels.isPending}
                                  data-testid='supported-models-fetch-all-groups'
                                >
                                  <RefreshCw className={`mr-1 h-3.5 w-3.5 ${fetchingAllGroups ? 'animate-spin' : ''}`} />
                                  {fetchingAllGroups
                                    ? t('channels.dialogs.fields.keyGroup.fetchingModels')
                                    : t('channels.dialogs.fields.keyGroup.fetchAllGroupsModels')}
                                </Button>
                              </div>
                              <div className='space-y-2'>
                                {(keyGroups || []).map((group, index) => {
                                  const groupModels = group.models || [];
                                  const label = groupModelLabel(group, index, t);
                                  return (
                                    <div
                                      key={index}
                                      className='flex flex-wrap items-center gap-2 rounded-md border border-dashed px-2 py-1.5'
                                      data-testid={`supported-models-group-${index}`}
                                    >
                                      <Badge variant='outline' className='h-5 shrink-0 px-1.5 text-[10px] font-normal'>
                                        {label}
                                      </Badge>
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='sm'
                                        className='h-6 shrink-0 px-2 text-xs'
                                        onClick={() => handleFetchModelsForGroup(index)}
                                        disabled={fetchingGroupIndex === index || fetchModels.isPending}
                                      >
                                        <RefreshCw className={`mr-1 h-3.5 w-3.5 ${fetchingGroupIndex === index ? 'animate-spin' : ''}`} />
                                        {fetchingGroupIndex === index
                                          ? t('channels.dialogs.fields.keyGroup.fetchingModels')
                                          : t('channels.dialogs.fields.keyGroup.fetchModels')}
                                      </Button>
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='sm'
                                        className='text-muted-foreground h-6 shrink-0 px-2 text-xs'
                                        onClick={() => openGroupModelsPanel(index)}
                                      >
                                        {t('channels.dialogs.fields.keyGroup.editModels')}
                                      </Button>
                                      <span className='text-muted-foreground text-xs'>
                                        {groupModels.length > 0
                                          ? t('channels.dialogs.fields.keyGroup.modelsBadge', { count: groupModels.length })
                                          : t('channels.dialogs.fields.keyGroup.noModelsShort')}
                                      </span>
                                      {groupModels.slice(0, 3).map((model) => (
                                        <Badge key={model} variant='secondary' className='text-[11px]'>
                                          {model}
                                        </Badge>
                                      ))}
                                      {groupModels.length > 3 && (
                                        <Badge variant='outline' className='text-[11px] font-normal'>
                                          {t('channels.dialogs.fields.keyGroup.moreModels', { count: groupModels.length - 3 })}
                                        </Badge>
                                      )}
                                    </div>
                                  );
                                })}
                              </div>
                            </div>
                          )}
                        </div>
                      </div>

                      <FormField
                        control={form.control}
                        name='defaultTestModel'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.defaultTestModel.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              <SelectDropdown
                                defaultValue={field.value}
                                onValueChange={field.onChange}
                                items={supportedModels.map((model) => ({ value: model, label: model }))}
                                placeholder={t('channels.dialogs.fields.defaultTestModel.description')}
                                className='md:col-span-6'
                                disabled={supportedModels.length === 0}
                                isControlled={true}
                                data-testid='default-test-model-select'
                              />
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />

                      <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                        <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                          {t('channels.dialogs.userAgentPassThrough.label')}
                        </FormLabel>
                        <div className='space-y-1 md:col-span-6'>
                          <Select
                            value={passThroughUserAgent === null ? 'inherit' : passThroughUserAgent ? 'enabled' : 'disabled'}
                            onValueChange={(value) => setPassThroughUserAgent(value === 'inherit' ? null : value === 'enabled')}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('channels.dialogs.userAgentPassThrough.inherit')} />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='inherit'>{userAgentInheritLabel}</SelectItem>
                              <SelectItem value='enabled'>{t('channels.dialogs.userAgentPassThrough.enabled')}</SelectItem>
                              <SelectItem value='disabled'>{t('channels.dialogs.userAgentPassThrough.disabled')}</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </FormItem>

                      <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                        <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                          {t('channels.dialogs.bodyPassThrough.label')}
                        </FormLabel>
                        <div className='space-y-2 md:col-span-6'>
                          <Select
                            value={passThroughBody === null ? 'inherit' : passThroughBody ? 'enabled' : 'disabled'}
                            onValueChange={(value) => setPassThroughBody(value === 'inherit' ? null : value === 'enabled')}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('channels.dialogs.bodyPassThrough.inherit')} />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='inherit'>{passThroughInheritLabel}</SelectItem>
                              <SelectItem value='enabled'>{t('channels.dialogs.bodyPassThrough.enabled')}</SelectItem>
                              <SelectItem value='disabled'>{t('channels.dialogs.bodyPassThrough.disabled')}</SelectItem>
                            </SelectContent>
                          </Select>
                          {passThroughBody === true && (
                            <p className='text-xs text-amber-600 dark:text-amber-400'>{t('channels.dialogs.bodyPassThrough.warning')}</p>
                          )}
                        </div>
                      </FormItem>

                      <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                        <div className='flex items-center gap-1.5 pt-2 md:col-span-2 md:justify-start'>
                          <FormLabel className='font-medium'>{t('channels.dialogs.retryableStatusCodes.label')}</FormLabel>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type='button'
                                className='text-muted-foreground hover:text-foreground inline-flex items-center'
                                aria-label={t('channels.dialogs.retryableStatusCodes.tooltip')}
                              >
                                <Info className='h-3.5 w-3.5' />
                              </button>
                            </TooltipTrigger>
                            <TooltipContent className='max-w-sm'>
                              <p>{t('channels.dialogs.retryableStatusCodes.tooltip')}</p>
                            </TooltipContent>
                          </Tooltip>
                        </div>
                        <div className='md:col-span-6'>
                          <Input
                            value={retryableStatusCodesText}
                            onChange={(event) => setRetryableStatusCodesText(event.target.value)}
                            placeholder={t('channels.dialogs.retryableStatusCodes.placeholder')}
                            className='font-mono text-sm'
                          />
                        </div>
                      </FormItem>

                      <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                        <div className='flex items-center gap-1.5 pt-2 md:col-span-2 md:justify-start'>
                          <FormLabel className='font-medium'>{t('channels.dialogs.retryableErrorPatterns.label')}</FormLabel>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type='button'
                                className='text-muted-foreground hover:text-foreground inline-flex items-center'
                                aria-label={t('channels.dialogs.retryableErrorPatterns.description')}
                              >
                                <Info className='h-3.5 w-3.5' />
                              </button>
                            </TooltipTrigger>
                            <TooltipContent className='max-w-sm'>
                              <p>{t('channels.dialogs.retryableErrorPatterns.description')}</p>
                            </TooltipContent>
                          </Tooltip>
                        </div>
                        <div className='md:col-span-6'>
                          <Textarea
                            value={retryableErrorPatternsText}
                            onChange={(event) => setRetryableErrorPatternsText(event.target.value)}
                            placeholder={t('channels.dialogs.retryableErrorPatterns.placeholder')}
                            className='min-h-[88px] resize-y font-mono text-sm'
                          />
                        </div>
                      </FormItem>

                      <FormField
                        control={form.control}
                        name='tags'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.tags.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              <TagsAutocompleteInput
                                value={field.value || []}
                                onChange={field.onChange}
                                placeholder={t('channels.dialogs.fields.tags.placeholder')}
                                suggestions={allTags}
                                isLoading={isLoadingTags}
                              />
                              <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.tags.description')}</p>
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='remark'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.remark.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              <Textarea
                                placeholder={t('channels.dialogs.fields.remark.placeholder')}
                                className='min-h-[80px] resize-y'
                                {...field}
                                value={field.value || ''}
                              />
                              <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.remark.description')}</p>
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />
                    </div>
                  </div>
                </form>
              </Form>
            </div>

            {/* Expandable Side Panel */}
            <div
              className='border-border flex min-h-0 flex-col overflow-hidden border-l pt-1.5 pl-4 transition-all duration-300 ease-out'
              style={{
                width: showGroupModelsPanel || showKeyPanel ? '400px' : '0px',
                opacity: showGroupModelsPanel || showKeyPanel ? 1 : 0,
                paddingLeft: showGroupModelsPanel || showKeyPanel ? '16px' : '0px',
              }}
            >
              {/* API Key list panel for one group. */}
              <div
                className={`flex h-full min-h-0 flex-col transition-opacity duration-200 ${showKeyPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
              >
                <div className='mb-3 flex items-center justify-between'>
                  <div className='flex min-w-0 items-center gap-2'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      className='h-6 w-6 shrink-0 p-0'
                      onClick={() => {
                        setShowKeyPanel(false);
                        setActiveKeyGroupIndex(null);
                      }}
                    >
                      <PanelLeft className='h-4 w-4' />
                    </Button>
                    <h3 className='truncate text-sm font-semibold'>
                      {t('channels.dialogs.fields.keyGroup.keyPanelTitle', {
                        name: activeKeyGroupIndex === null ? '' : groupModelLabel(keyGroups?.[activeKeyGroupIndex], activeKeyGroupIndex, t),
                      })}
                    </h3>
                  </div>
                </div>

                <div className='min-h-0 flex-1 space-y-2 overflow-auto pr-3'>
                  {(activeKeyGroupIndex === null ? [] : keyGroups?.[activeKeyGroupIndex]?.apiKeys || []).map((key, keyIndex) => (
                    <div key={`${key}-${keyIndex}`} className='bg-muted rounded-md px-3 py-2 font-mono text-xs break-all'>
                      {key}
                    </div>
                  ))}
                  {activeKeyGroupIndex !== null && (keyGroups?.[activeKeyGroupIndex]?.apiKeys || []).length === 0 && (
                    <p className='text-muted-foreground py-8 text-center text-sm'>{t('channels.dialogs.fields.keyGroup.noKeys')}</p>
                  )}
                </div>
              </div>

              {/* Group Models Panel Content: pick models per group, with the source
                  group shown next to every model returned by a unified fetch. */}
              <div
                className={`flex h-full min-h-0 flex-col transition-opacity duration-200 ${showGroupModelsPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
              >
                <div className='mb-3 flex items-center justify-between'>
                  <div className='flex min-w-0 items-center gap-2'>
                    <Button type='button' variant='ghost' size='sm' className='h-6 w-6 shrink-0 p-0' onClick={closeGroupModelsPanel}>
                      <PanelLeft className='h-4 w-4' />
                    </Button>
                    <h3 className='truncate text-sm font-semibold'>
                      {activeGroupModelIndex !== null
                        ? t('channels.dialogs.fields.keyGroup.panelTitleForGroup', {
                            name: groupModelLabel(keyGroups?.[activeGroupModelIndex], activeGroupModelIndex, t),
                          })
                        : t('channels.dialogs.fields.keyGroup.panelTitleAll')}
                    </h3>
                  </div>
                </div>

                <p className='text-muted-foreground mb-3 text-xs'>
                  {activeGroupModelIndex !== null
                    ? t('channels.dialogs.fields.keyGroup.panelDescriptionForGroup')
                    : t('channels.dialogs.fields.keyGroup.panelDescriptionAll')}
                </p>

                <div className='relative mb-3'>
                  <Search className='text-muted-foreground absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2' />
                  <Input
                    placeholder={t('channels.dialogs.fields.supportedModels.searchPlaceholder')}
                    value={groupModelsSearch}
                    onChange={(e) => setGroupModelsSearch(e.target.value)}
                    className='h-8 pl-8 text-sm'
                  />
                </div>

                <div className='mb-3 flex items-center justify-between gap-2'>
                  <span className='text-muted-foreground text-xs'>
                    {t('channels.dialogs.fields.keyGroup.selectedCount', {
                      count: groupModelSelection.length,
                      total: groupModelCandidates.length,
                    })}
                  </span>
                  <div className='flex gap-1'>
                    <Button type='button' variant='outline' size='sm' className='h-6 px-2 text-xs' onClick={selectAllGroupModels}>
                      {t('channels.dialogs.buttons.selectAll')}
                    </Button>
                    <Button type='button' variant='outline' size='sm' className='h-6 px-2 text-xs' onClick={deselectAllGroupModels}>
                      {t('channels.dialogs.buttons.deselectAll')}
                    </Button>
                  </div>
                </div>

                <div className='min-h-0 flex-1 overflow-auto pr-3'>
                  {filteredGroupModelCandidates.length === 0 ? (
                    <p className='text-muted-foreground py-8 text-center text-sm'>{t('channels.dialogs.fields.keyGroup.noModels')}</p>
                  ) : (
                    <div className='space-y-1'>
                      {filteredGroupModelCandidates.map((model) => {
                        const selected = groupModelSelection.includes(model);
                        const sources = modelSourceGroups[model] || [];
                        return (
                          <label key={model} className='hover:bg-accent flex cursor-pointer items-start gap-2 rounded-md p-2 text-sm'>
                            <Checkbox checked={selected} onCheckedChange={() => toggleGroupModelSelection(model)} className='mt-0.5' />
                            <div className='flex min-w-0 flex-1 flex-wrap items-center gap-1'>
                              <code className='break-all'>{model}</code>
                              {sources.map((source) => (
                                <Badge key={source} variant='outline' className='h-5 shrink-0 px-1.5 text-[10px] font-normal'>
                                  {source}
                                </Badge>
                              ))}
                            </div>
                          </label>
                        );
                      })}
                    </div>
                  )}
                </div>

                <div className='mt-2 flex gap-2 border-t pt-2'>
                  <Button type='button' className='flex-1' size='sm' onClick={applyGroupModelSelection}>
                    {t('channels.dialogs.buttons.confirmSelection')}
                  </Button>
                </div>
              </div>
            </div>
          </div>
          <DialogFooter className='flex-shrink-0'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button
              type='submit'
              form='channel-form'
              disabled={isSubmitting || supportedModels.length === 0}
              data-testid='channel-submit-button'
            >
              {isSubmitting
                ? isEdit
                  ? t('common.buttons.editing')
                  : t('common.buttons.creating')
                : isEdit
                  ? t('common.buttons.edit')
                  : t('common.buttons.create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
