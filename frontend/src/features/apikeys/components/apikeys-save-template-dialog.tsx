import { useEffect, useMemo, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useApiKeyProfileTemplates, useCreateApiKeyProfileTemplate, useUpdateApiKeyProfileTemplate } from '../data/apikeys';
import type { ApiKeyProfile } from '../data/schema';

interface ApiKeySaveTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profileData: ApiKeyProfile;
  projectID: string | null;
}

const formSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().trim().min(1, t('apikeys.templates.templateNameRequired')),
    description: z.string().optional(),
  });

type FormValues = z.infer<ReturnType<typeof formSchemaFactory>>;

export function ApiKeySaveTemplateDialog({ open, onOpenChange, profileData, projectID }: ApiKeySaveTemplateDialogProps) {
  const { t } = useTranslation();
  const createTemplate = useCreateApiKeyProfileTemplate();
  const updateTemplate = useUpdateApiKeyProfileTemplate();
  const { data: existingTemplates } = useApiKeyProfileTemplates(projectID);
  const [overwriteValues, setOverwriteValues] = useState<FormValues | null>(null);

  const formSchema = useMemo(() => formSchemaFactory(t), [t]);

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: '',
      description: '',
    },
  });

  const templateName = form.watch('name');
  const matchingTemplate = useMemo(
    () => existingTemplates?.find((template) => template.name.trim().toLowerCase() === templateName?.trim().toLowerCase()),
    [existingTemplates, templateName]
  );

  useEffect(() => {
    if (open) {
      setOverwriteValues(null);
      form.reset({
        name: profileData.name || '',
        description: '',
      });
    }
  }, [open, profileData.name, form]);

  useEffect(() => {
    if (matchingTemplate && !form.formState.dirtyFields.description) {
      form.setValue('description', matchingTemplate.description ?? '');
    }
  }, [matchingTemplate, form]);

  const saveNewTemplate = async (values: FormValues) => {
    await createTemplate.mutateAsync({
      name: values.name.trim(),
      description: values.description || '',
      projectID,
      profile: profileData,
    });
    toast.success(t('apikeys.templates.successMessage'));
    onOpenChange(false);
  };

  const overwriteTemplate = async () => {
    if (!matchingTemplate || !overwriteValues) return;

    try {
      await updateTemplate.mutateAsync({
        id: matchingTemplate.id,
        input: {
          name: overwriteValues.name.trim(),
          description: overwriteValues.description || '',
          profile: {
            ...profileData,
            name: overwriteValues.name.trim(),
          },
        },
      });
      toast.success(t('apikeys.templates.overwriteSuccessMessage', { name: overwriteValues.name.trim() }));
      setOverwriteValues(null);
      onOpenChange(false);
    } catch {
      toast.error(t('apikeys.templates.overwriteErrorMessage'));
    }
  };

  const handleSubmit = async (values: FormValues) => {
    if (matchingTemplate) {
      setOverwriteValues(values);
      return;
    }

    try {
      await saveNewTemplate(values);
    } catch {
      toast.error(t('apikeys.templates.errorMessage'));
    }
  };

  const isSubmitting = createTemplate.isPending || updateTemplate.isPending;

  return (
    <>
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
                    {matchingTemplate && (
                      <p className='text-sm text-amber-600'>{t('apikeys.templates.overwriteHint', { name: matchingTemplate.name })}</p>
                    )}
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
                      <Textarea {...field} value={field.value ?? ''} placeholder={t('apikeys.templates.descriptionPlaceholder')} rows={3} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <DialogFooter className='gap-2 pt-2'>
                <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={isSubmitting}>
                  {t('common.buttons.cancel')}
                </Button>
                <Button type='submit' variant={matchingTemplate ? 'destructive' : 'default'} disabled={isSubmitting}>
                  {isSubmitting
                    ? t('common.buttons.saving')
                    : matchingTemplate
                      ? t('apikeys.templates.overwriteButton')
                      : t('apikeys.templates.saveButton')}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={overwriteValues !== null} onOpenChange={(isOpen) => !isOpen && setOverwriteValues(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('apikeys.templates.overwriteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('apikeys.templates.overwriteConfirmDescription', {
                name: matchingTemplate?.name ?? '',
                count: matchingTemplate?.linkedProfilesCount ?? 0,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={updateTemplate.isPending}>{t('common.buttons.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void overwriteTemplate();
              }}
              disabled={updateTemplate.isPending}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {updateTemplate.isPending
                ? t('common.buttons.saving')
                : (matchingTemplate?.linkedProfilesCount ?? 0) > 0
                  ? t('apikeys.templates.overwriteAndSyncButton')
                  : t('apikeys.templates.overwriteButton')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
