import { useEffect, useState, useCallback, useMemo, useRef } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash, IconSettings, IconChevronDown, IconChevronUp } from '@tabler/icons-react';
import { format, type Locale } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { useQueryModels } from '@/gql/models';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { useDebounce } from '@/hooks/use-debounce';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { AutoComplete } from '@/components/auto-complete';
import { useAllChannelsForOrdering } from '@/features/channels/data/channels';
import { useApiKeysContext } from '../context/apikeys-context';
import { useApiKeyQuotaUsages } from '../data/apikeys';
import { updateApiKeyProfilesInputSchemaFactory, type ApiKeyProfile, type ApiKeyProfileQuotaUsage, type UpdateApiKeyProfilesInput } from '../data/schema';

type ApiKeyQuotaPeriod = NonNullable<NonNullable<ApiKeyProfile['quota']>['period']>;

function quotaPeriodLabel(period: ApiKeyQuotaPeriod | null | undefined, t: (key: string) => string) {
  if (!period) return '-';

  const unitLabel = (unit: string) => {
    switch (unit) {
      case 'minute':
        return t('apikeys.profiles.quotaUnitMinute');
      case 'hour':
        return t('apikeys.profiles.quotaUnitHour');
      case 'day':
        return t('apikeys.profiles.quotaUnitDay');
      case 'month':
        return t('apikeys.profiles.quotaUnitMonth');
      default:
        return unit;
    }
  };

  switch (period.type) {
    case 'all_time':
      return t('apikeys.profiles.quotaPeriodAllTime');
    case 'past_duration': {
      const value = period.pastDuration?.value;
      const unit = period.pastDuration?.unit;
      const suffix = value && unit ? ` (${value} ${unitLabel(unit)})` : '';
      return `${t('apikeys.profiles.quotaPeriodPastDuration')}${suffix}`;
    }
    case 'calendar_duration': {
      const unit = period.calendarDuration?.unit;
      const suffix = unit ? ` (${unitLabel(unit)})` : '';
      return `${t('apikeys.profiles.quotaPeriodCalendarDuration')}${suffix}`;
    }
    default:
      return period.type;
  }
}

interface ApiKeyProfilesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: UpdateApiKeyProfilesInput) => void;
  loading?: boolean;
  initialData?: {
    activeProfile: string;
    profiles: ApiKeyProfile[];
  };
}

