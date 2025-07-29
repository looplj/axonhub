import { useMutation } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { SIGN_IN_MUTATION } from '@/gql/users'
import { useAuthStore } from '@/stores/authStore'
import { toast } from 'sonner'
import { useRouter } from '@tanstack/react-router'

export interface SignInInput {
  email: string
  password: string
}

export interface SignInPayload {
  user: {
    id: string
    email: string
    firstName?: string
    lastName?: string
    isOwner: boolean
    scopes: string[]
    roles: {
      edges: Array<{
        node: {
          id: string
          name: string
        }
      }>
    }
  }
  token: string
}

export function useSignIn() {
  const { setUser, setAccessToken } = useAuthStore((state) => state.auth)
  const router = useRouter()

  return useMutation({
    mutationFn: async (input: SignInInput): Promise<SignInPayload> => {
      const data = await graphqlRequest<{ signIn: SignInPayload }>(
        SIGN_IN_MUTATION,
        { input }
      )
      return data.signIn
    },
    onSuccess: (data) => {
      // Store the JWT token
      setAccessToken(data.token)
      
      // Store user info (adapt to existing AuthUser interface)
      setUser({
        accountNo: data.user.id,
        email: data.user.email,
        role: data.user.roles.edges.map(edge => edge.node.name),
        exp: 0 // Will be extracted from JWT if needed
      })

      toast.success('Successfully signed in!')
      
      // Redirect to dashboard
      router.navigate({ to: '/' })
    },
    onError: (error) => {
      toast.error(error.message || 'Failed to sign in')
    },
  })
}