import { HTMLAttributes, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import { useSignIn } from '@/features/auth/data/auth'

type UserAuthFormProps = HTMLAttributes<HTMLFormElement>

// Create form schema with dynamic validation messages
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z
      .string()
      .min(1, { message: t('auth.signIn.validation.emailRequired') })
      .email({ message: t('auth.signIn.validation.emailInvalid') }),
    password: z
      .string()
      .min(1, {
        message: t('auth.signIn.validation.passwordRequired'),
      })
      .min(7, {
        message: t('auth.signIn.validation.passwordMinLength'),
      }),
  })

export function UserAuthForm({ className, ...props }: UserAuthFormProps) {
  const { t } = useTranslation()
  const signInMutation = useSignIn()
  const [rememberMe, setRememberMe] = useState(false)

  const formSchema = createFormSchema(t)
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      email: '',
      password: '',
    },
  })

  function onSubmit(data: z.infer<typeof formSchema>) {
    signInMutation.mutate(data)
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-6', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>
                {t('auth.signIn.form.email.label')}
              </FormLabel>
              <FormControl>
                <Input
                  placeholder={t('auth.signIn.form.email.placeholder')}
                  className='border-slate-300 bg-white/70 text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
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
                <FormLabel className='text-sm font-medium text-slate-700'>
                  {t('auth.signIn.form.password.label')}
                </FormLabel>
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
                  className='border-slate-300 bg-white/70 text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
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
              <input
                type='checkbox'
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                className='sr-only'
              />
              <div
                className={`h-6 w-12 rounded-full border-2 transition-all duration-300 ${rememberMe ? 'border-slate-600 bg-slate-600' : 'border-slate-300 bg-slate-100'}`}
              >
                <div
                  className={`mt-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-300 ${rememberMe ? 'ml-0.5 translate-x-6' : 'translate-x-0.5'}`}
                ></div>
              </div>
            </div>
            <span className='text-sm text-slate-700'>
              {t('auth.signIn.form.rememberMe')}
            </span>
          </label>
        </div>

        {/* Submit Button */}
        <Button
          className='mt-6 w-full rounded-lg bg-slate-800 px-6 py-3 font-medium text-white shadow-lg transition-all duration-300 hover:bg-slate-700 hover:shadow-xl focus:ring-2 focus:ring-slate-500 focus:ring-offset-2 disabled:opacity-50'
          disabled={signInMutation.isPending}
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
    </Form>
  )
}
