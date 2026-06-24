import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import PromptProtectionRulesManagement from '@/features/prompt-protection-rules';

function ProtectedPromptProtectionRules() {
  return (
    <RouteGuard requiredScopes={['read_channels']} scopeLevel="system">
      <PromptProtectionRulesManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/prompt-protection-rules/')({
  component: ProtectedPromptProtectionRules,
});
