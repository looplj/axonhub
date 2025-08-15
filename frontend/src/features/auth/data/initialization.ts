import { useMutation, useQuery } from '@tanstack/react-query'
import { systemApi } from '@/lib/api-client'
import { useAuthStore } from '@/stores/authStore'
import { toast } from 'sonner'
import { useRouter } from '@tanstack/react-router'

export interface SystemStatus {
  isInitialized: boolean
}

export interface InitializeSystemInput {
  ownerEmail: string
  ownerPassword: string
  ownerFirstName: string
  ownerLastName: string
  brandName: string
}

export interface InitializeSystemPayload {
  success: boolean
  message: string
}

export function useSystemStatus() {
  return useQuery({
    queryKey: ['systemStatus'],
    queryFn: async (): Promise<SystemStatus> => {
      return await systemApi.getStatus()
    },
    retry: false,
    refetchOnWindowFocus: false,
  })
}

export function useInitializeSystem() {
  const router = useRouter()

  return useMutation({
    mutationFn: async (input: InitializeSystemInput): Promise<InitializeSystemPayload> => {
      return await systemApi.initialize(input)
    },
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message)
        
        // 初始化成功后跳转到登录页面
        router.navigate({ to: '/sign-in' })
      } else {
        toast.error(data.message)
      }
    },
    onError: (error: any) => {
      const errorMessage = error.message || 'Failed to initialize system'
      toast.error(errorMessage)
    }
  })
}