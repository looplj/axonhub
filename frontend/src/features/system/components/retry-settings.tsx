'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { ApiKeyAutoDisableRulesEditor } from '@/features/channels/components/api-key-auto-disable-rules-editor';
import {
  type ApiKeyAutoDisableRuleFormValue,
  RECOMMENDED_GLOBAL_AUTO_DISABLE_RULES,
  serializeApiKeyAutoDisableRules,
  toApiKeyAutoDisableRuleFormValues,
} from '@/features/channels/data/auto-disable';
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
    streamFirstEventTimeoutSeconds: 0,
    nonStreamResponseTimeoutSeconds: 0,
    loadBalancerStrategy: 'adaptive',
    traceStickyMode: 'PREFER_PREVIOUS_CHANNEL',
    emptyResponseDetection: false,
    upstreamErrorPolicy: {
      mode: 'passthrough',
      customMessage: '',
    },
    autoDisableChannel: {
      enabled: false,
      rules: [],
    },
  });

  useEffect(() => {
    if (retryPolicy) {
      setFormData({
        enabled: retryPolicy.enabled,
        maxChannelRetries: retryPolicy.maxChannelRetries,
        maxSingleChannelRetries: retryPolicy.maxSingleChannelRetries,
        retryDelayMs: retryPolicy.retryDelayMs,
        streamFirstEventTimeoutSeconds: retryPolicy.streamFirstEventTimeoutSeconds,
        nonStreamResponseTimeoutSeconds: retryPolicy.nonStreamResponseTimeoutSeconds,
        loadBalancerStrategy: retryPolicy.loadBalancerStrategy,
        traceStickyMode: retryPolicy.traceStickyMode,
        emptyResponseDetection: retryPolicy.emptyResponseDetection,
        upstreamErrorPolicy: {
          mode: retryPolicy.upstreamErrorPolicy?.mode || 'passthrough',
          customMessage: retryPolicy.upstreamErrorPolicy?.customMessage || '',
        },
        autoDisableChannel: {
          enabled: retryPolicy.autoDisableChannel?.enabled || false,
          rules: toApiKeyAutoDisableRuleFormValues(retryPolicy.autoDisableChannel?.rules),
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

  const handleUpstreamErrorPolicyChange = useCallback((field: 'mode' | 'customMessage', value: string) => {
    setFormData((prev) => ({
      ...prev,
      upstreamErrorPolicy: {
        ...prev.upstreamErrorPolicy,
        [field]: value,
      },
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

  const handleAutoDisableRulesChange = useCallback((rules: ApiKeyAutoDisableRuleFormValue[]) => {
    setFormData((prev) => {
      if (JSON.stringify(prev.autoDisableChannel?.rules) === JSON.stringify(rules)) {
        return prev;
      }
      return {
        ...prev,
        autoDisableChannel: {
          ...prev.autoDisableChannel,
          rules,
        },
      };
    });
  }, []);

  const insertRecommendedAutoDisableRules = useCallback(() => {
    setFormData((prev) => ({
      ...prev,
      autoDisableChannel: {
        ...prev.autoDisableChannel,
        rules: toApiKeyAutoDisableRuleFormValues(RECOMMENDED_GLOBAL_AUTO_DISABLE_RULES),
      },
    }));
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateRetryPolicy.mutateAsync({
        ...formData,
        autoDisableChannel: {
          enabled: formData.autoDisableChannel?.enabled || false,
          rules: serializeApiKeyAutoDisableRules(toApiKeyAutoDisableRuleFormValues(formData.autoDisableChannel?.rules)),
        },
      });
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

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='upstream-error-mode'>{t('system.retry.upstreamErrorPolicy.label')}</Label>
              <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.upstreamErrorPolicy.description')}</div>
              <Select
                value={formData.upstreamErrorPolicy?.mode || 'passthrough'}
                onValueChange={(value) => value && handleUpstreamErrorPolicyChange('mode', value)}
              >
                <SelectTrigger id='upstream-error-mode' className='w-56'>
                  <SelectValue placeholder={t('system.retry.upstreamErrorPolicy.placeholder')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='passthrough'>{t('system.retry.upstreamErrorPolicy.options.passthrough')}</SelectItem>
                  <SelectItem value='hidden'>{t('system.retry.upstreamErrorPolicy.options.hidden')}</SelectItem>
                  <SelectItem value='custom'>{t('system.retry.upstreamErrorPolicy.options.custom')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {formData.upstreamErrorPolicy?.mode === 'custom' && (
              <div className='space-y-2'>
                <Label htmlFor='upstream-error-custom-message'>{t('system.retry.upstreamErrorPolicy.customMessage.label')}</Label>
                <Textarea
                  id='upstream-error-custom-message'
                  value={formData.upstreamErrorPolicy?.customMessage || ''}
                  onChange={(e) => handleUpstreamErrorPolicyChange('customMessage', e.target.value)}
                  placeholder={t('system.retry.upstreamErrorPolicy.customMessage.placeholder')}
                  className='min-h-20'
                />
              </div>
            )}
          </div>

          <Separator />

          {/* Retry Configuration - Only show when enabled */}
          {formData.enabled && (
            <div className='space-y-4'>
              <div className='bg-muted/20 space-y-4 rounded-lg border p-4'>
                <div className='flex flex-wrap gap-x-8 gap-y-5'>
                  <div className='min-w-56 flex-1 space-y-2'>
                    <Label htmlFor='load-balancer-strategy'>{t('system.retry.loadBalancerStrategy.label')}</Label>
                    <div className='text-muted-foreground text-sm'>{t('system.retry.loadBalancerStrategy.description')}</div>
                    <Select
                      value={formData.loadBalancerStrategy || 'adaptive'}
                      onValueChange={(value) => value && handleInputChange('loadBalancerStrategy', value)}
                    >
                      <SelectTrigger id='load-balancer-strategy' className='w-56'>
                        <SelectValue placeholder={t('system.retry.loadBalancerStrategy.placeholder')} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='adaptive'>{t('system.retry.loadBalancerStrategy.options.adaptive')}</SelectItem>
                        <SelectItem value='failover'>{t('system.retry.loadBalancerStrategy.options.failover')}</SelectItem>
                        <SelectItem value='circuit-breaker'>{t('system.retry.loadBalancerStrategy.options.circuitBreaker')}</SelectItem>
                        <SelectItem value='round-robin'>{t('system.retry.loadBalancerStrategy.options.roundRobin')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='min-w-56 flex-1 space-y-2'>
                    <Label htmlFor='trace-sticky-mode'>{t('system.retry.traceStickyMode.label')}</Label>
                    <div className='text-muted-foreground text-sm'>{t('system.retry.traceStickyMode.description')}</div>
                    <Select
                      value={formData.traceStickyMode || 'PREFER_PREVIOUS_CHANNEL'}
                      onValueChange={(value) => value && handleInputChange('traceStickyMode', value)}
                    >
                      <SelectTrigger id='trace-sticky-mode' className='w-56'>
                        <SelectValue placeholder={t('system.retry.traceStickyMode.placeholder')} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='PREFER_PREVIOUS_CHANNEL'>
                          {t('system.retry.traceStickyMode.options.preferPreviousChannel')}
                        </SelectItem>
                        <SelectItem value='DISABLED'>{t('system.retry.traceStickyMode.options.disabled')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                {formData.loadBalancerStrategy && (
                  <div className='bg-muted/50 rounded-md border p-3'>
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

              {/* Response Timeouts */}
              <div className='grid gap-4 md:grid-cols-2'>
                <div className='space-y-2'>
                  <Label htmlFor='stream-first-event-timeout'>{t('system.retry.streamFirstEventTimeoutSeconds.label')}</Label>
                  <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.streamFirstEventTimeoutSeconds.description')}</div>
                  <div className='flex items-center space-x-2'>
                    <Input
                      id='stream-first-event-timeout'
                      type='number'
                      min='0'
                      max='600'
                      value={formData.streamFirstEventTimeoutSeconds}
                      onChange={(e) => handleInputChange('streamFirstEventTimeoutSeconds', parseInt(e.target.value) || 0)}
                      className='w-32'
                    />
                    <span className='text-muted-foreground text-sm'>s</span>
                  </div>
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='non-stream-response-timeout'>{t('system.retry.nonStreamResponseTimeoutSeconds.label')}</Label>
                  <div className='text-muted-foreground mb-2 text-sm'>{t('system.retry.nonStreamResponseTimeoutSeconds.description')}</div>
                  <div className='flex items-center space-x-2'>
                    <Input
                      id='non-stream-response-timeout'
                      type='number'
                      min='0'
                      max='600'
                      value={formData.nonStreamResponseTimeoutSeconds}
                      onChange={(e) => handleInputChange('nonStreamResponseTimeoutSeconds', parseInt(e.target.value) || 0)}
                      className='w-32'
                    />
                    <span className='text-muted-foreground text-sm'>s</span>
                  </div>
                </div>
              </div>

              {/* Empty Response Detection */}
              <div className='flex items-center justify-between'>
                <div className='space-y-0.5'>
                  <Label htmlFor='empty-response-detection' className='text-base'>
                    {t('system.retry.emptyResponseDetection.label')}
                  </Label>
                  <div className='text-muted-foreground text-sm'>{t('system.retry.emptyResponseDetection.description')}</div>
                </div>
                <Switch
                  id='empty-response-detection'
                  checked={formData.emptyResponseDetection || false}
                  onCheckedChange={(checked) => handleInputChange('emptyResponseDetection', checked)}
                />
              </div>

              <Separator />

              {/* Auto Disable Channel */}
              <div id='auto-disable-channel' className='space-y-4'>
                <div className='flex items-center justify-between'>
                  <div className='space-y-0.5'>
                    <Label htmlFor='auto-disable-channel-enabled' className='text-base'>
                      {t('system.retry.autoDisableChannel.label')}
                    </Label>
                    <div className='text-muted-foreground text-sm'>{t('system.retry.autoDisableChannel.description')}</div>
                  </div>
                  <Switch
                    id='auto-disable-channel-enabled'
                    checked={formData.autoDisableChannel?.enabled || false}
                    onCheckedChange={(checked) => handleAutoDisableChannelChange('enabled', checked)}
                  />
                </div>

                <p className='text-muted-foreground text-sm'>{t('system.retry.autoDisableChannel.noDeleteCredential')}</p>

                {(formData.autoDisableChannel?.rules?.length ?? 0) === 0 && (
                  <Button type='button' variant='outline' size='sm' onClick={insertRecommendedAutoDisableRules}>
                    {t('system.retry.autoDisableChannel.recommendedRules')}
                  </Button>
                )}

                <ApiKeyAutoDisableRulesEditor
                  rules={(formData.autoDisableChannel?.rules ?? []) as ApiKeyAutoDisableRuleFormValue[]}
                  onChange={handleAutoDisableRulesChange}
                  allowDelete={false}
                />
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
