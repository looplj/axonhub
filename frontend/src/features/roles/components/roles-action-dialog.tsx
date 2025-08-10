'use client'

import React from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
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
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { useRolesContext } from '../context/roles-context'
import { useCreateRole, useUpdateRole, useDeleteRole, useAllScopes } from '../data/roles'
import { createRoleInputSchema, updateRoleInputSchema } from '../data/schema'
import { ConfirmDialog } from '@/components/confirm-dialog'

// Create Role Dialog
export function CreateRoleDialog() {
  const { t } = useTranslation()
  const { isCreateDialogOpen, setIsCreateDialogOpen } = useRolesContext()
  const { data: scopes = [] } = useAllScopes()
  const createRole = useCreateRole()

  const form = useForm<z.infer<typeof createRoleInputSchema>>({
    resolver: zodResolver(createRoleInputSchema),
    defaultValues: {
      code: '',
      name: '',
      scopes: [],
    },
  })

  const onSubmit = async (values: z.infer<typeof createRoleInputSchema>) => {
    try {
      await createRole.mutateAsync(values)
      setIsCreateDialogOpen(false)
      form.reset()
    } catch (error) {
      // Error is handled by the mutation
    }
  }

  const handleClose = () => {
    setIsCreateDialogOpen(false)
    form.reset()
  }

  return (
    <Dialog open={isCreateDialogOpen} onOpenChange={handleClose}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('roles.dialog.create.title')}</DialogTitle>
          <DialogDescription>
            {t('roles.dialog.create.description')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            <div className='grid grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='code'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('roles.dialog.fields.code.label')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('roles.dialog.fields.code.placeholder')} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('roles.dialog.fields.code.description')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('roles.dialog.fields.name.label')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('roles.dialog.fields.name.placeholder')} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('roles.dialog.fields.name.description')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            
            <FormField
              control={form.control}
              name='scopes'
              render={() => (
                <FormItem>
                  <div className='mb-4'>
                    <FormLabel className='text-base'>{t('roles.dialog.fields.scopes.label')}</FormLabel>
                    <FormDescription>
                      {t('roles.dialog.fields.scopes.description')}
                    </FormDescription>
                  </div>
                  <ScrollArea className='h-[300px] w-full rounded-md border p-4'>
                    <div className='grid grid-cols-1 gap-3'>
                      {scopes.map((scope) => (
                        <FormField
                          key={scope.scope}
                          control={form.control}
                          name='scopes'
                          render={({ field }) => {
                            return (
                              <FormItem
                                key={scope.scope}
                                className='flex flex-row items-start space-x-3 space-y-0'
                              >
                                <FormControl>
                                  <Checkbox
                                    checked={field.value?.includes(scope.scope)}
                                    onCheckedChange={(checked) => {
                                      const currentValue = field.value || []
                                      return checked
                                        ? field.onChange([...currentValue, scope.scope])
                                        : field.onChange(
                                            currentValue.filter(
                                              (value) => value !== scope.scope
                                            )
                                          )
                                    }}
                                  />
                                </FormControl>
                                <div className='space-y-1 leading-none'>
                                  <FormLabel className='font-normal'>
                                    <Badge variant='outline' className='mr-2'>
                                      {scope.scope}
                                    </Badge>
                                    {scope.description}
                                  </FormLabel>
                                </div>
                              </FormItem>
                            )
                          }}
                        />
                      ))}
                    </div>
                  </ScrollArea>
                  <FormMessage />
                </FormItem>
              )}
            />
            
            <DialogFooter>
              <Button type='button' variant='outline' onClick={handleClose}>
                {t('roles.dialog.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={createRole.isPending}>
                {createRole.isPending ? t('roles.dialog.buttons.creating') : t('roles.dialog.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

// Edit Role Dialog
export function EditRoleDialog() {
  const { t } = useTranslation()
  const { editingRole, setEditingRole } = useRolesContext()
  const { data: scopes = [] } = useAllScopes()
  const updateRole = useUpdateRole()

  const form = useForm<z.infer<typeof updateRoleInputSchema>>({
    resolver: zodResolver(updateRoleInputSchema),
    defaultValues: {
      name: '',
      scopes: [],
    },
  })

  React.useEffect(() => {
    if (editingRole) {
      form.reset({
        name: editingRole.name,
        scopes: editingRole.scopes?.map((scope: any) => scope.id) || [],
      })
    }
  }, [editingRole, form])

  const onSubmit = async (values: z.infer<typeof updateRoleInputSchema>) => {
    if (!editingRole) return
    
    try {
      await updateRole.mutateAsync({ id: editingRole.id, input: values })
      setEditingRole(null)
    } catch (error) {
      // Error is handled by the mutation
    }
  }

  const handleClose = () => {
    setEditingRole(null)
    form.reset()
  }

  if (!editingRole) return null

  return (
    <Dialog open={!!editingRole} onOpenChange={handleClose}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('roles.dialog.edit.title')}</DialogTitle>
          <DialogDescription>
            {t('roles.dialog.edit.description')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            <div className='grid grid-cols-2 gap-4'>
              <div>
                <FormLabel>{t('roles.dialog.fields.code.label')}</FormLabel>
                <Input 
                  value={editingRole.code} 
                  disabled 
                  className='bg-muted'
                />
                <FormDescription>
                  {t('roles.dialog.edit.codeNotEditable')}
                </FormDescription>
              </div>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('roles.dialog.fields.name.label')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('roles.dialog.fields.name.placeholder')} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('roles.dialog.fields.name.description')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            
            <FormField
              control={form.control}
              name='scopes'
              render={() => (
                <FormItem>
                  <div className='mb-4'>
                    <FormLabel className='text-base'>{t('roles.dialog.fields.scopes.label')}</FormLabel>
                    <FormDescription>
                      {t('roles.dialog.fields.scopes.description')}
                    </FormDescription>
                  </div>
                  <ScrollArea className='h-[300px] w-full rounded-md border p-4'>
                    <div className='grid grid-cols-1 gap-3'>
                      {scopes.map((scope) => (
                        <FormField
                          key={scope.scope}
                          control={form.control}
                          name='scopes'
                          render={({ field }) => {
                            return (
                              <FormItem
                                key={scope.scope}
                                className='flex flex-row items-start space-x-3 space-y-0'
                              >
                                <FormControl>
                                  <Checkbox
                                    checked={field.value?.includes(scope.scope)}
                                    onCheckedChange={(checked) => {
                                      const currentValue = field.value || []
                                      return checked
                                        ? field.onChange([...currentValue, scope.scope])
                                        : field.onChange(
                                            currentValue.filter(
                                              (value) => value !== scope.scope
                                            )
                                          )
                                    }}
                                  />
                                </FormControl>
                                <div className='space-y-1 leading-none'>
                                  <FormLabel className='font-normal'>
                                    <Badge variant='outline' className='mr-2'>
                                      {scope.scope}
                                    </Badge>
                                    {scope.description}
                                  </FormLabel>
                                </div>
                              </FormItem>
                            )
                          }}
                        />
                      ))}
                    </div>
                  </ScrollArea>
                  <FormMessage />
                </FormItem>
              )}
            />
            
            <DialogFooter>
              <Button type='button' variant='outline' onClick={handleClose}>
                {t('roles.dialog.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={updateRole.isPending}>
                {updateRole.isPending ? t('roles.dialog.buttons.saving') : t('roles.dialog.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

// Delete Role Dialog
export function DeleteRoleDialog() {
  const { t } = useTranslation()
  const { deletingRole, setDeletingRole } = useRolesContext()
  const deleteRole = useDeleteRole()

  const handleConfirm = async () => {
    if (!deletingRole) return
    
    try {
      await deleteRole.mutateAsync(deletingRole.id)
      setDeletingRole(null)
    } catch (error) {
      // Error is handled by the mutation
    }
  }

  return (
    <ConfirmDialog
      open={!!deletingRole}
      onOpenChange={() => setDeletingRole(null)}
      title={t('roles.dialog.delete.title')}
      desc={t('roles.dialog.delete.description', { name: deletingRole?.name })}
      confirmText={t('roles.dialog.buttons.delete')}
      cancelBtnText={t('roles.dialog.buttons.cancel')}
      handleConfirm={handleConfirm}
      isLoading={deleteRole.isPending}
      destructive
    />
  )
}

// Combined Dialogs Component
export function RolesDialogs() {
  return (
    <>
      <CreateRoleDialog />
      <EditRoleDialog />
      <DeleteRoleDialog />
    </>
  )
}