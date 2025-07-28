'use client'

import React from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
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
          <DialogTitle>新建角色</DialogTitle>
          <DialogDescription>
            创建一个新的角色并配置其权限范围。
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
                    <FormLabel>角色代码</FormLabel>
                    <FormControl>
                      <Input placeholder='admin' {...field} />
                    </FormControl>
                    <FormDescription>
                      唯一标识符，只能包含字母、数字和下划线
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
                    <FormLabel>角色名称</FormLabel>
                    <FormControl>
                      <Input placeholder='管理员' {...field} />
                    </FormControl>
                    <FormDescription>
                      用户友好的角色名称
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
                    <FormLabel className='text-base'>权限范围</FormLabel>
                    <FormDescription>
                      选择此角色拥有的权限
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
                                      return checked
                                        ? field.onChange([...field.value, scope.scope])
                                        : field.onChange(
                                            field.value?.filter(
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
                取消
              </Button>
              <Button type='submit' disabled={createRole.isPending}>
                {createRole.isPending ? '创建中...' : '创建角色'}
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
  const { editingRole, setEditingRole } = useRolesContext()
  const { data: scopes = [] } = useAllScopes()
  const updateRole = useUpdateRole()

  const form = useForm<z.infer<typeof updateRoleInputSchema>>({
    resolver: zodResolver(updateRoleInputSchema),
    defaultValues: {
      name: editingRole?.name || '',
      scopes: editingRole?.scopes || [],
    },
  })

  // Update form when editingRole changes
  React.useEffect(() => {
    if (editingRole) {
      form.reset({
        name: editingRole.name,
        scopes: editingRole.scopes,
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
          <DialogTitle>编辑角色</DialogTitle>
          <DialogDescription>
            修改角色信息和权限配置。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            <div className='grid grid-cols-2 gap-4'>
              <div>
                <FormLabel>角色代码</FormLabel>
                <Input value={editingRole.code} disabled />
                <FormDescription>
                  角色代码不可修改
                </FormDescription>
              </div>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>角色名称</FormLabel>
                    <FormControl>
                      <Input placeholder='管理员' {...field} />
                    </FormControl>
                    <FormDescription>
                      用户友好的角色名称
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
                    <FormLabel className='text-base'>权限范围</FormLabel>
                    <FormDescription>
                      选择此角色拥有的权限
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
                                      return checked
                                        ? field.onChange([...field.value, scope.scope])
                                        : field.onChange(
                                            field.value?.filter(
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
                取消
              </Button>
              <Button type='submit' disabled={updateRole.isPending}>
                {updateRole.isPending ? '保存中...' : '保存更改'}
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
      title='删除角色'
      desc={`确定要删除角色 "${deletingRole?.name}" 吗？此操作无法撤销。`}
      confirmText='删除角色'
      cancelBtnText='取消'
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