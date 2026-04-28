'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useQuotaEnforcementSettings, useUpdateQuotaEnforcementSettings, type QuotaEnforcementMode } from '../data/system';

interface QuotaFormData {
  enabled: boolean;
  mode: QuotaEnforcementMode;
}

export function QuotaSettings() {
  const { t } = useTranslation();
  const { data: quotaSettings, isLoading } = useQuotaEnforcementSettings();
  const updateQuotaEnforcementSettings = useUpdateQuotaEnforcementSettings();

  const [formData, setFormData] = useState<QuotaFormData>({
    enabled: false,
    mode: 'EXHAUSTED_ONLY',
  });

  useEffect(() => {
    if (quotaSettings) {
      setFormData({
        enabled: quotaSettings.enabled,
        mode: quotaSettings.mode,
      });
    }
  }, [quotaSettings]);

  const handleToggleChange = useCallback((checked: boolean) => {
    setFormData((prev) => ({
      ...prev,
      enabled: checked,
    }));
  }, []);

  const handleModeChange = useCallback((value: string) => {
    setFormData((prev) => ({
      ...prev,
      mode: value as QuotaEnforcementMode,
    }));
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateQuotaEnforcementSettings.mutateAsync({
        enabled: formData.enabled,
        mode: formData.mode,
      });
    },
    [updateQuotaEnforcementSettings, formData]
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
        <CardTitle>{t('system.quota.title')}</CardTitle>
        <CardDescription>{t('system.quota.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className='space-y-6'>
          {/* Enable/Disable Quota Enforcement */}
          <div className='flex items-center justify-between' id='quota-enabled-switch'>
            <div className='space-y-0.5'>
              <Label htmlFor='quota-enabled' className='text-base'>
                {t('system.quota.enabled.label')}
              </Label>
              <div className='text-muted-foreground text-sm'>{t('system.quota.enabled.description')}</div>
            </div>
            <Switch id='quota-enabled' checked={formData.enabled} onCheckedChange={handleToggleChange} />
          </div>

          <Separator />

          {/* Mode Selection - Only show when enabled */}
          {formData.enabled && (
            <div className='space-y-4'>
              <div className='space-y-2'>
                <Label htmlFor='quota-mode'>{t('system.quota.mode.label')}</Label>
                <div className='text-muted-foreground mb-2 text-sm'>{t('system.quota.mode.description')}</div>
                <Select
                  value={formData.mode}
                  onValueChange={handleModeChange}
                >
                  <SelectTrigger id='quota-mode' className='w-56'>
                    <SelectValue placeholder={t('system.quota.mode.placeholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='EXHAUSTED_ONLY'>{t('system.quota.mode.options.exhaustedOnly')}</SelectItem>
                    <SelectItem value='DE_PRIORITIZE'>{t('system.quota.mode.options.dePrioritize')}</SelectItem>
                  </SelectContent>
                </Select>

                {/* Mode Documentation */}
                {formData.mode && (
                  <div className='bg-muted/50 mt-3 rounded-md border p-3'>
                    <div className='text-muted-foreground text-xs leading-relaxed'>
                      {t(`system.quota.mode.documentation.${formData.mode}`)}
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}

          <Separator />

          {/* Submit Button */}
          <div className='flex justify-end'>
            <Button type='submit' disabled={updateQuotaEnforcementSettings.isPending} className='min-w-24'>
              {updateQuotaEnforcementSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
