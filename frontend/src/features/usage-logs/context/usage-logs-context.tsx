'use client'

import { createContext, useContext, useState, ReactNode } from 'react'
import { UsageLog } from '../data/schema'

interface UsageLogsContextType {
  // Dialog states
  detailDialogOpen: boolean
  setDetailDialogOpen: (open: boolean) => void
  
  // Current selected items
  currentUsageLog: UsageLog | null
  setCurrentUsageLog: (usageLog: UsageLog | null) => void
  
  // Table selection
  selectedUsageLogs: string[]
  setSelectedUsageLogs: (ids: string[]) => void
}

const UsageLogsContext = createContext<UsageLogsContextType | undefined>(undefined)

interface UsageLogsProviderProps {
  children: ReactNode
}

export function UsageLogsProvider({ children }: UsageLogsProviderProps) {
  const [detailDialogOpen, setDetailDialogOpen] = useState(false)
  const [currentUsageLog, setCurrentUsageLog] = useState<UsageLog | null>(null)
  const [selectedUsageLogs, setSelectedUsageLogs] = useState<string[]>([])

  return (
    <UsageLogsContext.Provider
      value={{
        detailDialogOpen,
        setDetailDialogOpen,
        currentUsageLog,
        setCurrentUsageLog,
        selectedUsageLogs,
        setSelectedUsageLogs,
      }}
    >
      {children}
    </UsageLogsContext.Provider>
  )
}

export function useUsageLogsContext() {
  const context = useContext(UsageLogsContext)
  if (!context) {
    throw new Error('useUsageLogsContext must be used within a UsageLogsProvider')
  }
  return context
}