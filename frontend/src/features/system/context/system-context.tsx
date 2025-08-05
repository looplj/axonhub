'use client'

import React, { createContext, useContext, useState } from 'react'

interface SystemContextType {
  isLoading: boolean
  setIsLoading: (loading: boolean) => void
}

const SystemContext = createContext<SystemContextType | undefined>(undefined)

export function useSystemContext() {
  const context = useContext(SystemContext)
  if (!context) {
    throw new Error('useSystemContext must be used within a SystemProvider')
  }
  return context
}

interface SystemProviderProps {
  children: React.ReactNode
}

export default function SystemProvider({ children }: SystemProviderProps) {
  const [isLoading, setIsLoading] = useState(false)

  const value: SystemContextType = {
    isLoading,
    setIsLoading,
  }

  return (
    <SystemContext.Provider value={value}>
      {children}
    </SystemContext.Provider>
  )
}