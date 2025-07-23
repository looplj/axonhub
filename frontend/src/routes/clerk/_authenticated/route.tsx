import { createFileRoute } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/authenticated-layout'

export const Route = createFileRoute('/clerk/_authenticated')({
  component: AuthenticatedLayout,
})
