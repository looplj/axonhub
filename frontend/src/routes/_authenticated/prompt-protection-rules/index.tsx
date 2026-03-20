import { createFileRoute } from '@tanstack/react-router';
import PromptProtectionRulesManagement from '@/features/prompt-protection-rules';

export const Route = createFileRoute('/_authenticated/prompt-protection-rules/')({
  component: PromptProtectionRulesManagement,
});
