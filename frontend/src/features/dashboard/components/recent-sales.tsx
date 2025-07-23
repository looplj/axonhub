import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { useTopUsers } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

export function RecentSales() {
  const { data: topUsers, isLoading, error } = useTopUsers(5)

  if (isLoading) {
    return (
      <div className="space-y-8">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center">
            <Skeleton className="h-9 w-9 rounded-full" />
            <div className="ml-4 space-y-1">
              <Skeleton className="h-4 w-[120px]" />
              <Skeleton className="h-3 w-[160px]" />
            </div>
            <Skeleton className="ml-auto h-4 w-[60px]" />
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-red-500 text-sm">
        Error loading top users: {error.message}
      </div>
    )
  }

  if (!topUsers || topUsers.length === 0) {
    return (
      <div className="text-muted-foreground text-sm">
        No user data available
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {topUsers.map((user) => (
        <div key={user.userId} className="flex items-center">
          <Avatar className="h-9 w-9">
            <AvatarImage src={`https://avatar.vercel.sh/${user.userEmail}`} alt="Avatar" />
            <AvatarFallback>
              {user.userName
                .split(' ')
                .map((n) => n[0])
                .join('')
                .toUpperCase()
                .slice(0, 2)}
            </AvatarFallback>
          </Avatar>
          <div className="ml-4 space-y-1">
            <p className="text-sm font-medium leading-none">{user.userName}</p>
            <p className="text-sm text-muted-foreground">
              {user.userEmail}
            </p>
          </div>
          <div className="ml-auto font-medium">
            {user.requestCount} requests
          </div>
        </div>
      ))}
    </div>
  )
}
