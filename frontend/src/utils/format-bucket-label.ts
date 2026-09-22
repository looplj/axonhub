/** Bucket labels are "YYYY-MM-DD" for daily resolution and "YYYY-MM-DD HH:00" for
 * hourly, so the label itself tells us which granularity the server chose. */
export function formatBucketLabel(bucket: string, locale: string): string {
  const isHourly = bucket.includes(' ');
  const [datePart, timePart] = isHourly ? bucket.split(' ') : [bucket, undefined];
  const [year, month, day] = datePart.split('-').map(Number);
  const dateObj = new Date(Date.UTC(year, month - 1, day));

  if (timePart) {
    return dateObj.toLocaleString(locale, {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      hour12: false,
      timeZone: 'UTC',
    });
  }

  return dateObj.toLocaleDateString(locale, {
    month: '2-digit',
    day: '2-digit',
    timeZone: 'UTC',
  });
}
