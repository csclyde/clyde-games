import MarkdownIt from 'markdown-it';

const md = new MarkdownIt({ html: false, linkify: true, breaks: true, maxNesting: 20 });
md.validateLink = url => /^https?:\/\//i.test(url);
const esc = md.utils.escapeHtml;
const baseLink = md.renderer.rules.link_open || ((tokens, i, options, _env, self) => self.renderToken(tokens, i, options));
md.renderer.rules.link_open = (tokens, i, options, env, self) => {
  tokens[i].attrSet('target', '_blank');
  tokens[i].attrSet('rel', 'noreferrer noopener');
  return baseLink(tokens, i, options, env, self);
};
// Don't contact external servers (or expose admin browsing) until someone clicks.
md.renderer.rules.image = (tokens, i) => {
  const token = tokens[i], href = token.attrGet('src');
  const label = token.content || 'Image';
  return href && md.validateLink(href) ? '<a href="' + esc(href) + '" target="_blank" rel="noreferrer noopener">' + esc(label) + '</a>' : esc(label);
};

function quotePreview(tokens, start) {
  const chunks = [];
  for (let i = start + 1; i < tokens.length; i++) {
    const t = tokens[i];
    if (t.type === 'blockquote_close' && t.level === tokens[start].level) break;
    if (t.type === 'inline' && t.children) {
      chunks.push(t.children.map(c => c.type === 'softbreak' || c.type === 'hardbreak' ? ' ' : c.type === 'text' || c.type === 'code_inline' || c.type === 'image' ? c.content : '').join(''));
    }
    if (t.type === 'fence' || t.type === 'code_block') chunks.push(t.content);
    if (chunks.join(' ').length > 300) break;
  }
  const value = chunks.join(' ').replace(/\s+/g, ' ').trim();
  const characters = [...value];
  return characters.length > 240 ? characters.slice(0, 239).join('') + '…' : value;
}
md.renderer.rules.blockquote_open = (tokens, i) =>
  '<details class="feedback-quote"><summary><span class="feedback-quote-preview">' +
  esc(quotePreview(tokens, i) || 'Quoted message') + '</span></summary><blockquote>\n';
md.renderer.rules.blockquote_close = () => '</blockquote></details>\n';

// Discord uses native Markdown plus a >>> prefix to quote the rest of a message.
// The bot appends provenance lines; those should remain visible outside the quote.
export function discordToMarkdown(message) {
  let fence = null, quoted = false;
  return message.replace(/\r\n?/g, '\n').split('\n').map(line => {
    if (/^(?:(?:Discord (?:message|link): )?https:\/\/(?:www\.)?discord(?:app)?\.com\/channels\/|Manually flagged (?:thread|forum) reply:|Attachments:$)/.test(line) && !fence) {
      quoted = false;
      return '\n' + line;
    }
    const out = !fence && /^\s*>>>\s/.test(line) ? (quoted = true, line.replace(/^\s*>>>\s/, '')) : line;
    const opening = out.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (opening) {
      const mark = opening[1];
      if (!fence) fence = mark;
      else if (mark[0] === fence[0] && mark.length >= fence.length && !opening[2].trim()) fence = null;
    }
    return quoted ? '> ' + out : out;
  }).join('\n');
}

export function renderFeedbackMarkdown(message, { discord = false } = {}) {
  const text = typeof message === 'string' ? message : '';
  return md.render(discord ? discordToMarkdown(text) : text);
}
