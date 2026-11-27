import {publicMarketingURL} from './marketing-targets.mjs';

const metricLabels = {
  youtube_views: 'views', youtube_short_views: 'views', reddit_score: 'net score'
};

function itemKey(href) {
  const u = new URL(href);
  const [kind, second] = u.pathname.split('/').filter(Boolean);
  const youtube = u.hostname === 'youtube.com' || u.hostname.endsWith('.youtube.com');
  const video = u.hostname === 'youtu.be' ? kind : !youtube ? null :
    kind === 'watch' ? u.searchParams.get('v') : ['shorts', 'live', 'embed'].includes(kind) ? second : null;
  if (/^[a-zA-Z0-9_-]{11}$/.test(video || '')) return 'youtube:' + video;
  if (u.hostname === 'reddit.com' || u.hostname.endsWith('.reddit.com')) {
    const match = u.pathname.match(/^\/r\/[a-z0-9_]+\/comments\/([a-z0-9]+)(?:\/[^/]+(?:\/([a-z0-9]+))?)?\/?$/i);
    if (match) return 'reddit:' + match[1].toLowerCase() + (match[2] ? ':' + match[2].toLowerCase() : '');
  }
  u.hash = '';
  return u.href;
}

export function coverageHistory(target) {
  const rows = new Map();
  for (const item of target.creator_evidence?.coverage?.items || []) {
    const href = publicMarketingURL(item.url);
    if (!href || !item.title) continue;
    const key = itemKey(href);
    const row = rows.get(key);
    if (!row || (!row.date && item.date)) rows.set(key, {href, title: item.title, date: item.date || '', metric: null});
  }
  for (const item of target.impact_evidence?.items || []) {
    const href = publicMarketingURL(item.url);
    const label = metricLabels[item.metric];
    if (!href || !item.title || !label) continue;
    const key = itemKey(href);
    const row = rows.get(key) || {href, title: item.title, date: '', metric: null};
    if (Number.isFinite(item.value) && (!row.metric || item.observed_on > row.metric.observedOn)) {
      row.metric = {value: item.value, label, observedOn: item.observed_on, shared: item.scope === 'shared'};
    }
    rows.set(key, row);
  }
  return Array.from(rows.values()).sort((a, b) => b.date.localeCompare(a.date) || a.title.localeCompare(b.title));
}
