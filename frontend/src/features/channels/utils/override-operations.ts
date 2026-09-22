// Shared helpers for editing channel override header/body operations.
// Used by the per-channel override dialog and the standalone template manager.
import { z } from 'zod';
import type { TFunction } from 'i18next';
import { OverrideOperation, overrideOperationSchema } from '../data/schema';

export type OpType = OverrideOperation['op'];

export function parseValueForDisplay(value: any): string {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}

export const AUTH_HEADER_KEYS = ['authorization', 'proxy-authorization', 'x-api-key', 'x-api-secret', 'x-api-token'];

export const overrideFormSchema = z.object({
  headerOverrideOperations: z.array(overrideOperationSchema).optional(),
  bodyOverrideOperations: z.array(overrideOperationSchema).optional(),
});

export type OverrideFormValues = z.infer<typeof overrideFormSchema>;

export const OP_LABELS: Record<OpType, string> = {
  set: 'channels.dialogs.settings.overrides.body.opSet',
  set_if_absent: 'channels.dialogs.settings.overrides.body.opSetIfAbsent',
  delete: 'channels.dialogs.settings.overrides.body.opDelete',
  rename: 'channels.dialogs.settings.overrides.body.opRename',
  copy: 'channels.dialogs.settings.overrides.body.opCopy',
  array_append: 'channels.dialogs.settings.overrides.body.opArrayAppend',
  array_prepend: 'channels.dialogs.settings.overrides.body.opArrayPrepend',
  array_insert: 'channels.dialogs.settings.overrides.body.opArrayInsert',
  array_remove: 'channels.dialogs.settings.overrides.body.opArrayRemove',
};

// Body operations support array manipulation; headers only support scalar set/delete/rename/copy.
export const BODY_OP_TYPES: OpType[] = [
  'set',
  'set_if_absent',
  'delete',
  'rename',
  'copy',
  'array_append',
  'array_prepend',
  'array_insert',
  'array_remove',
];
export const HEADER_OP_TYPES: OpType[] = ['set', 'delete', 'rename', 'copy'];

const ARRAY_OPS: OpType[] = ['array_append', 'array_prepend', 'array_insert', 'array_remove'];
const ARRAY_INSERT_OPS: OpType[] = ['array_append', 'array_prepend', 'array_insert'];

export function isArrayOp(op: OpType | undefined): boolean {
  return !!op && ARRAY_OPS.includes(op);
}

export function isArrayInsertOp(op: OpType | undefined): boolean {
  return !!op && ARRAY_INSERT_OPS.includes(op);
}

export function isValidBodyOp(op: OverrideOperation): boolean {
  if (op.op === 'set_if_absent') return !!op.path?.trim() && parseValueForDisplay(op.value).trim() !== '';
  if (op.op === 'set' || op.op === 'delete') return !!op.path?.trim();
  if (op.op === 'rename' || op.op === 'copy') return !!op.from?.trim() && !!op.to?.trim();
  if (op.op === 'array_append' || op.op === 'array_prepend') return !!op.path?.trim();
  if (op.op === 'array_insert') return !!op.path?.trim() && typeof op.index === 'number';
  if (op.op === 'array_remove') return !!op.path?.trim() && !!op.match?.path?.trim() && !!op.match?.eq?.trim();
  return false;
}

export function isValidHeaderOp(op: OverrideOperation): boolean {
  if (op.op === 'set' || op.op === 'delete') return !!op.path?.trim();
  if (op.op === 'rename' || op.op === 'copy') return !!op.from?.trim() && !!op.to?.trim();
  return false;
}

/**
 * Validates header and body operations, returning the first human-readable
 * error message to show, or null when everything is valid.
 */
export function validateOverrideOperations(
  t: TFunction,
  headerOps: OverrideOperation[],
  bodyOps: OverrideOperation[]
): string | null {
  for (let i = 0; i < bodyOps.length; i++) {
    const op = bodyOps[i];
    if (op.op === 'set' || op.op === 'set_if_absent' || op.op === 'delete' || isArrayOp(op.op)) {
      if (!op.path?.trim()) {
        return t('channels.dialogs.settings.overrides.validation.emptyPath', { index: i + 1, op: op.op });
      }
    }
    if (op.op === 'set_if_absent' && parseValueForDisplay(op.value).trim() === '') {
      return t('channels.dialogs.settings.overrides.validation.missingValue', { index: i + 1, op: op.op });
    }
    if (op.op === 'rename' || op.op === 'copy') {
      if (!op.from?.trim() || !op.to?.trim()) {
        return t('channels.dialogs.settings.overrides.validation.emptyFromTo', { index: i + 1, op: op.op });
      }
    }
    if (op.op === 'array_insert' && typeof op.index !== 'number') {
      return t('channels.dialogs.settings.overrides.validation.missingIndex', { index: i + 1 });
    }
    if (op.op === 'array_remove') {
      if (!op.match?.path?.trim() || !op.match?.eq?.trim()) {
        return t('channels.dialogs.settings.overrides.validation.missingMatch', { index: i + 1 });
      }
    }
  }

  for (let i = 0; i < headerOps.length; i++) {
    const op = headerOps[i];
    if (op.op === 'set' || op.op === 'delete') {
      if (!op.path?.trim()) {
        return t('channels.dialogs.settings.overrides.validation.emptyHeaderPath', { index: i + 1, op: op.op });
      }
    }
    if (op.op === 'rename' || op.op === 'copy') {
      if (!op.from?.trim() || !op.to?.trim()) {
        return t('channels.dialogs.settings.overrides.validation.emptyHeaderFromTo', { index: i + 1, op: op.op });
      }
    }
  }

  return null;
}

