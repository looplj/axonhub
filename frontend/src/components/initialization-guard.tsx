import { useEffect } from 'react'
import { useRouter } from '@tanstack/react-router'
import { useSystemStatus } from '@/features/auth/data/initialization'
import { Skeleton } from '@/components/ui/skeleton'

interface InitializationGuardProps {
  children: React.ReactNode
}

export function InitializationGuard({ children }: InitializationGuardProps) {
  const router = useRouter()
  const { data: systemStatus, isLoading, error } = useSystemStatus()

  useEffect(() => {
    // Only redirect if we have data and system is not initialized
    if (systemStatus && !systemStatus.isInitialized) {
      // Check if we're not already on the initialization page
      const currentPath = window.location.pathname
      if (currentPath !== '/initialization') {
        router.navigate({ to: '/initialization' })
      }
    }
  }, [systemStatus, router])

  // Show loading skeleton while checking system status
  if (isLoading) {
    return (
      <div className='flex h-screen items-center justify-center'>
        <div className='space-y-4'>
          <Skeleton className='h-8 w-48' />
          <Skeleton className='h-4 w-32' />
        </div>
      </div>
    )
  }

  // Show error if failed to check system status
  if (error) {
    return (
      <div className='flex h-screen items-center justify-center'>
        <div className='text-center'>
          <h1 className='text-2xl font-bold text-red-600'>System Error</h1>
          <p className='text-muted-foreground'>Failed to check system status</p>
        </div>
      </div>
    )
  }

  // If system is not initialized and we're not on initialization page, don't render children
  if (systemStatus && !systemStatus.isInitialized && window.location.pathname !== '/initialization') {
    return null
  }

  return <>{children}</>
}