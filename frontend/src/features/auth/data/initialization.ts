import { useMutation, useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { SYSTEM_STATUS_QUERY, INITIALIZE_SYSTEM_MUTATION } from '@/gql/users'
import { useAuthStore } from '@/stores/authStore'
import { toast } from 'sonner'
import { useRouter } from '@tanstack/react-router'

export interface SystemStatus {
  isInitialized: boolean
}

export interface InitializeSystemInput {
  ownerEmail: string
  ownerPassword: string
  secretKey?: string
}

export interface InitializeSystemPayload {
  success: boolean
  message: string
  user?: {
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
  token?: string
}

export function useSystemStatus() {
  return useQuery({
    queryKey: ['systemStatus'],
    queryFn: async (): Promise<SystemStatus> => {
      const data = await graphqlRequest<{ systemStatus: SystemStatus }>(
        SYSTEM_STATUS_QUERY
      )
      return data.systemStatus
    },
    retry: false,
    refetchOnWindowFocus: false,
  })
}

export function useInitializeSystem() {
  const { setUser, setAccessToken } = useAuthStore((state) => state.auth)
  const router = useRouter()

  return useMutation({
    mutationFn: async (input: InitializeSystemInput): Promise<InitializeSystemPayload> => {
      const data = await graphqlRequest<{ initializeSystem: InitializeSystemPayload }>(
        INITIALIZE_SYSTEM_MUTATION,
        { input }
      )
      return data.initializeSystem
    },
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message)
        
        // If user and token are provided, log them in
        if (data.user && data.token) {
          // Store the JWT token
          setAccessToken(data.token)
          
          // Store user info
          setUser({
            accountNo: data.user.id,
            email: data.user.email,
            role: data.user.roles.edges.map(edge => edge.node.name),
            exp: 0 // Will be extracted from JWT if needed
          })

          // Redirect to dashboard
          router.navigate({ to: '/' })
        }
      } else {
        toast.error(data.message)
      }
    },
    onError: (error) => {
      toast.error(error.message || 'Failed to initialize system')
    },
  })
}