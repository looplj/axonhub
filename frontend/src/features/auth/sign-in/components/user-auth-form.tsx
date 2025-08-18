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
              <FormLabel className='text-sm font-medium text-[#F0F0F0]'>
                {t('auth.signIn.form.email.label')}
              </FormLabel>
              <FormControl>
                <Input
                  placeholder={t('auth.signIn.form.email.placeholder')}
                  className='focus-particles hover-glow border-[#C0C0C0]/30 bg-[#1A1A1A]/50 text-[#F0F0F0] backdrop-blur-sm transition-all duration-300 placeholder:text-[#B0B0B0] focus:border-[#E8E8E8] focus:bg-[#1A1A1A]/70'
                  {...field}
                />
              </FormControl>
              <FormMessage className='text-[#FF2E4D]' />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem className='relative'>
              <div className='flex items-center justify-between'>
                <FormLabel className='text-sm font-medium text-[#F0F0F0]'>
                  {t('auth.signIn.form.password.label')}
                </FormLabel>
                <Link
                  to='/forgot-password'
                  className='hover-glow text-sm font-medium text-[#C0C0C0] transition-colors hover:text-[#E0E0E0]'
                >
                  {t('auth.signIn.links.forgotPassword')}
                </Link>
              </div>
              <FormControl>
                <PasswordInput
                  placeholder={t('auth.signIn.form.password.placeholder')}
                  className='focus-particles hover-glow border-[#C0C0C0]/30 bg-[#1A1A1A]/50 text-[#F0F0F0] backdrop-blur-sm transition-all duration-300 placeholder:text-[#B0B0B0] focus:border-[#E8E8E8] focus:bg-[#1A1A1A]/70'
                  {...field}
                />
              </FormControl>
              <FormMessage className='text-[#FF2E4D]' />
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
                className={`h-6 w-12 rounded-full border-2 transition-all duration-300 ${rememberMe ? 'border-[#C0C0C0] bg-[#C0C0C0]' : 'border-[#C0C0C0]/30 bg-[#2A2A2A]'}`}
              >
                <div
                  className={`mt-0.5 h-4 w-4 rounded-full bg-[#1A1A1A] transition-transform duration-300 ${rememberMe ? 'ml-0.5 translate-x-6' : 'translate-x-0.5'}`}
                ></div>
              </div>
            </div>
            <span className='text-sm text-[#F0F0F0]'>
              {t('auth.signIn.form.rememberMe')}
            </span>
          </label>
        </div>

        {/* Submit Button */}
        <Button
          className='animate-breathing-glow mt-4 rounded-lg bg-[#FF2E4D] px-6 py-4 font-semibold text-[#F0F0F0] shadow-[0_0_20px_rgba(255,46,77,0.3)] transition-all duration-300 hover:bg-[#FF1A3D] hover:shadow-[0_0_30px_rgba(255,46,77,0.5)]'
          disabled={signInMutation.isPending}
        >
          {signInMutation.isPending ? (
            <div className='flex items-center gap-2'>
              <div className='h-4 w-4 animate-spin rounded-full border-2 border-[#F0F0F0]/30 border-t-[#F0F0F0]'></div>
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
