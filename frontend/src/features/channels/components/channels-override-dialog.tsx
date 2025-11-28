import { useEffect, useState, useCallback } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useUpdateChannel } from '../data/channels'
import { Channel, HeaderEntry } from '../data/schema'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

const AUTH_HEADER_KEYS = ['authorization', 'proxy-authorization','x-api-key','x-api-secret','x-api-token']

const overrideFormSchema = z.object({
  overrideHeaders: z.array(z.object({
    key: z.string().min(1, 'Header key is required'),
    value: z.string(),
  })).optional(),
  overrideParameters: z
    .string()
    .optional()
    .superRefine((val, ctx) => {
      if (!val || val.trim() === '') return

      let parsed: unknown
      try {
        parsed = JSON.parse(val)
      } catch {
        ctx.addIssue({
          code: 'custom',
          message: 'channels.validation.overrideParametersInvalidJson',
        })
        return
      }

      if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
        ctx.addIssue({
          code: 'custom',
          message: 'channels.validation.overrideParametersInvalidJson',
        })
        return
      }

      if (Object.prototype.hasOwnProperty.call(parsed, 'stream')) {
        ctx.addIssue({
          code: 'custom',
          message: 'channels.validation.overrideParametersStreamNotAllowed',
        })
      }
    }),
})

