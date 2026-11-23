// 职责: 请求模型目录预览、批量应用和站点定时检查设置。
// 关联文件:
//   frontend/src/features/models/components/models-catalog-import.tsx
//   internal/server/gql/model.graphql
//   frontend/src/features/models/data/schema.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import type { ReferencePriceField } from './schema';

export interface ImportScope {
  channels: boolean;
  providers: string[];
  ignorePrefix: boolean;
  tracked: boolean;
}
export interface ImportRate {
  field: ReferencePriceField;
  base: number | null;
  tiers: { label: string; value: number }[];
}
export interface Offer {
  canonical: string;
  provider: string;
  modelID: string;
  name: string;
  developer: string;
  identity: string;
  type: string;
  rates: ImportRate[];
}
export interface LocalRate {
  field: ReferencePriceField;
  value: number | null;
  follow: boolean;
  canonical: string;
  provider: string;
  modelID: string;
}
export interface ImportRow {
  manualMerge?: boolean;
  mergeTarget: string;
  mergeTargetOccupied: boolean;
  modelID: string;
  name: string;
  members: { id: string; identity: string }[];
  routes: { channelID: number; channelName: string; actual: string; requests: string[] }[];
  version: string;
  archived: boolean;
  changed: boolean;
  preferred: Pick<Offer, 'provider' | 'modelID'> | null;
  offers: Offer[];
  prices: LocalRate[];
}
export interface CatalogPreview {
  providers: string[];
  revision: string;
  source: string;
  upstreamURL: string;
  rows: ImportRow[];
}
export interface PriceSelection {
  field: ReferencePriceField;
  action: 'FOLLOW' | 'REBIND' | 'FIX' | 'FIX_BASE' | 'TIER';
  provider: string;
  modelID: string;
  tier?: string;
  expected?: number;
  identity?: string;
}
export interface ImportSelection {
  manualMerge?: boolean;
  modelID: string;
  name: string;
  developer: string;
  members: ImportRow['members'];
  routes: { channelID: number; actual: string; request?: string }[];
  version: string;
  provider: string;
  type: string;
  sourceModelID: string;
  identity: string;
  prices: PriceSelection[];
}
export interface CheckSettings {
  enabled: boolean;
  time: string;
  timezone: string;
  providers: string[];
  ignorePrefix: boolean;
}
export interface CheckStatus {
  settings: CheckSettings;
  nextCheck: string | null;
  lastAttempt: string | null;
  lastSuccess: string | null;
  failed: boolean;
  available: number;
  changed: number;
}
export interface Outcome {
  modelID: string;
  status: 'applied' | 'invalid' | 'conflict' | 'catalogChanged' | 'failed';
}

const offerFields = 'canonical provider modelID name developer identity type rates { field base tiers { label value } }';
const statusFields = 'settings { enabled time timezone providers ignorePrefix } nextCheck lastAttempt lastSuccess failed available changed';
const previewQuery = `query CatalogImportPreview($scope: CatalogImportScope!) {
  catalogImportPreview(scope: $scope) {
    source upstreamURL revision providers rows { modelID name members { id identity } routes { channelID channelName actual requests } version archived changed
      offers { ${offerFields} } preferred { provider modelID } mergeTarget mergeTargetOccupied prices { field value follow canonical provider modelID }
    }
  }
}`;
export function useCatalogPreview(scope: ImportScope, enabled: boolean) {
  return useQuery({
    queryKey: ['catalog-import', scope],
    enabled,
    queryFn: async ({ signal }) =>
      (await graphqlRequest<{ catalogImportPreview: CatalogPreview }>(previewQuery, { scope }, undefined, { signal })).catalogImportPreview,
    refetchOnWindowFocus: false,
    staleTime: 60_000,
  });
}
export function useCatalogApply() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (variables: { scope: ImportScope; revision: string; selections: ImportSelection[] }) =>
      (
        await graphqlRequest<{ applyCatalogImport: Outcome[] }>(
          `mutation ApplyCatalogImport($scope: CatalogImportScope!, $revision: String!, $selections: [CatalogImportSelection!]!) {
        applyCatalogImport(scope: $scope, revision: $revision, selections: $selections) { modelID status }
      }`,
          variables
        )
      ).applyCatalogImport,
    onSettled: () => Promise.all([
      client.invalidateQueries({ queryKey: ['models'] }),
      client.invalidateQueries({ queryKey: ['catalog-import'] }),
    ]),
  });
}
export function useCatalogCheckStatus(enabled = true) {
  return useQuery({
    queryKey: ['model-catalog-check'],
    enabled,
    queryFn: async () =>
      (await graphqlRequest<{ catalogCheckStatus: CheckStatus }>(`query CatalogCheckStatus { catalogCheckStatus { ${statusFields} } }`))
        .catalogCheckStatus,
    refetchInterval: 60_000,
  });
}
export function useSaveCatalogSchedule() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (input: CheckSettings) =>
      (
        await graphqlRequest<{ saveCatalogCheckSettings: CheckStatus }>(
          `mutation SaveCatalogCheckSettings($input: CatalogCheckSettingsInput!) { saveCatalogCheckSettings(input: $input) { ${statusFields} } }`,
          { input }
        )
      ).saveCatalogCheckSettings,
    onSuccess: (data) => client.setQueryData(['model-catalog-check'], data),
  });
}
export function useCheckCatalogNow() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      (
        await graphqlRequest<{ checkModelCatalogNow: CheckStatus }>(
          `mutation CheckModelCatalogNow { checkModelCatalogNow { ${statusFields} } }`
        )
      ).checkModelCatalogNow,
    onSuccess: (data) => {
      client.setQueryData(['model-catalog-check'], data);
      client.invalidateQueries({ queryKey: ['catalog-import'] });
    },
  });
}
