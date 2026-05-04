import { useEffect, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useApiKeyProfileTemplates, useCreateApiKeyProfileTemplate } from '../data/apikeys';
import type { ApiKeyProfile } from '../data/schema';

interface ApiKeySaveTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profileData: ApiKeyProfile;
  projectID: string | null;
}

const formSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('apikeys.templates.templateNameRequired')),
    description: z.string().optional(),
  });

type FormValues = z.infer<ReturnType<typeof formSchemaFactory>>;

export function ApiKeySaveTemplateDialog({ open, onOpenChange, profileData, projectID }: ApiKeySaveTemplateDialogProps) {
  const { t } = useTranslation();
  const createTemplate = useCreateApiKeyProfileTemplate();
  const { data: existingTemplates } = useApiKeyProfileTemplates(projectID);

  const formSchema = useMemo(() => formSchemaFactory(t), [t]);

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: '',
      description: '',
    },
  });

  const templateName = form.watch('name');

  useEffect(() => {
    if (open) {
      form.reset({
        name: profileData.name || '',
        description: '',
      });
    }
  }, [open, profileData.name, form]);

  useEffect(() => {
    if (!templateName?.trim()) {
      form.clearErrors('name');
      return;
    }

    const isDuplicate = existingTemplates?.some(
      (template) => template.name.trim().toLowerCase() === templateName.trim().toLowerCase()
    );

    if (isDuplicate) {
      form.setError('name', {
        type: 'manual',
        message: t('apikeys.templates.templateNameDuplicate'),
      });
    } else {
      form.clearErrors('name');
    }
  }, [templateName, existingTemplates, form, t]);

  const handleSubmit = async (values: FormValues) => {
    try {
      await createTemplate.mutateAsync({
        name: values.name,
        description: values.description || '',
        projectID,
        profile: profileData,
      });
      toast.success(t('apikeys.templates.successMessage'));
      onOpenChange(false);
    } catch {
      toast.error(t('apikeys.templates.errorMessage'));
    }
  };

  const isSubmitting = createTemplate.isPending;
  const hasDuplicateError = form.formState.errors.name?.type === 'manual';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('apikeys.templates.saveTitle')}</DialogTitle>
          <DialogDescription>{t('apikeys.templates.saveDescription')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(handleSubmit)} className='space-y-4'>
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
                      rows={3}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter className='gap-2 pt-2'>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={isSubmitting}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={isSubmitting || hasDuplicateError}>
                {isSubmitting ? t('common.buttons.saving') : t('apikeys.templates.saveButton')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
