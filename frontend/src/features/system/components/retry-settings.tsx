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
import { useMyProjects } from '@/features/projects/data/projects';
import { useSelectedProjectId } from '@/stores/projectStore';
import {
  applyCooldownSelectionChange,
  clampCooldownMinutes,
  projectIDToNumber,
  resolveCooldownSelection,
} from '../data/client-cancellation';
import {
  useRetryPolicy,
  useUpdateRetryPolicy,
  type ClientCancellationAction,
  type ClientCancellationPolicyInput,
  type ClientCancellationScope,
  type RetryPolicyInput,
} from '../data/system';

const CLIENT_CANCELLATION_COOLDOWN_PRESETS = [5, 10, 20, 30, 60] as const;

function ClientCancellationTimingFields({
  idPrefix,
  policy,
  onChange,
}: {
  idPrefix: string;
  policy: ClientCancellationPolicyInput;
  onChange: (field: 'threshold' | 'windowMinutes' | 'cooldownMinutes' | 'action', value: number | ClientCancellationAction) => void;
}) {
  const { t } = useTranslation();
  const cooldownMinutes = policy.cooldownMinutes ?? 30;
  // Choosing "custom" keeps the current minutes but switches the control into
  // free-input mode. Without this local flag the select would snap straight back
  // to the preset that still matches the value and the number input would never
  // appear.
  const [customCooldown, setCustomCooldown] = useState(false);
  // The stored number alone cannot express "custom" (it still matches a preset),
  // so the flag and the derived selection keep the entry clickable.
  const cooldownSelection = resolveCooldownSelection(cooldownMinutes, customCooldown);

  return (
    <div className='grid gap-4 md:grid-cols-4'>
      <div className='space-y-2'>
        <Label htmlFor={`${idPrefix}-threshold`}>{t('system.retry.clientCancellation.threshold.label')}</Label>
        <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.threshold.description')}</div>
        <Input
          id={`${idPrefix}-threshold`}
          type='number'
          min='1'
          max='100'
          value={policy.threshold ?? 2}
          onChange={(e) => {
            if (e.target.value.trim() === '') {
              return;
            }
            onChange('threshold', Math.max(1, parseInt(e.target.value, 10) || 1));
          }}
          className='w-32'
        />
      </div>

      <div className='space-y-2'>
        <Label htmlFor={`${idPrefix}-window`}>{t('system.retry.clientCancellation.window.label')}</Label>
        <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.window.description')}</div>
        <Input
          id={`${idPrefix}-window`}
          type='number'
          min='1'
          max='10080'
          value={policy.windowMinutes ?? 10}
          onChange={(e) => {
            if (e.target.value.trim() === '') {
              return;
            }
            onChange('windowMinutes', Math.max(1, parseInt(e.target.value, 10) || 1));
          }}
          className='w-32'
        />
      </div>

      <div className='space-y-2'>
        <Label htmlFor={`${idPrefix}-action`}>{t('system.retry.clientCancellation.action.label')}</Label>
        <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.action.description')}</div>
        <Select value={policy.action || 'switch_and_cooldown'} onValueChange={(value) => onChange('action', value as ClientCancellationAction)}>
          <SelectTrigger id={`${idPrefix}-action`} className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='switch_and_cooldown'>{t('system.retry.clientCancellation.action.options.switchAndCooldown')}</SelectItem>
            <SelectItem value='cooldown_only'>{t('system.retry.clientCancellation.action.options.cooldownOnly')}</SelectItem>
            <SelectItem value='cooldown_model'>{t('system.retry.clientCancellation.action.options.cooldownModel')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className='space-y-2'>
        <Label htmlFor={`${idPrefix}-cooldown-preset`}>{t('system.retry.clientCancellation.cooldown.label')}</Label>
        <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.cooldown.description')}</div>
        <Select
          value={cooldownSelection.value}
          onValueChange={(value) => {
            const change = applyCooldownSelectionChange(value);
            setCustomCooldown(change.custom);
            if (change.minutes !== undefined) {
              onChange('cooldownMinutes', change.minutes);
            }
          }}
        >
          <SelectTrigger id={`${idPrefix}-cooldown-preset`} className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CLIENT_CANCELLATION_COOLDOWN_PRESETS.map((minutes) => (
              <SelectItem key={minutes} value={String(minutes)}>
                {t(`system.retry.clientCancellation.cooldown.presets.${minutes}`)}
              </SelectItem>
            ))}
            <SelectItem value='custom'>{t('system.retry.clientCancellation.cooldown.custom')}</SelectItem>
          </SelectContent>
        </Select>
        {cooldownSelection.custom && (
          <div className='flex items-center gap-2'>
            <Input
              id={`${idPrefix}-cooldown-custom`}
              type='number'
              min='1'
              max='10080'
              value={cooldownMinutes}
              onChange={(e) => {
                // An empty field is a transient state while the user retypes;
                // ignoring it keeps the stored duration from collapsing to 1.
                const raw = e.target.value;
                if (raw.trim() === '') {
                  setCustomCooldown(true);
                  return;
                }
                setCustomCooldown(true);
                onChange('cooldownMinutes', clampCooldownMinutes(parseInt(raw, 10)));
              }}
              className='w-32'
            />
            <span className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.cooldown.minutes')}</span>
          </div>
        )}
      </div>
    </div>
  );
}

