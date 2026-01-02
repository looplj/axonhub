import { createContext, useContext, useState, useRef, ReactNode } from 'react'
import { Role } from '../data/schema'

type RoleDialogType = 'create' | 'edit' | 'delete' | 'bulkDelete'

interface RolesContextType {
  editingRole: Role | null
  setEditingRole: (role: Role | null) => void
  deletingRole: Role | null
  setDeletingRole: (role: Role | null) => void
  selectedRoles: Role[]
  setSelectedRoles: (roles: Role[]) => void
  isDialogOpen: Record<RoleDialogType, boolean>
  openDialog: (type: RoleDialogType, role?: Role | Role[]) => void
  closeDialog: (type?: RoleDialogType) => void
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
  const [selectedRoles, setSelectedRoles] = useState<Role[]>([])
  const [isDialogOpen, setIsDialogOpen] = useState<Record<RoleDialogType, boolean>>({
    create: false,
    edit: false,
    delete: false,
    bulkDelete: false,
  })
  const resetRowSelectionRef = useRef<() => void>(() => {})

  const openDialog = (type: RoleDialogType, role?: Role | Role[]) => {
    if (role) {
      if (Array.isArray(role)) {
        setSelectedRoles(role)
      } else {
        setEditingRole(role)
        setDeletingRole(role)
      }
    }
    setIsDialogOpen((prev) => ({ ...prev, [type]: true }))
  }

  const closeDialog = (type?: RoleDialogType) => {
    if (type) {
      setIsDialogOpen((prev) => ({ ...prev, [type]: false }))
      if (type === 'delete' || type === 'edit') {
        setEditingRole(null)
        setDeletingRole(null)
      }
      if (type === 'bulkDelete') {
        setSelectedRoles([])
      }
    } else {
      setIsDialogOpen({
        create: false,
        edit: false,
        delete: false,
        bulkDelete: false,
      })
      setEditingRole(null)
      setDeletingRole(null)
      setSelectedRoles([])
    }
  }

  return (
    <RolesContext.Provider
      value={{
        editingRole,
        setEditingRole,
        deletingRole,
        setDeletingRole,
        selectedRoles,
        setSelectedRoles,
        isDialogOpen,
        openDialog,
        closeDialog,
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