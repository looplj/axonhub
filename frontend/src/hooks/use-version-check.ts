import { useEffect, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { graphqlRequest } from '@/gql/graphql'
import { useAuthStore } from '@/stores/authStore'
import { CHECK_FOR_UPDATE_QUERY, type VersionCheck } from '@/features/system/data/system'
import i18n from '@/lib/i18n'

const VERSION_CHECK_STORAGE_KEY = 'axonhub_dismissed_version'
const VERSION_CHECK_TIMESTAMP_KEY = 'axonhub_version_check_timestamp'
const VERSION_CHECK_INTERVAL = 10 * 60 * 1000 // 10 minutes in milliseconds

/**
 * Hook to check for new versions and show toast notification.
 * Only shows to owners and only once per version.
 */
export function useVersionCheck() {
  const user = useAuthStore((state) => state.auth.user)
  const isOwner = user?.isOwner ?? false

  const shouldCheckVersion = useCallback(() => {
    if (!isOwner) return false

    const lastCheckTime = localStorage.getItem(VERSION_CHECK_TIMESTAMP_KEY)
    if (!lastCheckTime) return true

    const timeSinceLastCheck = Date.now() - parseInt(lastCheckTime, 10)
    return timeSinceLastCheck >= VERSION_CHECK_INTERVAL
  }, [isOwner])

  const { data: updateCheck } = useQuery({
    queryKey: ['versionCheck'],
    queryFn: async () => {
      const data = await graphqlRequest<{ checkForUpdate: VersionCheck }>(CHECK_FOR_UPDATE_QUERY)
      return data.checkForUpdate
    },
    //@ts-ignore
    onSuccess: () => {
      // Store the timestamp after successful check
      localStorage.setItem(VERSION_CHECK_TIMESTAMP_KEY, Date.now().toString())
    },
    enabled: shouldCheckVersion(),
    retry: false,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnMount: false,
    refetchOnReconnect: false,
  })

  const showUpdateToast = useCallback((latestVersion: string, releaseUrl: string) => {
    toast.info(i18n.t('system.about.updateCheck.newVersionAvailable'), {
      description: `${i18n.t('system.about.updateCheck.latestVersion')}: ${latestVersion}`,
      duration: 10000,
      action: {
        label: i18n.t('system.about.updateCheck.viewRelease'),
        onClick: () => {
          window.open(releaseUrl, '_blank', 'noopener,noreferrer')
        },
      },
    })
  }, [])

  useEffect(() => {
    if (!isOwner || !updateCheck) return

    if (!updateCheck.hasUpdate) return

    // Check if this version was already dismissed
    const dismissedVersion = localStorage.getItem(VERSION_CHECK_STORAGE_KEY)
    if (dismissedVersion === updateCheck.latestVersion) return

    // Show toast and mark as shown
    showUpdateToast(updateCheck.latestVersion, updateCheck.releaseUrl)
    localStorage.setItem(VERSION_CHECK_STORAGE_KEY, updateCheck.latestVersion)
  }, [isOwner, updateCheck, showUpdateToast])
}
