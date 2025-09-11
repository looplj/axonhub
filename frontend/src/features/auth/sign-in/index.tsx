import { UserAuthForm } from './components/user-auth-form'
import AnimatedLineBackground from './components/animated-line-background'
import { useTranslation } from 'react-i18next'
import './login-styles.css'
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth'

export default function SignIn() {
  const { t } = useTranslation()

  return (
    <AuthLayout>
      <AnimatedLineBackground key="optimized-layout" />
      <TwoColumnAuth
        title={t('auth.signIn.title')}
        description={t('auth.signIn.subtitle')}
        rightFooter={
          <p className="text-xs sm:text-sm text-slate-500 leading-relaxed">
            {t('auth.signIn.footer.agreement')}
          </p>
        }
      >
        <UserAuthForm />
      </TwoColumnAuth>
    </AuthLayout>
  )
}
