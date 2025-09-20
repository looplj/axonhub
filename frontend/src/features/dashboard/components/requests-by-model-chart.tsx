'use client'

import { useTranslation } from 'react-i18next'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, ResponsiveContainer, Tooltip } from 'recharts'
import { useRequestsByModel } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

const CHART_COLOR = 'var(--chart-1)'

export function RequestsByModelChart() {
  const { t } = useTranslation()
  const { data: modelData, isLoading, error } = useRequestsByModel()

  if (isLoading) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <Skeleton className="h-[250px] w-full rounded-md" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <div className="text-red-500 text-sm">
          {t('dashboard.charts.errorLoadingModelData')} {error.message}
        </div>
      </div>
    )
  }

  if (!modelData || modelData.length === 0) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <div className="text-muted-foreground text-sm">
          {t('dashboard.charts.noModelData')}
        </div>
      </div>
    )
  }

  const data = modelData.map((item) => ({
    name: item.modelId,
    count: item.count,
  }))

  return (
    <ResponsiveContainer width="100%" height={300}>
      <BarChart
        data={data}
        margin={{
          top: 20,
          right: 30,
          left: 20,
          bottom: 5,
        }}
      >
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis 
          dataKey="name" 
          angle={-45}
          textAnchor="end"
          height={80}
          fontSize={12}
        />
        <YAxis />
        <Tooltip />
        <Bar 
          dataKey="count" 
          fill={CHART_COLOR}
          radius={[4, 4, 0, 0]}
        />
      </BarChart>
    </ResponsiveContainer>
  )
}