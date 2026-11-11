// 职责: 展示 Claude 重置权益卡片，并使用用户选中的权益。
// 关联文件:
//   frontend/src/components/quota-badges.tsx
//   frontend/src/features/system/data/quotas.ts
//   frontend/src/locales/en.json
//   frontend/src/locales/zh.json
//   frontend/src/locales/en/system.json
//   frontend/src/locales/zh-CN/system.json
// 注意事项:
//   剩余次数不代表现在可兑换；后端会再次检查所选权益资格，重置不会自动重试。
//   权益名称只按 clears 覆盖的窗口推导，不按上游 label 判断，上游 label 只放在悬停提示里。
//   模型周额度的模型名来自后端 windowModels，不在前端写死。
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Loader2, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { resetChannelQuotaNow, type ProviderQuotaReset, type ProviderQuotaResetList } from '@/features/system/data/quotas';

const CLAUDE_WINDOW_LABEL_KEYS: Record<string, string> = {
  five_hour: 'quota.window.5h',
  seven_day: 'quota.window.7d',
  // Model-scoped weekly limit; used when the backend cannot name the model.
  seven_day_overage_included: 'quota.claude.scopedWeekly',
};

const CLAUDE_BLOCK_REASON_KEYS: Record<string, string> = {
  cooldown: 'quota.claude.reason.cooldown',
  requires_limit: 'quota.claude.reason.requiresLimit',
  blocked: 'quota.claude.reason.blocked',
  upstream: 'quota.claude.reason.upstream',
  unavailable: 'quota.claude.reason.unavailable',
};

export function isFullClaudeReset(clears: string[] = []) {
  return clears.includes('five_hour') && clears.includes('seven_day');
}

// null means the grant can be redeemed now. Upstream reasons carry their code ("upstream:<code>")
// so grants blocked by different upstream codes are not merged into one message.
export function getClaudeGrantBlockReason(grant: ProviderQuotaReset, coolingDown: boolean) {
  if (coolingDown) return 'cooldown';
  if (grant.redeemable === true && (grant.remaining ?? 0) > 0) return null;
  if (grant.upstreamReason && (grant.unavailableReason === 'upstream' || grant.unavailableReason === 'not_usable' || !grant.unavailableReason)) {
    return `upstream:${grant.upstreamReason}`;
  }
  return grant.unavailableReason && grant.unavailableReason in CLAUDE_BLOCK_REASON_KEYS ? grant.unavailableReason : 'unavailable';
}

// Returns the reason only when every grant is blocked for the same reason, so it can be shown once.
export function getSharedClaudeBlockReason(reasons: (string | null)[]) {
  if (reasons.length === 0 || reasons.some((reason) => reason === null)) return null;
  return reasons.every((reason) => reason === reasons[0]) ? reasons[0] : null;
}

