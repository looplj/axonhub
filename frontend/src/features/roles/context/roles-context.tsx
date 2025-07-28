import { createContext, useContext, useState, ReactNode } from 'react'
import { Role } from '../data/schema'

interface RolesContextType {
  editingRole: Role | null
  setEditingRole: (role: Role | null) => void
  deletingRole: Role | null
  setDeletingRole: (role: Role | null) => void
  isCreateDialogOpen: boolean
  setIsCreateDialogOpen: (open: boolean) => void
}

const RolesContext = createContext<RolesContextType | undefined>(undefined)

export function useRolesContext() {
  const context = useContext(RolesContext)
  if (!context) {
    throw new Error('useRolesContext must be used within a RolesProvider')
  }
  return context
}

interface RolesProviderProps {
  children: ReactNode
}

export default function RolesProvider({ children }: RolesProviderProps) {
  const [editingRole, setEditingRole] = useState<Role | null>(null)
  const [deletingRole, setDeletingRole] = useState<Role | null>(null)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)

  return (
    <RolesContext.Provider
      value={{
        editingRole,
        setEditingRole,
        deletingRole,
        setDeletingRole,
        isCreateDialogOpen,
        setIsCreateDialogOpen,
      }}
    >
      {children}
    </RolesContext.Provider>
  )
}