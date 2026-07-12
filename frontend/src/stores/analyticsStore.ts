import { create } from 'zustand';
import type { AnalyticsFilter } from '@/features/analytics/data/analytics';

interface AnalyticsFilterState {
  filter: AnalyticsFilter;
  setStartTime: (time: string | null) => void;
  setEndTime: (time: string | null) => void;
  setProjectIDs: (ids: string[]) => void;
  setChannelIDs: (ids: string[]) => void;
  setModelIDs: (ids: string[]) => void;
  setAPIKeyIDs: (ids: string[]) => void;
  setUserIDs: (ids: string[]) => void;
  resetFilter: () => void;
}

const defaultFilter: AnalyticsFilter = {
  startTime: null,
  endTime: null,
  projectIDs: undefined,
  channelIDs: undefined,
  modelIDs: undefined,
  apiKeyIDs: undefined,
  userIDs: undefined,
};

export const useAnalyticsFilterStore = create<AnalyticsFilterState>((set) => ({
  filter: { ...defaultFilter },

  setStartTime: (time) =>
    set((state) => ({
      filter: { ...state.filter, startTime: time },
    })),

  setEndTime: (time) =>
    set((state) => ({
      filter: { ...state.filter, endTime: time },
    })),

  setProjectIDs: (ids) =>
    set((state) => ({
      filter: { ...state.filter, projectIDs: ids.length > 0 ? ids : undefined },
    })),

  setChannelIDs: (ids) =>
    set((state) => ({
      filter: { ...state.filter, channelIDs: ids.length > 0 ? ids : undefined },
    })),

  setModelIDs: (ids) =>
    set((state) => ({
      filter: { ...state.filter, modelIDs: ids.length > 0 ? ids : undefined },
    })),

  setAPIKeyIDs: (ids) =>
    set((state) => ({
      filter: { ...state.filter, apiKeyIDs: ids.length > 0 ? ids : undefined },
    })),

  setUserIDs: (ids) =>
    set((state) => ({
      filter: { ...state.filter, userIDs: ids.length > 0 ? ids : undefined },
    })),

  resetFilter: () =>
    set({ filter: { ...defaultFilter } }),
}));
