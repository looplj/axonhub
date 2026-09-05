export function isVideoRequestFormat(format: string | null | undefined): boolean {
  return format === 'openai/video' || format === 'seedance/video' || format === 'zenmux/video';
}

export function getVideoLastFrameURL(responseBody: unknown): string | undefined {
  if (!isRecord(responseBody)) return undefined;

  const video = isRecord(responseBody.video) ? responseBody.video : undefined;
  const content = isRecord(responseBody.content) ? responseBody.content : undefined;

  return firstNonEmptyString(
    responseBody.last_frame_url,
    responseBody.lastFrameURL,
    video?.last_frame_url,
    video?.lastFrameURL,
    content?.last_frame_url,
    content?.lastFrameURL
  );
}

function firstNonEmptyString(...values: unknown[]): string | undefined {
  return values.find((value): value is string => typeof value === 'string' && value.trim().length > 0);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
