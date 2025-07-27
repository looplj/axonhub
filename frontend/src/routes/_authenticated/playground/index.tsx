import { createFileRoute } from '@tanstack/react-router'
import Playground from '@/features/playground'

export const Route = createFileRoute('/_authenticated/playground/')({
  component: Playground,
})