'use client';

import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { graphqlRequest } from '@/gql/graphql';
import { ROLES_QUERY } from '@/gql/roles';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';
import { canEditUserPermissions } from '@/lib/permission-utils';
import { formatUserName } from '@/lib/utils';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage, useFormField } from '@/components/ui/form';
import { ScopesSelect } from '@/components/scopes-select';
import { useDebounce } from '@/hooks/use-debounce';
import { User } from '../data/schema';
import { useAddUserToProject, useUpdateProjectUser, useAllUsersOptions } from '../data/users';

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

interface UserFieldPickerProps {
  field: { value: string; onChange: (value: string) => void };
  options: { value: string; label: string }[];
  isLoading: boolean;
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
  searchValue: string;
  onSearchValueChange: (value: string) => void;
  portalContainer: HTMLDivElement | null;
  onOpenChange: (open: boolean) => void;
}

// The picker's inner input is not wrapped by <FormControl>, so the form a11y
// ids (label htmlFor, required-error association) are passed through explicitly.
function UserFieldPicker({
  field,
  options,
  isLoading,
  hasMore,
  isLoadingMore,
  onLoadMore,
  searchValue,
  onSearchValueChange,
  portalContainer,
  onOpenChange,
}: UserFieldPickerProps) {
  const { t } = useTranslation();
  const { error, formItemId, formDescriptionId, formMessageId } = useFormField();

  return (
    <AutoCompleteSelect
      selectedValue={field.value || ''}
      onSelectedValueChange={(value) => field.onChange(value)}
      items={options}
      isLoading={isLoading}
      emptyMessage={t('users.form.noUsersFound')}
      placeholder={t('users.form.selectUserPlaceholder')}
      portalContainer={portalContainer}
      searchValue={searchValue}
      onSearchValueChange={onSearchValueChange}
      hasMore={hasMore}
      isLoadingMore={isLoadingMore}
      onLoadMore={onLoadMore}
      onOpenChange={onOpenChange}
      inputProps={{
        id: formItemId,
        ...(error ? { 'aria-invalid': true } : {}),
        'aria-describedby': `${formDescriptionId} ${formMessageId}`.trim(),
      }}
    />
  );
}

export function ProjectUserActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const currentUser = useAuthStore((state) => state.auth.user);
  const selectedProjectId = useSelectedProjectId();
  const isEdit = !!currentRow;
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [canEdit, setCanEdit] = useState(true);
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  // Add-to-project user picker: the input text is owned by the picker; a
  // debounced copy drives the server-side email search so results are not
  // capped at the first page. The picker clears its input on open, which also
  // clears the search term, so the selected label is never fed back as a filter.
  const [pickerOpen, setPickerOpen] = useState(false);
  const [searchValue, setSearchValue] = useState('');
  const debouncedSearchValue = useDebounce(searchValue, 300);
  const {
    data: usersData,
    isFetching,
    isFetchingNextPage,
    fetchNextPage,
    hasNextPage,
  } = useAllUsersOptions({ search: pickerOpen ? debouncedSearchValue : '', enabled: open && pickerOpen });
  const userOptions = useMemo(() => {
    const options = new Map<string, { value: string; label: string }>();
    for (const page of usersData?.pages ?? []) {
      for (const edge of page.edges) {
        options.set(edge.node.id, {
          value: edge.node.id,
          label: `${formatUserName(edge.node.firstName, edge.node.lastName)} (${edge.node.email})`,
        });
      }
    }
    return Array.from(options.values());
  }, [usersData]);

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

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
          setSearchValue('');
          setPickerOpen(false);
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
                      <UserFieldPicker
                        field={field}
                        options={userOptions}
                        isLoading={isFetching && !usersData}
                        hasMore={!!hasNextPage}
                        isLoadingMore={isFetchingNextPage}
                        onLoadMore={fetchNextPage}
                        searchValue={searchValue}
                        onSearchValueChange={setSearchValue}
                        portalContainer={dialogContent}
                        onOpenChange={setPickerOpen}
                      />
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
