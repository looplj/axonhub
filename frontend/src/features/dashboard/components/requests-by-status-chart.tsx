'use client'

import { useTranslation } from 'react-i18next'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { useRequestsByStatus } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

export function RequestsByStatusChart() {
  const { t } = useTranslation()
  const { data: statusData, isLoading, error } = useRequestsByStatus()

  if (isLoading) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <Skeleton className="h-[250px] w-full" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <div className="text-red-500 text-sm">
          {t('dashboard.charts.errorLoadingStatusData')} {error.message}
        </div>
      </div>
    )
  }

  if (!statusData || statusData.length === 0) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <div className="text-muted-foreground text-sm">
          {t('dashboard.charts.noStatusData')}
        </div>
      </div>
    )
  }

  const data = statusData.map((item) => ({
    status: item.status,
    count: item.count,
  }))

  return (
    <ResponsiveContainer width="100%" height={300}>
      <BarChart data={data} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="status" />
        <YAxis />
        <Tooltip />
        <Bar dataKey="count" fill="var(--chart-1)" />
      </BarChart>
    </ResponsiveContainer>
  )
}