'use client'

import React, { useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { Loader2, Save } from 'lucide-react'
import { useSystemSettings, useUpdateSystemSettings } from '../data/system'
import { useSystemContext } from '../context/system-context'

export function SystemSettings() {
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
        <span className='ml-2 text-muted-foreground'>加载系统设置...</span>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>存储设置</CardTitle>
          <CardDescription>
            配置系统如何处理和存储响应数据
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='store-chunks'>存储响应块</Label>
              <div className='text-sm text-muted-foreground'>
                启用后，系统将在数据库中存储响应数据块，便于后续分析和处理
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
          <CardTitle>其他设置</CardTitle>
          <CardDescription>
            更多系统配置选项将在此处添加
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-muted-foreground'>
            更多配置选项即将推出...
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
                保存中...
              </>
            ) : (
              <>
                <Save className='mr-2 h-4 w-4' />
                保存设置
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  )
}