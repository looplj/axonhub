'use client'

import { useState, useEffect, useCallback, useMemo } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { graphqlRequest } from '@/gql/graphql'
import { ROLES_QUERY, ALL_SCOPES_QUERY } from '@/gql/roles'
import { X, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSelectedProjectId } from '@/stores/projectStore'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useAddUserToProject, useAllUsers } from '../data/users'

interface Role {
  id: string
  name: string
  description?: string
  scopes?: string[]
}

interface ScopeInfo {
  scope: string
  description?: string
  levels?: string[]
}

const formSchema = z.object({
  userId: z.string().min(1, 'Please select a user'),
  isOwner: z.boolean().optional(),
  roleIDs: z.array(z.string()).optional(),
  scopes: z.array(z.string()).optional(),
})

type AddUserForm = z.infer<typeof formSchema>

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function UsersAddToProjectDialog({ open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const selectedProjectId = useSelectedProjectId()
  const [roles, setRoles] = useState<Role[]>([])
  const [allScopes, setAllScopes] = useState<ScopeInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')

  const addUserToProject = useAddUserToProject()

  // Fetch all users - only when dialog is open
  const { data: usersData, isLoading: usersLoading } = useAllUsers(
    {
      first: 100,
      where: searchTerm ? { emailContainsFold: searchTerm } : undefined,
    },
    { enabled: open }
  )

  const form = useForm<AddUserForm>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      userId: '',
      isOwner: false,
      roleIDs: [],
      scopes: [],
    },
  })

  const loadRolesAndScopes = useCallback(async () => {
    if (!selectedProjectId) return

    setLoading(true)
    try {
      const [rolesData, scopesData] = await Promise.all([
        graphqlRequest(ROLES_QUERY, {
          first: 100,
          where: { projectID: selectedProjectId },
        }),
        graphqlRequest(ALL_SCOPES_QUERY, { level: 'project' }),
      ])

      const rolesResponse = rolesData as {
        roles: {
          edges: Array<{
            node: {
              id: string
              name: string
              description?: string
              scopes?: string[]
            }
          }>
        }
      }

      const scopesResponse = scopesData as {
        allScopes: Array<{
          scope: string
          description?: string
          levels?: string[]
        }>
      }

      setRoles(rolesResponse.roles.edges.map((edge) => edge.node))
      setAllScopes(scopesResponse.allScopes)
    } catch (error) {
      console.error('Failed to load roles and scopes:', error)
      toast.error(t('common.errors.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t, selectedProjectId])

  useEffect(() => {
    if (open) {
      loadRolesAndScopes()
    }
  }, [open, loadRolesAndScopes])

  const onSubmit = async (values: AddUserForm) => {
    try {
      await addUserToProject.mutateAsync({
        userId: values.userId,
        isOwner: values.isOwner,
        scopes: values.scopes,
        roleIDs: values.roleIDs,
      })

      form.reset()
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to add user to project:', error)
      toast.error(t('users.messages.addToProjectError'))
    }
  }

  const handleRoleToggle = (roleId: string) => {
    const currentRoles = form.getValues('roleIDs') || []
    const newRoles = currentRoles.includes(roleId)
      ? currentRoles.filter((id: string) => id !== roleId)
      : [...currentRoles, roleId]
    form.setValue('roleIDs', newRoles)
  }

  const handleScopeToggle = (scopeName: string) => {
    const currentScopes = form.getValues('scopes') || []
    const newScopes = currentScopes.includes(scopeName)
      ? currentScopes.filter((name: string) => name !== scopeName)
      : [...currentScopes, scopeName]
    form.setValue('scopes', newScopes)
  }

  const handleScopeRemove = (scopeName: string) => {
    const currentScopes = form.getValues('scopes') || []
    const newScopes = currentScopes.filter((name: string) => name !== scopeName)
    form.setValue('scopes', newScopes)
  }

  const availableUsers = usersData?.edges?.map((edge) => edge.node) || []

  // Get selected user for display in title
  const selectedUser = useMemo(() => {
    const userId = form.watch('userId')
    if (!userId) return null
    return availableUsers.find((user) => user.id === userId)
  }, [form.watch('userId'), availableUsers])

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
          setSearchTerm('')
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>
            {selectedUser
              ? `Add ${selectedUser.firstName} ${selectedUser.lastName} to a project with specific roles and permissions.`
              : t('users.dialogs.addToProject.title')}
          </DialogTitle>
          <DialogDescription>{t('users.dialogs.addToProject.description')}</DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-y-auto'>
          <Form {...form}>
            <form id='add-user-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
              {/* User Selection */}
              <FormField
                control={form.control}
                name='userId'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.selectUser')}</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('users.form.selectUserPlaceholder')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <div className='flex items-center border-b px-3 pb-2'>
                          <Search className='mr-2 h-4 w-4 shrink-0 opacity-50' />
                          <Input
                            placeholder={t('users.form.searchUsers')}
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className='h-8 border-0 p-0 focus-visible:ring-0'
                          />
                        </div>
                        {usersLoading ? (
                          <div className='text-muted-foreground p-2 text-center text-sm'>{t('common.loading')}</div>
                        ) : availableUsers.length === 0 ? (
                          <div className='text-muted-foreground p-2 text-center text-sm'>
                            {t('users.form.noUsersFound')}
                          </div>
                        ) : (
                          availableUsers.map((user) => (
                            <SelectItem key={user.id} value={user.id}>
                              {user.firstName} {user.lastName} ({user.email})
                            </SelectItem>
                          ))
                        )}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Project Owner Checkbox */}
              <FormField
                control={form.control}
                name='isOwner'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-start space-y-0 space-x-3'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                    <div className='space-y-1 leading-none'>
                      <FormLabel>{t('users.form.isOwner')}</FormLabel>
                      <p className='text-muted-foreground text-sm'>{t('users.form.ownerDescription')}</p>
                    </div>
                  </FormItem>
                )}
              />

              {/* Roles Section */}
              <div className='space-y-3'>
                <FormLabel>{t('users.form.projectRoles')}</FormLabel>
                {loading ? (
                  <div>{t('users.form.loadingRoles')}</div>
                ) : roles.length === 0 ? (
                  <div className='text-muted-foreground text-sm'>{t('users.form.noProjectRoles')}</div>
                ) : (
                  <div className='grid grid-cols-2 gap-2'>
                    {roles.map((role) => (
                      <div key={role.id} className='flex items-center space-x-2'>
                        <Checkbox
                          id={`role-${role.id}`}
                          checked={(form.watch('roleIDs') || []).includes(role.id)}
                          onCheckedChange={() => handleRoleToggle(role.id)}
                        />
                        <label
                          htmlFor={`role-${role.id}`}
                          className='text-sm leading-none font-medium peer-disabled:cursor-not-allowed peer-disabled:opacity-70'
                        >
                          {role.name}
                        </label>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Scopes Section */}
              <div className='space-y-3'>
                <FormLabel>{t('users.form.projectScopes')}</FormLabel>

                {/* Selected Scopes */}
                <div className='flex flex-wrap gap-2'>
                  {(form.watch('scopes') || []).map((scope) => (
                    <Badge key={scope} variant='secondary' className='flex items-center gap-1'>
                      {scope}
                      <X className='h-3 w-3 cursor-pointer' onClick={() => handleScopeRemove(scope as string)} />
                    </Badge>
                  ))}
                </div>

                {/* Available Scopes */}
                {loading ? (
                  <div>{t('users.form.loadingScopes')}</div>
                ) : (
                  <div className='grid max-h-32 grid-cols-2 gap-2 overflow-y-auto rounded border p-2'>
                    {allScopes.map((scope) => (
                      <div key={scope.scope} className='flex items-start space-x-2'>
                        <Checkbox
                          id={`scope-${scope.scope}`}
                          checked={(form.watch('scopes') || []).includes(scope.scope)}
                          onCheckedChange={() => handleScopeToggle(scope.scope)}
                        />
                        <div className='space-y-1 leading-none'>
                          <label
                            htmlFor={`scope-${scope.scope}`}
                            className='text-sm leading-none font-medium peer-disabled:cursor-not-allowed peer-disabled:opacity-70'
                          >
                            <Badge variant='outline' className='mr-2'>
                              {scope.scope}
                            </Badge>
                            {t(`scopes.${scope.scope}`)}
                          </label>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </form>
          </Form>
        </div>

        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => {
              form.reset()
              onOpenChange(false)
            }}
          >
            {t('common.cancel')}
          </Button>
          <Button type='submit' form='add-user-form' disabled={addUserToProject.isPending}>
            {addUserToProject.isPending ? t('users.buttons.adding') : t('users.buttons.addToProject')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
