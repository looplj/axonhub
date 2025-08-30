import { useCallback, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { X, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
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
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useUpdateChannel } from '../data/channels'
import { Channel, ModelMapping } from '../data/schema'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

// 扩展 schema 以包含模型映射的校验规则
const createChannelSettingsFormSchema = (supportedModels: string[]) =>
  z.object({
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

export function ChannelsSettingsDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannel = useUpdateChannel()
  const [modelMappings, setModelMappings] = useState<ModelMapping[]>(currentRow.settings?.modelMappings || [])
  const [newMapping, setNewMapping] = useState({ from: '', to: '' })

  const channelSettingsFormSchema = createChannelSettingsFormSchema(currentRow.supportedModels)

  const form = useForm<z.infer<typeof channelSettingsFormSchema>>({
    resolver: zodResolver(channelSettingsFormSchema),
    defaultValues: {
      modelMappings: currentRow.settings?.modelMappings || [],
    },
  })

  const onSubmit = async (values: z.infer<typeof channelSettingsFormSchema>) => {
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
            modelMappings: values.modelMappings,
          },
        },
      })
      toast.success(t('channels.messages.updateSuccess'))
      onOpenChange(false)
    } catch (_error) {
      toast.error(t('channels.messages.updateError'))
    }
  }

  const addMapping = useCallback(() => {
    if (newMapping.from.trim() && newMapping.to.trim()) {
      const exists = modelMappings.some((mapping) => mapping.from === newMapping.from.trim())
      if (!exists) {
        const newMappings = [...modelMappings, { from: newMapping.from.trim(), to: newMapping.to.trim() }]
        setModelMappings(newMappings)
        form.setValue('modelMappings', newMappings)
        form.clearErrors('modelMappings')
        setNewMapping({ from: '', to: '' })
      } else {
        toast.warning(t('channels.messages.duplicateMappingWarning'))
      }
    }
  }, [form, modelMappings, setModelMappings, newMapping, t])

  const removeMapping = useCallback(
    (index: number) => {
      const newMappings = modelMappings.filter((_, i) => i !== index)
      setModelMappings(newMappings)
      form.setValue('modelMappings', newMappings)
      form.clearErrors('modelMappings')
    },
    [form, modelMappings, setModelMappings]
  )

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          setModelMappings(currentRow.settings?.modelMappings || [])
          setNewMapping({ from: '', to: '' })
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.settings.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.settings.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-6'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.dialogs.settings.basicInfo.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.settings.basicInfo.description')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='grid grid-cols-2 gap-4'>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialogs.fields.name.label')}</label>
                  <p className='text-muted-foreground text-sm'>{currentRow.name}</p>
                </div>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialogs.fields.type.label')}</label>
                  <p className='text-muted-foreground text-sm'>{currentRow.type}</p>
                </div>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialogs.fields.baseURL.label')}</label>
                  <p className='text-muted-foreground text-sm'>{currentRow.baseURL}</p>
                </div>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialogs.fields.defaultTestModel.label')}</label>
                  <p className='text-muted-foreground text-sm'>{currentRow.defaultTestModel}</p>
                </div>
              </div>
              <div>
                <label className='text-sm font-medium'>{t('channels.dialogs.fields.supportedModels.label')}</label>
                <div className='mt-1 flex flex-wrap gap-1'>
                  {currentRow.supportedModels.map((model) => (
                    <Badge key={model} variant='outline' className='text-xs'>
                      {model}
                    </Badge>
                  ))}
                </div>
              </div>
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
                >
                  <Plus size={16} />
                </Button>
                {(newMapping.from.trim() || newMapping.to.trim()) && (
                  <Button type='button' variant='outline' size='sm' onClick={() => setNewMapping({ from: '', to: '' })}>
                    <Trash2 size={16} />
                  </Button>
                )}
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

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('channels.dialogs.buttons.cancel')}
          </Button>
          <Button type='button' onClick={form.handleSubmit(onSubmit)} disabled={updateChannel.isPending}>
            {updateChannel.isPending ? t('channels.dialogs.buttons.saving') : t('channels.dialogs.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
