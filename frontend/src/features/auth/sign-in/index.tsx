import { useTranslation } from 'react-i18next';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import AnimatedLineBackground from './components/animated-line-background';
import { UserAuthForm } from './components/user-auth-form';
import './login-styles.css';

export default function SignIn() {
  const { t } = useTranslation();

  return (
    <AuthLayout>
      <AnimatedLineBackground key='optimized-layout' />
      <TwoColumnAuth
        title={t('auth.signIn.title')}
        description={t('auth.signIn.subtitle')}
        rightFooter={<p className='text-xs leading-relaxed text-slate-500 sm:text-sm'>{t('auth.signIn.footer.agreement')}</p>}
      >
        <UserAuthForm />
      </TwoColumnAuth>
    </AuthLayout>
  );
}
