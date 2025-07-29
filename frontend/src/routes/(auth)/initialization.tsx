import { createFileRoute } from '@tanstack/react-router'
import Initialization from '@/features/auth/initialization'

export const Route = createFileRoute('/(auth)/initialization')({
  component: Initialization,
})