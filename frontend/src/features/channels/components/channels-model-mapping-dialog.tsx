import { useEffect, useMemo, useState, useCallback } from 'react';
import type { KeyboardEvent } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus, X, Lightbulb } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useUpdateChannel } from '../data/channels';
import { Channel, ModelMapping } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

// 扩展 schema 以包含模型映射的校验规则
const createModelMappingFormSchema = (supportedModels: string[]) =>
  z.object({
    extraModelPrefix: z.string().optional(),
    modelMappings: z
      .array(
        z.object({
          from: z.string().min(1, 'Original model is required'),
          to: z.string().min(1, 'Target model is required'),
        })
      )
      .refine(
        (mappings) => {
          // 检查是否所有 from 字段都是唯一的
          const fromValues = mappings.map((m) => m.from);
          return new Set(fromValues).size === fromValues.length;
        },
        {
          message: 'Each original model can only be mapped once',
        }
      )
      .refine(
        (mappings) => {
          // 检查所有目标模型是否在支持的模型列表中
          return mappings.every((m) => supportedModels.includes(m.to));
        },
        {
          message: 'Target model must be in supported models',
        }
      ),
    autoTrimedModelPrefixes: z.array(z.string()).optional(),
    hideOriginalModels: z.boolean().optional(),
  });

const extractAliasFromModelPath = (modelPath: string): string => {
  if (!modelPath) {
    return '';
  }
  const segments = modelPath.split('/');
  return segments[segments.length - 1]?.trim() ?? '';
};

const extractAllPrefixes = (models: string[]): string[] => {
  if (!models || models.length === 0) {
    return [];
  }

  const prefixes = new Set<string>();
  models.forEach((model) => {
    const segments = model.split('/');
    for (let i = 1; i < segments.length; i++) {
      const prefix = segments.slice(0, i).join('/');
      if (prefix) {
        prefixes.add(prefix);
      }
    }
  });

  return Array.from(prefixes).sort();
};

