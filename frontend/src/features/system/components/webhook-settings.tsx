'use client';

import React, { useCallback, useEffect, useState } from 'react';
import { Loader2, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { proxyTypeSchema, type ProxyConfig, type ProxyType } from '@/features/channels/data/schema';
import { useProxyPresets, useUpdateWebhookNotifierConfig, useWebhookNotifierConfig, type WebhookNotifierConfig, type WebhookTarget } from '../data/system';

const AUTO_DISABLED_EVENT = 'channel.auto_disabled';

const DEFAULT_WEBHOOK_BODY_TEMPLATE = `{
  "event": "{{.Event}}",
  "severity": "{{.Severity}}",
  "occurred_at": "{{.OccurredAt}}",
  "channel": {
    "id": {{.Channel.ID}},
    "name": "{{.Channel.Name}}",
    "provider": "{{.Channel.Provider}}",
    "status": "{{.Channel.Status}}"
  },
  "trigger": {
    "status_code": {{.Trigger.StatusCode}},
    "threshold": {{.Trigger.Threshold}},
    "actual_count": {{.Trigger.ActualCount}},
    "reason": "{{.Trigger.Reason}}"
  }
}`;

function createDefaultTarget(index: number): WebhookTarget {
  return {
    name: index === 0 ? 'default' : `target-${index + 1}`,
    enabled: false,
    url: '',
    proxy: undefined,
    timeoutMs: 3000,
    headers: [{ key: 'Content-Type', value: 'application/json' }],
    body: DEFAULT_WEBHOOK_BODY_TEMPLATE,
  };
}

const DEFAULT_WEBHOOK_CONFIG: WebhookNotifierConfig = {
  targets: [],
  subscriptions: [],
};

export function WebhookSettings() {
  const { t } = useTranslation();
  const { data: webhookConfig, isLoading } = useWebhookNotifierConfig();
  const { data: proxyPresets = [] } = useProxyPresets();
  const updateWebhookNotifierConfig = useUpdateWebhookNotifierConfig();
  const [formData, setFormData] = useState<WebhookNotifierConfig>(DEFAULT_WEBHOOK_CONFIG);

  useEffect(() => {
    if (webhookConfig) {
      setFormData({
        targets: webhookConfig.targets || [],
        subscriptions: webhookConfig.subscriptions || [],
      });
    }
  }, [webhookConfig]);

  const getSubscribedTargetNames = useCallback(
    () => new Set(formData.subscriptions.find((subscription) => subscription.event === AUTO_DISABLED_EVENT)?.targetNames || []),
    [formData.subscriptions]
  );

  const isTargetSubscribed = useCallback(
    (targetName: string) => getSubscribedTargetNames().has(targetName),
    [getSubscribedTargetNames]
  );

  const addTarget = useCallback(() => {
    setFormData((prev) => ({
      ...prev,
      targets: [...prev.targets, createDefaultTarget(prev.targets.length)],
    }));
  }, []);

  const removeTarget = useCallback((index: number) => {
    setFormData((prev) => {
      const removedTarget = prev.targets[index];
      const nextTargets = prev.targets.filter((_, i) => i !== index);
      const nextSubscriptions = prev.subscriptions
        .map((subscription) => ({
          ...subscription,
          targetNames: subscription.targetNames.filter((name) => name !== removedTarget?.name),
        }))
        .filter((subscription) => subscription.targetNames.length > 0);

      return {
        ...prev,
        targets: nextTargets,
        subscriptions: nextSubscriptions,
      };
    });
  }, []);

  const handleTargetChange = useCallback((index: number, field: 'name' | 'url' | 'timeoutMs' | 'body' | 'enabled', value: string | number | boolean) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) => (i === index ? { ...target, [field]: value } : target)),
    }));
  }, []);

  const handleTargetProxyChange = useCallback((index: number, proxy: ProxyConfig | undefined) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) => (i === index ? { ...target, proxy } : target)),
    }));
  }, []);

  const handleTargetProxyTypeChange = useCallback((index: number, type: ProxyType) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) => {
        if (i !== index) {
          return target;
        }

        if (type === proxyTypeSchema.enum.disabled) {
          return { ...target, proxy: undefined };
        }

        return {
          ...target,
          proxy: {
            type,
            ...(type === proxyTypeSchema.enum.url ? {
              url: target.proxy?.type === proxyTypeSchema.enum.url ? target.proxy.url : '',
              username: target.proxy?.type === proxyTypeSchema.enum.url ? target.proxy.username : '',
              password: target.proxy?.type === proxyTypeSchema.enum.url ? target.proxy.password : '',
            } : {}),
          },
        };
      }),
    }));
  }, []);

  const handleTargetProxyFieldChange = useCallback((index: number, field: 'url' | 'username' | 'password', value: string) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) => {
        if (i !== index) {
          return target;
        }

        return {
          ...target,
          proxy: {
            type: target.proxy?.type || proxyTypeSchema.enum.url,
            url: field === 'url' ? value : (target.proxy?.url || ''),
            username: field === 'username' ? value : (target.proxy?.username || ''),
            password: field === 'password' ? value : (target.proxy?.password || ''),
          },
        };
      }),
    }));
  }, []);

  const handleTargetNameChange = useCallback((index: number, value: string) => {
    setFormData((prev) => {
      const previousName = prev.targets[index]?.name;
      const nextTargets = prev.targets.map((target, i) => (i === index ? { ...target, name: value } : target));
      const nextSubscriptions = prev.subscriptions.map((subscription) => ({
        ...subscription,
        targetNames: subscription.targetNames.map((targetName) => (targetName === previousName ? value : targetName)),
      }));

      return {
        ...prev,
        targets: nextTargets,
        subscriptions: nextSubscriptions,
      };
    });
  }, []);

  const handleHeaderChange = useCallback((targetIndex: number, headerIndex: number, field: 'key' | 'value', value: string) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) => {
        if (i !== targetIndex) {
          return target;
        }

        const headers = [...(target.headers || [])];
        headers[headerIndex] = { ...headers[headerIndex], [field]: value };

        return { ...target, headers };
      }),
    }));
  }, []);

  const addHeader = useCallback((targetIndex: number) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) =>
        i === targetIndex
          ? {
              ...target,
              headers: [...(target.headers || []), { key: '', value: '' }],
            }
          : target
      ),
    }));
  }, []);

  const removeHeader = useCallback((targetIndex: number, headerIndex: number) => {
    setFormData((prev) => ({
      ...prev,
      targets: prev.targets.map((target, i) =>
        i === targetIndex
          ? {
              ...target,
              headers: (target.headers || []).filter((_, idx) => idx !== headerIndex),
            }
          : target
      ),
    }));
  }, []);

  const handleSubscriptionChange = useCallback((targetName: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.subscriptions.find((subscription) => subscription.event === AUTO_DISABLED_EVENT);
      const nextTargetNames = checked
        ? Array.from(new Set([...(current?.targetNames || []), targetName]))
        : (current?.targetNames || []).filter((name) => name !== targetName);

      const nextSubscriptions = prev.subscriptions.filter((subscription) => subscription.event !== AUTO_DISABLED_EVENT);
      if (nextTargetNames.length > 0) {
        nextSubscriptions.push({
          event: AUTO_DISABLED_EVENT,
          targetNames: nextTargetNames,
        });
      }

      return {
        ...prev,
        subscriptions: nextSubscriptions,
      };
    });
  }, []);

  const validateTargets = useCallback(() => {
    const normalizedNames = new Set<string>();

    for (const target of formData.targets) {
      const normalizedName = target.name.trim();
      if (!normalizedName) {
        toast.error(t('system.webhook.validation.nameRequired'));
        return false;
      }

      if (normalizedNames.has(normalizedName)) {
        toast.error(t('system.webhook.validation.nameUnique'));
        return false;
      }

      normalizedNames.add(normalizedName)
      if (target.enabled && !target.url.trim()) {
        toast.error(t('system.webhook.validation.urlRequired'));
        return false;
      }
    }

    return true;
  }, [formData.targets, t]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      if (!validateTargets()) {
        return;
      }

      const normalizedTargets = formData.targets.map((target) => ({
        ...target,
        name: target.name.trim(),
        url: target.url.trim(),
        proxy: target.proxy?.type === proxyTypeSchema.enum.url
          ? (() => {
              const proxyURL = target.proxy.url?.trim() || '';
              if (!proxyURL) {
                return undefined;
              }

              return {
                ...target.proxy,
                url: proxyURL,
                username: target.proxy.username?.trim() || undefined,
                password: target.proxy.password?.trim() || undefined,
              };
            })()
          : target.proxy,
      }));

      const validTargetNames = new Set(normalizedTargets.map((target) => target.name));
      const normalizedSubscriptions = formData.subscriptions
        .map((subscription) => ({
          ...subscription,
          targetNames: subscription.targetNames.filter((targetName) => validTargetNames.has(targetName.trim())).map((targetName) => targetName.trim()),
        }))
        .filter((subscription) => subscription.targetNames.length > 0);

      await updateWebhookNotifierConfig.mutateAsync({
        targets: normalizedTargets,
        subscriptions: normalizedSubscriptions,
      });
    },
    [formData, updateWebhookNotifierConfig, validateTargets]
  );

  if (isLoading) {
    return (
      <div className='flex items-center justify-center p-8'>
        <Loader2 className='h-8 w-8 animate-spin' />
      </div>
    );
  }

  const subscribedTargetCount = getSubscribedTargetNames().size;
  const normalizedNameCounts = formData.targets.reduce<Record<string, number>>((acc, target) => {
    const normalizedName = target.name.trim();
    if (!normalizedName) {
      return acc;
    }

    acc[normalizedName] = (acc[normalizedName] || 0) + 1;
    return acc;
  }, {});

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.webhook.title')}</CardTitle>
        <CardDescription>{t('system.webhook.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <div className='bg-muted/50 space-y-2 rounded-md border p-4'>
            <div className='text-sm font-medium'>{t('system.webhook.availableEvents.title')}</div>
            <div className='text-muted-foreground text-sm'>{t('system.webhook.availableEvents.description')}</div>
            <div className='bg-background flex items-center justify-between rounded-md border p-3'>
              <div className='space-y-1'>
                <div className='font-mono text-xs'>{AUTO_DISABLED_EVENT}</div>
                <div className='text-muted-foreground text-sm'>{t('system.webhook.events.channelAutoDisabled')}</div>
              </div>
              <div className='text-muted-foreground text-sm'>
                {t('system.webhook.subscriptionCount', { count: subscribedTargetCount })}
              </div>
            </div>
          </div>

          <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
            <div className='space-y-0.5'>
              <div className='text-sm font-medium'>{t('system.webhook.targets.title')}</div>
              <div className='text-muted-foreground text-sm'>{t('system.webhook.targets.description')}</div>
            </div>
            <Button type='button' variant='outline' size='sm' onClick={addTarget} className='w-full sm:w-auto'>
              <Plus className='mr-1 h-4 w-4' />
              {t('system.webhook.targets.add')}
            </Button>
          </div>

          {formData.targets.length === 0 ? (
            <div className='text-muted-foreground rounded-md border border-dashed p-6 text-sm'>{t('system.webhook.targets.empty')}</div>
          ) : (
            <div className='space-y-4'>
              {formData.targets.map((target, targetIndex) => {
                const targetName = target.name.trim();
                const targetSubscribed = targetName ? isTargetSubscribed(targetName) : false;
                const hasDuplicateName = !!targetName && normalizedNameCounts[targetName] > 1;
                const proxyType = target.proxy?.type || proxyTypeSchema.enum.disabled;

                return (
                  <div key={targetIndex} className='space-y-4 rounded-md border p-3 sm:p-4'>
                    <div className='flex items-start justify-between gap-2'>
                      <div className='space-y-1 min-w-0 flex-1'>
                        <div className='text-sm font-medium'>{t('system.webhook.target.title', { index: targetIndex + 1 })}</div>
                        <div className='text-muted-foreground text-sm'>{t('system.webhook.target.description')}</div>
                      </div>
                      <Button type='button' variant='ghost' size='icon' onClick={() => removeTarget(targetIndex)} className='shrink-0'>
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>

                    <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 rounded-md border p-3'>
                      <div className='space-y-0.5 min-w-0'>
                        <Label htmlFor={`webhook-enabled-${targetIndex}`} className='text-sm font-medium'>
                          {t('system.webhook.enabled.label')}
                        </Label>
                        <div className='text-muted-foreground text-sm'>{t('system.webhook.enabled.description')}</div>
                      </div>
                      <Switch
                        id={`webhook-enabled-${targetIndex}`}
                        checked={target.enabled}
                        onCheckedChange={(checked) => handleTargetChange(targetIndex, 'enabled', checked)}
                        className='shrink-0'
                      />
                    </div>

                    <div className='grid gap-4 sm:grid-cols-2'>
                      <div className='space-y-2'>
                        <Label htmlFor={`webhook-name-${targetIndex}`}>{t('system.webhook.name')}</Label>
                        <Input
                          id={`webhook-name-${targetIndex}`}
                          value={target.name}
                          onChange={(e) => handleTargetNameChange(targetIndex, e.target.value)}
                          aria-invalid={hasDuplicateName || !targetName}
                        />
                        <div className='text-muted-foreground text-xs'>{t('system.webhook.nameHint')}</div>
                        {!targetName && <div className='text-destructive text-xs'>{t('system.webhook.validation.nameRequired')}</div>}
                        {hasDuplicateName && <div className='text-destructive text-xs'>{t('system.webhook.validation.nameUnique')}</div>}
                      </div>
                      <div className='space-y-2'>
                        <Label htmlFor={`webhook-timeout-${targetIndex}`}>{t('system.webhook.timeout')}</Label>
                        <Input
                          id={`webhook-timeout-${targetIndex}`}
                          type='number'
                          min='100'
                          max='10000'
                          value={target.timeoutMs}
                          onChange={(e) => handleTargetChange(targetIndex, 'timeoutMs', parseInt(e.target.value) || 3000)}
                        />
                      </div>
                    </div>

                    <div className='space-y-2'>
                      <Label htmlFor={`webhook-url-${targetIndex}`}>{t('system.webhook.url')}</Label>
                      <Input
                        id={`webhook-url-${targetIndex}`}
                        value={target.url}
                        onChange={(e) => handleTargetChange(targetIndex, 'url', e.target.value)}
                        aria-invalid={target.enabled && !target.url.trim()}
                      />
                      {target.enabled && !target.url.trim() && <div className='text-destructive text-xs'>{t('system.webhook.validation.urlRequired')}</div>}
                    </div>

                    <div className='space-y-4 rounded-md border p-3 sm:p-4'>
                      <div className='space-y-1'>
                        <div className='text-sm font-medium'>{t('system.webhook.proxy.title')}</div>
                        <div className='text-muted-foreground text-sm'>{t('system.webhook.proxy.description')}</div>
                      </div>

                      <div className='space-y-2'>
                        <Label>{t('system.webhook.proxy.type')}</Label>
                        <Select value={proxyType} onValueChange={(value) => handleTargetProxyTypeChange(targetIndex, value as ProxyType)}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value={proxyTypeSchema.enum.disabled}>{t('system.webhook.proxy.types.disabled')}</SelectItem>
                            <SelectItem value={proxyTypeSchema.enum.environment}>{t('system.webhook.proxy.types.environment')}</SelectItem>
                            <SelectItem value={proxyTypeSchema.enum.url}>{t('system.webhook.proxy.types.url')}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      {proxyType === proxyTypeSchema.enum.url && proxyPresets.length > 0 && (
                        <div className='space-y-2'>
                          <Label>{t('system.webhook.proxy.presets')}</Label>
                          <Select
                            onValueChange={(presetURL) => {
                              const preset = proxyPresets.find((item) => item.url === presetURL);
                              if (!preset) {
                                return;
                              }

                              handleTargetProxyChange(targetIndex, {
                                type: proxyTypeSchema.enum.url,
                                url: preset.url,
                                username: preset.username || '',
                                password: preset.password || '',
                              });
                            }}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('system.webhook.proxy.presetsPlaceholder')} />
                            </SelectTrigger>
                            <SelectContent>
                              {proxyPresets.map((preset) => (
                                <SelectItem key={preset.url} value={preset.url}>
                                  {preset.name || preset.url}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                      )}

                      {proxyType === proxyTypeSchema.enum.environment && (
                        <div className='text-muted-foreground rounded-md border p-3 text-sm'>{t('system.webhook.proxy.environmentHint')}</div>
                      )}

                      {proxyType === proxyTypeSchema.enum.url && (
                        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                          <div className='space-y-2 sm:col-span-2 lg:col-span-3'>
                            <Label htmlFor={`webhook-proxy-url-${targetIndex}`}>{t('system.webhook.proxy.url')}</Label>
                            <Input
                              id={`webhook-proxy-url-${targetIndex}`}
                              value={target.proxy?.url || ''}
                              onChange={(e) => handleTargetProxyFieldChange(targetIndex, 'url', e.target.value)}
                            />
                          </div>
                          <div className='space-y-2'>
                            <Label htmlFor={`webhook-proxy-username-${targetIndex}`}>{t('system.webhook.proxy.username')}</Label>
                            <Input
                              id={`webhook-proxy-username-${targetIndex}`}
                              value={target.proxy?.username || ''}
                              onChange={(e) => handleTargetProxyFieldChange(targetIndex, 'username', e.target.value)}
                            />
                          </div>
                          <div className='space-y-2 sm:col-span-2 lg:col-span-2'>
                            <Label htmlFor={`webhook-proxy-password-${targetIndex}`}>{t('system.webhook.proxy.password')}</Label>
                            <Input
                              id={`webhook-proxy-password-${targetIndex}`}
                              type='password'
                              value={target.proxy?.password || ''}
                              onChange={(e) => handleTargetProxyFieldChange(targetIndex, 'password', e.target.value)}
                            />
                          </div>
                        </div>
                      )}
                    </div>

                    <div className='space-y-3 rounded-md border p-3 sm:p-4'>
                      <div className='space-y-1'>
                        <div className='text-sm font-medium'>{t('system.webhook.subscription')}</div>
                        <div className='text-muted-foreground text-sm'>{t('system.webhook.subscriptionHelp')}</div>
                      </div>
                      <label className='flex items-start gap-3'>
                        <Checkbox
                          checked={targetSubscribed}
                          onCheckedChange={(checked) => handleSubscriptionChange(target.name.trim(), checked === true)}
                          disabled={!target.name.trim()}
                          className='shrink-0 mt-0.5'
                        />
                        <div className='space-y-1 min-w-0 flex-1'>
                          <div className='font-mono text-xs break-all'>{AUTO_DISABLED_EVENT}</div>
                          <div className='text-muted-foreground text-sm'>{t('system.webhook.events.channelAutoDisabled')}</div>
                        </div>
                      </label>
                    </div>

                    <div className='space-y-3'>
                      <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
                        <Label className='text-sm font-medium'>{t('system.webhook.headers')}</Label>
                        <Button type='button' variant='outline' size='sm' onClick={() => addHeader(targetIndex)} className='w-full sm:w-auto'>
                          <Plus className='mr-1 h-4 w-4' />
                          {t('system.webhook.addHeader')}
                        </Button>
                      </div>
                      {(target.headers || []).map((header, headerIndex) => (
                        <div key={headerIndex} className='flex flex-col sm:flex-row items-start sm:items-center gap-2'>
                          <Input value={header.key} placeholder='Header' onChange={(e) => handleHeaderChange(targetIndex, headerIndex, 'key', e.target.value)} className='w-full' />
                          <Input value={header.value} placeholder='Value' onChange={(e) => handleHeaderChange(targetIndex, headerIndex, 'value', e.target.value)} className='w-full' />
                          <Button type='button' variant='ghost' size='icon' onClick={() => removeHeader(targetIndex, headerIndex)} className='shrink-0'>
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      ))}
                    </div>

                    <div className='space-y-2'>
                      <Label htmlFor={`webhook-body-${targetIndex}`}>{t('system.webhook.body')}</Label>
                      <textarea
                        id={`webhook-body-${targetIndex}`}
                        value={target.body}
                        onChange={(e) => handleTargetChange(targetIndex, 'body', e.target.value)}
                        className='border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring min-h-56 w-full rounded-md border px-3 py-2 font-mono text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none'
                      />
                      <div className='text-muted-foreground text-xs'>{t('system.webhook.templateHelp')}</div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          <div className='flex justify-end'>
            <Button type='submit' disabled={updateWebhookNotifierConfig.isPending} className='w-full sm:w-auto sm:min-w-24'>
              {updateWebhookNotifierConfig.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
