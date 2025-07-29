import { useEffect } from 'react'
import { useRouter } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore'
import { Skeleton } from '@/components/ui/skeleton'

interface AuthGuardProps {
  children: React.ReactNode
}

export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter()
  const { accessToken } = useAuthStore((state) => state.auth)

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

  return <>{children}</>
}