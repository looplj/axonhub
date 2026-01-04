'use client';

import { useState, useEffect, useCallback } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { graphqlRequest } from '@/gql/graphql';
import { ROLES_QUERY } from '@/gql/roles';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { SelectDropdown } from '@/components/select-dropdown';
import { ScopesSelect } from '@/components/ui/scopes-select';
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

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersAddToProjectDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [userProjectIds, setUserProjectIds] = useState<string[]>([]);
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

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
          setUserProjectIds([]);
        }
      };

      loadUserProjects();
    } else if (!open) {
      // Reset when dialog closes
      setUserProjectIds([]);
    }
  }, [open, currentRow?.id]);

  const loadRoles = useCallback(
    async (projectId: string) => {
      if (!projectId) return;

      setLoading(true);
      try {
        const rolesData = await graphqlRequest(ROLES_QUERY, {
          first: 100,
          where: { projectID: projectId },
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
      } catch (error) {
        toast.error(t('common.errors.userLoadFailed'));
      } finally {
        setLoading(false);
      }
    },
    [t]
  );

  useEffect(() => {
    if (selectedProjectId) {
      loadRoles(selectedProjectId);
    }
  }, [selectedProjectId, loadRoles]);

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
    } catch (error) {
      toast.error(t('common.errors.somethingWentWrong'));
    } finally {
      setSubmitting(false);
    }
  };

  const handleRoleToggle = (roleId: string) => {
    const currentRoles = form.getValues('roleIDs') || [];
    const newRoles = currentRoles.includes(roleId) ? currentRoles.filter((id: string) => id !== roleId) : [...currentRoles, roleId];
    form.setValue('roleIDs', newRoles);
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
      <DialogContent className='sm:max-w-2xl' ref={setDialogContent}>
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
