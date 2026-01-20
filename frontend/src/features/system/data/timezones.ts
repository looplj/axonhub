import { getTimeZones as tzGetTimeZones } from '@vvo/tzdb';

const utcTimezone = {
  name: 'UTC',
  alternativeName: 'Coordinated Universal Time (UTC)',
  abbreviation: 'UTC',
  group: ['UTC', 'ETC/UTC', 'ETC/UCT'],
  countryName: '',
  continentCode: '',
  continentName: '',
  mainCities: [''],
  rawOffsetInMinutes: 0,
  rawFormat: '+00:00 Coordinated Universal Time (UTC)',
  currentTimeOffsetInMinutes: 0,
  currentTimeFormat: '+00:00 Coordinated Universal Time (UTC)',
};

const ALL_VALID_TIMEZONES = tzGetTimeZones();
// @ts-ignore
ALL_VALID_TIMEZONES.push(utcTimezone);

export const getTimeZones = () => {
  return ALL_VALID_TIMEZONES;
};

export const GMTTimeZoneOptions = getTimeZones()
  .map((tz) => {
    const timeFormat = tz.currentTimeFormat;
    return {
      time: timeFormat,
      value: tz.name,
      label: `(GMT${timeFormat.slice(0, 6)}) ${tz.name}`,
    };
  })
  .sort((a, b) => {
    if (a.label < b.label) {
      return -1;
    }
    if (a.label > b.label) {
      return 1;
    }
    return 0;
  });
