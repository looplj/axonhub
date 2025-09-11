'use client'

import { useState, useEffect, useCallback } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { graphqlRequest } from '@/gql/graphql'
import { ROLES_QUERY, ALL_SCOPES_QUERY } from '@/gql/roles'
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { User, CreateUserInput, UpdateUserInput } from '../data/schema'
import { useCreateUser, useUpdateUser } from '../data/users'

// 创建表单验证模式的工厂函数
const createFormSchema = (t: (key: string) => string) =>
  z
    .object({
      firstName: z.string().min(1, t('users.validation.firstNameRequired')),
      lastName: z.string().min(1, t('users.validation.lastNameRequired')),
      email: z.string().email(t('users.validation.emailInvalid')),
      password: z.string().optional(),
      confirmPassword: z.string().optional(),
      isOwner: z.boolean().optional(),
      roleIDs: z.array(z.string()).optional(),
      scopes: z.array(z.string()).optional(),
    })
    .superRefine((data, ctx) => {
      // 只在创建用户且提供了密码时验证
      if (data.password || data.confirmPassword) {
        if (!data.password || data.password.length < 6) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: t('users.validation.passwordMinLength'),
            path: ['password'],
          })
        }

        if (data.password !== data.confirmPassword) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: t('users.validation.passwordsNotMatch'),
            path: ['confirmPassword'],
          })
        }
      }
    })

interface Role {
  id: string
  name: string
  description?: string
  scopes?: string[]
}

interface ScopeInfo {
  scope: string
  description?: string
}

interface Props {
  currentRow?: User
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function UsersActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const isEdit = !!currentRow
  const [roles, setRoles] = useState<Role[]>([])
  const [allScopes, setAllScopes] = useState<ScopeInfo[]>([])
  const [loading, setLoading] = useState(false)

  const createUser = useCreateUser()
  const updateUser = useUpdateUser()

  // 创建表单验证模式
  const formSchema = createFormSchema(t)
  type UserForm = z.infer<typeof formSchema>

