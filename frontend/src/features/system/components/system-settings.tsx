'use client'

import React, { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { Loader2, Save } from 'lucide-react'
import { useSystemSettings, useUpdateSystemSettings } from '../data/system'
import { useSystemContext } from '../context/system-context'

export function SystemSettings() {
  const { t } = useTranslation()
  const { data: settings, isLoading: isLoadingSettings } = useSystemSettings()
  const updateSettings = useUpdateSystemSettings()
  const { isLoading, setIsLoading } = useSystemContext()
  
  const [storeChunks, setStoreChunks] = useState(settings?.storeChunks ?? false)

  // Update local state when settings are loaded
  React.useEffect(() => {
    if (settings) {
      setStoreChunks(settings.storeChunks)
    }
  }, [settings])

  const handleSave = async () => {
    setIsLoading(true)
    try {
      await updateSettings.mutateAsync({
        storeChunks,
      })
    } finally {
      setIsLoading(false)
    }
  }

  const hasChanges = settings && settings.storeChunks !== storeChunks

  if (isLoadingSettings) {
    return (
      <div className='flex items-center justify-center h-32'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='ml-2 text-muted-foreground'>{t('system.loading')}</span>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.storage.title')}</CardTitle>
          <CardDescription>
            {t('system.storage.description')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='store-chunks'>{t('system.storage.storeChunks.label')}</Label>
              <div className='text-sm text-muted-foreground'>
                {t('system.storage.storeChunks.description')}
              </div>
            </div>
            <Switch
              id='store-chunks'
              checked={storeChunks}
              onCheckedChange={setStoreChunks}
              disabled={isLoading}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('system.other.title')}</CardTitle>
          <CardDescription>
            {t('system.other.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-muted-foreground'>
            {t('system.other.comingSoon')}
          </div>
        </CardContent>
      </Card>

      {hasChanges && (
        <div className='flex justify-end'>
          <Button
            onClick={handleSave}
            disabled={isLoading || updateSettings.isPending}
            className='min-w-[100px]'
          >
            {(isLoading || updateSettings.isPending) ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('system.buttons.saving')}
              </>
            ) : (
              <>
                <Save className='mr-2 h-4 w-4' />
                {t('system.buttons.save')}
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  )
}