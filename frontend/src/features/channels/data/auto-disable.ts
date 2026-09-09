import type { APIKeyAutoDisableMode, APIKeyAutoDisableRule, CapabilityPolicy } from './schema';

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

export function availabilityPoliciesPayload(input: {
  stream?: CapabilityPolicy;
  mode: APIKeyAutoDisableMode;
  rules: ApiKeyAutoDisableRuleFormValue[];
}): {
  stream?: CapabilityPolicy;
  apiKeyAutoDisableMode: APIKeyAutoDisableMode;
  apiKeyAutoDisableRules: APIKeyAutoDisableRule[] | null;
  emptiedCustom: boolean;
} {
  let savedMode = input.mode;
  const serialized = serializeApiKeyAutoDisableRules(input.rules);
  let emptiedCustom = false;
  if (savedMode === 'custom' && serialized.length === 0) {
    savedMode = 'inherit';
    emptiedCustom = true;
  }

  return {
    stream: input.stream,
    apiKeyAutoDisableMode: savedMode,
    apiKeyAutoDisableRules: savedMode === 'inherit' ? null : serialized,
    emptiedCustom,
  };
}
