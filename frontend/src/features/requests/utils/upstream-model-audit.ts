import type { TFunction } from 'i18next';

interface ModelAuditExecution {
  status?: 'pending' | 'processing' | 'completed' | 'failed' | 'canceled';
  modelID?: string | null;
  upstreamModelID?: string | null;
}

type ModelAuditStatus = 'matched' | 'mismatched' | 'unknown';

export type ModelAuditVerdictTone = 'success' | 'danger' | 'pending' | 'muted';

export interface ModelAuditVerdict {
  tone: ModelAuditVerdictTone;
  message: string;
}

export const MODEL_AUDIT_VERDICT_CLASS: Record<ModelAuditVerdictTone, string> = {
  success: 'font-mono text-xs text-emerald-600 dark:text-emerald-400',
  danger: 'text-xs font-medium text-destructive',
  pending: 'text-xs font-medium text-sky-700 dark:text-sky-300',
  muted: 'text-muted-foreground text-xs',
};

export interface ModelAuditSummary {
  status: ModelAuditStatus;
  matchedUpstreamIds: string[];
  upstreamModelIds: string[];
  mismatchedModelIds: string[];
  equalModelIds: string[];
  unknownCount: number;
  comparedCount: number;
}

// Compares the upstream-reported name with the client-requested modelID.
// Channel model mapping is treated as a mismatch. The list only sees the first
// 10 executions; executions outside that window are not part of the verdict.
export function getUpstreamModelAudit(executions: readonly ModelAuditExecution[]): ModelAuditSummary {
  const matchedUpstreamIds = new Set<string>();
  const equalModelIds = new Set<string>();
  const upstreamModelIds = new Set<string>();
  const mismatchedModelIds = new Set<string>();
  let unknownCount = 0;
  let hasCompletedComparison = false;
  let blockingUnknownCount = 0;

  for (const execution of executions) {
    const requestedModel = execution.modelID?.trim() ?? '';
    const reportedModel = execution.upstreamModelID?.trim() ?? '';
    if (reportedModel) upstreamModelIds.add(reportedModel);
    if (!requestedModel || !reportedModel) {
      unknownCount++;
      if (execution.status !== 'failed' && execution.status !== 'canceled') blockingUnknownCount++;
      continue;
    }
    hasCompletedComparison ||= execution.status === 'completed';
    if (reportedModel !== requestedModel) {
      mismatchedModelIds.add(reportedModel);
    } else {
      equalModelIds.add(reportedModel);
      if (execution.status === 'completed') matchedUpstreamIds.add(reportedModel);
    }
  }

  const status: ModelAuditStatus =
    mismatchedModelIds.size > 0
      ? 'mismatched'
      : executions.length === 0 || (unknownCount > 0 && (!hasCompletedComparison || blockingUnknownCount > 0))
        ? 'unknown'
        : 'matched';

  return {
    status,
    matchedUpstreamIds: Array.from(matchedUpstreamIds),
    equalModelIds: Array.from(equalModelIds),
    upstreamModelIds: Array.from(upstreamModelIds),
    mismatchedModelIds: Array.from(mismatchedModelIds),
    unknownCount,
    comparedCount: executions.length - unknownCount,
  };
}

export function getRequestModelAuditTooltip(modelAudit: ModelAuditSummary, requestStatus: ModelAuditExecution['status'], t: TFunction) {
  if (requestStatus === 'pending' || requestStatus === 'processing') return t('requests.tooltips.upstreamModelRequestProcessing');
  if (requestStatus === 'failed' || requestStatus === 'canceled') return t('requests.tooltips.upstreamModelRequestFailed');

  if (modelAudit.status === 'matched') {
    if (modelAudit.matchedUpstreamIds.length === 0) return t('requests.tooltips.upstreamModelUnknown');
    return t(
      modelAudit.unknownCount > 0 ? 'requests.tooltips.upstreamModelMatchedAfterRetries' : 'requests.tooltips.upstreamModelMatching',
      { model: modelAudit.matchedUpstreamIds.join(', '), unknown: modelAudit.unknownCount }
    );
  }

  let tooltip = t('requests.tooltips.upstreamModelUnknown');
  if (modelAudit.status === 'mismatched') {
    tooltip = t('requests.tooltips.upstreamModelMismatch', { model: modelAudit.mismatchedModelIds.join(', ') });
  }
  if (modelAudit.unknownCount > 0 && modelAudit.comparedCount > 0) {
    const partial = t('requests.tooltips.upstreamModelPartial', { compared: modelAudit.comparedCount, unknown: modelAudit.unknownCount });
    return modelAudit.status === 'unknown' ? partial : `${tooltip} ${partial}`;
  }
  return tooltip;
}

// One verdict per execution row. Lifecycle decides the tone, so a failed retry
// that happens to match can never render as a green success conclusion.
export function getExecutionModelAuditVerdict(
  modelAudit: ModelAuditSummary,
  executionStatus: ModelAuditExecution['status'],
  t: TFunction
): ModelAuditVerdict {
  if (executionStatus === 'pending' || executionStatus === 'processing') {
    return { tone: 'pending', message: t('requests.tooltips.upstreamModelRequestProcessing') };
  }

  if (modelAudit.status === 'mismatched') {
    return {
      tone: 'danger',
      message: t('requests.detail.upstreamModelMismatch', { model: modelAudit.mismatchedModelIds.join(', ') }),
    };
  }

  const failed = executionStatus === 'failed' || executionStatus === 'canceled';
  if (failed) {
    if (modelAudit.equalModelIds.length === 0) {
      return { tone: 'danger', message: t('requests.tooltips.upstreamModelRequestFailed') };
    }
    return {
      tone: 'muted',
      message: t('requests.detail.upstreamModelMatchedButFailed', { model: modelAudit.equalModelIds.join(', ') }),
    };
  }

  if (modelAudit.status === 'matched') {
    return { tone: 'success', message: t('requests.detail.upstreamModelMatched') };
  }

  return { tone: 'muted', message: t('requests.tooltips.upstreamModelUnknown') };
}
