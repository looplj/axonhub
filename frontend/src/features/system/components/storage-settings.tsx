'use client'

import React, { useState } from 'react'
import { Loader2, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useDataStorages } from '@/features/data-storages/data/data-storages'
import { useSystemContext } from '../context/system-context'
import {
  useStoragePolicy,
  useUpdateStoragePolicy,
  useDefaultDataStorageID,
  useUpdateDefaultDataStorage,
  CleanupOption,
} from '../data/system'

export function StorageSettings() {
  const { t } = useTranslation()
  const { data: storagePolicy, isLoading: isLoadingStoragePolicy } = useStoragePolicy()
  const { data: defaultDataStorageID, isLoading: isLoadingDefaultDataStorage } = useDefaultDataStorageID()
  const { data: dataStorages } = useDataStorages({
    first: 100,
    where: { statusIn: ['active'] },
  })
  const updateStoragePolicy = useUpdateStoragePolicy()
  const updateDefaultDataStorage = useUpdateDefaultDataStorage()
  const { isLoading, setIsLoading } = useSystemContext()

  const [storagePolicyState, setStoragePolicyState] = useState({
    storeChunks: storagePolicy?.storeChunks ?? false,
    storeRequestBody: storagePolicy?.storeRequestBody ?? true,
    storeResponseBody: storagePolicy?.storeResponseBody ?? true,
    cleanupOptions: storagePolicy?.cleanupOptions ?? [],
  })

  const [selectedDataStorageID, setSelectedDataStorageID] = useState<string | undefined>(defaultDataStorageID || undefined)

  // Update local state when storage policy is loaded
  React.useEffect(() => {
    if (storagePolicy) {
      setStoragePolicyState({
        storeChunks: storagePolicy.storeChunks,
        storeRequestBody: storagePolicy.storeRequestBody,
        storeResponseBody: storagePolicy.storeResponseBody,
        cleanupOptions: storagePolicy.cleanupOptions,
      })
    }
  }, [storagePolicy])

  // Update selected data storage when loaded
  React.useEffect(() => {
    if (defaultDataStorageID) {
      setSelectedDataStorageID(defaultDataStorageID)
    }
  }, [defaultDataStorageID])

  const handleSaveStoragePolicy = async () => {
    setIsLoading(true)
    try {
      await updateStoragePolicy.mutateAsync({
        storeChunks: storagePolicyState.storeChunks,
        storeRequestBody: storagePolicyState.storeRequestBody,
        storeResponseBody: storagePolicyState.storeResponseBody,
        cleanupOptions: storagePolicyState.cleanupOptions.map((option) => ({
          resourceType: option.resourceType,
          enabled: option.enabled,
          cleanupDays: option.cleanupDays,
        })),
      })
    } finally {
      setIsLoading(false)
    }
  }

  const handleSaveDefaultDataStorage = async () => {
    if (!selectedDataStorageID) return

    setIsLoading(true)
    try {
      await updateDefaultDataStorage.mutateAsync({
        dataStorageID: selectedDataStorageID,
      })
    } finally {
      setIsLoading(false)
    }
  }

  const handleCleanupOptionChange = (index: number, field: keyof CleanupOption, value: any) => {
    const newOptions = [...storagePolicyState.cleanupOptions]
    newOptions[index] = {
      ...newOptions[index],
      [field]: value,
    }
    setStoragePolicyState({
      ...storagePolicyState,
      cleanupOptions: newOptions,
    })
  }

  const hasStoragePolicyChanges =
    storagePolicy &&
    (storagePolicy.storeChunks !== storagePolicyState.storeChunks ||
      storagePolicy.storeRequestBody !== storagePolicyState.storeRequestBody ||
      storagePolicy.storeResponseBody !== storagePolicyState.storeResponseBody ||
      JSON.stringify(storagePolicy.cleanupOptions) !== JSON.stringify(storagePolicyState.cleanupOptions))

  const hasDataStorageChanges = defaultDataStorageID !== selectedDataStorageID

  if (isLoadingStoragePolicy || isLoadingDefaultDataStorage) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      {/* Data Storage Selection */}
      <Card>
        <CardHeader>
          <CardTitle>{t('system.storage.dataStorage.title')}</CardTitle>
          <CardDescription>{t('system.storage.dataStorage.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-2'>
            <Label htmlFor='default-data-storage'>{t('system.storage.dataStorage.label')}</Label>
            <Select value={selectedDataStorageID} onValueChange={setSelectedDataStorageID} disabled={isLoading}>
              <SelectTrigger id='default-data-storage'>
                <SelectValue placeholder={t('system.storage.dataStorage.placeholder')} />
              </SelectTrigger>
              <SelectContent>
                {dataStorages?.edges?.map((edge) => (
                  <SelectItem key={edge.node.id} value={edge.node.id}>
                    {edge.node.name} ({edge.node.type})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {hasDataStorageChanges && (
            <div className='flex justify-end'>
              <Button onClick={handleSaveDefaultDataStorage} disabled={isLoading || updateDefaultDataStorage.isPending} size='sm'>
                {isLoading || updateDefaultDataStorage.isPending ? (
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
        </CardContent>
      </Card>

      {/* Storage Policy */}
      <Card>
        <CardHeader>
          <CardTitle>{t('system.storage.policy.title')}</CardTitle>
          <CardDescription>{t('system.storage.policy.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='flex items-center justify-between' id='storage-enabled-switch'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-chunks'>{t('system.storage.policy.storeChunks.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.storeChunks.description')}</div>
            </div>
            <Switch
              id='storage-policy-store-chunks'
              checked={storagePolicyState.storeChunks}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  storeChunks: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-request-body'>{t('system.storage.policy.storeRequestBody.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.storeRequestBody.description')}</div>
            </div>
            <Switch
              id='storage-policy-store-request-body'
              checked={storagePolicyState.storeRequestBody}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  storeRequestBody: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-response-body'>{t('system.storage.policy.storeResponseBody.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.storeResponseBody.description')}</div>
            </div>
            <Switch
              id='storage-policy-store-response-body'
              checked={storagePolicyState.storeResponseBody}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  storeResponseBody: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <div className='text-lg font-medium'>{t('system.storage.policy.cleanupOptions')}</div>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.cleanupDescription')}</div>
            </div>
            {storagePolicyState.cleanupOptions.map((option, index) => (
              <div
                key={option.resourceType}
                className='flex flex-col gap-4 rounded-lg border p-4'
                id={'storage-cleanup-option-' + option.resourceType}
              >
                <div className='flex items-center justify-between'>
                  <div className='font-medium'>{t(`system.storage.policy.resourceTypes.${option.resourceType}`)}</div>
                  <Switch
                    checked={option.enabled}
                    onCheckedChange={(checked) => handleCleanupOptionChange(index, 'enabled', checked)}
                    disabled={isLoading}
                  />
                </div>
                {option.enabled && (
                  <div className='flex items-center gap-2'>
                    <Label htmlFor={`cleanup-days-${index}`}>{t('system.storage.policy.cleanupDays')}</Label>
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
          <Button onClick={handleSaveStoragePolicy} disabled={isLoading || updateStoragePolicy.isPending} className='min-w-[100px]'>
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
