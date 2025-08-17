import { HTMLAttributes } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { IconBrandFacebook, IconBrandGithub } from '@tabler/icons-react'
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

const formSchema = z.object({
  email: z
    .string()
    .min(1, { message: 'Please enter your email' })
    .email({ message: 'Invalid email address' }),
  password: z
    .string()
    .min(1, {
      message: 'Please enter your password',
    })
    .min(7, {
      message: 'Password must be at least 7 characters long',
    }),
})

export function UserAuthForm({ className, ...props }: UserAuthFormProps) {
  const signInMutation = useSignIn()

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
            <FormItem className='animate-fade-in-up animation-delay-300'>
              <FormLabel className='text-white/90 text-sm font-medium'>Email</FormLabel>
              <FormControl>
                <Input 
                  placeholder='name@example.com' 
                  className='bg-white/10 border-white/20 text-white placeholder:text-white/50 backdrop-blur-sm focus:bg-white/15 focus:border-cyan-400/50 transition-all duration-300'
                  {...field} 
                />
              </FormControl>
              <FormMessage className='text-red-300' />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem className='relative animate-fade-in-up animation-delay-300'>
              <div className='flex justify-between items-center'>
                <FormLabel className='text-white/90 text-sm font-medium'>Password</FormLabel>
                {/* <Link
                  to='/forgot-password'
                  className='text-cyan-300 hover:text-cyan-200 text-sm font-medium transition-colors'
                >
                  Forgot password?
                </Link> */}
              </div>
              <FormControl>
                <PasswordInput 
                  placeholder='Enter your password' 
                  className='bg-white/10 border-white/20 text-white placeholder:text-white/50 backdrop-blur-sm focus:bg-white/15 focus:border-cyan-400/50 transition-all duration-300'
                  {...field} 
                />
              </FormControl>
              <FormMessage className='text-red-300' />
            </FormItem>
          )}
        />
        <Button 
          className='mt-4 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-white font-semibold py-3 px-6 rounded-lg shadow-lg hover:shadow-xl transform hover:scale-[1.02] transition-all duration-300 animate-fade-in-up animation-delay-300' 
          disabled={signInMutation.isPending}
        >
          {signInMutation.isPending ? (
            <div className='flex items-center gap-2'>
              <div className='w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin'></div>
              Signing In...
            </div>
          ) : (
            'Sign In'
          )}
        </Button>
      </form>
    </Form>
  )
}
