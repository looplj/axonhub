'use client';

import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Pencil } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useSaveProxyPreset, ProxyPreset } from '../data/system';

interface Props {
  preset: ProxyPreset;
  trigger?: React.ReactNode;
}

interface FormValues {
  name: string;
}

export function ProxyPresetEditDialog({ preset, trigger }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const savePreset = useSaveProxyPreset();

  const form = useForm<FormValues>({
    defaultValues: {
      name: preset.name || '',
    },
  });

  const onSubmit = async (values: FormValues) => {
    savePreset.mutate(
      {
        name: values.name || undefined,
        url: preset.url,
        username: preset.username,
        password: preset.password,
      },
      {
        onSuccess: () => {
          setOpen(false);
        },
      }
    );
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset({ name: preset.name || '' });
        }
        setOpen(state);
      }}
    >
      {trigger ? (
        trigger
      ) : (
        <Button
          variant='ghost'
          size='sm'
          className='h-8 w-8 p-0'
          onClick={() => setOpen(true)}
        >
          <Pencil className='h-4 w-4' />
        </Button>
      )}
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('system.proxy.edit.title')}</DialogTitle>
          <DialogDescription>{t('system.proxy.edit.description')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form className='space-y-4'>
            <FormItem>
              <FormLabel>{t('system.proxy.columns.url')}</FormLabel>
              <FormControl>
                <Input value={preset.url} disabled className='bg-muted' />
              </FormControl>
            </FormItem>

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('system.proxy.edit.nameLabel')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('system.proxy.edit.namePlaceholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>

        <DialogFooter>
          <Button variant='outline' onClick={() => setOpen(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={form.handleSubmit(onSubmit)} disabled={savePreset.isPending}>
            {savePreset.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}