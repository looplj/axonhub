import { toast } from 'sonner'
import { getTokenFromStorage, removeTokenFromStorage } from '@/stores/authStore'
import i18n from '@/lib/i18n'

export const GRAPHQL_ENDPOINT = '/admin/graphql'

// GraphQL client function with token support
export async function graphqlRequest<T>(
  query: string,
  variables?: Record<string, any>
): Promise<T> {
  // Get token from localStorage
  const token = getTokenFromStorage()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  // Add Authorization header if token exists
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(GRAPHQL_ENDPOINT, {
    method: 'POST',
    headers,
    body: JSON.stringify({
      query,
      variables,
    }),
  })

  // Handle 401 Unauthorized
  if (response.status === 401) {
    // Clear token and redirect to login
    removeTokenFromStorage()
    toast.error(i18n.t('common.errors.sessionExpiredSignIn'))
    window.location.href = '/sign-in'
    throw new Error('Unauthorized')
  }

  const result = await response.json()

  if (result.errors) {
    // Check for authentication errors
    const authError = result.errors.find(
      (error: any) =>
        error.message?.includes('unauthorized') ||
        error.message?.includes('unauthenticated') ||
        error.extensions?.code === 'UNAUTHENTICATED'
    )

    if (authError) {
      // Clear token and redirect to login
      removeTokenFromStorage()
      toast.error(i18n.t('common.errors.sessionExpiredSignIn'))
      window.location.href = '/sign-in'
      throw new Error('Unauthorized')
    }

    throw new Error(result.errors[0]?.message || 'GraphQL Error')
  }

  return result.data
}
