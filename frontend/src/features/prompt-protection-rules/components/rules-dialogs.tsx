import { RulesActionDialog } from './rules-action-dialog';
import { RulesBulkDeleteDialog } from './rules-bulk-delete-dialog';
import { RulesBulkDisableDialog } from './rules-bulk-disable-dialog';
import { RulesBulkEnableDialog } from './rules-bulk-enable-dialog';
import { RulesDeleteDialog } from './rules-delete-dialog';

export function RulesDialogs() {
  return (
    <>
      <RulesActionDialog />
      <RulesDeleteDialog />
      <RulesBulkEnableDialog />
      <RulesBulkDisableDialog />
      <RulesBulkDeleteDialog />
    </>
  );
}
