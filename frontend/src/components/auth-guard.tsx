import { useEffect } from 'react'
import { useRouter } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore'
import { useMe } from '@/features/auth/data/auth'
import { Skeleton } from '@/components/ui/skeleton'

interface AuthGuardProps {
  children: React.ReactNode
}

export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter()
  const { accessToken } = useAuthStore((state) => state.auth)
  
  // Automatically fetch user info when token is available
  const { isLoading: isMeLoading, error: meError } = useMe()

  useEffect(() => {
    // If no token, redirect to sign-in
    if (!accessToken) {
      const currentPath = window.location.pathname
      // Don't redirect if already on auth pages
      if (!currentPath.startsWith('/sign-in') && 
          !currentPath.startsWith('/sign-up') && 
          !currentPath.startsWith('/initialization') &&
          !currentPath.startsWith('/forgot-password') &&
          !currentPath.startsWith('/otp')) {
        router.navigate({ to: '/sign-in' })
      }
    }
  }, [accessToken, router])

  // Handle me query error (e.g., token expired)
  useEffect(() => {
    if (meError && accessToken) {
      // Token might be expired, redirect to sign-in
      router.navigate({ to: '/sign-in' })
    }
  }, [meError, accessToken, router])

  // Show loading while checking auth
  if (!accessToken) {
    const currentPath = window.location.pathname
    // Don't show loading on auth pages
    if (currentPath.startsWith('/sign-in') || 
        currentPath.startsWith('/sign-up') || 
        currentPath.startsWith('/initialization') ||
        currentPath.startsWith('/forgot-password') ||
        currentPath.startsWith('/otp')) {
      return <>{children}</>
    }

    return (
      <div className='flex h-screen items-center justify-center'>
        <div className='space-y-4'>
          <Skeleton className='h-8 w-48' />
          <Skeleton className='h-4 w-32' />
        </div>
      </div>
    )
  }

  // Show loading while fetching user info
  if (accessToken && isMeLoading) {
    return (
      <div className='flex h-screen items-center justify-center'>
        <div className='space-y-4'>
          <Skeleton className='h-8 w-48' />
          <Skeleton className='h-4 w-32' />
        </div>
      </div>
    )
  }

  return <>{children}</>
}