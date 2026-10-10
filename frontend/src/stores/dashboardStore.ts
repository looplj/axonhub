import { create } from 'zustand';
import type { RelativeTimeWindow } from '@/features/dashboard/utils/time-window';

export interface TimeRangeValue {
  startTime: string | null;
  endTime: string | null;
  timeWindow?: RelativeTimeWindow;
}

interface DashboardTimeState extends TimeRangeValue {
  setRange: (range: TimeRangeValue) => void;
  reset: () => void;
}

/** No preset carries concrete dates for a rolling window, so the dashboard opens on the
 * trailing 24 hours with no preset button highlighted. */
export const DEFAULT_TIME_RANGE: TimeRangeValue = {
  startTime: null,
  endTime: null,
  timeWindow: 'last24Hours',
};

/** Globally shared date range; dashboard and analytics read the same store so their date
 * filters stay in sync. The relative window is dashboard-only: the analytics page reads
 * just the two dates and stays unfiltered unless it picks a range itself. */
export const useDashboardTimeStore = create<DashboardTimeState>((set) => ({
  ...DEFAULT_TIME_RANGE,
  setRange: (range) => {
    // Both dates cleared means "nothing picked yet", which is not a state the page can
    // render — every card would fall back to its own default window.
    if (!range.startTime && !range.endTime) {
      set(DEFAULT_TIME_RANGE);
      return;
    }

    // timeWindow is written unconditionally: the filter's own handlers build ranges with
    // only the two date fields, so leaving it out would keep the relative window alive
    // and let it override the dates the user just picked.
    set({ startTime: range.startTime ?? null, endTime: range.endTime ?? null, timeWindow: range.timeWindow });
  },
  reset: () => set(DEFAULT_TIME_RANGE),
}));