export function ApiKeyProfilesDialog({ open, onOpenChange, onSubmit, loading = false, initialData }: ApiKeyProfilesDialogProps) {
  const { t, i18n } = useTranslation();
  const { selectedApiKey } = useApiKeysContext();
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels();
  // 用于解决 Dialog 内 Popover 无法滚动的问题
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const apiKeyId = selectedApiKey?.id ?? '';
  const quotaUsagesQuery = useApiKeyQuotaUsages(apiKeyId, {
    enabled: open && !!apiKeyId,
    refetchInterval: open ? 10000 : undefined,
  });
  const quotaUsageByProfileName = useMemo(() => {
    const map = new Map<string, ApiKeyProfileQuotaUsage>();
    quotaUsagesQuery.data?.forEach((u) => {
      map.set(u.profileName, u);
    });
    return map;
  }, [quotaUsagesQuery.data]);

  useEffect(() => {
    if (open) {
      fetchModels({
        statusIn: ['enabled'],
        includeMapping: true,
        includePrefix: true,
      });
    }
  }, [open, fetchModels]);

  const defaultValues = useMemo(
    () => ({
      activeProfile: '',
      profiles: [] as ApiKeyProfile[],
    }),
    []
  );

  const form = useForm<UpdateApiKeyProfilesInput>({
    resolver: zodResolver(updateApiKeyProfilesInputSchemaFactory(t)),
    defaultValues,
  });

  const lastInitialDataRef = useRef<string | null>(null);
  const profileRefs = useRef<(HTMLDivElement | null)[]>([]);
  const normalizedInitialData = useMemo(() => {
    if (initialData?.profiles?.length) {
      const fallbackActiveProfile = initialData.activeProfile?.trim()
        ? initialData.activeProfile
        : initialData.profiles[0]?.name || defaultValues.activeProfile;

      return {
        activeProfile: fallbackActiveProfile,
        profiles: initialData.profiles,
      };
    }

    return defaultValues;
  }, [initialData, defaultValues]);
  const normalizedSerialized = useMemo(() => JSON.stringify(normalizedInitialData), [normalizedInitialData]);

  const {
    fields: profileFields,
    append: appendProfile,
    remove: removeProfile,
  } = useFieldArray({
    control: form.control,
    name: 'profiles',
  });

  // Watch profile names to update activeProfile dropdown options
  const watchedProfiles = form.watch('profiles') || [];
  const profileNames = watchedProfiles.map((profile) => profile.name || '');

  useEffect(() => {
    const nonEmptyProfiles = watchedProfiles.filter((profile) => profile?.name?.trim());
    const currentActiveProfile = form.getValues('activeProfile') || '';

    if (nonEmptyProfiles.length === 0) {
      if (currentActiveProfile !== '') {
        form.setValue('activeProfile', '');
      }
      return;
    }

    const activeMatchesExisting = nonEmptyProfiles.some((profile) => profile.name === currentActiveProfile);
    if (!activeMatchesExisting) {
      form.setValue('activeProfile', nonEmptyProfiles[0].name);
    }
  }, [watchedProfiles, form]);

  // Reset form when dialog opens or when incoming data actually changes
  useEffect(() => {
    if (!open) {
      lastInitialDataRef.current = null;
      return;
    }

    if (loading) {
      return;
    }

    if (lastInitialDataRef.current === normalizedSerialized) {
      return;
    }

    form.reset(normalizedInitialData);
    lastInitialDataRef.current = normalizedSerialized;
  }, [open, loading, form, normalizedInitialData, normalizedSerialized]);

  // Scroll to active profile after profiles rendered
  useEffect(() => {
    if (!open || loading || profileFields.length === 0) {
      return;
    }

    const scrollToActiveProfile = (retryCount = 0) => {
      const maxRetries = 10;
      const activeProfileName = form.getValues('activeProfile');

      if (!activeProfileName) {
        return;
      }

      const activeIndex = profileFields.findIndex((field) => field.name === activeProfileName);

      if (activeIndex < 0) {
        return;
      }

      const targetRef = profileRefs.current[activeIndex];

      if (targetRef) {
        targetRef.scrollIntoView({ behavior: 'smooth', block: 'center' });
      } else if (retryCount < maxRetries) {
        // Retry after a short delay if ref not yet available
        requestAnimationFrame(() => {
          setTimeout(() => scrollToActiveProfile(retryCount + 1), 50);
        });
      }
    };

    // Wait for next frame to ensure rendering
    requestAnimationFrame(() => {
      setTimeout(scrollToActiveProfile, 100);
    });
  }, [open, loading, profileFields, form]);

  const handleSubmit = useCallback(
    (data: UpdateApiKeyProfilesInput) => {
      // Clear any previous form-level errors
      form.clearErrors('profiles');
      onSubmit(data);
    },
    [form, onSubmit]
  );

  const addProfile = useCallback(() => {
    appendProfile({
      name: `Profile ${profileFields.length + 1}`,
      modelMappings: [],
      channelIDs: [],
      channelTags: [],
      modelIDs: [],
      loadBalanceStrategy: null,
    });
  }, [appendProfile, profileFields]);

  const removeProfileHandler = useCallback(
    (index: number) => {
      if (profileFields.length > 1) {
        removeProfile(index);
        // If we're removing the active profile, set active to the first remaining profile
        const currentActiveProfile = form.getValues('activeProfile');
        const removedProfile = profileFields[index];
        if (currentActiveProfile === removedProfile.name) {
          const remainingProfiles = profileFields.filter((_, i) => i !== index);
          if (remainingProfiles.length > 0) {
            form.setValue('activeProfile', remainingProfiles[0].name);
          }
        }
      }
    },
    [form, profileFields, removeProfile]
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent ref={setDialogContent} className='flex max-h-[90vh] flex-col sm:max-w-4xl'>
        <DialogHeader className='shrink-0 text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <IconSettings className='h-5 w-5' />
            {t('apikeys.profiles.title')}
          </DialogTitle>
          <DialogDescription>
            {t('apikeys.profiles.description', {
              name: selectedApiKey?.name,
            })}
          </DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col'>
          {/* Fixed Add Profile Section at Top */}
          <div className='bg-background shrink-0 border-b p-4'>
            <Form {...form}>
              <form id='apikey-profiles-form' onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6'>
                <div className='flex items-center justify-between'>
                  <h3 className='text-lg font-medium'>{t('apikeys.profiles.profilesTitle')}</h3>
                  <Button type='button' variant='outline' size='sm' onClick={addProfile} className='flex items-center gap-2'>
                    <IconPlus className='h-4 w-4' />
                    {t('apikeys.profiles.addProfile')}
                  </Button>
                </div>
              </form>
            </Form>
          </div>

          {/* Scrollable Profiles Section */}
          {profileFields.length > 0 && (
            <div className='flex-1 overflow-y-auto py-1'>
              <Form {...form}>
                <form onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6 px-4'>
                  <div className='space-y-4'>
                    <div className='space-y-4'>
                      {profileFields.map((profile, profileIndex) => {
                        const activeProfileName = form.getValues('activeProfile');
                        const isActive = profile.name === activeProfileName;

                        return (
                          <div
                            key={profile.id}
                            className={profileIndex === 0 ? 'mt-4' : ''}
                            ref={(el) => {
                              profileRefs.current[profileIndex] = el;
                            }}
                          >
                            <ProfileCard
                              profileIndex={profileIndex}
                              form={form}
                              onRemove={() => removeProfileHandler(profileIndex)}
                              canRemove={profileFields.length > 1}
                              availableModels={availableModels?.map((model) => model.id) || []}
                              t={t}
                              locale={locale}
                              quotaUsageByProfileName={quotaUsageByProfileName}
                              defaultExpanded={isActive}
                              portalContainer={dialogContent}
                            />
                          </div>
                        );
                      })}
                    </div>
                  </div>
                </form>
              </Form>
            </div>
          )}

          {/* Fixed Active Profile Section at Bottom */}
          <div className='bg-background mt-4 shrink-0 border-t px-4 py-2'>
            <Form {...form}>
              <FormField
                control={form.control}
                name='activeProfile'
                render={({ field }) => (
                  <FormItem className='flex items-center space-y-0 gap-x-3'>
                    <FormLabel className='shrink-0 font-medium'>{t('apikeys.profiles.activeProfile')}</FormLabel>
                    <FormControl>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <SelectTrigger>
                          <SelectValue placeholder={t('apikeys.profiles.selectActiveProfile')} />
                        </SelectTrigger>
                        <SelectContent>
                          {profileNames
                            .filter((name) => name.trim() !== '')
                            .map((profileName) => (
                              <SelectItem key={profileName} value={profileName}>
                                {profileName}
                              </SelectItem>
                            ))}
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </Form>
          </div>
        </div>

        <DialogFooter className='flex-col items-stretch gap-2 sm:flex-row sm:items-center sm:justify-end'>
          {/* Display form-level validation errors */}
          {/* {(form.formState.errors.profiles ||
            Object.keys(form.formState.errors).some((key) => key.startsWith('profiles.'))) && (
            <div className='text-destructive w-full text-sm'>
              {form.formState.errors.profiles?.message || t('apikeys.validation.duplicateProfileName')}
            </div>
          )} */}
          <div className='flex w-full gap-2 sm:w-auto'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={loading}>
              {t('common.buttons.cancel')}
            </Button>
            <Button
              type='submit'
              form='apikey-profiles-form'
              disabled={loading || !form.formState.isValid || Object.keys(form.formState.errors).length > 0}
            >
              {loading ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

interface ProfileCardProps {
  profileIndex: number;
  form: ReturnType<typeof useForm<UpdateApiKeyProfilesInput>>;
  onRemove: () => void;
  canRemove: boolean;
  availableModels: string[];
  t: (key: string) => string;
  locale: Locale;
  quotaUsageByProfileName: Map<string, ApiKeyProfileQuotaUsage>;
  defaultExpanded?: boolean;
  /** Popover Portal 容器元素，解决 Dialog 内无法滚动的问题 */
  portalContainer?: HTMLElement | null;
}

function ProfileCard({
  profileIndex,
  form,
  onRemove,
  canRemove,
  availableModels,
  t,
  locale,
  quotaUsageByProfileName,
  defaultExpanded = false,
  portalContainer,
}: ProfileCardProps) {
  const [localProfileName, setLocalProfileName] = useState('');
  const [isCollapsed, setIsCollapsed] = useState(!defaultExpanded);
  const { data: channelsData } = useAllChannelsForOrdering({ enabled: true });

  const debouncedProfileName = useDebounce(localProfileName, 500);

  // 从所有渠道中提取唯一标签
  const allTags = useMemo(() => {
    const tagsSet = new Set<string>();
    channelsData?.edges?.forEach((edge) => {
      edge.node.tags?.forEach((tag) => {
        if (tag) tagsSet.add(tag);
      });
    });
    return Array.from(tagsSet).sort();
  }, [channelsData]);

  const {
    fields: mappingFields,
    append: appendMapping,
    remove: removeMapping,
  } = useFieldArray({
    control: form.control,
    name: `profiles.${profileIndex}.modelMappings`,
  });

  // Watch all profiles to check for duplicates
  const allProfiles = form.watch('profiles') || [];
  const profileName = form.watch(`profiles.${profileIndex}.name`);
  const quotaUsage = profileName ? quotaUsageByProfileName.get(profileName) : undefined;
  const currentQuota = form.watch(`profiles.${profileIndex}.quota`);
  const quotaUsagePeriod = (currentQuota?.period ?? quotaUsage?.quota?.period) as ApiKeyQuotaPeriod | null | undefined;
  const quotaUsageEnd = quotaUsage?.window.end ?? (quotaUsagePeriod?.type !== 'calendar_duration' ? new Date() : null);

  // Initialize local state from form value
  useEffect(() => {
    const currentName = form.getValues(`profiles.${profileIndex}.name`);
    setLocalProfileName(currentName || '');
  }, [form, profileIndex]);

  // Immediate duplicate check (no debounce for error display)
  const checkDuplicate = useCallback(
    (value: string) => {
      const trimmedValue = value.trim().toLowerCase();
      if (trimmedValue === '') {
        form.clearErrors(`profiles.${profileIndex}.name`);
        return;
      }

      const otherProfiles = allProfiles.filter((_profile: ApiKeyProfile, idx: number) => idx !== profileIndex);
      const isDuplicate = otherProfiles.some((p: ApiKeyProfile) => p.name && p.name.trim().toLowerCase() === trimmedValue);

      if (isDuplicate) {
        form.setError(`profiles.${profileIndex}.name`, {
          type: 'manual',
          message: t('apikeys.validation.duplicateProfileName'),
        });
      } else {
        form.clearErrors(`profiles.${profileIndex}.name`);
      }
    },
    [form, profileIndex, allProfiles, t]
  );
  // Debounced form value update for performance
  useEffect(() => {
    checkDuplicate(debouncedProfileName);
  }, [debouncedProfileName, checkDuplicate]);

  const addMapping = useCallback(() => {
    appendMapping({ from: '', to: '' });
  }, [appendMapping]);

  return (
    <Card>
      <CardHeader className='pb-3'>
        <div className='flex items-center justify-between gap-2'>
          <CardTitle className='min-w-0 flex-1 text-base'>
            <FormField
              control={form.control}
              name={`profiles.${profileIndex}.name`}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <Input
                      value={field.value}
                      onChange={(e) => {
                        const newValue = e.target.value;
                        setLocalProfileName(newValue);
                        field.onChange(newValue);
                      }}
                      onBlur={field.onBlur}
                      placeholder={t('apikeys.profiles.profileName')}
                      className='w-full font-medium md:w-[12em]'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardTitle>
          <div className='flex shrink-0 items-center gap-1'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => setIsCollapsed((prev) => !prev)}
              className='hover:bg-accent'
              aria-expanded={!isCollapsed}
              aria-label={isCollapsed ? t('apikeys.profiles.expand') : t('apikeys.profiles.collapse')}
            >
              {isCollapsed ? <IconChevronDown className='h-4 w-4' /> : <IconChevronUp className='h-4 w-4' />}
            </Button>
            {canRemove && (
              <Button type='button' variant='ghost' size='sm' onClick={onRemove} className='text-destructive hover:text-destructive'>
                <IconTrash className='h-4 w-4' />
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      {!isCollapsed && (
        <CardContent className='space-y-6'>
          {/* Quota Section */}
          <div className='space-y-4'>
            <div className='flex items-center justify-between gap-3'>
              <div>
                <h4 className='text-sm font-medium'>{t('apikeys.profiles.quotaTitle')}</h4>
                <p className='text-muted-foreground mt-1 text-xs'>{t('apikeys.profiles.quotaDescription')}</p>
              </div>
              <FormField
                control={form.control}
                name={`profiles.${profileIndex}.quota`}
                render={({ field }) => (
                  <FormItem className='flex items-center space-y-0 gap-x-2'>
                    <FormLabel className='text-sm'>{t('apikeys.profiles.quotaEnabled')}</FormLabel>
                    <FormControl>
                      <Switch
                        checked={field.value != null}
                        onCheckedChange={(checked) => {
                          if (checked) {
                            field.onChange({
                              requests: null,
                              totalTokens: null,
                              cost: null,
                              period: {
                                type: 'all_time',
                                pastDuration: null,
                                calendarDuration: null,
                              },
                            });
                          } else {
                            field.onChange(null);
                          }
                        }}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>

            {form.watch(`profiles.${profileIndex}.quota`) != null && (
              <div className='space-y-4'>
                <div className='grid gap-4 md:grid-cols-3'>
                  <FormField
                    control={form.control}
                    name={`profiles.${profileIndex}.quota.requests`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('apikeys.profiles.quotaRequests')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            value={(field.value as unknown as number | null | undefined) ?? ''}
                            onChange={(e) => {
                              const v = e.target.value;
                              field.onChange(v === '' ? null : Number(v));
                            }}
                            placeholder={t('apikeys.profiles.quotaRequestsPlaceholder')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name={`profiles.${profileIndex}.quota.totalTokens`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('apikeys.profiles.quotaTotalTokens')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            value={(field.value as unknown as number | null | undefined) ?? ''}
                            onChange={(e) => {
                              const v = e.target.value;
                              field.onChange(v === '' ? null : Number(v));
                            }}
                            placeholder={t('apikeys.profiles.quotaTotalTokensPlaceholder')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name={`profiles.${profileIndex}.quota.cost`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('apikeys.profiles.quotaCost')}</FormLabel>
                        <FormControl>
                          <Input
                            inputMode='decimal'
                            value={(field.value as unknown as number | null | undefined) ?? ''}
                            onChange={(e) => {
                              const v = e.target.value;
                              field.onChange(v === '' ? null : Number(v));
                            }}
                            placeholder={t('apikeys.profiles.quotaCostPlaceholder')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                <div className='grid gap-4 md:grid-cols-3'>
                  <FormField
                    control={form.control}
                    name={`profiles.${profileIndex}.quota.period.type`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('apikeys.profiles.quotaPeriodType')}</FormLabel>
                        <FormControl>
                          <Select
                            value={field.value}
                            onValueChange={(value) => {
                              field.onChange(value);
                              if (value === 'past_duration') {
                                form.setValue(`profiles.${profileIndex}.quota.period.pastDuration`, { value: 1, unit: 'day' });
                                form.setValue(`profiles.${profileIndex}.quota.period.calendarDuration`, null);
                              } else if (value === 'calendar_duration') {
                                form.setValue(`profiles.${profileIndex}.quota.period.calendarDuration`, { unit: 'day' });
                                form.setValue(`profiles.${profileIndex}.quota.period.pastDuration`, null);
                              } else {
                                form.setValue(`profiles.${profileIndex}.quota.period.pastDuration`, null);
                                form.setValue(`profiles.${profileIndex}.quota.period.calendarDuration`, null);
                              }
                            }}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='all_time'>{t('apikeys.profiles.quotaPeriodAllTime')}</SelectItem>
                              <SelectItem value='past_duration'>{t('apikeys.profiles.quotaPeriodPastDuration')}</SelectItem>
                              <SelectItem value='calendar_duration'>{t('apikeys.profiles.quotaPeriodCalendarDuration')}</SelectItem>
                            </SelectContent>
                          </Select>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {form.watch(`profiles.${profileIndex}.quota.period.type`) === 'past_duration' && (
                    <>
                      <FormField
                        control={form.control}
                        name={`profiles.${profileIndex}.quota.period.pastDuration.value`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('apikeys.profiles.quotaPastDurationValue')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={1}
                                value={(field.value as unknown as number | null | undefined) ?? ''}
                                onChange={(e) => field.onChange(Number(e.target.value))}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name={`profiles.${profileIndex}.quota.period.pastDuration.unit`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('apikeys.profiles.quotaPastDurationUnit')}</FormLabel>
                            <FormControl>
                              <Select value={field.value} onValueChange={field.onChange}>
                                <SelectTrigger>
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value='minute'>{t('apikeys.profiles.quotaUnitMinute')}</SelectItem>
                                  <SelectItem value='hour'>{t('apikeys.profiles.quotaUnitHour')}</SelectItem>
                                  <SelectItem value='day'>{t('apikeys.profiles.quotaUnitDay')}</SelectItem>
                                </SelectContent>
                              </Select>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </>
                  )}

                  {form.watch(`profiles.${profileIndex}.quota.period.type`) === 'calendar_duration' && (
                    <FormField
                      control={form.control}
                      name={`profiles.${profileIndex}.quota.period.calendarDuration.unit`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('apikeys.profiles.quotaCalendarDurationUnit')}</FormLabel>
                          <FormControl>
                            <Select value={field.value} onValueChange={field.onChange}>
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value='day'>{t('apikeys.profiles.quotaUnitDay')}</SelectItem>
                                <SelectItem value='month'>{t('apikeys.profiles.quotaUnitMonth')}</SelectItem>
                              </SelectContent>
                            </Select>
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}
                </div>

                {quotaUsage && (
                  <div className='space-y-2 rounded-md border p-3'>
                    <div className='text-xs font-medium'>{t('apikeys.profiles.quotaUsageTitle')}</div>
                    <div className='text-muted-foreground text-xs'>
                      {t('apikeys.profiles.quotaPeriodType')}: {quotaPeriodLabel(quotaUsagePeriod, t)}
                    </div>
                    <div className='grid gap-3 md:grid-cols-3'>
                      <div>
                        <div className='text-muted-foreground text-xs'>{t('apikeys.profiles.quotaRequests')}</div>
                        <div className='text-sm'>
                          {quotaUsage.usage.requestCount}/{currentQuota?.requests ?? '∞'}
                        </div>
                      </div>
                      <div>
                        <div className='text-muted-foreground text-xs'>{t('apikeys.profiles.quotaTotalTokens')}</div>
                        <div className='text-sm'>
                          {quotaUsage.usage.totalTokens}/{currentQuota?.totalTokens ?? '∞'}
                        </div>
                      </div>
                      <div>
                        <div className='text-muted-foreground text-xs'>{t('apikeys.profiles.quotaCost')}</div>
                        <div className='text-sm'>
                          {(quotaUsage.usage.totalCost ?? 0).toLocaleString(undefined, { maximumFractionDigits: 6 })}/{currentQuota?.cost ?? '∞'}
                        </div>
                      </div>
                    </div>
                    <div className='text-muted-foreground grid gap-2 text-xs md:grid-cols-2'>
                      <div>
                        {t('common.filters.startTime')}{' '}
                        {quotaUsage.window.start ? format(quotaUsage.window.start, 'PPpp', { locale }) : '-'}
                      </div>
                      <div>
                        {t('common.filters.endTime')}{' '}
                        {quotaUsageEnd ? format(quotaUsageEnd, 'PPpp', { locale }) : '-'}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Load Balancer Strategy */}
          <div className='border-t pt-6'>
            <FormField
              control={form.control}
              name={`profiles.${profileIndex}.loadBalanceStrategy`}
              render={({ field }) => (
                <FormItem className='space-y-4'>
                  <div className='flex items-center justify-between gap-3'>
                    <div>
                      <h4 className='text-sm font-medium'>{t('apikeys.profiles.loadBalancerStrategy')}</h4>
                      <FormDescription className='mt-1 text-xs'>
                        {field.value === 'adaptive'
                          ? t('system.retry.loadBalancerStrategy.documentation.adaptive')
                          : field.value === 'failover'
                          ? t('system.retry.loadBalancerStrategy.documentation.failover')
                          : field.value === 'circuit-breaker'
                          ? t('system.retry.loadBalancerStrategy.documentation.circuit-breaker')
                          : t('apikeys.profiles.loadBalancerStrategyDescription')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Select
                        onValueChange={(val) => field.onChange(val === 'system_default' ? null : val)}
                        value={field.value || 'system_default'}
                      >
                        <SelectTrigger className='w-[140px]'>
                          <SelectValue placeholder={t('apikeys.profiles.loadBalancerStrategyPlaceholder')} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value='system_default'>{t('apikeys.profiles.loadBalancerStrategyPlaceholder')}</SelectItem>
                          <SelectItem value='adaptive'>{t('system.retry.loadBalancerStrategy.options.adaptive')}</SelectItem>
                          <SelectItem value='failover'>{t('system.retry.loadBalancerStrategy.options.failover')}</SelectItem>
                          <SelectItem value='circuit-breaker'>{t('system.retry.loadBalancerStrategy.options.circuitBreaker')}</SelectItem>
                        </SelectContent>
                      </Select>
                    </FormControl>
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='border-t pt-6'>
            <div className='flex items-center justify-between'>
              <h4 className='text-sm font-medium'>{t('apikeys.profiles.modelMappings')}</h4>
              <Button type='button' variant='outline' size='sm' onClick={addMapping} className='mb-3 flex items-center gap-2'>
                <IconPlus className='h-4 w-4' />
                {t('apikeys.profiles.addMapping')}
              </Button>
            </div>

            {mappingFields.length === 0 && (
              <p className='text-muted-foreground py-4 text-center text-sm'>{t('apikeys.profiles.noMappings')}</p>
            )}

            <div className='space-y-3'>
              {mappingFields.map((mapping, mappingIndex) => (
                <MappingRow
                  key={mapping.id}
                  profileIndex={profileIndex}
                  mappingIndex={mappingIndex}
                  form={form}
                  onRemove={() => removeMapping(mappingIndex)}
                  availableModels={availableModels}
                  t={t}
                  portalContainer={portalContainer}
                />
              ))}
            </div>
          </div>

          {/* Model IDs Restrictions Section */}
          <div className='border-t pt-6'>
            <h4 className='mb-3 text-sm font-medium'>{t('apikeys.profiles.allowedModels')}</h4>
            <p className='text-muted-foreground mb-3 text-xs'>{t('apikeys.profiles.allowedModelsDescription')}</p>
            <FormField
              control={form.control}
              name={`profiles.${profileIndex}.modelIDs`}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <TagsAutocompleteInput
                      value={field.value || []}
                      onChange={field.onChange}
                      placeholder={t('apikeys.profiles.allowedModels')}
                      suggestions={availableModels}
                      className='h-auto min-h-9 py-1'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          {/* Channel Restrictions Section */}
          <div className='border-t pt-6'>
            <h4 className='mb-3 text-sm font-medium'>{t('apikeys.profiles.allowedChannels')}</h4>
            <p className='text-muted-foreground mb-3 text-xs'>{t('apikeys.profiles.allowedChannelsDescription')}</p>
            <FormField
              control={form.control}
              name={`profiles.${profileIndex}.channelIDs`}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <TagsAutocompleteInput
                      value={(field.value || []).map((id) => {
                        const channel = channelsData?.edges?.find((edge) => parseInt(extractNumberID(edge.node.id), 10) === id);
                        return channel?.node.name || id.toString();
                      })}
                      onChange={(tags) => {
                        const ids = tags
                          .map((tag) => {
                            const channel = channelsData?.edges?.find((edge) => edge.node.name === tag);
                            return channel ? parseInt(extractNumberID(channel.node.id), 10) : parseInt(tag);
                          })
                          .filter((id) => !isNaN(id));
                        field.onChange(ids);
                      }}
                      placeholder={t('apikeys.profiles.allowedChannels')}
                      suggestions={channelsData?.edges?.map((edge) => edge.node.name) || []}
                      className='h-auto min-h-9 py-1'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          {/* Channel Tags Restrictions Section */}
          <div className='border-t pt-6'>
            <h4 className='mb-3 text-sm font-medium'>{t('apikeys.profiles.allowedChannelTags')}</h4>
            <p className='text-muted-foreground mb-3 text-xs'>{t('apikeys.profiles.allowedChannelTagsDescription')}</p>
            <FormField
              control={form.control}
              name={`profiles.${profileIndex}.channelTags`}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <TagsAutocompleteInput
                      value={field.value || []}
                      onChange={field.onChange}
                      placeholder={t('apikeys.profiles.allowedChannelTags')}
                      suggestions={allTags}
                      className='h-auto min-h-9 py-1'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </CardContent>
      )}
    </Card>
  );
}

interface MappingRowProps {
  profileIndex: number;
  mappingIndex: number;
  form: ReturnType<typeof useForm<UpdateApiKeyProfilesInput>>;
  onRemove: () => void;
  availableModels: string[];
  t: (key: string) => string;
  /** Popover Portal 容器元素，解决 Dialog 内无法滚动的问题 */
  portalContainer?: HTMLElement | null;
}

function MappingRow({ profileIndex, mappingIndex, form, onRemove, availableModels, t, portalContainer }: MappingRowProps) {
  const fromFieldName = `profiles.${profileIndex}.modelMappings.${mappingIndex}.from` as const;
  const toFieldName = `profiles.${profileIndex}.modelMappings.${mappingIndex}.to` as const;

  const fromValue = form.watch(fromFieldName);
  const toValue = form.watch(toFieldName);

  const [fromSearch, setFromSearch] = useState(fromValue || '');
  const [toSearch, setToSearch] = useState(toValue || '');

  useEffect(() => {
    setFromSearch(fromValue || '');
  }, [fromValue]);

  useEffect(() => {
    setToSearch(toValue || '');
  }, [toValue]);

  useEffect(() => {
    form.trigger(fromFieldName);
  }, [form, fromFieldName, fromValue]);

  useEffect(() => {
    form.trigger(toFieldName);
  }, [form, toFieldName, toValue]);

  const modelOptions = useMemo(() => availableModels.map((model) => ({ value: model, label: model })), [availableModels]);

  return (
    <div className='flex items-start gap-3'>
      <FormField
        control={form.control}
        name={fromFieldName}
        render={({ field }) => (
          <FormItem className='flex-1'>
            <FormControl>
              <AutoComplete
                selectedValue={field.value || ''}
                onSelectedValueChange={(value) => {
                  field.onChange(value);
                }}
                searchValue={fromSearch}
                onSearchValueChange={setFromSearch}
                items={modelOptions}
                placeholder={t('apikeys.profiles.sourceModel')}
                emptyMessage={t('apikeys.profiles.noModelsFound')}
                portalContainer={portalContainer}
              />
            </FormControl>
            {/* <div className='text-muted-foreground mt-1 text-xs'>{t('apikeys.profiles.regexSupported')}</div> */}
            <FormMessage />
          </FormItem>
        )}
      />
      <span className='text-muted-foreground flex h-10 items-center'>→</span>
      <FormField
        control={form.control}
        name={toFieldName}
        render={({ field }) => (
          <FormItem className='flex-1'>
            <FormControl>
              <AutoComplete
                selectedValue={field.value || ''}
                onSelectedValueChange={(value) => {
                  field.onChange(value);
                }}
                searchValue={toSearch}
                onSearchValueChange={setToSearch}
                items={modelOptions}
                placeholder={t('apikeys.profiles.targetModel')}
                emptyMessage={t('apikeys.profiles.noModelsFound')}
                portalContainer={portalContainer}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <Button type='button' variant='ghost' size='sm' onClick={onRemove} className='text-destructive hover:text-destructive'>
        <IconTrash className='h-4 w-4' />
      </Button>
    </div>
  );
}
