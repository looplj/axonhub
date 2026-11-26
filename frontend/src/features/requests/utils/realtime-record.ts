// 职责: 根据审计响应元数据区分 Realtime 会话记录与单次 Response 记录。
// 关联文件:
//   frontend/src/features/requests/components/request-detail-content.tsx

export type RealtimeAuditRecordKind = 'session' | 'response';

export function getRealtimeAuditRecordKind(
  format: string | undefined,
  responseBody: unknown
): RealtimeAuditRecordKind | undefined {
  if (format !== 'openai/realtime') return undefined;
  if (!responseBody || typeof responseBody !== 'object' || Array.isArray(responseBody)) return 'session';

  const body = responseBody as Record<string, unknown>;
  const responseId = body.response_id;
  if ((typeof responseId === 'string' && responseId.trim().length > 0) || body.response_create_rejected === true) {
    return 'response';
  }

  return 'session';
}
