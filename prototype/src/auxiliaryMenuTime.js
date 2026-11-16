import { formatDate } from './i18n.js';
// Shared specimens use a fixed UTC data day, not the viewer's local timezone.
export function syntheticSnapshotTime(dataDay) {
  const value=new Date(dataDay);
  value.setUTCHours(14,30,0,0);
  return value.toISOString();
}
export function formatSnapshotTime(value,lang) {
  return `${formatDate(new Date(value),lang,{dateStyle:'short',timeStyle:'short'})} UTC`;
}
