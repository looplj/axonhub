import { useMutation, useQuery } from '@tanstack/react-query'
import { authApi, setTokenInStorage, removeTokenFromStorage } from '@/lib/api-client'
import { useAuthStore } from '@/stores/authStore'
import { toast } from 'sonner'
import { useRouter } from '@tanstack/react-router'
import { graphqlRequest } from '@/gql/graphql'
import { ME_QUERY } from '@/gql/users'

export interface SignInInput {
  email: string
  password: string
}

interface MeResponse {
  me: {
    email: string
    firstName: string
    lastName: string
    isOwner: boolean
    scopes: string[]
    roles: Array<{
      id: string
      name: string
    }>
  }
}

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      const data = await graphqlRequest<MeResponse>(ME_QUERY)
      return data.me
    },
    enabled: !!useAuthStore.getState().auth.accessToken,
    retry: false,
  })
}

export function useSignIn() {
  const { setUser, setAccessToken } = useAuthStore((state) => state.auth)
  const router = useRouter()

  return useMutation({
    mutationFn: async (input: SignInInput) => {
      return await authApi.signIn(input)
    },
    onSuccess: (data) => {
      // Store token in localStorage
      setTokenInStorage(data.token)
      
      // Update auth store
      setAccessToken(data.token)
      setUser({
        email: data.user.email,
        firstName: data.user.firstName || '',
        lastName: data.user.lastName || '',
        isOwner: data.user.isOwner,
        scopes: data.user.scopes,
        roles: data.user.roles.map(role => ({
          id: role.id,
          name: role.name
        }))
      })
      
      toast.success('Successfully signed in!')
      
      // Redirect to home page
      router.navigate({ to: '/' })
    },
    onError: (error: any) => {
      const errorMessage = error.message || 'Failed to sign in'
      toast.error(errorMessage)
    }
  })
}

export function useSignOut() {
  const { reset } = useAuthStore((state) => state.auth)
  const router = useRouter()

  return () => {
    // Clear token from localStorage
    removeTokenFromStorage()
    
    // Clear auth store
    reset()
    
    toast.success('Successfully signed out!')
    
    // Redirect to sign in page
    router.navigate({ to: '/sign-in' })
  }
}