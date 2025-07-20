export const GRAPHQL_ENDPOINT = import.meta.env.VITE_SERVER_ADMIN_GRAPHQL_URL

// GraphQL client function
export async function graphqlRequest<T>(
  query: string,
  variables?: Record<string, any>
): Promise<T> {
  const response = await fetch(GRAPHQL_ENDPOINT, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      query,
      variables,
    }),
  })

  const result = await response.json()

  if (result.errors) {
    throw new Error(result.errors[0]?.message || 'GraphQL Error')
  }

  return result.data
}
