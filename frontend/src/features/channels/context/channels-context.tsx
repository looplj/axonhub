import React, { useRef, useState } from 'react'
import useDialogState from '@/hooks/use-dialog-state'
import { Channel } from '../data/schema'

type ChannelsDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'settings'
  | 'modelMapping'
  | 'overrides'
  | 'proxy'
  | 'status'
  | 'test'
  | 'bulkImport'
  | 'archive'
  | 'bulkOrdering'
  | 'bulkArchive'
  | 'bulkDisable'
  | 'bulkEnable'
  | 'bulkDelete'
  | 'errorResolved'

interface ChannelsContextType {
  open: ChannelsDialogType | null
  setOpen: (str: ChannelsDialogType | null) => void
  currentRow: Channel | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Channel | null>>
  selectedChannels: Channel[]
  setSelectedChannels: React.Dispatch<React.SetStateAction<Channel[]>>
  resetRowSelection: () => void
  setResetRowSelection: (fn: () => void) => void
}

const ChannelsContext = React.createContext<ChannelsContextType | null>(null)

interface Props {
  children: React.ReactNode
}

export default function ChannelsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ChannelsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Channel | null>(null)
  const [selectedChannels, setSelectedChannels] = useState<Channel[]>([])
  const resetRowSelectionRef = useRef<() => void>(() => {})

  return (
    <ChannelsContext.Provider
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        selectedChannels,
        setSelectedChannels,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn
        },
      }}
    >
      {children}
    </ChannelsContext.Provider>
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
