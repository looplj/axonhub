'use client'

import React, { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Loader2 } from 'lucide-react'
import { useRetryPolicy, useUpdateRetryPolicy, type RetryPolicyInput } from '../data/system'

export function RetrySettings() {
  const { t } = useTranslation()
  const { data: retryPolicy, isLoading } = useRetryPolicy()
  const updateRetryPolicy = useUpdateRetryPolicy()

  const [formData, setFormData] = useState<RetryPolicyInput>({
    enabled: true,
    maxChannelRetries: 3,
    maxSingleChannelRetries: 2,
    retryDelayMs: 1000,
  })

  useEffect(() => {
    if (retryPolicy) {
      setFormData({
        enabled: retryPolicy.enabled,
        maxChannelRetries: retryPolicy.maxChannelRetries,
        maxSingleChannelRetries: retryPolicy.maxSingleChannelRetries,
        retryDelayMs: retryPolicy.retryDelayMs,
      })
    }
  }, [retryPolicy])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    await updateRetryPolicy.mutateAsync(formData)
  }

  const handleInputChange = (field: keyof RetryPolicyInput, value: string | boolean | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }))
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="h-8 w-8 animate-spin" />
      </div>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.retry.title')}</CardTitle>
        <CardDescription>
          {t('system.retry.description')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-6">
          {/* Enable/Disable Retry */}
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="retry-enabled" className="text-base">
                {t('system.retry.enabled.label')}
              </Label>
              <div className="text-sm text-muted-foreground">
                {t('system.retry.enabled.description')}
              </div>
            </div>
            <Switch
              id="retry-enabled"
              checked={formData.enabled}
              onCheckedChange={(checked) => handleInputChange('enabled', checked)}
            />
          </div>

          <Separator />

          {/* Retry Configuration - Only show when enabled */}
          {formData.enabled && (
            <div className="space-y-4">
              {/* Max Channel Retries */}
              <div className="space-y-2">
                <Label htmlFor="max-channel-retries">
                  {t('system.retry.maxChannelRetries.label')}
                </Label>
                <div className="text-sm text-muted-foreground mb-2">
                  {t('system.retry.maxChannelRetries.description')}
                </div>
                <Input
                  id="max-channel-retries"
                  type="number"
                  min="0"
                  max="10"
                  value={formData.maxChannelRetries}
                  onChange={(e) => handleInputChange('maxChannelRetries', parseInt(e.target.value) || 0)}
                  className="w-32"
                />
              </div>

              {/* Max Single Channel Retries */}
              <div className="space-y-2">
                <Label htmlFor="max-single-channel-retries">
                  {t('system.retry.maxSingleChannelRetries.label')}
                </Label>
                <div className="text-sm text-muted-foreground mb-2">
                  {t('system.retry.maxSingleChannelRetries.description')}
                </div>
                <Input
                  id="max-single-channel-retries"
                  type="number"
                  min="0"
                  max="5"
                  value={formData.maxSingleChannelRetries}
                  onChange={(e) => handleInputChange('maxSingleChannelRetries', parseInt(e.target.value) || 0)}
                  className="w-32"
                />
              </div>

              {/* Retry Delay */}
              <div className="space-y-2">
                <Label htmlFor="retry-delay">
                  {t('system.retry.retryDelayMs.label')}
                </Label>
                <div className="text-sm text-muted-foreground mb-2">
                  {t('system.retry.retryDelayMs.description')}
                </div>
                <div className="flex items-center space-x-2">
                  <Input
                    id="retry-delay"
                    type="number"
                    min="100"
                    max="10000"
                    step="100"
                    value={formData.retryDelayMs}
                    onChange={(e) => handleInputChange('retryDelayMs', parseInt(e.target.value) || 1000)}
                    className="w-32"
                  />
                  <span className="text-sm text-muted-foreground">ms</span>
                </div>
              </div>
            </div>
          )}

          <Separator />

          {/* Submit Button */}
          <div className="flex justify-end">
            <Button 
              type="submit" 
              disabled={updateRetryPolicy.isPending}
              className="min-w-24"
            >
              {updateRetryPolicy.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                t('common.buttons.save')
              )}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
