import assert from 'node:assert/strict';
import test from 'node:test';
import {compareMarketingTargets,marketingComposer,marketingHandoff,marketingPlatformGroups,publicMarketingURL} from './marketing-targets.mjs';

test('impact order keeps New separate and platform groups use actual channel types', () => {
 const target=(name,level,extra={})=>({name,impact:{level},...extra});
 const rows=[target('Low',1),target('Unscored',0),target('New',0,{new_status:{is_new:true}}),target('OK',2),target('Great',3)];
 assert.deepEqual([...rows].sort(compareMarketingTargets).map(x=>x.name),['Great','OK','New','Unscored','Low']);
 const groups=marketingPlatformGroups([
  target('One',1,{kind:'creator',creator_platform:'twitch'}),
  target('Two',3,{kind:'creator',creator_platform:'twitch'}),
  target('Three',2,{kind:'creator',creator_platform:'youtube'}),
  target('Four',0,{kind:'destination',url:'http://old.reddit.com/r/Example'}),
  target('Four but fake',0,{kind:'destination',url:'https://reddit.com.example.invalid/r/Example'}),
 ]);
 assert.deepEqual(groups.map(g=>[g.name,g.targets.map(t=>t.name)]),[
  ['Twitch',['Two','One']],['YouTube',['Three']],['Reddit',['Four']],['Other destinations',['Four but fake']]
 ]);
});

test('Gmail passes only explicitly encoded fields for an exact private address', () => {
 const target={kind:'creator',creator:{email:'fixture+qa@example.invalid'}};
 const body='One & two\nこんにちは + 100%';
 const built=new URL(marketingHandoff(target,'Subject + # & café',body).url);
 assert.equal(built.origin,'https://mail.google.com');
 assert.equal(built.searchParams.get('to'),'fixture+qa@example.invalid');
 assert.equal(built.searchParams.get('su'),'Subject + # & café');
 assert.equal(built.searchParams.get('body'),body);
 assert.equal(built.searchParams.has('cc'),false);
 const full=marketingHandoff(target,'Subject','あ'.repeat(5000));
 assert.equal(new URL(full.url).searchParams.has('body'),false);
 assert.match(full.fallback,/Copy it/);
 for(const email of ['a@example.invalid,b@example.invalid','a@example.invalid\nBcc: b@example.invalid','Name <a@example.invalid>']){
  assert.equal(marketingComposer({kind:'creator',creator:{email}}),'');
 }
});

test('Reddit opens the exact community and X uses the documented body field', () => {
 const reddit=marketingHandoff({kind:'destination',url:'https://old.reddit.com/r/Example/'},'Private title','Private body');
 assert.equal(reddit.url,'https://www.reddit.com/r/Example/submit');
 assert.match(reddit.fallback,/copied title and body/);
 const x=marketingHandoff({kind:'destination',url:'https://twitter.com/Clyde'},'Ignored title','Body + # &');
 assert.equal(new URL(x.url).searchParams.get('text'),'Body + # &');
 assert.equal(marketingHandoff({kind:'destination',url:'https://x.com/Clyde'},'', 'x'.repeat(281)).url,'https://x.com/intent/tweet');
 for(const url of ['javascript:alert(1)','https://x.com@wrong.org','https://reddit.com.wrong.org/r/Example','https://www.reddit.com/r/NotValid-More/','https://wrong.org/x.com','https://x.com:123/']){
  assert.equal(marketingComposer({kind:'destination',url}),'');
 }
 for(const url of ['javascript:alert(1)','https://127.0.0.1','https://127.1/','https://[::1]','https://host.local/','https://user@host.test/']){
  assert.equal(publicMarketingURL(url),'');
 }
});
