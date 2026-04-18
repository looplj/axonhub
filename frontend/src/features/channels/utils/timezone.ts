import { parseISO } from 'date-fns';

function normalizeUtcIso(iso: string): string {
  if (!iso.endsWith('Z') && !/[+-]\d{2}:?\d{2}$/.test(iso)) {
    return iso + 'Z';
  }
  return iso;
}

function parseUtcIso(iso: string): Date {
  return parseISO(normalizeUtcIso(iso));
}

function pad(n: number): string {
  return String(n).padStart(2, '0');
}

interface TzDateTimeParts {
  year: string;
  month: string;
  day: string;
  hour: string;
  minute: string;
  second?: string;
}

function getTzParts(date: Date, timezone: string, withSeconds = false): TzDateTimeParts | null {
  try {
    const options: Intl.DateTimeFormatOptions = {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
      timeZone: timezone, hour12: false,
    };
    if (withSeconds) options.second = '2-digit';

    const parts = new Intl.DateTimeFormat('en-CA', options).formatToParts(date);
    const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';

    let hour = parseInt(get('hour'), 10);
    if (hour === 24) hour = 0;

    return {
      year: get('year'),
      month: get('month'),
      day: get('day'),
      hour: pad(hour),
      minute: get('minute'),
      ...(withSeconds ? { second: get('second') } : {}),
    };
  } catch {
    return null;
  }
}

export function formatInTz(iso: string | null | undefined, timezone: string, formatStr: 'yyyy-MM-dd HH:mm' | 'HH:mm:ss' | 'yyyy-MM-dd HH:mm:ss'): string {
  if (!iso) return '';
  try {
    const d = parseUtcIso(iso);
    const withSeconds = formatStr === 'HH:mm:ss' || formatStr === 'yyyy-MM-dd HH:mm:ss';
    const p = getTzParts(d, timezone, withSeconds);
    if (!p) return '';

    switch (formatStr) {
      case 'yyyy-MM-dd HH:mm':
        return `${p.year}-${p.month}-${p.day} ${p.hour}:${p.minute}`;
      case 'HH:mm:ss':
        return `${p.hour}:${p.minute}:${p.second ?? '00'}`;
      case 'yyyy-MM-dd HH:mm:ss':
        return `${p.year}-${p.month}-${p.day} ${p.hour}:${p.minute}:${p.second ?? '00'}`;
    }
  } catch {
    return '';
  }
  return '';
}

export function utcToTzDatetime(iso: string | null | undefined, timezone: string): string {
  if (!iso) return '';
  try {
    const d = parseUtcIso(iso);
    const p = getTzParts(d, timezone, false);
    if (!p) return '';
    return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`;
  } catch {
    return '';
  }
}

export function tzDatetimeToUtc(localValue: string, timezone: string): string | null {
  if (!localValue) return null;
  try {
    const [datePart, timePart] = localValue.split('T');
    if (!datePart || !timePart) return null;
    const [year, month, day] = datePart.split('-').map(Number);
    const timeSegments = timePart.split(':').map(Number);
    const hour = timeSegments[0];
    const minute = timeSegments[1] || 0;
    const second = timeSegments[2] || 0;
    if (isNaN(year) || isNaN(month) || isNaN(day) || isNaN(hour) || isNaN(minute) || isNaN(second)) return null;

    const seedMs = Date.UTC(year, month - 1, day, hour, minute, second);
    let utcMs = seedMs - getTimezoneOffsetMs(timezone, new Date(seedMs));
    for (let i = 0; i < 2; i++) {
      utcMs = seedMs - getTimezoneOffsetMs(timezone, new Date(utcMs));
    }
    return new Date(utcMs).toISOString();
  } catch {
    return null;
  }
}

export function getTzTimeValue(iso: string | null | undefined, timezone: string): string {
  if (!iso) return '';
  try {
    const d = parseUtcIso(iso);
    const p = getTzParts(d, timezone, false);
    if (!p) return '';
    return `${p.hour}:${p.minute}`;
  } catch {
    return '';
  }
}

export function toBrowserLocalTime(iso: string | null | undefined): string {
  if (!iso) return '';
  try {
    const d = parseUtcIso(iso);
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  } catch {
    return '';
  }
}

export function fromBrowserLocalTime(browserLocalTime: string, datePartInTz: string, timezone: string): string | null {
  if (!browserLocalTime) return null;
  try {
    const [year, month, day] = datePartInTz.split('-').map(Number);
    const [bh, bm] = browserLocalTime.split(':').map(Number);
    if (isNaN(year) || isNaN(month) || isNaN(day) || isNaN(bh) || isNaN(bm)) return null;

    const browserLocalDate = new Date(year, month - 1, day, bh, bm, 0);
    const utcInstant = new Date(browserLocalDate.getTime());
    const wallClockInConfiguredTz = getTzParts(utcInstant, timezone, false);
    if (!wallClockInConfiguredTz) return null;

    return tzDatetimeToUtc(
      `${wallClockInConfiguredTz.year}-${wallClockInConfiguredTz.month}-${wallClockInConfiguredTz.day}T${wallClockInConfiguredTz.hour}:${wallClockInConfiguredTz.minute}`,
      timezone,
    );
  } catch {
    return null;
  }
}

export function getTzDateParts(timezone: string): { year: number; month: number; day: number } {
  const now = new Date();
  const p = getTzParts(now, timezone, false);
  if (p) {
    return { year: parseInt(p.year, 10), month: parseInt(p.month, 10), day: parseInt(p.day, 10) };
  }
  return { year: now.getFullYear(), month: now.getMonth() + 1, day: now.getDate() };
}

function getTimezoneOffsetMs(timezone: string, referenceDate: Date): number {
  try {
    const tzParts = new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone,
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    }).formatToParts(referenceDate);
    const utcParts = new Intl.DateTimeFormat('en-CA', {
      timeZone: 'UTC',
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    }).formatToParts(referenceDate);
    const get = (parts: Intl.DateTimeFormatPart[], type: string) => parseInt(parts.find((p) => p.type === type)?.value ?? '0', 10);
    const tzMs = Date.UTC(get(tzParts, 'year'), get(tzParts, 'month') - 1, get(tzParts, 'day'), get(tzParts, 'hour'), get(tzParts, 'minute'), get(tzParts, 'second'));
    const utcMs = Date.UTC(get(utcParts, 'year'), get(utcParts, 'month') - 1, get(utcParts, 'day'), get(utcParts, 'hour'), get(utcParts, 'minute'), get(utcParts, 'second'));
    return tzMs - utcMs;
  } catch {
    return 0;
  }
}

export function getTimezoneAbbrev(timezone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      timeZoneName: 'short',
    }).formatToParts(new Date());
    const tzPart = parts.find((p) => p.type === 'timeZoneName');
    return tzPart?.value ?? timezone;
  } catch {
    return timezone;
  }
}
