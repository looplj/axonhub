import { create } from 'zustand';
import type { AnalyticsFilter } from '@/features/analytics/data/analytics';

/** Dimension slices of the analytics filter; the time range is held by the shared
 * time store instead. */
export type AnalyticsDimensionFilter = Pick<
  AnalyticsFilter,
  'projectIDs' | 'channelIDs' | 'modelIDs' | 'apiKeyIDs' | 'userIDs'
>;

interface AnalyticsFilterState {
  dimensions: AnalyticsDimensionFilter;
  setProjectIDs: (ids: string[]) => void;
  setChannelIDs: (ids: string[]) => void;
  setModelIDs: (ids: string[]) => void;
  setAPIKeyIDs: (ids: string[]) => void;
  setUserIDs: (ids: string[]) => void;
  resetDimensionFilters: () => void;
}

const defaultDimensions: AnalyticsDimensionFilter = {
  projectIDs: undefined,
  channelIDs: undefined,
  modelIDs: undefined,
  apiKeyIDs: undefined,
  userIDs: undefined,
};

/** Analytics dimension filters: project/channel/model/API key/user. */
export const useAnalyticsFilterStore = create<AnalyticsFilterState>((set) => ({
  dimensions: { ...defaultDimensions },

  setProjectIDs: (ids) =>
    set((state) => ({
      dimensions: { ...state.dimensions, projectIDs: ids.length > 0 ? ids : undefined },
    })),

  setChannelIDs: (ids) =>
    set((state) => ({
      dimensions: { ...state.dimensions, channelIDs: ids.length > 0 ? ids : undefined },
    })),

  setModelIDs: (ids) =>
    set((state) => ({
      dimensions: { ...state.dimensions, modelIDs: ids.length > 0 ? ids : undefined },
    })),

  setAPIKeyIDs: (ids) =>
    set((state) => ({
      dimensions: { ...state.dimensions, apiKeyIDs: ids.length > 0 ? ids : undefined },
    })),

  setUserIDs: (ids) =>
    set((state) => ({
      dimensions: { ...state.dimensions, userIDs: ids.length > 0 ? ids : undefined },
    })),

  resetDimensionFilters: () => set({ dimensions: { ...defaultDimensions } }),
}));
