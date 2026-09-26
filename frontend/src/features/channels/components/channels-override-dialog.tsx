import { useEffect, useState, useCallback, useMemo } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2, Save, ChevronDown, Pencil, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { useDebounce } from '@/hooks/use-debounce';
import { useUpdateChannel } from '../data/channels';
import { Channel, OverrideOperation } from '../data/schema';
import {
  ChannelOverrideTemplate,
  useChannelOverrideTemplates,
  useCreateChannelOverrideTemplate,
  useDeleteChannelOverrideTemplate,
} from '../data/templates';
import { mergeChannelSettingsForUpdate, mergeOverrideHeaders, mergeOverrideOperations } from '../utils/merge';
import {
  OverrideFormValues,
  isValidBodyOp,
  isValidHeaderOp,
  overrideFormSchema,
  validateOverrideOperations,
} from '../utils/override-operations';
import { ChannelOverrideTemplateEditDialog } from './channel-override-template-edit-dialog';
import { HeaderOperationRow, OperationRow } from './override-operation-rows';


interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}


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


export function ChannelsOverrideDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const createTemplate = useCreateChannelOverrideTemplate();
  const deleteTemplate = useDeleteChannelOverrideTemplate();

  const [showSaveTemplateDialog, setShowSaveTemplateDialog] = useState(false);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [editingTemplate, setEditingTemplate] = useState<ChannelOverrideTemplate | null>(null);
  const [templateSearchOpen, setTemplateSearchOpen] = useState(false);
  const [templateSearchValue, setTemplateSearchValue] = useState('');
  const debouncedTemplateSearchValue = useDebounce(templateSearchValue, 300);

  const { data: templatesData } = useChannelOverrideTemplates(
    {
      search: debouncedTemplateSearchValue,
      first: 50,
    },
    {
      enabled: open,
    }
  );

  const templates = useMemo(() => templatesData?.edges?.map((edge) => edge.node) || [], [templatesData]);

  const form = useForm<OverrideFormValues>({
    resolver: zodResolver(overrideFormSchema),
    defaultValues: {
      headerOverrideOperations: currentRow.settings?.headerOverrideOperations || [],
      bodyOverrideOperations: currentRow.settings?.bodyOverrideOperations || [],
    },
  });

  const {
    fields: headerFields,
    append: appendHeader,
    remove: removeHeader,
    replace: replaceHeaders,
  } = useFieldArray({
    control: form.control,
    name: 'headerOverrideOperations',
  });

  const {
    fields: bodyFields,
    append: appendBody,
    remove: removeBody,
    replace: replaceBodies,
  } = useFieldArray({
    control: form.control,
    name: 'bodyOverrideOperations',
  });

  useEffect(() => {
    const nextHeaders = currentRow.settings?.headerOverrideOperations || [];
    const nextParams = currentRow.settings?.bodyOverrideOperations || [];
    form.reset({
      headerOverrideOperations: nextHeaders,
      bodyOverrideOperations: nextParams,
    });
  }, [currentRow, open, form]);

  const addHeaderOp = useCallback(() => {
    appendHeader({ op: 'set', path: '', value: '' });
  }, [appendHeader]);

  const removeHeaderOp = useCallback(
    (index: number) => {
      removeHeader(index);
    },
    [removeHeader]
  );

  const updateHeaderOp = useCallback(
    (index: number, data: Partial<OverrideOperation>) => {
      Object.entries(data).forEach(([key, value]) => {
        form.setValue(`headerOverrideOperations.${index}.${key}` as any, value);
      });
    },
    [form]
  );

  const addBodyOperation = useCallback(() => {
    appendBody({ op: 'set', path: '', value: '' });
  }, [appendBody]);

  const removeBodyOperation = useCallback(
    (index: number) => {
      removeBody(index);
    },
    [removeBody]
  );

  const updateBodyOperation = useCallback(
    (index: number, data: Partial<OverrideOperation>) => {
      Object.entries(data).forEach(([key, value]) => {
        form.setValue(`bodyOverrideOperations.${index}.${key}` as any, value);
      });
    },
    [form]
  );

  const onSubmit = async (data: OverrideFormValues) => {
    const bodyOps = data.bodyOverrideOperations || [];
    const headerOps = data.headerOverrideOperations || [];

    const validationError = validateOverrideOperations(t, headerOps, bodyOps);
    if (validationError) {
      toast.error(validationError);
      return;
    }

    try {
      const validHeaderOps = headerOps.filter(isValidHeaderOp);
      const validBodyOps = bodyOps.filter(isValidBodyOp);
      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        bodyOverrideOperations: validBodyOps,
        headerOverrideOperations: validHeaderOps,
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
      toast.error(t('common.errors.internalServerError'));
    }
  };

  const handleApplyTemplate = useCallback(
    async (templateId?: string) => {
      const id = templateId || selectedTemplateId;
      if (!id) return;

      const template = templates.find((t) => t.id === id);
      if (!template) return;

      try {
        const templateHeaders = template.headerOverrideOperations || [];
        const templateBodyOps = template.bodyOverrideOperations || [];

        const currentHeaders = form.getValues('headerOverrideOperations') || [];
        const currentBodyOps = form.getValues('bodyOverrideOperations') || [];

        const mergedHeaders = mergeOverrideHeaders(currentHeaders, templateHeaders);
        const mergedBodyOps = mergeOverrideOperations(currentBodyOps, templateBodyOps);

        replaceHeaders(mergedHeaders);
        replaceBodies(mergedBodyOps);

        toast.success(t('channels.templates.messages.applied'));
      } catch {
        toast.error(t('common.errors.internalServerError'));
      }
    },
    [selectedTemplateId, templates, form, replaceHeaders, replaceBodies, t]
  );

  const handleSaveAsTemplate = useCallback(
    async (name: string, description?: string) => {
      const headerOps = form.getValues('headerOverrideOperations') || [];
      const bodyOps = form.getValues('bodyOverrideOperations') || [];

      const validHeaderOps = headerOps.filter(isValidHeaderOp);
      const validBodyOps = bodyOps.filter(isValidBodyOp);
      try {
        await createTemplate.mutateAsync({
          name,
          description,
          headerOverrideOperations: validHeaderOps,
          bodyOverrideOperations: validBodyOps,
        });
        setShowSaveTemplateDialog(false);
      } catch {
        // Error already handled by mutation
      }
    },
    [form, createTemplate]
  );

  const handleDeleteTemplate = useCallback(
    async (e: React.MouseEvent, templateId: string) => {
      e.stopPropagation();
      e.preventDefault();
      await deleteTemplate.mutateAsync(templateId);
      if (selectedTemplateId === templateId) {
        setSelectedTemplateId(null);
      }
    },
    [deleteTemplate, selectedTemplateId]
  );

  const headerOpsCount = headerFields.length;
  const bodyOpsCount = bodyFields.length;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='h-[85vh] p-0 sm:max-w-[900px]'>
        <DialogHeader className='flex flex-row items-center justify-between border-b px-6 py-4 pr-12'>
          <div className='space-y-0.5'>
            <DialogTitle data-testid='override-dialog-title'>{t('channels.dialogs.settings.overrides.title')}</DialogTitle>
            <DialogDescription>{t('channels.templates.section.description')}</DialogDescription>
          </div>
          <div className='flex items-center gap-2'>
            <Popover open={templateSearchOpen} onOpenChange={setTemplateSearchOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant='outline'
                  size='sm'
                  role='combobox'
                  aria-expanded={templateSearchOpen}
                  className='h-9 w-[200px] justify-between'
                  type='button'
                >
                  <span className='truncate'>
                    {selectedTemplateId ? templates.find((t) => t.id === selectedTemplateId)?.name : t('channels.templates.selectTemplate')}
                  </span>
                  <ChevronDown className='ml-2 h-4 w-4 shrink-0 opacity-50' />
                </Button>
              </PopoverTrigger>
              <PopoverContent className='w-[300px] p-0' align='end'>
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
                            // Auto apply when selected in this compact UI
                            setTimeout(() => handleApplyTemplate(template.id), 0);
                          }}
                        >
                          <div className='flex flex-1 items-center justify-between'>
                            <div className='flex flex-col'>
                              <span className='font-medium'>{template.name}</span>
                              {template.description && (
                                <span className='text-muted-foreground line-clamp-1 text-xs'>{template.description}</span>
                              )}
                            </div>
                            <div className='flex shrink-0 items-center gap-0.5'>
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon'
                                className='text-muted-foreground h-6 w-6'
                                title={t('common.buttons.edit')}
                                data-testid={`template-edit-${template.id}`}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  e.preventDefault();
                                  setTemplateSearchOpen(false);
                                  setEditingTemplate(template);
                                }}
                              >
                                <Pencil className='h-3.5 w-3.5' />
                              </Button>
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon'
                                className='text-muted-foreground hover:text-destructive h-6 w-6'
                                title={t('common.buttons.delete')}
                                onClick={(e) => handleDeleteTemplate(e, template.id)}
                                disabled={deleteTemplate.isPending}
                              >
                                <Trash2 className='h-3.5 w-3.5' />
                              </Button>
                            </div>
                          </div>
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => setShowSaveTemplateDialog(true)}
              disabled={headerOpsCount === 0 && bodyOpsCount === 0}
              title={t('channels.templates.saveAsTemplate')}
              className='h-9 px-3'
            >
              <Save className='h-4 w-4' />
            </Button>
          </div>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)} className='flex h-[calc(85vh-80px)] flex-col'>
          <div className='flex flex-1 flex-col space-y-6 overflow-hidden px-6 pt-6'>
            {/* Override Tabs */}
            <Tabs defaultValue='body' className='flex min-h-0 w-full flex-1 flex-col overflow-hidden'>
              <TabsList className='grid w-full shrink-0 grid-cols-2'>
                <TabsTrigger value='body'>{t('channels.dialogs.settings.overrides.body.title')}</TabsTrigger>
                <TabsTrigger value='headers'>{t('channels.dialogs.settings.overrides.headers.title')}</TabsTrigger>
              </TabsList>

              <TabsContent value='headers' className='mt-4 flex flex-1 flex-col overflow-hidden'>
                <Card data-testid='override-headers-section' className='flex flex-1 flex-col overflow-hidden'>
                  <CardHeader className='shrink-0'>
                    <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrides.headers.title')}</CardTitle>
                    <CardDescription>{t('channels.dialogs.settings.overrides.headers.description')}</CardDescription>
                  </CardHeader>
                  <CardContent className='min-h-0 flex-1 space-y-3 overflow-y-auto'>
                    {headerFields.map((field, index) => (
                      <HeaderOperationRow
                        key={field.id}
                        index={index}
                        control={form.control}
                        onUpdate={updateHeaderOp}
                        onRemove={removeHeaderOp}
                      />
                    ))}

                    <Button type='button' variant='outline' onClick={addHeaderOp} className='w-full' data-testid='add-header-button'>
                      {t('channels.dialogs.settings.overrides.addButton')}
                    </Button>

                    {form.formState.errors.headerOverrideOperations?.message && (
                      <p className='text-destructive text-sm'>{t(form.formState.errors.headerOverrideOperations.message.toString())}</p>
                    )}
                  </CardContent>
                </Card>
              </TabsContent>

              <TabsContent value='body' className='mt-4 flex flex-1 flex-col overflow-hidden'>
                <Card data-testid='override-parameters-section' className='flex flex-1 flex-col overflow-hidden'>
                  <CardHeader className='shrink-0'>
                    <CardTitle className='text-lg'>{t('channels.dialogs.settings.overrides.body.title')}</CardTitle>
                    <CardDescription>{t('channels.dialogs.settings.overrides.body.description')}</CardDescription>
                  </CardHeader>
                  <CardContent className='min-h-0 flex-1 space-y-3 overflow-y-auto'>
                    {bodyFields.map((field, index) => (
                      <OperationRow
                        key={field.id}
                        index={index}
                        control={form.control}
                        fieldName='bodyOverrideOperations'
                        onUpdate={updateBodyOperation}
                        onRemove={removeBodyOperation}
                      />
                    ))}

                    <Button type='button' variant='outline' onClick={addBodyOperation} className='w-full' data-testid='add-param-button'>
                      {t('channels.dialogs.settings.overrides.addButton')}
                    </Button>

                    {form.formState.errors.bodyOverrideOperations?.message && (
                      <p className='text-destructive text-sm'>{t(form.formState.errors.bodyOverrideOperations.message.toString())}</p>
                    )}
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          </div>

          <DialogFooter className='mt-6 shrink-0 border-t px-6 py-4'>
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

      <ChannelOverrideTemplateEditDialog
        open={editingTemplate !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) {
            setEditingTemplate(null);
          }
        }}
        template={editingTemplate}
      />
    </Dialog>
  );
}
