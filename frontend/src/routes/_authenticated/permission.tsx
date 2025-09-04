import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/permission')({
  component: () => <div>Permission Management</div>,
})