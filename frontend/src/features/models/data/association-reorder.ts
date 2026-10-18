// Rewrites association priorities so the on-screen order and the numeric
// priority agree: the first row becomes 0, the second 1, and so on.
//
// The model association dialog renders rows sorted by priority, so dragging a
// row to a new position is expressed purely as a priority rewrite — the
// underlying field array is never reordered, which keeps each row's component
// state (expanded sections, search boxes) attached to the right rule.
export interface AssociationOrderEntry {
  // Index of the row within the form's field array.
  index: number;
  // Stable identity used for drag matching (react-hook-form field id).
  id: string;
}

// Given the currently displayed order and the dragged/over ids, returns the
// index→priority assignments for the new order. Returns null when nothing moved
// or either id is unknown, so callers can leave the form untouched.
export function computeAssociationReorder(
  ordered: AssociationOrderEntry[],
  activeId: string,
  overId: string
): Array<{ index: number; priority: number }> | null {
  if (activeId === overId) {
    return null;
  }

  const from = ordered.findIndex((entry) => entry.id === activeId);
  const to = ordered.findIndex((entry) => entry.id === overId);
  if (from === -1 || to === -1) {
    return null;
  }

  const reordered = [...ordered];
  const [moved] = reordered.splice(from, 1);
  reordered.splice(to, 0, moved);

  return reordered.map((entry, position) => ({ index: entry.index, priority: position }));
}
