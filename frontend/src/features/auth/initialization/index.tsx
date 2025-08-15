import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { useTranslation } from 'react-i18next'
import AuthLayout from '../auth-layout'
import { InitializationForm } from './components/initialization-form'

export default function Initialization() {
  const { t } = useTranslation()
  
  return (
    <AuthLayout>
      <Card className='gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>{t('initialization.title')}</CardTitle>
          <CardDescription>
            {t('initialization.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <InitializationForm />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}