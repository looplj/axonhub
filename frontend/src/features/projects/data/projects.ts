import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { useSelectedProjectId, useProjectStore } from '@/stores/projectStore';
import { Project, ProjectConnection, CreateProjectInput, UpdateProjectInput, projectConnectionSchema, projectSchema, type UpdateProjectProfilesInput } from './schema';

// GraphQL queries and mutations
const PROJECTS_QUERY = `
  query GetProjects($first: Int, $after: Cursor, $orderBy: ProjectOrder, $where: ProjectWhereInput) {
    projects(first: $first, after: $after, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          description
          status
          profiles {
            activeProfile
            profiles {
              name
              channelIDs
              channelTags
              channelTagsMatchMode
            }
          }
        }
        cursor
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
      totalCount
    }
  }
`;

const CREATE_PROJECT_MUTATION = `
  mutation CreateProject($input: CreateProjectInput!) {
    createProject(input: $input) {
      id
      name
      description
      status
      createdAt
      updatedAt
    }
  }
`;

const UPDATE_PROJECT_MUTATION = `
  mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
    updateProject(id: $id, input: $input) {
      id
      name
      description
      status
      createdAt
      updatedAt
    }
  }
`;

const UPDATE_PROJECT_STATUS_MUTATION = `
  mutation UpdateProjectStatus($id: ID!, $status: ProjectStatus!) {
    updateProjectStatus(id: $id, status: $status) {
      id
      name
      description
      status
      createdAt
      updatedAt
    }
  }
`;

const DELETE_PROJECT_MUTATION = `
  mutation DeleteProject($id: ID!) {
    deleteProject(id: $id)
  }
`;

const MY_PROJECTS_QUERY = `
  query MyProjects {
    myProjects {
        id
        name
        description
        status
        createdAt
        updatedAt
    }
  }
`;

// Query hooks
export function useProjects(
  variables: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: any;
  } = {}
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  const queryVariables = {
    ...variables,
    orderBy: variables.orderBy || { field: 'CREATED_AT', direction: 'DESC' },
  };

  return useQuery({
    queryKey: ['projects', queryVariables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ projects: ProjectConnection }>(PROJECTS_QUERY, queryVariables);
        return projectConnectionSchema.parse(data?.projects);
      } catch (error) {
        handleError(error, t('projects.errors.loadProjectsFailed'));
        throw error;
      }
    },
  });
}

export function useProject(id: string) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['project', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ projects: ProjectConnection }>(PROJECTS_QUERY, { where: { id } });
        const project = data.projects.edges[0]?.node;
        if (!project) {
          throw new Error('Project not found');
        }
        return projectSchema.parse(project);
      } catch (error) {
        handleError(error, t('projects.errors.loadProjectDetailFailed'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

export function useMyProjects() {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['myProjects'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ myProjects: Project[] }>(MY_PROJECTS_QUERY);

        if (!data || !data.myProjects) {
          return [];
        }

        // myProjects 直接返回项目数组，不是 connection 格式
        const projects = data.myProjects.map((project) => projectSchema.parse(project));

        return projects;
      } catch (error) {
        handleError(error, t('projects.errors.loadMyProjectsFailed'));
        return [];
      }
    },
  });
}

// Mutation hooks
export function useCreateProject() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreateProjectInput) => {
      try {
        const data = await graphqlRequest<{ createProject: Project }>(CREATE_PROJECT_MUTATION, { input });
        return projectSchema.parse(data.createProject);
      } catch (error) {
        handleError(error, i18n.t('projects.errors.createProjectFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      queryClient.invalidateQueries({ queryKey: ['myProjects'] });
      toast.success(i18n.t('common.success.projectCreated'));
    },
  });
}

export function useUpdateProject() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateProjectInput }) => {
      try {
        const data = await graphqlRequest<{ updateProject: Project }>(UPDATE_PROJECT_MUTATION, { id, input });
        return projectSchema.parse(data.updateProject);
      } catch (error) {
        handleError(error, i18n.t('projects.errors.updateProjectFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      queryClient.invalidateQueries({ queryKey: ['project'] });
      queryClient.invalidateQueries({ queryKey: ['myProjects'] });
      toast.success(i18n.t('common.success.projectUpdated'));
    },
  });
}

export function useArchiveProject() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        const data = await graphqlRequest<{ updateProjectStatus: Project }>(UPDATE_PROJECT_STATUS_MUTATION, {
          id,
          status: 'archived',
        });
        return projectSchema.parse(data.updateProjectStatus);
      } catch (error) {
        handleError(error, i18n.t('projects.errors.archiveProjectFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      queryClient.invalidateQueries({ queryKey: ['project'] });
      queryClient.invalidateQueries({ queryKey: ['myProjects'] });
      toast.success(i18n.t('common.success.projectArchived'));
    },
  });
}

export function useActivateProject() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        const data = await graphqlRequest<{ updateProjectStatus: Project }>(UPDATE_PROJECT_STATUS_MUTATION, {
          id,
          status: 'active',
        });
        return projectSchema.parse(data.updateProjectStatus);
      } catch (error) {
        handleError(error, i18n.t('projects.errors.activateProjectFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      queryClient.invalidateQueries({ queryKey: ['project'] });
      queryClient.invalidateQueries({ queryKey: ['myProjects'] });
      toast.success(i18n.t('common.success.projectActivated'));
    },
  });
}

export function useDeleteProject() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();
  const { clearSelectedProjectId } = useProjectStore();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        const data = await graphqlRequest<{ deleteProject: boolean }>(DELETE_PROJECT_MUTATION, { id });
        return data.deleteProject;
      } catch (error) {
        handleError(error, i18n.t('projects.errors.deleteProjectFailed'));
        throw error;
      }
    },
    onSuccess: (_, deletedId) => {
      // Invalidate queries
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      queryClient.invalidateQueries({ queryKey: ['project'] });
      queryClient.invalidateQueries({ queryKey: ['myProjects'] });
      
      // If the deleted project is the currently selected one, clear the selection
      // The project-switcher will automatically select the first available project
      if (selectedProjectId === deletedId) {
        clearSelectedProjectId();
        toast.success(i18n.t('common.success.projectDeleted'));
      } else {
        toast.success(i18n.t('common.success.projectDeleted'));
      }
    },
  });
}

const UPDATE_PROJECT_PROFILES_MUTATION = `
  mutation UpdateProjectProfiles($id: ID!, $input: UpdateProjectProfilesInput!) {
    updateProjectProfiles(id: $id, input: $input) {
      id
      name
      profiles {
        activeProfile
        profiles {
          name
          channelIDs
          channelTags
          channelTagsMatchMode
        }
      }
    }
  }
`;

export function useUpdateProjectProfiles() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateProjectProfilesInput }) => {
      try {
        const data = await graphqlRequest<{ updateProjectProfiles: Project }>(UPDATE_PROJECT_PROFILES_MUTATION, { id, input });
        return data.updateProjectProfiles;
      } catch (error) {
        handleError(error, i18n.t('projects.profiles.errors.updateFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      queryClient.invalidateQueries({ queryKey: ['project'] });
      queryClient.invalidateQueries({ queryKey: ['myProjects'] });
      toast.success(i18n.t('projects.profiles.messages.updateSuccess'));
    },
  });
}
