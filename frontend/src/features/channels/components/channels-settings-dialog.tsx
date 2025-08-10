'use client'

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
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
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { X, Plus } from 'lucide-react'
import { Channel, channelSettingsSchema } from '../data/schema'
import { useUpdateChannel } from '../data/channels'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

export function ChannelsSettingsDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannel = useUpdateChannel()
  const [modelMappings, setModelMappings] = useState(
    currentRow.settings?.modelMappings || []
  )
  const [newMapping, setNewMapping] = useState({ from: '', to: '' })

  const form = useForm<z.infer<typeof channelSettingsSchema>>({
    resolver: zodResolver(channelSettingsSchema),
    defaultValues: {
      modelMappings: currentRow.settings?.modelMappings || [],
    },
  })

  const onSubmit = async (values: z.infer<typeof channelSettingsSchema>) => {
    try {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            modelMappings,
          },
        },
      })
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to update channel settings:', error)
    }
  }

  const addMapping = () => {
    if (newMapping.from.trim() && newMapping.to.trim()) {
      const exists = modelMappings.some(
        mapping => mapping.from === newMapping.from.trim()
      )
      if (!exists) {
        setModelMappings([
          ...modelMappings,
          { from: newMapping.from.trim(), to: newMapping.to.trim() }
        ])
        setNewMapping({ from: '', to: '' })
      }
    }
  }

  const removeMapping = (index: number) => {
    setModelMappings(modelMappings.filter((_, i) => i !== index))
  }

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
          <DialogTitle>{t('channels.settings.title')}</DialogTitle>
          <DialogDescription>
            {t('channels.settings.description', { name: currentRow.name })}
          </DialogDescription>
        </DialogHeader>
        
        <div className='space-y-6'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.settings.basicInfo.title')}</CardTitle>
              <CardDescription>
                {t('channels.settings.basicInfo.description')}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='grid grid-cols-2 gap-4'>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialog.fields.name.label')}</label>
                  <p className='text-sm text-muted-foreground'>{currentRow.name}</p>
                </div>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialog.fields.type.label')}</label>
                  <p className='text-sm text-muted-foreground'>{currentRow.type}</p>
                </div>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialog.fields.baseURL.label')}</label>
                  <p className='text-sm text-muted-foreground'>{currentRow.baseURL}</p>
                </div>
                <div>
                  <label className='text-sm font-medium'>{t('channels.dialog.fields.defaultTestModel.label')}</label>
                  <p className='text-sm text-muted-foreground'>{currentRow.defaultTestModel}</p>
                </div>
              </div>
              <div>
                <label className='text-sm font-medium'>{t('channels.dialog.fields.supportedModels.label')}</label>
                <div className='flex flex-wrap gap-1 mt-1'>
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
              <CardTitle className='text-lg'>{t('channels.settings.modelMapping.title')}</CardTitle>
              <CardDescription>
                {t('channels.settings.modelMapping.description')}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='flex gap-2'>
                <Input
                  placeholder={t('channels.settings.modelMapping.originalModel')}
                  value={newMapping.from}
                  onChange={(e) => setNewMapping({ ...newMapping, from: e.target.value })}
                  className='flex-1'
                />
                <span className='flex items-center text-muted-foreground'>→</span>
                <Input
                  placeholder={t('channels.settings.modelMapping.targetModel')}
                  value={newMapping.to}
                  onChange={(e) => setNewMapping({ ...newMapping, to: e.target.value })}
                  className='flex-1'
                />
                <Button
                  type='button'
                  onClick={addMapping}
                  size='sm'
                  disabled={!newMapping.from.trim() || !newMapping.to.trim()}
                >
                  <Plus size={16} />
                </Button>
              </div>
              
              <div className='space-y-2'>
                {modelMappings.length === 0 ? (
                  <p className='text-sm text-muted-foreground text-center py-4'>
                    {t('channels.settings.modelMapping.noMappings')}
                  </p>
                ) : (
                  modelMappings.map((mapping, index) => (
                    <div
                      key={index}
                      className='flex items-center justify-between p-3 border rounded-lg'
                    >
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
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('channels.dialog.buttons.cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => onSubmit({ modelMappings })}
            disabled={updateChannel.isPending}
          >
            {updateChannel.isPending ? t('channels.dialog.buttons.saving') : t('channels.dialog.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}