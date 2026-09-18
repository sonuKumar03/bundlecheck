import moment from 'moment';

export function formatSimpleDate(d: Date): string {
  // Gotcha: Simple date formatting using heavy moment (70KB) instead of Intl.DateTimeFormat
  return moment(d).format('YYYY-MM-DD');
}
