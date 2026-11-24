import test from 'node:test';
import assert from 'node:assert/strict';
import {feedbackPresentation} from './feedback-presentation.mjs';
import {renderFeedbackMarkdown} from './feedback-markdown.mjs';
const link='https://discord.com/channels/1/2/3';
test('Discord source, name and channel no longer duplicate metadata in body',()=>{
 const message='>>> A quote\nSome more\n\nDiscord message: '+link+'\nAuthor: someone (5583085634)\nChannel: bugs';
 const p=feedbackPresentation({Platform:'Discord',Message:message},message);
 assert.equal(p.username,'someone');
 assert.equal(p.category,'#bugs');
 assert.equal(p.body,'>>> A quote\nSome more\n\n\n'+link);
 const rendered=renderFeedbackMarkdown(p.body,{discord:true});
 assert.match(rendered,/<\/details>\s*<p><a href="https:\/\/discord\.com\/channels/);
});
test('preserves attachment lines and strips footer from manually flagged replies', ()=>{
 const content='Original\n\nAttachments:\nhttps://cdn.example/a\n\nDiscord message: '+link+'\nAuthor: Primary (1)\nChannel: first\n\nManually flagged thread reply:\nAnother report\n\nDiscord Link: https://discord.com/channels/1/2/4\nAuthor: Secondary (2)\nChannel: second';
 const p=feedbackPresentation({Source:'discord',Message:content},content);
 assert.equal(p.username,'Primary');
 assert.equal(p.category,'#first');
 assert(!/Author:|Channel:|Discord Link:|Discord message:/.test(p.body));
 assert(p.body.includes('https://cdn.example/a') && p.body.includes('Another report'));
});
test('Steam reviewer fallback never displays the numeric id',()=>{
 const row={Platform:'Steam',Category:'Steam Review',PID:'76561197960435530'};
 assert.equal(feedbackPresentation(row,'Review').username,'Steam reviewer');
 assert.equal(feedbackPresentation({...row,DisplayAuthor:'Reviewer'},'Review').username,'Reviewer');
});
