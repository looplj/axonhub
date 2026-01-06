'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { graphqlRequest } from '@/gql/graphql';
import { UPDATE_USER_MUTATION } from '@/gql/users';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { User, changePasswordFormSchema } from '../data/schema';

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersChangePasswordDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const form = useForm({
    resolver: zodResolver(changePasswordFormSchema(t)),
    defaultValues: {
      newPassword: '',
      confirmPassword: '',
    },
  });

  const onSubmit = async (values: any) => {
    try {
      if (!currentRow?.id) {
        throw new Error('No user selected');
      }

      // 使用 GraphQL updateUser mutation 进行真正的密码修改
      await graphqlRequest(UPDATE_USER_MUTATION, {
        id: currentRow.id,
        input: {
          password: values.newPassword,
        },
      });

      toast.success(t('users.messages.passwordChangeSuccess'));
      form.reset();
      onOpenChange(false);
    } catch (error) {
          toast.error(t('users.messages.passwordChangeError'));
        }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('users.dialogs.changePassword.title')}</DialogTitle>
          <DialogDescription>
            {t('users.dialogs.changePassword.description', {
              firstName: currentRow?.firstName || '',
              lastName: currentRow?.lastName || '',
              name: `${currentRow?.firstName} ${currentRow?.lastName}`,
              email: currentRow?.email || '',
            })}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form id='change-password-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='newPassword'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('users.form.newPassword')}</FormLabel>
                  <FormControl>
                    <Input type='password' placeholder={t('users.form.placeholders.newPasswordPlaceholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='confirmPassword'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('users.form.confirmNewPassword')}</FormLabel>
                  <FormControl>
                    <Input type='password' placeholder={t('users.form.placeholders.confirmNewPasswordPlaceholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='submit' form='change-password-form' disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting ? t('users.buttons.changing') : t('users.buttons.changePassword')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
