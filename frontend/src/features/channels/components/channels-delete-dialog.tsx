'use client'

import { useState } from 'react'
import { IconAlertTriangle } from '@tabler/icons-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Channel } from '../data/schema'
import { useDeleteChannel } from '../data/channels'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

export function ChannelsDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const [value, setValue] = useState('')
  const deleteChannel = useDeleteChannel()

  const handleDelete = async () => {
    if (value.trim() !== currentRow.name) return

    try {
      await deleteChannel.mutateAsync(currentRow.id)
      onOpenChange(false)
      setValue('')
    } catch (error) {
      console.error('Failed to delete channel:', error)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(state) => {
        if (!state) setValue('')
        onOpenChange(state)
      }}
      handleConfirm={handleDelete}
      disabled={value.trim() !== currentRow.name || deleteChannel.isPending}
      title={
        <span className='text-destructive'>
          <IconAlertTriangle
            className='stroke-destructive mr-1 inline-block'
            size={18}
          />{' '}
          删除 Channel
        </span>
      }
      desc={
        <>
          <Alert variant='destructive'>
            <IconAlertTriangle className='h-4 w-4' />
            <AlertTitle>警告</AlertTitle>
            <AlertDescription>
              此操作无法撤销。这将永久删除 Channel 及其所有相关数据。
            </AlertDescription>
          </Alert>
          <div className='space-y-2'>
            <Label htmlFor='channel-name'>
              请输入 Channel 名称 <strong>{currentRow.name}</strong> 以确认删除：
            </Label>
            <Input
              id='channel-name'
              placeholder={currentRow.name}
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
          </div>
        </>
      }
      confirmText={deleteChannel.isPending ? '删除中...' : '删除 Channel'}
    />
  )
}