'use client'

import { useTranslation } from 'react-i18next'
import {
  Line,
  LineChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  Legend,
  type TooltipProps,
} from 'recharts'
import { formatNumber } from '@/utils/format-number'
import { Skeleton } from '@/components/ui/skeleton'
import { ModelTokenTrend, ModelTokenTrendData } from '../data/dashboard'

interface ModelTokenChartProps {
  trends: ModelTokenTrend[]
  models: string[]
  dates: string[]
  isLoading?: boolean
}

const COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  '#8884d8',
  '#82ca9d',
  '#ffc658',
  '#ff7300',
  '#00ff00',
]

export function ModelTokenChart({ trends, models, dates, isLoading }: ModelTokenChartProps) {
  const { t } = useTranslation()

  if (isLoading) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <Skeleton className='h-[250px] w-full rounded-md' />
      </div>
    )
  }

  if (!trends || trends.length === 0) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noTrendData')}</div>
      </div>
    )
  }

  // Transform data for line chart
  const chartData = dates.map(date => {
    const dateData: any = { date }

    models.forEach(model => {
      const modelData = trends.find(t => t.modelId === model && t.date === date)
      if (modelData) {
        dateData[`${model}_input`] = modelData.inputTokens
        dateData[`${model}_output`] = modelData.outputTokens
        dateData[`${model}_total`] = modelData.totalTokens
      }
    })

    return dateData
  })

  // Create line configurations for each model
  const lineConfigs = models.map((model, index) => ({
    model,
    color: COLORS[index % COLORS.length],
    dataKey: `${model}_total`,
    strokeWidth: 2,
  }))

  type TokenTooltipProps = TooltipProps<number, string> & {
    active?: boolean
    payload?: Array<{
      name?: string
      value?: number
      color?: string
      dataKey?: string
    }>
    label?: string
  }

  const tooltipContent = (props: TokenTooltipProps) => {
    const { active, payload, label } = props

    if (!active || !payload?.length) return null

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium mb-2'>{label}</div>
        {payload.map((entry, index) => {
          const modelId = entry.dataKey?.replace('_total', '') || ''
          const inputTokens = entry.payload?.[`${modelId}_input`] || 0
          const outputTokens = entry.payload?.[`${modelId}_output`] || 0
          const totalTokens = entry.value || 0

          return (
            <div key={index} className='space-y-1'>
              <div className='flex items-center gap-2'>
                <span
                  className='inline-block w-3 h-3 rounded-full'
                  style={{ backgroundColor: entry.color }}
                />
                <span className='font-medium'>{modelId}</span>
              </div>
              <div className='ml-5 space-y-1 text-muted-foreground'>
                <div>Input: {formatNumber(inputTokens)}</div>
                <div>Output: {formatNumber(outputTokens)}</div>
                <div>Total: {formatNumber(totalTokens)}</div>
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <div className='space-y-4'>
      <ResponsiveContainer width='100%' height={350}>
        <LineChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
          <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
          <XAxis
            dataKey='date'
            tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(value) => formatNumber(value)}
          />
          <Tooltip
            content={tooltipContent}
            cursor={{ stroke: 'var(--border)', strokeWidth: 1 }}
          />
          <Legend
            wrapperStyle={{ paddingTop: '20px' }}
            iconType='line'
          />

          {lineConfigs.map((config) => (
            <Line
              key={config.model}
              type='monotone'
              dataKey={config.dataKey}
              stroke={config.color}
              strokeWidth={config.strokeWidth}
              dot={{ fill: config.color, strokeWidth: 2, r: 4 }}
              activeDot={{ r: 6, stroke: config.color, strokeWidth: 2 }}
              name={`${config.model} (Total)`}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>

      {/* Additional detailed charts */}
      {models.length <= 3 && ( // Only show detailed charts for few models to avoid clutter
        <div className='space-y-4'>
          <h4 className='text-sm font-medium'>{t('dashboard.charts.inputTokens')}</h4>
          <ResponsiveContainer width='100%' height={200}>
            <LineChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
              <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
              <XAxis
                dataKey='date'
                tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(value) => formatNumber(value)}
              />
              <Tooltip
                formatter={(value: number) => [formatNumber(value), 'Input Tokens']}
                labelStyle={{ color: 'var(--foreground)' }}
                contentStyle={{
                  backgroundColor: 'var(--background)',
                  border: '1px solid var(--border)',
                  borderRadius: '6px'
                }}
              />

              {lineConfigs.map((config) => (
                <Line
                  key={`${config.model}_input`}
                  type='monotone'
                  dataKey={`${config.model}_input`}
                  stroke={config.color}
                  strokeWidth={2}
                  dot={false}
                  name={`${config.model} (Input)`}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>

          <h4 className='text-sm font-medium'>{t('dashboard.charts.outputTokens')}</h4>
          <ResponsiveContainer width='100%' height={200}>
            <LineChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
              <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
              <XAxis
                dataKey='date'
                tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(value) => formatNumber(value)}
              />
              <Tooltip
                formatter={(value: number) => [formatNumber(value), 'Output Tokens']}
                labelStyle={{ color: 'var(--foreground)' }}
                contentStyle={{
                  backgroundColor: 'var(--background)',
                  border: '1px solid var(--border)',
                  borderRadius: '6px'
                }}
              />

              {lineConfigs.map((config) => (
                <Line
                  key={`${config.model}_output`}
                  type='monotone'
                  dataKey={`${config.model}_output`}
                  stroke={config.color}
                  strokeWidth={2}
                  dot={false}
                  name={`${config.model} (Output)`}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}