import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useUpdateChannel } from '../data/channels'
import { Channel } from '../data/schema'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

const overrideParametersFormSchema = z.object({
  overrideParameters: z.string().optional().refine(
    (val) => {
      if (!val || val.trim() === '') return true
      try {
        JSON.parse(val)
        return true
      } catch {
        return false
      }
    },
    {
      message: 'channels.validation.overrideParametersInvalidJson',
    }
  ),
})

export function ChannelsOverrideParametersDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannel = useUpdateChannel()

  const [overrideParametersText, setOverrideParametersText] = useState<string>(
    currentRow.settings?.overrideParameters || ''
  )

  const form = useForm<z.infer<typeof overrideParametersFormSchema>>({
    resolver: zodResolver(overrideParametersFormSchema),
    defaultValues: {
      overrideParameters: currentRow.settings?.overrideParameters || '',
    },
  })

  useEffect(() => {
    const nextValue = currentRow.settings?.overrideParameters || ''
    setOverrideParametersText(nextValue)
    form.reset({ overrideParameters: nextValue })
  }, [currentRow, open, form])

  const onSubmit = async (values: z.infer<typeof overrideParametersFormSchema>) => {
    try {
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
          <DialogTitle>{t('channels.dialogs.settings.overrideParameters.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.settings.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)}>
          <div className='space-y-6'>
            <Card>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrideParameters.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.overrideParameters.description')}</CardDescription>
              </CardHeader>
              <CardContent>
                <Textarea
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
                  className='font-mono min-h-[200px]'
                />
                {form.formState.errors.overrideParameters?.message && (
                  <p className='text-destructive mt-2 text-sm'>
                    {t(form.formState.errors.overrideParameters.message.toString())}
                  </p>
                )}
              </CardContent>
            </Card>
          </div>

          <DialogFooter className='mt-6'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='submit' disabled={updateChannel.isPending}>
              {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
