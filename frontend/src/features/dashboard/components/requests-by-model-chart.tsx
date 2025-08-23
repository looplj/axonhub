'use client'

import { useTranslation } from 'react-i18next'
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from 'recharts'
import { useRequestsByModel } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

const COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-1)'
]

export function RequestsByModelChart() {
  const { t } = useTranslation()
  const { data: modelData, isLoading, error } = useRequestsByModel()

  if (isLoading) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <Skeleton className="h-[250px] w-[250px] rounded-full" />
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
    value: item.count,
  }))

  return (
    <ResponsiveContainer width="100%" height={300}>
      <PieChart>
        <Pie
          data={data}
          cx="50%"
          cy="50%"
          labelLine={false}
          outerRadius={80}
          fill="#8884d8"
          dataKey="value"
        >
          {data.map((_entry, index) => (
            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
          ))}
        </Pie>
        <Tooltip />
        <Legend />
      </PieChart>
    </ResponsiveContainer>
  )
}