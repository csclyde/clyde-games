import {targetRecipient} from './marketing-targets.mjs';

export function creatorOutreach(target) {
  const outreach=target.history?.outreach_status;
  const reply=target.creator_evidence?.reply;
  const replied=reply?.status==='replied_to_outreach';
  const initiated=reply?.status==='inbound_first'||outreach==='inbound_first';
  const contacted=outreach==='contacted'||replied||reply?.status==='bounced';
  const address=targetRecipient(target.creator?.email);
  let url='';
  if (address&&(contacted||initiated)) {
    const incoming=replied||initiated;
    const direction=incoming?'from:':'to:';
    const date=incoming&&/^\d{4}-\d{2}-\d{2}$/.test(reply?.date||'')?' on:'+reply.date.replace(/-/g,'/'):'';
    url='https://mail.google.com/mail/u/0/#search/'+encodeURIComponent(direction+address+date);
  }
  return {
    contact:outreach==='contacted'?'Contacted':initiated?'Creator reached out first':contacted?'Contacted':
      outreach==='none_found'?'No outreach found':outreach==='identity_unverified'?'Contact unverified':'Not checked',
    response:replied?'Replied':reply?.status==='bounced'?'Delivery failed':
      initiated?'Creator initiated':reply?.status==='no_reply_found'?'No reply found':'Not checked',
    responded:replied,
    date:(replied||reply?.status==='bounced'||initiated)?reply?.date||'':'',
    url,
    linkText:replied?'Find reply in Gmail':initiated?'Find conversation in Gmail':'Find outreach in Gmail'
  };
}
