import { HTMLAttributes } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { passwordSchema } from '@/lib/validation';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/password-input';
import { useSignIn, useOIDCProviders, useOIDCAuthorize } from '@/features/auth/data/auth';
import { LogIn } from 'lucide-react';

type UserAuthFormProps = HTMLAttributes<HTMLFormElement> & {
  redirect?: string;
};

// Create form schema with dynamic validation messages
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z
      .string()
      .min(1, { message: t('auth.signIn.validation.emailRequired') })
      .pipe(z.email({ message: t('auth.signIn.validation.emailInvalid') })),
    password: passwordSchema(t),
  });

export function UserAuthForm({ className, redirect, ...props }: UserAuthFormProps) {
  const { t } = useTranslation();
  const signInMutation = useSignIn(redirect);
  const { data: oidcProviders } = useOIDCProviders();
  const oidcAuthorizeMutation = useOIDCAuthorize(redirect);

  const formSchema = createFormSchema(t);
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      email: '',
      password: '',
    },
  });

  function onSubmit(data: z.infer<typeof formSchema>) {
    signInMutation.mutate(data);
  }

  const isPasswordLoginDisabled = oidcProviders?.some((p) => p.active && p.oidc_login_only);
  
  return (
    <Form {...form}>
      {!isPasswordLoginDisabled && (
        <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-6', className)} {...props}>
          <FormField
            control={form.control}
            name='email'
            render={({ field }) => (
              <FormItem>
                <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signIn.form.email.label')}</FormLabel>
                <FormControl>
                  <Input
                    type='email'
                    autoComplete='username'
                    placeholder={t('auth.signIn.form.email.placeholder')}
                    className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-[#A8844E] focus:!bg-white'
                    data-testid='sign-in-email'
                    {...field}
                  />
                </FormControl>
                <FormMessage className='text-red-600' />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='password'
            render={({ field }) => (
              <FormItem className='relative'>
                <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signIn.form.password.label')}</FormLabel>
                <FormControl>
                  <PasswordInput
                    autoComplete='current-password'
                    placeholder={t('auth.signIn.form.password.placeholder')}
                    className='border-slate-300 bg-white text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-[#A8844E] focus:bg-white'
                    data-testid='sign-in-password'
                    {...field}
                  />
                </FormControl>
                <FormMessage className='text-red-600' />
              </FormItem>
            )}
          />

          {/* Submit Button */}
          <Button
            type='submit'
            className='mt-2 w-full rounded-lg bg-[#1A2023] px-6 py-3 font-medium text-white shadow-lg transition-all duration-300 hover:bg-[#2A3138] hover:shadow-xl focus:ring-2 focus:ring-[#A8844E] focus:ring-offset-2 disabled:opacity-50'
            disabled={signInMutation.isPending}
            data-testid='sign-in-submit'
          >
            {signInMutation.isPending ? (
              <div className='flex items-center justify-center gap-2'>
                <div className='h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white'></div>
                {t('auth.signIn.form.signingIn')}
              </div>
            ) : (
              t('auth.signIn.form.signInButton')
            )}
          </Button>
        </form>
      )}
        
        {oidcProviders && oidcProviders.length > 0 && (
          <div className={cn(!isPasswordLoginDisabled && 'mt-6')}>
            {!isPasswordLoginDisabled && (
              <div className='relative'>
                <div className='absolute inset-0 flex items-center'>
                  <span className='w-full border-t border-slate-300' />
                </div>
                <div className='relative flex justify-center text-xs uppercase'>
                  <span className='bg-white px-2 text-slate-500'>{t('auth.signIn.oidc.divider')}</span>
                </div>
              </div>
            )}

            <div className={cn(oidcProviders.length > 0 && !isPasswordLoginDisabled && 'mt-6', 'grid gap-2')}>
              {oidcProviders.map((provider) => {
                const isInactive = provider.active === false;
                const providerId = provider.id || provider.name;
                const providerLabel = provider.display_name || provider.name;

                return (
                  <Button
                    key={providerId}
                    type='button'
                    variant='outline'
                    className={cn(
                      'h-auto w-full border-slate-300 py-3 disabled:opacity-50',
                      isInactive && 'border-2 border-destructive'
                    )}
                    style={
                      provider.button_color
                        ? {
                            backgroundColor: provider.button_color,
                            color: '#ffffff',
                            borderColor: isInactive ? 'var(--destructive)' : provider.button_color,
                          }
                        : undefined
                    }
                    disabled={oidcAuthorizeMutation.isPending}
                    onClick={() => oidcAuthorizeMutation.mutate(providerId)}
                    title={isInactive ? t('common.status.inactiveRetry') : undefined}
                  >
                    {oidcAuthorizeMutation.isPending && oidcAuthorizeMutation.variables === providerId ? (
                      <div className='mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current/30 border-t-current'></div>
                    ) : provider.icon_url ? (
                      <img src={provider.icon_url} alt={providerLabel} className='mr-2 h-4 w-4 object-contain' />
                    ) : (
                      <LogIn className='mr-2 h-4 w-4' />
                    )}
                    <span className='flex min-w-0 flex-col items-center'>
                      <span className='truncate'>{providerLabel}</span>
                      {isInactive && <span className='text-xs font-medium text-current/85'>{t('common.status.inactiveRetry')}</span>}
                    </span>
                  </Button>
                );
              })}
            </div>
          </div>
        )}

    </Form>
  );
}
