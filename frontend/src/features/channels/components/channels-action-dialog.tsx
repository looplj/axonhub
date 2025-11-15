'use client'

import { useCallback, useMemo, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { X, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { AutoCompleteSelect } from '@/components/auto-complete-select'
import { SelectDropdown } from '@/components/select-dropdown'
import { useCreateChannel, useUpdateChannel, useFetchModels } from '../data/channels'
import { getDefaultBaseURL, getDefaultModels } from '../data/constants'
import { CHANNEL_CONFIGS } from '../data/constants'
import { Channel, ChannelType, createChannelInputSchema, updateChannelInputSchema } from '../data/schema'

interface Props {
  currentRow?: Channel
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ChannelsActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const isEdit = !!currentRow
  const createChannel = useCreateChannel()
  const updateChannel = useUpdateChannel()
  const fetchModels = useFetchModels()
  const [supportedModels, setSupportedModels] = useState<string[]>(currentRow?.supportedModels || [])
  const [newModel, setNewModel] = useState('')
  const [selectedDefaultModels, setSelectedDefaultModels] = useState<string[]>([])
  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [useFetchedModels, setUseFetchedModels] = useState(false)

  const channelTypes = useMemo(
    () =>
      Object.keys(CHANNEL_CONFIGS).map((type) => ({
        value: type,
        label: t(`channels.types.${type}`),
      })),
    [t]
  )

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
            baseURL: getDefaultBaseURL('openai'),
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
  const selectedChannelConfig = selectedType ? CHANNEL_CONFIGS[selectedType] : undefined
  const selectedApiFormat = selectedChannelConfig?.apiFormat

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

  const handleFetchModels = useCallback(async () => {
    const channelType = form.getValues('type')
    const baseURL = form.getValues('baseURL')
    const apiKey = form.getValues('credentials.apiKey')

    if (!channelType || !baseURL) {
      return
    }

    try {
      const result = await fetchModels.mutateAsync({
        channelType,
        baseURL,
        apiKey: isEdit ? undefined : apiKey,
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
                render={({ field }) => (
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
                          if (!isEdit) {
                            const baseURL = getDefaultBaseURL(value as any)
                            if (baseURL) {
                              form.setValue('baseURL', baseURL)
                            }
                          }
                        }}
                        items={availableChannelTypes}
                        placeholder={t('channels.dialogs.fields.type.description')}
                        className='col-span-6'
                        isControlled={true}
                        disabled={isEdit}
                      />
                    </FormControl>
                    {selectedApiFormat && (
                      <p className='text-muted-foreground col-span-6 col-start-3 mt-2 flex items-center gap-2 text-xs'>
                        <span className='text-foreground font-medium'>
                          {t('channels.dialogs.fields.apiFormat.label')}
                        </span>
                        <Badge variant='outline'>{selectedApiFormat}</Badge>
                      </p>
                    )}
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

              {selectedType !== 'anthropic_aws' && selectedType !== 'anthropic_gcp' && (
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

              {selectedType === 'anthropic_aws' && (
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

              {selectedType === 'anthropic_gcp' && (
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

                  {useFetchedModels && fetchedModels.length > 100 && (
                    <p className='text-muted-foreground text-sm'>
                      {t('channels.dialogs.fields.supportedModels.largeListHint', { count: fetchedModels.length })}
                    </p>
                  )}

                  {/* Quick add models section */}
                  {(() => {
                    const currentType = form.watch('type')
                    const quickModels = useFetchedModels
                      ? fetchedModels
                      : currentType
                        ? getDefaultModels(currentType)
                        : []
                    return quickModels && quickModels.length > 0 && !useFetchedModels
                  })() && (
                    <div className='pt-3'>
                      <div className='mb-2 flex items-center justify-between'>
                        <span className='text-sm font-medium'>
                          {t('channels.dialogs.fields.supportedModels.defaultModelsLabel')}
                        </span>
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
                        {((): string[] => {
                          const currentType = form.watch('type')
                          return currentType ? getDefaultModels(currentType) : []
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

                  {/* Fetched models section - only show when models are fetched and <= 20 */}
                  {useFetchedModels && fetchedModels.length > 0 && fetchedModels.length <= 20 && (
                    <div className='pt-3'>
                      <div className='mb-2 flex items-center justify-between'>
                        <span className='text-sm font-medium'>
                          {t('channels.dialogs.fields.supportedModels.fetchedModelsLabel')}
                        </span>
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
                        {fetchedModels.map((model: string) => (
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
                render={({ field }) => (
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
