import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { CHANNEL_CONFIGS } from '../data/constants'
import type { ChannelTypeCount } from '../data/channels'

interface ChannelsTypeTabsProps {
  typeCounts: ChannelTypeCount[]
  selectedTab: string
  onTabChange: (tab: string) => void
}

interface GroupedTypeCount {
  prefix: string
  types: string[]
  totalCount: number
}

/**
 * Groups channel types by their prefix and aggregates counts
 * For example: deepseek (5) and deepseek_anthropic (3) -> deepseek (8)
 */
function groupTypesByPrefix(typeCounts: ChannelTypeCount[]): GroupedTypeCount[] {
  const groups = new Map<string, { types: string[]; totalCount: number }>()

  typeCounts.forEach(({ type, count }) => {
    // Find the base prefix (before the first underscore or the whole string)
    const prefix = type.split('_')[0]
    
    if (!groups.has(prefix)) {
      groups.set(prefix, { types: [], totalCount: 0 })
    }
    const group = groups.get(prefix)!
    group.types.push(type)
    group.totalCount += count
  })

  // Convert to array and sort by count (descending), then by prefix
  return Array.from(groups.entries())
    .map(([prefix, { types, totalCount }]) => ({
      prefix,
      types,
      totalCount,
    }))
    .sort((a, b) => b.totalCount - a.totalCount || a.prefix.localeCompare(b.prefix))
}

export function ChannelsTypeTabs({ typeCounts, selectedTab, onTabChange }: ChannelsTypeTabsProps) {
  const { t } = useTranslation()

  // Group types by prefix and get top 8
  const groupedTypes = useMemo(() => {
    const groups = groupTypesByPrefix(typeCounts)
    return groups.slice(0, 8)
  }, [typeCounts])

  // Calculate total count for "all" tab
  const totalCount = useMemo(() => {
    return typeCounts.reduce((sum, { count }) => sum + count, 0)
  }, [typeCounts])

  if (typeCounts.length === 0) {
    return null
  }

  // Get icon for a prefix
  const getIcon = (prefix: string) => {
    const config = CHANNEL_CONFIGS[prefix as keyof typeof CHANNEL_CONFIGS]
    return config?.icon
  }

  return (
    <div className="w-full mb-4">
      <div className="flex gap-1 border-b overflow-x-auto">
        {/* All tab */}
        <button
          onClick={() => onTabChange('all')}
          className={cn(
            'flex items-center gap-2 px-4 py-2.5 text-sm font-medium whitespace-nowrap transition-colors relative',
            selectedTab === 'all'
              ? 'text-foreground'
              : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {t('channels.tabs.all')} ({totalCount})
          {selectedTab === 'all' && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />
          )}
        </button>

        {/* Type tabs */}
        {groupedTypes.map(({ prefix, totalCount }) => {
          const Icon = getIcon(prefix)
          return (
            <button
              key={prefix}
              onClick={() => onTabChange(prefix)}
              className={cn(
                'flex items-center gap-2 px-4 py-2.5 text-sm font-medium whitespace-nowrap transition-colors relative',
                selectedTab === prefix
                  ? 'text-foreground'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {Icon && <Icon size={16} />}
              {t(`channels.types.${prefix}`)} ({totalCount})
              {selectedTab === prefix && (
                <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
