'use client'

import React, { useState } from 'react'
import { Loader2, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useSystemContext } from '../context/system-context'
import { 
  useStoragePolicy,
  useUpdateStoragePolicy,
  CleanupOption
} from '../data/system'

export function StorageSettings() {
  const { t } = useTranslation()
  const { data: storagePolicy, isLoading: isLoadingStoragePolicy } = useStoragePolicy()
  const updateStoragePolicy = useUpdateStoragePolicy()
  const { isLoading, setIsLoading } = useSystemContext()

  const [storagePolicyState, setStoragePolicyState] = useState({
    storeChunks: storagePolicy?.storeChunks ?? false,
    storeRequestBody: storagePolicy?.storeRequestBody ?? true,
    storeResponseBody: storagePolicy?.storeResponseBody ?? true,
    cleanupOptions: storagePolicy?.cleanupOptions ?? []
  })

  // Update local state when storage policy is loaded
  React.useEffect(() => {
    if (storagePolicy) {
      setStoragePolicyState({
        storeChunks: storagePolicy.storeChunks,
        storeRequestBody: storagePolicy.storeRequestBody,
        storeResponseBody: storagePolicy.storeResponseBody,
        cleanupOptions: storagePolicy.cleanupOptions
      })
    }
  }, [storagePolicy])

  const handleSaveStoragePolicy = async () => {
    setIsLoading(true)
    try {
      await updateStoragePolicy.mutateAsync({
        storeChunks: storagePolicyState.storeChunks,
        storeRequestBody: storagePolicyState.storeRequestBody,
        storeResponseBody: storagePolicyState.storeResponseBody,
        cleanupOptions: storagePolicyState.cleanupOptions.map(option => ({
          resourceType: option.resourceType,
          enabled: option.enabled,
          cleanupDays: option.cleanupDays
        }))
      })
    } finally {
      setIsLoading(false)
    }
  }

  const handleCleanupOptionChange = (index: number, field: keyof CleanupOption, value: any) => {
    const newOptions = [...storagePolicyState.cleanupOptions]
    newOptions[index] = {
      ...newOptions[index],
      [field]: value
    }
    setStoragePolicyState({
      ...storagePolicyState,
      cleanupOptions: newOptions
    })
  }

  const hasStoragePolicyChanges = 
    storagePolicy && 
    (storagePolicy.storeChunks !== storagePolicyState.storeChunks ||
      storagePolicy.storeRequestBody !== storagePolicyState.storeRequestBody ||
      storagePolicy.storeResponseBody !== storagePolicyState.storeResponseBody ||
      JSON.stringify(storagePolicy.cleanupOptions) !== JSON.stringify(storagePolicyState.cleanupOptions))

  if (isLoadingStoragePolicy) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>
          {t('loading')}
        </span>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.storage.policy.title')}</CardTitle>
          <CardDescription>{t('system.storage.policy.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-chunks'>
                {t('system.storage.policy.storeChunks.label')}
              </Label>
              <div className='text-muted-foreground text-sm'>
                {t('system.storage.policy.storeChunks.description')}
              </div>
            </div>
            <Switch
              id='storage-policy-store-chunks'
              checked={storagePolicyState.storeChunks}
              onCheckedChange={(checked) => setStoragePolicyState({
                ...storagePolicyState,
                storeChunks: checked
              })}
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-request-body'>
                {t('system.storage.policy.storeRequestBody.label', 'Store request body')}
              </Label>
              <div className='text-muted-foreground text-sm'>
                {t('system.storage.policy.storeRequestBody.description', 'If disabled, original request body will not be persisted.')}
              </div>
            </div>
            <Switch
              id='storage-policy-store-request-body'
              checked={storagePolicyState.storeRequestBody}
              onCheckedChange={(checked) => setStoragePolicyState({
                ...storagePolicyState,
                storeRequestBody: checked
              })}
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-response-body'>
                {t('system.storage.policy.storeResponseBody.label', 'Store response body')}
              </Label>
              <div className='text-muted-foreground text-sm'>
                {t('system.storage.policy.storeResponseBody.description', 'If disabled, final response body will not be persisted.')}
              </div>
            </div>
            <Switch
              id='storage-policy-store-response-body'
              checked={storagePolicyState.storeResponseBody}
              onCheckedChange={(checked) => setStoragePolicyState({
                ...storagePolicyState,
                storeResponseBody: checked
              })}
              disabled={isLoading}
            />
          </div>

          <div className='space-y-4'>
            <div className='text-lg font-medium'>
              {t('system.storage.policy.cleanupOptions')}
            </div>
            {storagePolicyState.cleanupOptions.map((option, index) => (
              <div key={option.resourceType} className='flex flex-col gap-4 rounded-lg border p-4'>
                <div className='flex items-center justify-between'>
                  <div className='font-medium'>
                    {t(`system.storage.policy.resourceTypes.${option.resourceType}`) || option.resourceType}
                  </div>
                  <Switch
                    checked={option.enabled}
                    onCheckedChange={(checked) => handleCleanupOptionChange(index, 'enabled', checked)}
                    disabled={isLoading}
                  />
                </div>
                {option.enabled && (
                  <div className='flex items-center gap-2'>
                    <Label htmlFor={`cleanup-days-${index}`}>
                      {t('system.storage.policy.cleanupDays')}
                    </Label>
                    <Input
                      id={`cleanup-days-${index}`}
                      type='number'
                      min='1'
                      max='365'
                      value={option.cleanupDays}
                      onChange={(e) => handleCleanupOptionChange(index, 'cleanupDays', parseInt(e.target.value) || 1)}
                      className='w-24'
                      disabled={isLoading}
                    />
                    <span>{t('system.storage.policy.days')}</span>
                  </div>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {hasStoragePolicyChanges && (
        <div className='flex justify-end'>
          <Button
            onClick={handleSaveStoragePolicy}
            disabled={isLoading || updateStoragePolicy.isPending}
            className='min-w-[100px]'
          >
            {isLoading || updateStoragePolicy.isPending ? (
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