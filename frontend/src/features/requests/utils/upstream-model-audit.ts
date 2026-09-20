import type { TFunction } from 'i18next';

interface ModelAuditExecution {
  status?: 'pending' | 'processing' | 'completed' | 'failed' | 'canceled';
  modelID?: string | null;
  outboundModelID?: string | null;
  upstreamModelID?: string | null;
  upstreamModelIds?: readonly string[] | null;
}

type ModelAuditStatus = 'matched' | 'mismatched' | 'unknown' | 'conflicting';

// Detail-page audit. List rows use the backend's full execution-set summary.
export function getUpstreamModelAudit(executions: readonly ModelAuditExecution[]) {
  const matchedUpstreamIds = new Set<string>();
  const upstreamModelIds = new Set<string>();
  const mismatchedModelIds = new Set<string>();
  const conflictingModelIds = new Set<string>();
  let unknownCount = 0;
  let conflictCount = 0;
  let hasCompletedComparison = false;
  let blockingUnknownCount = 0;

  for (const execution of executions) {
    // Routing modelID is not evidence of what was sent after body overrides.
    const sentModel = execution.outboundModelID;
    const reportedModels = Array.from(
      new Set([execution.upstreamModelID, ...(execution.upstreamModelIds ?? [])].filter((model): model is string => Boolean(model?.trim())))
    );
    for (const model of reportedModels) upstreamModelIds.add(model);
    if (reportedModels.length > 1) {
      conflictCount++;
      for (const model of reportedModels) conflictingModelIds.add(model);
    }
    if (!sentModel?.trim() || reportedModels.length === 0) {
      unknownCount++;
      if (execution.status !== 'failed' && execution.status !== 'canceled') blockingUnknownCount++;
      continue;
    }
    hasCompletedComparison ||= execution.status === 'completed';

    // Compare within this execution before deduplicating the display values.
    for (const model of reportedModels) {
      if (model !== sentModel) mismatchedModelIds.add(model);
      else if (execution.status === 'completed') matchedUpstreamIds.add(model);
    }
  }

  const status: ModelAuditStatus =
    conflictCount > 0
      ? 'conflicting'
      : mismatchedModelIds.size > 0
        ? 'mismatched'
        : executions.length === 0 || (unknownCount > 0 && (!hasCompletedComparison || blockingUnknownCount > 0))
          ? 'unknown'
          : 'matched';

  return {
    status,
    matchedUpstreamIds: Array.from(matchedUpstreamIds),
    upstreamModelIds: Array.from(upstreamModelIds),
    mismatchedModelIds: Array.from(mismatchedModelIds),
    conflictingModelIds: Array.from(conflictingModelIds),
    unknownCount,
    comparedCount: executions.length - unknownCount,
    conflictCount,
  };
}

// List tooltips use the complete backend summary, never the paginated executions.
export function getRequestModelAuditTooltip(
  modelAudit: ReturnType<typeof getUpstreamModelAudit>,
  requestStatus: ModelAuditExecution['status'],
  t: TFunction
) {
  if (requestStatus === 'pending' || requestStatus === 'processing') return t('requests.tooltips.upstreamModelRequestProcessing');
  if (requestStatus === 'failed' || requestStatus === 'canceled') return t('requests.tooltips.upstreamModelRequestFailed');

  if (modelAudit.status === 'matched') {
    // Missing successful evidence must not fall back to failed retry names.
    if (modelAudit.matchedUpstreamIds.length === 0) return t('requests.tooltips.upstreamModelUnknown');
    return t(
      modelAudit.unknownCount > 0 ? 'requests.tooltips.upstreamModelMatchedAfterRetries' : 'requests.tooltips.upstreamModelMatching',
      { model: modelAudit.matchedUpstreamIds.join(', '), unknown: modelAudit.unknownCount }
    );
  }

  let tooltip = t('requests.tooltips.upstreamModelUnknown');
  if (modelAudit.status === 'mismatched') {
    tooltip = t('requests.tooltips.upstreamModelMismatch', { model: modelAudit.mismatchedModelIds.join(', ') });
  } else if (modelAudit.status === 'conflicting') {
    tooltip = t('requests.tooltips.upstreamModelConflict', { model: modelAudit.conflictingModelIds.join(', ') });
  }
  if (modelAudit.unknownCount > 0 && modelAudit.comparedCount > 0) {
    const partial = t('requests.tooltips.upstreamModelPartial', { compared: modelAudit.comparedCount, unknown: modelAudit.unknownCount });
    return modelAudit.status === 'unknown' ? partial : `${tooltip} ${partial}`;
  }
  return tooltip;
}
