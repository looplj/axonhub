import { createContext, useContext, useState } from 'react'
import { ApiKey } from '../data/schema'

type ApiKeyDialogType = 'create' | 'edit' | 'delete' | 'status' | 'view'

interface ApiKeysContextType {
  selectedApiKey: ApiKey | null
  setSelectedApiKey: (apiKey: ApiKey | null) => void
  isDialogOpen: Record<ApiKeyDialogType, boolean>
  openDialog: (type: ApiKeyDialogType, apiKey?: ApiKey) => void
  closeDialog: (type?: ApiKeyDialogType) => void
}

const ApiKeysContext = createContext<ApiKeysContextType | undefined>(undefined)

export function ApiKeysProvider({ children }: { children: React.ReactNode }) {
  const [selectedApiKey, setSelectedApiKey] = useState<ApiKey | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState<Record<ApiKeyDialogType, boolean>>({
    create: false,
    edit: false,
    delete: false,
    status: false,
    view: false,
  })

  const openDialog = (type: ApiKeyDialogType, apiKey?: ApiKey) => {
    if (apiKey) {
      setSelectedApiKey(apiKey)
    }
    setIsDialogOpen((prev) => ({ ...prev, [type]: true }))
  }

  const closeDialog = (type?: ApiKeyDialogType) => {
    if (type) {
      setIsDialogOpen((prev) => ({ ...prev, [type]: false }))
      if (type === 'delete' || type === 'edit' || type === 'view') {
        setSelectedApiKey(null)
      }
    } else {
      // Close all dialogs
      setIsDialogOpen({
        create: false,
        edit: false,
        delete: false,
        status: false,
        view: false,
      })
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

export default ApiKeysProvider

export function useApiKeysContext() {
  const context = useContext(ApiKeysContext)
  if (context === undefined) {
    throw new Error('useApiKeysContext must be used within a ApiKeysProvider')
  }
  return context
}