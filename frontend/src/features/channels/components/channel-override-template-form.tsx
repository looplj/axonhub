// Shared editor for channel override templates: name, description, and the
// header/body override operations. Used by the standalone template manager and
// by the inline edit dialog opened from the override dialog template picker.
import { useCallback, useState } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { OverrideOperation } from '../data/schema';
import {
  OverrideFormValues,
  isValidBodyOp,
  isValidHeaderOp,
  overrideFormSchema,
  validateOverrideOperations,
} from '../utils/override-operations';
import { HeaderOperationRow, OperationRow } from './override-operation-rows';

export interface ChannelOverrideTemplateFormValue {
  name: string;
  description: string;
  headerOverrideOperations: OverrideOperation[];
  bodyOverrideOperations: OverrideOperation[];
}

interface Props {
  initialValue?: Partial<ChannelOverrideTemplateFormValue>;
  isSaving: boolean;
  submitLabel: string;
  onSubmit: (value: ChannelOverrideTemplateFormValue) => void;
}

export function ChannelOverrideTemplateForm({ initialValue, isSaving, submitLabel, onSubmit }: Props) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialValue?.name ?? '');
  const [description, setDescription] = useState(initialValue?.description ?? '');

  const form = useForm<OverrideFormValues>({
    resolver: zodResolver(overrideFormSchema),
    defaultValues: {
      headerOverrideOperations: initialValue?.headerOverrideOperations || [],
      bodyOverrideOperations: initialValue?.bodyOverrideOperations || [],
    },
  });

  const {
    fields: headerFields,
    append: appendHeader,
    remove: removeHeader,
  } = useFieldArray({
    control: form.control,
    name: 'headerOverrideOperations',
  });

  const {
    fields: bodyFields,
    append: appendBody,
    remove: removeBody,
  } = useFieldArray({
    control: form.control,
    name: 'bodyOverrideOperations',
  });

  const addHeaderOp = useCallback(() => {
    appendHeader({ op: 'set', path: '', value: '' });
  }, [appendHeader]);

  const addBodyOp = useCallback(() => {
    appendBody({ op: 'set', path: '', value: '' });
  }, [appendBody]);

  const updateHeaderOp = useCallback(
    (index: number, data: Partial<OverrideOperation>) => {
      Object.entries(data).forEach(([key, value]) => {
        form.setValue(`headerOverrideOperations.${index}.${key}` as any, value);
      });
    },
    [form]
  );

  const updateBodyOp = useCallback(
    (index: number, data: Partial<OverrideOperation>) => {
      Object.entries(data).forEach(([key, value]) => {
        form.setValue(`bodyOverrideOperations.${index}.${key}` as any, value);
      });
    },
    [form]
  );

  const handleSubmit = useCallback(() => {
    const trimmedName = name.trim();
    if (!trimmedName) {
      toast.error(t('channels.templates.validation.nameRequired'));
      return;
    }

    const headerOps = form.getValues('headerOverrideOperations') || [];
    const bodyOps = form.getValues('bodyOverrideOperations') || [];

    const validationError = validateOverrideOperations(t, headerOps, bodyOps);
    if (validationError) {
      toast.error(validationError);
      return;
    }

    onSubmit({
      name: trimmedName,
      description: description.trim(),
      headerOverrideOperations: headerOps.filter(isValidHeaderOp),
      bodyOverrideOperations: bodyOps.filter(isValidBodyOp),
    });
  }, [name, description, form, onSubmit, t]);

  return (
    <div className='flex min-h-0 flex-1 flex-col space-y-4'>
      <div className='grid shrink-0 gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor='template-form-name'>{t('channels.templates.fields.name')}</Label>
          <Input
            id='template-form-name'
            data-testid='template-form-name'
            placeholder={t('channels.templates.fields.namePlaceholder')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={isSaving}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='template-form-description'>{t('channels.templates.fields.description')}</Label>
          <Textarea
            id='template-form-description'
            data-testid='template-form-description'
            className='min-h-[38px] resize-y'
            rows={1}
            placeholder={t('channels.templates.fields.descriptionPlaceholder')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={isSaving}
          />
        </div>
      </div>

      <Tabs defaultValue='headers' className='flex min-h-0 flex-1 flex-col overflow-hidden'>
        <TabsList className='grid w-full shrink-0 grid-cols-2'>
          <TabsTrigger value='headers'>{t('channels.dialogs.settings.overrides.headers.title')}</TabsTrigger>
          <TabsTrigger value='body'>{t('channels.dialogs.settings.overrides.body.title')}</TabsTrigger>
        </TabsList>

        <TabsContent value='headers' className='mt-4 flex min-h-0 flex-1 flex-col overflow-hidden'>
          <Card className='flex min-h-0 flex-1 flex-col overflow-hidden'>
            <CardHeader className='shrink-0 pb-3'>
              <CardTitle className='text-base'>{t('channels.dialogs.settings.overrides.headers.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.settings.overrides.headers.description')}</CardDescription>
            </CardHeader>
            <CardContent className='min-h-0 flex-1 space-y-3 overflow-y-auto'>
              {headerFields.map((field, index) => (
                <HeaderOperationRow
                  key={field.id}
                  index={index}
                  control={form.control}
                  onUpdate={updateHeaderOp}
                  onRemove={removeHeader}
                />
              ))}

              <Button
                type='button'
                variant='outline'
                onClick={addHeaderOp}
                className='w-full'
                data-testid='template-add-header-op'
              >
                {t('channels.dialogs.settings.overrides.addButton')}
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value='body' className='mt-4 flex min-h-0 flex-1 flex-col overflow-hidden'>
          <Card className='flex min-h-0 flex-1 flex-col overflow-hidden'>
            <CardHeader className='shrink-0 pb-3'>
              <CardTitle className='text-base'>{t('channels.dialogs.settings.overrides.body.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.settings.overrides.body.description')}</CardDescription>
            </CardHeader>
            <CardContent className='min-h-0 flex-1 space-y-3 overflow-y-auto'>
              {bodyFields.map((field, index) => (
                <OperationRow
                  key={field.id}
                  index={index}
                  control={form.control}
                  fieldName='bodyOverrideOperations'
                  onUpdate={updateBodyOp}
                  onRemove={removeBody}
                />
              ))}

              <Button
                type='button'
                variant='outline'
                onClick={addBodyOp}
                className='w-full'
                data-testid='template-add-body-op'
              >
                {t('channels.dialogs.settings.overrides.addButton')}
              </Button>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <div className='flex shrink-0 justify-end gap-2 border-t pt-4'>
        <Button type='button' onClick={handleSubmit} disabled={isSaving} data-testid='template-form-submit'>
          {isSaving ? (
            <>
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              {t('common.buttons.saving')}
            </>
          ) : (
            submitLabel
          )}
        </Button>
      </div>
    </div>
  );
}
