import { useTranslation } from 'react-i18next';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import { UserAuthForm } from './components/user-auth-form';

export default function SignIn() {
  const { t } = useTranslation();

  return (
    <AuthLayout>
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
