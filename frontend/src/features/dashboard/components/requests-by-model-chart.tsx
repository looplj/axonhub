'use client'

import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type TooltipProps,
} from 'recharts'
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

  const total = modelData.reduce((sum, item) => sum + item.count, 0)
  const chartData = modelData
    .map((item) => ({
      name: item.modelId,
      value: item.count,
    }))
    .sort((a, b) => b.value - a.value)

  const legendItems = chartData.map((item, index) => ({
    ...item,
    index: index + 1,
    color: COLORS[index % COLORS.length],
    percent: total ? (item.value / total) * 100 : 0,
  }))

  type ModelTooltipProps = TooltipProps<number, string> & {
    payload?: Array<{
      name?: string
      value?: number
    }>
  }

  const tooltipContent = (props: ModelTooltipProps) => {
    const payload = props.payload

    if (!props.active || !payload?.length) return null

    const [{ name, value }] = payload
    const percent = total ? ((value ?? 0) / total) * 100 : 0

    return (
      <div className="rounded-md border bg-background/90 px-3 py-2 text-xs shadow-sm backdrop-blur">
        <div className="text-sm font-medium text-foreground">{name}</div>
        <div className="text-muted-foreground">
          {value?.toLocaleString()} ({percent.toFixed(0)}%)
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <ResponsiveContainer width="100%" height={320}>
        <BarChart data={chartData} barSize={32}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="name" hide />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={60}
            tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
          />
          <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
          <Bar dataKey="value" radius={[6, 6, 0, 0]}>
            {chartData.map((_, index) => (
              <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>

      <div className="grid gap-4 sm:grid-cols-2">
        {legendItems.map((item) => (
          <div key={item.name} className="flex items-start gap-3">
            <span className="text-sm font-semibold text-muted-foreground">
              {item.index.toString().padStart(2, '0')}.
            </span>
            <div className="flex flex-1 items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <span
                  className="h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: item.color }}
                />
                <span className="text-sm font-medium text-foreground">{item.name}</span>
              </div>
              <div className="text-right">
                <div className="text-sm font-medium text-foreground">
                  {item.value.toLocaleString()}
                </div>
                <div className="text-xs text-muted-foreground">{item.percent.toFixed(0)}%</div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}