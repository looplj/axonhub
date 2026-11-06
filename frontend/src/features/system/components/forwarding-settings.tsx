'use client';

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  useUserAgentPassThroughSettings,
  useUpdateUserAgentPassThroughSettings,
  usePassThroughSettings,
  useUpdatePassThroughSettings,
  useUsageCostInjectionSettings,
  useUpdateUsageCostInjectionSettings,
} from '../data/system';

// How requests and responses are forwarded between clients and upstream
// channels. Every switch saves immediately.
export function ForwardingSettings() {
  const { t } = useTranslation();

  // User-Agent Pass-Through settings
  const { data: uaSettings, isLoading: isLoadingUASettings } = useUserAgentPassThroughSettings();
  const updateUASettings = useUpdateUserAgentPassThroughSettings();
  const [uaPassThroughEnabled, setUaPassThroughEnabled] = useState(false);

  // Pass-Through (request/response body) settings
  const { data: ptSettings, isLoading: isLoadingPTSettings } = usePassThroughSettings();
  const updatePTSettings = useUpdatePassThroughSettings();
  const [passThroughEnabled, setPassThroughEnabled] = useState(false);

  const { data: usageCostSettings, isLoading: isLoadingUsageCostSettings } = useUsageCostInjectionSettings();
  const updateUsageCostSettings = useUpdateUsageCostInjectionSettings();
  const [usageCostInjectionEnabled, setUsageCostInjectionEnabled] = useState(false);

  // Update UA pass-through state when loaded
  useEffect(() => {
    if (uaSettings) {
      setUaPassThroughEnabled(uaSettings.enabled);
    }
  }, [uaSettings]);

  // Update pass-through state when loaded
  useEffect(() => {
    if (ptSettings) {
      setPassThroughEnabled(ptSettings.enabled);
    }
  }, [ptSettings]);

  useEffect(() => {
    if (usageCostSettings) {
      setUsageCostInjectionEnabled(usageCostSettings.enabled);
    }
  }, [usageCostSettings]);

  const handleUAPassThroughChange = async (enabled: boolean) => {
    const previousValue = uaPassThroughEnabled;
    setUaPassThroughEnabled(enabled);
    try {
      await updateUASettings.mutateAsync({ enabled });
    } catch {
      // Revert state on error
      setUaPassThroughEnabled(previousValue);
    }
  };

  const handlePassThroughChange = async (enabled: boolean) => {
    const previousValue = passThroughEnabled;
    setPassThroughEnabled(enabled);
    try {
      await updatePTSettings.mutateAsync({ enabled });
    } catch {
      // Revert state on error
      setPassThroughEnabled(previousValue);
    }
  };

  const handleUsageCostInjectionChange = async (enabled: boolean) => {
    const previousValue = usageCostInjectionEnabled;
    setUsageCostInjectionEnabled(enabled);
    try {
      await updateUsageCostSettings.mutateAsync({ enabled });
    } catch {
      setUsageCostInjectionEnabled(previousValue);
    }
  };

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.passThroughGroup.title')}</CardTitle>
          <CardDescription>{t('system.passThroughGroup.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='ua-pass-through'>{t('system.userAgentPassThrough.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.userAgentPassThrough.helpText')}</div>
            </div>
            <Switch
              id='ua-pass-through'
              checked={uaPassThroughEnabled}
              onCheckedChange={handleUAPassThroughChange}
              disabled={isLoadingUASettings || updateUASettings.isPending}
            />
          </div>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='pass-through'>{t('system.passThrough.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.passThrough.helpText')}</div>
            </div>
            <Switch
              id='pass-through'
              checked={passThroughEnabled}
              onCheckedChange={handlePassThroughChange}
              disabled={isLoadingPTSettings || updatePTSettings.isPending}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('system.usageCostInjection.title')}</CardTitle>
          <CardDescription>{t('system.usageCostInjection.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='usage-cost-injection'>{t('system.usageCostInjection.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.usageCostInjection.helpText')}</div>
            </div>
            <Switch
              id='usage-cost-injection'
              checked={usageCostInjectionEnabled}
              onCheckedChange={handleUsageCostInjectionChange}
              disabled={isLoadingUsageCostSettings || updateUsageCostSettings.isPending}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
