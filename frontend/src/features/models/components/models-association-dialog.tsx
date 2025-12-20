import { useEffect, useMemo, useCallback, useState } from 'react'
import { z } from 'zod'
import { useForm, useFieldArray, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { IconPlus, IconTrash } from '@tabler/icons-react'
import { useQueryModels, useQueryModelChannelConnections, ModelAssociationInput, ModelChannelConnection } from '@/gql/models'
import { useTranslation } from 'react-i18next'
import { extractNumberIDAsNumber } from '@/lib/utils'
import { useDebounce } from '@/hooks/use-debounce'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { AutoComplete } from '@/components/auto-complete'
import { useAllChannelsForOrdering } from '@/features/channels/data/channels'
import { useModels } from '../context/models-context'
import { useUpdateModel } from '../data/models'
import { ModelAssociation } from '../data/schema'

// Helper function to validate regex pattern
const isValidRegex = (pattern: string): boolean => {
  if (!pattern) return true
  try {
    new RegExp(pattern)
    return true
  } catch {
    return false
  }
}

const associationFormSchema = z.object({
  associations: z
    .array(
      z.object({
        type: z.enum(['channel_model', 'channel_regex', 'regex']),
        channelId: z.number().optional(),
        modelId: z.string().optional(),
        pattern: z.string().optional(),
      })
    )
    .superRefine((associations, ctx) => {
      associations.forEach((assoc, index) => {
        if (assoc.type === 'channel_model' || assoc.type === 'channel_regex') {
          if (!assoc.channelId) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Channel is required',
              path: [index, 'channelId'],
            })
          }
        }
        if (assoc.type === 'channel_model') {
          if (!assoc.modelId || assoc.modelId.trim() === '') {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Model ID is required',
              path: [index, 'modelId'],
            })
          }
        }
        if (assoc.type === 'channel_regex' || assoc.type === 'regex') {
          if (!assoc.pattern || assoc.pattern.trim() === '') {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Pattern is required',
              path: [index, 'pattern'],
            })
          } else if (!isValidRegex(assoc.pattern)) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Invalid regex pattern',
              path: [index, 'pattern'],
            })
          }
        }
      })
    }),
})

type AssociationFormData = z.infer<typeof associationFormSchema>

