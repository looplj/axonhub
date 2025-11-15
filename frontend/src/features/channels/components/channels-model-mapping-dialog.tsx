import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Plus, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useUpdateChannel } from '../data/channels'
import { Channel, ModelMapping } from '../data/schema'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

// 扩展 schema 以包含模型映射的校验规则
const createModelMappingFormSchema = (supportedModels: string[]) =>
  z.object({
    extraModelPrefix: z.string().optional(),
    modelMappings: z
      .array(
        z.object({
          from: z.string().min(1, 'Original model is required'),
          to: z.string().min(1, 'Target model is required'),
        })
      )
      .refine(
        (mappings) => {
          // 检查是否所有 from 字段都是唯一的
          const fromValues = mappings.map((m) => m.from)
          return new Set(fromValues).size === fromValues.length
        },
        {
          message: 'Each original model can only be mapped once',
        }
      )
      .refine(
        (mappings) => {
          // 检查所有目标模型是否在支持的模型列表中
          return mappings.every((m) => supportedModels.includes(m.to))
        },
        {
          message: 'Target model must be in supported models',
        }
      ),
  })

export function ChannelsModelMappingDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannel = useUpdateChannel()

  const [modelMappings, setModelMappings] = useState<ModelMapping[]>(currentRow.settings?.modelMappings || [])
  const [newMapping, setNewMapping] = useState({ from: '', to: '' })

  const modelMappingFormSchema = createModelMappingFormSchema(currentRow.supportedModels)

  const form = useForm<z.infer<typeof modelMappingFormSchema>>({
    resolver: zodResolver(modelMappingFormSchema),
    defaultValues: {
      extraModelPrefix: currentRow.settings?.extraModelPrefix || '',
      modelMappings: currentRow.settings?.modelMappings || [],
    },
  })

  useEffect(() => {
    const nextExtraModelPrefix = currentRow.settings?.extraModelPrefix || ''
    const nextMappings = currentRow.settings?.modelMappings || []
    setModelMappings(nextMappings)
    setNewMapping({ from: '', to: '' })
    form.reset({
      extraModelPrefix: nextExtraModelPrefix,
      modelMappings: nextMappings,
    })
  }, [currentRow, open, form])

  const onSubmit = async (values: z.infer<typeof modelMappingFormSchema>) => {
    // 检查是否有未添加的映射
    if (newMapping.from.trim() || newMapping.to.trim()) {
      toast.warning(t('channels.messages.pendingMappingWarning'))
      return
    }

    try {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            extraModelPrefix: values.extraModelPrefix,
            modelMappings: values.modelMappings,
            overrideParameters: currentRow.settings?.overrideParameters,
          },
        },
      })
      toast.success(t('channels.messages.updateSuccess'))
      onOpenChange(false)
    } catch (_error) {
      toast.error(t('channels.messages.updateError'))
    }
  }

  const addMapping = () => {
    if (!newMapping.from.trim() || !newMapping.to.trim()) {
      return
    }

    const updatedMappings = [...modelMappings, newMapping]
    setModelMappings(updatedMappings)
    form.setValue('modelMappings', updatedMappings, {
      shouldValidate: true,
      shouldDirty: true,
    })
    setNewMapping({ from: '', to: '' })
  }

  const removeMapping = (index: number) => {
    const updatedMappings = modelMappings.filter((_, i) => i !== index)
    setModelMappings(updatedMappings)
    form.setValue('modelMappings', updatedMappings)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[800px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.dialogs.settings.modelMapping.title')}</DialogTitle>
          {/* <DialogDescription>{t('channels.dialogs.settings.modelMapping.description', { name: currentRow.name })}</DialogDescription> */}
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)}>
          <div className='space-y-6'>
            <Card>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.extraModelPrefix.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.extraModelPrefix.description')}</CardDescription>
              </CardHeader>
              <CardContent>
                <Input
                  placeholder={t('channels.dialogs.settings.extraModelPrefix.placeholder')}
                  value={form.watch('extraModelPrefix') || ''}
                  onChange={(e) => form.setValue('extraModelPrefix', e.target.value)}
                />
                {form.formState.errors.extraModelPrefix?.message && (
                  <p className='text-destructive mt-2 text-sm'>
                    {form.formState.errors.extraModelPrefix.message.toString()}
                  </p>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.modelMapping.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.modelMapping.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='flex gap-2'>
                  <Input
                    placeholder={t('channels.dialogs.settings.modelMapping.originalModel')}
                    value={newMapping.from}
                    onChange={(e) => setNewMapping({ ...newMapping, from: e.target.value })}
                    className='flex-1'
                  />
                  <span className='text-muted-foreground flex items-center'>→</span>
                  <Select value={newMapping.to} onValueChange={(value) => setNewMapping({ ...newMapping, to: value })}>
                    <SelectTrigger className='flex-1'>
                      <SelectValue placeholder={t('channels.dialogs.settings.modelMapping.targetModel')} />
                    </SelectTrigger>
                    <SelectContent>
                      {currentRow.supportedModels.map((model) => (
                        <SelectItem key={model} value={model}>
                          {model}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button
                    type='button'
                    onClick={addMapping}
                    size='sm'
                    disabled={!newMapping.from.trim() || !newMapping.to.trim()}
                    data-testid='add-model-mapping-button'
                    aria-label={t('channels.dialogs.settings.modelMapping.addMappingButton', {
                      defaultValue: 'Add mapping',
                    })}
                  >
                    <Plus size={16} />
                  </Button>
                </div>

                {/* 显示表单错误 */}
                {form.formState.errors.modelMappings?.message && (
                  <p className='text-destructive text-sm'>{form.formState.errors.modelMappings.message.toString()}</p>
                )}

                <div className='space-y-2'>
                  {modelMappings.length === 0 ? (
                    <p className='text-muted-foreground py-4 text-center text-sm'>
                      {t('channels.dialogs.settings.modelMapping.noMappings')}
                    </p>
                  ) : (
                    modelMappings.map((mapping, index) => (
                      <div key={index} className='flex items-center justify-between rounded-lg border p-3'>
                        <div className='flex items-center gap-2'>
                          <Badge variant='outline'>{mapping.from}</Badge>
                          <span className='text-muted-foreground'>→</span>
                          <Badge variant='outline'>{mapping.to}</Badge>
                        </div>
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={() => removeMapping(index)}
                          className='text-destructive hover:text-destructive'
                        >
                          <X size={16} />
                        </Button>
                      </div>
                    ))
                  )}
                </div>
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
