// 职责: 请求模型目录预览和批量应用。
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
export interface Outcome {
  modelID: string;
  status: 'applied' | 'invalid' | 'conflict' | 'catalogChanged' | 'failed';
}

const offerFields = 'canonical provider modelID name developer identity type rates { field base tiers { label value } }';
const previewQuery = `query CatalogImportPreview($scope: CatalogImportScope!) {
  catalogImportPreview(scope: $scope) {
    source upstreamURL revision providers rows { modelID name members { id identity } routes { channelID channelName actual requests } version archived changed
      offers { ${offerFields} } preferred { provider modelID } mergeTarget mergeTargetOccupied prices { field value follow canonical provider modelID }
    }
  }
}`;
export function useCatalogPreview(scope: ImportScope) {
  return useQuery({
    queryKey: ['catalog-import', scope],
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
