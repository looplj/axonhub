import { useEffect, useState, useCallback, useMemo } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { useQueryModels } from '@/gql/models';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { extractNumberID } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { Textarea } from '@/components/ui/textarea';
import { AutoComplete } from '@/components/auto-complete';
import { useAllChannelSummarys } from '@/features/channels/data/channels';
import { useApiKeyProfileTemplates, useCreateApiKeyProfileTemplate } from '../data/apikeys';
import { formSchemaFactory, type FormValues } from '../data/template-form-schema';

interface ApiKeyCreateTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ApiKeyCreateTemplateDialog({ open, onOpenChange }: ApiKeyCreateTemplateDialogProps) {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const createTemplate = useCreateApiKeyProfileTemplate();
  const { data: existingTemplates } = useApiKeyProfileTemplates(selectedProjectId);
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels();
  const { data: channelsData } = useAllChannelSummarys(selectedProjectId, { enabled: true });
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  const allTags = useMemo(() => {
    const tagsSet = new Set<string>();
    channelsData?.edges?.forEach((edge) => {
      edge.node.tags?.forEach((tag) => {
        if (tag) tagsSet.add(tag);
      });
    });
    return Array.from(tagsSet).sort();
  }, [channelsData]);

  const formSchema = useMemo(() => formSchemaFactory(t), [t]);

