'use client'

import { useTranslation } from 'react-i18next'
import { IconAlertTriangle, IconFlask, IconLoader2 } from '@tabler/icons-react'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { Channel } from '../data/schema'
import { useUpdateChannelStatus, useTestChannel } from '../data/channels'
import { useState } from 'react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

export function ChannelsStatusDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannelStatus = useUpdateChannelStatus()
  const testChannel = useTestChannel()
  const [testResult, setTestResult] = useState<{ 
    success: boolean; 
    latency?: number; 
    message?: string | null;
    error?: string | null;
  } | null>(null)

  const handleStatusChange = async () => {
    const newStatus = currentRow.status === 'enabled' ? 'disabled' : 'enabled'
    
    try {
      await updateChannelStatus.mutateAsync({
        id: currentRow.id,
        status: newStatus,
      })
      onOpenChange(false)
      setTestResult(null)
    } catch (error) {
      console.error('Failed to update channel status:', error)
    }
  }

  const handleTestChannel = async () => {
    try {
      const result = await testChannel.mutateAsync({
        channelID: currentRow.id,
        modelID: currentRow.defaultTestModel || undefined,
      })
      setTestResult({ 
        success: result.success, 
        latency: result.latency,
        message: result.message,
        error: result.error
      })
    } catch (error) {
      setTestResult({ 
        success: false, 
        error: error instanceof Error ? error.message : 'Unknown error occurred'
      })
    }
  }

  const isDisabling = currentRow.status === 'enabled'
  const title = isDisabling ? t('channels.dialogs.status.disable.title') : t('channels.dialogs.status.enable.title')
  
  // Enhanced description with warning for enabling
  const getDescription = () => {
    if (isDisabling) {
      return t('channels.dialogs.status.disable.description', { name: currentRow.name })
    }
    
    const baseDescription = t('channels.dialogs.status.enable.description', { name: currentRow.name })
    const warningText = t('channels.dialogs.status.enable.warning')
    return (
      <div className="space-y-3">
        <p>{baseDescription}</p>
        <div className="p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-md">
          <div className="flex items-start space-x-2">
            <IconAlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" />
            <div className="text-sm text-amber-800 dark:text-amber-200">
              <p className="font-medium">{t('channels.dialogs.status.enable.warningTitle')}</p>
              <p className="mt-1">{warningText}</p>
            </div>
          </div>
        </div>
        
        {/* Test section */}
        {currentRow.defaultTestModel && (
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{t('channels.dialogs.status.enable.testRecommended')}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={handleTestChannel}
                disabled={testChannel.isPending}
                className="h-8"
              >
                {testChannel.isPending ? (
                  <IconLoader2 className="h-3 w-3 animate-spin mr-1" />
                ) : (
                  <IconFlask className="h-3 w-3 mr-1" />
                )}
                {t('channels.dialogs.status.enable.testButton')}
              </Button>
            </div>
            
            {testResult && (
              <div className={`p-3 rounded text-sm ${
                testResult.success 
                  ? 'bg-green-50 dark:bg-green-900/20 text-green-800 dark:text-green-200 border border-green-200 dark:border-green-800'
                  : 'bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200 border border-red-200 dark:border-red-800'
              }`}>
                <div className="space-y-2">
                  <div className="font-medium">
                    {testResult.success 
                      ? t('channels.dialogs.status.enable.testSuccess', { latency: testResult.latency?.toFixed(2) })
                      : t('channels.dialogs.status.enable.testFailed')
                    }
                  </div>
                  
                  {/* Show test message if available */}
                  {testResult.message && testResult.success && (
                    <div className="text-xs opacity-75">
                      <span className="font-medium">{t('channels.dialogs.status.enable.testMessage')}:</span> {testResult.message}
                    </div>
                  )}
                  
                  {/* Show detailed error if test failed */}
                  {testResult.error && !testResult.success && (
                    <div className="text-xs">
                      <span className="font-medium">{t('channels.dialogs.status.enable.errorDetails')}:</span>
                      <div className="mt-1 p-2 bg-red-100 dark:bg-red-900/30 rounded border-l-2 border-red-400 dark:border-red-600">
                        {testResult.error}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    )
  }

  const actionText = isDisabling ? t('channels.dialogs.status.disable.button') : t('channels.dialogs.status.enable.button')

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleStatusChange}
      disabled={updateChannelStatus.isPending}
      title={
        <span className={isDisabling ? 'text-destructive' : 'text-green-600'}>
          <IconAlertTriangle
            className={`${isDisabling ? 'stroke-destructive' : 'stroke-green-600'} mr-1 inline-block`}
            size={18}
          />
          {title}
        </span>
      }
      desc={getDescription()}
      confirmText={actionText}
      cancelBtnText={t('channels.dialogs.buttons.cancel')}
    />
  )
}