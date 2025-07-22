import { createContext, useContext, useState, ReactNode } from 'react'
import { Model } from '../data/schema'

type DialogType = 'create' | 'edit' | 'delete'

interface ModelsContextType {
  isDialogOpen: Record<DialogType, boolean>
  selectedModel: Model | null
  openDialog: (type: DialogType, model?: Model) => void
  closeDialog: (type: DialogType) => void
}

const ModelsContext = createContext<ModelsContextType | undefined>(undefined)

interface ModelsProviderProps {
  children: ReactNode
}

export function ModelsProvider({ children }: ModelsProviderProps) {
  const [isDialogOpen, setIsDialogOpen] = useState<Record<DialogType, boolean>>({
    create: false,
    edit: false,
    delete: false,
  })
  const [selectedModel, setSelectedModel] = useState<Model | null>(null)

  const openDialog = (type: DialogType, model?: Model) => {
    setIsDialogOpen(prev => ({ ...prev, [type]: true }))
    if (model) {
      setSelectedModel(model)
    }
  }

  const closeDialog = (type: DialogType) => {
    setIsDialogOpen(prev => ({ ...prev, [type]: false }))
    if (type !== 'create') {
      setSelectedModel(null)
    }
  }

  return (
    <ModelsContext.Provider
      value={{
        isDialogOpen,
        selectedModel,
        openDialog,
        closeDialog,
      }}
    >
      {children}
    </ModelsContext.Provider>
  )
}

export function useModelsContext() {
  const context = useContext(ModelsContext)
  if (context === undefined) {
    throw new Error('useModelsContext must be used within a ModelsProvider')
  }
  return context
}