import { createFileRoute } from '@tanstack/react-router'
import ChannelsManagement from '@/features/channels'

export const Route = createFileRoute('/_authenticated/channels/')({
  component: ChannelsManagement,
})