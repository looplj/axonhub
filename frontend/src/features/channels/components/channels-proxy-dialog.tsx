'use client'

import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { IconPlayerPlay } from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useUpdateChannel, useTestChannel } from '../data/channels'
import { Channel } from '../data/schema'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

// Proxy type enum
export enum ProxyType {
  DISABLED = 'disabled',
  ENVIRONMENT = 'environment',
  URL = 'url',
}

// Proxy configuration schema
const proxyConfigSchema = z.object({
  type: z.nativeEnum(ProxyType),
  url: z.string().optional(),
  username: z.string().optional(),
  password: z.string().optional(),
}).refine(
  (data) => {
    // If type is URL, url field is required
    if (data.type === ProxyType.URL) {
      return !!data.url && data.url.trim() !== ''
    }
    return true
  },
  {
    message: 'Proxy URL is required when type is URL',
    path: ['url'],
  }
)

type ProxyConfig = z.infer<typeof proxyConfigSchema>

export function ChannelsProxyDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannel = useUpdateChannel()
  const testChannel = useTestChannel()
  const [isTesting, setIsTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message?: string|null; latency?: number } | null>(null)

  const form = useForm<ProxyConfig>({
    resolver: zodResolver(proxyConfigSchema),
    defaultValues: {
      type: (currentRow.settings?.proxy?.type as ProxyType) || ProxyType.ENVIRONMENT,
      url: currentRow.settings?.proxy?.url || '',
      username: currentRow.settings?.proxy?.username || '',
      password: currentRow.settings?.proxy?.password || '',
    },
  })

  const selectedProxyType = form.watch('type')

  const onSubmit = async (values: ProxyConfig) => {
    try {
      // Prepare proxy config
      const proxyConfig = {
        type: values.type,
        ...(values.type === ProxyType.URL && {
          url: values.url,
          username: values.username || undefined,
          password: values.password || undefined,
        }),
      }

      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            extraModelPrefix: currentRow.settings?.extraModelPrefix,
            modelMappings: currentRow.settings?.modelMappings || [],
            overrideParameters: currentRow.settings?.overrideParameters,
            proxy: proxyConfig,
          },
        },
      })
      toast.success(t('channels.messages.updateSuccess'))
      onOpenChange(false)
    } catch (_error) {
      toast.error(t('channels.messages.updateError'))
    }
  }

  const handleTest = async () => {
    setIsTesting(true)
    setTestResult(null)

    try {
      // Get current form values
      const values = form.getValues()
      
      // Prepare proxy config for testing
      const proxyConfig = {
        type: values.type,
        ...(values.type === ProxyType.URL && {
          url: values.url,
          username: values.username || undefined,
          password: values.password || undefined,
        }),
      }

      const result = await testChannel.mutateAsync({
        channelID: currentRow.id,
        modelID: currentRow.defaultTestModel,
        proxy: proxyConfig,
      })

      setTestResult({
        success: result.success,
        message: result.success ? result.message : result.error || 'Unknown error',
        latency: result.latency,
      })

      if (result.success) {
        toast.success(t('channels.dialogs.proxy.testSuccess'))
      } else {
        toast.error(result.error || t('channels.dialogs.proxy.testFailed'))
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error'
      setTestResult({
        success: false,
        message: errorMessage,
      })
      toast.error(errorMessage)
    } finally {
      setIsTesting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
          setTestResult(null)
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.proxy.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.proxy.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-6'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.dialogs.proxy.config.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.proxy.config.description')}</CardDescription>
            </CardHeader>
            <CardContent>
              <Form {...form}>
                <form className='space-y-4'>
                  <FormField
                    control={form.control}
                    name='type'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.dialogs.proxy.fields.type.label')}</FormLabel>
                        <Select onValueChange={field.onChange} value={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('channels.dialogs.proxy.fields.type.placeholder')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value={ProxyType.DISABLED}>
                              {t('channels.dialogs.proxy.types.disabled')}
                            </SelectItem>
                            <SelectItem value={ProxyType.ENVIRONMENT}>
                              {t('channels.dialogs.proxy.types.environment')}
                            </SelectItem>
                            <SelectItem value={ProxyType.URL}>
                              {t('channels.dialogs.proxy.types.url')}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {selectedProxyType === ProxyType.URL && (
                    <>
                      <FormField
                        control={form.control}
                        name='url'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('channels.dialogs.proxy.fields.url.label')}</FormLabel>
                            <FormControl>
                              <Input
                                placeholder={t('channels.dialogs.proxy.fields.url.placeholder')}
                                {...field}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='username'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('channels.dialogs.proxy.fields.username.label')}</FormLabel>
                            <FormControl>
                              <Input
                                placeholder={t('channels.dialogs.proxy.fields.username.placeholder')}
                                {...field}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='password'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('channels.dialogs.proxy.fields.password.label')}</FormLabel>
                            <FormControl>
                              <Input
                                type='password'
                                placeholder={t('channels.dialogs.proxy.fields.password.placeholder')}
                                {...field}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </>
                  )}

                  {selectedProxyType === ProxyType.ENVIRONMENT && (
                    <div className='text-muted-foreground rounded-md border p-3 text-sm'>
                      {t('channels.dialogs.proxy.environmentHint')}
                    </div>
                  )}

                  {selectedProxyType === ProxyType.DISABLED && (
                    <div className='text-muted-foreground rounded-md border p-3 text-sm'>
                      {t('channels.dialogs.proxy.disabledHint')}
                    </div>
                  )}
                </form>
              </Form>
            </CardContent>
          </Card>

          {/* Test Result Card */}
          {testResult && (
            <Card className={testResult.success ? 'border-green-200' : 'border-red-200'}>
              <CardHeader>
                <CardTitle className={`text-lg ${testResult.success ? 'text-green-800' : 'text-red-800'}`}>
                  {testResult.success ? t('channels.dialogs.proxy.testSuccess') : t('channels.dialogs.proxy.testFailed')}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {testResult.latency && (
                  <p className='text-sm'>
                    <span className='font-medium'>{t('channels.dialogs.proxy.latency')}:</span> {testResult.latency.toFixed(2)}s
                  </p>
                )}
                {testResult.message && (
                  <p className='text-muted-foreground mt-2 text-sm'>{testResult.message}</p>
                )}
              </CardContent>
            </Card>
          )}
        </div>

        <DialogFooter className='flex justify-between'>
          <Button
            type='button'
            variant='outline'
            onClick={handleTest}
            disabled={isTesting || testChannel.isPending}
          >
            <IconPlayerPlay className='mr-2 h-4 w-4' />
            {isTesting ? t('channels.dialogs.proxy.testing') : t('channels.dialogs.proxy.test')}
          </Button>
          <div className='flex gap-2'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='button' onClick={form.handleSubmit(onSubmit)} disabled={updateChannel.isPending}>
              {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
