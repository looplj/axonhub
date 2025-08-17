import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import AuthLayout from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export default function SignIn() {
  return (
    <AuthLayout>
      <Card className='backdrop-blur-xl bg-white/10 border-white/20 shadow-2xl animate-fade-in-up animation-delay-300'>
        <CardHeader className='text-center pb-8'>
          <CardTitle className='text-2xl font-bold text-white mb-2 animate-fade-in-up animation-delay-300'>
            Welcome Back
          </CardTitle>
          <CardDescription className='text-cyan-100/80 text-base animate-fade-in-up animation-delay-300'>
            Enter your credentials to access your account
          </CardDescription>
        </CardHeader>
        <CardContent className='animate-fade-in-up animation-delay-300'>
          <UserAuthForm />
        </CardContent>
        {/* <CardFooter className='pt-6 animate-fade-in-up animation-delay-300'>
          <p className='text-cyan-100/60 text-center text-sm w-full'>
            By signing in, you agree to our{' '}
            <a
              href='/terms'
              className='text-cyan-300 hover:text-cyan-200 underline underline-offset-4 transition-colors'
            >
              Terms of Service
            </a>{' '}
            and{' '}
            <a
              href='/privacy'
              className='text-cyan-300 hover:text-cyan-200 underline underline-offset-4 transition-colors'
            >
              Privacy Policy
            </a>
          </p>
        </CardFooter> */}
      </Card>
    </AuthLayout>
  )
}
