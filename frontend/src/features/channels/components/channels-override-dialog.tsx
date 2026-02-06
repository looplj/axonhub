import { useEffect, useState, useCallback } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2, Save, Download } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Textarea } from '@/components/ui/textarea';
import { useUpdateChannel } from '../data/channels';
import { Channel, HeaderEntry } from '../data/schema';

interface ParameterEntry {
  key: string;
  value: string;
}

function jsonToParams(json: string): ParameterEntry[] {
  try {
    const parsed = JSON.parse(json || '{}');
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      return [];
    }
    return Object.entries(parsed).map(([key, value]) => ({
      key,
      value: typeof value === 'string' ? value : JSON.stringify(value),
    }));
  } catch (e) {
    return [];
  }
}

function paramsToJson(params: ParameterEntry[]): string {
  const obj: Record<string, any> = {};
  params.forEach((p) => {
    const key = p.key.trim();
    if (key) {
      try {
        // Try to parse the value as JSON (for numbers, booleans, objects, arrays)
        // This allows users to input 0.7 as a number, true as a boolean, etc.
        obj[key] = JSON.parse(p.value);
      } catch (e) {
        // If it's not valid JSON, treat it as a string
        obj[key] = p.value;
      }
    }
  });
  return JSON.stringify(obj);
}
import { useChannelOverrideTemplates, useCreateChannelOverrideTemplate } from '../data/templates';
import { mergeChannelSettingsForUpdate, mergeOverrideHeaders, mergeOverrideParameters, normalizeOverrideParameters } from '../utils/merge';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

const AUTH_HEADER_KEYS = ['authorization', 'proxy-authorization', 'x-api-key', 'x-api-secret', 'x-api-token'];

interface SaveTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (name: string, description?: string) => void;
  isSaving: boolean;
}

function SaveTemplateDialog({ open, onOpenChange, onSave, isSaving }: SaveTemplateDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const handleSave = () => {
    if (!name.trim()) {
      toast.error(t('channels.templates.validation.nameRequired'));
      return;
    }
    onSave(name.trim(), description.trim() || undefined);
  };

  const handleOpenChange = (newOpen: boolean) => {
    if (!newOpen) {
      setName('');
      setDescription('');
    }
    onOpenChange(newOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.templates.dialogs.save.title')}</DialogTitle>
          <DialogDescription>{t('channels.templates.dialogs.save.description')}</DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label htmlFor='template-name'>{t('channels.templates.fields.name')}</Label>
            <Input
              id='template-name'
              placeholder={t('channels.templates.fields.namePlaceholder')}
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isSaving}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='template-description'>{t('channels.templates.fields.description')}</Label>
            <Textarea
              id='template-description'
              placeholder={t('channels.templates.fields.descriptionPlaceholder')}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={isSaving}
              className='min-h-[80px]'
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => handleOpenChange(false)} disabled={isSaving}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={isSaving}>
            {isSaving ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('common.buttons.saving')}
              </>
            ) : (
              <>
                <Save className='mr-2 h-4 w-4' />
                {t('common.buttons.save')}
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const overrideFormSchema = z.object({
  overrideHeaders: z
    .array(
      z.object({
        key: z.string().min(1, 'Header key is required'),
        value: z.string(),
      })
    )
    .optional(),
  overrideParameters: z
    .string()
    .optional()
    .superRefine((val, ctx) => {
      if (!val || val.trim() === '') return;

      let parsed: unknown;
      try {
        parsed = JSON.parse(val);
      } catch {
        ctx.addIssue({
          code: 'custom',
          message: 'channels.validation.overrideParametersInvalidJson',
        });
        return;
      }

      if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
        ctx.addIssue({
          code: 'custom',
          message: 'channels.validation.overrideParametersInvalidJson',
        });
        return;
      }

      if (Object.prototype.hasOwnProperty.call(parsed, 'stream')) {
        ctx.addIssue({
          code: 'custom',
          message: 'channels.validation.overrideParametersStreamNotAllowed',
        });
      }
    }),
});

