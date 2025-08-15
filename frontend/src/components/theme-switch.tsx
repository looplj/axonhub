import { useEffect } from 'react'
import { IconCheck, IconMoon, IconSun, IconPalette } from '@tabler/icons-react'
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

const colorSchemes = [
  { name: 'blue', label: 'Blue', color: 'bg-blue-500' },
  { name: 'green', label: 'Green', color: 'bg-green-500' },
  { name: 'purple', label: 'Purple', color: 'bg-purple-500' },
  { name: 'orange', label: 'Orange', color: 'bg-orange-500' },
  { name: 'red', label: 'Red', color: 'bg-red-500' },
  { name: 'black', label: 'Black', color: 'bg-black' },
  { name: 'cream', label: 'Cream', color: 'bg-amber-100' },
] as const

export function ThemeSwitch() {
  const { theme, setTheme, colorScheme, setColorScheme } = useTheme()

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
          <span className='sr-only'>Toggle theme</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onClick={() => setTheme('light')}>
          Light{' '}
          <IconCheck
            size={14}
            className={cn('ml-auto', theme !== 'light' && 'hidden')}
          />
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('dark')}>
          Dark
          <IconCheck
            size={14}
            className={cn('ml-auto', theme !== 'dark' && 'hidden')}
          />
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('system')}>
          System
          <IconCheck
            size={14}
            className={cn('ml-auto', theme !== 'system' && 'hidden')}
          />
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <IconPalette size={14} className='mr-2' />
            Color Scheme
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
