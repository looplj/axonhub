// 职责: 解析 Vertex service account JSON，并生成不含私钥的列表标识、提交载荷与校验消息。
// 关联文件:
//   frontend/src/features/channels/data/schema.ts
//   frontend/src/features/channels/components/channels-action-dialog.tsx
//   frontend/src/features/channels/components/channels-api-key-management-dialog.tsx
//   frontend/src/locales/en/channels.json
//   frontend/src/locales/zh-CN/channels.json
// 遵循的 Rules:
//   .agents/rules/frontend-general.md
//   .agents/rules/frontend-i18n.md
// 注意事项:
//   credentialRef 由服务端输出，前端只读；提交载荷必须剔除它。
//   列表标识、复制控件只允许使用 client_email 或掩码后的 ref，不得包含原始 JSON 或私钥。
import { z } from 'zod';

export const VERTEX_DEFAULT_REGION = 'global';

// Vertex project and region are interpolated into the upstream request path, so the
// write schema constrains them to the characters Google permits.
export const vertexProjectIDPattern = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
export const vertexRegionPattern = /^[A-Za-z0-9][A-Za-z0-9-]*$/;

export type VertexServiceAccountParseError =
  | 'invalidJSON'
  | 'notServiceAccount'
  | 'missingClientEmail'
  | 'missingPrivateKey'
  | 'invalidPrivateKey';

export interface VertexServiceAccountJSON {
  /** Service account project from the JSON body; empty when the file omits it. */
  projectID: string;
  clientEmail: string;
  privateKey: string;
}

export type VertexServiceAccountParseResult =
  | { ok: true; value: VertexServiceAccountJSON }
  | { ok: false; error: VertexServiceAccountParseError };

const serviceAccountPrivateKeyPattern = /-----BEGIN [A-Z ]*PRIVATE KEY-----/;

/**
 * Parses a pasted or file-loaded service account JSON. Only the fields the channel
 * needs are retained; the raw text never leaves the form value it was pasted into.
 */
export function parseVertexServiceAccountJSON(raw: string): VertexServiceAccountParseResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return { ok: false, error: 'invalidJSON' };
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ok: false, error: 'invalidJSON' };
  }

  const record = parsed as Record<string, unknown>;
  if (record.type !== 'service_account') {
    return { ok: false, error: 'notServiceAccount' };
  }

  const clientEmail = typeof record.client_email === 'string' ? record.client_email.trim() : '';
  if (!clientEmail) {
    return { ok: false, error: 'missingClientEmail' };
  }

  const privateKey = typeof record.private_key === 'string' ? record.private_key.trim() : '';
  if (!privateKey) {
    return { ok: false, error: 'missingPrivateKey' };
  }
  if (!serviceAccountPrivateKeyPattern.test(privateKey)) {
    return { ok: false, error: 'invalidPrivateKey' };
  }

  return {
    ok: true,
    value: {
      projectID: typeof record.project_id === 'string' ? record.project_id.trim() : '',
      clientEmail,
      privateKey,
    },
  };
}

export function vertexServiceAccountParseErrorMessage(error: VertexServiceAccountParseError): string {
  switch (error) {
    case 'invalidJSON':
      return 'Vertex service account JSON is not valid JSON';
    case 'notServiceAccount':
      return 'Vertex service account JSON must have "type": "service_account"';
    case 'missingClientEmail':
      return 'Vertex service account JSON is missing client_email';
    case 'missingPrivateKey':
      return 'Vertex service account JSON is missing private_key';
    case 'invalidPrivateKey':
      return 'Vertex service account JSON private_key is not a PEM private key';
  }
}

export interface VertexServiceAccountRow {
  projectID: string;
  region?: string | null;
  jsonData: string;
  /** Server-issued identity; never submitted back. */
  credentialRef?: string | null;
}

export interface VertexServiceAccountInput {
  projectID: string;
  region: string;
  jsonData: string;
}

/**
 * Builds the GraphQL input rows. Blank drafts are dropped, the region falls back to
 * global, and credentialRef is stripped so the server stays the only ref issuer.
 */
export function toVertexServiceAccountInputs(
  rows: readonly VertexServiceAccountRow[] | null | undefined
): VertexServiceAccountInput[] {
  return (rows ?? [])
    .filter((row) => row.jsonData.trim().length > 0)
    .map((row) => ({
      projectID: row.projectID.trim(),
      region: (row.region ?? '').trim() || VERTEX_DEFAULT_REGION,
      jsonData: row.jsonData.trim(),
    }));
}

/**
 * Identity for list rows and project prefill. Returns empty strings for an unparsable
 * body so callers fall back to a masked credential ref instead of leaking raw JSON.
 */
export function getVertexServiceAccountIdentity(jsonData: string): { clientEmail: string; projectID: string } {
  const parsed = parseVertexServiceAccountJSON(jsonData);
  if (!parsed.ok) {
    return { clientEmail: '', projectID: '' };
  }

  return { clientEmail: parsed.value.clientEmail, projectID: parsed.value.projectID };
}

export function maskVertexCredentialRef(credentialRef: string): string {
  const tail = credentialRef.slice(-4);
  return `vertex-sa:****${tail}`;
}

// Shape-only write field schema: the write input must accept a row the channel type
// never submits, so strict per-row rules live in vertexServiceAccountCredentialSchema
// and are re-applied per channel type in schema.ts.
export const vertexServiceAccountShapeSchema = z.object({
  projectID: z.string(),
  region: z.string().default(VERTEX_DEFAULT_REGION),
  jsonData: z.string(),
});

/**
 * Strict per-row rules for a Vertex service account credential.
 * defaultRegion fills an omitted region; omit it where an empty region must be an error.
 */
export function vertexServiceAccountCredentialSchema(defaultRegion?: string) {
  const regionSchema = z
    .string()
    .trim()
    .regex(vertexRegionPattern, 'Vertex region contains invalid characters');

  return z.object({
    projectID: z
      .string()
      .trim()
      .regex(vertexProjectIDPattern, 'Vertex Project ID contains invalid characters'),
    region: defaultRegion ? regionSchema.default(defaultRegion) : regionSchema,
    jsonData: z
      .string()
      .trim()
      .min(1, 'Vertex service account JSON is required')
      .superRefine((value, ctx) => {
        const parsed = parseVertexServiceAccountJSON(value);
        if (!parsed.ok) {
          ctx.addIssue({ code: 'custom', message: vertexServiceAccountParseErrorMessage(parsed.error) });
        }
      }),
  });
}
