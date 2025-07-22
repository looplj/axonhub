import { createContext, useContext, useState } from 'react'
import { ApiKey } from '../data/schema'

type ApiKeyDialogType = 'create' | 'edit' | 'delete'

interface ApiKeysContextType {
  selectedApiKey: ApiKey | null
  setSelectedApiKey: (apiKey: ApiKey | null) => void
  isDialogOpen: Record<ApiKeyDialogType, boolean>
  openDialog: (type: ApiKeyDialogType, apiKey?: ApiKey) => void
  closeDialog: (type: ApiKeyDialogType) => void
}

const ApiKeysContext = createContext<ApiKeysContextType | undefined>(undefined)

export function ApiKeysProvider({ children }: { children: React.ReactNode }) {
  const [selectedApiKey, setSelectedApiKey] = useState<ApiKey | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState<Record<ApiKeyDialogType, boolean>>({
    create: false,
    edit: false,
    delete: false,
  })

  const openDialog = (type: ApiKeyDialogType, apiKey?: ApiKey) => {
    if (apiKey) {
      setSelectedApiKey(apiKey)
    }
    setIsDialogOpen((prev) => ({ ...prev, [type]: true }))
  }

  const closeDialog = (type: ApiKeyDialogType) => {
    setIsDialogOpen((prev) => ({ ...prev, [type]: false }))
    if (type === 'delete' || type === 'edit') {
      setSelectedApiKey(null)
    }
  }

  return (
    <ApiKeysContext.Provider
      value={{
        selectedApiKey,
        setSelectedApiKey,
        isDialogOpen,
        openDialog,
        closeDialog,
      }}
    >
      {children}
    </ApiKeysContext.Provider>
  )
}

export function useApiKeysContext() {
  const context = useContext(ApiKeysContext)
  if (context === undefined) {
    throw new Error('useApiKeysContext must be used within a ApiKeysProvider')
  }
  return context
}