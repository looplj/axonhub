import { useTranslation } from 'react-i18next'
import ContentSection from '../components/content-section'
import ProfileForm from './profile-form'

export default function SettingsProfile() {
  const { t } = useTranslation()

  return (
    <ContentSection
      title={t('profile.title')}
      desc={t('profile.description')}
    >
      <ProfileForm />
    </ContentSection>
  )
}
