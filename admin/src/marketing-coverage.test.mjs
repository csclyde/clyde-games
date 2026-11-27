import {test} from 'node:test';
import assert from 'node:assert/strict';
import {coverageHistory} from './marketing-coverage.mjs';

test('history joins video aliases, keeps the published date and uses the newest known count', () => {
 const rows=coverageHistory({
  creator_evidence:{coverage:{items:[
   {title:'First video',url:'https://youtu.be/ASDF1234567?t=10',date:'2026-01-15'},
   {title:'Later stream',url:'https://www.twitch.tv/videos/12345',date:'2026-05-02'}
  ]}},
  impact_evidence:{items:[
   {title:'Video',url:'https://www.youtube.com/watch?v=ASDF1234567',metric:'youtube_views',value:0,observed_on:'2026-06-01'},
   {title:'Earlier result',url:'https://www.youtube.com/live/ASDF1234567',metric:'youtube_views',value:500,observed_on:'2026-05-01'}
  ]}
 });
 assert.deepEqual(rows.map(row=>[row.title,row.date,row.metric?.value]),[
  ['Later stream','2026-05-02',undefined],['First video','2026-01-15',0]
 ]);
});

test('impact-only links stay undated and shared post scores retain their context', () => {
 const target={impact_evidence:{items:[
  {title:'Unsafe',url:'javascript:alert(1)',metric:'reddit_score',value:700},
  {title:'A post',url:'https://www.reddit.com/r/roguelikes/comments/1abc/post/',metric:'reddit_score',value:32,observed_on:'2026-03-01',scope:'shared'},
  {title:'A comment',url:'https://www.reddit.com/r/roguelikes/comments/1abc/post/2def/',metric:'reddit_score',value:null,observed_on:'2026-03-01'}
 ]}};
 const [comment,post]=coverageHistory(target);
 assert.equal(comment.title,'A comment');
 assert.equal(comment.metric,null);
 assert.equal(post.date,'');
 assert.deepEqual(post.metric,{value:32,label:'net score',observedOn:'2026-03-01',shared:true});
 assert.deepEqual(coverageHistory({}),[]);
});
