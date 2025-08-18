import { HTMLAttributes, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { IconBrandGithub, IconBrandGoogle, IconRefresh } from '@tabler/icons-react'
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
    .min(1, { message: '请输入您的邮箱' })
    .email({ message: '邮箱地址格式无效' }),
  password: z
    .string()
    .min(1, {
      message: '请输入您的密码',
    })
    .min(7, {
      message: '密码至少需要7个字符',
    }),
  captcha: z.string().min(1, { message: '请输入验证码' }),
})

export function UserAuthForm({ className, ...props }: UserAuthFormProps) {
  const signInMutation = useSignIn()
  const [rememberMe, setRememberMe] = useState(false)
  const [captcha, setCaptcha] = useState('A7B9')

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      email: '',
      password: '',
      captcha: '',
    },
  })

  function onSubmit(data: z.infer<typeof formSchema>) {
    signInMutation.mutate(data)
  }

  const refreshCaptcha = () => {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
    let result = ''
    for (let i = 0; i < 4; i++) {
      result += chars.charAt(Math.floor(Math.random() * chars.length))
    }
    setCaptcha(result)
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
              <FormLabel className='text-[#F0F0F0] text-sm font-medium'>邮箱</FormLabel>
              <FormControl>
                <Input 
                  placeholder='name@example.com' 
                  className='bg-[#1A1A1A]/50 border-[#C0C0C0]/30 text-[#F0F0F0] placeholder:text-[#B0B0B0] backdrop-blur-sm focus:bg-[#1A1A1A]/70 focus:border-[#E8E8E8] focus-particles hover-glow transition-all duration-300'
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
            <FormItem className='relative animate-fade-in-up animation-delay-500'>
              {/* <div className='flex justify-between items-center'>
                <FormLabel className='text-[#F0F0F0] text-sm font-medium'>密码</FormLabel>
                <Link
                  to='/forgot-password'
                  className='text-[#C0C0C0] hover:text-[#E0E0E0] text-sm font-medium transition-colors hover-glow'
                >
                  忘记密码？
                </Link>
              </div> */}
              <FormControl>
                <PasswordInput 
                  placeholder='输入您的密码' 
                  className='bg-[#1A1A1A]/50 border-[#C0C0C0]/30 text-[#F0F0F0] placeholder:text-[#B0B0B0] backdrop-blur-sm focus:bg-[#1A1A1A]/70 focus:border-[#E8E8E8] focus-particles hover-glow transition-all duration-300'
                  {...field} 
                />
              </FormControl>
              <FormMessage className='text-[#FF2E4D]' />
            </FormItem>
          )}
        />
        
        {/* <FormField
          control={form.control}
          name='captcha'
          render={({ field }) => (
            <FormItem className='animate-fade-in-up animation-delay-700'>
              <FormLabel className='text-[#F0F0F0] text-sm font-medium'>验证码</FormLabel>
              <div className='flex gap-3'>
                <FormControl>
                  <Input 
                    placeholder='输入验证码' 
                    className='bg-[#1A1A1A]/50 border-[#C0C0C0]/30 text-[#F0F0F0] placeholder:text-[#B0B0B0] backdrop-blur-sm focus:bg-[#1A1A1A]/70 focus:border-[#E8E8E8] focus-particles hover-glow transition-all duration-300 flex-1'
                    {...field} 
                  />
                </FormControl>
                <div className='flex items-center gap-2'>
                  <div className='bg-[#2A2A2A] border border-[#C0C0C0]/30 px-4 py-2 rounded font-mono text-[#C0C0C0] text-lg tracking-wider animate-neon-pulse'>
                    {captcha}
                  </div>
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    onClick={refreshCaptcha}
                    className='border-[#C0C0C0]/30 text-[#C0C0C0] hover:bg-[#C0C0C0]/10 hover:border-[#C0C0C0] transition-all duration-300 hover-glow'
                  >
                    <IconRefresh className='h-4 w-4' />
                  </Button>
                </div>
              </div>
              <FormMessage className='text-[#FF2E4D]' />
            </FormItem>
          )}
        /> */}
        
        {/* Remember Me Toggle */}
        <div className='flex items-center justify-between animate-fade-in-up animation-delay-1000'>
          <label className='flex items-center space-x-3 cursor-pointer'>
            <div className='relative'>
              <input
                type='checkbox'
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                className='sr-only'
              />
              <div className={`w-12 h-6 rounded-full border-2 transition-all duration-300 ${rememberMe ? 'bg-[#C0C0C0] border-[#C0C0C0]' : 'bg-[#2A2A2A] border-[#C0C0C0]/30'}`}>
                <div className={`w-4 h-4 bg-[#1A1A1A] rounded-full transition-transform duration-300 mt-0.5 ${rememberMe ? 'translate-x-6 ml-0.5' : 'translate-x-0.5'}`}></div>
              </div>
            </div>
            <span className='text-[#F0F0F0] text-sm'>记住账号</span>
          </label>
        </div>
        
        {/* Submit Button */}
        <Button 
          className='mt-4 bg-[#FF2E4D] hover:bg-[#FF1A3D] text-[#F0F0F0] font-semibold py-4 px-6 rounded-lg shadow-[0_0_20px_rgba(255,46,77,0.3)] hover:shadow-[0_0_30px_rgba(255,46,77,0.5)] animate-breathing-glow transition-all duration-300' 
          disabled={signInMutation.isPending}
        >
          {signInMutation.isPending ? (
            <div className='flex items-center gap-2'>
              <div className='w-4 h-4 border-2 border-[#F0F0F0]/30 border-t-[#F0F0F0] rounded-full animate-spin'></div>
              登录中...
            </div>
          ) : (
            '登录'
          )}
        </Button>
        
        {/* Third-party Login */}
        {/* <div className='space-y-4 animate-fade-in-up animation-delay-1200'>
          <div className='relative'>
            <div className='absolute inset-0 flex items-center'>
              <div className='w-full border-t border-[#C0C0C0]/20'></div>
            </div>
            <div className='relative flex justify-center text-sm'>
              <span className='bg-[#252525] px-4 text-[#B0B0B0]'>或使用第三方登录</span>
            </div>
          </div>
          
          <div className='grid grid-cols-2 gap-3'>
            <Button
              type='button'
              variant='outline'
              className='border-[#C0C0C0]/30 text-[#F0F0F0] hover:bg-[#C0C0C0]/10 hover:border-[#C0C0C0] hover:text-[#C0C0C0] transition-all duration-300 hover-glow'
            >
              <IconBrandGithub className='h-4 w-4 mr-2' />
              GitHub
            </Button>
            <Button
              type='button'
              variant='outline'
              className='border-[#C0C0C0]/30 text-[#F0F0F0] hover:bg-[#C0C0C0]/10 hover:border-[#C0C0C0] hover:text-[#C0C0C0] transition-all duration-300 hover-glow'
            >
              <IconBrandGoogle className='h-4 w-4 mr-2' />
              Google
            </Button>
          </div>
        </div> */}
        
        {/* Register Link */}
        {/* <div className='text-center animate-fade-in-up animation-delay-1400'>
          <span className='text-[#B0B0B0] text-sm'>新用户？</span>
          <Link
            to='/sign-up'
            className='text-[#C0C0C0] hover:text-[#E0E0E0] text-sm font-medium ml-2 transition-colors hover-glow'
          >
            创建账户
          </Link>
        </div> */}
      </form>
    </Form>
  )
}
