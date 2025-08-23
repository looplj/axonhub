'use client'

import { useTranslation } from 'react-i18next'
import { Bar, BarChart, ResponsiveContainer, XAxis, YAxis } from 'recharts'
import { useDailyRequestStats } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

export function DailyRequestStats() {
  const { t } = useTranslation()
  const { data: dailyStats, isLoading, error } = useDailyRequestStats(30)

  if (isLoading) {
    return (
      <div className="h-[350px] flex items-center justify-center">
        <Skeleton className="h-full w-full" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="h-[350px] flex items-center justify-center text-red-500">
        {t('dashboard.charts.errorLoadingChart')} {error.message}
      </div>
    )
  }

  // Transform data for the chart
  const chartData = dailyStats?.map(stat => ({
    name: new Date(stat.date).toLocaleDateString('en-US', { 
      month: 'short', 
      day: 'numeric' 
    }),
    total: stat.count,
  })) || []

  return (
    <ResponsiveContainer width="100%" height={350}>
      <BarChart data={chartData}>
        <XAxis
          dataKey="name"
          stroke="#888888"
          fontSize={12}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          stroke="#888888"
          fontSize={12}
          tickLine={false}
          axisLine={false}
          tickFormatter={(value) => `${value}`}
        />
        <Bar
          dataKey="total"
          fill="var(--chart-1)"
          radius={[4, 4, 0, 0]}
        />
      </BarChart>
    </ResponsiveContainer>
  )
}
