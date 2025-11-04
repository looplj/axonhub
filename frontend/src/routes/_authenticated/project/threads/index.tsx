import { createFileRoute } from '@tanstack/react-router'
import ThreadsManagement from '@/features/threads'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedProjectThreads() {
  return (
    <ProjectGuard>
      <ThreadsManagement />
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/threads/')({
  component: ProtectedProjectThreads,
})