export function ChannelsOverrideDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const createTemplate = useCreateChannelOverrideTemplate();

  // Template states
  const [showSaveTemplateDialog, setShowSaveTemplateDialog] = useState(false);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [templateSearchOpen, setTemplateSearchOpen] = useState(false);
  const [templateSearchValue, setTemplateSearchValue] = useState('');
  const [isApplyingTemplate, setIsApplyingTemplate] = useState(false);

  // Fetch templates for this channel type
  const { data: templatesData } = useChannelOverrideTemplates(
    {
      channelType: currentRow.type,
      search: templateSearchValue,
      first: 50,
    },
    {
      enabled: open,
    }
  );

  const templates = templatesData?.edges?.map((edge) => edge.node) || [];

  const [headers, setHeaders] = useState<HeaderEntry[]>(currentRow.settings?.overrideHeaders || []);
  const [parameters, setParameters] = useState<ParameterEntry[]>(jsonToParams(currentRow.settings?.overrideParameters || ''));

  const form = useForm<z.infer<typeof overrideFormSchema>>({
    resolver: zodResolver(overrideFormSchema),
    defaultValues: {
      overrideHeaders: currentRow.settings?.overrideHeaders || [],
      overrideParameters: currentRow.settings?.overrideParameters || '',
    },
  });

  useEffect(() => {
    const nextHeaders = currentRow.settings?.overrideHeaders || [];
    const nextParameters = currentRow.settings?.overrideParameters || '';
    setHeaders(nextHeaders);
    setParameters(jsonToParams(nextParameters));
    form.reset({
      overrideHeaders: nextHeaders,
      overrideParameters: nextParameters,
    });
  }, [currentRow, open, form]);

  const addHeader = useCallback(() => {
    const newHeaders = [...headers, { key: '', value: '' }];
    setHeaders(newHeaders);
    form.setValue('overrideHeaders', newHeaders);
  }, [headers, form]);

  const removeHeader = useCallback(
    (index: number) => {
      const newHeaders = headers.filter((_, i) => i !== index);
      setHeaders(newHeaders);
      form.setValue('overrideHeaders', newHeaders);
    },
    [headers, form]
  );

  const updateHeader = useCallback(
    (index: number, field: keyof HeaderEntry, value: string) => {
      const newHeaders = headers.map((header, i) => (i === index ? { ...header, [field]: value } : header));
      setHeaders(newHeaders);
      form.setValue('overrideHeaders', newHeaders);

      // Trigger validation for the specific field if it's the key field
      if (field === 'key') {
        form.trigger(`overrideHeaders.${index}.key`);
      }
    },
    [headers, form]
  );

  const addParameter = useCallback(() => {
    const newParameters = [...parameters, { key: '', value: '' }];
    setParameters(newParameters);
    form.setValue('overrideParameters', paramsToJson(newParameters));
  }, [parameters, form]);

  const removeParameter = useCallback(
    (index: number) => {
      const newParameters = parameters.filter((_, i) => i !== index);
      setParameters(newParameters);
      form.setValue('overrideParameters', paramsToJson(newParameters));
    },
    [parameters, form]
  );

  const updateParameter = useCallback(
    (index: number, field: keyof ParameterEntry, value: string) => {
      const newParameters = parameters.map((param, i) => (i === index ? { ...param, [field]: value } : param));
      setParameters(newParameters);
      form.setValue('overrideParameters', paramsToJson(newParameters), {
        shouldValidate: true,
        shouldDirty: true,
      });
    },
    [parameters, form]
  );

  const onSubmit = async (values: z.infer<typeof overrideFormSchema>) => {
    try {
      // Filter out headers with empty keys
      const validHeaders = values.overrideHeaders?.filter((header) => header.key.trim() !== '') || [];

      // Normalize and parse overrideParameters if provided
      const normalizedParams = normalizeOverrideParameters(values.overrideParameters || '');

      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        overrideParameters: normalizedParams,
        overrideHeaders: validHeaders,
      });

      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: nextSettings,
        },
      });
      toast.success(t('channels.messages.updateSuccess'));
      onOpenChange(false);
    } catch (_error) {
      toast.error(t('channels.messages.updateError'));
    }
  };

  // Handle apply template
  const handleApplyTemplate = useCallback(async () => {
    if (!selectedTemplateId) return;

    const template = templates.find((t) => t.id === selectedTemplateId);
    if (!template) return;

    setIsApplyingTemplate(true);
    try {
      // Parse template data
      const templateHeaders = template.overrideHeaders || [];
      const templateParams = template.overrideParameters || '';

      // Use merge utilities to match backend behavior
      const mergedHeaders = mergeOverrideHeaders(headers, templateHeaders);
      const mergedParams = mergeOverrideParameters(paramsToJson(parameters), templateParams);

      setHeaders(mergedHeaders);
      setParameters(jsonToParams(mergedParams));
      form.setValue('overrideHeaders', mergedHeaders);
      form.setValue('overrideParameters', mergedParams);

      toast.success(t('channels.templates.messages.applied'));
    } catch (error) {
      toast.error(t('channels.templates.messages.applyError'));
    } finally {
      setIsApplyingTemplate(false);
    }
  }, [selectedTemplateId, templates, headers, parameters, form, t]);

  // Handle save as template
  const handleSaveAsTemplate = useCallback(
    async (name: string, description?: string) => {
      // Filter out headers with empty keys
      const validHeaders = headers.filter((h) => h.key.trim() !== '');

      // Normalize empty parameters to "{}"
      const normalizedParams = normalizeOverrideParameters(paramsToJson(parameters));

      try {
        await createTemplate.mutateAsync({
          name,
          description,
          channelType: currentRow.type,
          overrideParameters: normalizedParams,
          overrideHeaders: validHeaders,
        });
        setShowSaveTemplateDialog(false);
      } catch (error) {
        // Error already handled by mutation
      }
    },
    [headers, parameters, currentRow.type, createTemplate]
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[800px]'>
        <DialogHeader>
          <DialogTitle data-testid='override-dialog-title'>{t('channels.dialogs.settings.overrides.title')}</DialogTitle>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)}>
          <div className='space-y-6'>
            {/* Template Section */}
            <Card data-testid='override-template-section'>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.templates.section.title')}</CardTitle>
                <CardDescription>{t('channels.templates.section.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-3'>
                <div className='flex flex-wrap gap-2'>
                  <div className='min-w-48 flex-1'>
                    <Popover open={templateSearchOpen} onOpenChange={setTemplateSearchOpen}>
                      <PopoverTrigger asChild>
                        <Button
                          variant='outline'
                          role='combobox'
                          aria-expanded={templateSearchOpen}
                          className='w-full justify-between'
                          type='button'
                        >
                          {selectedTemplateId
                            ? templates.find((t) => t.id === selectedTemplateId)?.name
                            : t('channels.templates.selectTemplate')}
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className='w-[400px] p-0'>
                        <Command>
                          <CommandInput
                            placeholder={t('channels.templates.searchPlaceholder')}
                            value={templateSearchValue}
                            onValueChange={setTemplateSearchValue}
                          />
                          <CommandList>
                            <CommandEmpty>{t('channels.templates.noTemplates')}</CommandEmpty>
                            <CommandGroup>
                              {templates.map((template) => (
                                <CommandItem
                                  key={template.id}
                                  value={template.id}
                                  onSelect={() => {
                                    setSelectedTemplateId(template.id);
                                    setTemplateSearchOpen(false);
                                  }}
                                >
                                  <div className='flex flex-col'>
                                    <span className='font-medium'>{template.name}</span>
                                    {template.description && <span className='text-muted-foreground text-xs'>{template.description}</span>}
                                  </div>
                                </CommandItem>
                              ))}
                            </CommandGroup>
                          </CommandList>
                        </Command>
                      </PopoverContent>
                    </Popover>
                  </div>
                  <Button
                    type='button'
                    variant='default'
                    onClick={handleApplyTemplate}
                    disabled={!selectedTemplateId || isApplyingTemplate}
                  >
                    {isApplyingTemplate ? (
                      <>
                        <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                        {t('channels.templates.applying')}
                      </>
                    ) : (
                      <>
                        <Download className='mr-2 h-4 w-4' />
                        {t('channels.templates.applyTemplate')}
                      </>
                    )}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => setShowSaveTemplateDialog(true)}
                    disabled={headers.length === 0 && parameters.length === 0}
                  >
                    <Save className='mr-2 h-4 w-4' />
                    {t('channels.templates.saveAsTemplate')}
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* Headers Section */}
            <Card data-testid='override-headers-section'>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrides.headers.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.overrides.headers.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-3'>
                {headers.map((header, index) => {
                  const fieldError = form.formState.errors.overrideHeaders?.[index]?.key;
                  const normalizedKey = header.key.trim().toLowerCase();
                  const isAuthHeader = normalizedKey !== '' && AUTH_HEADER_KEYS.includes(normalizedKey);
                  return (
                    <div key={index} className='flex items-start gap-3'>
                      <div className='flex-1 space-y-2'>
                        <Label htmlFor={`header-key-${index}`} className='text-sm font-medium'>
                          {t('channels.dialogs.settings.overrides.headers.key')}
                        </Label>
                        <Input
                          id={`header-key-${index}`}
                          data-testid={`header-key-${index}`}
                          placeholder='User-Agent'
                          value={header.key}
                          onChange={(e) => updateHeader(index, 'key', e.target.value)}
                          className={`font-mono ${fieldError ? 'border-destructive' : ''}`}
                        />
                        {isAuthHeader && (
                          <p className='text-destructive text-sm' role='alert'>
                            {t('channels.dialogs.settings.overrides.headers.sensitiveWarning')}
                          </p>
                        )}
                      </div>
                      <div className='flex-1 space-y-2'>
                        <Label htmlFor={`header-value-${index}`} className='text-sm font-medium'>
                          {t('channels.dialogs.settings.overrides.headers.value')}
                        </Label>
                        <Input
                          id={`header-value-${index}`}
                          data-testid={`header-value-${index}`}
                          placeholder={t('channels.dialogs.settings.overrides.headers.valuePlaceholder')}
                          value={header.value}
                          onChange={(e) => updateHeader(index, 'value', e.target.value)}
                          className='font-mono'
                        />
                      </div>
                      <div className='pt-7'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => removeHeader(index)}
                          className='px-3'
                          data-testid={`remove-header-${index}`}
                        >
                          {t('common.buttons.remove')}
                        </Button>
                      </div>
                    </div>
                  );
                })}

                <Button type='button' variant='outline' onClick={addHeader} className='w-full' data-testid='add-header-button'>
                  {t('channels.dialogs.settings.overrides.headers.addButton')}
                </Button>

                {form.formState.errors.overrideHeaders?.message && (
                  <p className='text-destructive text-sm'>{t(form.formState.errors.overrideHeaders.message.toString())}</p>
                )}
              </CardContent>
            </Card>

            {/* Parameters Section */}
            <Card data-testid='override-parameters-section'>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrides.parameters.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.settings.overrides.parameters.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-3'>
                {parameters.map((param, index) => (
                  <div key={index} className='flex items-start gap-3'>
                    <div className='flex-1 space-y-2'>
                      <Label htmlFor={`param-key-${index}`} className='text-sm font-medium'>
                        {t('channels.dialogs.settings.overrides.parameters.key')}
                      </Label>
                      <Input
                        id={`param-key-${index}`}
                        data-testid={`param-key-${index}`}
                        placeholder='temperature'
                        value={param.key}
                        onChange={(e) => updateParameter(index, 'key', e.target.value)}
                        className='font-mono'
                      />
                    </div>
                    <div className='flex-1 space-y-2'>
                      <Label htmlFor={`param-value-${index}`} className='text-sm font-medium'>
                        {t('channels.dialogs.settings.overrides.parameters.value')}
                      </Label>
                      <Input
                        id={`param-value-${index}`}
                        data-testid={`param-value-${index}`}
                        placeholder='0.7'
                        value={param.value}
                        onChange={(e) => updateParameter(index, 'value', e.target.value)}
                        className='font-mono'
                      />
                    </div>
                    <div className='pt-7'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => removeParameter(index)}
                        className='px-3'
                        data-testid={`remove-param-${index}`}
                      >
                        {t('common.buttons.remove')}
                      </Button>
                    </div>
                  </div>
                ))}

                <Button type='button' variant='outline' onClick={addParameter} className='w-full' data-testid='add-param-button'>
                  {t('channels.dialogs.settings.overrides.parameters.addButton')}
                </Button>

                {form.formState.errors.overrideParameters?.message && (
                  <p className='text-destructive text-sm'>{t(form.formState.errors.overrideParameters.message.toString())}</p>
                )}
              </CardContent>
            </Card>
          </div>

          <DialogFooter className='mt-6'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)} data-testid='override-cancel-button'>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='submit' disabled={updateChannel.isPending} data-testid='override-save-button'>
              {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>

      <SaveTemplateDialog
        open={showSaveTemplateDialog}
        onOpenChange={setShowSaveTemplateDialog}
        onSave={handleSaveAsTemplate}
        isSaving={createTemplate.isPending}
      />
    </Dialog>
  );
}
