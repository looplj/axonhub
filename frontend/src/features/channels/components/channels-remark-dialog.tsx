'use client'

import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { useChannels } from '../context/channels-context'
import { useUpdateChannel } from '../data/channels'

export function ChannelsRemarkDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, setCurrentRow } = useChannels()
  const updateChannel = useUpdateChannel()
  const [remark, setRemark] = useState('')

  const isOpen = open === 'remark' && !!currentRow

  useEffect(() => {
    if (isOpen && currentRow) {
      setRemark(currentRow.remark || '')
    }
  }, [isOpen, currentRow])

  const handleClose = useCallback(() => {
    setOpen(null)
    setTimeout(() => {
      setCurrentRow(null)
      setRemark('')
    }, 500)
  }, [setOpen, setCurrentRow])

  const handleSave = useCallback(async () => {
    if (!currentRow) return

    try {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          remark: remark.trim() || null,
        },
      })
      handleClose()
    } catch (error) {
      // Error is handled by the mutation
    }
  }, [currentRow, remark, updateChannel, handleClose])

  if (!currentRow) return null

  return (
    <Dialog open={isOpen} onOpenChange={(state) => !state && handleClose()}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('channels.dialogs.remark.title')}</DialogTitle>
          <DialogDescription>
            {t('channels.dialogs.remark.description', { name: currentRow.name })}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label htmlFor='remark'>{t('channels.dialogs.remark.label')}</Label>
            <Textarea
              id='remark'
              value={remark}
              onChange={(e) => setRemark(e.target.value)}
              placeholder={t('channels.dialogs.remark.placeholder')}
              className='min-h-[120px] resize-y'
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={handleClose}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={updateChannel.isPending}>
            {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