export function ChannelsOverrideDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannel = useUpdateChannel()

  const [headers, setHeaders] = useState<HeaderEntry[]>(
    currentRow.settings?.overrideHeaders || []
  )
  const [overrideParametersText, setOverrideParametersText] = useState<string>(
    currentRow.settings?.overrideParameters || ''
  )

  const form = useForm<z.infer<typeof overrideFormSchema>>({
    resolver: zodResolver(overrideFormSchema),
    defaultValues: {
      overrideHeaders: currentRow.settings?.overrideHeaders || [],
      overrideParameters: currentRow.settings?.overrideParameters || '',
    },
  })

  useEffect(() => {
    const nextHeaders = currentRow.settings?.overrideHeaders || []
    const nextParameters = currentRow.settings?.overrideParameters || ''
    setHeaders(nextHeaders)
    setOverrideParametersText(nextParameters)
    form.reset({ 
      overrideHeaders: nextHeaders,
      overrideParameters: nextParameters 
    })
  }, [currentRow, open, form])

  const addHeader = useCallback(() => {
    const newHeaders = [...headers, { key: '', value: '' }]
    setHeaders(newHeaders)
    form.setValue('overrideHeaders', newHeaders)
  }, [headers, form])

  const removeHeader = useCallback((index: number) => {
    const newHeaders = headers.filter((_, i) => i !== index)
    setHeaders(newHeaders)
    form.setValue('overrideHeaders', newHeaders)
  }, [headers, form])

  const updateHeader = useCallback((index: number, field: keyof HeaderEntry, value: string) => {
    const newHeaders = headers.map((header, i) => 
      i === index ? { ...header, [field]: value } : header
    )
    setHeaders(newHeaders)
    form.setValue('overrideHeaders', newHeaders)
    
    // Trigger validation for the specific field if it's the key field
    if (field === 'key') {
      form.trigger(`overrideHeaders.${index}.key`)
    }
  }, [headers, form])

  const onSubmit = async (values: z.infer<typeof overrideFormSchema>) => {
    try {
      // Filter out headers with empty keys
      const validHeaders = values.overrideHeaders?.filter(header => header.key.trim() !== '') || []
      
      // Parse overrideParameters if provided
      let overrideParameters: string | undefined
      if (values.overrideParameters && values.overrideParameters.trim()) {
        overrideParameters = values.overrideParameters
      }

      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            extraModelPrefix: currentRow.settings?.extraModelPrefix,
            modelMappings: currentRow.settings?.modelMappings || [],
            overrideParameters,
            overrideHeaders: validHeaders,
            proxy: currentRow.settings?.proxy,
          },
        },
      })
      toast.success(t('channels.messages.updateSuccess'))
      onOpenChange(false)
    } catch (_error) {
      toast.error(t('channels.messages.updateError'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[800px]'>
        <DialogHeader>
          <DialogTitle data-testid="override-dialog-title">{t('channels.dialogs.settings.overrides.title')}</DialogTitle>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)}>
          <div className='space-y-6'>
            {/* Headers Section */}
            <Card data-testid="override-headers-section">
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrides.headers.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.overrides.headers.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-3'>
                {headers.map((header, index) => {
                  const fieldError = form.formState.errors.overrideHeaders?.[index]?.key
                  const normalizedKey = header.key.trim().toLowerCase()
                  const isAuthHeader = normalizedKey !== '' && AUTH_HEADER_KEYS.includes(normalizedKey)
                  return (
                    <div key={index} className='flex gap-3 items-start'>
                      <div className='flex-1 space-y-2'>
                        <Label htmlFor={`header-key-${index}`} className='text-sm font-medium'>
                          {t('channels.dialogs.settings.overrides.headers.key')}
                        </Label>
                        <Input
                          id={`header-key-${index}`}
                          data-testid={`header-key-${index}`}
                          placeholder='User-Agent'
                          value={header.key}
                          onChange={(e) => updateHeader(index, 'key', e.target.value)}
                          className={`font-mono ${fieldError ? 'border-destructive' : ''}`}
                        />
                        {isAuthHeader && (
                          <p className='text-destructive text-sm' role='alert'>
                            {t('channels.dialogs.settings.overrides.headers.sensitiveWarning')}
                          </p>
                        )}
                      </div>
                      <div className='flex-1 space-y-2'>
                        <Label htmlFor={`header-value-${index}`} className='text-sm font-medium'>
                          {t('channels.dialogs.settings.overrides.headers.value')}
                        </Label>
                        <Input
                          id={`header-value-${index}`}
                          data-testid={`header-value-${index}`}
                          placeholder={t('channels.dialogs.settings.overrides.headers.valuePlaceholder')}
                          value={header.value}
                          onChange={(e) => updateHeader(index, 'value', e.target.value)}
                          className='font-mono'
                        />
                      </div>
                      <div className='pt-7'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => removeHeader(index)}
                          className='px-3'
                          data-testid={`remove-header-${index}`}
                        >
                          {t('common.buttons.remove')}
                        </Button>
                      </div>
                    </div>
                  )
                })}
                
                <Button
                  type='button'
                  variant='outline'
                  onClick={addHeader}
                  className='w-full'
                  data-testid='add-header-button'
                >
                  {t('channels.dialogs.settings.overrides.headers.addButton')}
                </Button>

                {form.formState.errors.overrideHeaders?.message && (
                  <p className='text-destructive text-sm'>
                    {t(form.formState.errors.overrideHeaders.message.toString())}
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Parameters Section */}
            <Card data-testid="override-parameters-section">
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrides.parameters.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.overrides.parameters.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-3'>
                <Textarea
                  data-testid='override-parameters-textarea'
                  placeholder='{"temperature": 0.7, "max_tokens": 1000}'
                  value={overrideParametersText}
                  onChange={(e) => {
                    const value = e.target.value
                    setOverrideParametersText(value)
                    form.setValue('overrideParameters', value, {
                      shouldValidate: true,
                      shouldDirty: true,
                    })
                  }}
                  className='min-h-[200px] font-mono'
                />
                {form.formState.errors.overrideParameters?.message && (
                  <p className='text-destructive text-sm'>
                    {t(form.formState.errors.overrideParameters.message.toString())}
                  </p>
                )}
              </CardContent>
            </Card>
          </div>

          <DialogFooter className='mt-6'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)} data-testid='override-cancel-button'>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='submit' disabled={updateChannel.isPending} data-testid='override-save-button'>
              {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
