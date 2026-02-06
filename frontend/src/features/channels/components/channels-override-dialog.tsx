import { useEffect, useState, useCallback } from 'react';
import { z } from 'zod';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2, Save, Download, ChevronDown, ChevronUp } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { useUpdateChannel } from '../data/channels';
import { Channel, OverrideOperation, overrideOperationSchema } from '../data/schema';
import { useChannelOverrideTemplates, useCreateChannelOverrideTemplate } from '../data/templates';
import { mergeChannelSettingsForUpdate, mergeOverrideHeaders, mergeOverrideParameters, normalizeOverrideParameters } from '../utils/merge';

type OpType = OverrideOperation['op'];

function parseOperations(raw: string): OverrideOperation[] {
  try {
    const parsed = JSON.parse(raw || '[]');
    if (Array.isArray(parsed)) {
      return parsed;
    }
    if (typeof parsed === 'object' && parsed !== null) {
      return Object.entries(parsed).map(([key, value]) =>
        value === '__AXONHUB_CLEAR__' ? { op: 'delete' as const, path: key } : { op: 'set' as const, path: key, value }
      );
    }
    return [];
  } catch {
    return [];
  }
}

function serializeOperations(ops: OverrideOperation[]): string {
  const cleaned = ops
    .filter((op) => {
      if (op.op === 'set' || op.op === 'delete') return op.path?.trim();
      if (op.op === 'rename' || op.op === 'copy') return op.from?.trim() && op.to?.trim();
      return false;
    })
    .map((op) => {
      const result: Record<string, any> = { op: op.op };
      if (op.op === 'set') {
        result.path = op.path;
        result.value = op.value;
      } else if (op.op === 'delete') {
        result.path = op.path;
      } else if (op.op === 'rename' || op.op === 'copy') {
        result.from = op.from;
        result.to = op.to;
      }
      if (op.condition?.trim()) {
        result.condition = op.condition;
      }
      return result;
    });
  return JSON.stringify(cleaned);
}

function parseValueForDisplay(value: any): string {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}

function parseValueForStorage(value: string): any {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  try {
    return JSON.parse(trimmed);
  } catch {
    return trimmed;
  }
}

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
  headerOverrideOperations: z.array(overrideOperationSchema).optional(),
  bodyOverrideOperations: z.array(overrideOperationSchema).optional(),
});

type OverrideFormValues = z.infer<typeof overrideFormSchema>;

const OP_LABELS: Record<OpType, string> = {
  set: 'channels.dialogs.settings.overrides.body.opSet',
  delete: 'channels.dialogs.settings.overrides.body.opDelete',
  rename: 'channels.dialogs.settings.overrides.body.opRename',
  copy: 'channels.dialogs.settings.overrides.body.opCopy',
};

interface OperationRowProps {
  index: number;
  field: OverrideOperation;
  onUpdate: (index: number, data: Partial<OverrideOperation>) => void;
  onRemove: (index: number) => void;
}

