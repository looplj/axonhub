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
import { Form, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { ScrollArea } from '@/components/ui/scroll-area';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { SelectDropdown } from '@/components/select-dropdown';
import {
  useCreateChannel,
  useUpdateChannel,
  useFetchModels,
  useBulkCreateChannels,
  useAllChannelNames,
  useAllChannelTags,
} from '../data/channels';
import { codexOAuthExchange, codexOAuthStart } from '../data/codex';
import { getDefaultBaseURL, getDefaultModels, CHANNEL_CONFIGS, OPENAI_CHAT_COMPLETIONS, OPENAI_RESPONSES } from '../data/config_channels';
import {
  PROVIDER_CONFIGS,
  getProviderFromChannelType,
  getApiFormatsForProvider,
  getChannelTypeForApiFormat,
} from '../data/config_providers';
import { Channel, ChannelType, ApiFormat, createChannelInputSchema, updateChannelInputSchema } from '../data/schema';

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
  const bulkCreateChannels = useBulkCreateChannels();
  const updateChannel = useUpdateChannel();
  const fetchModels = useFetchModels();
  const { data: allChannelNames = [], isSuccess: allChannelNamesLoaded } = useAllChannelNames({ enabled: open && isDuplicate });
  const { data: allTags = [], isLoading: isLoadingTags } = useAllChannelTags();
  const selectedProjectId = useSelectedProjectId();
  const [supportedModels, setSupportedModels] = useState<string[]>(() => initialRow?.supportedModels || []);
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
  const [showGcpJsonData, setShowGcpJsonData] = useState(false);
  const dialogContentRef = useRef<HTMLDivElement>(null);

  const [codexSessionId, setCodexSessionId] = useState<string | null>(null);
  const [codexAuthUrl, setCodexAuthUrl] = useState<string | null>(null);
  const [codexCallbackUrl, setCodexCallbackUrl] = useState('');
  const [isCodexStarting, setIsCodexStarting] = useState(false);
  const [isCodexExchanging, setIsCodexExchanging] = useState(false);

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
    if (!isEdit || !currentRow) return;

    const provider = getProviderFromChannelType(currentRow.type) || 'openai';
    setSelectedProvider(provider);
    const apiFormat = CHANNEL_CONFIGS[currentRow.type]?.apiFormat || OPENAI_CHAT_COMPLETIONS;
    setSelectedApiFormat(apiFormat);
    setUseGeminiVertex(currentRow.type === 'gemini_vertex');
    setUseAnthropicAws(currentRow.type === 'anthropic_aws');
  }, [isEdit, currentRow]);

  useEffect(() => {
    if (!open) {
      hasAutoSetDuplicateNameRef.current = false;
      setCodexSessionId(null);
      setCodexAuthUrl(null);
      setCodexCallbackUrl('');
      setIsCodexStarting(false);
      setIsCodexExchanging(false);
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
    if (open && showModelsPanel && isEdit && currentRow && currentRow.supportedModels.length > 0) {
      setShowSupportedModelsPanel(true);
    }
  }, [open, showModelsPanel, isEdit, currentRow]);

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
              apiKey: currentRow.credentials?.apiKey || '',
              aws: {
                accessKeyID: currentRow.credentials?.aws?.accessKeyID || '',
                secretAccessKey: currentRow.credentials?.aws?.secretAccessKey || '',
                region: currentRow.credentials?.aws?.region || '',
              },
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
                apiKey: duplicateFromRow.credentials?.apiKey || '',
                aws: {
                  accessKeyID: duplicateFromRow.credentials?.aws?.accessKeyID || '',
                  secretAccessKey: duplicateFromRow.credentials?.aws?.secretAccessKey || '',
                  region: duplicateFromRow.credentials?.aws?.region || '',
                },
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
                apiKey: '',
                aws: {
                  accessKeyID: '',
                  secretAccessKey: '',
                  region: '',
                },
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

  const baseURLPlaceholder = useMemo(() => {
    const currentType = selectedType || derivedChannelType;
    const defaultURL = getDefaultBaseURL(currentType);
    if (defaultURL) {
      return defaultURL;
    }
    return t('channels.dialogs.fields.baseURL.placeholder');
  }, [selectedType, derivedChannelType, t]);

  const wrapCodexUnsupported = useCallback(
    (enabled: boolean, children: React.ReactNode, wrapperClassName: string) => {
      if (!enabled) return children;
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className={wrapperClassName}>{children}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('channels.dialogs.fields.supportedModels.codexUnsupported')}</p>
          </TooltipContent>
        </Tooltip>
      );
    },
    [t]
  );

  // Sync form type when provider or API format changes (only for create mode)
  const handleProviderChange = useCallback(
    (provider: string) => {
      if (isEdit) return;
      setSelectedProvider(provider);

      if (provider !== 'gemini') {
        setUseGeminiVertex(false);
      }
      if (provider !== 'anthropic') {
        setUseAnthropicAws(false);
      }

      if (provider === 'codex') {
        setSelectedApiFormat(OPENAI_RESPONSES);
        form.setValue('type', 'codex');
        setFetchedModels([]);
        setUseFetchedModels(false);
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
      if (selectedProvider === 'codex') return;

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
    [isEdit, selectedApiFormat, form]
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
    [isEdit, selectedApiFormat, form]
  );

  const startCodexOAuth = useCallback(async () => {
    if (!selectedProjectId) {
      toast.error(t('channels.dialogs.codex.errors.projectRequired'));
      return;
    }

    setIsCodexStarting(true);
    try {
      const result = await codexOAuthStart({ 'X-Project-ID': selectedProjectId });
      setCodexSessionId(result.session_id);
      setCodexAuthUrl(result.auth_url);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setIsCodexStarting(false);
    }
  }, [selectedProjectId]);

  const exchangeCodexOAuth = useCallback(async () => {
    if (!selectedProjectId) {
      toast.error(t('channels.dialogs.codex.errors.projectRequired'));
      return;
    }

    if (!codexSessionId) {
      toast.error(t('channels.dialogs.codex.errors.sessionMissing'));
      return;
    }

    if (!codexCallbackUrl.trim()) {
      toast.error(t('channels.dialogs.codex.errors.callbackUrlRequired'));
      return;
    }

    setIsCodexExchanging(true);
    try {
      const result = await codexOAuthExchange(
        {
          session_id: codexSessionId,
          callback_url: codexCallbackUrl.trim(),
        },
        { 'X-Project-ID': selectedProjectId }
      );

      form.setValue('credentials.apiKey', result.credentials);
      toast.success(t('channels.dialogs.codex.messages.credentialsImported'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setIsCodexExchanging(false);
    }
  }, [selectedProjectId, codexSessionId, codexCallbackUrl, form]);

  useEffect(() => {
    if (isEdit) return;
    if (!isCodexType) {
      setCodexSessionId(null);
      setCodexAuthUrl(null);
      setCodexCallbackUrl('');
      return;
    }

    const baseURL = getDefaultBaseURL('codex');
    if (baseURL) {
      form.resetField('baseURL', { defaultValue: baseURL });
    }
  }, [isEdit, isCodexType, form]);

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    // Check if there are selected fetched models that haven't been confirmed
    if (selectedFetchedModels.length > 0) {
      toast.error(t('channels.messages.modelsNotConfirmed'));
      return;
    }

    try {
      const valuesForSubmit = isEdit
        ? values
        : {
            ...values,
            type: derivedChannelType,
          };

      const dataWithModels = {
        ...valuesForSubmit,
        supportedModels,
      };

      if (isCodexType) {
        const baseURL = getDefaultBaseURL('codex');
        if (baseURL) {
          dataWithModels.baseURL = baseURL;
        }
      }

      if (isEdit && currentRow) {
        // For edit mode, only include credentials if user actually entered new values
        const updateInput = {
          ...dataWithModels,
          // type 不能更新
          type: undefined,
        };

        // Check if any credential fields have actual values
        const hasApiKey = values.credentials?.apiKey && values.credentials.apiKey.trim() !== '';
        const hasAwsCredentials =
          values.credentials?.aws?.accessKeyID &&
          values.credentials.aws.accessKeyID.trim() !== '' &&
          values.credentials?.aws?.secretAccessKey &&
          values.credentials.aws.secretAccessKey.trim() !== '' &&
          values.credentials?.aws?.region &&
          values.credentials.aws.region.trim() !== '';
        const hasGcpCredentials =
          values.credentials?.gcp?.region &&
          values.credentials.gcp.region.trim() !== '' &&
          values.credentials?.gcp?.projectID &&
          values.credentials.gcp.projectID.trim() !== '' &&
          values.credentials?.gcp?.jsonData &&
          values.credentials.gcp.jsonData.trim() !== '';

        // Only include credentials if user provided new values
        if (!hasApiKey && !hasAwsCredentials && !hasGcpCredentials) {
          delete updateInput.credentials;
        }

        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: updateInput,
        });
      } else {
        // For create mode, check if multiple API keys are provided
        const apiKeys =
          valuesForSubmit.credentials?.apiKey
            ?.split('\n')
            .map((key) => key.trim())
            .filter((key) => key.length > 0) || [];

        if (apiKeys.length > 1) {
          if (isCodexType) {
      toast.error(t('channels.dialogs.codex.errors.bulkCreateUnsupported'));
      return;
          }

          const settings = values.settings ?? duplicateFromRow?.settings ?? undefined;
          const policies = values.policies ?? duplicateFromRow?.policies ?? undefined;
          // Bulk create: use bulk mutation
          await bulkCreateChannels.mutateAsync({
            type: valuesForSubmit.type as string,
            name: valuesForSubmit.name as string,
            baseURL: valuesForSubmit.baseURL,
            tags: valuesForSubmit.tags,
            apiKeys: apiKeys,
            supportedModels: supportedModels,
            autoSyncSupportedModels: valuesForSubmit.autoSyncSupportedModels,
            defaultTestModel: valuesForSubmit.defaultTestModel as string,
            settings,
            policies,
            orderingWeight: valuesForSubmit.orderingWeight ?? undefined,
            remark: valuesForSubmit.remark ?? undefined,
          });
        } else {
          // Single create: use existing mutation
          await createChannel.mutateAsync({
            ...(dataWithModels as z.infer<typeof createChannelInputSchema>),
            settings: values.settings ?? duplicateFromRow?.settings ?? undefined,
          });
        }
      }

      form.reset();
      setSupportedModels([]);
      onOpenChange(false);
    } catch (_error) {
      void _error;
    }
  };

  const addModel = () => {
    if (newModel.trim() && !supportedModels.includes(newModel.trim())) {
      setSupportedModels([...supportedModels, newModel.trim()]);
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
    setNewModel('');
  }, [newModel]);

  const removeModel = (model: string) => {
    setSupportedModels(supportedModels.filter((m) => m !== model));
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
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
  };

  const handleFetchModels = useCallback(async () => {
    const channelType = form.getValues('type');
    const baseURL = form.getValues('baseURL');
    const apiKey = form.getValues('credentials.apiKey');

    if (!channelType || !baseURL) {
      return;
    }

    try {
      // Extract first API key from potentially multi-line input
      const firstApiKey =
        apiKey
          ?.split('\n')
          .map((key) => key.trim())
          .filter((key) => key.length > 0)[0] || '';

      const result = await fetchModels.mutateAsync({
        channelType,
        baseURL,
        apiKey: isCodexType ? undefined : !isEdit ? firstApiKey : firstApiKey || undefined,
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
        setSelectedFetchedModels([]);
        setFetchedModelsSearch('');
        setShowNotAddedModelsOnly(false);
      }
    } catch (_error) {
      // Error is already handled by the mutation
    }
  }, [fetchModels, form, isEdit, currentRow, isCodexType]);

  const canFetchModels = () => {
    const baseURL = form.watch('baseURL');
    const apiKey = form.watch('credentials.apiKey');

    if (isCodexType) {
      return !!baseURL;
    }

    if (isEdit) {
      return !!baseURL;
    }

    return !!baseURL && !!apiKey;
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
    setSupportedModels((prev) => {
      const modelsToAdd: string[] = [];
      const modelsToRemove: string[] = [];

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

  // Remove deprecated models (models in supportedModels but not in fetchedModels)
  const removeDeprecatedModels = useCallback(() => {
    const fetchedModelsSet = new Set(fetchedModels);
    setSupportedModels((prev) => prev.filter((model) => fetchedModelsSet.has(model)));
  }, [fetchedModels]);

  // Count of deprecated models
  const deprecatedModelsCount = useMemo(() => {
    const fetchedModelsSet = new Set(fetchedModels);
    return supportedModels.filter((model) => !fetchedModelsSet.has(model)).length;
  }, [supportedModels, fetchedModels]);

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
            setSelectedDefaultModels([]);
            setFetchedModels([]);
            setUseFetchedModels(false);
            // Reset expandable panel states
            setShowFetchedModelsPanel(false);
            setShowSupportedModelsPanel(false);
            setFetchedModelsSearch('');
            setSupportedModelsSearch('');
            setSelectedFetchedModels([]);
            setShowNotAddedModelsOnly(false);
            setSupportedModelsExpanded(false);
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
          ref={dialogContentRef}
          className={`flex max-h-[90vh] flex-col transition-all duration-300 ${showFetchedModelsPanel || showSupportedModelsPanel ? 'sm:max-w-6xl' : 'sm:max-w-4xl'}`}
        >
          <DialogHeader className='flex-shrink-0 text-left'>
            <DialogTitle>{isEdit ? t('channels.dialogs.edit.title') : t('channels.dialogs.create.title')}</DialogTitle>
            <DialogDescription>
              {isEdit ? t('channels.dialogs.edit.description') : t('channels.dialogs.create.description')}
            </DialogDescription>
          </DialogHeader>
          <div className='flex min-h-0 flex-1 gap-4 overflow-hidden'>
            {/* Main Form Section */}
            <div
              className={`min-h-0 flex-1 flex flex-col overflow-hidden py-1 transition-all duration-300 ${showFetchedModelsPanel || showSupportedModelsPanel ? 'pr-2' : 'pr-0'}`}
            >
              <Form {...form}>
                <form id='channel-form' onSubmit={form.handleSubmit(onSubmit)} className='flex-1 flex flex-col min-h-0 space-y-6 p-0.5'>
                  {/* Provider Selection - Left Side */}
                  <div className='flex flex-1 min-h-0 gap-6 overflow-hidden'>
                    <div className='w-60 flex-shrink-0 flex flex-col min-h-0'>
                      <FormItem className='flex-1 flex flex-col min-h-0 space-y-2'>
                        <FormLabel className='text-base font-semibold'>{t('channels.dialogs.fields.provider.label')}</FormLabel>
                        <div ref={providerListRef} className={`flex-1 overflow-y-auto pr-2 ${isEdit ? 'cursor-not-allowed opacity-60' : ''}`}>
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
                    <div className='flex-1 overflow-y-auto space-y-6 pr-4'>
                      {selectedProvider !== 'jina' && selectedProvider !== 'codex' && (
                        <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                          <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                            {t('channels.dialogs.fields.apiFormat.label')}
                          </FormLabel>
                          <div className='col-span-6 space-y-1'>
                            <SelectDropdown
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
                        <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                          <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                            {t('channels.dialogs.fields.apiFormat.label')}
                          </FormLabel>
                          <div className='col-span-6 space-y-1'>
                            <div className='text-sm'>{getApiFormatLabel(OPENAI_RESPONSES)}</div>
                            <p className='text-muted-foreground mt-1 text-xs'>{t('channels.dialogs.fields.apiFormat.editDisabled')}</p>

                            <div className='mt-3 space-y-2'>
                              <div className='rounded-md border p-3'>
                                <div className='flex flex-wrap items-center gap-2'>
                                  <Button type='button' variant='secondary' onClick={startCodexOAuth} disabled={isCodexStarting}>
                                    {isCodexStarting
                                      ? t('channels.dialogs.codex.buttons.starting')
                                      : t('channels.dialogs.codex.buttons.startOAuth')}
                                  </Button>
                                  {codexAuthUrl && (
                                    <Button
                                      type='button'
                                      variant='ghost'
                                      onClick={() => window.open(codexAuthUrl, '_blank', 'noopener,noreferrer')}
                                    >
                                      {t('channels.dialogs.codex.buttons.openOAuthLink')}
                                    </Button>
                                  )}
                                </div>

                                {codexAuthUrl && (
                                  <div className='mt-3 space-y-2'>
                                    <FormLabel className='text-sm font-medium'>
                                      {t('channels.dialogs.codex.labels.authorizationUrl')}
                                    </FormLabel>
                                    <Textarea
                                      value={codexAuthUrl}
                                      readOnly
                                      className='min-h-[60px] resize-none font-mono text-xs'
                                      placeholder={t('channels.dialogs.codex.placeholders.authorizationUrl')}
                                    />
                                  </div>
                                )}

                                <div className='mt-3 space-y-2'>
                                  <FormLabel className='text-sm font-medium'>{t('channels.dialogs.codex.labels.callbackUrl')}</FormLabel>
                                  <Textarea
                                    value={codexCallbackUrl}
                                    onChange={(e) => setCodexCallbackUrl(e.target.value)}
                                    placeholder={t('channels.dialogs.codex.placeholders.callbackUrl')}
                                    className='min-h-[80px] resize-y font-mono text-xs'
                                  />
                                  <Button type='button' onClick={exchangeCodexOAuth} disabled={isCodexExchanging || !codexSessionId}>
                                    {isCodexExchanging
                                      ? t('channels.dialogs.codex.buttons.exchanging')
                                      : t('channels.dialogs.codex.buttons.exchangeAndFillApiKey')}
                                  </Button>
                                </div>

                                <p className='text-muted-foreground mt-2 text-xs'>
                                  {t('channels.dialogs.fields.apiFormat.codex.description')}
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
                          <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                            <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                              {t('channels.dialogs.fields.name.label')}
                            </FormLabel>
                            <div className='col-span-6 space-y-1'>
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

                      <FormField
                        control={form.control}
                        name='baseURL'
                        render={({ field, fieldState }) => (
                          <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                            <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                              {t('channels.dialogs.fields.baseURL.label')}
                            </FormLabel>
                            <div className='col-span-6 space-y-1'>
                              <Input
                                placeholder={baseURLPlaceholder}
                                autoComplete='new-password'
                                data-form-type='other'
                                aria-invalid={!!fieldState.error}
                                data-testid='channel-base-url-input'
                                disabled={isCodexType}
                                {...field}
                              />
                              <FormMessage />
                            </div>
                          </FormItem>
                        )}
                      />


                      {!isCodexType && selectedType !== 'anthropic_gcp' && (
                        <FormField
                          control={form.control}
                          name='credentials.apiKey'
                          render={({ field, fieldState }) => (
                            <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                              <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                {t('channels.dialogs.fields.apiKey.label')}
                              </FormLabel>
                              <div className='col-span-6 space-y-1'>
                                {isEdit ? (
                                  <div className='relative'>
                                    <Input
                                      type={showApiKey ? 'text' : 'password'}
                                      placeholder={t('channels.dialogs.fields.apiKey.editPlaceholder')}
                                      className='col-span-6 pr-20'
                                      autoComplete='new-password'
                                      data-form-type='other'
                                      aria-invalid={!!fieldState.error}
                                      data-testid='channel-api-key-input'
                                      {...field}
                                    />
                                    <div className='absolute top-1/2 right-1 flex -translate-y-1/2 gap-1'>
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='sm'
                                        className='h-7 w-7 p-0'
                                        onClick={() => setShowApiKey(!showApiKey)}
                                      >
                                        {showApiKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                                      </Button>
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='sm'
                                        className='h-7 w-7 p-0'
                                        onClick={() => {
                                          if (field.value) {
                                            navigator.clipboard.writeText(field.value);
                                            toast.success(t('channels.messages.credentialsCopied'));
                                          }
                                        }}
                                      >
                                        <Copy className='h-4 w-4' />
                                      </Button>
                                    </div>
                                  </div>
                                ) : (
                                  <>
                                    <Textarea
                                      placeholder={t('channels.dialogs.fields.apiKey.placeholder')}
                                      className='col-span-6 min-h-[80px] resize-y font-mono text-sm'
                                      autoComplete='new-password'
                                      data-form-type='other'
                                      aria-invalid={!!fieldState.error}
                                      data-testid='channel-api-key-input'
                                      {...field}
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

                      {selectedType === 'anthropic_gcp' && (
                        <>
                          <FormField
                            control={form.control}
                            name='credentials.gcp.region'
                            render={({ field, fieldState }) => (
                              <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                                <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                  {t('channels.dialogs.fields.gcpRegion.label')}
                                </FormLabel>
                                <div className='col-span-6 space-y-1'>
                                  <Input
                                    placeholder={t('channels.dialogs.fields.gcpRegion.placeholder')}
                                    className='col-span-6'
                                    autoComplete='off'
                                    aria-invalid={!!fieldState.error}
                                    {...field}
                                  />
                                  <FormMessage />
                                </div>
                              </FormItem>
                            )}
                          />

                          <FormField
                            control={form.control}
                            name='credentials.gcp.projectID'
                            render={({ field, fieldState }) => (
                              <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                                <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                  {t('channels.dialogs.fields.gcpProjectID.label')}
                                </FormLabel>
                                <div className='col-span-6 space-y-1'>
                                  <Input
                                    placeholder={t('channels.dialogs.fields.gcpProjectID.placeholder')}
                                    className='col-span-6'
                                    autoComplete='off'
                                    aria-invalid={!!fieldState.error}
                                    {...field}
                                  />
                                  <FormMessage />
                                </div>
                              </FormItem>
                            )}
                          />

                          <FormField
                            control={form.control}
                            name='credentials.gcp.jsonData'
                            render={({ field, fieldState }) => (
                              <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                                <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                  {t('channels.dialogs.fields.gcpJsonData.label')}
                                </FormLabel>
                                <div className='col-span-6 space-y-1'>
                                  <div className='relative'>
                                    <Textarea
                                      placeholder={`{
  "type": "service_account",
  "project_id": "project-123",
  "private_key_id": "fdfd",
  "private_key": "-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----\n",
  "client_email": "xxx@developer.gserviceaccount.com",
  "client_id": "client_213123123",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/xxx-compute%40developer.gserviceaccount.com",
  "universe_domain": "googleapis.com"
}`}
                                      className='col-span-6 min-h-[200px] resize-y pr-10 font-mono text-xs'
                                      aria-invalid={!!fieldState.error}
                                      {...field}
                                    />
                                    {isEdit && field.value && (
                                      <div className='absolute top-1 right-1 flex flex-col gap-1'>
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='sm'
                                          className='h-7 w-7 p-0'
                                          onClick={() => setShowGcpJsonData(!showGcpJsonData)}
                                        >
                                          {showGcpJsonData ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                                        </Button>
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='sm'
                                          className='h-7 w-7 p-0'
                                          onClick={() => {
                                            if (field.value) {
                                              navigator.clipboard.writeText(field.value);
                                              toast.success(t('common.copied'));
                                            }
                                          }}
                                        >
                                          <Copy className='h-4 w-4' />
                                        </Button>
                                      </div>
                                    )}
                                  </div>
                                  <FormMessage />
                                </div>
                              </FormItem>
                            )}
                          />
                        </>
                      )}

                      <div className='grid grid-cols-8 items-start gap-x-6'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.supportedModels.label')}
                        </FormLabel>
                        <div className='col-span-6 space-y-2'>
                          <div className='flex gap-2'>
                            {useFetchedModels && fetchedModels.length > 20 ? (
                              <AutoCompleteSelect
                                items={fetchedModels.map((model) => ({ value: model, label: model }))}
                                selectedValue={newModel}
                                onSelectedValueChange={setNewModel}
                                placeholder={t('channels.dialogs.fields.supportedModels.description')}
                                portalContainer={dialogContentRef.current}
                              />
                            ) : (
                              <Input
                                placeholder={t('channels.dialogs.fields.supportedModels.description')}
                                value={newModel}
                                onChange={(e) => setNewModel(e.target.value)}
                                onKeyPress={handleKeyPress}
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
                                onClick={() => setShowSupportedModelsPanel(true)}
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
                                <FormItem className={`flex items-center gap-2 ${isCodexType ? 'opacity-60' : ''}`}>
                                  {wrapCodexUnsupported(
                                    isCodexType,
                                    <Checkbox
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                      data-testid='auto-sync-supported-models-checkbox'
                                      disabled={isCodexType}
                                      className={isCodexType ? 'pointer-events-none' : undefined}
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
                                {wrapCodexUnsupported(
                                  isCodexType,
                                  <Button
                                    type='button'
                                    onClick={handleFetchModels}
                                    size='sm'
                                    variant='outline'
                                    disabled={!canFetchModels() || fetchModels.isPending || isCodexType}
                                  >
                                    <RefreshCw className={`mr-1 h-4 w-4 ${fetchModels.isPending ? 'animate-spin' : ''}`} />
                                    {t('channels.dialogs.buttons.fetchModels')}
                                  </Button>,
                                  'inline-flex w-fit'
                                )}
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
                          <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                            <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                              {t('channels.dialogs.fields.defaultTestModel.label')}
                            </FormLabel>
                            <div className='col-span-6 space-y-1'>
                              <SelectDropdown
                                defaultValue={field.value}
                                onValueChange={field.onChange}
                                items={supportedModels.map((model) => ({ value: model, label: model }))}
                                placeholder={t('channels.dialogs.fields.defaultTestModel.description')}
                                className='col-span-6'
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
                        name='policies.stream'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                            <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                              {t('channels.dialogs.fields.streamPolicy.label')}
                            </FormLabel>
                            <div className='col-span-6 space-y-1'>
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

                      <FormField
                        control={form.control}
                        name='tags'
                        render={({ field }) => (
                          <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                            <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                              {t('channels.dialogs.fields.tags.label')}
                            </FormLabel>
                            <div className='col-span-6 space-y-1'>
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
                          <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                            <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                              {t('channels.dialogs.fields.remark.label')}
                            </FormLabel>
                            <div className='col-span-6 space-y-1'>
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
                width: showFetchedModelsPanel || showSupportedModelsPanel ? '400px' : '0px',
                opacity: showFetchedModelsPanel || showSupportedModelsPanel ? 1 : 0,
                paddingLeft: showFetchedModelsPanel || showSupportedModelsPanel ? '16px' : '0px',
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
                      {t('channels.dialogs.fields.supportedModels.allModels', { count: supportedModels.length })}
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
