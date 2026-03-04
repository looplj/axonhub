//@ts-ignore
'use client';

import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Eye, EyeOff } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Button } from '@/components/ui/button';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAllChannelTags, useCreateChannel, useUpdateChannel } from '../data/channels';
import { Channel, ChannelType, channelTypeSchema } from '../data/schema';
import { getDefaultBaseURL, getDefaultModels } from '../data/config_channels';

const searchChannelTypeSchema = channelTypeSchema.refine((v) => v.startsWith('search_'), {
  message: 'Search channel type is required',
});

const createSchema = z
  .object({
    type: searchChannelTypeSchema,
    name: z.string().min(1, 'Name is required'),
    baseURL: z.string().url('Please enter a valid URL'),
    apiKeysText: z.string(),
    tags: z.array(z.string()).optional().default([]),
    orderingWeight: z.coerce.number().int().optional().default(0),
    remark: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (parseApiKeysText(data.apiKeysText).length === 0) {
      ctx.addIssue({
        code: 'custom',
        message: 'At least one API Key is required',
        path: ['apiKeysText'],
      });
    }
  });

type FormValues = z.infer<typeof createSchema>;

function getDefaultModelForType(type: ChannelType): string {
  const models = getDefaultModels(type);
  return models.includes('__search') ? '__search' : models[0] || '__search';
}

function parseApiKeysText(text: string): string[] {
  return [...new Set(text.split('\n').map((k) => k.trim()).filter((k) => k.length > 0))];
}

