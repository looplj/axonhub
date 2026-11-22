export function resolveSessionTrace(_ctx, request) {
  const sessionId = request.headers["x-deepseek-harness-session-id"]?.[0]?.trim()
  if (sessionId) {
    return { context: { axonhub: { traceId: sessionId } } }
  }
}
