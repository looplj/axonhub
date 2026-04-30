import { HTMLAttributes, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { passwordSchema } from '@/lib/validation';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/password-input';
import { useSignIn, useOIDCProviders, useOIDCAuthorize } from '@/features/auth/data/auth';
import { LogIn } from 'lucide-react';

type UserAuthFormProps = HTMLAttributes<HTMLFormElement>;

// Create form schema with dynamic validation messages
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z.email().min(1, { message: t('auth.signIn.validation.emailRequired') }),
    password: passwordSchema(t),
  });

export function UserAuthForm({ className, ...props }: UserAuthFormProps) {
  const { t } = useTranslation();
  const signInMutation = useSignIn();
  const [rememberMe, setRememberMe] = useState(false);
  const { data: oidcProviders } = useOIDCProviders();
  const oidcAuthorizeMutation = useOIDCAuthorize();

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
                    placeholder={t('auth.signIn.form.email.placeholder')}
                    className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
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
                <div className='flex items-center justify-between'>
                  <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signIn.form.password.label')}</FormLabel>
                  <Link
                    to='/forgot-password'
                    className='text-sm font-medium text-slate-500 transition-colors hover:text-slate-700 hover:underline'
                  >
                    {t('auth.signIn.links.forgotPassword')}
                  </Link>
                </div>
                <FormControl>
                  <PasswordInput
                    placeholder={t('auth.signIn.form.password.placeholder')}
                    className='border-slate-300 bg-white text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
                    data-testid='sign-in-password'
                    {...field}
                  />
                </FormControl>
                <FormMessage className='text-red-600' />
              </FormItem>
            )}
          />

          {/* Remember Me Toggle */}
          <div className='flex items-center justify-between'>
            <label className='flex cursor-pointer items-center space-x-3'>
              <div className='relative'>
                <input type='checkbox' checked={rememberMe} onChange={(e) => setRememberMe(e.target.checked)} className='sr-only' />
                <div
                  className={`h-6 w-12 rounded-full border-2 transition-all duration-300 ${rememberMe ? 'border-slate-600 bg-slate-600' : 'border-slate-300 bg-slate-100'}`}
                >
                  <div
                    className={`mt-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-300 ${rememberMe ? 'ml-0.5 translate-x-6' : 'translate-x-0.5'}`}
                  ></div>
                </div>
              </div>
              <span className='text-sm text-slate-700'>{t('auth.signIn.form.rememberMe')}</span>
            </label>
          </div>

          {/* Submit Button */}
          <Button
            type='submit'
            className='mt-6 w-full rounded-lg bg-slate-800 px-6 py-3 font-medium text-white shadow-lg transition-all duration-300 hover:bg-slate-700 hover:shadow-xl focus:ring-2 focus:ring-slate-500 focus:ring-offset-2 disabled:opacity-50'
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
                  <span className='bg-white px-2 text-slate-500'>Or continue with</span>
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
