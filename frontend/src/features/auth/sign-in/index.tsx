import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import AuthLayout from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export default function SignIn() {
  return (
    <AuthLayout>
      <Card className='backdrop-blur-xl bg-[#252525]/80 border-[#D5DDDE]/20 shadow-[0_0_50px_rgba(0,255,157,0.1)] animate-fade-in-up animation-delay-300 hover:shadow-[0_0_80px_rgba(0,255,157,0.2)] transition-all duration-500'>
        <CardHeader className='text-center pb-6'>
          <CardTitle className='text-2xl font-bold text-[#F0F0F0] mb-2 animate-fade-in-up animation-delay-300'>
            欢迎回来
          </CardTitle>
          <CardDescription className='text-[#B0B0B0] text-base animate-fade-in-up animation-delay-500'>
            输入您的凭据以访问您的账户
          </CardDescription>
        </CardHeader>
        <CardContent className='animate-fade-in-up animation-delay-700'>
          <UserAuthForm />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
