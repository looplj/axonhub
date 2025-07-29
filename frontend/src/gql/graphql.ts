import { toast } from 'sonner'

export const GRAPHQL_ENDPOINT = import.meta.env.VITE_SERVER_ADMIN_GRAPHQL_URL

const ACCESS_TOKEN = 'axonhub_access_token'

// Helper function to get token from localStorage
const getTokenFromStorage = (): string => {
  try {
    return localStorage.getItem(ACCESS_TOKEN) || ''
  } catch (error) {
    console.warn('Failed to read token from localStorage:', error)
    return ''
  }
}

// Helper function to remove token from localStorage
const removeTokenFromStorage = (): void => {
  try {
    localStorage.removeItem(ACCESS_TOKEN)
  } catch (error) {
    console.warn('Failed to remove token from localStorage:', error)
  }
}

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
    toast.error('Session expired. Please sign in again.')
    window.location.href = '/sign-in'
    throw new Error('Unauthorized')
  }

  const result = await response.json()

  if (result.errors) {
    // Check for authentication errors
    const authError = result.errors.find((error: any) => 
      error.message?.includes('unauthorized') || 
      error.message?.includes('unauthenticated') ||
      error.extensions?.code === 'UNAUTHENTICATED'
    )
    
    if (authError) {
      // Clear token and redirect to login
      removeTokenFromStorage()
      toast.error('Session expired. Please sign in again.')
      window.location.href = '/sign-in'
      throw new Error('Unauthorized')
    }
    
    throw new Error(result.errors[0]?.message || 'GraphQL Error')
  }

  return result.data
}
