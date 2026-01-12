import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useApiKeysContext } from '../context/apikeys-context';
import { useCreateApiKey } from '../data/apikeys';
import { CreateApiKeyInput, createApiKeyInputSchema } from '../data/schema';
import { ScopesSelect } from '@/components/scopes-select';

export function ApiKeysCreateDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, openDialog, setSelectedApiKey } = useApiKeysContext();
  const createApiKey = useCreateApiKey();
  const [isSubmitting, setIsSubmitting] = useState(false);

  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  const form = useForm<CreateApiKeyInput>({
    resolver: zodResolver(createApiKeyInputSchema),
    defaultValues: {
      name: '',
      type: 'user',
      scopes: undefined, // Don't set scopes for user type
    },
  });

  const apiKeyType = form.watch('type');

  const onSubmit = async (data: CreateApiKeyInput) => {
    setIsSubmitting(true);
    try {
      // Remove scopes if type is user (use backend default)
      const submitData = data.type === 'user' ? { ...data, scopes: undefined } : data;
      const result = await createApiKey.mutateAsync(submitData);
      form.reset();
      closeDialog('create');
      // Open view dialog with the created API key
      setSelectedApiKey(result.createAPIKey);
      openDialog('view', result.createAPIKey);
    } catch (error) {
      // Error is handled by the mutation
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleClose = () => {
    form.reset();
    closeDialog('create');
  };

  return (
    <Dialog open={isDialogOpen.create} onOpenChange={handleClose}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-[600px]' ref={setDialogContent}>
        <DialogHeader>
          <DialogTitle>{t('apikeys.dialogs.create.title')}</DialogTitle>
          <DialogDescription>{t('apikeys.dialogs.create.description')}</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('apikeys.dialogs.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('apikeys.dialogs.fields.name.placeholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='type'
              render={({ field }) => (
                <FormItem className='space-y-3'>
                  <FormLabel>{t('apikeys.dialogs.fields.type.label')}</FormLabel>
                  <FormControl>
                    <RadioGroup onValueChange={field.onChange} defaultValue={field.value} className='flex flex-col space-y-1'>
                      <FormItem className='flex items-center space-y-0 space-x-3'>
                        <FormControl>
                          <RadioGroupItem value='user' />
                        </FormControl>
                        <FormLabel className='font-normal'>{t('apikeys.dialogs.fields.type.user')}</FormLabel>
                      </FormItem>
                      <FormItem className='flex items-center space-y-0 space-x-3'>
                        <FormControl>
                          <RadioGroupItem value='service_account' />
                        </FormControl>
                        <FormLabel className='font-normal'>{t('apikeys.dialogs.fields.type.serviceAccount')}</FormLabel>
                      </FormItem>
                    </RadioGroup>
                  </FormControl>
                  <FormDescription>
                    {apiKeyType === 'user'
                      ? t('apikeys.dialogs.fields.type.userDescription')
                      : t('apikeys.dialogs.fields.type.serviceAccountDescription')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {apiKeyType === 'service_account' && (
              <FormField
                control={form.control}
                name='scopes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('apikeys.dialogs.fields.scopes.label')}</FormLabel>
                    <FormControl>
                      <ScopesSelect value={field.value || []} onChange={field.onChange} portalContainer={dialogContent} />
                    </FormControl>
                    <FormDescription>{t('apikeys.dialogs.fields.scopes.description')}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <DialogFooter className='flex-col items-stretch gap-2 sm:flex-row sm:items-center sm:justify-end'>
              <div className='flex w-full gap-2 sm:w-auto'>
                <Button type='button' variant='outline' onClick={handleClose} disabled={isSubmitting}>
                  {t('common.buttons.cancel')}
                </Button>
                <Button type='submit' disabled={isSubmitting}>
                  {isSubmitting ? t('common.buttons.creating') : t('common.buttons.create')}
                </Button>
              </div>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
