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
import { useProjectStore } from '@/stores/projectStore';
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
  const { setSelectedProjectId } = useProjectStore();
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
      setSelectedProjectId(response.user.projects[0]?.projectID ?? null);
      toast.success(t('users.messages.invitationRegistrationSuccess'));
      router.navigate({ to: '/project/playground' });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.internalServerError'));
    }
  };

  if (isLoadingInvitation) {
    return <p className='rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600'>{t('common.buttons.processing')}</p>;
  }

  if (invitationError) {
    return <p className='rounded-lg border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm text-destructive'>{invitationError}</p>;
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-4', className)} {...props}>
        <p className='rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm font-medium text-slate-600'>
          {t('users.invitation.joinProject', { projectName })}
        </p>
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('users.form.email')}</FormLabel>
              <FormControl>
                <Input type='email' placeholder='name@example.com' className='border-slate-300 !bg-white text-slate-800 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <div className='grid gap-4 sm:grid-cols-2'>
          <FormField
            control={form.control}
            name='firstName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('users.form.firstName')}</FormLabel>
                <FormControl>
                  <Input className='border-slate-300 !bg-white text-slate-800 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200' {...field} />
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
                  <Input className='border-slate-300 !bg-white text-slate-800 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200' {...field} />
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
                <PasswordInput placeholder='********' className='border-slate-300 !bg-white text-slate-800 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200' {...field} />
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
                <PasswordInput placeholder='********' className='border-slate-300 !bg-white text-slate-800 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button
          className='mt-2 w-full rounded-lg bg-slate-800 px-6 py-3 font-medium text-white shadow-lg transition-all duration-300 hover:bg-slate-700 hover:shadow-xl focus:ring-2 focus:ring-slate-500 focus:ring-offset-2 disabled:opacity-50'
          disabled={form.formState.isSubmitting}
        >
          {t('users.buttons.completeRegistration')}
        </Button>
      </form>
    </Form>
  );
}
