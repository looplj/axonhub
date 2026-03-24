'use client';

import React, { useState } from 'react';
import { Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { useSystemContext } from '../context/system-context';
import { currencyCodes } from '../data/currencies';
import { useGeneralSettings, useUpdateGeneralSettings } from '../data/system';
import { GMTTimeZoneOptions } from '../data/timezones';

import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TagsInput } from '@/components/ui/tags-input';
import { useGlobalCloakingConfig, useUpdateGlobalCloakingConfig, GlobalCloakingMode, SensitiveWordsMode } from '../data/system';
function CloakingSettings() {
  const { t } = useTranslation();
  const { data: settings, isLoading: isLoadingSettings } = useGlobalCloakingConfig();
  const updateSettings = useUpdateGlobalCloakingConfig();
  const { isLoading, setIsLoading } = useSystemContext();

  const [mode, setMode] = useState<GlobalCloakingMode>('AUTO');
  const [tlsFingerprint, setTlsFingerprint] = useState(false);
  const [headerAutoFill, setHeaderAutoFill] = useState(true);
  const [bodyCloak, setBodyCloak] = useState(true);
  const [cacheUserID, setCacheUserID] = useState(true);
  const [cacheControlAutoInject, setCacheControlAutoInject] = useState(true);
  const [sensitiveWordsMode, setSensitiveWordsMode] = useState<SensitiveWordsMode>('EXTEND');
  const [sensitiveWords, setSensitiveWords] = useState<string[]>([]);

  React.useEffect(() => {
    if (settings) {
      setMode(settings.mode || 'AUTO');
      setTlsFingerprint(settings.tlsFingerprint ?? false);
      setHeaderAutoFill(settings.headerAutoFill ?? true);
      setBodyCloak(settings.bodyCloak ?? true);
      setCacheUserID(settings.cacheUserID ?? true);
      setCacheControlAutoInject(settings.cacheControlAutoInject ?? true);
      setSensitiveWordsMode(settings.sensitiveWordsMode || 'EXTEND');
      setSensitiveWords(settings.sensitiveWords || []);
    }
  }, [settings]);

  const hasChanges = settings ? (
    settings.mode !== mode ||
    settings.tlsFingerprint !== tlsFingerprint ||
    settings.headerAutoFill !== headerAutoFill ||
    settings.bodyCloak !== bodyCloak ||
    settings.cacheUserID !== cacheUserID ||
    settings.cacheControlAutoInject !== cacheControlAutoInject ||
    settings.sensitiveWordsMode !== sensitiveWordsMode ||
    JSON.stringify(settings.sensitiveWords || []) !== JSON.stringify(sensitiveWords)
  ) : false;

  const handleSave = async () => {
    setIsLoading(true);
    try {
      await updateSettings.mutateAsync({
        mode,
        tlsFingerprint,
        headerAutoFill,
        bodyCloak,
        cacheUserID,
        cacheControlAutoInject,
        sensitiveWordsMode,
        sensitiveWords,
      });
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoadingSettings) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.cloaking.title')}</CardTitle>
          <CardDescription>{t('system.cloaking.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.mode.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.mode.description')}</div>
            </div>
            <Select value={mode} onValueChange={(v) => setMode(v as GlobalCloakingMode)}>
              <SelectTrigger className='w-[180px]'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="AUTO">{t('system.cloaking.mode.options.auto')}</SelectItem>
                <SelectItem value="ALWAYS">{t('system.cloaking.mode.options.always')}</SelectItem>
                <SelectItem value="NEVER">{t('system.cloaking.mode.options.never')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.tlsFingerprint.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.tlsFingerprint.description')}</div>
            </div>
            <Switch checked={tlsFingerprint} onCheckedChange={setTlsFingerprint} />
          </div>

          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.headerAutoFill.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.headerAutoFill.description')}</div>
            </div>
            <Switch checked={headerAutoFill} onCheckedChange={setHeaderAutoFill} />
          </div>

          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.bodyCloak.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.bodyCloak.description')}</div>
            </div>
            <Switch checked={bodyCloak} onCheckedChange={setBodyCloak} />
          </div>

          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.cacheUserID.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.cacheUserID.description')}</div>
            </div>
            <Switch checked={cacheUserID} onCheckedChange={setCacheUserID} />
          </div>

          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.cacheControlAutoInject.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.cacheControlAutoInject.description')}</div>
            </div>
            <Switch checked={cacheControlAutoInject} onCheckedChange={setCacheControlAutoInject} />
          </div>

          <div className='flex items-center justify-between rounded-lg border p-4'>
            <div className='space-y-0.5'>
              <Label>{t('system.cloaking.sensitiveWordsMode.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.cloaking.sensitiveWordsMode.description')}</div>
            </div>
            <Select value={sensitiveWordsMode} onValueChange={(v) => setSensitiveWordsMode(v as SensitiveWordsMode)}>
              <SelectTrigger className='w-[180px]'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="EXTEND">{t('system.cloaking.sensitiveWordsMode.options.extend')}</SelectItem>
                <SelectItem value="REPLACE">{t('system.cloaking.sensitiveWordsMode.options.replace')}</SelectItem>
                <SelectItem value="DISABLE">{t('system.cloaking.sensitiveWordsMode.options.disable')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {sensitiveWordsMode !== 'DISABLE' && (
            <div className='space-y-2 rounded-lg border p-4'>
              <div className='space-y-0.5'>
                <Label>{t('system.cloaking.sensitiveWords.label')}</Label>
                <div className='text-muted-foreground text-sm'>{t('system.cloaking.sensitiveWords.description')}</div>
              </div>
              <TagsInput 
                value={sensitiveWords}
                onChange={setSensitiveWords}
                placeholder={t('system.cloaking.sensitiveWords.placeholder')}
              />
            </div>
          )}
        </CardContent>
      </Card>

      {hasChanges && (
        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={isLoading || updateSettings.isPending} className='min-w-[100px]'>
            {isLoading || updateSettings.isPending ? (
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
  );
}

export function GeneralSettings() {
  const { t } = useTranslation();
  const { data: settings, isLoading: isLoadingSettings } = useGeneralSettings();
  const updateSettings = useUpdateGeneralSettings();
  const { isLoading, setIsLoading } = useSystemContext();

  const [currencyCode, setCurrencyCode] = useState('USD');
  const [timezone, setTimezone] = useState('UTC');

  const currencyItems = React.useMemo(
    () =>
      currencyCodes.map((code) => ({
        value: code,
        label: t(`currencies.${code}`),
      })),
    [t]
  );

  const timezoneItems = React.useMemo(() => GMTTimeZoneOptions, []);

  // Update local state when settings are loaded
  React.useEffect(() => {
    if (settings) {
      setCurrencyCode(settings.currencyCode || 'USD');
      setTimezone(settings.timezone || 'UTC');
    }
  }, [settings]);

  const handleSave = async () => {
    setIsLoading(true);
    try {
      await updateSettings.mutateAsync({
        currencyCode: currencyCode.trim(),
        timezone: timezone.trim(),
      });
    } finally {
      setIsLoading(false);
    }
  };

  const hasChanges = settings ? settings.currencyCode !== currencyCode || settings.timezone !== timezone : false;

  if (isLoadingSettings) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.general.title')}</CardTitle>
          <CardDescription>{t('system.general.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='space-y-2'>
            <Label htmlFor='currency-code'>{t('system.general.currencyCode.label')}</Label>
            <div className='max-w-md'>
              <AutoCompleteSelect
                selectedValue={currencyCode}
                onSelectedValueChange={setCurrencyCode}
                items={currencyItems}
                placeholder={t('system.general.currencyCode.placeholder')}
                isLoading={isLoadingSettings}
              />
            </div>
            <div className='text-muted-foreground text-sm'>{t('system.general.currencyCode.description')}</div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='timezone'>{t('system.general.timezone.label')}</Label>
            <div className='max-w-md'>
              <AutoCompleteSelect
                selectedValue={timezone}
                onSelectedValueChange={setTimezone}
                items={timezoneItems}
                placeholder={t('system.general.timezone.placeholder')}
                isLoading={isLoadingSettings}
              />
            </div>
            <div className='text-muted-foreground text-sm'>{t('system.general.timezone.description')}</div>
          </div>
        </CardContent>
      </Card>

      {hasChanges && (
        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={isLoading || updateSettings.isPending} className='min-w-[100px]'>
            {isLoading || updateSettings.isPending ? (
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
      <CloakingSettings />
    </div>
  );
}
