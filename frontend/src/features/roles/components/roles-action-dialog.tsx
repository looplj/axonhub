'use client';

import React from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useAllScopes } from '@/gql/scopes';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { filterGrantableScopes } from '@/lib/permission-utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useRolesContext } from '../context/roles-context';
import { useCreateRole, useUpdateRole, useDeleteRole, useBulkDeleteRoles } from '../data/roles';
import { createRoleInputSchema, updateRoleInputSchema } from '../data/schema';

// Create Role Dialog
export function CreateRoleDialog() {
  const { t } = useTranslation();
  const currentUser = useAuthStore((state) => state.auth.user);
  const { isDialogOpen, closeDialog } = useRolesContext();
  const { data: allScopes = [] } = useAllScopes();
  const createRole = useCreateRole();

  // 过滤当前用户可以授予的权限
  const scopes = allScopes.filter((scope) => filterGrantableScopes(currentUser, [scope.scope]).includes(scope.scope));

  const form = useForm<z.infer<typeof createRoleInputSchema>>({
    resolver: zodResolver(createRoleInputSchema),
    defaultValues: {
      name: '',
      scopes: [],
    },
  });

  const onSubmit = async (values: z.infer<typeof createRoleInputSchema>) => {
    try {
      await createRole.mutateAsync(values);
      closeDialog('create');
      form.reset();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  const handleClose = () => {
    closeDialog('create');
    form.reset();
  };

  return (
    <Dialog open={isDialogOpen.create} onOpenChange={handleClose}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('roles.dialogs.create.title')}</DialogTitle>
          <DialogDescription>{t('roles.dialogs.create.description')}</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            <FormField
              control={form.control}
              name='name'
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel>{t('roles.dialogs.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('roles.dialogs.fields.name.placeholder')} aria-invalid={!!fieldState.error} {...field} />
                  </FormControl>
                  <FormDescription>{t('roles.dialogs.fields.name.description')}</FormDescription>
                  <div className='min-h-[1.25rem]'>
                    <FormMessage />
                  </div>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='scopes'
              render={() => (
                <FormItem>
                  <div className='mb-4'>
                    <FormLabel className='text-base'>{t('roles.dialogs.fields.scopes.label')}</FormLabel>
                    <FormDescription>{t('roles.dialogs.fields.scopes.description')}</FormDescription>
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
                              <FormItem key={scope.scope} className='flex flex-row items-start space-y-0 space-x-3'>
                                <FormControl>
                                  <Checkbox
                                    checked={field.value?.includes(scope.scope)}
                                    onCheckedChange={(checked) => {
                                      const currentValue = field.value || [];
                                      return checked
                                        ? field.onChange([...currentValue, scope.scope])
                                        : field.onChange(currentValue.filter((value) => value !== scope.scope));
                                    }}
                                  />
                                </FormControl>
                                <div className='space-y-1 leading-none'>
                                  <FormLabel className='font-normal'>
                                    <Badge variant='outline' className='mr-2'>
                                      {scope.scope}
                                    </Badge>
                                    {t(`scopes.${scope.scope}`)}
                                  </FormLabel>
                                </div>
                              </FormItem>
                            );
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
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={createRole.isPending}>
                {createRole.isPending ? t('common.buttons.creating') : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

// Edit Role Dialog
export function EditRoleDialog() {
  const { t } = useTranslation();
  const currentUser = useAuthStore((state) => state.auth.user);
  const { editingRole, isDialogOpen, closeDialog } = useRolesContext();
  const { data: allScopes = [] } = useAllScopes();
  const updateRole = useUpdateRole();

  // 过滤当前用户可以授予的权限
  const scopes = allScopes.filter((scope) => filterGrantableScopes(currentUser, [scope.scope]).includes(scope.scope));

  const form = useForm<z.infer<typeof updateRoleInputSchema>>({
    resolver: zodResolver(updateRoleInputSchema),
    defaultValues: {
      name: '',
      scopes: [],
    },
  });

  React.useEffect(() => {
    if (editingRole) {
      form.reset({
        name: editingRole.name,
        scopes: editingRole.scopes?.map((scope: string) => scope) || [],
      });
    }
  }, [editingRole, form]);

  const onSubmit = async (values: z.infer<typeof updateRoleInputSchema>) => {
    if (!editingRole) return;

    try {
      await updateRole.mutateAsync({ id: editingRole.id, input: values });
      closeDialog('edit');
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  const handleClose = () => {
    closeDialog('edit');
    form.reset();
  };

  if (!editingRole) return null;

  return (
    <Dialog open={isDialogOpen.edit} onOpenChange={handleClose}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('roles.dialogs.edit.title')}</DialogTitle>
          <DialogDescription>{t('roles.dialogs.edit.description')}</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            <FormField
              control={form.control}
              name='name'
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel>{t('roles.dialogs.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('roles.dialogs.fields.name.placeholder')} aria-invalid={!!fieldState.error} {...field} />
                  </FormControl>
                  <FormDescription>{t('roles.dialogs.fields.name.description')}</FormDescription>
                  <div className='min-h-[1.25rem]'>
                    <FormMessage />
                  </div>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='scopes'
              render={() => (
                <FormItem>
                  <div className='mb-4'>
                    <FormLabel className='text-base'>{t('roles.dialogs.fields.scopes.label')}</FormLabel>
                    <FormDescription>{t('roles.dialogs.fields.scopes.description')}</FormDescription>
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
                              <FormItem key={scope.scope} className='flex flex-row items-start space-y-0 space-x-3'>
                                <FormControl>
                                  <Checkbox
                                    checked={field.value?.includes(scope.scope)}
                                    onCheckedChange={(checked) => {
                                      const currentValue = field.value || [];
                                      return checked
                                        ? field.onChange([...currentValue, scope.scope])
                                        : field.onChange(currentValue.filter((value) => value !== scope.scope));
                                    }}
                                  />
                                </FormControl>
                                <div className='space-y-1 leading-none'>
                                  <FormLabel className='font-normal'>
                                    <Badge variant='outline' className='mr-2'>
                                      {scope.scope}
                                    </Badge>
                                    {t(`scopes.${scope.scope}`)}
                                  </FormLabel>
                                </div>
                              </FormItem>
                            );
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
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={updateRole.isPending}>
                {updateRole.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

// Delete Role Dialog
export function DeleteRoleDialog() {
  const { t } = useTranslation();
  const { deletingRole, isDialogOpen, closeDialog } = useRolesContext();
  const deleteRole = useDeleteRole();

  const handleConfirm = async () => {
    if (!deletingRole) return;

    try {
      await deleteRole.mutateAsync(deletingRole.id);
      closeDialog('delete');
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.delete}
      onOpenChange={() => closeDialog('delete')}
      title={t('roles.dialogs.delete.title')}
      desc={t('roles.dialogs.delete.description', { name: deletingRole?.name })}
      confirmText={t('common.buttons.delete')}
      cancelBtnText={t('common.buttons.cancel')}
      handleConfirm={handleConfirm}
      isLoading={deleteRole.isPending}
      destructive
    />
  );
}

// Bulk Delete Roles Dialog
export function BulkDeleteRolesDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedRoles, resetRowSelection } = useRolesContext();
  const bulkDeleteRoles = useBulkDeleteRoles();

  const handleConfirm = async () => {
    if (selectedRoles.length === 0) return;

    try {
      const ids = selectedRoles.map((role) => role.id);
      await bulkDeleteRoles.mutateAsync(ids);
      resetRowSelection();
      closeDialog('bulkDelete');
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.bulkDelete}
      onOpenChange={() => closeDialog('bulkDelete')}
      title={t('roles.dialogs.bulkDelete.title')}
      desc={t('roles.dialogs.bulkDelete.description', { count: selectedRoles.length })}
      confirmText={t('common.buttons.delete')}
      cancelBtnText={t('common.buttons.cancel')}
      handleConfirm={handleConfirm}
      isLoading={bulkDeleteRoles.isPending}
      destructive
    />
  );
}

// Combined Dialogs Component
export function RolesDialogs() {
  return (
    <>
      <CreateRoleDialog />
      <EditRoleDialog />
      <DeleteRoleDialog />
      <BulkDeleteRolesDialog />
    </>
  );
}