export function ChannelsModelMappingDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const [modelMappings, setModelMappings] = useState<ModelMapping[]>(currentRow.settings?.modelMappings || []);
  const [newMapping, setNewMapping] = useState({ from: '', to: '' });
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editingDraft, setEditingDraft] = useState<ModelMapping | null>(null);
  const [editingError, setEditingError] = useState<string | null>(null);

  const prefixSuggestions = useMemo(() => extractAllPrefixes(currentRow.supportedModels), [currentRow.supportedModels]);

  const modelMappingFormSchema = createModelMappingFormSchema(currentRow.supportedModels);

  const form = useForm<z.infer<typeof modelMappingFormSchema>>({
    resolver: zodResolver(modelMappingFormSchema),
    defaultValues: {
      extraModelPrefix: currentRow.settings?.extraModelPrefix || '',
      modelMappings: currentRow.settings?.modelMappings || [],
      autoTrimedModelPrefixes: currentRow.settings?.autoTrimedModelPrefixes || [],
      hideOriginalModels: currentRow.settings?.hideOriginalModels || false,
    },
  });

  const handleAutoExtractAllPrefixes = useCallback(() => {
    if (prefixSuggestions.length === 0) {
      toast.warning(
        t('channels.dialogs.settings.autoTrimedModelPrefixes.noPrefixesFound', {
          defaultValue: 'No prefixes found in supported models',
        })
      );
      return;
    }

    const currentPrefixes = form.getValues('autoTrimedModelPrefixes') || [];
    const currentPrefixesSet = new Set(currentPrefixes);
    const newPrefixes = prefixSuggestions.filter((prefix) => !currentPrefixesSet.has(prefix));

    if (newPrefixes.length === 0) {
      toast.warning(
        t('channels.dialogs.settings.autoTrimedModelPrefixes.allPrefixesAlreadyAdded', {
          defaultValue: 'All prefixes have already been added',
        })
      );
      return;
    }

    form.setValue('autoTrimedModelPrefixes', [...currentPrefixes, ...newPrefixes]);
    toast.success(
      t('channels.dialogs.settings.autoTrimedModelPrefixes.prefixesAdded', {
        count: newPrefixes.length,
        defaultValue: `${newPrefixes.length} prefix(es) added successfully`,
      })
    );
  }, [form, prefixSuggestions, t]);

  const handleClearAllPrefixes = useCallback(() => {
    const currentPrefixes = form.getValues('autoTrimedModelPrefixes') || [];
    if (currentPrefixes.length === 0) {
      toast.warning(
        t('channels.dialogs.settings.autoTrimedModelPrefixes.noPrefixesToClear', {
          defaultValue: 'No prefixes to clear',
        })
      );
      return;
    }

    form.setValue('autoTrimedModelPrefixes', []);
    toast.success(
      t('channels.dialogs.settings.autoTrimedModelPrefixes.prefixesCleared', {
        count: currentPrefixes.length,
        defaultValue: `${currentPrefixes.length} prefix(es) cleared successfully`,
      })
    );
  }, [form, t]);

  const exitInlineEditing = () => {
    setEditingIndex(null);
    setEditingDraft(null);
    setEditingError(null);
  };

  const sanitizeMapping = (mapping: ModelMapping): ModelMapping => ({
    from: mapping.from.trim(),
    to: mapping.to.trim(),
  });

  const validateMappingDraft = (draft: ModelMapping, skipIndex?: number): string | null => {
    const normalized = sanitizeMapping(draft);
    if (!normalized.from || !normalized.to) {
      return t('channels.dialogs.settings.modelMapping.validationRequired', {
        defaultValue: 'Both alias and target model are required',
      });
    }
    if (!currentRow.supportedModels.includes(normalized.to)) {
      return t('channels.dialogs.settings.modelMapping.targetInvalid', {
        defaultValue: 'Target model must be in supported models',
      });
    }
    const isDuplicateFrom = modelMappings.some((mapping, idx) => idx !== skipIndex && mapping.from === normalized.from);
    if (isDuplicateFrom) {
      return t('channels.dialogs.settings.modelMapping.duplicateAlias', {
        defaultValue: 'Each original model can only be mapped once',
      });
    }
    return null;
  };

  useEffect(() => {
    const nextExtraModelPrefix = currentRow.settings?.extraModelPrefix || '';
    const nextMappings = currentRow.settings?.modelMappings || [];
    setModelMappings(nextMappings);
    setNewMapping({ from: '', to: '' });
    form.reset({
      extraModelPrefix: nextExtraModelPrefix,
      modelMappings: nextMappings,
      autoTrimedModelPrefixes: currentRow.settings?.autoTrimedModelPrefixes || [],
      hideOriginalModels: currentRow.settings?.hideOriginalModels || false,
    });
    exitInlineEditing();
  }, [currentRow, open, form]);

  const aliasSuggestion = useMemo(() => {
    const modelPath = newMapping.to;
    if (!modelPath) {
      return '';
    }

    return extractAliasFromModelPath(modelPath);
  }, [newMapping.to]);

  const applyAliasSuggestion = () => {
    if (!aliasSuggestion) {
      return;
    }
    setNewMapping((prev) => ({ ...prev, from: aliasSuggestion }));
  };

  const startEditing = (index: number) => {
    setEditingIndex(index);
    setEditingDraft(modelMappings[index]);
    setEditingError(null);
  };

  const handleInlineEditFieldChange = (key: keyof ModelMapping, value: string) => {
    if (!editingDraft) {
      return;
    }
    setEditingDraft({
      ...editingDraft,
      [key]: value,
    });
    // Clear error when user makes changes
    setEditingError(null);
  };

  const saveEditingDraft = () => {
    if (editingIndex === null || !editingDraft) {
      return;
    }
    const validationError = validateMappingDraft(editingDraft, editingIndex);
    if (validationError) {
      setEditingError(validationError);
      return;
    }
    const sanitizedDraft = sanitizeMapping(editingDraft);
    const updatedMappings = modelMappings.map((mapping, idx) => (idx === editingIndex ? sanitizedDraft : mapping));
    setModelMappings(updatedMappings);
    form.setValue('modelMappings', updatedMappings, {
      shouldValidate: true,
      shouldDirty: true,
    });
    exitInlineEditing();
  };

  const handleInlineEditKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      saveEditingDraft();
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      exitInlineEditing();
    }
  };

  const onSubmit = async (values: z.infer<typeof modelMappingFormSchema>) => {
    // 检查是否有未添加的映射
    if (newMapping.from.trim() || newMapping.to.trim()) {
      toast.warning(t('channels.messages.pendingMappingWarning'));
      return;
    }

    try {
      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        extraModelPrefix: values.extraModelPrefix,
        modelMappings: values.modelMappings,
        autoTrimedModelPrefixes: values.autoTrimedModelPrefixes || [],
        hideOriginalModels: values.hideOriginalModels,
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

  const addMapping = () => {
    if (!newMapping.from.trim() || !newMapping.to.trim()) {
      return;
    }

    const sanitizedMapping = sanitizeMapping(newMapping);
    const updatedMappings = [...modelMappings, sanitizedMapping];
    setModelMappings(updatedMappings);
    form.setValue('modelMappings', updatedMappings, {
      shouldValidate: true,
      shouldDirty: true,
    });
    setNewMapping({ from: '', to: '' });
  };

  const removeMapping = (index: number) => {
    const updatedMappings = modelMappings.filter((_, i) => i !== index);
    setModelMappings(updatedMappings);
    form.setValue('modelMappings', updatedMappings);
    if (editingIndex !== null) {
      if (editingIndex === index) {
        exitInlineEditing();
      } else if (editingIndex > index) {
        setEditingIndex(editingIndex - 1);
      }
    }
  };

  return (
    <TooltipProvider>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[800px]'>
          <DialogHeader>
            <DialogTitle>{t('channels.dialogs.settings.modelMapping.title')}</DialogTitle>
            <DialogDescription>{t('channels.dialogs.settings.modelMapping.description', { name: currentRow.name })}</DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)}>
              <div className='space-y-6'>
              <Card>
                <CardHeader>
                  <CardTitle className='text-lg'>{t('channels.dialogs.settings.modelMapping.hideOriginalModels.label')}</CardTitle>
                  <CardDescription>{t('channels.dialogs.settings.modelMapping.hideOriginalModels.description')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className='flex items-start gap-3'>
                    <Checkbox
                      id='hideOriginalModels'
                      checked={form.watch('hideOriginalModels') || false}
                      onCheckedChange={(checked) => form.setValue('hideOriginalModels', checked === true)}
                    />
                    <label
                      htmlFor='hideOriginalModels'
                      className='cursor-pointer text-sm leading-none font-medium peer-disabled:cursor-not-allowed peer-disabled:opacity-70'
                    >
                      {t('channels.dialogs.settings.modelMapping.hideOriginalModels.label')}
                    </label>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className='text-lg'>{t('channels.dialogs.settings.extraModelPrefix.title')}</CardTitle>
                  <CardDescription>{t('channels.dialogs.settings.extraModelPrefix.description')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <Input
                    placeholder={t('channels.dialogs.settings.extraModelPrefix.placeholder')}
                    value={form.watch('extraModelPrefix') || ''}
                    onChange={(e) => form.setValue('extraModelPrefix', e.target.value)}
                  />
                  {form.formState.errors.extraModelPrefix?.message && (
                    <p className='text-destructive mt-2 text-sm'>{form.formState.errors.extraModelPrefix.message.toString()}</p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className='text-lg'>{t('channels.dialogs.settings.autoTrimedModelPrefixes.title')}</CardTitle>
                  <CardDescription>{t('channels.dialogs.settings.autoTrimedModelPrefixes.description')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className='space-y-2'>
                    {/* 添加新前缀 */}
                    <FormField
                      control={form.control}
                      name='autoTrimedModelPrefixes'
                      render={({ field }) => (
                        <FormItem>
                          <FormControl>
                            <TagsAutocompleteInput
                              value={field.value || []}
                              onChange={field.onChange}
                              placeholder={t('channels.dialogs.settings.autoTrimedModelPrefixes.placeholder')}
                              suggestions={prefixSuggestions}
                              className='h-auto min-h-9 py-1'
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />

                    {/* 自动提取所有前缀 */}
                    {prefixSuggestions.length > 0 && (
                      <div className='flex items-center gap-2 pt-2'>
                        <Button type='button' variant='outline' size='sm' onClick={handleAutoExtractAllPrefixes} className='text-xs'>
                          {t('channels.dialogs.settings.autoTrimedModelPrefixes.autoExtractAll', {
                            defaultValue: 'Auto-extract all prefixes',
                          })}
                        </Button>
                        <Button type='button' variant='outline' size='sm' onClick={handleClearAllPrefixes} className='text-xs'>
                          {t('channels.dialogs.settings.autoTrimedModelPrefixes.clearAll', {
                            defaultValue: 'Clear all',
                          })}
                        </Button>
                        <span className='text-muted-foreground text-xs'>
                          {t('channels.dialogs.settings.autoTrimedModelPrefixes.prefixesDetected', {
                            count: prefixSuggestions.length,
                            defaultValue: `Detected ${prefixSuggestions.length} prefix(es)`,
                          })}
                        </span>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className='text-lg'>{t('channels.dialogs.settings.modelMapping.title')}</CardTitle>
                  <CardDescription>{t('channels.dialogs.settings.modelMapping.description')}</CardDescription>
                </CardHeader>
                <CardContent className='space-y-4'>
                  <div className='flex gap-2'>
                    <div className='flex flex-1 gap-2'>
                      <Input
                        placeholder={t('channels.dialogs.settings.modelMapping.originalModel')}
                        value={newMapping.from}
                        onChange={(e) => setNewMapping({ ...newMapping, from: e.target.value })}
                        className='flex-1'
                      />
                      {aliasSuggestion && newMapping.to && (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={applyAliasSuggestion}
                              disabled={!aliasSuggestion || newMapping.from.trim() === aliasSuggestion}
                              aria-label={t('channels.dialogs.settings.modelMapping.useSuggestion', {
                                alias: aliasSuggestion,
                                defaultValue: `Use ${aliasSuggestion}`,
                              })}
                            >
                              <Lightbulb className='h-4 w-4' />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>
                              {t('channels.dialogs.settings.modelMapping.useSuggestion', {
                                alias: aliasSuggestion,
                                defaultValue: `Use ${aliasSuggestion}`,
                              })}
                            </p>
                          </TooltipContent>
                        </Tooltip>
                      )}
                    </div>
                    <span className='text-muted-foreground flex items-center'>→</span>
                    <Select value={newMapping.to} onValueChange={(value) => setNewMapping({ ...newMapping, to: value })}>
                      <SelectTrigger className='flex-1'>
                        <SelectValue placeholder={t('channels.dialogs.settings.modelMapping.targetModel')} />
                      </SelectTrigger>
                      <SelectContent>
                        {currentRow.supportedModels.map((model) => (
                          <SelectItem key={model} value={model}>
                            {model}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Button
                      type='button'
                      onClick={addMapping}
                      size='sm'
                      disabled={!newMapping.from.trim() || !newMapping.to.trim()}
                      data-testid='add-model-mapping-button'
                      aria-label={t('channels.dialogs.settings.modelMapping.addMappingButton', {
                        defaultValue: 'Add mapping',
                      })}
                    >
                      <Plus size={16} />
                    </Button>
                    {(newMapping.from.trim() || newMapping.to.trim()) && (
                      <Button type='button' variant='outline' size='sm' onClick={() => setNewMapping({ from: '', to: '' })}>
                        <X size={16} />
                      </Button>
                    )}
                  </div>

                  {/* 显示表单错误 */}
                  {form.formState.errors.modelMappings?.message && (
                    <p className='text-destructive text-sm'>{form.formState.errors.modelMappings.message.toString()}</p>
                  )}

                  <div className='space-y-2'>
                    {modelMappings.length === 0 ? (
                      <p className='text-muted-foreground py-4 text-center text-sm'>
                        {t('channels.dialogs.settings.modelMapping.noMappings')}
                      </p>
                    ) : (
                      modelMappings.map((mapping, index) => {
                        const isEditing = editingIndex === index;
                        return (
                          <div key={index} className='rounded-lg border p-3'>
                            {isEditing ? (
                              <div className='space-y-2'>
                                <div className='flex flex-wrap items-center gap-3' onKeyDown={handleInlineEditKeyDown}>
                                  <div className='flex flex-1 items-center gap-2'>
                                    <Input
                                      value={editingDraft?.from ?? ''}
                                      onChange={(e) => handleInlineEditFieldChange('from', e.target.value)}
                                      autoFocus
                                      className='flex-1'
                                    />
                                    <span className='text-muted-foreground'>→</span>
                                    <Select
                                      value={editingDraft?.to ?? undefined}
                                      onValueChange={(value) => handleInlineEditFieldChange('to', value)}
                                    >
                                      <SelectTrigger className='min-w-[180px] flex-1'>
                                        <SelectValue placeholder={t('channels.dialogs.settings.modelMapping.targetModel')} />
                                      </SelectTrigger>
                                      <SelectContent>
                                        {currentRow.supportedModels.map((model) => (
                                          <SelectItem key={model} value={model}>
                                            {model}
                                          </SelectItem>
                                        ))}
                                      </SelectContent>
                                    </Select>
                                  </div>
                                  <div className='flex gap-2'>
                                    <Button
                                      type='button'
                                      size='sm'
                                      onClick={saveEditingDraft}
                                      disabled={!editingDraft?.from.trim() || !editingDraft?.to.trim()}
                                    >
                                      {t('common.buttons.save')}
                                    </Button>
                                    <Button type='button' variant='ghost' size='sm' onClick={exitInlineEditing}>
                                      {t('common.buttons.cancel')}
                                    </Button>
                                  </div>
                                </div>
                                {editingError && <p className='text-destructive text-sm'>{editingError}</p>}
                              </div>
                            ) : (
                              <div className='flex items-center justify-between'>
                                <div
                                  className='focus-within:outline-ring flex flex-1 cursor-pointer items-center gap-2 rounded p-1 focus-within:outline focus-within:outline-2 focus-within:outline-offset-2'
                                  onDoubleClick={() => startEditing(index)}
                                  onKeyDown={(e) => {
                                    if (e.key === 'Enter' || e.key === ' ') {
                                      e.preventDefault();
                                      startEditing(index);
                                    }
                                  }}
                                  tabIndex={0}
                                  role='button'
                                  aria-label={t('channels.dialogs.settings.modelMapping.editHint', {
                                    defaultValue: 'Double-click to edit',
                                  })}
                                  title={t('channels.dialogs.settings.modelMapping.editHint', {
                                    defaultValue: 'Double-click to edit',
                                  })}
                                >
                                  <Badge variant='outline'>{mapping.from}</Badge>
                                  <span className='text-muted-foreground'>→</span>
                                  <Badge variant='outline'>{mapping.to}</Badge>
                                </div>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='sm'
                                  onClick={() => removeMapping(index)}
                                  className='text-destructive hover:text-destructive'
                                >
                                  <X size={16} />
                                </Button>
                              </div>
                            )}
                          </div>
                        );
                      })
                    )}
                  </div>
                </CardContent>
              </Card>
            </div>

              <DialogFooter className='mt-6'>
                <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
                  {t('common.buttons.cancel')}
                </Button>
                <Button type='submit' disabled={updateChannel.isPending}>
                  {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>
    </TooltipProvider>
  );
}