  // 根据是否为编辑模式使用不同的表单配置
  const form = useForm<UserForm>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit
      ? {
          firstName: currentRow.firstName,
          lastName: currentRow.lastName,
          email: currentRow.email,
          password: '',
          confirmPassword: '',
          isOwner: currentRow.isOwner,
          roleIDs: currentRow.roles?.edges?.map((edge) => edge.node.id) || [],
          scopes: currentRow.scopes || [],
        }
      : {
          firstName: '',
          lastName: '',
          email: '',
          password: '',
          confirmPassword: '',
          isOwner: false,
          roleIDs: [],
          scopes: [],
        },
  })

  const loadRolesAndScopes = useCallback(async () => {
    setLoading(true)
    try {
      const [rolesData, scopesData] = await Promise.all([
        graphqlRequest(ROLES_QUERY, { first: 100 }),
        graphqlRequest(ALL_SCOPES_QUERY),
      ])

      // Type the responses properly
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
        }>
      }

      setRoles(rolesResponse.roles.edges.map((edge) => edge.node))
      setAllScopes(scopesResponse.allScopes)
    } catch (error) {
      console.error('Failed to load roles and scopes:', error)
      toast.error(t('common.errors.userLoadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t,setRoles, setAllScopes])

  // Load roles and scopes when dialog opens
  useEffect(() => {
    if (open) {
      loadRolesAndScopes()
    }
  }, [open, loadRolesAndScopes])

  const onSubmit = async (values: UserForm) => {
    try {
      if (isEdit && currentRow) {
        // For updates, we need to calculate role changes
        const currentRoleIDs =
          currentRow.roles?.edges?.map((edge) => edge.node.id) || []
        const newRoleIDs = values.roleIDs || []

        const addRoleIDs = newRoleIDs.filter(
          (id) => !currentRoleIDs.includes(id)
        )
        const removeRoleIDs = currentRoleIDs.filter(
          (id) => !newRoleIDs.includes(id)
        )

        const updateInput: UpdateUserInput = {
          firstName: values.firstName,
          lastName: values.lastName,
          email: values.email,
          isOwner: values.isOwner,
          scopes: values.scopes,
        }

        // Only add role fields if there are changes
        if (addRoleIDs.length > 0) {
          updateInput.addRoleIDs = addRoleIDs
        }
        if (removeRoleIDs.length > 0) {
          updateInput.removeRoleIDs = removeRoleIDs
        }

        await updateUser.mutateAsync({
          id: currentRow.id,
          input: updateInput,
        })
      } else {
        // 创建用户时，移除 confirmPassword 字段
        const createInput: CreateUserInput = {
          firstName: values.firstName,
          lastName: values.lastName,
          email: values.email,
          password: values.password || '',
          // 注意：不包含 confirmPassword
          isOwner: values.isOwner,
          scopes: values.scopes,
          roleIDs: values.roleIDs,
        }

        await createUser.mutateAsync(createInput)
      }

      form.reset()
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to save user:', error)
      toast.error(t('common.errors.userSaveFailed'))
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

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>
            {isEdit
              ? t('users.dialogs.edit.title')
              : t('users.dialogs.add.title')}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('users.dialogs.edit.description')
              : t('users.dialogs.add.description')}
          </DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-y-auto'>
          <Form {...form}>
            <form
              id='user-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className='space-y-6'
            >
              <div className='grid grid-cols-2 gap-4'>
                <FormField
                  control={form.control}
                  name='firstName'
                  render={({ field, fieldState }) => (
                    <FormItem>
                      <FormLabel>{t('users.form.firstName')}</FormLabel>
                      <FormControl>
                        <Input 
                          placeholder='John' 
                          aria-invalid={!!fieldState.error}
                          {...field} 
                        />
                      </FormControl>
                      <div className='min-h-[1.25rem]'>
                        <FormMessage />
                      </div>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='lastName'
                  render={({ field, fieldState }) => (
                    <FormItem>
                      <FormLabel>{t('users.form.lastName')}</FormLabel>
                      <FormControl>
                        <Input 
                          placeholder='Doe' 
                          aria-invalid={!!fieldState.error}
                          {...field} 
                        />
                      </FormControl>
                      <div className='min-h-[1.25rem]'>
                        <FormMessage />
                      </div>
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='email'
                render={({ field, fieldState }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.email')}</FormLabel>
                    <FormControl>
                      <Input 
                        placeholder='john.doe@example.com' 
                        aria-invalid={!!fieldState.error}
                        {...field} 
                      />
                    </FormControl>
                    <div className='min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />

              {/* Password fields - only show when creating new user */}
              {!isEdit && (
                <div className='grid grid-cols-2 gap-4'>
                  <FormField
                    control={form.control}
                    name='password'
                    render={({ field, fieldState }) => (
                      <FormItem>
                        <FormLabel>{t('users.form.password')}</FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder='Enter password'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='confirmPassword'
                    render={({ field, fieldState }) => (
                      <FormItem>
                        <FormLabel>{t('users.form.confirmPassword')}</FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder='Confirm password'
                            aria-invalid={!!fieldState.error}
                            {...field}
                          />
                        </FormControl>
                        <div className='min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />
                </div>
              )}

              <FormField
                control={form.control}
                name='isOwner'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-start space-y-0 space-x-3'>
                    <FormControl>
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <div className='space-y-1 leading-none'>
                      <FormLabel>{t('users.form.isOwner')}</FormLabel>
                      <p className='text-muted-foreground text-sm'>
                        {t('users.form.ownerDescription')}
                      </p>
                    </div>
                  </FormItem>
                )}
              />

              {/* Roles Section */}
              <div className='space-y-3'>
                <FormLabel>{t('users.form.roles')}</FormLabel>
                {loading ? (
                  <div>{t('users.form.loadingRoles')}</div>
                ) : (
                  <div className='grid grid-cols-2 gap-2'>
                    {roles.map((role) => (
                      <div
                        key={role.id}
                        className='flex items-center space-x-2'
                      >
                        <Checkbox
                          id={`role-${role.id}`}
                          checked={(form.watch('roleIDs') || []).includes(
                            role.id
                          )}
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
                <FormLabel>{t('users.form.scopes')}</FormLabel>

                {/* Selected Scopes */}
                <div className='flex flex-wrap gap-2'>
                  {(form.watch('scopes') || []).map((scope) => (
                    <Badge
                      key={scope}
                      variant='secondary'
                      className='flex items-center gap-1'
                    >
                      {scope}
                      <X
                        className='h-3 w-3 cursor-pointer'
                        onClick={() => handleScopeRemove(scope as string)}
                      />
                    </Badge>
                  ))}
                </div>

                {/* Available Scopes */}
                {loading ? (
                  <div>{t('users.form.loadingScopes')}</div>
                ) : (
                  <div className='grid max-h-32 grid-cols-2 gap-2 overflow-y-auto rounded border p-2'>
                    {allScopes.map((scope) => (
                      <div
                        key={scope.scope}
                        className='flex items-start space-x-2'
                      >
                        <Checkbox
                          id={`scope-${scope.scope}`}
                          checked={(form.watch('scopes') || []).includes(
                            scope.scope
                          )}
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
            type='submit'
            form='user-form'
            disabled={createUser.isPending || updateUser.isPending}
          >
            {createUser.isPending || updateUser.isPending
              ? t('users.buttons.saving')
              : t('users.buttons.saveChanges')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
