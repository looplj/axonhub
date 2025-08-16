'use client'

import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { SelectDropdown } from '@/components/select-dropdown'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { X } from 'lucide-react'
import { Channel, createChannelInputSchema, updateChannelInputSchema } from '../data/schema'
import { useCreateChannel, useUpdateChannel } from '../data/channels'
import { useState } from 'react'


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
  const [supportedModels, setSupportedModels] = useState<string[]>(
    currentRow?.supportedModels || []
  )
  const [newModel, setNewModel] = useState('')

  const channelTypes = [
    { value: 'openai', label: t('channels.types.openai') },
    { value: 'anthropic', label: t('channels.types.anthropic') },
    { value: 'anthropic_aws', label: t('channels.types.anthropic_aws') },
    { value: 'anthropic_gcp', label: t('channels.types.anthropic_gcp') },
    { value: 'gemini', label: t('channels.types.gemini') },
    { value: 'deepseek', label: t('channels.types.deepseek') },
    { value: 'doubao', label: t('channels.types.doubao') },
    { value: 'kimi', label: t('channels.types.kimi') },
    { value: 'anthropic_fake', label: t('channels.types.anthropic_fake') },
    { value: 'openai_fake', label: t('channels.types.openai_fake') },
  ]

  const formSchema = isEdit ? updateChannelInputSchema : createChannelInputSchema
  
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit && currentRow
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
            }
          },
        }
      : {
          type: 'openai',
          baseURL: '',
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
        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: dataWithModels,
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
    setSupportedModels(supportedModels.filter(m => m !== model))
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      addModel()
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
          setSupportedModels(currentRow?.supportedModels || [])
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-4xl max-h-[90vh]'>
        <DialogHeader className='text-left'>
          <DialogTitle>{isEdit ? t('channels.dialog.edit.title') : t('channels.dialog.create.title')}</DialogTitle>
          <DialogDescription>
            {isEdit ? t('channels.dialog.edit.description') : t('channels.dialog.create.description')}
          </DialogDescription>
        </DialogHeader>
        <div className='-mr-4 h-[36rem] w-full overflow-y-auto py-1 pr-4'>
          <Form {...form}>
            <form
              id='channel-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className='space-y-4 p-0.5'
            >
              <FormField
                control={form.control}
                name='type'
                disabled={isEdit}
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      {t('channels.dialog.fields.type.label')}
                    </FormLabel>
                    <FormControl>
                      <SelectDropdown
                        defaultValue={field.value}
                        onValueChange={field.onChange}
                        items={channelTypes}
                        placeholder={t('channels.dialog.fields.type.description')}
                        className='col-span-4'
                        isControlled={true}
                        disabled={isEdit}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      {t('channels.dialog.fields.name.label')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('channels.dialog.fields.name.placeholder')}
                        className='col-span-4'
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='baseURL'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      {t('channels.dialog.fields.baseURL.label')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('channels.dialog.fields.baseURL.placeholder')}
                        className='col-span-4'
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />

              {form.watch('type') !== 'anthropic_aws' && form.watch('type') !== 'anthropic_gcp' && (
                <FormField
                  control={form.control}
                  name='credentials.apiKey'
                  render={({ field }) => (
                    <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                      <FormLabel className='col-span-2 text-right'>
                        {t('channels.dialog.fields.apiKey.label')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={isEdit ? t('channels.dialog.fields.apiKey.editPlaceholder') : t('channels.dialog.fields.apiKey.placeholder')}
                          className='col-span-4'
                          autoComplete='off'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage className='col-span-4 col-start-3' />
                    </FormItem>
                  )}
                />
              )}

              {form.watch('type') === 'anthropic_aws' && (
                <>
                  <FormField
                    control={form.control}
                    name='credentials.aws.accessKeyID'
                    render={({ field }) => (
                      <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                        <FormLabel className='col-span-2 text-right'>
                          {t('channels.dialog.fields.awsAccessKeyID.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder={t('channels.dialog.fields.awsAccessKeyID.placeholder')}
                            className='col-span-4'
                            autoComplete='off'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage className='col-span-4 col-start-3' />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.aws.secretAccessKey'
                    render={({ field }) => (
                      <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                        <FormLabel className='col-span-2 text-right'>
                          {t('channels.dialog.fields.awsSecretAccessKey.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder={t('channels.dialog.fields.awsSecretAccessKey.placeholder')}
                            className='col-span-4'
                            autoComplete='off'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage className='col-span-4 col-start-3' />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.aws.region'
                    render={({ field }) => (
                      <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                        <FormLabel className='col-span-2 text-right'>
                          {t('channels.dialog.fields.awsRegion.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.dialog.fields.awsRegion.placeholder')}
                            className='col-span-4'
                            autoComplete='off'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage className='col-span-4 col-start-3' />
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
                    render={({ field }) => (
                      <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                        <FormLabel className='col-span-2 text-right'>
                          {t('channels.dialog.fields.gcpRegion.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.dialog.fields.gcpRegion.placeholder')}
                            className='col-span-4'
                            autoComplete='off'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage className='col-span-4 col-start-3' />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.gcp.projectID'
                    render={({ field }) => (
                      <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                        <FormLabel className='col-span-2 text-right'>
                          {t('channels.dialog.fields.gcpProjectID.label')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.dialog.fields.gcpProjectID.placeholder')}
                            className='col-span-4'
                            autoComplete='off'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage className='col-span-4 col-start-3' />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='credentials.gcp.jsonData'
                    render={({ field }) => (
                      <FormItem className='grid grid-cols-6 items-start space-y-0 gap-x-4 gap-y-1'>
                        <FormLabel className='col-span-2 text-right pt-2'>
                          {t('channels.dialog.fields.gcpJsonData.label')}
                        </FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder={`{\n  "type": "service_account",\n  "project_id": "project-123",\n  "private_key_id": "fdfd",\n  "private_key": "-----BEGIN PRIVATE KEY-----\\n-----END PRIVATE KEY-----\\n",\n  "client_email": "xxx@developer.gserviceaccount.com",\n  "client_id": "client_213123123",\n  "auth_uri": "https://accounts.google.com/o/oauth2/auth",\n  "token_uri": "https://oauth2.googleapis.com/token",\n  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",\n  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/xxx-compute%40developer.gserviceaccount.com",\n  "universe_domain": "googleapis.com"\n}`}
                            className='col-span-4 min-h-[200px] font-mono text-xs resize-y'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage className='col-span-4 col-start-3' />
                      </FormItem>
                    )}
                  />
                </>
              )}

              <div className='grid grid-cols-6 items-start space-y-0 gap-x-4 gap-y-1'>
                <label className='col-span-2 text-right text-sm font-medium'>
                  {t('channels.dialog.fields.supportedModels.label')}
                </label>
                <div className='col-span-4 space-y-2'>
                  <div className='flex gap-2'>
                    <Input
                      placeholder={t('channels.dialog.fields.supportedModels.description')}
                      value={newModel}
                      onChange={(e) => setNewModel(e.target.value)}
                      onKeyPress={handleKeyPress}
                      className='flex-1'
                    />
                    <Button type='button' onClick={addModel} size='sm'>
                      {t('channels.dialog.buttons.add')}
                    </Button>
                  </div>
                  <div className='flex flex-wrap gap-1'>
                    {supportedModels.map((model) => (
                      <Badge key={model} variant='secondary' className='text-xs'>
                        {model}
                        <button
                          type='button'
                          onClick={() => removeModel(model)}
                          className='ml-1 hover:text-destructive'
                        >
                          <X size={12} />
                        </button>
                      </Badge>
                    ))}
                  </div>
                  {supportedModels.length === 0 && (
                    <p className='text-sm text-muted-foreground'>
                      {t('channels.dialog.fields.supportedModels.required')}
                    </p>
                  )}
                </div>
              </div>

              <FormField
                control={form.control}
                name='defaultTestModel'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      {t('channels.dialog.fields.defaultTestModel.label')}
                    </FormLabel>
                    <FormControl>
                      <SelectDropdown
                        defaultValue={field.value}
                        onValueChange={field.onChange}
                        items={supportedModels.map(model => ({ value: model, label: model }))}
                        placeholder={t('channels.dialog.fields.defaultTestModel.description')}
                        className='col-span-4'
                        disabled={supportedModels.length === 0}
                        isControlled={true}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
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
            disabled={
              createChannel.isPending || 
              updateChannel.isPending || 
              supportedModels.length === 0
            }
          >
            {createChannel.isPending || updateChannel.isPending 
              ? (isEdit ? t('channels.dialog.buttons.updating') : t('channels.dialog.buttons.creating'))
              : (isEdit ? t('channels.dialog.buttons.update') : t('channels.dialog.buttons.create'))
            }
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}