import { useEffect } from 'react'
import { IconCheck, IconMoon, IconSun, IconPalette } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useTheme } from '@/context/theme-context'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export function ThemeSwitch() {
  const { theme, setTheme, colorScheme, setColorScheme } = useTheme()
  const { t } = useTranslation()

  const colorSchemes = [
    { name: 'blue', label: t('theme.colors.blue'), color: 'bg-blue-500' },
    { name: 'green', label: t('theme.colors.green'), color: 'bg-green-500' },
    { name: 'purple', label: t('theme.colors.purple'), color: 'bg-purple-500' },
    { name: 'orange', label: t('theme.colors.orange'), color: 'bg-orange-500' },
    { name: 'red', label: t('theme.colors.red'), color: 'bg-red-500' },
    { name: 'black', label: t('theme.colors.black'), color: 'bg-black' },
    { name: 'cream', label: t('theme.colors.cream'), color: 'bg-amber-100' },
    { name: 'claude', label: t('theme.colors.claude'), color: 'bg-amber-600' },
  ] as const

  /* Update theme-color meta tag when theme is updated */
  useEffect(() => {
    const themeColor = theme === 'dark' ? '#020817' : '#fff'
    const metaThemeColor = document.querySelector("meta[name='theme-color']")
    if (metaThemeColor) metaThemeColor.setAttribute('content', themeColor)
  }, [theme])

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' size='icon' className='scale-95 rounded-full'>
          <IconSun className='size-[1.2rem] scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90' />
          <IconMoon className='absolute size-[1.2rem] scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0' />
          <span className='sr-only'>{t('theme.toggle')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onClick={() => setTheme('light')}>
          {t('theme.light')}{' '}
          <IconCheck
            size={14}
            className={cn('ml-auto', theme !== 'light' && 'hidden')}
          />
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('dark')}>
          {t('theme.dark')}
          <IconCheck
            size={14}
            className={cn('ml-auto', theme !== 'dark' && 'hidden')}
          />
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('system')}>
          {t('theme.system')}
          <IconCheck
            size={14}
            className={cn('ml-auto', theme !== 'system' && 'hidden')}
          />
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <IconPalette size={14} className='mr-2' />
            {t('theme.colorScheme')}
            <div className={cn('ml-auto h-3 w-3 rounded-full', 
              colorSchemes.find(s => s.name === colorScheme)?.color || 'bg-blue-500'
            )} />
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {colorSchemes.map((scheme) => (
              <DropdownMenuItem
                key={scheme.name}
                onClick={() => setColorScheme(scheme.name)}
                className='flex items-center justify-between'
              >
                <div className='flex items-center'>
                  <div className={cn('mr-2 h-3 w-3 rounded-full', scheme.color)} />
                  {scheme.label}
                </div>
                <IconCheck
                  size={14}
                  className={cn('ml-auto', colorScheme !== scheme.name && 'hidden')}
                />
              </DropdownMenuItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
