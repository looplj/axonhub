'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Loader2, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useRetryPolicy, useUpdateRetryPolicy, type RetryPolicyInput } from '../data/system';

export function RetrySettings() {
  const { t } = useTranslation();
  const { data: retryPolicy, isLoading } = useRetryPolicy();
  const updateRetryPolicy = useUpdateRetryPolicy();

  const [formData, setFormData] = useState<RetryPolicyInput>({
    enabled: true,
    maxChannelRetries: 3,
    maxSingleChannelRetries: 2,
    retryDelayMs: 1000,
    loadBalancerStrategy: 'adaptive',
    autoDisableChannel: {
      enabled: false,
      statuses: [],
    },
  });

  useEffect(() => {
    if (retryPolicy) {
      setFormData({
        enabled: retryPolicy.enabled,
        maxChannelRetries: retryPolicy.maxChannelRetries,
        maxSingleChannelRetries: retryPolicy.maxSingleChannelRetries,
        retryDelayMs: retryPolicy.retryDelayMs,
        loadBalancerStrategy: retryPolicy.loadBalancerStrategy,
        autoDisableChannel: {
          enabled: retryPolicy.autoDisableChannel?.enabled || false,
          statuses: retryPolicy.autoDisableChannel?.statuses || [],
        },
      });
    }
  }, [retryPolicy]);

  const handleInputChange = useCallback((field: keyof RetryPolicyInput, value: string | boolean | number) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
  }, []);

  const handleAutoDisableChannelChange = useCallback((field: 'enabled', value: boolean) => {
    setFormData((prev) => ({
      ...prev,
      autoDisableChannel: {
        ...prev.autoDisableChannel,
        [field]: value,
      },
    }));
  }, []);

  const handleStatusChange = useCallback((index: number, field: 'status' | 'times', value: number) => {
    setFormData((prev) => ({
      ...prev,
      autoDisableChannel: {
        ...prev.autoDisableChannel,
        statuses: prev.autoDisableChannel?.statuses?.map((s, i) => (i === index ? { ...s, [field]: value } : s)) || [],
      },
    }));
  }, []);

  const addStatus = useCallback(() => {
    setFormData((prev) => ({
      ...prev,
      autoDisableChannel: {
        ...prev.autoDisableChannel,
        statuses: [...(prev.autoDisableChannel?.statuses || []), { status: 500, times: 3 }],
      },
    }));
  }, []);

  const removeStatus = useCallback((index: number) => {
    setFormData((prev) => ({
      ...prev,
      autoDisableChannel: {
        ...prev.autoDisableChannel,
        statuses: prev.autoDisableChannel?.statuses?.filter((_, i) => i !== index) || [],
      },
    }));
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateRetryPolicy.mutateAsync(formData);
    },
    [updateRetryPolicy, formData]
  );

  if (isLoading) {
    return (
      <div className='flex items-center justify-center p-8'>
        <Loader2 className='h-8 w-8 animate-spin' />
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.retry.title')}</CardTitle>
        <CardDescription>{t('system.retry.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className='space-y-6'>
          {/* Enable/Disable Retry */}
          <div className='flex items-center justify-between' id='retry-enabled-switch'>
            <div className='space-y-0.5'>
              <Label htmlFor='retry-enabled' className='text-base'>
                {t('system.retry.enabled.label')}
              </Label>
              <div className='text-muted-foreground text-sm'>{t('system.retry.enabled.description')}</div>
            </div>
            <Switch id='retry-enabled' checked={formData.enabled} onCheckedChange={(checked) => handleInputChange('enabled', checked)} />
          </div>

          <Separator />

          {/* Retry Configuration - Only show when enabled */}
          {formData.enabled && (
            <div className='space-y-4'>
              <div className='space-y-2'>
                <Label htmlFor='load-balancer-strategy'>{t('system.retry.loadBalancerStrategy.label')}</Label>
                <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.loadBalancerStrategy.description')}</div>
                <Select
                  value={formData.loadBalancerStrategy || 'adaptive'}
                  onValueChange={(value) => value && handleInputChange('loadBalancerStrategy', value)}
                >
                  <SelectTrigger id='load-balancer-strategy' className='w-56'>
                    <SelectValue placeholder={t('system.retry.loadBalancerStrategy.placeholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='adaptive'>{t('system.retry.loadBalancerStrategy.options.adaptive')}</SelectItem>
                    <SelectItem value='weighted'>{t('system.retry.loadBalancerStrategy.options.weighted')}</SelectItem>
                    <SelectItem value='circuit-breaker'>{t('system.retry.loadBalancerStrategy.options.healthAware')}</SelectItem>
                  </SelectContent>
                </Select>

                {/* Strategy Documentation */}
                {formData.loadBalancerStrategy && (
                  <div className='bg-muted/50 mt-3 rounded-md border p-3'>
                    <div className='text-muted-foreground text-xs leading-relaxed'>
                      {t(`system.retry.loadBalancerStrategy.documentation.${formData.loadBalancerStrategy}`)}
                    </div>
                  </div>
                )}
              </div>

              {/* Max Channel Retries */}
              <div className='space-y-2' id='retry-max-retries'>
                <Label htmlFor='max-channel-retries'>{t('system.retry.maxChannelRetries.label')}</Label>
                <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.maxChannelRetries.description')}</div>
                <Input
                  id='max-channel-retries'
                  type='number'
                  min='0'
                  max='10'
                  value={formData.maxChannelRetries}
                  onChange={(e) => handleInputChange('maxChannelRetries', parseInt(e.target.value) || 0)}
                  className='w-32'
                />
              </div>

              {/* Max Single Channel Retries */}
              <div className='space-y-2'>
                <Label htmlFor='max-single-channel-retries'>{t('system.retry.maxSingleChannelRetries.label')}</Label>
                <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.maxSingleChannelRetries.description')}</div>
                <Input
                  id='max-single-channel-retries'
                  type='number'
                  min='0'
                  max='5'
                  value={formData.maxSingleChannelRetries}
                  onChange={(e) => handleInputChange('maxSingleChannelRetries', parseInt(e.target.value) || 0)}
                  className='w-32'
                />
              </div>

              {/* Retry Delay */}
              <div className='space-y-2'>
                <Label htmlFor='retry-delay'>{t('system.retry.retryDelayMs.label')}</Label>
                <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.retryDelayMs.description')}</div>
                <div className='flex items-center space-x-2'>
                  <Input
                    id='retry-delay'
                    type='number'
                    min='100'
                    max='10000'
                    step='100'
                    value={formData.retryDelayMs}
                    onChange={(e) => handleInputChange('retryDelayMs', parseInt(e.target.value) || 1000)}
                    className='w-32'
                  />
                  <span className='text-muted-foreground text-sm'>ms</span>
                </div>
              </div>

              <Separator />

              {/* Auto Disable Channel */}
              <div className='space-y-4'>
                <div className='flex items-center justify-between'>
                  <div className='space-y-0.5'>
                    <Label htmlFor='auto-disable-channel' className='text-base'>
                      {t('system.retry.autoDisableChannel.label')}
                    </Label>
                    <div className='text-muted-foreground text-sm'>{t('system.retry.autoDisableChannel.description')}</div>
                  </div>
                  <Switch
                    id='auto-disable-channel'
                    checked={formData.autoDisableChannel?.enabled || false}
                    onCheckedChange={(checked) => handleAutoDisableChannelChange('enabled', checked)}
                  />
                </div>

                {formData.autoDisableChannel?.enabled && (
                  <div className='space-y-3'>
                    <div className='flex items-center justify-between'>
                      <Label className='text-sm font-medium'>{t('system.retry.autoDisableChannel.statuses.label')}</Label>
                      <Button type='button' variant='outline' size='sm' onClick={addStatus}>
                        <Plus className='mr-1 h-4 w-4' />
                        {t('system.retry.autoDisableChannel.statuses.add')}
                      </Button>
                    </div>

                    {formData.autoDisableChannel?.statuses && formData.autoDisableChannel.statuses.length > 0 ? (
                      <div className='space-y-2'>
                        {formData.autoDisableChannel.statuses.map((statusItem, index) => (
                          <div key={index} className='flex items-center space-x-2'>
                            <Input
                              type='number'
                              placeholder={t('system.retry.autoDisableChannel.statuses.statusPlaceholder')}
                              value={statusItem.status}
                              onChange={(e) => handleStatusChange(index, 'status', parseInt(e.target.value) || 0)}
                              className='w-24'
                              min='400'
                              max='599'
                            />
                            <span className='text-muted-foreground text-sm'>{t('system.retry.autoDisableChannel.statuses.times')}</span>
                            <Input
                              type='number'
                              placeholder={t('system.retry.autoDisableChannel.statuses.timesPlaceholder')}
                              value={statusItem.times}
                              onChange={(e) => handleStatusChange(index, 'times', parseInt(e.target.value) || 0)}
                              className='w-24'
                              min='1'
                              max='100'
                            />
                            <Button type='button' variant='ghost' size='icon' onClick={() => removeStatus(index)}>
                              <Trash2 className='h-4 w-4' />
                            </Button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className='text-muted-foreground text-sm'>{t('system.retry.autoDisableChannel.statuses.empty')}</div>
                    )}
                  </div>
                )}
              </div>
            </div>
          )}

          <Separator />

          {/* Submit Button */}
          <div className='flex justify-end'>
            <Button type='submit' disabled={updateRetryPolicy.isPending} className='min-w-24'>
              {updateRetryPolicy.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