export function ModelsAssociationDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow } = useModels()
  const updateModel = useUpdateModel()
  const { data: channelsData } = useAllChannelsForOrdering({ enabled: open === 'association' })
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels()
  const { mutateAsync: queryConnections } = useQueryModelChannelConnections()
  const [connections, setConnections] = useState<ModelChannelConnection[]>([])
  const [channelFilter, setChannelFilter] = useState('')

  const isOpen = open === 'association'

  useEffect(() => {
    if (isOpen) {
      fetchModels({
        statusIn: ['enabled'],
        includeMapping: true,
        includePrefix: true,
      })
    }
  }, [isOpen, fetchModels])

  // Build channel options for select
  const channelOptions = useMemo(() => {
    if (!channelsData?.edges) return []
    return channelsData.edges.map((edge) => ({
      value: extractNumberIDAsNumber(edge.node.id),
      label: edge.node.name,
      supportedModels: edge.node.supportedModels || [],
    }))
  }, [channelsData])

  // Build all available model options
  const allModelOptions = useMemo(() => {
    if (!availableModels) return []
    return availableModels.map((model) => ({
      value: model.id,
      label: model.id,
    }))
  }, [availableModels])

  const form = useForm<AssociationFormData>({
    resolver: zodResolver(associationFormSchema),
    defaultValues: {
      associations: [],
    },
  })

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'associations',
  })

  // Watch associations for debounced preview - useWatch triggers re-renders
  const watchedAssociations = useWatch({
    control: form.control,
    name: 'associations',
    defaultValue: [],
  })
  // Serialize to string for stable comparison in debounce
  const associationsString = JSON.stringify(watchedAssociations)
  const debouncedAssociationsString = useDebounce(associationsString, 500)

  // Query connections when associations change
  useEffect(() => {
    if (!isOpen) {
      setConnections([])
      return
    }

    let debouncedAssociations
    try {
      debouncedAssociations = JSON.parse(debouncedAssociationsString)
    } catch {
      setConnections([])
      return
    }

    if (!debouncedAssociations || debouncedAssociations.length === 0) {
      setConnections([])
      return
    }

    const fetchConnections = async () => {
      try {
        const associations: ModelAssociationInput[] = debouncedAssociations
          .filter((assoc: any) => {
            if (assoc.type === 'channel_model') {
              return assoc.channelId && assoc.modelId
            } else if (assoc.type === 'channel_regex') {
              return assoc.channelId && assoc.pattern
            } else if (assoc.type === 'regex') {
              return assoc.pattern
            }
            return false
          })
          .map((assoc: any) => {
            if (assoc.type === 'channel_model') {
              return {
                type: 'channel_model' as const,
                channelModel: {
                  channelId: assoc.channelId!,
                  modelId: assoc.modelId!,
                },
              }
            } else if (assoc.type === 'channel_regex') {
              return {
                type: 'channel_regex' as const,
                channelRegex: {
                  channelId: assoc.channelId!,
                  pattern: assoc.pattern!,
                },
              }
            } else {
              return {
                type: 'regex' as const,
                regex: {
                  pattern: assoc.pattern!,
                },
              }
            }
          })

        if (associations.length > 0) {
          const result = await queryConnections(associations)
          setConnections(result)
        } else {
          setConnections([])
        }
      } catch (error) {
        console.error('Failed to query connections:', error)
        setConnections([])
      }
    }

    fetchConnections()
  }, [debouncedAssociationsString, isOpen, queryConnections])

  useEffect(() => {
    if (isOpen && currentRow) {
      const associations = currentRow.settings?.associations || []
      form.reset({
        associations: associations.map((assoc) => ({
          type: assoc.type,
          channelId: assoc.channelModel?.channelId || assoc.channelRegex?.channelId,
          modelId: assoc.channelModel?.modelId,
          pattern: assoc.channelRegex?.pattern || assoc.regex?.pattern,
        })),
      })
    }
  }, [isOpen, currentRow, form])

  const onSubmit = async (data: AssociationFormData) => {
    if (!currentRow) return

    try {
      const associations: ModelAssociation[] = data.associations.map((assoc) => {
        if (assoc.type === 'channel_model') {
          return {
            type: 'channel_model',
            channelModel: {
              channelId: assoc.channelId || 0,
              modelId: assoc.modelId || '',
            },
            channelRegex: null,
            regex: null,
          }
        } else if (assoc.type === 'channel_regex') {
          return {
            type: 'channel_regex',
            channelModel: null,
            channelRegex: {
              channelId: assoc.channelId || 0,
              pattern: assoc.pattern || '',
            },
            regex: null,
          }
        } else {
          return {
            type: 'regex',
            channelModel: null,
            channelRegex: null,
            regex: {
              pattern: assoc.pattern || '',
            },
          }
        }
      })

      await updateModel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            associations,
          },
        },
      })
      handleClose()
    } catch (_error) {
      // Error is handled by mutation
    }
  }

  const handleClose = useCallback(() => {
    setOpen(null)
    form.reset()
    setConnections([])
    setChannelFilter('')
  }, [setOpen, form])

  const handleAddAssociation = useCallback(() => {
    append({
      type: 'channel_model',
      channelId: undefined,
      modelId: '',
      pattern: '',
    })
  }, [append])

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'enabled':
        return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-400 dark:border-emerald-800'
      case 'disabled':
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700'
      case 'archived':
        return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:border-amber-800'
      default:
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700'
    }
  }

  const getTypeColor = (type: string) => {
    const colors = {
      openai: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-400',
      anthropic: 'bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950 dark:text-purple-400',
      deepseek: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-950 dark:text-indigo-400',
      doubao: 'bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-950 dark:text-orange-400',
      kimi: 'bg-pink-50 text-pink-700 border-pink-200 dark:bg-pink-950 dark:text-pink-400',
    }
    return colors[type as keyof typeof colors] || 'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-900 dark:text-gray-400'
  }

  // Filter connections by channel name
  const filteredConnections = useMemo(() => {
    if (!channelFilter.trim()) return connections
    const filter = channelFilter.toLowerCase().trim()
    return connections.filter((conn) => conn.channel.name.toLowerCase().includes(filter))
  }, [connections, channelFilter])

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='flex h-[85vh] max-h-[800px] flex-col sm:max-w-6xl'>
        <DialogHeader className='shrink-0 text-left'>
          <DialogTitle>{t('models.dialogs.association.title')}</DialogTitle>
          <DialogDescription>{t('models.dialogs.association.description', { name: currentRow?.name })}</DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 gap-6'>
          {/* Left Side - Association Rules */}
          <div className='flex min-h-0 flex-[2] flex-col'>
            {/* Fixed Add Rule Section at Top */}
            <div className='bg-background shrink-0 border-b pb-4'>
              <Button type='button' variant='outline' onClick={handleAddAssociation} className='w-full'>
                <IconPlus className='mr-2 h-4 w-4' />
                {t('models.dialogs.association.addRule')}
              </Button>
            </div>

            {/* Scrollable Rules Section */}
            <div className='flex-1 overflow-y-auto py-4'>
              <Form {...form}>
                <form id='association-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-3'>
                  {fields.length === 0 && (
                    <p className='text-muted-foreground py-8 text-center text-sm'>{t('models.dialogs.association.noRules')}</p>
                  )}

                  {fields.map((field, index) => (
                    <AssociationRow
                      key={field.id}
                      index={index}
                      form={form}
                      channelOptions={channelOptions}
                      allModelOptions={allModelOptions}
                      onRemove={() => remove(index)}
                      t={t}
                    />
                  ))}
                </form>
              </Form>
            </div>
          </div>

          {/* Right Side - Preview */}
          <div className='flex min-h-0 flex-1 flex-col border-l pl-6'>
            <div className='shrink-0 space-y-2 pb-4'>
              <h3 className='text-sm font-semibold'>{t('models.dialogs.association.preview')}</h3>
              <p className='text-muted-foreground text-xs'>{t('models.dialogs.association.previewDescription')}</p>
              <Input
                placeholder={t('models.dialogs.association.filterByChannel')}
                value={channelFilter}
                onChange={(e) => setChannelFilter(e.target.value)}
                className='h-8'
              />
            </div>
            <div className='flex-1 overflow-y-auto'>
              {filteredConnections.length === 0 ? (
                <p className='text-muted-foreground py-8 text-center text-sm'>
                  {channelFilter.trim()
                    ? t('models.dialogs.association.noFilteredConnections')
                    : t('models.dialogs.association.noConnections')}
                </p>
              ) : (
                <div className='space-y-3'>
                  {filteredConnections.map((conn) => (
                    <div key={conn.channel.id} className='rounded-lg border p-3'>
                      <div className='mb-2 flex items-start justify-between gap-2'>
                        <div className='flex flex-col gap-1.5'>
                          <span className='text-sm font-medium'>{conn.channel.name}</span>
                          <div className='flex items-center gap-1.5'>
                            <Badge variant='outline' className={`h-5 px-1.5 text-[10px] font-normal ${getTypeColor(conn.channel.type)}`}>
                              {t(`channels.types.${conn.channel.type}`, conn.channel.type)}
                            </Badge>
                            <Badge
                              variant='outline'
                              className={`h-5 px-1.5 text-[10px] font-normal ${getStatusColor(conn.channel.status)}`}
                            >
                              {t(`channels.status.${conn.channel.status}`)}
                            </Badge>
                          </div>
                        </div>
                      </div>
                      <div className='space-y-1'>
                        {conn.modelIds.map((modelId) => (
                          <div key={modelId} className='bg-muted rounded px-2 py-1 text-xs'>
                            {modelId}
                          </div>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        <DialogFooter className='shrink-0 border-t pt-4'>
          <Button type='button' variant='outline' onClick={handleClose}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='submit' form='association-form' disabled={updateModel.isPending || !form.formState.isValid}>
            {t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface AssociationRowProps {
  index: number
  form: ReturnType<typeof useForm<AssociationFormData>>
  channelOptions: { value: number; label: string; supportedModels: string[] }[]
  allModelOptions: { value: string; label: string }[]
  onRemove: () => void
  t: (key: string) => string
}

function AssociationRow({ index, form, channelOptions, allModelOptions, onRemove, t }: AssociationRowProps) {
  const type = form.watch(`associations.${index}.type`)
  const channelId = form.watch(`associations.${index}.channelId`)
  const [modelSearch, setModelSearch] = useState('')

  const showChannel = type === 'channel_model' || type === 'channel_regex'
  const showModel = type === 'channel_model'
  const showPattern = type === 'channel_regex' || type === 'regex'

  // Filter model options based on selected channel's supported models
  const modelOptions = useMemo(() => {
    if (!showModel || !channelId) {
      return []
    }

    const selectedChannel = channelOptions.find((option) => option.value === channelId)
    if (!selectedChannel?.supportedModels?.length) {
      return []
    }

    // Filter all models to only include those supported by the selected channel
    return allModelOptions.filter((model) => selectedChannel.supportedModels.includes(model.value))
  }, [channelId, channelOptions, allModelOptions, showModel])

  return (
    <div className='flex items-start gap-2 rounded-lg border p-3'>
      {/* Type Select */}
      <FormField
        control={form.control}
        name={`associations.${index}.type`}
        render={({ field }) => (
          <FormItem className='w-36 shrink-0'>
            <FormControl>
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger className='h-9'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='channel_model'>{t('models.dialogs.association.types.channelModel')}</SelectItem>
                  <SelectItem value='channel_regex'>{t('models.dialogs.association.types.channelRegex')}</SelectItem>
                  <SelectItem value='regex'>{t('models.dialogs.association.types.regex')}</SelectItem>
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      {/* Channel Select */}
      {showChannel && (
        <FormField
          control={form.control}
          name={`associations.${index}.channelId`}
          render={({ field }) => (
            <FormItem className='flex-1'>
              <FormControl>
                <Select value={field.value?.toString() || ''} onValueChange={(value) => field.onChange(Number(value))}>
                  <SelectTrigger className='h-9'>
                    <SelectValue placeholder={t('models.dialogs.association.selectChannel')} />
                  </SelectTrigger>
                  <SelectContent>
                    {channelOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value.toString()}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* Model Select/AutoComplete */}
      {showModel && (
        <FormField
          control={form.control}
          name={`associations.${index}.modelId`}
          render={({ field }) => (
            <FormItem className='flex-1'>
              <FormControl>
                <AutoComplete
                  selectedValue={field.value?.toString() || ''}
                  onSelectedValueChange={(value) => {
                    field.onChange(value)
                    setModelSearch(value)
                  }}
                  searchValue={modelSearch || field.value?.toString() || ''}
                  onSearchValueChange={setModelSearch}
                  items={modelOptions}
                  placeholder={t('models.dialogs.association.selectModel')}
                  emptyMessage={
                    modelOptions.length === 0 && channelId
                      ? t('models.dialogs.association.noChannelModelsAvailable')
                      : t('models.dialogs.association.selectChannelFirst')
                  }
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* Pattern Input */}
      {showPattern && (
        <FormField
          control={form.control}
          name={`associations.${index}.pattern`}
          render={({ field }) => (
            <FormItem className='flex-1'>
              <FormControl>
                <Input
                  {...field}
                  value={field.value?.toString() || ''}
                  placeholder={t('models.dialogs.association.patternPlaceholder')}
                  className='h-9'
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* Delete Button */}
      <Button
        type='button'
        variant='ghost'
        size='sm'
        onClick={onRemove}
        className='text-destructive hover:text-destructive h-9 w-9 shrink-0 p-0'
      >
        <IconTrash className='h-4 w-4' />
      </Button>
    </div>
  )
}
