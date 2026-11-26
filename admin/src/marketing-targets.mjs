const impactLabels = ['Unscored', 'Low', 'OK', 'Great'];
const platformLabels = {
  youtube: 'YouTube', twitch: 'Twitch', reddit: 'Reddit', x: 'X',
  bluesky: 'Bluesky', itch: 'itch.io', other: 'Other creators', web: 'Other destinations'
};
const platformOrder = ['twitch', 'youtube', 'reddit', 'x', 'bluesky', 'itch', 'other', 'web'];
const belongsTo = (host, domain) => host === domain || host.endsWith('.' + domain);

function safeURL(raw, allowLegacyHTTP = false) {
  if (typeof raw !== 'string' || !raw || /[\\<>\x00-\x20\x7f]/u.test(raw)) return null;
  try {
    const url = new URL(raw);
    const hostname = url.hostname.replace(/\.$/, '');
    if ((url.protocol !== 'https:' && (!allowLegacyHTTP || url.protocol !== 'http:')) ||
        url.username || url.password || url.port || !hostname.includes('.') ||
        /^[\d.]+$/.test(hostname) || /\.(?:local|localhost|internal)$/u.test(hostname)) return null;
    return url;
  } catch {
    return null;
  }
}

export const publicMarketingURL = raw => safeURL(raw)?.href || '';

function hostPlatform(raw) {
  const host = safeURL(raw, true)?.hostname.replace(/\.$/, '');
  if (!host) return '';
  if (belongsTo(host, 'reddit.com')) return 'reddit';
  if (belongsTo(host, 'youtube.com') || host === 'youtu.be') return 'youtube';
  if (belongsTo(host, 'twitch.tv')) return 'twitch';
  if (belongsTo(host, 'x.com') || belongsTo(host, 'twitter.com')) return 'x';
  if (belongsTo(host, 'bsky.app')) return 'bluesky';
  if (belongsTo(host, 'itch.io')) return 'itch';
  return '';
}

function platformKey(target) {
  const detected = hostPlatform(target.url);
  if (target.kind === 'creator') {
    if (['youtube', 'twitch'].includes(target.creator_platform)) return target.creator_platform;
    return detected || 'other';
  }
  return detected || 'web';
}

export function targetPlatform(target) {
  const key = platformKey(target);
  if (key === 'web') return 'Public destination';
  if (key === 'other') return 'Other creator';
  return platformLabels[key];
}

export function impactLevel(target) {
  return Number.isInteger(target.impact?.level) && impactLabels[target.impact.level] ? target.impact.level : 0;
}

export const impactLabel = target => impactLabels[impactLevel(target)];

function priority(target) {
  const level = impactLevel(target);
  if (level === 3) return 0;
  if (level === 2) return 1;
  if (target.new_status?.is_new) return 2;
  return level === 0 ? 3 : 4;
}

export function compareMarketingTargets(a, b) {
  return priority(a) - priority(b) || a.name.localeCompare(b.name, 'en', {numeric: true, sensitivity: 'base'}) || a.id - b.id;
}

export function marketingPlatformGroups(targets) {
  const byPlatform = new Map();
  for (const target of [...targets].sort(compareMarketingTargets)) {
    const key = platformKey(target);
    if (!byPlatform.has(key)) byPlatform.set(key, []);
    byPlatform.get(key).push(target);
  }
  return platformOrder.filter(key => byPlatform.has(key)).
    map(key => ({id: key, name: platformLabels[key], targets: byPlatform.get(key)}));
}

export function targetRecipient(value) {
  if (typeof value !== 'string' || value.length > 254) return '';
  const oneAddress = /^[A-Za-z0-9!#$%&'*+/=?^_`{|}~.-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$/u;
  return oneAddress.test(value) ? value : '';
}

function subredditName(raw) {
  if (hostPlatform(raw) !== 'reddit') return '';
  return safeURL(raw, true).pathname.match(/^\/r\/([A-Za-z0-9_]{2,30})\/?$/iu)?.[1] || '';
}

export function marketingComposer(target) {
  if (target.kind === 'creator') return targetRecipient(target.creator?.email) ? 'gmail' : '';
  if (subredditName(target.url)) return 'reddit';
  return hostPlatform(target.url) === 'x' ? 'x' : '';
}

export function marketingHandoff(target, title, body) {
  const composer = marketingComposer(target);
  if (composer === 'gmail') {
    const url = new URL('https://mail.google.com/mail/');
    url.search = new URLSearchParams({view: 'cm', fs: '1', to: targetRecipient(target.creator.email), su: title.replace(/[\r\n]+/g, ' ')});
    url.searchParams.set('body', body);
    // Some browser/gateway paths reject long multibyte query strings. Keep
    // the recipient and subject and offer the full body through Copy Body.
    if (url.href.length > 7500) {
      url.searchParams.delete('body');
      return {url: url.href, fallback: 'The full body is too long to prefill reliably. Copy it and paste it in Gmail.'};
    }
    return {url: url.href, fallback: ''};
  }

  if (composer === 'reddit') {
    return {url: 'https://www.reddit.com/r/' + subredditName(target.url) + '/submit',
      fallback: 'Paste the copied title and body into Reddit.'};
  }

  if (composer === 'x') {
    const url = new URL('https://x.com/intent/tweet');
    const fallback = [...body.replace(/https?:\/\/[^\s]+/gu, 'x'.repeat(23))].
      reduce((sum, char) => sum + (char.codePointAt(0) > 0x10ff ? 2 : 1), 0) > 280;
    if (!fallback && body) url.searchParams.set('text', body);
    return {url: url.href, fallback: fallback ? 'Copy the full body and shorten it to fit in X.' : ''};
  }
  return null;
}
