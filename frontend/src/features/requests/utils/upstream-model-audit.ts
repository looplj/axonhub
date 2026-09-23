import type { TFunction } from 'i18next';

interface ModelAuditExecution {
  status?: 'pending' | 'processing' | 'completed' | 'failed' | 'canceled';
  modelID?: string | null;
  upstreamModelID?: string | null;
}

type ModelAuditStatus = 'matched' | 'mismatched' | 'unknown';

type ModelAuditVerdictTone = 'success' | 'danger' | 'pending' | 'muted';

interface ModelAuditVerdict {
  tone: ModelAuditVerdictTone;
  message: string;
}

export const MODEL_AUDIT_VERDICT_CLASS: Record<ModelAuditVerdictTone, string> = {
  success: 'font-mono text-xs text-emerald-600 dark:text-emerald-400',
  danger: 'text-xs font-medium text-destructive',
  pending: 'text-xs font-medium text-sky-700 dark:text-sky-300',
  muted: 'text-muted-foreground text-xs',
};

interface ModelAuditSummary {
  status: ModelAuditStatus;
  matchedUpstreamIds: string[];
  mismatchedModelIds: string[];
  unknownCount: number;
  comparedCount: number;
}

// Compares the upstream-reported name with the channel model used for routing
// and pricing. Provider transformations can legitimately change the wire model.
// The list only sees the first 10 executions; executions outside that window
// are not part of the verdict.
export function getUpstreamModelAudit(executions: readonly ModelAuditExecution[]): ModelAuditSummary {
  const matchedUpstreamIds = new Set<string>();
  const mismatchedModelIds = new Set<string>();
  let unknownCount = 0;
  let hasBlockingUnknown = false;

  for (const execution of executions) {
    const channelModel = execution.modelID?.trim() ?? '';
    const reportedModel = execution.upstreamModelID?.trim() ?? '';
    if (!channelModel || !reportedModel) {
      unknownCount++;
      if (execution.status !== 'failed' && execution.status !== 'canceled') hasBlockingUnknown = true;
      continue;
    }
    if (reportedModel !== channelModel) {
      mismatchedModelIds.add(reportedModel);
    } else if (execution.status === 'completed') {
      matchedUpstreamIds.add(reportedModel);
    }
  }

  const status: ModelAuditStatus =
    mismatchedModelIds.size > 0
      ? 'mismatched'
      : executions.length === 0 || hasBlockingUnknown || (unknownCount > 0 && matchedUpstreamIds.size === 0)
        ? 'unknown'
        : 'matched';

  return {
    status,
    matchedUpstreamIds: Array.from(matchedUpstreamIds),
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
  execution: ModelAuditExecution,
  t: TFunction
): ModelAuditVerdict {
  const { status: executionStatus } = execution;
  const channelModel = execution.modelID?.trim() ?? '';
  const reportedModel = execution.upstreamModelID?.trim() ?? '';
  const canCompare = channelModel !== '' && reportedModel !== '';

  if (executionStatus === 'pending' || executionStatus === 'processing') {
    return { tone: 'pending', message: t('requests.tooltips.upstreamModelRequestProcessing') };
  }

  if (canCompare && channelModel !== reportedModel) {
    return {
      tone: 'danger',
      message: t('requests.detail.upstreamModelMismatch', { model: reportedModel }),
    };
  }

  const failed = executionStatus === 'failed' || executionStatus === 'canceled';
  if (failed) {
    if (!canCompare) {
      return { tone: 'danger', message: t('requests.tooltips.upstreamModelRequestFailed') };
    }
    return {
      tone: 'muted',
      message: t('requests.detail.upstreamModelMatchedButFailed', { model: reportedModel }),
    };
  }

  if (canCompare) {
    return { tone: 'success', message: t('requests.detail.upstreamModelMatched') };
  }

  return { tone: 'muted', message: t('requests.tooltips.upstreamModelUnknown') };
}
