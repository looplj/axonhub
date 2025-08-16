import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { IconLanguage } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useLanguage } from '@/hooks/useLanguage'

const languages = [
  { code: 'en', name: 'English' },
  { code: 'zh', name: '中文' },
]

export function LanguageSwitch() {
  const { currentLanguage, changeLanguage, isUpdating } = useLanguage()
  const { t } = useTranslation()

  const currentLanguageInfo = languages.find(lang => lang.code === currentLanguage) || languages[0]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="h-8 w-8 px-0" disabled={isUpdating}>
          <IconLanguage className="h-4 w-4" />
          <span className="sr-only">{t('language.toggle')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {languages.map((language) => (
          <DropdownMenuItem
            key={language.code}
            onClick={() => changeLanguage(language.code)}
            className={currentLanguage === language.code ? 'bg-accent' : ''}
            disabled={isUpdating}
          >
            {language.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}