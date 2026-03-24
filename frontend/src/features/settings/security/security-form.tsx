import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useAuthStore } from '@/stores/authStore';
import { graphqlRequest } from '@/gql/graphql';
import { UPDATE_MY_PASSWORD_MUTATION } from '@/gql/users';

const getFormSchema = (hasPassword: boolean) => {
  const schema = z.object({
    oldPassword: hasPassword ? z.string().min(1, 'Current password is required') : z.string().optional(),
    newPassword: z.string().min(8, 'Password must be at least 8 characters'),
    confirmPassword: z.string().min(1, 'Confirm new password is required'),
  }).refine((data) => data.newPassword === data.confirmPassword, {
    message: "Passwords don't match",
    path: ['confirmPassword'],
  });
  return schema;
};

type FormValues = {
    oldPassword?: string;
    newPassword: string;
    confirmPassword: string;
};

export default function SecurityForm() {
  const { t } = useTranslation();
  const [isUpdating, setIsUpdating] = useState(false);
  const user = useAuthStore((state) => state.auth.user);

  const hasPassword = user?.hasPassword ?? true;
  const formSchema = getFormSchema(hasPassword);

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema) as any,
    defaultValues: {
      oldPassword: '',
      newPassword: '',
      confirmPassword: '',
    },
  });

  const onSubmit = async (data: FormValues) => {
    setIsUpdating(true);
    try {
      if (!user) {
        throw new Error('User not loaded');
      }

      await graphqlRequest(UPDATE_MY_PASSWORD_MUTATION, {
        input: {
          oldPassword: hasPassword ? data.oldPassword : null,
          newPassword: data.newPassword,
        },
      });

      toast.success(t('security.messages.passwordChangeSuccess', 'Password changed successfully'));
      form.reset();
    } catch (error: any) {
      console.error('Failed to change password:', error);
      toast.error(
        t('security.messages.passwordChangeError', 'Failed to change password: ') +
          (error.response?.errors?.[0]?.message || error.message || t('common.errors.unknown')),
      );
    } finally {
      setIsUpdating(false);
    }
  };

  return (
    <div className='space-y-8'>
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <h3 className='text-lg font-medium'>
            {hasPassword ? t('security.password.changeTitle', 'Change Password') : t('security.password.setTitle', 'Set Initial Password')}
          </h3>
          
          {hasPassword && (
            <FormField
              control={form.control}
              name='oldPassword'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('security.password.oldPassword', 'Current Password')}</FormLabel>
                  <FormControl>
                    <Input type='password' placeholder='********' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}

          <FormField
            control={form.control}
            name='newPassword'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('security.password.newPassword', 'New Password')}</FormLabel>
                <FormControl>
                  <Input type='password' placeholder='********' {...field} />
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
                <FormLabel>{t('security.password.confirmPassword', 'Confirm New Password')}</FormLabel>
                <FormControl>
                  <Input type='password' placeholder='********' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div>
            <Button type='submit' disabled={isUpdating}>
              {isUpdating 
                ? t('common.saving', 'Saving...') 
                : (hasPassword 
                    ? t('security.password.changeButton', 'Change Password') 
                    : t('security.password.setButton', 'Set Password'))}
            </Button>
          </div>
        </form>
      </Form>
    </div>
  );
}
