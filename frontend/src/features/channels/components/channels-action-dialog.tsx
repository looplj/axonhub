'use client'

import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { SelectDropdown } from '@/components/select-dropdown'
import { useCreateChannel, useUpdateChannel } from '../data/channels'
import { Channel, createChannelInputSchema, updateChannelInputSchema } from '../data/schema'

interface Props {
  currentRow?: Channel
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Default base URLs for different channel types
const defaultBaseUrls: Record<string, string> = {
  openai: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com/v1',
  anthropic_aws: 'https://bedrock-runtime.us-east-1.amazonaws.com',
  anthropic_gcp: 'https://us-east5-aiplatform.googleapis.com',
  gemini_openai: 'https://generativelanguage.googleapis.com/v1beta/openai',
  deepseek: 'https://api.deepseek.com/v1',
  doubao: 'https://ark.cn-beijing.volces.com/api/v3',
  kimi: 'https://api.moonshot.cn/v1',
  zhipu: 'https://open.bigmodel.cn/api/paas/v4',
  zai: 'https://open.bigmodel.cn/api/paas/v4',
  deepseek_anthropic: 'https://api.deepseek.com/anthropic',
  kimi_anthropic: 'https://api.moonshot.cn/anthropic',
  zhipu_anthropic: 'https://open.bigmodel.cn/api/anthropic',
  zai_anthropic: 'https://open.bigmodel.cn/api/anthropic',
  openrouter: 'https://openrouter.ai/api/v1',
  // Fake types are not allowed for creation
  // anthropic_fake: 'https://api.anthropic.com/v1',
  // openai_fake: 'https://api.openai.com/v1',
}

// Default models for different channel types
export const defaultModels: Record<string, string[]> = {
  openai: ['gpt-3.5-turbo', `gpt-4.5`, 'gpt-4.1', 'gpt-4-turbo', 'gpt-4o', 'gpt-4o-mini', 'gpt-5'],
  anthropic: [
    'claude-opus-4-1',
    'claude-opus-4-0',
    'claude-sonnet-4-0',
    'claude-3-7-sonnet-latest',
    'claude-3-5-haiku-latest',
  ],
  anthropic_aws: [
    'anthropic.claude-opus-4-1-20250805-v1:0',
    'anthropic.claude-opus-4-20250514-v1:0',
    'anthropic.claude-sonnet-4-20250514-v1:0',
    'anthropic.claude-3-7-sonnet-20250219-v1:0',
    'anthropic.claude-3-5-haiku-20241022-v1:0',
  ],
  anthropic_gcp: [
    'claude-opus-4-1@20250805',
    'claude-opus-4@20250514',
    'claude-sonnet-4@20250514',
    'claude-3-7-sonnet@20250219',
    'claude-3-5-haiku@20241022',
  ],
  gemini_openai: ['gemini-2.5-pro', 'gemini-2.5-flash'],
  deepseek: ['deepseek-chat', 'deepseek-reasoner'],
  doubao: ['doubao-seed-1.6', 'doubao-seed-1.6-flash'],
  moonshot: ['kimi-k2-0711-preview', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
  zhipu: ['glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  zai: ['glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  deepseek_anthropic: ['deepseek-chat', 'deepseek-reasoner'],
  moonshot_anthropic: ['kimi-k2-0711-preview', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
  zhipu_anthropic: ['glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  zai_anthropic: ['glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  anthropic_fake: [
    'claude-opus-4-1',
    'claude-opus-4-0',
    'claude-sonnet-4-0',
    'claude-3-7-sonnet-latest',
    'claude-3-5-haiku-latest',
  ],
  openai_fake: ['gpt-3.5-turbo', `gpt-4.5`, 'gpt-4.1', 'gpt-4-turbo', 'gpt-4o', 'gpt-4o-mini', 'gpt-5'],
  openrouter: [
    // DeepSeek
    'deepseek/deepseek-chat-v3.1:free',
    'deepseek/deepseek-chat-v3.1',
    'deepseek/deepseek-r1-0528:free',
    'deepseek/deepseek-r1-0528',
    'deepseek/deepseek-r1:free',
    'deepseek/deepseek-r1',
    'deepseek/deepseek-chat-v3-0324:free',
    'deepseek/deepseek-chat-v3-0324',

    // Moonshot
    'moonshotai/kimi-k2:free',
    'moonshotai/kimi-k2-0905',

    // Zai
    'z-ai/glm-4.5',
    'z-ai/glm-4.5-air',
    'z-ai/glm-4.5-air:free',

    // Google
    "google/gemini-2.5-flash-lite",
    "google/gemini-2.5-flash",
    "google/gemini-2.5-pro",
    
    // Anthropic
    "anthropic/claude-opus-4",
    "anthropic/claude-sonnet-4",
    "anthropic/claude-3.7-sonnet",
  ],
}

export function ChannelsActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const isEdit = !!currentRow
  const createChannel = useCreateChannel()
  const updateChannel = useUpdateChannel()
  const [supportedModels, setSupportedModels] = useState<string[]>(currentRow?.supportedModels || [])
  const [newModel, setNewModel] = useState('')
  const [selectedDefaultModels, setSelectedDefaultModels] = useState<string[]>([])

  const channelTypes = [
    { value: 'openai', label: t('channels.types.openai') },
    { value: 'anthropic', label: t('channels.types.anthropic') },
    { value: 'anthropic_aws', label: t('channels.types.anthropic_aws') },
    { value: 'anthropic_gcp', label: t('channels.types.anthropic_gcp') },
    { value: 'gemini_openai', label: t('channels.types.gemini_openai') },
    { value: 'deepseek', label: t('channels.types.deepseek') },
    { value: 'doubao', label: t('channels.types.doubao') },
    { value: 'moonshot', label: t('channels.types.moonshot') },
    { value: 'zhipu', label: t('channels.types.zhipu') },
    { value: 'zai', label: t('channels.types.zai') },
    { value: 'deepseek_anthropic', label: t('channels.types.deepseek_anthropic') },
    { value: 'moonshot_anthropic', label: t('channels.types.moonshot_anthropic') },
    { value: 'zhipu_anthropic', label: t('channels.types.zhipu_anthropic') },
    { value: 'zai_anthropic', label: t('channels.types.zai_anthropic') },
    { value: 'openrouter', label: t('channels.types.openrouter') },
    { value: 'anthropic_fake', label: t('channels.types.anthropic_fake') },
    { value: 'openai_fake', label: t('channels.types.openai_fake') },
  ]

  // Filter out fake types for new channels, but keep them for editing existing channels
  const availableChannelTypes = isEdit ? channelTypes : channelTypes.filter((type) => !type.value.endsWith('_fake'))

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
            type: 'openai',
            baseURL: defaultBaseUrls.openai,
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

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    try {
      const dataWithModels = {
        ...values,
        supportedModels,
      }

      if (isEdit && currentRow) {
        // For edit mode, only include credentials if user actually entered new values
        const updateInput = { ...dataWithModels }

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
        await createChannel.mutateAsync(dataWithModels as any)
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

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
          setSupportedModels(currentRow?.supportedModels || [])
          setSelectedDefaultModels([])
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='max-h-[90vh] sm:max-w-4xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{isEdit ? t('channels.dialogs.edit.title') : t('channels.dialogs.create.title')}</DialogTitle>
          <DialogDescription>
            {isEdit ? t('channels.dialogs.edit.description') : t('channels.dialogs.create.description')}
          </DialogDescription>
        </DialogHeader>
        <div className='-mr-4 h-[36rem] w-full overflow-y-auto py-1 pr-4'>
          <Form {...form}>
            <form id='channel-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 p-0.5'>
              <FormField
                control={form.control}
                name='type'
                disabled={isEdit}
                render={({ field, fieldState }) => (
                  <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                    <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                      {t('channels.dialogs.fields.type.label')}
                    </FormLabel>
                    <FormControl>
                      <SelectDropdown
                        defaultValue={field.value}
                        onValueChange={(value) => {
                          field.onChange(value)
                          // Auto-fill base URL when type changes (only for new channels)
                          if (!isEdit && defaultBaseUrls[value]) {
                            form.setValue('baseURL', defaultBaseUrls[value])
                          }
                        }}
                        items={availableChannelTypes}
                        placeholder={t('channels.dialogs.fields.type.description')}
                        className='col-span-6'
                        isControlled={true}
                        disabled={isEdit}
                      />
                    </FormControl>
                    <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field, fieldState }) => (
                  <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                    <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                      {t('channels.dialogs.fields.name.label')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('channels.dialogs.fields.name.placeholder')}
                        className='col-span-6'
                        autoComplete='off'
                        aria-invalid={!!fieldState.error}
                        {...field}
                      />
                    </FormControl>
                    <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='baseURL'
                render={({ field, fieldState }) => (
                  <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                    <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                      {t('channels.dialogs.fields.baseURL.label')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('channels.dialogs.fields.baseURL.placeholder')}
                        className='col-span-6'
                        autoComplete='off'
                        aria-invalid={!!fieldState.error}
                        {...field}
                      />
                    </FormControl>
                    <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />

              {form.watch('type') !== 'anthropic_aws' && form.watch('type') !== 'anthropic_gcp' && (
                <FormField
                  control={form.control}
                  name='credentials.apiKey'
                  render={({ field, fieldState }) => (
                    <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                      <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                        {t('channels.dialogs.fields.apiKey.label')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={
                            isEdit
                              ? t('channels.dialogs.fields.apiKey.editPlaceholder')
                              : t('channels.dialogs.fields.apiKey.placeholder')
                          }
                          className='col-span-6'
                          autoComplete='off'
                          aria-invalid={!!fieldState.error}
                          {...field}
                        />
                      </FormControl>
                      <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                        <FormMessage />
                      </div>
                    </FormItem>
                  )}
                />
              )}

              {form.watch('type') === 'anthropic_aws' && (
                <>
                  <FormField
                    control={form.control}
                    name='credentials.aws.accessKeyID'
                    render={({ field, fieldState }) => (
                      <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.awsAccessKeyID.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder={t('channels.dialogs.fields.awsAccessKeyID.placeholder')}
                            className='col-span-6'
                            autoComplete='off'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.aws.secretAccessKey'
                    render={({ field, fieldState }) => (
                      <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.awsSecretAccessKey.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder={t('channels.dialogs.fields.awsSecretAccessKey.placeholder')}
                            className='col-span-6'
                            autoComplete='off'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.aws.region'
                    render={({ field, fieldState }) => (
                      <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.awsRegion.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.dialogs.fields.awsRegion.placeholder')}
                            className='col-span-6'
                            autoComplete='off'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />
                </>
              )}

              {form.watch('type') === 'anthropic_gcp' && (
                <>
                  <FormField
                    control={form.control}
                    name='credentials.gcp.region'
                    render={({ field, fieldState }) => (
                      <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.gcpRegion.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.dialogs.fields.gcpRegion.placeholder')}
                            className='col-span-6'
                            autoComplete='off'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.gcp.projectID'
                    render={({ field, fieldState }) => (
                      <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.gcpProjectID.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.dialogs.fields.gcpProjectID.placeholder')}
                            className='col-span-6'
                            autoComplete='off'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.gcp.jsonData'
                    render={({ field, fieldState }) => (
                      <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                        <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                          {t('channels.dialogs.fields.gcpJsonData.label')}
                        </FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder={`{\n  "type": "service_account",\n  "project_id": "project-123",\n  "private_key_id": "fdfd",\n  "private_key": "-----BEGIN PRIVATE KEY-----\\n-----END PRIVATE KEY-----\\n",\n  "client_email": "xxx@developer.gserviceaccount.com",\n  "client_id": "client_213123123",\n  "auth_uri": "https://accounts.google.com/o/oauth2/auth",\n  "token_uri": "https://oauth2.googleapis.com/token",\n  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",\n  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/xxx-compute%40developer.gserviceaccount.com",\n  "universe_domain": "googleapis.com"\n}`}
                            className='col-span-6 min-h-[200px] resize-y font-mono text-xs'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />
                </>
              )}

              <div className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                  {t('channels.dialogs.fields.supportedModels.label')}
                </FormLabel>
                <div className='col-span-6 space-y-2'>
                  <div className='flex gap-2'>
                    <Input
                      placeholder={t('channels.dialogs.fields.supportedModels.description')}
                      value={newModel}
                      onChange={(e) => setNewModel(e.target.value)}
                      onKeyPress={handleKeyPress}
                      className='flex-1'
                    />
                    <Button type='button' onClick={addModel} size='sm'>
                      {t('channels.dialogs.buttons.add')}
                    </Button>
                  </div>

                  <div className='flex flex-wrap gap-1'>
                    {supportedModels.map((model) => (
                      <Badge key={model} variant='secondary' className='text-xs'>
                        {model}
                        <button
                          type='button'
                          onClick={() => removeModel(model)}
                          className='hover:text-destructive ml-1'
                        >
                          <X size={12} />
                        </button>
                      </Badge>
                    ))}
                  </div>

                  {/* Quick add models section */}
                  {(() => {
                    const currentType = form.watch('type') as keyof typeof defaultModels | undefined
                    const quickModels = currentType ? defaultModels[currentType] : undefined
                    return quickModels && quickModels.length > 0
                  })() && (
                    <div className='pt-3'>
                      <div className='mb-2 flex items-center justify-between'>
                        <span className='text-sm font-medium'>
                          {t('channels.dialogs.fields.supportedModels.defaultModelsLabel', 'Quick Add Models')}
                        </span>
                        <Button
                          type='button'
                          onClick={addSelectedDefaultModels}
                          size='sm'
                          variant='outline'
                          disabled={selectedDefaultModels.length === 0}
                        >
                          {t('channels.dialogs.buttons.addSelected', 'Add Selected')}
                        </Button>
                      </div>
                      <div className='flex flex-wrap gap-2'>
                        {((): string[] => {
                          const currentType = form.watch('type') as keyof typeof defaultModels | undefined
                          return currentType ? defaultModels[currentType] : []
                        })().map((model: string) => (
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
                    <p className='text-muted-foreground text-sm'>
                      {t('channels.dialogs.fields.supportedModels.required')}
                    </p>
                  )}
                </div>
              </div>

              <FormField
                control={form.control}
                name='defaultTestModel'
                render={({ field, fieldState }) => (
                  <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                    <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                      {t('channels.dialogs.fields.defaultTestModel.label')}
                    </FormLabel>
                    <FormControl>
                      <SelectDropdown
                        defaultValue={field.value}
                        onValueChange={field.onChange}
                        items={supportedModels.map((model) => ({ value: model, label: model }))}
                        placeholder={t('channels.dialogs.fields.defaultTestModel.description')}
                        className='col-span-6'
                        disabled={supportedModels.length === 0}
                        isControlled={true}
                      />
                    </FormControl>
                    <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />
            </form>
          </Form>
        </div>
        <DialogFooter>
          <Button
            type='submit'
            form='channel-form'
            disabled={createChannel.isPending || updateChannel.isPending || supportedModels.length === 0}
          >
            {createChannel.isPending || updateChannel.isPending
              ? isEdit
                ? t('channels.dialogs.buttons.updating')
                : t('channels.dialogs.buttons.creating')
              : isEdit
                ? t('channels.dialogs.buttons.update')
                : t('channels.dialogs.buttons.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
