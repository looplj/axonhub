import type { Column, Table } from '@tanstack/react-table';

export const NON_REORDERABLE_COLUMN_IDS = ['details', 'status', 'source'] as const;

export function isReorderableColumn<TData>(column: Column<TData>): boolean {
  if ((NON_REORDERABLE_COLUMN_IDS as readonly string[]).includes(column.id)) return false;
  const header = column.columnDef.header;
  return typeof header === 'function' || typeof header === 'string';
}

export function getReorderableColumnIds<TData>(table: Table<TData>): string[] {
  return table.getAllLeafColumns().filter(isReorderableColumn).map((c) => c.id);
}

export interface ReconcileColumnOrderParams {
  allColumnIds: string[];
  reorderableColumnIds: string[];
  storedOrder: string[] | null;
  pinnedLast?: string[];
  pinned?: string[];
}

export function reconcileColumnOrder({
  allColumnIds,
  reorderableColumnIds,
  storedOrder,
  pinnedLast = [],
  pinned = [],
}: ReconcileColumnOrderParams): string[] {
  const valid = Array.from(new Set((storedOrder ?? []).filter((id) => reorderableColumnIds.includes(id))));
  const missing = reorderableColumnIds.filter((id) => !valid.includes(id));
  const ordered = [...valid, ...missing];
  const pinnedValid = pinned.filter((id) => allColumnIds.includes(id));
  const pinnedLastValid = pinnedLast.filter((id) => allColumnIds.includes(id));
  return [...ordered, ...pinnedValid, ...pinnedLastValid];
}

export function readColumnOrder(storageKey: string, version: number): string[] | null {
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    const obj = parsed as { v?: number; order?: unknown };
    if (obj && obj.v === version && Array.isArray(obj.order)) return obj.order as string[];
  } catch {
    /* corrupt or unavailable storage */
  }
  return null;
}

export function writeColumnOrder(storageKey: string, version: number, order: string[]): void {
  try {
    localStorage.setItem(storageKey, JSON.stringify({ v: version, order }));
  } catch {
    /* quota / unavailable storage */
  }
}
