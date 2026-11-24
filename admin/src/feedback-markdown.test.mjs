import assert from 'node:assert/strict';
import test from 'node:test';
import { renderFeedbackMarkdown, discordToMarkdown } from './feedback-markdown.mjs';

test('collapses each quote independently and keeps reply content outside', () => {
  const input = '> **Alice:**\n> ' + 'Words from the other user. '.repeat(21) + '\n> > Nested\n\nActual reply';
  const code = renderFeedbackMarkdown(input);
  assert.match(code, /<details class="feedback-quote"><summary><span[^>]*>Alice:/);
  assert.match(code, /…<\/span><\/summary>/);
  assert.equal((code.match(/<details/g) || []).length, 2);
  assert.doesNotMatch(code, /<details[^>]+\bopen\b/);
  assert.match(code, /<\/details>\s*<p>Actual reply<\/p>/);
});

test('handles ordinary Markdown but not raw HTML, active URLs or inline remote images', () => {
  const code = renderFeedbackMarkdown('## Heading\n\n**Bold** *italic* ~~strike~~ `code`\n\n<script>text</script>\n\n[bad](javascript:alert%281%29) [bad2](data:text/html,hello) ![See image](https://example.org/test.png)\n\nhttps://example.com');
  assert.match(code, /<h2>Heading<\/h2>/);
  assert.match(code, /<strong>Bold<\/strong> <em>italic<\/em> <s>strike<\/s> <code>code<\/code>/);
  assert.doesNotMatch(code, /<script|<img|href="(?:javascript|data):/i);
  assert.match(code, /href="https:\/\/example.org\/test.png" target="_blank" rel="noreferrer noopener">See image/);
  assert.match(code, /href="https:\/\/example.com"/);
});

test('Discord multiline quote leaves the source and attachments visible, and leaves code fences alone', () => {
  const input = '>>> Line one\nline two\n\nAttachments:\nhttps://cdn.example.net/file\n\nDiscord message: https://discord.com/channels/1/2/3';
  const code = renderFeedbackMarkdown(input,{discord:true});
  assert.match(code, /<details.*?>.*?Line one line two/s);
  assert.match(code, /<\/details>\s*<p>Attachments:/);
  assert.match(code, /href="https:\/\/discord.com\/channels\/1\/2\/3"/);
  const fenced = '```\n>>> stays literal\n```';
  assert.equal(discordToMarkdown(fenced),fenced);
});
