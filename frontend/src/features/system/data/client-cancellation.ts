// Client cancellation cooldown helpers.
//
// The cooldown duration is edited with a select that mixes preset durations with
// a free "custom" entry. The select is bound to the stored number, so the custom
// entry cannot be represented by the value alone: picking it must keep the
// current minutes and switch the control into free-input mode.

export const CLIENT_CANCELLATION_COOLDOWN_PRESETS = [5, 10, 20, 30, 60] as const;

export const CLIENT_CANCELLATION_COOLDOWN_MIN_MINUTES = 1;
export const CLIENT_CANCELLATION_COOLDOWN_MAX_MINUTES = 7 * 24 * 60;

export interface CooldownSelection {
  /** Value handed to the select control. */
  value: string;
  /** Whether the free-input minutes field must be visible. */
  custom: boolean;
}

export interface CooldownSelectionChange {
  /** New cooldown duration, or undefined to keep the current one. */
  minutes?: number;
  /** Whether the control stays in free-input mode. */
  custom: boolean;
}

export function clampCooldownMinutes(minutes: number): number {
  if (!Number.isFinite(minutes)) {
    return CLIENT_CANCELLATION_COOLDOWN_MIN_MINUTES;
  }

  const truncated = Math.trunc(minutes);

  return Math.min(CLIENT_CANCELLATION_COOLDOWN_MAX_MINUTES, Math.max(CLIENT_CANCELLATION_COOLDOWN_MIN_MINUTES, truncated));
}

export function isPresetCooldownMinutes(minutes: number): boolean {
  return (CLIENT_CANCELLATION_COOLDOWN_PRESETS as readonly number[]).includes(minutes);
}

/**
 * Resolves the select value from the stored minutes plus the local custom flag.
 * A stored value that matches no preset is always shown as custom.
 */
export function resolveCooldownSelection(minutes: number, custom: boolean): CooldownSelection {
  if (!custom && isPresetCooldownMinutes(minutes)) {
    return { value: String(minutes), custom: false };
  }

  return { value: 'custom', custom: true };
}

/**
 * Applies a select change. Selecting "custom" keeps the current duration so the
 * operator can edit it instead of losing the value.
 */
export function applyCooldownSelectionChange(selected: string): CooldownSelectionChange {
  if (selected === 'custom') {
    return { custom: true };
  }

  // Radix fires onValueChange with an empty string when the controlled value
  // matches no item. Number('') is 0, which must not silently become 1 minute.
  const minutes = Number(selected);
  if (!Number.isFinite(minutes) || minutes <= 0) {
    return { custom: true };
  }

  return { minutes: clampCooldownMinutes(minutes), custom: false };
}
/**
 * AxonHub exposes entity ids as "gid://axonhub/<type>/<id>" GUIDs, while the
 * client-cancellation project override stores a plain numeric project id.
 * Numeric strings and plain numbers are accepted as-is.
 */
export function projectIDToNumber(id: string | number | null | undefined): number {
  if (typeof id === 'number') {
    return Number.isFinite(id) ? Math.trunc(id) : 0;
  }

  if (!id) {
    return 0;
  }

  const trimmed = id.trim();
  const guidMatch = /^gid:\/\/axonhub\/[^/]+\/(\d+)$/.exec(trimmed);
  if (guidMatch) {
    return Number(guidMatch[1]);
  }

  if (/^\d+$/.test(trimmed)) {
    return Number(trimmed);
  }

  return 0;
}
