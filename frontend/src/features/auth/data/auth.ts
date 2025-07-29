import { useMutation } from '@tanstack/react-query'
import { authApi, setTokenInStorage, removeTokenFromStorage } from '@/lib/api-client'
import { useAuthStore } from '@/stores/authStore'
import { toast } from 'sonner'
import { useRouter } from '@tanstack/react-router'

export interface SignInInput {
  email: string
  password: string
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
        accountNo: data.user.id,
        email: data.user.email,
        role: data.user.roles.map(role => role.name),
        exp: 0 // Will be extracted from JWT if needed
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