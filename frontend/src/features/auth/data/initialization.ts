import { useMutation, useQuery } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { toast } from 'sonner'
import { systemApi } from '@/lib/api-client'
import i18n from '@/lib/i18n'

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
    retry: 1, // Allow one retry to handle network hiccups on first load
    retryDelay: 500, // Wait 500ms before retry
    refetchOnWindowFocus: false,
    staleTime: 5 * 60 * 1000, // Consider data fresh for 5 minutes
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
      const errorMessage = error.message || i18n.t('common.errors.initializationFailed')
      toast.error(errorMessage)
    },
  })
}