export function ClaudeResetCredits({ channelID, resets }: { channelID: string; resets?: ProviderQuotaResetList }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [pendingID, setPendingID] = useState<string | null>(null);
  const now = Date.now();
  const grants = (resets?.resets ?? [])
    .filter((grant) => (!grant.expiresAt || Date.parse(grant.expiresAt) > now) && (!grant.grantedAt || Date.parse(grant.grantedAt) <= now))
    .sort((a, b) => Number(!!b.recommended) - Number(!!a.recommended));
  const remaining = grants.reduce((total, grant) => total + Math.max(0, grant.remaining ?? 0), 0);
  const coolingDown = !!resets?.cooldownUntil && Date.parse(resets.cooldownUntil) > now;
  const known = resets?.eligible !== undefined && !resets.error;
  const eligible = known && resets?.eligible === true;
  const reasons = grants.map((grant) => (eligible ? getClaudeGrantBlockReason(grant, coolingDown) : 'unavailable'));
  const sharedReason = eligible ? getSharedClaudeBlockReason(reasons) : null;

  const formatTime = (value: string) =>
    new Intl.DateTimeFormat(i18n.language, {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      ...(new Date(value).getFullYear() !== new Date(now).getFullYear() ? { year: 'numeric' as const } : {}),
    }).format(new Date(value));
  const reasonText = (reason: string) => {
    if (reason === 'cooldown' && coolingDown) return t('quota.claude.reason.cooldownUntil', { time: formatTime(resets!.cooldownUntil!) });
    if (reason.startsWith('upstream:')) return t('quota.claude.reason.upstream', { reason: reason.slice('upstream:'.length) });
    return t(CLAUDE_BLOCK_REASON_KEYS[reason] ?? CLAUDE_BLOCK_REASON_KEYS.unavailable);
  };
  const windowLabel = (window: string) => {
    const model = resets?.windowModels?.[window];
    if (model) return t('quota.claude.modelWeekly', { model });
    return CLAUDE_WINDOW_LABEL_KEYS[window] ? t(CLAUDE_WINDOW_LABEL_KEYS[window]) : window.replace(/_/g, ' ');
  };
  const grantName = (grant: ProviderQuotaReset) => {
    if (!grant.clears?.length) return t('quota.claude.genericReset');
    return t(isFullClaudeReset(grant.clears) ? 'quota.claude.fullReset' : 'quota.claude.partialReset');
  };

  const summary = !known
    ? t('quota.claude.resetUnknown')
    : !eligible
      ? t(resets?.ineligibleReason === 'cli_version' ? 'quota.claude.resetClientVersion' : 'quota.claude.resetIneligible')
      : grants.length === 0
        ? t('quota.claude.resetUnavailable')
        : sharedReason
          ? reasonText(sharedReason)
          : null;

  const redeem = async (grant: ProviderQuotaReset, index: number) => {
    if (pendingID || reasons[index] !== null) return;
    setPendingID(grant.id);
    try {
      await resetChannelQuotaNow(channelID, grant.id);
      toast.success(t('quota.claude.resetSuccess'));
    } catch (error) {
      toast.error(t('quota.claude.resetError'), { description: error instanceof Error ? error.message : String(error) });
    } finally {
      try {
        await queryClient.invalidateQueries({ queryKey: ['provider-quotas'] });
      } finally {
        setPendingID(null);
      }
    }
  };

  return (
    <div className='border-border/60 space-y-3 border-t border-dashed pt-3 text-xs'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-muted-foreground font-medium'>{t('quota.claude.resetCredits')}</span>
        <span className='shrink-0 font-medium tabular-nums'>{eligible ? t('quota.claude.resetCount', { count: remaining }) : t('quota.label.unavailable')}</span>
      </div>
      {summary && <p className='text-muted-foreground'>{summary}</p>}
      {grants.length > 0 && (
        <div className='space-y-2'>
          {grants.map((grant, index) => {
            const name = grantName(grant);
            const reason = reasons[index];
            return (
              <div key={grant.id} className='border-border/60 bg-muted/20 space-y-2 rounded-lg border p-3'>
                <div className='flex items-center justify-between gap-2'>
                  <span className='min-w-0 truncate font-medium' title={grant.title || name}>
                    {name}
                    {(grant.remaining ?? 0) > 1 && (
                      <span className='text-muted-foreground ml-1 tabular-nums'>{t('quota.claude.grantCount', { count: grant.remaining })}</span>
                    )}
                  </span>
                  {grant.recommended && (
                    <Badge variant='secondary' className='px-1.5 py-0 text-[10px]'>
                      {t('quota.claude.recommended')}
                    </Badge>
                  )}
                </div>
                <div className='flex flex-wrap gap-1'>
                  {(grant.clears ?? []).map((window) => (
                    <Badge key={window} variant='outline' className='text-muted-foreground rounded-md px-1.5 py-0 text-[10px]'>
                      {windowLabel(window)}
                    </Badge>
                  ))}
                </div>
                <div className='flex items-center justify-between gap-3'>
                  <span className='text-muted-foreground min-w-0 text-[11px] tabular-nums'>
                    {grant.expiresAt ? t('quota.claude.expiresAt', { time: formatTime(grant.expiresAt) }) : t('quota.claude.noExpiry')}
                  </span>
                  <Button
                    size='sm'
                    variant='outline'
                    className='h-7 shrink-0 gap-1.5 rounded-md px-2.5 text-xs'
                    disabled={!!pendingID || reason !== null}
                    onClick={() => redeem(grant, index)}
                    aria-label={t('quota.claude.useNamedReset', { name })}
                  >
                    {pendingID === grant.id ? <Loader2 className='size-3 animate-spin' /> : <RotateCcw className='size-3' />}
                    {pendingID === grant.id ? t('quota.claude.resetting') : t('quota.claude.useReset')}
                  </Button>
                </div>
                {eligible && !sharedReason && reason && <p className='text-muted-foreground text-[11px]'>{reasonText(reason)}</p>}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
