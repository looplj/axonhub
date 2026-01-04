'use client';

import React from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { ScopesSelect } from '@/components/ui/scopes-select';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useRolesContext } from '../context/roles-context';
import { useCreateRole, useUpdateRole, useDeleteRole } from '../data/roles';
import { createRoleInputSchema, updateRoleInputSchema } from '../data/schema';

// Create Role Dialog
export function CreateRoleDialog() {
  const { t } = useTranslation();
  const { isCreateDialogOpen, setIsCreateDialogOpen } = useRolesContext();
  const createRole = useCreateRole();
  const selectedProjectId = useSelectedProjectId();
  const [dialogContent, setDialogContent] = React.useState<HTMLDivElement | null>(null);

  const form = useForm<z.infer<typeof createRoleInputSchema>>({
    resolver: zodResolver(createRoleInputSchema),
    defaultValues: {
      projectID: selectedProjectId || '',
      name: '',
      scopes: [],
    },
  });

  // Update projectID when selectedProjectId changes
  React.useEffect(() => {
    if (selectedProjectId) {
      form.setValue('projectID', selectedProjectId);
    }
  }, [selectedProjectId, form]);

  const onSubmit = async (values: z.infer<typeof createRoleInputSchema>) => {
    try {
      await createRole.mutateAsync(values);
      setIsCreateDialogOpen(false);
      form.reset();
    } catch (error) {
      // Error is handled by the mutation
    }
  };

  const handleClose = () => {
    setIsCreateDialogOpen(false);
    form.reset();
  };

  return (
    <Dialog open={isCreateDialogOpen} onOpenChange={handleClose}>
      <DialogContent className='max-w-2xl' ref={setDialogContent}>
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
              render={({ field }) => (
                <FormItem>
                  <div className='mb-4'>
                    <FormLabel className='text-base'>{t('roles.dialogs.fields.scopes.label')}</FormLabel>
                    <FormDescription>{t('roles.dialogs.fields.scopes.description')}</FormDescription>
                  </div>
                  <FormControl>
                    <ScopesSelect value={field.value || []} onChange={field.onChange} portalContainer={dialogContent} />
                  </FormControl>
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
  const { editingRole, setEditingRole } = useRolesContext();
  const updateRole = useUpdateRole();
  const [dialogContent, setDialogContent] = React.useState<HTMLDivElement | null>(null);

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
      setEditingRole(null);
    } catch (error) {
      // Error is handled by the mutation
    }
  };

  const handleClose = () => {
    setEditingRole(null);
    form.reset();
  };

  if (!editingRole) return null;

  return (
    <Dialog open={!!editingRole} onOpenChange={handleClose}>
      <DialogContent className='max-w-2xl' ref={setDialogContent}>
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
              render={({ field }) => (
                <FormItem>
                  <div className='mb-4'>
                    <FormLabel className='text-base'>{t('roles.dialogs.fields.scopes.label')}</FormLabel>
                    <FormDescription>{t('roles.dialogs.fields.scopes.description')}</FormDescription>
                  </div>
                  <FormControl>
                    <ScopesSelect value={field.value || []} onChange={field.onChange} portalContainer={dialogContent} />
                  </FormControl>
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
  const { deletingRole, setDeletingRole } = useRolesContext();
  const deleteRole = useDeleteRole();

  const handleConfirm = async () => {
    if (!deletingRole) return;

    try {
      await deleteRole.mutateAsync(deletingRole.id);
      setDeletingRole(null);
    } catch (error) {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={!!deletingRole}
      onOpenChange={() => setDeletingRole(null)}
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

// Combined Dialogs Component
export function RolesDialogs() {
  return (
    <>
      <CreateRoleDialog />
      <EditRoleDialog />
      <DeleteRoleDialog />
    </>
  );
}