export function SearchChannelDialog({
  open,
  onOpenChange,
  currentRow,
  duplicateFromRow,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow?: Channel;
  duplicateFromRow?: Channel;
}) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;
  const isDuplicate = !!duplicateFromRow && !isEdit;
  const initialRow = currentRow || duplicateFromRow;
  const createChannel = useCreateChannel();
  const updateChannel = useUpdateChannel();
  const { data: allTags = [] } = useAllChannelTags();

  const defaultType = useMemo<ChannelType>(() => {
    if (initialRow?.type && initialRow.type.startsWith('search_')) return initialRow.type;
    return 'search_tavily';
  }, [initialRow?.type]);

  const [selectedType, setSelectedType] = useState<ChannelType>(defaultType);
  const [showApiKey, setShowApiKey] = useState(false);

  const getInitialBaseURL = (row: typeof initialRow) => {
    const stored = row?.baseURL;
    return stored != null && stored !== '' ? stored : getDefaultBaseURL(defaultType);
  };

  const form = useForm<FormValues>({
    resolver: zodResolver(createSchema),
    defaultValues: {
      type: defaultType,
      name: initialRow?.name || '',
      baseURL: getInitialBaseURL(initialRow),
      apiKeysText: (initialRow?.credentials?.apiKeys || []).join('\n'),
      tags: initialRow?.tags || [],
      orderingWeight: initialRow?.orderingWeight || 0,
      remark: initialRow?.remark || '',
    },
  });

  useEffect(() => {
    if (!open) return;
    setSelectedType(defaultType);
    setShowApiKey(false);
    form.reset({
      type: defaultType,
      name: initialRow?.name || '',
      baseURL: getInitialBaseURL(initialRow),
      apiKeysText: (initialRow?.credentials?.apiKeys || []).join('\n'),
      tags: initialRow?.tags || [],
      orderingWeight: initialRow?.orderingWeight || 0,
      remark: initialRow?.remark || '',
    });
  }, [open, defaultType, initialRow, form]);

  useEffect(() => {
    form.setValue('type', selectedType, { shouldDirty: true });
    const currentBaseURL = form.getValues('baseURL');
    if (!currentBaseURL) {
      const url = getDefaultBaseURL(selectedType);
      if (url) {
        form.setValue('baseURL', url, { shouldDirty: true });
      }
    }
  }, [selectedType, form]);

  const title = isEdit
    ? t('channels.dialogs.search.editTitle', 'Edit Search Channel')
    : t('channels.dialogs.search.addTitle', 'Add Search Channel');

  const description = isEdit
    ? t('channels.dialogs.search.editDescription', 'Update search channel settings.')
    : t('channels.dialogs.search.addDescription', 'Create a new search channel.');

  const onSubmit = async (values: FormValues) => {
    const channelType = values.type as ChannelType;
    const supportedModels = getDefaultModels(channelType);
    const defaultTestModel = getDefaultModelForType(channelType);
    const apiKeys = parseApiKeysText(values.apiKeysText);

    if (isEdit && currentRow) {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          name: values.name,
          baseURL: values.baseURL,
          supportedModels,
          autoSyncSupportedModels: false,
          manualModels: [],
          defaultTestModel,
          tags: values.tags || [],
          orderingWeight: values.orderingWeight,
          remark: values.remark || null,
          credentials: { apiKeys },
        },
      });
    } else {
      await createChannel.mutateAsync({
        type: channelType,
        name: values.name,
        baseURL: values.baseURL,
        policies: { stream: 'forbid' },
        supportedModels,
        autoSyncSupportedModels: false,
        manualModels: [],
        tags: values.tags || [],
        defaultTestModel,
        orderingWeight: values.orderingWeight,
        remark: values.remark || undefined,
        settings: { modelMappings: [] },
        credentials: { apiKeys },
      });
    }

    onOpenChange(false);
  };

  const typeItems: { value: ChannelType; label: string }[] = [
    { value: 'search_tavily', label: t('channels.types.search_tavily') },
    { value: 'search_brave', label: t('channels.types.search_brave') },
    { value: 'search_exa', label: t('channels.types.search_exa', 'Exa Search') },
  ];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('channels.dialogs.fields.type.label', 'Type')}</FormLabel>
                    <Select
                      value={selectedType}
                      onValueChange={(v) => {
                        const parsed = channelTypeSchema.safeParse(v);
                        if (parsed.success) {
                          setSelectedType(parsed.data);
                          field.onChange(parsed.data);
                        }
                      }}
                      disabled={isEdit}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {typeItems.map((it) => (
                          <SelectItem key={it.value} value={it.value}>
                            {it.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('channels.dialogs.fields.name.label')}</FormLabel>
                    <Input autoComplete='off' data-testid='search-channel-name-input' {...field} />
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='baseURL'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.dialogs.fields.baseURL.label')}</FormLabel>
                  <Input autoComplete='new-password' data-form-type='other' data-testid='search-channel-base-url-input' {...field} />
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='apiKeysText'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.dialogs.fields.apiKey.label')}</FormLabel>
                  <div className='relative'>
                    {isEdit ? (
                      <Tooltip open={!showApiKey ? undefined : false}>
                        <TooltipTrigger asChild>
                          <Textarea
                            value={
                              showApiKey
                                ? field.value || ''
                                : parseApiKeysText(field.value || '')
                                    .map((k) => (k.length > 8 ? k.slice(0, 4) + '****' + k.slice(-4) : '****'))
                                    .join('\n')
                            }
                            onChange={(e) => {
                              if (!showApiKey) return;
                              field.onChange(e.target.value);
                            }}
                            readOnly={!showApiKey}
                            placeholder={t('channels.dialogs.fields.apiKey.editPlaceholder')}
                            className='min-h-[90px] resize-y pr-10 font-mono text-sm'
                            autoComplete='new-password'
                            data-form-type='other'
                            data-testid='search-channel-api-keys-input'
                          />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>{t('channels.dialogs.fields.apiKey.revealToEditHint')}</p>
                        </TooltipContent>
                      </Tooltip>
                    ) : (
                      <Textarea
                        value={field.value || ''}
                        onChange={(e) => field.onChange(e.target.value)}
                        placeholder={t('channels.dialogs.fields.apiKey.placeholder')}
                        className='min-h-[90px] resize-y pr-10 font-mono text-sm'
                        autoComplete='new-password'
                        data-form-type='other'
                        data-testid='search-channel-api-keys-input'
                      />
                    )}
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      className='absolute top-2 right-2 h-7 w-7 p-0'
                      onClick={() => setShowApiKey((prev) => !prev)}
                    >
                      {showApiKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                    </Button>
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='tags'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('channels.dialogs.fields.tags.label', 'Tags')}</FormLabel>
                    <TagsAutocompleteInput
                      value={field.value || []}
                      onChange={field.onChange}
                      suggestions={allTags}
                      placeholder={t('channels.dialogs.fields.tags.placeholder', 'Add tags...')}
                    />
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='orderingWeight'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('channels.columns.orderingWeight')}</FormLabel>
                    <Input type='number' {...field} />
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='remark'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.expandedRow.remark')}</FormLabel>
                  <Input {...field} />
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel', 'Cancel')}
              </Button>
              <Button type='submit' disabled={createChannel.isPending || updateChannel.isPending}>
                {isEdit ? t('common.buttons.save', 'Save') : t('common.buttons.create', 'Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
