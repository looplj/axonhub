import { HTMLAttributes, useEffect, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useRouter } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { authApi } from '@/lib/api-client';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/password-input';

type SignUpFormProps = HTMLAttributes<HTMLFormElement>;

const formSchema = z
  .object({
    email: z.string().email(),
    firstName: z.string().min(1),
    lastName: z.string().min(1),
    password: z.string().min(7),
    confirmPassword: z.string(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords don't match.",
    path: ['confirmPassword'],
  });

export function SignUpForm({ className, ...props }: SignUpFormProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const { setUser, setAccessToken } = useAuthStore((state) => state.auth);
  const invitationToken = new URLSearchParams(window.location.search).get('invite');
  const [projectName, setProjectName] = useState('');
  const [invitationError, setInvitationError] = useState(!invitationToken ? t('users.invitation.required') : '');
  const [isLoadingInvitation, setIsLoadingInvitation] = useState(Boolean(invitationToken));

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: { email: '', firstName: '', lastName: '', password: '', confirmPassword: '' },
  });

  useEffect(() => {
    if (!invitationToken) {
      return;
    }

    authApi
      .getInvitation(invitationToken)
      .then((invitation) => {
        setProjectName(invitation.projectName);
      })
      .catch((error) => setInvitationError(error instanceof Error ? error.message : t('users.invitation.invalid')))
      .finally(() => setIsLoadingInvitation(false));
  }, [invitationToken, t]);

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    if (!invitationToken || invitationError) {
      return;
    }

    try {
      const response = await authApi.registerInvitation(invitationToken, values);
      setAccessToken(response.token);
      setUser(response.user);
      toast.success(t('users.messages.invitationRegistrationSuccess'));
      router.navigate({ to: '/project/playground' });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.internalServerError'));
    }
  };

  if (isLoadingInvitation) {
    return <p className='text-sm text-muted-foreground'>{t('common.buttons.processing')}</p>;
  }

  if (invitationError) {
    return <p className='text-sm text-destructive'>{invitationError}</p>;
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-3', className)} {...props}>
        <p className='text-sm text-muted-foreground'>{t('users.invitation.joinProject', { projectName })}</p>
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('users.form.email')}</FormLabel>
              <FormControl>
                <Input type='email' placeholder='name@example.com' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <div className='grid grid-cols-2 gap-3'>
          <FormField
            control={form.control}
            name='firstName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('users.form.firstName')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='lastName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('users.form.lastName')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('users.form.password')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder='********' {...field} />
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
              <FormLabel>{t('users.form.confirmPassword')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder='********' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button className='mt-2' disabled={form.formState.isSubmitting}>
          {t('users.buttons.completeRegistration')}
        </Button>
      </form>
    </Form>
  );
}
