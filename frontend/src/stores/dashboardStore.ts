import { create } from 'zustand';

export interface TimeRangeValue {
  startTime: string | null;
  endTime: string | null;
}

interface DashboardTimeState extends TimeRangeValue {
  setRange: (range: TimeRangeValue) => void;
  reset: () => void;
}

/** Globally shared time range; dashboard and analytics read the same store so their
 * filters stay in sync. */
export const useDashboardTimeStore = create<DashboardTimeState>((set) => ({
  startTime: null,
  endTime: null,
  setRange: ({ startTime, endTime }) => set({ startTime, endTime }),
  reset: () => set({ startTime: null, endTime: null }),
}));