function OperationRow({ index, field, onUpdate, onRemove }: OperationRowProps) {
  const { t } = useTranslation();
  const [showCondition, setShowCondition] = useState(!!field.condition);

  const needsPathOnly = field.op === 'set' || field.op === 'delete';
  const needsFromTo = field.op === 'rename' || field.op === 'copy';
  const needsValue = field.op === 'set';

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-center gap-3'>
        <div className='w-36'>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.op')}</Label>
          <Select
            value={field.op}
            onValueChange={(v) => onUpdate(index, { op: v as OpType })}
          >
            <SelectTrigger data-testid={`op-type-${index}`} className='mt-1'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(OP_LABELS) as OpType[]).map((opType) => (
                <SelectItem key={opType} value={opType}>
                  {t(OP_LABELS[opType])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {needsPathOnly && (
          <div className='flex-1'>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.path')}</Label>
            <Input
              data-testid={`op-path-${index}`}
              className='mt-1 font-mono'
              placeholder={t('channels.dialogs.settings.overrides.body.pathPlaceholder')}
              value={field.path || ''}
              onChange={(e) => onUpdate(index, { path: e.target.value })}
            />
          </div>
        )}

        {needsFromTo && (
          <>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.from')}</Label>
              <Input
                data-testid={`op-from-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.body.fromPlaceholder')}
                value={field.from || ''}
                onChange={(e) => onUpdate(index, { from: e.target.value })}
              />
            </div>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.to')}</Label>
              <Input
                data-testid={`op-to-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.body.toPlaceholder')}
                value={field.to || ''}
                onChange={(e) => onUpdate(index, { to: e.target.value })}
              />
            </div>
          </>
        )}

        <div className='pt-5'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => onRemove(index)}
            className='px-3'
            data-testid={`remove-op-${index}`}
          >
            {t('common.buttons.remove')}
          </Button>
        </div>
      </div>

      {needsValue && (
        <div>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.value')}</Label>
          <Input
            data-testid={`op-value-${index}`}
            className='mt-1 font-mono'
            placeholder={t('channels.dialogs.settings.overrides.body.valuePlaceholder')}
            value={parseValueForDisplay(field.value)}
            onChange={(e) => onUpdate(index, { value: parseValueForStorage(e.target.value) })}
          />
        </div>
      )}

      <div>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='text-muted-foreground h-6 px-1 text-xs'
          onClick={() => setShowCondition(!showCondition)}
        >
          {showCondition ? <ChevronUp className='mr-1 h-3 w-3' /> : <ChevronDown className='mr-1 h-3 w-3' />}
          {t('channels.dialogs.settings.overrides.body.condition')}
        </Button>
        {showCondition && (
          <Input
            data-testid={`op-condition-${index}`}
            className='mt-1 font-mono text-sm'
            placeholder={t('channels.dialogs.settings.overrides.body.conditionPlaceholder')}
            value={field.condition || ''}
            onChange={(e) => onUpdate(index, { condition: e.target.value })}
          />
        )}
      </div>
    </div>
  );
}

interface HeaderOperationRowProps {
  index: number;
  field: OverrideOperation;
  onUpdate: (index: number, data: Partial<OverrideOperation>) => void;
  onRemove: (index: number) => void;
}

function HeaderOperationRow({ index, field, onUpdate, onRemove }: HeaderOperationRowProps) {
  const { t } = useTranslation();
  const [showCondition, setShowCondition] = useState(!!field.condition);

  const opType = field.op;
  const needsPathOnly = opType === 'set' || opType === 'delete';
  const needsFromTo = opType === 'rename' || opType === 'copy';
  const needsValue = opType === 'set';

  const normalizedKey = (field.path || '').trim().toLowerCase();
  const isAuthHeader = normalizedKey !== '' && AUTH_HEADER_KEYS.includes(normalizedKey);

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-center gap-3'>
        <div className='w-36'>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.op')}</Label>
          <Select
            value={opType}
            onValueChange={(v) => onUpdate(index, { op: v as OpType })}
          >
            <SelectTrigger data-testid={`header-op-type-${index}`} className='mt-1'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(OP_LABELS) as OpType[]).map((opType) => (
                <SelectItem key={opType} value={opType}>
                  {t(OP_LABELS[opType])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {needsPathOnly && (
          <div className='flex-1'>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.key')}</Label>
            <Input
              data-testid={`header-op-path-${index}`}
              className='mt-1 font-mono'
              placeholder={t('channels.dialogs.settings.overrides.headers.keyPlaceholder')}
              value={field.path || ''}
              onChange={(e) => onUpdate(index, { path: e.target.value })}
            />
            {isAuthHeader && (
              <p className='text-destructive mt-1 text-sm' role='alert'>
                {t('channels.dialogs.settings.overrides.headers.sensitiveWarning')}
              </p>
            )}
          </div>
        )}

        {needsFromTo && (
          <>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.from')}</Label>
              <Input
                data-testid={`header-op-from-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.headers.fromPlaceholder')}
                value={field.from || ''}
                onChange={(e) => onUpdate(index, { from: e.target.value })}
              />
            </div>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.to')}</Label>
              <Input
                data-testid={`header-op-to-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.headers.toPlaceholder')}
                value={field.to || ''}
                onChange={(e) => onUpdate(index, { to: e.target.value })}
              />
            </div>
          </>
        )}

        <div className='pt-5'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => onRemove(index)}
            className='px-3'
            data-testid={`remove-header-op-${index}`}
          >
            {t('common.buttons.remove')}
          </Button>
        </div>
      </div>

      {needsValue && (
        <div>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.value')}</Label>
          <Input
            data-testid={`header-op-value-${index}`}
            className='mt-1 font-mono'
            placeholder={t('channels.dialogs.settings.overrides.headers.valuePlaceholder')}
            value={parseValueForDisplay(field.value)}
            onChange={(e) => onUpdate(index, { value: e.target.value })}
          />
        </div>
      )}

      <div>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='text-muted-foreground h-6 px-1 text-xs'
          onClick={() => setShowCondition(!showCondition)}
        >
          {showCondition ? <ChevronUp className='mr-1 h-3 w-3' /> : <ChevronDown className='mr-1 h-3 w-3' />}
          {t('channels.dialogs.settings.overrides.body.condition')}
        </Button>
        {showCondition && (
          <Input
            data-testid={`header-op-condition-${index}`}
            className='mt-1 font-mono text-sm'
            placeholder={t('channels.dialogs.settings.overrides.body.conditionPlaceholder')}
            value={field.condition || ''}
            onChange={(e) => onUpdate(index, { condition: e.target.value })}
          />
        )}
      </div>
    </div>
  );
}

export function ChannelsOverrideDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const createTemplate = useCreateChannelOverrideTemplate();

  const [showSaveTemplateDialog, setShowSaveTemplateDialog] = useState(false);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [templateSearchOpen, setTemplateSearchOpen] = useState(false);
  const [templateSearchValue, setTemplateSearchValue] = useState('');
  const [isApplyingTemplate, setIsApplyingTemplate] = useState(false);

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
    update: updateHeader,
    replace: replaceHeaders,
  } = useFieldArray({
    control: form.control,
    name: 'headerOverrideOperations',
  });

  const {
    fields: bodyFields,
    append: appendBody,
    remove: removeBody,
    update: updateBody,
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
      const currentField = headerFields[index];
      updateHeader(index, { ...currentField, ...data });
    },
    [headerFields, updateHeader]
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
      const currentField = bodyFields[index];
      updateBody(index, { ...currentField, ...data });
    },
    [bodyFields, updateBody]
  );

  const onSubmit = async (data: OverrideFormValues) => {
    const bodyOps = data.bodyOverrideOperations || [];
    const headerOps = data.headerOverrideOperations || [];

    // Validate body operations
    for (let i = 0; i < bodyOps.length; i++) {
      const op = bodyOps[i];
      if (op.op === 'set' || op.op === 'delete') {
        if (!op.path?.trim()) {
          toast.error(t('channels.dialogs.settings.overrides.validation.emptyPath', { index: i + 1, op: op.op }));
          return;
        }
      }
      if (op.op === 'rename' || op.op === 'copy') {
        if (!op.from?.trim() || !op.to?.trim()) {
          toast.error(t('channels.dialogs.settings.overrides.validation.emptyFromTo', { index: i + 1, op: op.op }));
          return;
        }
      }
    }

    // Validate header operations
    for (let i = 0; i < headerOps.length; i++) {
      const op = headerOps[i];
      if (op.op === 'set' || op.op === 'delete') {
        if (!op.path?.trim()) {
          toast.error(t('channels.dialogs.settings.overrides.validation.emptyHeaderPath', { index: i + 1, op: op.op }));
          return;
        }
      }
      if (op.op === 'rename' || op.op === 'copy') {
        if (!op.from?.trim() || !op.to?.trim()) {
          toast.error(t('channels.dialogs.settings.overrides.validation.emptyHeaderFromTo', { index: i + 1, op: op.op }));
          return;
        }
      }
    }

    try {
      const validHeaderOps = headerOps.filter((h) => {
        if (h.op === 'set' || h.op === 'delete') return h.path?.trim();
        if (h.op === 'rename' || h.op === 'copy') return h.from?.trim() && h.to?.trim();
        return false;
      });
      const validBodyOps = bodyOps.filter((b) => {
        if (b.op === 'set' || b.op === 'delete') return b.path?.trim();
        if (b.op === 'rename' || b.op === 'copy') return b.from?.trim() && b.to?.trim();
        return false;
      });
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
      toast.error(t('channels.messages.updateError'));
    }
  };

  const handleApplyTemplate = useCallback(
    async (templateId?: string) => {
      const id = templateId || selectedTemplateId;
      if (!id) return;

      const template = templates.find((t) => t.id === id);
      if (!template) return;

      setIsApplyingTemplate(true);
      try {
        const templateHeaders = template.overrideHeaders || [];
        const templateParams = template.overrideParameters || '';

        const currentHeaders = form.getValues('headerOverrideOperations') || [];
        const currentBodyOps = form.getValues('bodyOverrideOperations') || [];

        const mergedHeaders = mergeOverrideHeaders(currentHeaders, templateHeaders);
        const mergedParams = mergeOverrideParameters(serializeOperations(currentBodyOps), templateParams);

        replaceHeaders(mergedHeaders);
        replaceBodies(parseOperations(mergedParams));

        toast.success(t('channels.templates.messages.applied'));
      } catch (error) {
        toast.error(t('channels.templates.messages.applyError'));
      } finally {
        setIsApplyingTemplate(false);
      }
    },
    [selectedTemplateId, templates, form, replaceHeaders, replaceBodies, t]
  );

  const handleSaveAsTemplate = useCallback(
    async (name: string, description?: string) => {
      const headerOps = form.getValues('headerOverrideOperations') || [];
      const bodyOps = form.getValues('bodyOverrideOperations') || [];

      const validHeaderOps = headerOps.filter((h) => {
        if (h.op === 'set' || h.op === 'delete') return h.path?.trim();
        if (h.op === 'rename' || h.op === 'copy') return h.from?.trim() && h.to?.trim();
        return false;
      });
      try {
        await createTemplate.mutateAsync({
          name,
          description,
          channelType: currentRow.type,
          overrideParameters: normalizeOverrideParameters(serializeOperations(bodyOps)),
          overrideHeaders: validHeaderOps,
        });
        setShowSaveTemplateDialog(false);
      } catch (error) {
        // Error already handled by mutation
      }
    },
    [form, currentRow.type, createTemplate]
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
                          <div className='flex flex-col'>
                            <span className='font-medium'>{template.name}</span>
                            {template.description && (
                              <span className='text-muted-foreground line-clamp-1 text-xs'>{template.description}</span>
                            )}
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
                        field={field}
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
                        field={field}
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
    </Dialog>
  );
}
