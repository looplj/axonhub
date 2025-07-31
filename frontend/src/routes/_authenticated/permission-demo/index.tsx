import { createFileRoute } from '@tanstack/react-router'
import PermissionDemo from '@/features/permission-demo'

export const Route = createFileRoute('/_authenticated/permission-demo/')({
  component: PermissionDemo,
})