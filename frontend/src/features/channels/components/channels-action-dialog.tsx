'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, RefreshCw, Search, ChevronLeft, ChevronRight, PanelLeft, Plus, Trash2, Eye, EyeOff, Copy } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { SelectDropdown } from '@/components/select-dropdown';
import { antigravityOAuthExchange, antigravityOAuthStart } from '../data/antigravity';
import {
  useCreateChannel,
  useUpdateChannel,
  useFetchModels,
  useBulkCreateChannels,
  useAllChannelNames,
  useAllChannelTags,
  useChannelDisabledAPIKeys,
} from '../data/channels';
import { claudecodeOAuthExchange, claudecodeOAuthStart } from '../data/claudecode';
import { codexOAuthExchange, codexOAuthStart } from '../data/codex';
import {
  getDefaultBaseURL,
  getDefaultModels,
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
import { Channel, ChannelType, ApiFormat, createChannelInputSchema, updateChannelInputSchema } from '../data/schema';
import { ProxyConfig, useOAuthFlow } from '../hooks/use-oauth-flow';
import { ManualModelBadge } from './manual-model-badge';
import { ProxyType } from './channels-proxy-dialog';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

interface Props {
  currentRow?: Channel;
  duplicateFromRow?: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  showModelsPanel?: boolean;
}

const MAX_MODELS_DISPLAY = 2;

const duplicateNameRegex = /^(.*) \((\d+)\)$/;

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

export function ChannelsActionDialog({ currentRow, duplicateFromRow, open, onOpenChange, showModelsPanel = false }: Props) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;
  const isDuplicate = !!duplicateFromRow && !isEdit;
  const initialRow: Channel | undefined = currentRow || duplicateFromRow;
  const createChannel = useCreateChannel();
  const updateChannel = useUpdateChannel();
  const fetchModels = useFetchModels();
  const { data: allChannelNames = [], isSuccess: allChannelNamesLoaded } = useAllChannelNames({ enabled: open && isDuplicate });
  const { data: allTags = [], isLoading: isLoadingTags } = useAllChannelTags();
  const selectedProjectId = useSelectedProjectId();
  const [supportedModels, setSupportedModels] = useState<string[]>(() => initialRow?.supportedModels || []);
  const [manualModels, setManualModels] = useState<string[]>(() => initialRow?.manualModels || []);
  const [newModel, setNewModel] = useState('');
  const [selectedDefaultModels, setSelectedDefaultModels] = useState<string[]>([]);
  const [fetchedModels, setFetchedModels] = useState<string[]>([]);
  const [useFetchedModels, setUseFetchedModels] = useState(false);
  const providerRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const providerListRef = useRef<HTMLDivElement | null>(null);

  // Expandable panel states
  const [showFetchedModelsPanel, setShowFetchedModelsPanel] = useState(false);
  const [showSupportedModelsPanel, setShowSupportedModelsPanel] = useState(false);
  const [fetchedModelsSearch, setFetchedModelsSearch] = useState('');
  const [supportedModelsSearch, setSupportedModelsSearch] = useState('');
  const [selectedFetchedModels, setSelectedFetchedModels] = useState<string[]>([]);
  const [showNotAddedModelsOnly, setShowNotAddedModelsOnly] = useState(false);
  const [supportedModelsExpanded, setSupportedModelsExpanded] = useState(false);
  const [showClearAllPopover, setShowClearAllPopover] = useState(false);
  const hasAutoSetDuplicateNameRef = useRef(false);
  const [showApiKey, setShowApiKey] = useState(false);
  const [showApiKeysPanel, setShowApiKeysPanel] = useState(false);
  const [apiKeysSearch, setApiKeysSearch] = useState('');
  const [selectedKeysToRemove, setSelectedKeysToRemove] = useState<Set<string>>(new Set());
  const [confirmRemoveSelectedOpen, setConfirmRemoveSelectedOpen] = useState(false);
  const [confirmRemoveKey, setConfirmRemoveKey] = useState<string | null>(null);
  const [showGcpJsonData, setShowGcpJsonData] = useState(false);
  const [authMode, setAuthMode] = useState<'official' | 'third-party'>('official');
  const dialogContentRef = useRef<HTMLDivElement>(null);

  const [proxyType, setProxyType] = useState<ProxyType>(() => {
    if (initialRow?.settings?.proxy?.type) {
      return initialRow.settings.proxy.type as ProxyType;
    }
    return ProxyType.ENVIRONMENT;
  });
  const [proxyUrl, setProxyUrl] = useState(() => initialRow?.settings?.proxy?.url || '');
  const [proxyUsername, setProxyUsername] = useState(() => initialRow?.settings?.proxy?.username || '');
  const [proxyPassword, setProxyPassword] = useState(() => initialRow?.settings?.proxy?.password || '');

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

  // OAuth flows using the reusable hook
  // OAuth credentials are stored in apiKey field as JSON string, not in apiKeys array
  const codexOAuth = useOAuthFlow({
    startFn: codexOAuthStart,
    exchangeFn: codexOAuthExchange,
    projectId: selectedProjectId,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });

  const claudecodeOAuth = useOAuthFlow({
    startFn: claudecodeOAuthStart,
    exchangeFn: claudecodeOAuthExchange,
    projectId: selectedProjectId,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });

  const antigravityOAuth = useOAuthFlow({
    startFn: antigravityOAuthStart,
    exchangeFn: antigravityOAuthExchange,
    projectId: selectedProjectId,
    proxyConfig,
    onSuccess: (credentials) => {
      form.setValue('credentials.apiKey', credentials);
    },
  });

  // Provider-based selection state
  const [selectedProvider, setSelectedProvider] = useState<string>(() => {
    if (initialRow) {
      return getProviderFromChannelType(initialRow.type) || 'openai';
    }
    return 'openai';
  });
  const [selectedApiFormat, setSelectedApiFormat] = useState<ApiFormat>(() => {
    if (initialRow) {
      return CHANNEL_CONFIGS[initialRow.type as ChannelType]?.apiFormat || 'openai/chat_completions';
    }
    return 'openai/chat_completions';
  });
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

  useEffect(() => {
    if (!initialRow) return;

    const provider = getProviderFromChannelType(initialRow.type) || 'openai';
    setSelectedProvider(provider);
    const apiFormat = CHANNEL_CONFIGS[initialRow.type as ChannelType]?.apiFormat || OPENAI_CHAT_COMPLETIONS;
    setSelectedApiFormat(apiFormat);
    setUseGeminiVertex(initialRow.type === 'gemini_vertex');
    setUseAnthropicAws(initialRow.type === 'anthropic_aws');

    // Detect authMode for codex and claudecode
    if (initialRow.type === 'codex') {
      try {
        const apiKey = initialRow.credentials?.apiKey || '';
        const json = JSON.parse(apiKey);
        if (json.access_token && json.refresh_token) {
          setAuthMode('official');
        } else {
          setAuthMode('third-party');
        }
      } catch {
        setAuthMode('third-party');
      }
    } else if (initialRow.type === 'claudecode') {
      const apiKey = initialRow.credentials?.apiKey || '';
      const defaultURL = getDefaultBaseURL('claudecode');
      // For Claude Code, it's official if it's an official token (sk-ant-oat or sk-ant-api03) or uses the default base URL
      if (apiKey.includes('sk-ant-oat') || apiKey.includes('sk-ant-api03') || initialRow.baseURL === defaultURL) {
        setAuthMode('official');
      } else {
        setAuthMode('third-party');
      }
    }
  }, [initialRow]);

  useEffect(() => {
    if (!open) {
      hasAutoSetDuplicateNameRef.current = false;
      codexOAuth.reset();
      claudecodeOAuth.reset();
      antigravityOAuth.reset();
    }
  }, [open, codexOAuth, claudecodeOAuth, antigravityOAuth]);

  useEffect(() => {
    if (!open) {
      setShowApiKey(false);
      setShowApiKeysPanel(false);
      setApiKeysSearch('');
      setSelectedKeysToRemove(new Set());
      setConfirmRemoveSelectedOpen(false);
      setConfirmRemoveKey(null);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;

    const timer = setTimeout(() => {
      const target = providerRefs.current[selectedProvider];
      const container = providerListRef.current;
      if (target && container) {
        const containerHeight = container.clientHeight;
        const targetOffsetTop = target.offsetTop;
        const targetHeight = target.clientHeight;

        const targetCenter = targetOffsetTop + targetHeight / 2;
        const scrollTop = targetCenter - containerHeight / 2;

        container.scrollTop = Math.max(0, scrollTop);
      }
    }, 100);

    return () => clearTimeout(timer);
  }, [open, isEdit, selectedProvider]);

  // Auto-open supported models panel when showModelsPanel is true
  useEffect(() => {
    if (open && showModelsPanel && initialRow && initialRow.supportedModels.length > 0) {
      setShowSupportedModelsPanel(true);
      setShowFetchedModelsPanel(false);
      setShowApiKeysPanel(false);
    }
  }, [open, showModelsPanel, initialRow]);

  // Sync manualModels when dialog opens with new initialRow
  useEffect(() => {
    if (open && initialRow) {
      setManualModels(initialRow.manualModels || []);
    }
  }, [open, initialRow]);

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

  const getApiFormatLabel = useCallback(
    (format: ApiFormat) => {
      return t(`channels.dialogs.fields.apiFormat.formats.${format}`);
    },
    [t]
  );

  // Determine the actual channel type based on provider and API format
  const derivedChannelType = useMemo(() => {
    if (isEdit && currentRow) {
      return currentRow.type;
    }

    // If gemini/contents is selected and vertex checkbox is checked, use gemini_vertex
    if (selectedApiFormat === 'gemini/contents' && useGeminiVertex) {
      return 'gemini_vertex';
    }

    // If anthropic/messages is selected, check which variant is selected
    if (selectedApiFormat === 'anthropic/messages') {
      if (useAnthropicAws) return 'anthropic_aws';
    }

    return getChannelTypeForApiFormat(selectedProvider, selectedApiFormat) || 'openai';
  }, [isEdit, currentRow, selectedProvider, selectedApiFormat, useGeminiVertex, useAnthropicAws]);

  const formSchema = isEdit ? updateChannelInputSchema : createChannelInputSchema;

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues:
      isEdit && currentRow
        ? {
            type: currentRow.type,
            baseURL: currentRow.baseURL,
            name: currentRow.name,
            policies: currentRow.policies ?? { stream: 'unlimited' },
            supportedModels: currentRow.supportedModels,
            autoSyncSupportedModels: currentRow.autoSyncSupportedModels,
            defaultTestModel: currentRow.defaultTestModel,
            tags: currentRow.tags || [],
            remark: currentRow.remark || '',
            credentials: {
              // OAuth 类型 (codex/claudecode/antigravity) 的凭据存储在 apiKey 字段，不放入 apiKeys
              apiKey: currentRow.credentials?.apiKey || undefined,
              apiKeys: currentRow.credentials?.apiKeys || [],
              gcp: {
                region: currentRow.credentials?.gcp?.region || '',
                projectID: currentRow.credentials?.gcp?.projectID || '',
                jsonData: currentRow.credentials?.gcp?.jsonData || '',
              },
            },
          }
        : duplicateFromRow
          ? {
              type: duplicateFromRow.type,
              baseURL: duplicateFromRow.baseURL,
              name: duplicateFromRow.name,
              policies: duplicateFromRow.policies ?? { stream: 'unlimited' },
              supportedModels: duplicateFromRow.supportedModels,
              autoSyncSupportedModels: duplicateFromRow.autoSyncSupportedModels,
              defaultTestModel: duplicateFromRow.defaultTestModel,
              tags: duplicateFromRow.tags || [],
              remark: duplicateFromRow.remark || '',
              settings: duplicateFromRow.settings ?? undefined,
              credentials: {
                // OAuth 类型 (codex/claudecode/antigravity) 的凭据存储在 apiKey 字段，不放入 apiKeys
                apiKey: duplicateFromRow.credentials?.apiKey || undefined,
                apiKeys: duplicateFromRow.credentials?.apiKeys || [],
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

  const apiKeys = form.watch('credentials.apiKeys');
  const apiKeysCount = useMemo(() => (apiKeys || []).filter((k) => k.trim().length > 0).length, [apiKeys]);

  const { data: disabledKeys = [] } = useChannelDisabledAPIKeys(currentRow?.id || '', {
    enabled: isEdit && !!currentRow?.id && showApiKeysPanel,
  });

  const disabledKeySet = useMemo(() => new Set(disabledKeys.map((dk) => dk.key)), [disabledKeys]);

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

  const isCodexType = (selectedType || derivedChannelType) === 'codex';
  const isAntigravityType = (selectedType || derivedChannelType) === 'antigravity';
  const isClaudeCodeType = (selectedType || derivedChannelType) === 'claudecode';

  useEffect(() => {
    // Only force stream: 'require' for new Codex channels, not when editing existing ones
    if (isCodexType && !isEdit) {
      form.setValue('policies.stream', 'require');
    }
  }, [isCodexType, isEdit, form]);

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
    const defaultURL = getDefaultBaseURL(currentType);
    if (defaultURL) {
      return defaultURL;
    }
    return t('channels.dialogs.fields.baseURL.placeholder');
  }, [selectedType, derivedChannelType, t]);

  // Sync form type when provider or API format changes (only for create mode)
  const handleProviderChange = useCallback(
    (provider: string) => {
      if (isEdit) return;
      setSelectedProvider(provider);
      setAuthMode('official');

      if (provider !== 'gemini') {
        setUseGeminiVertex(false);
      }
      if (provider !== 'anthropic') {
        setUseAnthropicAws(false);
      }

      if (provider === 'codex') {
        setSelectedApiFormat(OPENAI_RESPONSES);
        form.setValue('type', 'codex');
        form.setValue('policies.stream', 'require');
        setFetchedModels([]);
        setUseFetchedModels(false);
        return;
      }

      if (provider === 'antigravity') {
        setSelectedApiFormat(GEMINI_CONTENTS);
        form.setValue('type', 'antigravity');
        setFetchedModels([]);
        setUseFetchedModels(false);
        // Set default Base URL only if empty
        const baseURL = getDefaultBaseURL('antigravity');
        const currentURL = form.getValues('baseURL');
        if (baseURL && !isDuplicate && (!currentURL || currentURL === '')) {
          form.setValue('baseURL', baseURL);
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
            : getChannelTypeForApiFormat(provider, newFormat);
      if (newChannelType) {
        form.setValue('type', newChannelType);
        if (!isDuplicate) {
          const baseURL = getDefaultBaseURL(newChannelType);
          if (baseURL) {
            form.resetField('baseURL', { defaultValue: baseURL });
          }
        }
        setFetchedModels([]);
        setUseFetchedModels(false);
      }
    },
    [isEdit, form, useGeminiVertex, useAnthropicAws, isDuplicate, selectedApiFormat]
  );

  const handleApiFormatChange = useCallback(
    (format: ApiFormat) => {
      if (isEdit) return;
      if (selectedProvider === 'codex' || selectedProvider === 'antigravity') return;

      setSelectedApiFormat(format);

      // Reset vertex checkbox if not gemini/contents
      if (format !== 'gemini/contents') {
        setUseGeminiVertex(false);
      }
      // Reset anthropic checkboxes if not anthropic/messages
      if (format !== 'anthropic/messages') {
        setUseAnthropicAws(false);
      }

      const channelTypeFromFormat = getChannelTypeForApiFormat(selectedProvider, format);
      const newChannelType =
        format === 'gemini/contents' && useGeminiVertex
          ? 'gemini_vertex'
          : format === 'anthropic/messages' && useAnthropicAws
            ? 'anthropic_aws'
            : channelTypeFromFormat;
      if (newChannelType) {
        form.setValue('type', newChannelType);

        const baseURLFieldState = form.getFieldState('baseURL', form.formState);
        if (!baseURLFieldState.isDirty && !isDuplicate) {
          const baseURL = getDefaultBaseURL(newChannelType);
          if (baseURL) {
            form.resetField('baseURL', { defaultValue: baseURL });
          }
        }
      }
    },
    [isEdit, selectedProvider, form, useGeminiVertex, useAnthropicAws, isDuplicate]
  );

  const handleGeminiVertexChange = useCallback(
    (checked: boolean) => {
      if (isEdit) return;
      setUseGeminiVertex(checked);

      if (selectedApiFormat === 'gemini/contents') {
        const newChannelType = checked ? 'gemini_vertex' : 'gemini';
        form.setValue('type', newChannelType);

        const baseURLFieldState = form.getFieldState('baseURL', form.formState);
        if (!baseURLFieldState.isDirty && !isDuplicate) {
          const baseURL = getDefaultBaseURL(newChannelType);
          if (baseURL) {
            form.resetField('baseURL', { defaultValue: baseURL });
          }
        }
      }
    },
    [isEdit, selectedApiFormat, form, isDuplicate]
  );

  const handleAnthropicAwsChange = useCallback(
    (checked: boolean) => {
      if (isEdit) return;
      setUseAnthropicAws(checked);

      if (selectedApiFormat === 'anthropic/messages') {
        const newChannelType = checked ? 'anthropic_aws' : 'anthropic';
        form.setValue('type', newChannelType);

        const baseURLFieldState = form.getFieldState('baseURL', form.formState);
        if (!baseURLFieldState.isDirty && !isDuplicate) {
          const baseURL = getDefaultBaseURL(newChannelType);
          if (baseURL) {
            form.resetField('baseURL', { defaultValue: baseURL });
          }
        }
      }
    },
    [isEdit, selectedApiFormat, form, isDuplicate]
  );

  useEffect(() => {
    if (isEdit || isDuplicate) return;

    if (!isCodexType) {
      codexOAuth.reset();
    }
    if (selectedProvider !== 'claudecode') {
      claudecodeOAuth.reset();
    }
    if (selectedProvider !== 'antigravity') {
      antigravityOAuth.reset();
    }

    const providerToChannelType: Partial<Record<string, ChannelType>> = {
      claudecode: authMode === 'official' ? 'claudecode' : undefined,
      codex: authMode === 'official' ? 'codex' : undefined,
      antigravity: 'antigravity',
    };

    let channelTypeForURL: ChannelType | undefined = providerToChannelType[selectedProvider];

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
  }, [isEdit, isDuplicate, isCodexType, selectedProvider, authMode, form, codexOAuth, claudecodeOAuth, antigravityOAuth]);

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

          <p className='text-muted-foreground mt-2 text-xs'>{description}</p>
        </div>
      </div>
    ),
    [t]
  );

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    // Check if there are selected fetched models that haven't been confirmed
    if (selectedFetchedModels.length > 0) {
      toast.error(t('channels.messages.modelsNotConfirmed'));
      return;
    }

    try {
      if (values.credentials?.apiKeys) {
        values.credentials.apiKeys = [...new Set(values.credentials.apiKeys.filter((k) => k.trim().length > 0))];
      }

      const valuesForSubmit = isEdit
        ? values
        : {
            ...values,
            type: derivedChannelType,
          };

      const dataWithModels = {
        ...valuesForSubmit,
        supportedModels,
        manualModels,
      };

      if ((isCodexType || isClaudeCodeType) && authMode === 'official' && !isDuplicate) {
        const currentType = selectedType || derivedChannelType;
        const baseURL = getDefaultBaseURL(currentType);
        if (baseURL) {
          dataWithModels.baseURL = baseURL;
        }
      }

      if (isEdit && currentRow) {
        const updateInput = {
          ...dataWithModels,
          settings: undefined,
          type: undefined,
        } as z.infer<typeof updateChannelInputSchema>;

        const apiKey = values.credentials?.apiKey || '';
        const hasApiKey = apiKey.trim().length > 0;
        const apiKeys = values.credentials?.apiKeys || [];
        const hasApiKeys = apiKeys.length > 0 && apiKeys.some((k) => k.trim() !== '');
        const hasGcpCredentials =
          values.credentials?.gcp?.region &&
          values.credentials.gcp.region.trim() !== '' &&
          values.credentials?.gcp?.projectID &&
          values.credentials.gcp.projectID.trim() !== '' &&
          values.credentials?.gcp?.jsonData &&
          values.credentials.gcp.jsonData.trim() !== '';

        if (!hasApiKey && !hasApiKeys && !hasGcpCredentials) {
          delete updateInput.credentials;
        }

        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: updateInput,
        });
      } else {
        const proxyConfig = {
          type: proxyType as 'disabled' | 'environment' | 'url',
          ...(proxyType === ProxyType.URL && {
            url: proxyUrl,
            username: proxyUsername || undefined,
            password: proxyPassword || undefined,
          }),
        };

        const nextSettings = mergeChannelSettingsForUpdate(values.settings, {
          proxy: proxyConfig,
        });

        await createChannel.mutateAsync({
          ...(dataWithModels as z.infer<typeof createChannelInputSchema>),
          settings: nextSettings,
        } as z.infer<typeof createChannelInputSchema>);
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

  const removeModel = (model: string) => {
    setSupportedModels(supportedModels.filter((m) => m !== model));
    setManualModels(manualModels.filter((m) => m !== model));
  };

  const isModelManual = (model: string): boolean => {
    return manualModels.includes(model);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addModel();
    }
  };

  const toggleDefaultModel = (model: string) => {
    setSelectedDefaultModels((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]));
  };

  const addSelectedDefaultModels = () => {
    const newModels = selectedDefaultModels.filter((model) => !supportedModels.includes(model));
    if (newModels.length > 0) {
      setSupportedModels((prev) => [...prev, ...newModels]);
      setSelectedDefaultModels([]);
    }
  };

  const handleClearAllSupportedModels = () => {
    setSupportedModels([]);
    setManualModels([]);
  };

  const handleFetchModels = useCallback(async () => {
    const channelType = form.getValues('type');
    const baseURL = form.getValues('baseURL');
    const apiKeys = form.getValues('credentials.apiKeys');

    if (!channelType || !baseURL) {
      return;
    }

    try {
      // Extract first API key from the array
      const firstApiKey = apiKeys?.find((key) => key.trim().length > 0) || '';

      const result = await fetchModels.mutateAsync({
        channelType,
        baseURL,
        apiKey: !isEdit ? firstApiKey : firstApiKey || undefined,
        channelID: isEdit ? currentRow?.id : undefined,
      });

      if (result.error) {
        toast.error(result.error);
        return;
      }

      const models = result.models.map((m) => m.id);
      if (models?.length) {
        setFetchedModels(models);
        setUseFetchedModels(true);
        setShowFetchedModelsPanel(true);
        setShowSupportedModelsPanel(false);
        setShowApiKeysPanel(false);
        setSelectedFetchedModels([]);
        setFetchedModelsSearch('');
        setShowNotAddedModelsOnly(false);
      }
    } catch (_error) {
      // Error is already handled by the mutation
    }
  }, [fetchModels, form, isEdit, currentRow]);

  const canFetchModels = () => {
    const baseURL = form.watch('baseURL');
    const apiKeys = form.watch('credentials.apiKeys');
    const hasApiKey = apiKeys?.some((key) => key.trim().length > 0);

    if (isCodexType || isAntigravityType) {
      return !!baseURL;
    }

    if (isEdit) {
      return !!baseURL;
    }

    return !!baseURL && hasApiKey;
  };

  // Memoize quick models to avoid re-evaluating on every render
  const currentType = form.watch('type');
  const quickModels = useMemo(() => {
    if (useFetchedModels || !currentType) return [];
    return getDefaultModels(currentType);
  }, [currentType, useFetchedModels]);

  // Filtered fetched models based on search and filter
  const filteredFetchedModels = useMemo(() => {
    let models = fetchedModels;
    if (showNotAddedModelsOnly) {
      models = models.filter((model) => !supportedModels.includes(model));
    }
    if (fetchedModelsSearch.trim()) {
      const search = fetchedModelsSearch.toLowerCase();
      models = models.filter((model) => model.toLowerCase().includes(search));
    }
    return models;
  }, [fetchedModels, fetchedModelsSearch, showNotAddedModelsOnly, supportedModels]);

  // Toggle selection for fetched model
  const toggleFetchedModelSelection = useCallback((model: string) => {
    setSelectedFetchedModels((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]));
  }, []);

  // Select all filtered models
  const selectAllFilteredModels = useCallback(() => {
    setSelectedFetchedModels(filteredFetchedModels);
  }, [filteredFetchedModels]);

  // Deselect all
  const deselectAllFetchedModels = useCallback(() => {
    setSelectedFetchedModels([]);
  }, []);

  // Add or remove selected fetched models to supported models
  const addSelectedFetchedModels = useCallback(() => {
    const modelsToRemove: string[] = [];

    setSupportedModels((prev) => {
      const modelsToAdd: string[] = [];

      selectedFetchedModels.forEach((model) => {
        if (prev.includes(model)) {
          modelsToRemove.push(model);
        } else {
          modelsToAdd.push(model);
        }
      });

      const afterRemoval = prev.filter((m) => !modelsToRemove.includes(m));
      return [...afterRemoval, ...modelsToAdd];
    });

    // Remove toggled-off models from manualModels
    if (modelsToRemove.length > 0) {
      setManualModels((prev) => prev.filter((m) => !modelsToRemove.includes(m)));
    }

    setSelectedFetchedModels([]);
  }, [selectedFetchedModels]);

  // Close panel handler
  const closeFetchedModelsPanel = useCallback(() => {
    setShowFetchedModelsPanel(false);
    setSelectedFetchedModels([]);
    setFetchedModelsSearch('');
    setShowNotAddedModelsOnly(false);
  }, []);

  // Close supported models panel handler
  const closeSupportedModelsPanel = useCallback(() => {
    setShowSupportedModelsPanel(false);
  }, []);

  const closeApiKeysPanel = useCallback(() => {
    setShowApiKeysPanel(false);
    setApiKeysSearch('');
    setSelectedKeysToRemove(new Set());
    setConfirmRemoveSelectedOpen(false);
    setConfirmRemoveKey(null);
  }, []);

  const removeApiKeys = useCallback(
    (keysToRemove: string[]) => {
      const currentKeys = form.getValues('credentials.apiKeys') || [];
      const nextKeys = currentKeys.filter((k) => !keysToRemove.includes(k));
      const validNextKeys = nextKeys.filter((k) => k.trim().length > 0);
      if (validNextKeys.length === 0) {
        toast.error(t('channels.dialogs.fields.apiKey.mustKeepOne'));
        setConfirmRemoveSelectedOpen(false);
        setConfirmRemoveKey(null);
        return;
      }
      form.setValue('credentials.apiKeys', nextKeys, { shouldDirty: true, shouldTouch: true });
      setSelectedKeysToRemove(new Set());
      setConfirmRemoveSelectedOpen(false);
      setConfirmRemoveKey(null);
    },
    [form, t]
  );

  // Remove deprecated models (models in supportedModels but not in fetchedModels and not manual)
  const removeDeprecatedModels = useCallback(() => {
    const fetchedModelsSet = new Set(fetchedModels);
    const manualModelsSet = new Set(manualModels);
    // Deprecated = not fetched AND not manual
    const deprecatedModels = supportedModels.filter(
      (model) => !fetchedModelsSet.has(model) && !manualModelsSet.has(model)
    );
    // Keep fetched models and manual models in supportedModels
    setSupportedModels((prev) =>
      prev.filter((model) => fetchedModelsSet.has(model) || manualModelsSet.has(model))
    );
    // Remove deprecated models from manualModels (should be none, but for consistency)
    setManualModels((prev) => prev.filter((model) => !deprecatedModels.includes(model)));
  }, [fetchedModels, supportedModels, manualModels]);

  // Count of deprecated models
  const deprecatedModelsCount = useMemo(() => {
    const fetchedModelsSet = new Set(fetchedModels);
    const manualModelsSet = new Set(manualModels);
    // Count only models that are neither fetched nor manual
    return supportedModels.filter(
      (model) => !fetchedModelsSet.has(model) && !manualModelsSet.has(model)
    ).length;
  }, [supportedModels, fetchedModels, manualModels]);

  // Models to display (limited to MAX_MODELS_DISPLAY unless expanded)
  const displayedSupportedModels = useMemo(() => {
    if (supportedModels.length <= MAX_MODELS_DISPLAY) {
      return supportedModels;
    }
    return supportedModels.slice(0, MAX_MODELS_DISPLAY);
  }, [supportedModels]);

  // Filtered supported models based on search
  const filteredSupportedModels = useMemo(() => {
    if (!supportedModelsSearch.trim()) {
      return supportedModels;
    }
    const search = supportedModelsSearch.toLowerCase();
    return supportedModels.filter((model) => model.toLowerCase().includes(search));
  }, [supportedModels, supportedModelsSearch]);

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(state) => {
          if (!state) {
            form.reset();
            setSupportedModels(initialRow?.supportedModels || []);
            setManualModels(initialRow?.manualModels || []);
            setSelectedDefaultModels([]);
            setFetchedModels([]);
            setUseFetchedModels(false);
            // Reset expandable panel states
            setShowFetchedModelsPanel(false);
            setShowSupportedModelsPanel(false);
            setShowApiKeysPanel(false);
            setFetchedModelsSearch('');
            setSupportedModelsSearch('');
            setSelectedFetchedModels([]);
            setShowNotAddedModelsOnly(false);
            setSupportedModelsExpanded(false);
            setApiKeysSearch('');
            setSelectedKeysToRemove(new Set());
            setConfirmRemoveSelectedOpen(false);
            setConfirmRemoveKey(null);
            setShowApiKey(false);
            // Reset proxy state
            if (initialRow?.settings?.proxy?.type) {
              setProxyType(initialRow.settings.proxy.type as ProxyType);
            } else {
              setProxyType(ProxyType.ENVIRONMENT);
            }
            setProxyUrl(initialRow?.settings?.proxy?.url || '');
            setProxyUsername(initialRow?.settings?.proxy?.username || '');
            setProxyPassword(initialRow?.settings?.proxy?.password || '');
            // Reset provider and API format state
            if (initialRow) {
              setSelectedProvider(getProviderFromChannelType(initialRow.type) || 'openai');
              setSelectedApiFormat(CHANNEL_CONFIGS[initialRow.type as ChannelType]?.apiFormat || OPENAI_CHAT_COMPLETIONS);
              setUseGeminiVertex(initialRow.type === 'gemini_vertex');
              setUseAnthropicAws(initialRow.type === 'anthropic_aws');
            } else {
              setSelectedProvider('openai');
              setSelectedApiFormat(OPENAI_CHAT_COMPLETIONS);
              setUseGeminiVertex(false);
              setUseAnthropicAws(false);
            }
          }
          onOpenChange(state);
        }}
      >
        <DialogContent
          className={`flex max-h-[90vh] flex-col transition-all duration-300 ${showFetchedModelsPanel || showSupportedModelsPanel || showApiKeysPanel ? 'sm:max-w-6xl' : 'sm:max-w-4xl'}`}
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
              className={`flex min-h-0 flex-1 flex-col overflow-hidden py-1 transition-all duration-300 ${showFetchedModelsPanel || showSupportedModelsPanel || showApiKeysPanel ? 'pr-2' : 'pr-0'}`}
            >
              <Form {...form}>
                <form id='channel-form' onSubmit={form.handleSubmit(onSubmit)} className='flex min-h-0 flex-1 flex-col space-y-6 p-0.5'>
                  {/* Provider Selection - Left Side */}
                  <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-hidden md:flex-row md:gap-6'>
                    <div className='flex max-h-48 min-h-0 w-full flex-shrink-0 flex-col md:max-h-none md:w-60'>
                      <FormItem className='flex min-h-0 flex-1 flex-col space-y-2'>
                        <FormLabel className='text-base font-semibold'>{t('channels.dialogs.fields.provider.label')}</FormLabel>
                        <div
                          ref={providerListRef}
                          className={`flex-1 overflow-y-auto pr-2 ${isEdit ? 'cursor-not-allowed opacity-60' : ''}`}
                        >
                          <RadioGroup value={selectedProvider} onValueChange={handleProviderChange} disabled={isEdit} className='space-y-2'>
                            {availableProviders.map((provider) => {
                              const Icon = provider.icon;
                              const isSelected = provider.key === selectedProvider;
                              return (
                                <div
                                  key={provider.key}
                                  ref={(el) => {
                                    providerRefs.current[provider.key] = el;
                                  }}
                                  className={`flex items-center space-x-3 rounded-lg border p-3 transition-colors ${
                                    isEdit
                                      ? isSelected
                                        ? 'border-primary bg-muted/80 cursor-not-allowed shadow-sm'
                                        : 'cursor-not-allowed opacity-60'
                                      : (isSelected ? 'border-primary bg-accent/40 shadow-sm' : '') + ' hover:bg-accent/50'
                                  }`}
                                >
                                  <RadioGroupItem
                                    value={provider.key}
                                    id={`provider-${provider.key}`}
                                    disabled={isEdit}
                                    data-testid={`provider-${provider.key}`}
                                  />
                                  {Icon && <Icon size={20} className='flex-shrink-0' />}
                                  <FormLabel htmlFor={`provider-${provider.key}`} className='flex-1 cursor-pointer font-normal'>
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
                      {selectedProvider !== 'jina' && selectedProvider !== 'codex' && selectedProvider !== 'claudecode' && (
                        <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                            {t('channels.dialogs.fields.apiFormat.label')}
                          </FormLabel>
                          <div className='max-w-64 space-y-1 md:col-span-6 md:max-w-none'>
                            <SelectDropdown
                              key={selectedProvider}
                              defaultValue={selectedApiFormat}
                              onValueChange={(value) => handleApiFormatChange(value as ApiFormat)}
                              disabled={isEdit}
                              placeholder={t('channels.dialogs.fields.apiFormat.placeholder')}
                              data-testid='api-format-select'
                              isControlled={true}
                              items={availableApiFormats.map((format) => ({
                                value: format,
                                label: getApiFormatLabel(format),
                              }))}
                            />
                            {isEdit && (
                              <p className='text-muted-foreground mt-1 text-xs'>{t('channels.dialogs.fields.apiFormat.editDisabled')}</p>
                            )}
                            {selectedApiFormat === 'gemini/contents' && (
                              <div className='mt-3'>
                                <label
                                  className={`flex items-center gap-2 text-sm ${isEdit ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}
                                >
                                  <Checkbox
                                    checked={useGeminiVertex}
                                    onCheckedChange={(checked) => handleGeminiVertexChange(checked === true)}
                                    disabled={isEdit}
                                  />
                                  <span>{t('channels.dialogs.fields.apiFormat.geminiVertex.label')}</span>
                                </label>
                              </div>
                            )}
                            {selectedApiFormat === 'anthropic/messages' && selectedProvider === 'anthropic' && (
                              <div className='mt-3 space-y-2'>
                                <label
                                  className={`flex items-center gap-2 text-sm ${isEdit ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}
                                >
                                  <Checkbox
                                    checked={useAnthropicAws}
                                    onCheckedChange={(checked) => handleAnthropicAwsChange(checked === true)}
                                    disabled={isEdit}
                                  />
                                  <span>{t('channels.dialogs.fields.apiFormat.anthropicAWS.label')}</span>
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
                            <div className='text-sm'>{getApiFormatLabel(OPENAI_RESPONSES)}</div>
                            <p className='text-muted-foreground mt-1 text-xs'>{t('channels.dialogs.fields.apiFormat.editDisabled')}</p>
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

                                <p className='text-muted-foreground mt-2 text-xs'>
                                  {t('channels.dialogs.fields.apiFormat.antigravity.description')}
                                </p>
                              </div>
                            </div>
                          </div>
                        </FormItem>
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

                      {(isCodexType || isClaudeCodeType) && (
                        <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                          <div className='col-span-2' />
                          <div className='space-y-4 md:col-span-6'>
                            <Tabs
                              value={authMode}
                              onValueChange={(value) => {
                                const mode = value as 'official' | 'third-party';
                                setAuthMode(mode);
                                if (mode === 'official') {
                                  const currentType = selectedType || derivedChannelType;
                                  const defaultURL = getDefaultBaseURL(currentType);
                                  if (defaultURL) {
                                    form.setValue('baseURL', defaultURL);
                                  }
                                }
                              }}
                              className='w-full'
                            >
                              <TabsList className='grid w-full grid-cols-2'>
                                <TabsTrigger value='official' disabled={isEdit}>
                                  {t('channels.dialogs.authMode.official')}
                                </TabsTrigger>
                                <TabsTrigger value='third-party' disabled={isEdit}>
                                  {t('channels.dialogs.authMode.thirdParty')}
                                </TabsTrigger>
                              </TabsList>
                            </Tabs>

                            {authMode === 'official' && (
                              <div className='space-y-2'>
                                {isCodexType && renderOAuthSection(codexOAuth, t('channels.dialogs.fields.apiFormat.codex.description'))}
                                {isClaudeCodeType &&
                                  renderOAuthSection(claudecodeOAuth, t('channels.dialogs.fields.apiFormat.claudecode.description'))}
                              </div>
                            )}
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
                                  ((isCodexType || isClaudeCodeType) && authMode === 'official') || selectedProvider === 'antigravity'
                                }
                                {...field}
                              />
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />

                      {(!(isCodexType || isClaudeCodeType) || authMode === 'third-party') &&
                        selectedProvider !== 'antigravity' &&
                        selectedType !== 'anthropic_gcp' && (
                          <FormField
                            control={form.control}
                            name='credentials.apiKeys'
                            render={({ field, fieldState }) => (
                              <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                                <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                                  {t('channels.dialogs.fields.apiKey.label')}
                                </FormLabel>
                                <div className='space-y-1 md:col-span-6'>
                                  {isEdit ? (
                                    <div className='relative'>
                                      <Tooltip open={!showApiKey ? undefined : false}>
                                        <TooltipTrigger asChild>
                                          <Textarea
                                            value={
                                              showApiKey
                                                ? field.value?.join('\n') || ''
                                                : (field.value || [])
                                                    .map((k) => (k.length > 8 ? k.slice(0, 4) + '****' + k.slice(-4) : '****'))
                                                    .join('\n')
                                            }
                                            onChange={(e) => {
                                              if (!showApiKey) return;
                                              const keys = e.target.value.split('\n');
                                              field.onChange(keys);
                                            }}
                                            onBlur={(e) => {
                                              if (!showApiKey) return;
                                              const keys = [
                                                ...new Set(
                                                  e.target.value
                                                    .split('\n')
                                                    .map((k) => k.trim())
                                                    .filter((k) => k.length > 0)
                                                ),
                                              ];
                                              field.onChange(keys);
                                              field.onBlur();
                                            }}
                                            readOnly={!showApiKey}
                                            placeholder={t('channels.dialogs.fields.apiKey.editPlaceholder')}
                                            className='min-h-[80px] resize-y pr-10 font-mono text-sm md:col-span-6'
                                            autoComplete='new-password'
                                            data-form-type='other'
                                            aria-invalid={!!fieldState.error}
                                            data-testid='channel-api-key-input'
                                          />
                                        </TooltipTrigger>
                                        <TooltipContent>
                                          <p>{t('channels.dialogs.fields.apiKey.revealToEditHint')}</p>
                                        </TooltipContent>
                                      </Tooltip>
                                      <div className='absolute top-2 right-2 flex flex-col gap-1'>
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='sm'
                                          className='h-7 w-7 p-0'
                                          onClick={() => {
                                            const next = !showApiKey;
                                            setShowApiKey(next);

                                            if (!next) {
                                              setShowApiKeysPanel(false);
                                              return;
                                            }

                                            if (next) {
                                              setShowApiKeysPanel(true);
                                              setShowFetchedModelsPanel(false);
                                              setShowSupportedModelsPanel(false);
                                            }
                                          }}
                                        >
                                          {showApiKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                                        </Button>
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='sm'
                                          className='h-7 w-7 p-0'
                                          onClick={() => {
                                            const keys = field.value || [];
                                            if (keys.length > 0) {
                                              navigator.clipboard.writeText(keys.join('\n'));
                                              toast.success(t('channels.messages.credentialsCopied'));
                                            }
                                          }}
                                        >
                                          <Copy className='h-4 w-4' />
                                        </Button>
                                      </div>
                                      <p className='text-muted-foreground mt-1 text-xs'>
                                        {t('channels.dialogs.fields.apiKey.multiLineHint')}
                                      </p>
                                    </div>
                                  ) : (
                                    <>
                                      <Textarea
                                        value={field.value?.join('\n') || ''}
                                        onChange={(e) => {
                                          const keys = e.target.value.split('\n');
                                          field.onChange(keys);
                                        }}
                                        onBlur={(e) => {
                                          const keys = [
                                            ...new Set(
                                              e.target.value
                                                .split('\n')
                                                .map((k) => k.trim())
                                                .filter((k) => k.length > 0)
                                            ),
                                          ];
                                          field.onChange(keys);
                                          field.onBlur();
                                        }}
                                        placeholder={t('channels.dialogs.fields.apiKey.placeholder')}
                                        className='min-h-[80px] resize-y font-mono text-sm md:col-span-6'
                                        autoComplete='new-password'
                                        data-form-type='other'
                                        aria-invalid={!!fieldState.error}
                                        data-testid='channel-api-key-input'
                                      />
                                      <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.apiKey.multiLineHint')}</p>
                                    </>
                                  )}
                                  <FormMessage />
                                </div>
                              </FormItem>
                            )}
                          />
                        )}

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

                      <FormField
                        control={form.control}
                        name='policies.stream'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
                            <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>
                              {t('channels.dialogs.fields.streamPolicy.label')}
                            </FormLabel>
                            <div className='space-y-1 md:col-span-6'>
                              {wrapUnsupported(
                                isCodexType,
                                <SelectDropdown
                                  defaultValue={(field.value as string) || 'unlimited'}
                                  onValueChange={(value) => field.onChange(value)}
                                  placeholder={t('channels.dialogs.fields.streamPolicy.placeholder')}
                                  data-testid='channel-stream-policy-select'
                                  isControlled={true}
                                  disabled={isCodexType}
                                  items={[
                                    { value: 'unlimited', label: t('channels.dialogs.fields.streamPolicy.options.unlimited') },
                                    { value: 'require', label: t('channels.dialogs.fields.streamPolicy.options.require') },
                                    { value: 'forbid', label: t('channels.dialogs.fields.streamPolicy.options.forbid') },
                                  ]}
                                />,
                                'w-full'
                              )}
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
                            {useFetchedModels && fetchedModels.length > 20 ? (
                              <AutoCompleteSelect
                                items={fetchedModels.map((model) => ({ value: model, label: model }))}
                                selectedValue={newModel}
                                onSelectedValueChange={setNewModel}
                                placeholder={t('channels.dialogs.fields.supportedModels.description')}
                              />
                            ) : (
                              <Input
                                placeholder={t('channels.dialogs.fields.supportedModels.description')}
                                value={newModel}
                                onChange={(e) => setNewModel(e.target.value)}
                                onKeyDown={handleKeyDown}
                                className='flex-1'
                              />
                            )}
                            <Button type='button' onClick={addModel} size='sm'>
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

                          {/* Supported models display - limited to 3 with expand button */}
                          <div className='flex flex-wrap items-center gap-1'>
                            {displayedSupportedModels.map((model) => (
                              <Badge key={model} variant='secondary' className='text-xs'>
                                {model}
                                <ManualModelBadge isManual={isModelManual(model)} className='ml-1' />
                                <button type='button' onClick={() => removeModel(model)} className='hover:text-destructive ml-1'>
                                  <X size={12} />
                                </button>
                              </Badge>
                            ))}
                            {supportedModels.length > MAX_MODELS_DISPLAY && !supportedModelsExpanded && (
                              <Button
                                type='button'
                                variant='ghost'
                                size='sm'
                                className='h-6 px-2 text-xs'
                                onClick={() => {
                                  setShowSupportedModelsPanel(true)
                                  setShowFetchedModelsPanel(false)
                                  setShowApiKeysPanel(false)
                                }}
                              >
                                <ChevronRight className='mr-1 h-3 w-3' />
                                {t('channels.dialogs.fields.supportedModels.showMore', {
                                  count: supportedModels.length - MAX_MODELS_DISPLAY,
                                })}
                              </Button>
                            )}
                          </div>

                          {/* Auto sync checkbox */}
                          <div className='pt-3'>
                            <FormField
                              control={form.control}
                              name='autoSyncSupportedModels'
                              render={({ field }) => (
                                <FormItem className={`flex items-center gap-2 ${isCodexType || isClaudeCodeType ? 'opacity-60' : ''}`}>
                                  {wrapUnsupported(
                                    isCodexType || isClaudeCodeType,
                                    <Checkbox
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                      data-testid='auto-sync-supported-models-checkbox'
                                      disabled={isCodexType || isClaudeCodeType}
                                      className={isCodexType || isClaudeCodeType ? 'pointer-events-none' : undefined}
                                    />,
                                    'inline-flex items-center'
                                  )}
                                  <div className='space-y-0.5'>
                                    <FormLabel className='cursor-pointer text-sm font-normal'>
                                      {t('channels.dialogs.fields.autoSyncSupportedModels.label')}
                                    </FormLabel>
                                    <p className='text-muted-foreground text-xs'>
                                      {t('channels.dialogs.fields.autoSyncSupportedModels.description')}
                                    </p>
                                  </div>
                                </FormItem>
                              )}
                            />
                          </div>

                          {/* Quick add models section */}
                          <div className='pt-3'>
                            <div className='mb-2 flex items-center justify-between'>
                              <span className='text-sm font-medium'>{t('channels.dialogs.fields.supportedModels.defaultModelsLabel')}</span>
                              <div className='flex items-center gap-2'>
                                <Button
                                  type='button'
                                  onClick={handleFetchModels}
                                  size='sm'
                                  variant='outline'
                                  disabled={!canFetchModels() || fetchModels.isPending}
                                >
                                  <RefreshCw className={`mr-1 h-4 w-4 ${fetchModels.isPending ? 'animate-spin' : ''}`} />
                                  {t('channels.dialogs.buttons.fetchModels')}
                                </Button>
                                <Button
                                  type='button'
                                  onClick={addSelectedDefaultModels}
                                  size='sm'
                                  variant='outline'
                                  disabled={selectedDefaultModels.length === 0}
                                  data-testid='add-selected-models-button'
                                >
                                  <Plus className='mr-1 h-4 w-4' />
                                  {t('channels.dialogs.buttons.addSelected')}
                                </Button>
                              </div>
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              {quickModels.map((model: string) => (
                                <Badge
                                  key={model}
                                  variant={selectedDefaultModels.includes(model) ? 'default' : 'secondary'}
                                  className='cursor-pointer text-xs'
                                  onClick={() => toggleDefaultModel(model)}
                                  data-testid={`quick-model-${model}`}
                                >
                                  {model}
                                  {selectedDefaultModels.includes(model) && <span className='ml-1'>✓</span>}
                                </Badge>
                              ))}
                            </div>
                          </div>
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
              className='border-border flex min-h-0 flex-col overflow-hidden border-l pl-4 transition-all duration-300 ease-out'
              style={{
                width: showFetchedModelsPanel || showSupportedModelsPanel || showApiKeysPanel ? '400px' : '0px',
                opacity: showFetchedModelsPanel || showSupportedModelsPanel || showApiKeysPanel ? 1 : 0,
                paddingLeft: showFetchedModelsPanel || showSupportedModelsPanel || showApiKeysPanel ? '16px' : '0px',
              }}
            >
              {/* Fetched Models Panel Content */}
              <div
                className={`flex h-full min-h-0 flex-col transition-opacity duration-200 ${showFetchedModelsPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
              >
                <div className='mb-3 flex items-center justify-between'>
                  <h3 className='text-sm font-semibold'>{t('channels.dialogs.fields.supportedModels.fetchedModelsLabel')}</h3>
                  <Button type='button' variant='ghost' size='sm' onClick={closeFetchedModelsPanel}>
                    <ChevronLeft className='h-4 w-4' />
                  </Button>
                </div>

                {/* Search */}
                <div className='relative mb-3'>
                  <Search className='text-muted-foreground absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2' />
                  <Input
                    placeholder={t('channels.dialogs.fields.supportedModels.searchPlaceholder')}
                    value={fetchedModelsSearch}
                    onChange={(e) => setFetchedModelsSearch(e.target.value)}
                    className='h-8 pl-8 text-sm'
                  />
                </div>

                {/* Filter and Actions */}
                <div className='mb-3 flex items-center justify-between gap-2'>
                  <label className='flex cursor-pointer items-center gap-2 text-xs'>
                    <Checkbox checked={showNotAddedModelsOnly} onCheckedChange={(checked) => setShowNotAddedModelsOnly(checked === true)} />
                    {t('channels.dialogs.fields.supportedModels.showNotAddedOnly')}
                  </label>
                  <div className='flex gap-1'>
                    <Button type='button' variant='outline' size='sm' className='h-6 px-2 text-xs' onClick={selectAllFilteredModels}>
                      {t('channels.dialogs.buttons.selectAll')}
                    </Button>
                    <Button type='button' variant='outline' size='sm' className='h-6 px-2 text-xs' onClick={deselectAllFetchedModels}>
                      {t('channels.dialogs.buttons.deselectAll')}
                    </Button>
                  </div>
                </div>

                {/* Model List */}
                <ScrollArea className='min-h-0 flex-1' type='always'>
                  <div className='space-y-1 pr-3'>
                    {filteredFetchedModels.map((model) => {
                      const isAdded = supportedModels.includes(model);
                      const isSelected = selectedFetchedModels.includes(model);
                      return (
                        <div
                          key={model}
                          className={`flex items-center gap-2 rounded-md p-2 text-sm transition-colors ${
                            isAdded && !isSelected
                              ? 'bg-muted/50 text-muted-foreground'
                              : isSelected
                                ? 'bg-primary/10 border-primary/30 border'
                                : 'hover:bg-accent cursor-pointer'
                          }`}
                        >
                          <Checkbox checked={isSelected} onCheckedChange={() => toggleFetchedModelSelection(model)} />
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span
                                className='max-w-[200px] flex-1 cursor-pointer truncate'
                                onClick={() => toggleFetchedModelSelection(model)}
                              >
                                {model}
                              </span>
                            </TooltipTrigger>
                            <TooltipContent>
                              <p className='max-w-xs break-all'>{model}</p>
                            </TooltipContent>
                          </Tooltip>
                          {isAdded && !isSelected && (
                            <Badge variant='secondary' className='shrink-0 text-xs'>
                              {t('channels.dialogs.fields.supportedModels.added')}
                            </Badge>
                          )}
                          {isAdded && isSelected && (
                            <Badge variant='destructive' className='shrink-0 text-xs'>
                              {t('channels.dialogs.fields.supportedModels.willRemove')}
                            </Badge>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </ScrollArea>

                {/* Action Buttons */}
                <div className='mt-2 flex gap-2 border-t pt-2'>
                  <Button
                    type='button'
                    className='flex-1'
                    size='sm'
                    onClick={addSelectedFetchedModels}
                    disabled={selectedFetchedModels.length === 0}
                  >
                    {selectedFetchedModels.some((model) => supportedModels.includes(model))
                      ? t('channels.dialogs.buttons.confirmSelection')
                      : t('channels.dialogs.buttons.addSelectedCount', { count: selectedFetchedModels.length })}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    className='flex-1'
                    size='sm'
                    onClick={removeDeprecatedModels}
                    disabled={deprecatedModelsCount === 0}
                  >
                    <Trash2 className='mr-1 h-4 w-4' />
                    {t('channels.dialogs.buttons.removeDeprecated', { count: deprecatedModelsCount })}
                  </Button>
                </div>
              </div>

              {showFetchedModelsPanel && showSupportedModelsPanel && <div className='border-border my-2 border-t' />}

              {/* API Keys Panel Content */}
              <div
                className={`flex h-full min-h-0 flex-col transition-opacity duration-200 ${showApiKeysPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
              >
                <div className='mb-3 flex items-center justify-between'>
                  <h3 className='text-sm font-semibold'>{t('channels.dialogs.fields.apiKey.panelTitle', { count: apiKeysCount })}</h3>
                  <Button type='button' variant='ghost' size='sm' onClick={closeApiKeysPanel}>
                    <ChevronLeft className='h-4 w-4' />
                  </Button>
                </div>

                <p className='text-muted-foreground mb-3 text-xs'>{t('channels.dialogs.fields.apiKey.panelDescription')}</p>

                {/* Search */}
                <div className='relative mb-3'>
                  <Search className='text-muted-foreground absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2' />
                  <Input
                    placeholder={t('channels.dialogs.fields.apiKey.searchPlaceholder')}
                    value={apiKeysSearch}
                    onChange={(e) => setApiKeysSearch(e.target.value)}
                    className='h-8 pl-8 text-sm'
                  />
                </div>

                {/* Keys List */}
                <ScrollArea className='min-h-0 flex-1' type='always'>
                  <div className='space-y-1 pr-3'>
                    {(() => {
                      const validKeys = (apiKeys || []).map((k) => k.trim()).filter((k) => k.length > 0);
                      const isLastKey = validKeys.length <= 1;
                      return validKeys
                      .filter((k) => {
                        if (!apiKeysSearch.trim()) return true;
                        const search = apiKeysSearch.trim().toLowerCase();
                        return k.toLowerCase().includes(search) || k.slice(-4).toLowerCase().includes(search);
                      })
                      .map((key) => {
                        const isSelected = selectedKeysToRemove.has(key);
                        const isDisabled = disabledKeySet.has(key);
                        const masked = key.length > 8 ? `${key.slice(0, 4)}****${key.slice(-4)}` : `****${key.slice(-4)}`;

                        return (
                          <div key={key} className='hover:bg-accent flex items-center justify-between gap-2 rounded-md p-2 text-sm'>
                            <div className='flex min-w-0 items-center gap-2'>
                              <Checkbox
                                checked={isSelected}
                                disabled={isLastKey}
                                onCheckedChange={(checked) => {
                                  setSelectedKeysToRemove((prev) => {
                                    const next = new Set(prev);
                                    if (checked) {
                                      next.add(key);
                                    } else {
                                      next.delete(key);
                                    }
                                    return next;
                                  });
                                }}
                                aria-label={t('common.columns.selectRow')}
                              />

                              <div className='min-w-0'>
                                <div className='flex min-w-0 items-center gap-2'>
                                  <code className='bg-muted shrink-0 rounded px-2 py-0.5 font-mono text-xs'>{masked}</code>
                                  {isDisabled && (
                                    <Badge variant='destructive' className='h-5 px-2 text-[10px]'>
                                      {t('channels.dialogs.fields.apiKey.disabled')}
                                    </Badge>
                                  )}
                                </div>
                                {isDisabled && (
                                  <p className='text-muted-foreground mt-1 text-xs'>{t('channels.dialogs.fields.apiKey.disabledHint')}</p>
                                )}
                              </div>
                            </div>

                            {isLastKey ? (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <span className='inline-flex'>
                                    <Button type='button' variant='ghost' size='sm' className='text-muted-foreground h-7 w-7 p-0' disabled>
                                      <Trash2 className='h-4 w-4' />
                                    </Button>
                                  </span>
                                </TooltipTrigger>
                                <TooltipContent>
                                  <p>{t('channels.dialogs.fields.apiKey.mustKeepOne')}</p>
                                </TooltipContent>
                              </Tooltip>
                            ) : (
                              <Popover open={confirmRemoveKey === key} onOpenChange={(isOpen) => setConfirmRemoveKey(isOpen ? key : null)}>
                                <PopoverTrigger asChild>
                                  <Button type='button' variant='ghost' size='sm' className='text-destructive h-7 w-7 p-0'>
                                    <Trash2 className='h-4 w-4' />
                                  </Button>
                                </PopoverTrigger>
                                <PopoverContent className='w-72'>
                                  <div className='flex flex-col gap-3'>
                                    <p className='text-sm'>{t('channels.dialogs.fields.apiKey.confirmRemoveSingle')}</p>
                                    <div className='flex justify-end gap-2'>
                                      <Button size='sm' variant='outline' onClick={() => setConfirmRemoveKey(null)}>
                                        {t('common.buttons.cancel')}
                                      </Button>
                                      <Button size='sm' variant='destructive' onClick={() => removeApiKeys([key])}>
                                        {t('common.buttons.confirm')}
                                      </Button>
                                    </div>
                                  </div>
                                </PopoverContent>
                              </Popover>
                            )}
                          </div>
                        );
                      });
                    })()}
                  </div>
                </ScrollArea>

                {/* Action Buttons */}
                <div className='mt-2 flex gap-2 border-t pt-2'>
                  <Popover open={confirmRemoveSelectedOpen} onOpenChange={setConfirmRemoveSelectedOpen}>
                    <PopoverTrigger asChild>
                      <Button type='button' variant='destructive' size='sm' className='flex-1' disabled={selectedKeysToRemove.size === 0}>
                        <Trash2 className='mr-1 h-4 w-4' />
                        {t('channels.dialogs.fields.apiKey.removeSelected', { count: selectedKeysToRemove.size })}
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent className='w-80' align='end'>
                      <div className='flex flex-col gap-3'>
                        <p className='text-sm'>
                          {t('channels.dialogs.fields.apiKey.confirmRemoveSelected', { count: selectedKeysToRemove.size })}
                        </p>
                        <div className='flex justify-end gap-2'>
                          <Button size='sm' variant='outline' onClick={() => setConfirmRemoveSelectedOpen(false)}>
                            {t('common.buttons.cancel')}
                          </Button>
                          <Button size='sm' variant='destructive' onClick={() => removeApiKeys(Array.from(selectedKeysToRemove))}>
                            {t('common.buttons.confirm')}
                          </Button>
                        </div>
                      </div>
                    </PopoverContent>
                  </Popover>

                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='flex-1'
                    onClick={() => {
                      setSelectedKeysToRemove(new Set());
                      setConfirmRemoveSelectedOpen(false);
                      setConfirmRemoveKey(null);
                    }}
                    disabled={selectedKeysToRemove.size === 0}
                  >
                    {t('channels.dialogs.fields.apiKey.clearSelection')}
                  </Button>
                </div>
              </div>

              {/* Supported Models Panel Content */}
              <div
                className={`flex h-full min-h-0 flex-col transition-opacity duration-200 ${showSupportedModelsPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
              >
                <div className='mb-3 flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <Button type='button' variant='ghost' size='sm' className='h-6 w-6 p-0' onClick={closeSupportedModelsPanel}>
                      <PanelLeft className='h-4 w-4' />
                    </Button>
                    <h3 className='text-sm font-semibold'>
                      {manualModels.length > 0
                        ? t('channels.dialogs.fields.supportedModels.allModelsWithManual', {
                            autoCount: Math.max(0, supportedModels.length - manualModels.length),
                            manualCount: manualModels.length,
                          })
                        : t('channels.dialogs.fields.supportedModels.allModels', { count: supportedModels.length })}
                    </h3>
                  </div>
                  <Popover open={showClearAllPopover} onOpenChange={setShowClearAllPopover}>
                    <PopoverTrigger asChild>
                      <Button type='button' variant='ghost' size='sm' disabled={supportedModels.length === 0}>
                        <X className='h-4 w-4' />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent className='border-destructive/50 bg-background w-80' align='end'>
                      <div className='space-y-3'>
                        <div className='space-y-1'>
                          <h4 className='leading-none font-medium'>{t('channels.dialogs.fields.supportedModels.clearAllTitle')}</h4>
                          <p className='text-muted-foreground text-sm'>
                            {t('channels.dialogs.fields.supportedModels.clearAllDescription', { count: supportedModels.length })}
                          </p>
                        </div>
                        <div className='flex justify-end gap-2'>
                          <Button type='button' variant='ghost' size='sm' onClick={() => setShowClearAllPopover(false)}>
                            {t('common.buttons.cancel')}
                          </Button>
                          <Button
                            type='button'
                            variant='destructive'
                            size='sm'
                            onClick={() => {
                              handleClearAllSupportedModels();
                              setShowClearAllPopover(false);
                            }}
                          >
                            {t('channels.dialogs.buttons.clearAll')}
                          </Button>
                        </div>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>

                {/* Search */}
                <div className='relative mb-3'>
                  <Search className='text-muted-foreground absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2' />
                  <Input
                    placeholder={t('channels.dialogs.fields.supportedModels.searchPlaceholder')}
                    value={supportedModelsSearch}
                    onChange={(e) => setSupportedModelsSearch(e.target.value)}
                    className='h-8 pl-8 text-sm'
                  />
                </div>

                {/* Model List */}
                <ScrollArea className='min-h-0 flex-1' type='always'>
                  <div className='space-y-1 pr-3'>
                    {filteredSupportedModels.map((model) => (
                      <div key={model} className='hover:bg-accent flex items-center gap-2 rounded-md p-2 text-sm'>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className='w-0 flex-1 cursor-help truncate'>{model}</span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p className='max-w-xs break-all'>{model}</p>
                          </TooltipContent>
                        </Tooltip>
                        <ManualModelBadge isManual={isModelManual(model)} />
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          className='hover:text-destructive h-6 w-6 shrink-0 p-0'
                          onClick={() => removeModel(model)}
                        >
                          <X className='h-3 w-3' />
                        </Button>
                      </div>
                    ))}
                  </div>
                </ScrollArea>
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
              disabled={createChannel.isPending || updateChannel.isPending || supportedModels.length === 0}
              data-testid='channel-submit-button'
            >
              {createChannel.isPending || updateChannel.isPending
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
