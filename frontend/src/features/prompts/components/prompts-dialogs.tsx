import { PromptsActionDialog } from './prompts-action-dialog';
import { PromptsDeleteDialog } from './prompts-delete-dialog';
import { PromptsBulkEnableDialog } from './prompts-bulk-enable-dialog';
import { PromptsBulkDisableDialog } from './prompts-bulk-disable-dialog';
import { PromptsBulkDeleteDialog } from './prompts-bulk-delete-dialog';

export function PromptsDialogs() {
  return (
    <>
      <PromptsActionDialog />
      <PromptsDeleteDialog />
      <PromptsBulkEnableDialog />
      <PromptsBulkDisableDialog />
      <PromptsBulkDeleteDialog />
    </>
  );
}
