import { useTranslation } from 'react-i18next'
import AuthLayout from '../auth-layout'
import TwoColumnAuth from '../components/two-column-auth'
import AnimatedLineBackground from '../sign-in/components/animated-line-background'
import { InitializationForm } from './components/initialization-form'

export default function Initialization() {
  const { t } = useTranslation()

  return (
    <AuthLayout>
      <AnimatedLineBackground key="optimized-layout" />
      <TwoColumnAuth
        title={t('initialization.title')}
        description={t('initialization.description')}
        rightMaxWidthClassName='max-w-2xl'
      >
        <InitializationForm />
      </TwoColumnAuth>
    </AuthLayout>
  )
}