import React, { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks/use-debounce'
import { usePermissions } from '@/hooks/usePermissions'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { createColumns } from './components/users-columns'
import { UsersDialogs } from './components/users-dialogs'
import { UsersPrimaryButtons } from './components/users-primary-buttons'
import { UsersTable } from './components/users-table'
import UsersProvider from './context/users-context'
import { useUsers } from './data/users'

function UsersContent() {
  const { t } = useTranslation()
  const { userPermissions } = usePermissions()

  // Filter states
  const [nameFilter, setNameFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [roleFilter, setRoleFilter] = useState<string[]>([])

  const debouncedNameFilter = useDebounce(nameFilter, 300)

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(
    () => createColumns(t, userPermissions.canWrite),
    [t, userPermissions.canWrite]
  )

  // Fetch all project users (no server-side filtering for project users)
  const { data, isLoading, error: _error } = useUsers()

  // Apply client-side filtering
  const filteredData = React.useMemo(() => {
    if (!data?.edges) return []
    
    let filtered = data.edges.map(edge => edge.node)
    
    // Filter by name (firstName or lastName)
    if (debouncedNameFilter) {
      const searchLower = debouncedNameFilter.toLowerCase()
      filtered = filtered.filter(user => {
        const firstName = user.firstName?.toLowerCase() || ''
        const lastName = user.lastName?.toLowerCase() || ''
        const email = user.email?.toLowerCase() || ''
        return firstName.includes(searchLower) || 
               lastName.includes(searchLower) || 
               email.includes(searchLower)
      })
    }
    
    // Filter by status
    if (statusFilter.length > 0) {
      filtered = filtered.filter(user => statusFilter.includes(user.status))
    }
    
    // Filter by role (if needed in the future)
    if (roleFilter.length > 0) {
      // Note: This would need to be implemented based on the actual user role relationship
      // For now, we'll leave it as a placeholder
    }
    
    return filtered
  }, [data, debouncedNameFilter, statusFilter, roleFilter])

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <UsersTable
        data={filteredData}
        columns={columns}
        loading={isLoading}
        nameFilter={nameFilter}
        statusFilter={statusFilter}
        roleFilter={roleFilter}
        onNameFilterChange={setNameFilter}
        onStatusFilterChange={setStatusFilter}
        onRoleFilterChange={setRoleFilter}
      />
      </div>
  )
}

export default function UsersManagement() {
  const { t } = useTranslation()

  return (
    <UsersProvider>
      <Header fixed>
        {/* <Search /> */}
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>
              {t('projectUsers.title')}
            </h2>
            <p className='text-muted-foreground'>{t('projectUsers.description')}</p>
          </div>
          <UsersPrimaryButtons />
        </div>
        <UsersContent />
      </Main>
      <UsersDialogs />
    </UsersProvider>
  )
}
