'use client'

import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from 'recharts'
import { useRequestsByChannel } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884D8', '#82CA9D']

export function RequestsByChannelChart() {
  const { data: channelData, isLoading, error } = useRequestsByChannel()

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
          Error loading channel data: {error.message}
        </div>
      </div>
    )
  }

  if (!channelData || channelData.length === 0) {
    return (
      <div className="h-[300px] flex items-center justify-center">
        <div className="text-muted-foreground text-sm">
          No channel data available
        </div>
      </div>
    )
  }

  const data = channelData.map((item) => ({
    name: item.channelName,
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
          label={({ name, percent }) => `${name} ${percent ? (percent * 100).toFixed(0) : 0}%`}
          outerRadius={80}
          fill="#8884d8"
          dataKey="value"
        >
          {data.map((entry, index) => (
            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
          ))}
        </Pie>
        <Tooltip />
        <Legend />
      </PieChart>
    </ResponsiveContainer>
  )
}