  const defaultValues = useMemo(
    () => ({
      name: '',
      description: '',
      profile: {
        name: '',
        modelMappings: [] as { from: string; to: string }[],
        channelIDs: [] as number[],
        channelTags: [] as string[],
        channelTagsMatchMode: 'any' as const,
        modelIDs: [] as string[],
        loadBalanceStrategy: null as string | null,
        quota: null as FormValues['profile']['quota'],
      },
    }),
    []
  );

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues,
  });

  const watchName = form.watch('name');
  useEffect(() => {
    form.setValue('profile.name', watchName);
  }, [watchName, form]);

  useEffect(() => {
    if (open) {
      form.reset(defaultValues);
      fetchModels({ statusIn: ['enabled'], includeMapping: true, includePrefix: true });
    }
  }, [open, form, defaultValues, fetchModels]);

  const templateName = form.watch('name');

  useEffect(() => {
    if (!templateName?.trim()) {
      form.clearErrors('name');
      return;
    }
    const isDuplicate = existingTemplates?.some((tmpl) => tmpl.name.trim().toLowerCase() === templateName.trim().toLowerCase());
    if (isDuplicate) {
      form.setError('name', { type: 'manual', message: t('apikeys.templates.templateNameDuplicate') });
    } else {
      form.clearErrors('name');
    }
  }, [templateName, existingTemplates, form, t]);

  const {
    fields: mappingFields,
    append: appendMapping,
    remove: removeMapping,
  } = useFieldArray({ control: form.control, name: 'profile.modelMappings' });

  const addMapping = useCallback(() => {
    appendMapping({ from: '', to: '' });
  }, [appendMapping]);

  const channelTagsMatchMode = form.watch('profile.channelTagsMatchMode');
  const isExcludeMode = channelTagsMatchMode === 'none';
  const hasQuota = form.watch('profile.quota') != null;
  const quotaPeriodType = form.watch('profile.quota.period.type');

  const handleSubmit = async (values: FormValues) => {
    try {
      await createTemplate.mutateAsync({
        name: values.name,
        description: values.description || '',
        projectID: selectedProjectId,
        profile: {
          ...values.profile,
          name: values.name,
        },
      });
      toast.success(t('apikeys.profileTemplates.createSuccess'));
      onOpenChange(false);
    } catch {
      toast.error(t('apikeys.profileTemplates.createError'));
    }
  };

  const isSubmitting = createTemplate.isPending;
  const hasDuplicateError = form.formState.errors.name?.type === 'manual';
  const modelOptions = useMemo(
    () => (availableModels?.map((model) => model.id) || []).map((m) => ({ value: m, label: m })),
    [availableModels]
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent ref={setDialogContent} className='flex max-h-[90vh] flex-col sm:max-w-4xl'>
        <DialogHeader className='shrink-0 text-left'>
          <DialogTitle>{t('apikeys.profileTemplates.createTitle')}</DialogTitle>
          <DialogDescription>{t('apikeys.profileTemplates.createDescription')}</DialogDescription>
        </DialogHeader>

        <div className='flex-1 overflow-y-auto px-1'>
          <Form {...form}>
            <form id='create-template-form' onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6'>
              <div className='space-y-4 border-b pb-6'>
                <h3 className='text-lg font-medium'>{t('apikeys.profileTemplates.templateDetails')}</h3>
                <div className='grid gap-4 md:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('apikeys.templates.nameLabel')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('apikeys.templates.namePlaceholder')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='description'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('apikeys.templates.descriptionLabel')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            value={field.value ?? ''}
                            placeholder={t('apikeys.templates.descriptionPlaceholder')}
                            rows={2}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </div>

              <div className='border-t pt-6'>
                <FormField
                  control={form.control}
                  name='profile.loadBalanceStrategy'
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
                                  : field.value === 'round-robin'
                                    ? t('system.retry.loadBalancerStrategy.documentation.round-robin')
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
                              <SelectItem value='circuit-breaker'>
                                {t('system.retry.loadBalancerStrategy.options.circuitBreaker')}
                              </SelectItem>
                              <SelectItem value='round-robin'>{t('system.retry.loadBalancerStrategy.options.roundRobin')}</SelectItem>
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
                    <CreateMappingRow
                      key={mapping.id}
                      mappingIndex={mappingIndex}
                      form={form}
                      onRemove={() => removeMapping(mappingIndex)}
                      modelOptions={modelOptions}
                      t={t}
                      portalContainer={dialogContent}
                    />
                  ))}
                </div>
              </div>

              <div className='border-t pt-6'>
                <h4 className='mb-3 text-sm font-medium'>{t('apikeys.profiles.allowedModels')}</h4>
                <p className='text-muted-foreground mb-3 text-xs'>{t('apikeys.profiles.allowedModelsDescription')}</p>
                <FormField
                  control={form.control}
                  name='profile.modelIDs'
                  render={({ field }) => (
                    <FormItem>
                      <FormControl>
                        <TagsAutocompleteInput
                          value={field.value || []}
                          onChange={field.onChange}
                          placeholder={t('apikeys.profiles.allowedModels')}
                          suggestions={availableModels?.map((model) => model.id) || []}
                          className='h-auto min-h-9 py-1'
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='border-t pt-6'>
                <h4 className='mb-3 text-sm font-medium'>{t('apikeys.profiles.allowedChannels')}</h4>
                <p className='text-muted-foreground mb-3 text-xs'>{t('apikeys.profiles.allowedChannelsDescription')}</p>
                <FormField
                  control={form.control}
                  name='profile.channelIDs'
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

              <div className='border-t pt-6'>
                <div className='mb-3 flex items-start justify-between gap-3'>
                  <div>
                    <h4 className='text-sm font-medium'>
                      {t(isExcludeMode ? 'apikeys.profiles.excludedChannelTags' : 'apikeys.profiles.allowedChannelTags')}
                    </h4>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t(
                        isExcludeMode ? 'apikeys.profiles.excludedChannelTagsDescription' : 'apikeys.profiles.allowedChannelTagsDescription'
                      )}
                    </p>
                  </div>
                  <FormField
                    control={form.control}
                    name='profile.channelTagsMatchMode'
                    render={({ field }) => (
                      <FormItem className='w-[180px]'>
                        <FormLabel>{t('apikeys.profiles.allowedChannelTagsMatchMode')}</FormLabel>
                        <FormControl>
                          <Select value={field.value || 'any'} onValueChange={field.onChange}>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='any'>{t('apikeys.profiles.allowedChannelTagsMatchModeAny')}</SelectItem>
                              <SelectItem value='all'>{t('apikeys.profiles.allowedChannelTagsMatchModeAll')}</SelectItem>
                              <SelectItem value='none'>{t('apikeys.profiles.allowedChannelTagsMatchModeNone')}</SelectItem>
                            </SelectContent>
                          </Select>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                <FormField
                  control={form.control}
                  name='profile.channelTags'
                  render={({ field }) => (
                    <FormItem>
                      <FormControl>
                        <TagsAutocompleteInput
                          value={field.value || []}
                          onChange={field.onChange}
                          placeholder={t(isExcludeMode ? 'apikeys.profiles.excludedChannelTags' : 'apikeys.profiles.allowedChannelTags')}
                          suggestions={allTags}
                          className='h-auto min-h-9 py-1'
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='border-t pt-6'>
                <div className='space-y-4'>
                  <div className='flex items-center justify-between gap-3'>
                    <div>
                      <h4 className='text-sm font-medium'>{t('apikeys.profiles.quotaTitle')}</h4>
                      <p className='text-muted-foreground mt-1 text-xs'>{t('apikeys.profiles.quotaDescription')}</p>
                    </div>
                    <FormField
                      control={form.control}
                      name='profile.quota'
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

                  {hasQuota && (
                    <div className='space-y-4'>
                      <div className='grid gap-4 md:grid-cols-3'>
                        <FormField
                          control={form.control}
                          name='profile.quota.requests'
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
                          name='profile.quota.totalTokens'
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
                          name='profile.quota.cost'
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
                          name='profile.quota.period.type'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('apikeys.profiles.quotaPeriodType')}</FormLabel>
                              <FormControl>
                                <Select
                                  value={field.value}
                                  onValueChange={(value) => {
                                    field.onChange(value);
                                    if (value === 'past_duration') {
                                      form.setValue('profile.quota.period.pastDuration', { value: 1, unit: 'day' });
                                      form.setValue('profile.quota.period.calendarDuration', null);
                                    } else if (value === 'calendar_duration') {
                                      form.setValue('profile.quota.period.calendarDuration', { unit: 'day' });
                                      form.setValue('profile.quota.period.pastDuration', null);
                                    } else {
                                      form.setValue('profile.quota.period.pastDuration', null);
                                      form.setValue('profile.quota.period.calendarDuration', null);
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

                        {quotaPeriodType === 'past_duration' && (
                          <>
                            <FormField
                              control={form.control}
                              name='profile.quota.period.pastDuration.value'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>{t('apikeys.profiles.quotaPastDurationValue')}</FormLabel>
                                  <FormControl>
                                    <Input
                                      type='number'
                                      min={1}
                                      value={(field.value as unknown as number | null | undefined) ?? ''}
                                      onChange={(e) => {
                                        const v = e.target.value;
                                        field.onChange(v === '' ? null : Number(v));
                                      }}
                                    />
                                  </FormControl>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />
                            <FormField
                              control={form.control}
                              name='profile.quota.period.pastDuration.unit'
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

                        {quotaPeriodType === 'calendar_duration' && (
                          <FormField
                            control={form.control}
                            name='profile.quota.period.calendarDuration.unit'
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
                    </div>
                  )}
                </div>
              </div>
            </form>
          </Form>
        </div>

        <DialogFooter className='shrink-0'>
          <div className='flex w-full gap-2 sm:w-auto'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={isSubmitting}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='submit' form='create-template-form' disabled={isSubmitting || hasDuplicateError}>
              {isSubmitting ? t('common.buttons.saving') : t('apikeys.profileTemplates.createButton')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

interface CreateMappingRowProps {
  mappingIndex: number;
  form: ReturnType<typeof useForm<FormValues>>;
  onRemove: () => void;
  modelOptions: { value: string; label: string }[];
  t: (key: string) => string;
  portalContainer?: HTMLElement | null;
}

function CreateMappingRow({ mappingIndex, form, onRemove, modelOptions, t, portalContainer }: CreateMappingRowProps) {
  const fromFieldName = `profile.modelMappings.${mappingIndex}.from` as const;
  const toFieldName = `profile.modelMappings.${mappingIndex}.to` as const;

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
                onSelectedValueChange={(value) => field.onChange(value)}
                searchValue={fromSearch}
                onSearchValueChange={setFromSearch}
                items={modelOptions}
                placeholder={t('apikeys.profiles.sourceModel')}
                emptyMessage={t('apikeys.profiles.noModelsFound')}
                portalContainer={portalContainer}
              />
            </FormControl>
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
                onSelectedValueChange={(value) => field.onChange(value)}
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
