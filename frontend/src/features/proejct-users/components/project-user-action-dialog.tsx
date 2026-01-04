'use client';

import { useState, useEffect, useCallback, useMemo } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { graphqlRequest } from '@/gql/graphql';
import { ROLES_QUERY } from '@/gql/roles';
import { Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';
import { canEditUserPermissions } from '@/lib/permission-utils';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ScopesSelect } from '@/components/ui/scopes-select';
import { User } from '../data/schema';
import { useAddUserToProject, useUpdateProjectUser, useAllUsers } from '../data/users';

interface Role {
  id: string;
  name: string;
  description?: string;
  scopes?: string[];
}

const formSchema = z.object({
  userId: z.string().optional(),
  isOwner: z.boolean().optional(),
  roleIDs: z.array(z.string()).optional(),
  scopes: z.array(z.string()).optional(),
});

type UserForm = z.infer<typeof formSchema>;

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ProjectUserActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const currentUser = useAuthStore((state) => state.auth.user);
  const selectedProjectId = useSelectedProjectId();
  const isEdit = !!currentRow;
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [canEdit, setCanEdit] = useState(true);
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  const addUserToProject = useAddUserToProject();
  const updateProjectUser = useUpdateProjectUser();

  // Create dynamic schema based on edit mode
  const dynamicFormSchema = useMemo(() => {
    if (isEdit) {
      return formSchema;
    }
    return formSchema.extend({
      userId: z.string().min(1, t('users.validation.userRequired')),
    });
  }, [isEdit, t]);

  // Fetch all users - only when dialog is open
  const { data: usersData, isLoading: usersLoading } = useAllUsers(
    {
      first: 100,
      where: searchTerm ? { emailContainsFold: searchTerm } : undefined,
    },
    { enabled: open }
  );

  const form = useForm<UserForm>({
    resolver: zodResolver(dynamicFormSchema),
    defaultValues: isEdit
      ? {
          userId: currentRow.id,
          isOwner: currentRow.isOwner,
          roleIDs: currentRow.roles?.edges?.map((edge) => edge.node.id) || [],
          scopes: currentRow.scopes || [],
        }
      : {
          userId: '',
          isOwner: false,
          roleIDs: [],
          scopes: [],
        },
  });

  const loadRolesAndScopes = useCallback(async () => {
    if (!selectedProjectId) return;

    setLoading(true);
    try {
      const rolesData = await graphqlRequest(ROLES_QUERY, {
        first: 100,
        where: { projectID: selectedProjectId },
      });

      const rolesResponse = rolesData as {
        roles: {
          edges: Array<{
            node: {
              id: string;
              name: string;
              description?: string;
              scopes?: string[];
            };
          }>;
        };
      };

      setRoles(rolesResponse.roles.edges.map((edge) => edge.node));

      if (isEdit && currentRow) {
        const targetScopes = currentRow.scopes || [];
        const canEditTarget = canEditUserPermissions(currentUser, targetScopes, currentRow.isOwner || false, selectedProjectId);
        setCanEdit(canEditTarget);
      }
    } catch (error) {
      toast.error(t('common.errors.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t, selectedProjectId, isEdit, currentRow, currentUser]);

  useEffect(() => {
    if (open) {
      loadRolesAndScopes();
    }
  }, [open, loadRolesAndScopes]);

  const onSubmit = async (values: UserForm) => {
    if (isEdit && currentRow) {
      const currentRoleIDs = currentRow.roles?.edges?.map((edge) => edge.node.id) || [];
      const newRoleIDs = values.roleIDs || [];

      const addRoleIDs = newRoleIDs.filter((id) => !currentRoleIDs.includes(id));
      const removeRoleIDs = currentRoleIDs.filter((id) => !newRoleIDs.includes(id));

      await updateProjectUser.mutateAsync({
        userId: currentRow.id,
        isOwner: values.isOwner,
        scopes: values.scopes,
        addRoleIDs: addRoleIDs.length > 0 ? addRoleIDs : undefined,
        removeRoleIDs: removeRoleIDs.length > 0 ? removeRoleIDs : undefined,
      });

      form.reset();
      onOpenChange(false);
    } else {
      await addUserToProject.mutateAsync({
        userId: values.userId || '',
        isOwner: values.isOwner,
        scopes: values.scopes,
        roleIDs: values.roleIDs,
      });

      form.reset();
      onOpenChange(false);
    }
  };

  const handleRoleToggle = (roleId: string) => {
    const currentRoles = form.getValues('roleIDs') || [];
    const newRoles = currentRoles.includes(roleId) ? currentRoles.filter((id: string) => id !== roleId) : [...currentRoles, roleId];
    form.setValue('roleIDs', newRoles);
  };

  const availableUsers = usersData?.edges?.map((edge) => edge.node) || [];

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
          setSearchTerm('');
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className='sm:max-w-2xl' ref={setDialogContent}>
        <DialogHeader className='text-left'>
          <DialogTitle>
            {isEdit
              ? t('users.dialogs.edit.title')
              : t('users.dialogs.addToProject.title')}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('users.dialogs.edit.description')
              : t('users.dialogs.addToProject.description')}
          </DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-y-auto'>
          <Form {...form}>
            <form id='user-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
              {/* User Selection - only show in add mode */}
              {!isEdit && (
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
                            <div className='text-muted-foreground p-2 text-center text-sm'>{t('users.form.noUsersFound')}</div>
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
              )}

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
              <FormField
                control={form.control}
                name='scopes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.projectScopes')}</FormLabel>
                    <FormControl>
                      <ScopesSelect value={field.value || []} onChange={field.onChange} portalContainer={dialogContent} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </form>
          </Form>
        </div>

        <DialogFooter>
          {isEdit && !canEdit && <p className='text-destructive mr-auto text-sm'>{t('users.errors.insufficientPermissions')}</p>}
          <Button
            variant='outline'
            onClick={() => {
              form.reset();
              onOpenChange(false);
            }}
          >
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type='submit'
            form='user-form'
            disabled={addUserToProject.isPending || updateProjectUser.isPending || (isEdit && !canEdit)}
          >
            {addUserToProject.isPending || updateProjectUser.isPending
              ? t('users.buttons.adding')
              : isEdit
                ? t('common.buttons.saveChanges')
                : t('users.buttons.addToProject')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
