import AuthLayout from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'
import { useTranslation } from 'react-i18next'
import './login-styles.css'
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card.tsx";

export default function SignIn() {
  const { t } = useTranslation()

  return (
    <AuthLayout>
      <Card className='backdrop-blur-xl bg-[#252525]/80 border-[#D5DDDE]/20 shadow-[0_0_50px_rgba(0,255,157,0.1)] animate-fade-in-up animation-delay-300 hover:shadow-[0_0_80px_rgba(0,255,157,0.2)] transition-all duration-500'>
        <CardHeader className='text-center pb-6'>
          <CardTitle className='text-2xl font-bold text-[#F0F0F0] mb-2 animate-fade-in-up animation-delay-300'>
            {t('auth.signIn.title')}
          </CardTitle>
          <CardDescription className='text-[#B0B0B0] text-base animate-fade-in-up animation-delay-500'>
            {t('auth.signIn.subtitle')}
          </CardDescription>
        </CardHeader>
        <CardContent className='animate-fade-in-up animation-delay-700'>
          <UserAuthForm />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
