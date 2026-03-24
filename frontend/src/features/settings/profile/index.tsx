import { useTranslation } from 'react-i18next';
import ContentSection from '../components/content-section';
import ProfileForm from './profile-form';
import SecurityForm from '../security/security-form';
import OidcManagement from '../security/oidc-management';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useOIDCProviders } from '@/features/auth/data/auth';

export default function SettingsProfile() {
  const { t } = useTranslation();
  const { data: providers = [] } = useOIDCProviders();

  return (
    <ContentSection title={t('profile.title')} desc={t('profile.description')}>
      <Tabs defaultValue="profile" className="w-full">
        <TabsList className="mb-4">
          <TabsTrigger value="profile">{t('profile.title')}</TabsTrigger>
          <TabsTrigger value="security">{t('security.title', 'Security')}</TabsTrigger>
          {providers.length > 0 && <TabsTrigger value="oidc">OIDC</TabsTrigger>}
        </TabsList>
        <TabsContent value="profile">
          <ProfileForm />
        </TabsContent>
        <TabsContent value="security">
          <SecurityForm />
        </TabsContent>
        {providers.length > 0 && (
          <TabsContent value="oidc">
            <OidcManagement providers={providers} />
          </TabsContent>
        )}
      </Tabs>
    </ContentSection>
  );
}
