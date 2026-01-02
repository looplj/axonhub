'use client';

import { useState, useEffect, useCallback } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { graphqlRequest } from '@/gql/graphql';
import { ROLES_QUERY, ALL_SCOPES_QUERY } from '@/gql/roles';
import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { SelectDropdown } from '@/components/select-dropdown';
import { useProjects } from '@/features/projects/data/projects';
import { User } from '../data/schema';

// GraphQL query to get user's existing projects
const USER_PROJECTS_QUERY = `
  query UserProjects($userId: ID!) {
    node(id: $userId) {
      ... on User {
        id
        projectUsers {
          projectID
        }
      }
    }
  }
`;

// GraphQL mutation to add user to project
const ADD_USER_TO_PROJECT_MUTATION = `
  mutation AddUserToProject($input: AddUserToProjectInput!) {
    addUserToProject(input: $input) {
      id
      userID
      projectID
      isOwner
      scopes
    }
  }
`;

const createFormSchema = (t: (key: string) => string) =>
  z.object({
    projectId: z.string().min(1, t('users.validation.projectRequired')),
    isOwner: z.boolean().optional(),
    roleIDs: z.array(z.string()).optional(),
    scopes: z.array(z.string()).optional(),
  });

interface Role {
  id: string;
  name: string;
  description?: string;
  scopes?: string[];
}

interface ScopeInfo {
  scope: string;
  description?: string;
  levels?: string[];
}

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersAddToProjectDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const [roles, setRoles] = useState<Role[]>([]);
  const [allScopes, setAllScopes] = useState<ScopeInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [userProjectIds, setUserProjectIds] = useState<string[]>([]);

  // Fetch all projects
  const { data: projectsData } = useProjects({ first: 100 });

  const formSchema = createFormSchema(t);
  type AddToProjectForm = z.infer<typeof formSchema>;

  const form = useForm<AddToProjectForm>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      projectId: '',
      isOwner: false,
      roleIDs: [],
      scopes: [],
    },
  });

  const selectedProjectId = form.watch('projectId');

  // Load user's existing projects when dialog opens
  useEffect(() => {
    if (open && currentRow?.id) {
      const loadUserProjects = async () => {
        try {
          const data = await graphqlRequest(USER_PROJECTS_QUERY, {
            userId: currentRow.id,
          });

          const response = data as {
            node: {
              id: string;
              projectUsers: Array<{ projectID: string }>;
            };
          };

          const projectIds = response.node.projectUsers?.map((pu) => pu.projectID) || [];
          setUserProjectIds(projectIds);
        } catch (error) {
          console.error('Failed to load user projects:', error);
          setUserProjectIds([]);
        }
      };

      loadUserProjects();
    } else if (!open) {
      // Reset when dialog closes
      setUserProjectIds([]);
    }
  }, [open, currentRow?.id]);

  const loadRolesAndScopes = useCallback(
    async (projectId: string) => {
      if (!projectId) return;

      setLoading(true);
      try {
        const [rolesData, scopesData] = await Promise.all([
          graphqlRequest(ROLES_QUERY, {
            first: 100,
            where: { projectID: projectId },
          }),
          graphqlRequest(ALL_SCOPES_QUERY, { level: 'project' }),
        ]);

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

        const scopesResponse = scopesData as {
          allScopes: Array<{
            scope: string;
            description?: string;
            levels?: string[];
          }>;
        };

        setRoles(rolesResponse.roles.edges.map((edge) => edge.node));
        setAllScopes(scopesResponse.allScopes);
      } catch (error) {
        console.error('Failed to load roles and scopes:', error);
        toast.error(t('common.errors.userLoadFailed'));
      } finally {
        setLoading(false);
      }
    },
    [t]
  );

  useEffect(() => {
    if (selectedProjectId) {
      loadRolesAndScopes(selectedProjectId);
    }
  }, [selectedProjectId, loadRolesAndScopes]);

  const onSubmit = async (values: AddToProjectForm) => {
    if (!currentRow) return;

    setSubmitting(true);
    try {
      const headers = { 'X-Project-ID': values.projectId };
      await graphqlRequest(
        ADD_USER_TO_PROJECT_MUTATION,
        {
          input: {
            projectId: values.projectId,
            userId: currentRow.id,
            isOwner: values.isOwner,
            scopes: values.scopes,
            roleIDs: values.roleIDs,
          },
        },
        headers
      );

      toast.success(t('users.messages.addToProjectSuccess'));
      form.reset();
      onOpenChange(false);
    } catch (error: any) {
      console.error('Failed to add user to project:', error);
      toast.error(t('users.messages.addToProjectError') + `: ${error.message}`);
    } finally {
      setSubmitting(false);
    }
  };

  const handleRoleToggle = (roleId: string) => {
    const currentRoles = form.getValues('roleIDs') || [];
    const newRoles = currentRoles.includes(roleId) ? currentRoles.filter((id: string) => id !== roleId) : [...currentRoles, roleId];
    form.setValue('roleIDs', newRoles);
  };

  const handleScopeToggle = (scopeName: string) => {
    const currentScopes = form.getValues('scopes') || [];
    const newScopes = currentScopes.includes(scopeName)
      ? currentScopes.filter((name: string) => name !== scopeName)
      : [...currentScopes, scopeName];
    form.setValue('scopes', newScopes);
  };

  const handleScopeRemove = (scopeName: string) => {
    const currentScopes = form.getValues('scopes') || [];
    const newScopes = currentScopes.filter((name: string) => name !== scopeName);
    form.setValue('scopes', newScopes);
  };

  // Mark projects that the user is already a member of as disabled
  const projects =
    projectsData?.edges?.map((edge) => ({
      label: edge.node.name,
      value: edge.node.id,
      disabled: userProjectIds.includes(edge.node.id),
    })) || [];

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('users.dialogs.addToProject.title')}</DialogTitle>
          <DialogDescription>
            {currentRow &&
              t('users.dialogs.addToProject.description', {
                firstName: currentRow.firstName,
                lastName: currentRow.lastName,
              })}
          </DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-y-auto'>
          <Form {...form}>
            <form id='add-to-project-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
              <FormField
                control={form.control}
                name='projectId'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.selectProject')}</FormLabel>
                    <SelectDropdown
                      defaultValue={field.value}
                      onValueChange={field.onChange}
                      placeholder={t('users.form.selectProjectPlaceholder')}
                      items={projects}
                    />
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectedProjectId && (
                <>
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
                </>
              )}
            </form>
          </Form>
        </div>

        <DialogFooter>
          <Button type='submit' form='add-to-project-form' disabled={submitting}>
            {submitting ? t('users.buttons.adding') : t('users.buttons.addToProject')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
