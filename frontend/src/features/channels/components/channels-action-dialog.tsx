'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { X, RefreshCw, Search, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Form, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import { AutoCompleteSelect } from '@/components/auto-complete-select'
import { SelectDropdown } from '@/components/select-dropdown'
import { useCreateChannel, useUpdateChannel, useFetchModels, useBulkCreateChannels } from '../data/channels'
import { getDefaultBaseURL, getDefaultModels, CHANNEL_CONFIGS, OPENAI_CHAT_COMPLETIONS } from '../data/config_channels'
import {
  PROVIDER_CONFIGS,
  getProviderFromChannelType,
  getApiFormatsForProvider,
  getChannelTypeForApiFormat,
} from '../data/config_providers'
import { Channel, ChannelType, ApiFormat, createChannelInputSchema, updateChannelInputSchema } from '../data/schema'

interface Props {
  currentRow?: Channel
  open: boolean
  onOpenChange: (open: boolean) => void
  showModelsPanel?: boolean
}

export function ChannelsActionDialog({ currentRow, open, onOpenChange, showModelsPanel = false }: Props) {
  const { t } = useTranslation()
  const isEdit = !!currentRow
  const createChannel = useCreateChannel()
  const bulkCreateChannels = useBulkCreateChannels()
  const updateChannel = useUpdateChannel()
  const fetchModels = useFetchModels()
  const [supportedModels, setSupportedModels] = useState<string[]>(currentRow?.supportedModels || [])
  const [newModel, setNewModel] = useState('')
  const [selectedDefaultModels, setSelectedDefaultModels] = useState<string[]>([])
  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [useFetchedModels, setUseFetchedModels] = useState(false)
  const providerRefs = useRef<Record<string, HTMLDivElement | null>>({})

  // Expandable panel states
  const [showFetchedModelsPanel, setShowFetchedModelsPanel] = useState(false)
  const [showSupportedModelsPanel, setShowSupportedModelsPanel] = useState(false)
  const [fetchedModelsSearch, setFetchedModelsSearch] = useState('')
  const [selectedFetchedModels, setSelectedFetchedModels] = useState<string[]>([])
  const [showAddedModelsOnly, setShowAddedModelsOnly] = useState(false)
  const [supportedModelsExpanded, setSupportedModelsExpanded] = useState(false)

  // Provider-based selection state
  const [selectedProvider, setSelectedProvider] = useState<string>(() => {
    if (currentRow) {
      return getProviderFromChannelType(currentRow.type) || 'openai'
    }
    return 'openai'
  })
  const [selectedApiFormat, setSelectedApiFormat] = useState<ApiFormat>(() => {
    if (currentRow) {
      return CHANNEL_CONFIGS[currentRow.type]?.apiFormat || 'openai/chat_completions'
    }
    return 'openai/chat_completions'
  })

  useEffect(() => {
    if (!isEdit || !currentRow) return

    const provider = getProviderFromChannelType(currentRow.type) || 'openai'
    setSelectedProvider(provider)
    const apiFormat = CHANNEL_CONFIGS[currentRow.type]?.apiFormat || OPENAI_CHAT_COMPLETIONS
    setSelectedApiFormat(apiFormat)
  }, [isEdit, currentRow])

  useEffect(() => {
    if (!open || !isEdit) return

    const frame = requestAnimationFrame(() => {
      const target = providerRefs.current[selectedProvider]
      target?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    })

    return () => cancelAnimationFrame(frame)
  }, [open, isEdit, selectedProvider])

  // Auto-open supported models panel when showModelsPanel is true
  useEffect(() => {
    if (open && showModelsPanel && isEdit && currentRow && currentRow.supportedModels.length > 0) {
      setShowSupportedModelsPanel(true)
    }
  }, [open, showModelsPanel, isEdit, currentRow])

  // Get available providers (excluding fake types)
  const availableProviders = useMemo(
    () =>
      Object.entries(PROVIDER_CONFIGS)
        .filter(([, config]) => {
          // Filter out providers that only have fake types
          const nonFakeTypes = config.channelTypes.filter((t) => !t.endsWith('_fake'))
          return nonFakeTypes.length > 0
        })
        .map(([key, config]) => ({
          key,
          label: t(`channels.providers.${key}`),
          icon: config.icon,
          channelTypes: config.channelTypes.filter((t) => !t.endsWith('_fake')),
        })),
    [t]
  )

  // Get available API formats for selected provider
  const availableApiFormats = useMemo(() => {
    return getApiFormatsForProvider(selectedProvider)
  }, [selectedProvider])

  const getApiFormatLabel = useCallback(
    (format: ApiFormat) => {
      // Convert 'openai/chat_completions' to 'openaiChatCompletions'
      const formatKey = format
        .split('/')
        .map((part, index) => (index === 0 ? part : part.charAt(0).toUpperCase() + part.slice(1)))
        .join('')
      return t(`channels.dialogs.fields.apiFormat.formats.${formatKey}`)
    },
    [t]
  )

  // Determine the actual channel type based on provider and API format
  const derivedChannelType = useMemo(() => {
    if (isEdit && currentRow) {
      return currentRow.type
    }
    return getChannelTypeForApiFormat(selectedProvider, selectedApiFormat) || 'openai'
  }, [isEdit, currentRow, selectedProvider, selectedApiFormat])

  const baseURLPlaceholder = useMemo(() => {
    const currentType = selectedType || derivedChannelType
    const defaultURL = getDefaultBaseURL(currentType)
    if (defaultURL) {
      return defaultURL
    }
    return t('channels.dialogs.fields.baseURL.placeholder')
  }, [selectedType, derivedChannelType, t])

  const formSchema = isEdit ? updateChannelInputSchema : createChannelInputSchema

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues:
      isEdit && currentRow
        ? {
            type: currentRow.type,
            baseURL: currentRow.baseURL,
            name: currentRow.name,
            supportedModels: currentRow.supportedModels,
            defaultTestModel: currentRow.defaultTestModel,
            credentials: {
              apiKey: '', // credentials字段是敏感字段，不从API返回
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
          }
        : {
            type: derivedChannelType,
            baseURL: getDefaultBaseURL(derivedChannelType),
            name: '',
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
          },
  })

  const selectedType = form.watch('type') as ChannelType | undefined

  // Sync form type when provider or API format changes (only for create mode)
  const handleProviderChange = useCallback(
    (provider: string) => {
      if (isEdit) return
      setSelectedProvider(provider)
      const formats = getApiFormatsForProvider(provider)
      // Default to first available format
      const newFormat = formats[0] || 'openai/chat_completions'
      setSelectedApiFormat(newFormat)
      const newChannelType = getChannelTypeForApiFormat(provider, newFormat)
      if (newChannelType) {
        form.setValue('type', newChannelType)
        const baseURL = getDefaultBaseURL(newChannelType)
        if (baseURL) {
          form.setValue('baseURL', baseURL)
        }
        // Reset models when provider changes
        setSupportedModels([])
        setFetchedModels([])
        setUseFetchedModels(false)
      }
    },
    [isEdit, form]
  )

  const handleApiFormatChange = useCallback(
    (format: ApiFormat) => {
      if (isEdit) return
      setSelectedApiFormat(format)
      const newChannelType = getChannelTypeForApiFormat(selectedProvider, format)
      if (newChannelType) {
        form.setValue('type', newChannelType)
        const baseURL = getDefaultBaseURL(newChannelType)
        if (baseURL) {
          form.setValue('baseURL', baseURL)
        }
        // Reset models when API format changes
        setSupportedModels([])
        setFetchedModels([])
        setUseFetchedModels(false)
      }
    },
    [isEdit, selectedProvider, form]
  )

  useEffect(() => {
    if (isEdit) return
    if (!availableApiFormats.includes(selectedApiFormat)) {
      const fallbackFormat = availableApiFormats[0] || OPENAI_CHAT_COMPLETIONS
      handleApiFormatChange(fallbackFormat)
    }
  }, [availableApiFormats, selectedApiFormat, handleApiFormatChange, isEdit])

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    try {
      const dataWithModels = {
        ...values,
        supportedModels,
      }

      if (isEdit && currentRow) {
        // For edit mode, only include credentials if user actually entered new values
        const updateInput = {
          ...dataWithModels,
          // type 不能更新
          type: undefined,
        }

        // Check if any credential fields have actual values
        const hasApiKey = values.credentials?.apiKey && values.credentials.apiKey.trim() !== ''
        const hasAwsCredentials =
          values.credentials?.aws?.accessKeyID &&
          values.credentials.aws.accessKeyID.trim() !== '' &&
          values.credentials?.aws?.secretAccessKey &&
          values.credentials.aws.secretAccessKey.trim() !== '' &&
          values.credentials?.aws?.region &&
          values.credentials.aws.region.trim() !== ''
        const hasGcpCredentials =
          values.credentials?.gcp?.region &&
          values.credentials.gcp.region.trim() !== '' &&
          values.credentials?.gcp?.projectID &&
          values.credentials.gcp.projectID.trim() !== '' &&
          values.credentials?.gcp?.jsonData &&
          values.credentials.gcp.jsonData.trim() !== ''

        // Only include credentials if user provided new values
        if (!hasApiKey && !hasAwsCredentials && !hasGcpCredentials) {
          delete updateInput.credentials
        }

        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: updateInput,
        })
      } else {
        // For create mode, check if multiple API keys are provided
        const apiKeys =
          values.credentials?.apiKey
            ?.split('\n')
            .map((key) => key.trim())
            .filter((key) => key.length > 0) || []

        if (apiKeys.length > 1) {
          // Bulk create: use bulk mutation
          await bulkCreateChannels.mutateAsync({
            type: values.type as string,
            name: values.name as string,
            baseURL: values.baseURL,
            apiKeys: apiKeys,
            supportedModels: supportedModels,
            defaultTestModel: values.defaultTestModel as string,
            settings: values.settings,
          })
        } else {
          // Single create: use existing mutation
          await createChannel.mutateAsync(dataWithModels as any)
        }
      }

      form.reset()
      setSupportedModels([])
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to save channel:', error)
    }
  }

  const addModel = () => {
    if (newModel.trim() && !supportedModels.includes(newModel.trim())) {
      setSupportedModels([...supportedModels, newModel.trim()])
      setNewModel('')
    }
  }

  const removeModel = (model: string) => {
    setSupportedModels(supportedModels.filter((m) => m !== model))
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      addModel()
    }
  }

  const toggleDefaultModel = (model: string) => {
    setSelectedDefaultModels((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]))
  }

  const addSelectedDefaultModels = () => {
    const newModels = selectedDefaultModels.filter((model) => !supportedModels.includes(model))
    if (newModels.length > 0) {
      setSupportedModels((prev) => [...prev, ...newModels])
      setSelectedDefaultModels([])
    }
  }

  const handleFetchModels = useCallback(async () => {
    const channelType = form.getValues('type')
    const baseURL = form.getValues('baseURL')
    const apiKey = form.getValues('credentials.apiKey')

    if (!channelType || !baseURL) {
      return
    }

    try {
      // Only use the first API key when multiple keys are provided
      const firstApiKey = isEdit ? undefined : (apiKey?.split('\n').map(key => key.trim()).filter(key => key.length > 0)[0] || '')

      const result = await fetchModels.mutateAsync({
        channelType,
        baseURL,
        apiKey: firstApiKey,
        channelID: isEdit ? currentRow?.id : undefined,
      })

      if (result.error) {
        toast.error(result.error)
        return
      }

      const models = result.models.map((m) => m.id)
      if (models?.length) {
        setFetchedModels(models)
        setUseFetchedModels(true)
        setShowFetchedModelsPanel(true)
        setSelectedFetchedModels([])
        setFetchedModelsSearch('')
        setShowAddedModelsOnly(false)
      }
    } catch (error) {
      // Error is already handled by the mutation
    }
  }, [fetchModels, form, isEdit, currentRow])

  const canFetchModels = () => {
    const baseURL = form.watch('baseURL')
    const apiKey = form.watch('credentials.apiKey')

    if (isEdit) {
      return !!baseURL
    }

    return !!baseURL && !!apiKey
  }

  // Memoize quick models to avoid re-evaluating on every render
  const currentType = form.watch('type')
  const quickModels = useMemo(() => {
    if (useFetchedModels || !currentType) return []
    return getDefaultModels(currentType)
  }, [currentType, useFetchedModels])

  // Filtered fetched models based on search and filter
  const filteredFetchedModels = useMemo(() => {
    let models = fetchedModels
    if (showAddedModelsOnly) {
      models = models.filter((model) => supportedModels.includes(model))
    }
    if (fetchedModelsSearch.trim()) {
      const search = fetchedModelsSearch.toLowerCase()
      models = models.filter((model) => model.toLowerCase().includes(search))
    }
    return models
  }, [fetchedModels, fetchedModelsSearch, showAddedModelsOnly, supportedModels])

  // Toggle selection for fetched model
  const toggleFetchedModelSelection = useCallback((model: string) => {
    setSelectedFetchedModels((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]))
  }, [])

  // Select all filtered models
  const selectAllFilteredModels = useCallback(() => {
    setSelectedFetchedModels(filteredFetchedModels)
  }, [filteredFetchedModels])

  // Deselect all
  const deselectAllFetchedModels = useCallback(() => {
    setSelectedFetchedModels([])
  }, [])

  // Add or remove selected fetched models to supported models
  const addSelectedFetchedModels = useCallback(() => {
    setSupportedModels((prev) => {
      const modelsToAdd: string[] = []
      const modelsToRemove: string[] = []

      selectedFetchedModels.forEach((model) => {
        if (prev.includes(model)) {
          modelsToRemove.push(model)
        } else {
          modelsToAdd.push(model)
        }
      })

      const afterRemoval = prev.filter((m) => !modelsToRemove.includes(m))
      return [...afterRemoval, ...modelsToAdd]
    })

    setSelectedFetchedModels([])
  }, [selectedFetchedModels])

  // Close panel handler
  const closeFetchedModelsPanel = useCallback(() => {
    setShowFetchedModelsPanel(false)
    setSelectedFetchedModels([])
    setFetchedModelsSearch('')
    setShowAddedModelsOnly(false)
  }, [])

  // Close supported models panel handler
  const closeSupportedModelsPanel = useCallback(() => {
    setShowSupportedModelsPanel(false)
  }, [])

  // Models to display (limited to 5 unless expanded)
  const displayedSupportedModels = useMemo(() => {
    if (supportedModels.length <= 5) {
      return supportedModels
    }
    return supportedModels.slice(0, 5)
  }, [supportedModels])

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
          setSupportedModels(currentRow?.supportedModels || [])
          setSelectedDefaultModels([])
          setFetchedModels([])
          setUseFetchedModels(false)
          // Reset expandable panel states
          setShowFetchedModelsPanel(false)
          setShowSupportedModelsPanel(false)
          setFetchedModelsSearch('')
          setSelectedFetchedModels([])
          setShowAddedModelsOnly(false)
          setSupportedModelsExpanded(false)
          // Reset provider and API format state
          if (currentRow) {
            setSelectedProvider(getProviderFromChannelType(currentRow.type) || 'openai')
            setSelectedApiFormat(CHANNEL_CONFIGS[currentRow.type]?.apiFormat || OPENAI_CHAT_COMPLETIONS)
          } else {
            setSelectedProvider('openai')
            setSelectedApiFormat(OPENAI_CHAT_COMPLETIONS)
          }
        }
        onOpenChange(state)
      }}
    >
      <DialogContent
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
            className={`min-h-0 flex-1 overflow-y-auto py-1 pr-4 transition-all duration-300 ${showFetchedModelsPanel || showSupportedModelsPanel ? '-mr-2' : '-mr-4'}`}
          >
            <Form {...form}>
              <form id='channel-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-6 p-0.5'>
                {/* Provider Selection - Left Side */}
                <div className='flex gap-6'>
                  <div className='w-80 flex-shrink-0'>
                    <FormItem className='space-y-2'>
                      <FormLabel className='text-base font-semibold'>{t('channels.dialogs.fields.provider.label')}</FormLabel>
                      <div className={`max-h-[500px] overflow-y-auto pr-2 ${isEdit ? 'cursor-not-allowed opacity-60' : ''}`}>
                        <RadioGroup value={selectedProvider} onValueChange={handleProviderChange} disabled={isEdit} className='space-y-2'>
                          {availableProviders.map((provider) => {
                            const Icon = provider.icon
                            const isSelected = provider.key === selectedProvider
                            return (
                              <div
                                key={provider.key}
                                ref={(el) => {
                                  providerRefs.current[provider.key] = el
                                }}
                                className={`flex items-center space-x-3 rounded-lg border p-3 transition-colors ${
                                  isEdit
                                    ? isSelected
                                      ? 'border-primary bg-muted/80 cursor-not-allowed shadow-sm'
                                      : 'cursor-not-allowed opacity-60'
                                    : `${isSelected ? 'border-primary bg-accent/40 shadow-sm' : ''} hover:bg-accent/50`
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
                            )
                          })}
                        </RadioGroup>
                      </div>
                    </FormItem>
                    {/* Hidden field to keep form type in sync */}
                    <FormField control={form.control} name='type' render={() => <input type='hidden' />} />
                  </div>

                  {/* Right Side - Form Fields */}
                  <div className='flex-1 space-y-6'>
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
                      </div>
                    </FormItem>

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
                          </div>
                          <FormMessage />
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
                              autoComplete='off'
                              aria-invalid={!!fieldState.error}
                              data-testid='channel-base-url-input'
                              {...field}
                            />
                          </div>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    {selectedType !== 'anthropic_aws' && selectedType !== 'anthropic_gcp' && (
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
                                <Input
                                  type='password'
                                  placeholder={t('channels.dialogs.fields.apiKey.editPlaceholder')}
                                  className='col-span-6'
                                  autoComplete='off'
                                  aria-invalid={!!fieldState.error}
                                  data-testid='channel-api-key-input'
                                  {...field}
                                />
                              ) : (
                                <>
                                  <Textarea
                                    placeholder={t('channels.dialogs.fields.apiKey.placeholder')}
                                    className='col-span-6 min-h-[80px] resize-y font-mono text-sm'
                                    autoComplete='off'
                                    aria-invalid={!!fieldState.error}
                                    data-testid='channel-api-key-input'
                                    {...field}
                                  />
                                  <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.apiKey.multiLineHint')}</p>
                                </>
                              )}
                            </div>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    )}

                    {selectedType === 'anthropic_aws' && (
                      <>
                        <FormField
                          control={form.control}
                          name='credentials.aws.accessKeyID'
                          render={({ field, fieldState }) => (
                            <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                              <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                {t('channels.dialogs.fields.awsAccessKeyID.label')}
                              </FormLabel>
                              <div className='col-span-6 space-y-1'>
                                <Input
                                  type='password'
                                  placeholder={t('channels.dialogs.fields.awsAccessKeyID.placeholder')}
                                  className='col-span-6'
                                  autoComplete='off'
                                  aria-invalid={!!fieldState.error}
                                  {...field}
                                />
                              </div>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name='credentials.aws.secretAccessKey'
                          render={({ field, fieldState }) => (
                            <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                              <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                {t('channels.dialogs.fields.awsSecretAccessKey.label')}
                              </FormLabel>
                              <div className='col-span-6 space-y-1'>
                                <Input
                                  type='password'
                                  placeholder={t('channels.dialogs.fields.awsSecretAccessKey.placeholder')}
                                  className='col-span-6'
                                  autoComplete='off'
                                  aria-invalid={!!fieldState.error}
                                  {...field}
                                />
                              </div>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name='credentials.aws.region'
                          render={({ field, fieldState }) => (
                            <FormItem className='grid grid-cols-8 items-start gap-x-6'>
                              <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                                {t('channels.dialogs.fields.awsRegion.label')}
                              </FormLabel>
                              <div className='col-span-6 space-y-1'>
                                <Input
                                  placeholder={t('channels.dialogs.fields.awsRegion.placeholder')}
                                  className='col-span-6'
                                  autoComplete='off'
                                  aria-invalid={!!fieldState.error}
                                  {...field}
                                />
                              </div>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      </>
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
                              </div>
                              <FormMessage />
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
                              </div>
                              <FormMessage />
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
                                  className='col-span-6 min-h-[200px] resize-y font-mono text-xs'
                                  aria-invalid={!!fieldState.error}
                                  {...field}
                                />
                              </div>
                              <FormMessage />
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
                        </div>

                        {/* Supported models display - limited to 5 with expand button */}
                        <div className='flex flex-wrap items-center gap-1'>
                          {displayedSupportedModels.map((model) => (
                            <Badge key={model} variant='secondary' className='text-xs'>
                              {model}
                              <button type='button' onClick={() => removeModel(model)} className='hover:text-destructive ml-1'>
                                <X size={12} />
                              </button>
                            </Badge>
                          ))}
                          {supportedModels.length > 5 && !supportedModelsExpanded && (
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              className='h-6 px-2 text-xs'
                              onClick={() => setShowSupportedModelsPanel(true)}
                            >
                              <ChevronRight className='mr-1 h-3 w-3' />
                              {t('channels.dialogs.fields.supportedModels.showMore', { count: supportedModels.length - 5 })}
                            </Button>
                          )}
                        </div>

                        {/* Quick add models section */}
                        {quickModels.length > 0 && (
                          <div className='pt-3'>
                            <div className='mb-2 flex items-center justify-between'>
                              <span className='text-sm font-medium'>{t('channels.dialogs.fields.supportedModels.defaultModelsLabel')}</span>
                              <Button
                                type='button'
                                onClick={addSelectedDefaultModels}
                                size='sm'
                                variant='outline'
                                disabled={selectedDefaultModels.length === 0}
                              >
                                {t('channels.dialogs.buttons.addSelected')}
                              </Button>
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              {quickModels.map((model: string) => (
                                <Badge
                                  key={model}
                                  variant={selectedDefaultModels.includes(model) ? 'default' : 'secondary'}
                                  className='cursor-pointer text-xs'
                                  onClick={() => toggleDefaultModel(model)}
                                >
                                  {model}
                                  {selectedDefaultModels.includes(model) && <span className='ml-1'>✓</span>}
                                </Badge>
                              ))}
                            </div>
                          </div>
                        )}

                        {supportedModels.length === 0 && (
                          <p className='text-muted-foreground text-sm'>{t('channels.dialogs.fields.supportedModels.required')}</p>
                        )}
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
                          </div>
                          <FormMessage />
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
            className='border-border flex flex-col overflow-hidden border-l pl-4 transition-all duration-300 ease-out'
            style={{
              width: showFetchedModelsPanel || showSupportedModelsPanel ? '320px' : '0px',
              opacity: showFetchedModelsPanel || showSupportedModelsPanel ? 1 : 0,
              paddingLeft: showFetchedModelsPanel || showSupportedModelsPanel ? '16px' : '0px',
            }}
          >
            {/* Fetched Models Panel Content */}
            <div
              className={`flex h-full flex-col transition-opacity duration-200 ${showFetchedModelsPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
            >
              <div className='mb-3 flex items-center justify-between'>
                <h3 className='text-sm font-semibold'>{t('channels.dialogs.fields.supportedModels.fetchedModelsLabel')}</h3>
                <Button type='button' variant='ghost' size='sm' onClick={closeFetchedModelsPanel}>
                  <X className='h-4 w-4' />
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
                  <Checkbox checked={showAddedModelsOnly} onCheckedChange={(checked) => setShowAddedModelsOnly(checked === true)} />
                  {t('channels.dialogs.fields.supportedModels.showAddedOnly')}
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
              <div className='-mr-2 max-h-[400px] flex-1 overflow-y-auto pr-2'>
                <div className='space-y-1'>
                  {filteredFetchedModels.map((model) => {
                    const isAdded = supportedModels.includes(model)
                    const isSelected = selectedFetchedModels.includes(model)
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
                        <span className='flex-1 cursor-pointer truncate' onClick={() => toggleFetchedModelSelection(model)}>
                          {model}
                        </span>
                        {isAdded && !isSelected && (
                          <Badge variant='secondary' className='text-xs'>
                            {t('channels.dialogs.fields.supportedModels.added')}
                          </Badge>
                        )}
                        {isAdded && isSelected && (
                          <Badge variant='destructive' className='text-xs'>
                            {t('channels.dialogs.fields.supportedModels.willRemove')}
                          </Badge>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Add Selected Button */}
              <div className='mt-3 border-t pt-3'>
                <Button
                  type='button'
                  className='w-full'
                  size='sm'
                  onClick={addSelectedFetchedModels}
                  disabled={selectedFetchedModels.length === 0}
                >
                  {selectedFetchedModels.some((model) => supportedModels.includes(model))
                    ? t('channels.dialogs.buttons.confirmSelection')
                    : t('channels.dialogs.buttons.addSelectedCount', { count: selectedFetchedModels.length })}
                </Button>
              </div>
            </div>

            {/* Supported Models Panel Content */}
            <div
              className={`flex h-full flex-col transition-opacity duration-200 ${showSupportedModelsPanel ? 'opacity-100' : 'pointer-events-none absolute opacity-0'}`}
            >
              <div className='mb-3 flex items-center justify-between'>
                <h3 className='text-sm font-semibold'>
                  {t('channels.dialogs.fields.supportedModels.allModels', { count: supportedModels.length })}
                </h3>
                <Button type='button' variant='ghost' size='sm' onClick={closeSupportedModelsPanel}>
                  <X className='h-4 w-4' />
                </Button>
              </div>

              {/* Model List */}
              <div className='-mr-2 max-h-[400px] flex-1 overflow-y-auto pr-2'>
                <div className='space-y-1'>
                  {supportedModels.map((model) => (
                    <div key={model} className='hover:bg-accent flex items-center justify-between gap-2 rounded-md p-2 text-sm'>
                      <span className='flex-1 truncate'>{model}</span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='hover:text-destructive h-6 w-6 p-0'
                        onClick={() => removeModel(model)}
                      >
                        <X className='h-3 w-3' />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
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
  )
}
