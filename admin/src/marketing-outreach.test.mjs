import {test} from 'node:test';
import assert from 'node:assert/strict';
import {creatorOutreach} from './marketing-outreach.mjs';

const creator={email:'creator@example.com'};
const status=(outreach,status,date='')=>creatorOutreach({creator,history:{outreach_status:outreach},creator_evidence:{reply:{status,date}}});

test('creator summary distinguishes replies, unknowns, bounces, and inbound-first conversations',()=>{
 const replied=status('contacted','replied_to_outreach','2026-11-21');
 assert.equal(replied.contact,'Contacted');
 assert.equal(replied.response,'Replied');
 assert.equal(replied.date,'2026-11-21');
 assert.equal(decodeURIComponent(new URL(replied.url).hash),'#search/from:creator@example.com on:2026/11/21');
 assert.equal(status('contacted','no_reply_found').response,'No reply found');
 assert.equal(status('contacted','bounced').response,'Delivery failed');
 assert.equal(status('identity_unverified','not_checked').response,'Not checked');
 assert.equal(status('identity_unverified','not_checked').url,'');
 assert.equal(status('none_found','not_checked').contact,'No outreach found');
 assert.equal(status('inbound_first','inbound_first').contact,'Creator reached out first');
});
