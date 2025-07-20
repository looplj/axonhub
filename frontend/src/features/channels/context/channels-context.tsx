import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog-state'
import { Channel } from '../data/schema'

type ChannelsDialogType = 'add' | 'edit' | 'delete' | 'settings'

interface ChannelsContextType {
  open: ChannelsDialogType | null
  setOpen: (str: ChannelsDialogType | null) => void
  currentRow: Channel | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Channel | null>>
}

const ChannelsContext = React.createContext<ChannelsContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export default function ChannelsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ChannelsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Channel | null>(null)

  return (
    <ChannelsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </ChannelsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useChannels = () => {
  const channelsContext = React.useContext(ChannelsContext)

  if (!channelsContext) {
    throw new Error('useChannels has to be used within <ChannelsContext>')
  }

  return channelsContext
}