export function RetrySettings() {
  const { t } = useTranslation();
  const { data: retryPolicy, isLoading } = useRetryPolicy();
  const updateRetryPolicy = useUpdateRetryPolicy();
  const { data: projects = [] } = useMyProjects();
  const selectedProjectId = useSelectedProjectId();
  const [cancellationProjectId, setCancellationProjectId] = useState('');

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
    clientCancellationPolicy: {
      enabled: true,
      threshold: 2,
      cooldownMinutes: 30,
      action: 'switch_and_cooldown',
      scope: 'global',
      projectOverrides: [],
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
        clientCancellationPolicy: {
          enabled: retryPolicy.clientCancellationPolicy?.enabled ?? true,
          threshold: retryPolicy.clientCancellationPolicy?.threshold ?? 2,
          windowMinutes: retryPolicy.clientCancellationPolicy?.windowMinutes ?? 10,
          cooldownMinutes: retryPolicy.clientCancellationPolicy?.cooldownMinutes ?? 30,
          action: retryPolicy.clientCancellationPolicy?.action ?? 'switch_and_cooldown',
          scope: retryPolicy.clientCancellationPolicy?.scope ?? 'global',
          projectOverrides: retryPolicy.clientCancellationPolicy?.projectOverrides ?? [],
        },
      });
    }
  }, [retryPolicy]);

  useEffect(() => {
    if (cancellationProjectId) return;
    const initialProjectId = selectedProjectId || (projects[0]?.id ? String(projects[0].id) : '');
    if (initialProjectId) setCancellationProjectId(initialProjectId);
  }, [cancellationProjectId, projects, selectedProjectId]);

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

  const handleClientCancellationChange = useCallback(
    (field: 'enabled' | 'threshold' | 'cooldownMinutes' | 'action' | 'scope', value: boolean | number | string) => {
      setFormData((prev) => ({
        ...prev,
        clientCancellationPolicy: {
          ...prev.clientCancellationPolicy,
          [field]: value,
        },
      }));
    },
    []
  );

  // Project ids arrive as "gid://axonhub/project/<id>" GUIDs while the override
  // stores the plain numeric id, so it has to be decoded before matching/saving.
  const cancellationProjectID = projectIDToNumber(cancellationProjectId);
  const activeProjectOverride = formData.clientCancellationPolicy?.projectOverrides?.find(
    (override) => override.projectId === cancellationProjectID
  );

  const handleProjectCancellationOverrideToggle = useCallback(
    (enabled: boolean) => {
      if (!cancellationProjectID) return;
      setFormData((prev) => {
        const policy = prev.clientCancellationPolicy;
        const overrides = policy?.projectOverrides ?? [];
        const existing = overrides.find((override) => override.projectId === cancellationProjectID);
        if (existing) {
          return {
            ...prev,
            clientCancellationPolicy: {
              ...policy,
              projectOverrides: overrides.map((override) =>
                override.projectId === cancellationProjectID ? { ...override, enabled } : override
              ),
            },
          };
        }
        return {
          ...prev,
          clientCancellationPolicy: {
            ...policy,
            projectOverrides: [
              ...overrides,
              {
                projectId: cancellationProjectID,
                enabled,
                threshold: policy?.threshold ?? 2,
                windowMinutes: policy?.windowMinutes ?? 10,
                cooldownMinutes: policy?.cooldownMinutes ?? 30,
                action: policy?.action ?? 'switch_and_cooldown',
              },
            ],
          },
        };
      });
    },
    [cancellationProjectID]
  );

  const handleProjectCancellationChange = useCallback(
    (field: 'threshold' | 'windowMinutes' | 'cooldownMinutes' | 'action', value: number | ClientCancellationAction) => {
      if (!cancellationProjectID) return;
      setFormData((prev) => {
        const policy = prev.clientCancellationPolicy;
        return {
          ...prev,
          clientCancellationPolicy: {
            ...policy,
            projectOverrides: (policy?.projectOverrides ?? []).map((override) =>
              override.projectId === cancellationProjectID ? { ...override, [field]: value } : override
            ),
          },
        };
      });
    },
    [cancellationProjectID]
  );

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

          <div id='client-cancellation' className='space-y-4 rounded-lg border p-4'>
            <div className='flex items-center justify-between'>
              <div className='space-y-0.5'>
                <Label htmlFor='client-cancellation-enabled' className='text-base'>
                  {t('system.retry.clientCancellation.label')}
                </Label>
                <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.description')}</div>
              </div>
              <Switch
                id='client-cancellation-enabled'
                checked={formData.clientCancellationPolicy?.enabled ?? true}
                onCheckedChange={(checked) => handleClientCancellationChange('enabled', checked)}
              />
            </div>

            {(formData.clientCancellationPolicy?.enabled ?? true) && (
              <div className='space-y-5'>
                <div className='space-y-2'>
                  <Label htmlFor='client-cancellation-scope'>{t('system.retry.clientCancellation.scope.label')}</Label>
                  <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.scope.description')}</div>
                  <Select
                    value={formData.clientCancellationPolicy?.scope || 'global'}
                    onValueChange={(value) => handleClientCancellationChange('scope', value as ClientCancellationScope)}
                  >
                    <SelectTrigger id='client-cancellation-scope' className='w-full md:w-80'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='global'>{t('system.retry.clientCancellation.scope.options.global')}</SelectItem>
                      <SelectItem value='project_first'>
                        {t('system.retry.clientCancellation.scope.options.projectFirst')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className='bg-muted/20 space-y-3 rounded-md border p-4'>
                  <div className='font-medium'>{t('system.retry.clientCancellation.globalTitle')}</div>
                  <ClientCancellationTimingFields
                    idPrefix='client-cancellation-global'
                    policy={formData.clientCancellationPolicy ?? {}}
                    onChange={handleClientCancellationChange}
                  />
                </div>

                {formData.clientCancellationPolicy?.scope === 'project_first' && (
                  <div className='bg-muted/20 space-y-4 rounded-md border p-4'>
                    <div className='space-y-2'>
                      <Label htmlFor='client-cancellation-project'>{t('system.retry.clientCancellation.project.label')}</Label>
                      <div className='text-muted-foreground text-sm'>{t('system.retry.clientCancellation.project.description')}</div>
                      <Select value={cancellationProjectId} onValueChange={(value) => value && setCancellationProjectId(value)}>
                        <SelectTrigger id='client-cancellation-project' className='w-full md:w-80'>
                          <SelectValue placeholder={t('system.retry.clientCancellation.project.placeholder')} />
                        </SelectTrigger>
                        <SelectContent>
                          {projects.map((project) => (
                            <SelectItem key={project.id} value={String(project.id)}>
                              {project.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className='flex items-center justify-between gap-4'>
                      <div className='space-y-0.5'>
                        <Label htmlFor='client-cancellation-project-enabled' className='text-base'>
                          {t('system.retry.clientCancellation.project.overrideLabel')}
                        </Label>
                        <div className='text-muted-foreground text-sm'>
                          {t('system.retry.clientCancellation.project.overrideDescription')}
                        </div>
                      </div>
                      <Switch
                        id='client-cancellation-project-enabled'
                        checked={activeProjectOverride?.enabled ?? false}
                        disabled={!cancellationProjectID}
                        onCheckedChange={handleProjectCancellationOverrideToggle}
                      />
                    </div>

                    {activeProjectOverride?.enabled && (
                      <ClientCancellationTimingFields
                        key={`client-cancellation-project-${cancellationProjectID}`}
                        idPrefix='client-cancellation-project'
                        policy={activeProjectOverride}
                        onChange={handleProjectCancellationChange}
                      />
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

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
