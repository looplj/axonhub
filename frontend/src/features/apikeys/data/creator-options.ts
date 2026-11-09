import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';

// The creator filter only needs identity fields. Fetching roles here would
// require read_roles in addition to the page's read_users permission.
export const API_KEY_CREATORS_QUERY = `
  query ApiKeyCreators {
    users(first: 100, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          email
          firstName
          lastName
        }
      }
    }
  }
`;

const creatorConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: z.object({
        id: z.string(),
        email: z.string(),
        firstName: z.string().nullable(),
        lastName: z.string().nullable(),
      }),
    })
  ),
});

export function useApiKeyCreators(enabled: boolean) {
  const projectId = useSelectedProjectId();
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['users', 'apiKeyCreators', projectId],
    enabled: enabled && !!projectId,
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ users: unknown }>(API_KEY_CREATORS_QUERY, undefined, {
          'X-Project-ID': projectId!,
        });
        return creatorConnectionSchema.parse(data.users);
      } catch (error) {
        handleError(error, t('common.errors.loadFailed'));
        throw error;
      }
    },
  });
}
