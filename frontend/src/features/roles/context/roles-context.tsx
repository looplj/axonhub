import { createContext, useContext, useState, useRef, ReactNode } from 'react'
import { Role } from '../data/schema'

interface RolesContextType {
  editingRole: Role | null
  setEditingRole: (role: Role | null) => void
  deletingRole: Role | null
  setDeletingRole: (role: Role | null) => void
  deletingRoles: Role[]
  setDeletingRoles: (roles: Role[]) => void
  isCreateDialogOpen: boolean
  setIsCreateDialogOpen: (open: boolean) => void
  resetRowSelection: () => void
  setResetRowSelection: (fn: () => void) => void
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
  const [deletingRoles, setDeletingRoles] = useState<Role[]>([])
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const resetRowSelectionRef = useRef<() => void>(() => {})

  return (
    <RolesContext.Provider
      value={{
        editingRole,
        setEditingRole,
        deletingRole,
        setDeletingRole,
        deletingRoles,
        setDeletingRoles,
        isCreateDialogOpen,
        setIsCreateDialogOpen,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn
        },
      }}
    >
      {children}
    </RolesContext.Provider>
  )
}