import type {
  APIKeyAutoDisableMode,
  APIKeyAutoDisableRule,
  BulkAutoDisableAction,
  BulkUpdateChannelAutoDisableInput,
  CapabilityPolicy,
  ChannelScheduledDisable,
} from './schema';

export type { BulkAutoDisableAction, BulkUpdateChannelAutoDisableInput };

export type AutoDisablePoliciesInput = {
  apiKeyAutoDisableMode?: APIKeyAutoDisableMode | null;
  apiKeyAutoDisableRules?: APIKeyAutoDisableRule[] | null;
};

export type ApiKeyAutoDisableRuleFormValue = {
  statusCodes?: number[] | null;
  keywordPatterns?: string[] | null;
  times: number;
  action: APIKeyAutoDisableRule['action'];
  disableDurationMinutes?: number | null;
  disableUntilCron?: string | null;
  disableUntilTimezone?: string | null;
};

export const RECOMMENDED_GLOBAL_AUTO_DISABLE_RULES: APIKeyAutoDisableRule[] = [
  { statusCodes: [401], times: 3, action: 'permanent_disable' },
  { statusCodes: [429], times: 3, action: 'temporary_disable', disableDurationMinutes: 30 },
];

export function effectiveAutoDisableMode(policies?: AutoDisablePoliciesInput | null): APIKeyAutoDisableMode {
  const mode = policies?.apiKeyAutoDisableMode;
  const hasRules = (policies?.apiKeyAutoDisableRules?.length ?? 0) > 0;
  if (mode === 'off') return 'off';
  if (mode === 'custom') return hasRules ? 'custom' : 'inherit';
  if (mode === 'inherit') return 'inherit';
  return hasRules ? 'custom' : 'inherit';
}

export function toApiKeyAutoDisableRuleFormValues(rules?: APIKeyAutoDisableRule[] | null): ApiKeyAutoDisableRuleFormValue[] {
  return (
    rules?.map((rule) => ({
      statusCodes: rule.statusCodes ?? [],
      keywordPatterns: rule.keywordPatterns ?? [],
      times: rule.times,
      action: rule.action,
      disableDurationMinutes: rule.action === 'temporary_disable' ? (rule.disableDurationMinutes ?? 30) : null,
      disableUntilCron: rule.action === 'disable_until_cron' ? (rule.disableUntilCron ?? '0 0 * * *') : null,
      disableUntilTimezone: rule.action === 'disable_until_cron' ? rule.disableUntilTimezone || 'UTC' : null,
    })) ?? []
  );
}

export function serializeApiKeyAutoDisableRules(rules: ApiKeyAutoDisableRuleFormValue[]): APIKeyAutoDisableRule[] {
  return rules.map((rule) => ({
    statusCodes: rule.statusCodes?.filter((code): code is number => code != null) ?? [],
    keywordPatterns: rule.keywordPatterns?.map((pattern) => pattern.trim()).filter(Boolean) ?? [],
    times: rule.times,
    action: rule.action,
    disableDurationMinutes: rule.action === 'temporary_disable' ? (rule.disableDurationMinutes ?? null) : null,
    disableUntilCron: rule.action === 'disable_until_cron' ? rule.disableUntilCron?.trim() || null : null,
    disableUntilTimezone: rule.action === 'disable_until_cron' ? rule.disableUntilTimezone?.trim() || null : null,
  }));
}

export type ScheduledDisableFormValue = {
  disableCron: string;
  disableTimezone: string;
  reenableMode: 'none' | 'opposite' | 'cron';
  reenableCron: string;
  reenableTimezone: string;
};

export function toScheduledDisableFormValue(value?: ChannelScheduledDisable | null): ScheduledDisableFormValue {
  const mode = value?.reenableMode === 'opposite' ? 'opposite' : value?.reenableMode === 'cron' ? 'cron' : 'none';
  // Opposite mode derives the re-enable from the disable cron's timezone, so
  // the stored re-enable fields are not used.
  const cronMode = mode === 'cron';
  return {
    disableCron: value?.disableCron ?? '',
    disableTimezone: value?.disableTimezone?.trim() || 'UTC',
    reenableMode: mode,
    reenableCron: cronMode ? (value?.reenableCron ?? '') : '',
    reenableTimezone: cronMode ? value?.reenableTimezone?.trim() || 'UTC' : 'UTC',
  };
}

export function availabilityPoliciesPayload(input: {
  stream?: CapabilityPolicy;
  mode: APIKeyAutoDisableMode;
  rules: ApiKeyAutoDisableRuleFormValue[];
  scheduledDisable?: ScheduledDisableFormValue | null;
}): {
  stream?: CapabilityPolicy;
  apiKeyAutoDisableMode: APIKeyAutoDisableMode;
  apiKeyAutoDisableRules: APIKeyAutoDisableRule[] | null;
  scheduledDisable: ChannelScheduledDisable | null;
  emptiedCustom: boolean;
} {
  let savedMode = input.mode;
  const serialized = serializeApiKeyAutoDisableRules(input.rules);
  let emptiedCustom = false;
  if (savedMode === 'custom' && serialized.length === 0) {
    savedMode = 'inherit';
    emptiedCustom = true;
  }

  const scheduledDisable: ChannelScheduledDisable | null =
    input.scheduledDisable && input.scheduledDisable.disableCron.trim() !== ''
      ? (() => {
          const form = input.scheduledDisable;
          const reenableMode = form.reenableMode === 'none' ? null : form.reenableMode;
          const cronMode = form.reenableMode === 'cron';
          return {
            disableCron: form.disableCron.trim(),
            disableTimezone: form.disableTimezone.trim() || null,
            reenableMode,
            reenableCron: cronMode ? form.reenableCron.trim() || null : null,
            reenableTimezone: cronMode ? form.reenableTimezone.trim() || null : null,
          };
        })()
      : null;

  return {
    stream: input.stream,
    apiKeyAutoDisableMode: savedMode,
    apiKeyAutoDisableRules: savedMode === 'inherit' ? null : serialized,
    scheduledDisable,
    emptiedCustom,
  };
}

export function buildBulkAutoDisableInput(input: {
  channelIDs: string[];
  action: BulkAutoDisableAction;
  rules: ApiKeyAutoDisableRuleFormValue[];
}): { ok: true; input: BulkUpdateChannelAutoDisableInput } | { ok: false; error: 'empty_rules' } {
  if (input.action === 'write_rules') {
    const rules = serializeApiKeyAutoDisableRules(input.rules);
    if (rules.length === 0) {
      return { ok: false, error: 'empty_rules' };
    }
    return { ok: true, input: { channelIDs: input.channelIDs, action: 'write_rules', rules } };
  }

  return { ok: true, input: { channelIDs: input.channelIDs, action: input.action } };
}
