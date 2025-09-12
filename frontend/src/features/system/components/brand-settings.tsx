'use client'

import React, { useState, useRef } from 'react'
import { Loader2, Save, Upload, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useSystemContext } from '../context/system-context'
import { 
  useBrandSettings, 
  useUpdateBrandSettings
} from '../data/system'

export function BrandSettings() {
  const { t } = useTranslation()
  const { data: settings, isLoading: isLoadingSettings } = useBrandSettings()
  const updateSettings = useUpdateBrandSettings()
  const { isLoading, setIsLoading } = useSystemContext()

  const [brandName, setBrandName] = useState(settings?.brandName ?? '')
  const [brandLogo, setBrandLogo] = useState(settings?.brandLogo ?? '')
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Update local state when settings are loaded
  React.useEffect(() => {
    if (settings) {
      setBrandName(settings.brandName ?? '')
      setBrandLogo(settings.brandLogo ?? '')
    }
  }, [settings])

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    // Validate file type
    if (!['image/png', 'image/jpeg'].includes(file.type)) {
      toast.error(t('system.general.brandLogo.invalidFormat'))
      return
    }

    // Validate file size (max 2MB)
    if (file.size > 2 * 1024 * 1024) {
      toast.error(t('system.general.brandLogo.fileTooLarge'))
      return
    }

    const reader = new FileReader()
    reader.onload = (e) => {
      const img = new Image()
      img.onload = () => {
        // Check if image is square
        if (img.width !== img.height) {
          toast.error(t('system.general.brandLogo.notSquare'))
          return
        }
        setBrandLogo(e.target?.result as string)
      }
      img.src = e.target?.result as string
    }
    reader.readAsDataURL(file)
  }

  const handleRemoveLogo = () => {
    setBrandLogo('')
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleSave = async () => {
    setIsLoading(true)
    try {
      await updateSettings.mutateAsync({
        brandName: brandName.trim() || undefined,
        brandLogo: brandLogo || undefined,
      })
    } finally {
      setIsLoading(false)
    }
  }

  const hasChanges =
    settings &&
    ((settings.brandName ?? '') !== brandName ||
      (settings.brandLogo ?? '') !== brandLogo)

  if (isLoadingSettings) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>
          {t('loading')}
        </span>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.general.title')}</CardTitle>
          <CardDescription>{t('system.general.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='space-y-2'>
            <Label htmlFor='brand-name'>
              {t('system.general.brandName.label')}
            </Label>
            <Input
              id='brand-name'
              type='text'
              placeholder={t('system.general.brandName.placeholder')}
              value={brandName}
              onChange={(e) => setBrandName(e.target.value)}
              disabled={isLoading}
              className='max-w-md'
            />
            <div className='text-muted-foreground text-sm'>
              {t('system.general.brandName.description')}
            </div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='brand-logo'>
              {t('system.general.brandLogo.label')}
            </Label>
            {brandLogo && (
              <div className='flex justify-center mb-4 max-w-md'>
                <div className='relative'>
                  <img
                    src={brandLogo}
                    alt='Brand Logo Preview'
                    className='w-60 h-60 object-cover rounded border'
                  />
                  <Button
                    type='button'
                    variant='destructive'
                    size='sm'
                    onClick={handleRemoveLogo}
                    disabled={isLoading}
                    className='absolute -top-2 -right-2 h-6 w-6 rounded-full p-0'
                  >
                    <X className='h-3 w-3' />
                  </Button>
                </div>
              </div>
            )}
            <div className='space-y-2'>
              <input
                ref={fileInputRef}
                type='file'
                accept='image/png,image/jpeg'
                onChange={handleFileUpload}
                className='hidden'
                id='brand-logo'
              />
              <Button
                type='button'
                variant='outline'
                onClick={() => fileInputRef.current?.click()}
                disabled={isLoading}
                className='w-full max-w-md'
              >
                <Upload className='mr-2 h-4 w-4' />
                {t('system.general.brandLogo.upload')}
              </Button>
              <div className='text-muted-foreground text-sm'>
                {t('system.general.brandLogo.description')}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {hasChanges && (
        <div className='flex justify-end'>
          <Button
            onClick={handleSave}
            disabled={isLoading || updateSettings.isPending}
            className='min-w-[100px]'
          >
            {isLoading || updateSettings.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('system.buttons.saving')}
              </>
            ) : (
              <>
                <Save className='mr-2 h-4 w-4' />
                {t('system.buttons.save')}
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  )
}