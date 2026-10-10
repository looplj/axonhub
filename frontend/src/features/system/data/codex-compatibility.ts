// Pure rules and state for the Codex compatibility settings tab.
// This module has no runtime imports so it can be unit-tested without a bundler.

export type CodexChannelStatus = 'enabled' | 'disabled' | 'archived';

export interface CodexCatalogChannel {
  readonly id: number;
  readonly name: string;
  readonly status: CodexChannelStatus;
}

export interface CodexCompatibilitySettings {
  readonly enabled: boolean;
  readonly channelID: number | null;
}

export type UpdateCodexCompatibilitySettingsInput = CodexCompatibilitySettings;

export interface CodexCatalogTestResult {
  readonly success: boolean;
  readonly modelCount: number;
  readonly upstreamStatus: number | null;
  readonly error: string | null;
}

// --- access --------------------------------------------------------------------------------------------

export interface CodexAccess {
  readonly canListChannels: boolean;
  readonly canEdit: boolean;
}

export type CodexSettingsViewState = 'permission' | 'loading' | 'loadFailed' | 'ready';

export function codexSettingsViewState(input: {
  readonly canReadSettings: boolean;
  readonly isLoading: boolean;
  readonly hasData: boolean;
}): CodexSettingsViewState {
  if (!input.canReadSettings) return 'permission';
  if (input.isLoading) return 'loading';
  return input.hasData ? 'ready' : 'loadFailed';
}

// Listing needs settings and channel read, and every write or test needs the same plus settings write,
// because the backend authorizes a saved channel reference with the caller's channel read scope.
export function codexAccess(hasScope: (scope: string) => boolean): CodexAccess {
  const canListChannels = hasScope('read_settings') && hasScope('read_channels');
  return { canListChannels, canEdit: canListChannels && hasScope('write_settings') };
}

// --- channel selection ---------------------------------------------------------------------------------

// The backend lists channels in storage order, which can change between loads; keep the options stable.
export function sortCodexChannels(channels: readonly CodexCatalogChannel[]): CodexCatalogChannel[] {
  return [...channels].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base', numeric: true }) || a.id - b.id);
}

export const NO_CODEX_CHANNEL_VALUE = 'none';

export function channelSelectValue(channelID: number | null): string {
  return channelID === null ? NO_CODEX_CHANNEL_VALUE : String(channelID);
}

export function parseChannelSelectValue(value: string): number | null {
  if (!/^[1-9]\d*$/.test(value)) {
    return null;
  }
  const channelID = Number(value);
  return Number.isSafeInteger(channelID) ? channelID : null;
}

interface ResolvedSelection {
  readonly channelID: number | null;
  readonly referenceMissing: boolean;
}

// A saved reference to a deleted channel must not be silently re-submitted: the backend would reject it.
// Without a channel list (no permission) the reference is kept as is because nothing can be proven.
export function resolveSelection(
  saved: CodexCompatibilitySettings,
  channels: readonly CodexCatalogChannel[] | undefined
): ResolvedSelection {
  if (saved.channelID === null || channels === undefined) {
    return { channelID: saved.channelID, referenceMissing: false };
  }
  const listed = channels.some((channel) => channel.id === saved.channelID);
  return listed ? { channelID: saved.channelID, referenceMissing: false } : { channelID: null, referenceMissing: true };
}

// --- form rules ----------------------------------------------------------------------------------------

type CodexFormIssue = 'channel_required';

function codexFormIssue(form: CodexCompatibilitySettings): CodexFormIssue | null {
  return form.enabled && form.channelID === null ? 'channel_required' : null;
}

export function toUpdateInput(form: CodexCompatibilitySettings): UpdateCodexCompatibilitySettingsInput {
  return { enabled: form.enabled, channelID: form.channelID };
}

// --- catalog test state --------------------------------------------------------------------------------

export type CodexTestState =
  | { readonly phase: 'idle' }
  | { readonly phase: 'running'; readonly seq: number; readonly channelID: number }
  | { readonly phase: 'succeeded'; readonly channelID: number; readonly modelCount: number }
  | { readonly phase: 'failed'; readonly channelID: number; readonly message: string | null; readonly upstreamStatus: number | null };

type CodexTestAction =
  | { readonly type: 'started'; readonly seq: number; readonly channelID: number }
  | { readonly type: 'finished'; readonly seq: number; readonly result: CodexCatalogTestResult }
  | { readonly type: 'rejected'; readonly seq: number; readonly message: string }
  | { readonly type: 'cancelled' }
  | { readonly type: 'selectionChanged'; readonly channelID: number | null };

export const idleCodexTest: CodexTestState = { phase: 'idle' };

function assertNever(action: never): never {
  throw new Error(`Unhandled Codex test action: ${JSON.stringify(action)}`);
}

// Responses are applied only to the exact request that is still running, so a late answer for a
// cancelled, superseded, or switched-away selection can never surface as the current result.
export function codexTestReducer(state: CodexTestState, action: CodexTestAction): CodexTestState {
  switch (action.type) {
    case 'started':
      return { phase: 'running', seq: action.seq, channelID: action.channelID };
    case 'finished':
      if (state.phase !== 'running' || state.seq !== action.seq) {
        return state;
      }
      return action.result.success
        ? { phase: 'succeeded', channelID: state.channelID, modelCount: action.result.modelCount }
        : { phase: 'failed', channelID: state.channelID, message: action.result.error, upstreamStatus: action.result.upstreamStatus };
    case 'rejected':
      if (state.phase !== 'running' || state.seq !== action.seq) {
        return state;
      }
      return { phase: 'failed', channelID: state.channelID, message: action.message, upstreamStatus: null };
    case 'cancelled':
      return state.phase === 'running' ? idleCodexTest : state;
    case 'selectionChanged':
      return state.phase !== 'idle' && state.channelID !== action.channelID ? idleCodexTest : state;
    default:
      return assertNever(action);
  }
}

export function visibleCodexTest(state: CodexTestState, selectedChannelID: number | null): CodexTestState {
  return state.phase === 'idle' || state.channelID === selectedChannelID ? state : idleCodexTest;
}

// --- derived controls ----------------------------------------------------------------------------------

interface CodexControlsInput {
  readonly access: CodexAccess;
  readonly form: CodexCompatibilitySettings;
  readonly saved: CodexCompatibilitySettings;
  readonly saving: boolean;
  readonly test: CodexTestState;
}

interface CodexControls {
  readonly issue: CodexFormIssue | null;
  readonly canSave: boolean;
  readonly canTest: boolean;
}

// Saving ignores the test outcome and testing ignores the global switch and channel status:
// the test reads the selected channel's own catalog and never gates or enables anything.
export function codexFormControls({ access, form, saved, saving, test }: CodexControlsInput): CodexControls {
  const issue = codexFormIssue(form);
  const changed = form.enabled !== saved.enabled || form.channelID !== saved.channelID;
  return {
    issue,
    canSave: access.canEdit && changed && issue === null && !saving,
    canTest: access.canEdit && form.channelID !== null && test.phase !== 'running',
  };
}
