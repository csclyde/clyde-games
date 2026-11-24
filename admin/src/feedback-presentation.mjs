const discordSource = /^\s*(?:Discord (?:message|link):\s*)?(https:\/\/(?:www\.)?discord(?:app)?\.com\/channels\/\d+\/\d+\/\d+\/?)(?:\s*)$/i;

export function discordFeedbackParts(message) {
  const lines = (message || '').replace(/\r\n?/g, '\n').split('\n');
  let username = '', channel = '', fence = null;
  const body = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const code = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (code) {
      if (!fence) fence = code[1];
      else if (code[1][0] === fence[0] && code[1].length >= fence.length && !code[2].trim()) fence = null;
      body.push(line);
      continue;
    }
    const source = !fence && line.match(discordSource);
    if (!source) { body.push(line); continue; }
    const authorLine = lines[i + 1]?.match(/^Author:\s*(.+)$/i);
    const channelLine = lines[i + 2]?.match(/^Channel:\s*(.+)$/i);
    if (authorLine && channelLine) {
      if (!username) username = authorLine[1].trim().replace(/\s+\((?:\d+|unknown)\)\s*$/i, '');
      if (!channel) channel = channelLine[1].trim();
      i += 2;
    }
    body.push('', source[1], '');
  }
  if (/^(?:unknown(?: user)?|\d+)$/i.test(username)) username = '';
  if (/^(?:unknown|\d+)$/i.test(channel)) channel = '';
  return { body: body.join('\n').trim(), username, channel: channel ? '#' + channel.replace(/^#+/, '') : '' };
}

export function feedbackPresentation(item, displayed) {
  const discord = item.Platform?.toLowerCase() === 'discord' || item.Source === 'discord';
  const steamReview = item.Source === 'steam_reviews' || (item.Platform?.toLowerCase() === 'steam' && item.Category?.toLowerCase() === 'steam review');
  if (discord) {
    const { username, channel } = discordFeedbackParts(item.Message);
    return { body: discordFeedbackParts(displayed).body, discord, username: username || 'Discord user', category: channel || item.Category };
  }
  return { body: displayed, discord, username: steamReview ? item.DisplayAuthor || 'Steam reviewer' : item.PID, category: item.Category };
}
