// Build is the API's date-bearing build ID. Never use report/receipt time.
export function crashBuildDate(build) {
 const match = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})?)?$/.exec(build);
 if (!match) return null;
 const [, y, m, d, h = '00', min = '00', sec = '00', fraction = '', zone = 'Z'] = match;
 if (+m < 1 || +m > 12 || +d < 1 || +h > 23 || +min > 59 || +sec > 59) return null;
 const date = new Date(0);
 date.setUTCFullYear(+y, +m - 1, +d);
 date.setUTCHours(+h, +min, +sec, +(fraction + '000').slice(0, 3));
 if (date.getUTCFullYear() !== +y || date.getUTCMonth() !== +m - 1 || date.getUTCDate() !== +d) return null;
 let offset = 0;
 if (zone !== 'Z') {
  const hours = +zone.slice(1, 3), minutes = +zone.slice(4, 6);
  if (hours > 23 || minutes > 59) return null;
  offset = (hours * 60 + minutes) * (zone[0] === '+' ? 1 : -1);
 }
 return { time: date.getTime() - offset * 60000, submillisecond: +(fraction.padEnd(9, '0').slice(3) || 0), day: `${y}-${m}-${d}` };
}
const compareText = (a, b) => a < b ? -1 : a > b ? 1 : 0;
// Legacy Count may span unknown builds. /crash/builds is report-derived, not an
// authoritative release catalog, so it cannot justify inferred zero-count bars.
export function crashBuildSeries(reports = []) {
 const buckets = new Map();
 let legacyRows = 0, legacyOccurrences = 0, recorded = 0;
 for (const report of reports) {
  if (report.OccurrenceVersion !== 1) { legacyRows++; legacyOccurrences += report.Count ?? 0; continue; }
  const project = report.Project || '';
  const build = typeof report.Build === 'string' && report.Build.trim() ? report.Build : '';
  const key = JSON.stringify([project, build]);
  if (!buckets.has(key)) buckets.set(key, { key, project, build, date: crashBuildDate(build), count: 0 });
  buckets.get(key).count++;
  recorded++;
 }
 const builds = [...buckets.values()].sort((a, b) => {
  if (!!a.date !== !!b.date) return a.date ? -1 : 1;
  return (a.date && b.date ? a.date.time - b.date.time || a.date.submillisecond - b.date.submillisecond : 0)
   || compareText(a.project, b.project) || compareText(a.build, b.build);
 });
 return { builds, recorded, legacyRows, legacyOccurrences, max: Math.max(1, ...builds.map(build => build.count)) };
}
