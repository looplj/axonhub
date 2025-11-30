import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { BarChart4, RefreshCw, Download, TrendingUp } from 'lucide-react'

import { formatNumber } from '@/utils/format-number'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useModelTokenStats } from '../data/dashboard'
import { ModelTokenChart } from './model-token-chart'
import { ModelTokenTable } from './model-token-table'

interface ModelTokenStatsCardProps {
  availableModels?: string[]
  defaultModels?: string[]
  autoRefresh?: boolean
}

export function ModelTokenStatsCard({
  availableModels = [],
  defaultModels = [],
  autoRefresh = false
}: ModelTokenStatsCardProps) {
  const { t } = useTranslation()
  const [selectedModels, setSelectedModels] = useState<string[]>(defaultModels)
  const [period, setPeriod] = useState<'day' | 'week' | 'month'>('day')
  const [isRefreshing, setIsRefreshing] = useState(false)

  const { data: stats, isLoading, error, refetch } = useModelTokenStats(
    selectedModels.length > 0 ? selectedModels : undefined,
    period,
    undefined // Use current date
  )

  // Auto refresh functionality
  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      refetch()
    }, 30000) // Refresh every 30 seconds

    return () => clearInterval(interval)
  }, [autoRefresh, refetch])

  const handleRefresh = async () => {
    setIsRefreshing(true)
    await refetch()
    setIsRefreshing(false)
  }

  const handleExportCSV = () => {
    if (!stats?.currentPeriod || stats.currentPeriod.length === 0) return

    const headers = ['Model', 'Period', 'Date', 'Input Tokens', 'Output Tokens', 'Cached Tokens', 'Total Tokens']
    const rows = stats.currentPeriod.map(stat => [
      stat.modelId,
      stat.period,
      stat.date,
      stat.totalInputTokens.toString(),
      stat.totalOutputTokens.toString(),
      stat.totalCachedTokens.toString(),
      stat.totalTokens.toString()
    ])

    const csvContent = [headers, ...rows].map(row => row.join(',')).join('\n')
    const blob = new Blob([csvContent], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `model-token-stats-${period}-${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[200px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <Skeleton className='mb-4 h-8 w-[120px]' />
          <Skeleton className='mb-2 h-32 w-full' />
          <Skeleton className='h-24 w-full' />
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.modelTokenStats')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='bg-primary/10 text-primary flex h-9 w-9 items-center justify-center rounded-full dark:bg-primary/20'>
            <TrendingUp className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.modelTokenStats')}</CardTitle>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            variant='ghost'
            size='sm'
            onClick={handleRefresh}
            disabled={isRefreshing}
            className='h-8 w-8 p-0'
          >
            <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
          </Button>
          <Button
            variant='ghost'
            size='sm'
            onClick={handleExportCSV}
            disabled={!stats?.currentPeriod || stats.currentPeriod.length === 0}
            className='h-8 w-8 p-0'
          >
            <Download className='h-4 w-4' />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-4'>
          {/* Controls */}
          <div className='flex flex-wrap items-center gap-2'>
            <Select value={period} onValueChange={(value) => setPeriod(value as 'day' | 'week' | 'month')}>
              <SelectTrigger className='w-[120px] h-8'>
                <SelectValue placeholder={t('dashboard.stats.selectPeriod')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='day'>{t('dashboard.stats.day')}</SelectItem>
                <SelectItem value='week'>{t('dashboard.stats.week')}</SelectItem>
                <SelectItem value='month'>{t('dashboard.stats.month')}</SelectItem>
              </SelectContent>
            </Select>

            {availableModels.length > 0 && (
              <Select
                value={selectedModels[0] || ''}
                onValueChange={(value) => setSelectedModels(value ? [value] : [])}
              >
                <SelectTrigger className='w-[150px] h-8'>
                  <SelectValue placeholder={t('dashboard.stats.selectModel')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value=''>All Models</SelectItem>
                  {availableModels.map(model => (
                    <SelectItem key={model} value={model}>{model}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {/* Stats Overview */}
          {stats?.currentPeriod && stats.currentPeriod.length > 0 ? (
            <div className='grid grid-cols-2 md:grid-cols-4 gap-4'>
              {stats.currentPeriod.map((stat) => (
                <div key={stat.modelId} className='rounded-lg border p-3'>
                  <div className='text-sm font-medium text-muted-foreground mb-1'>
                    {stat.modelId}
                  </div>
                  <div className='text-2xl font-bold'>
                    {formatNumber(stat.totalTokens)}
                  </div>
                  <div className='text-xs text-muted-foreground mt-1'>
                    {formatNumber(stat.totalInputTokens)} in • {formatNumber(stat.totalOutputTokens)} out
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className='text-center py-8 text-muted-foreground'>
              {selectedModels.length === 0 ?
                t('dashboard.stats.selectModelPrompt') :
                t('dashboard.stats.noDataAvailable')
              }
            </div>
          )}

          {/* Tabs for detailed view */}
          {stats && stats.currentPeriod.length > 0 && (
            <Tabs defaultValue='chart' className='w-full'>
              <TabsList className='grid w-full grid-cols-2'>
                <TabsTrigger value='chart'>{t('dashboard.stats.chart')}</TabsTrigger>
                <TabsTrigger value='table'>{t('dashboard.stats.table')}</TabsTrigger>
              </TabsList>
              <TabsContent value='chart' className='mt-4'>
                <ModelTokenChart
                  trends={stats.trends.trends}
                  models={stats.trends.models}
                  dates={stats.trends.dates}
                />
              </TabsContent>
              <TabsContent value='table' className='mt-4'>
                <ModelTokenTable
                  currentPeriod={stats.currentPeriod}
                  onExport={handleExportCSV}
                />
              </TabsContent>
            </Tabs>
          )}
        </div>
      </CardContent>
    </Card>
  )
}