import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Overview } from './components/overview'
import { RecentSales } from './components/recent-sales'
import { RequestsByChannelChart } from './components/requests-by-channel-chart'
import { RequestsByStatusChart } from './components/requests-by-status-chart'
import { useDashboardStats } from './data/dashboard'
import { Header } from '@/components/layout/header'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ProfileDropdown } from '@/components/profile-dropdown'

export default function DashboardPage() {
  const { data: stats, isLoading, error } = useDashboardStats()

  if (isLoading) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <div className='flex items-center justify-between space-y-2'>
          <Skeleton className='h-8 w-[200px]' />
          <div className='flex items-center space-x-2'>
            <Skeleton className='h-10 w-[200px]' />
            <Skeleton className='h-10 w-[100px]' />
          </div>
        </div>
        <Tabs defaultValue='overview' className='space-y-4'>
          <Skeleton className='h-10 w-[400px]' />
          <div className='space-y-4'>
            <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className='h-[120px]' />
              ))}
            </div>
            <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
              <Skeleton className='col-span-4 h-[300px]' />
              <Skeleton className='col-span-3 h-[300px]' />
            </div>
          </div>
        </Tabs>
      </div>
    )
  }

  if (error) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <div className='text-red-500'>
          Error loading dashboard: {error.message}
        </div>
      </div>
    )
  }

  return (
    <div className='flex-1 space-y-4 p-8 pt-6'>
      <Header>
        {/* <TopNav links={topNav} /> */}
        <div className='ml-auto flex items-center space-x-4'>
          <Search />
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>
      <Tabs defaultValue='overview' className='space-y-4'>
        <TabsContent value='overview' className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Users
                </CardTitle>
                <svg
                  xmlns='http://www.w3.org/2000/svg'
                  viewBox='0 0 24 24'
                  fill='none'
                  stroke='currentColor'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  strokeWidth='2'
                  className='text-muted-foreground h-4 w-4'
                >
                  <path d='M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2' />
                  <circle cx='9' cy='7' r='4' />
                  <path d='M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75' />
                </svg>
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>
                  {stats?.totalUsers || 0}
                </div>
                <p className='text-muted-foreground text-xs'>
                  +20.1% from last month
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Channels
                </CardTitle>
                <svg
                  xmlns='http://www.w3.org/2000/svg'
                  viewBox='0 0 24 24'
                  fill='none'
                  stroke='currentColor'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  strokeWidth='2'
                  className='text-muted-foreground h-4 w-4'
                >
                  <rect width='20' height='14' x='2' y='5' rx='2' />
                  <path d='M2 10h20' />
                </svg>
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>
                  {stats?.totalChannels || 0}
                </div>
                <p className='text-muted-foreground text-xs'>
                  +180.1% from last month
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Requests
                </CardTitle>
                <svg
                  xmlns='http://www.w3.org/2000/svg'
                  viewBox='0 0 24 24'
                  fill='none'
                  stroke='currentColor'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  strokeWidth='2'
                  className='text-muted-foreground h-4 w-4'
                >
                  <path d='M22 12h-4l-3 9L9 3l-3 9H2' />
                </svg>
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>
                  {stats?.totalRequests || 0}
                </div>
                <p className='text-muted-foreground text-xs'>
                  +19% from last month
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Requests Today
                </CardTitle>
                <svg
                  xmlns='http://www.w3.org/2000/svg'
                  viewBox='0 0 24 24'
                  fill='none'
                  stroke='currentColor'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  strokeWidth='2'
                  className='text-muted-foreground h-4 w-4'
                >
                  <path d='M22 12h-4l-3 9L9 3l-3 9H2' />
                </svg>
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>
                  {stats?.requestsToday || 0}
                </div>
                <p className='text-muted-foreground text-xs'>
                  +201 since yesterday
                </p>
              </CardContent>
            </Card>
          </div>
          <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
            <Card className='col-span-4'>
              <CardHeader>
                <CardTitle>Daily Request Overview</CardTitle>
              </CardHeader>
              <CardContent className='pl-2'>
                <Overview />
              </CardContent>
            </Card>
            <Card className='col-span-3'>
              <CardHeader>
                <CardTitle>Top Users</CardTitle>
                <CardDescription>
                  Users with the most requests this month.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <RecentSales />
              </CardContent>
            </Card>
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <Card>
              <CardHeader>
                <CardTitle>Requests by Channel</CardTitle>
                <CardDescription>
                  Distribution of requests across different channels.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <RequestsByChannelChart />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Requests by Status</CardTitle>
                <CardDescription>
                  Breakdown of request statuses.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <RequestsByStatusChart />
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
