// Browser-local acknowledgement of stable IDs, scoped to the portal account.
// No player text, credentials, report resolution or server mutations.
const LIMIT=5000;
const project=s=>typeof s==='string'&&s.trim().length<=64?s.trim().toLowerCase():'';
export const feedbackID=v=>typeof v==='string'&&/^[1-9][0-9]{0,19}$/.test(v)&&BigInt(v)<=18446744073709551615n?v:Number.isSafeInteger(v)&&v>0?String(v):null;
const bounded=ids=>[...new Set(ids.map(feedbackID).filter(Boolean))].sort((a,b)=>BigInt(a)<BigInt(b)?-1:1).slice(-LIMIT);
export function createFeedbackActivity({scope,storage,onChange=()=>{}}){
 const key='clyde-feedback-seen-v1:'+scope;
 let initialized=false,seen=Object.create(null),snapshots=Object.create(null),persistent=!!storage;
 function read(raw){const data=JSON.parse(raw);if(data?.v!==1||data.initialized!==true||!data.seen||typeof data.seen!=='object'||Object.keys(data.seen).length>200)throw Error('Invalid acknowledgement');return Object.fromEntries(Object.entries(data.seen).filter(([p,ids])=>project(p)===p&&Array.isArray(ids)&&ids.length<=LIMIT).map(([p,ids])=>[p,bounded(ids)]));}
 try{const raw=storage?.getItem(key);if(raw){seen=Object.assign(Object.create(null),read(raw));initialized=true;}}catch{persistent=false;}
 function persist(){try{storage?.setItem(key,JSON.stringify({v:1,initialized,seen}));}catch{persistent=false;}}
 function state(){const unseen=Object.create(null);for(const [p,s] of Object.entries(snapshots)){const known=new Set(seen[p]||[]);unseen[p]=!s.complete||s.ids.some(id=>!known.has(id));}return {unseen,any:Object.values(unseen).some(Boolean),persistent};}
 function emit(){const result=state();onChange(result);return result;}
 return {
  key,
  ingest(items){
   const next=Object.create(null);for(const item of items){const p=project(item.id);if(!p||!Array.isArray(item.openFeedbackIds)||item.openFeedbackIds.length>LIMIT||typeof item.feedbackIdsComplete!=='boolean')throw Error('Live feedback indicators unavailable.');const ids=item.openFeedbackIds.map(feedbackID);if(ids.some(id=>!id)||new Set(ids).size!==ids.length)throw Error('Invalid feedback activity response.');next[p]={ids,complete:item.feedbackIdsComplete};}
   if(!initialized){for(const [p,s] of Object.entries(next))if(s.complete)seen[p]=s.ids;initialized=true;persist();}
   snapshots=next;return emit();
  },
  review(p,ids){p=project(p);if(!p)return state();const next=bounded([...(seen[p]||[]),...ids]);if(JSON.stringify(next)===JSON.stringify(seen[p]||[]))return state();seen[p]=next;persist();return emit();},
  merge(raw){try{const other=read(raw);let changed=!initialized;for(const [p,ids] of Object.entries(other)){const next=bounded([...(seen[p]||[]),...ids]);if(JSON.stringify(next)!==JSON.stringify(seen[p]||[])){seen[p]=next;changed=true;}}initialized=true;if(changed){persist();emit();}}catch{}},
  state
 };
}

// One in-flight request. Background checks remain enabled for pinned tabs;
// browser suspension/throttling is caught up on focus, visibility and reconnect.
export function startNavigationPolling({load,apply,onError=()=>{},document,window,navigator,foreground=15000,background=60000,timeout=10000}){
 let stopped=false,busy=false,pending=false,timer,debounce,abort,failures=0;
 const delay=()=>Math.min((document.visibilityState==='hidden'?background:foreground)*2**Math.min(failures,5),300000);
 function schedule(){clearTimeout(timer);if(!stopped&&navigator.onLine!==false)timer=setTimeout(run,delay());}
 async function run(){
  if(stopped)return;if(busy){pending=true;return;}if(navigator.onLine===false){onError('Offline. Updates resume when connected.');return;}
  clearTimeout(timer);busy=true;abort=new AbortController();const deadline=setTimeout(()=>abort.abort(),timeout);
  try{const data=await load(abort.signal);if(!stopped){apply(data);failures=0;}}
  catch(e){if(!stopped){failures++;onError(e.message||'Updates paused; retrying.');}}
  finally{clearTimeout(deadline);busy=false;abort=null;if(!stopped){if(pending){pending=false;timer=setTimeout(run,0);}else schedule();}}
 }
 function refresh(){clearTimeout(debounce);debounce=setTimeout(run,100);}
 function visible(){if(document.visibilityState!=='hidden')refresh();else schedule();}
 function offline(){clearTimeout(timer);onError('Offline. Updates resume when connected.');}
 const listeners=[[window,'focus',refresh],[window,'online',refresh],[window,'offline',offline],[window,'pageshow',refresh],[document,'visibilitychange',visible]];
 for(const [target,event,fn] of listeners)target.addEventListener(event,fn);
 run();return {refresh,stop(){stopped=true;clearTimeout(timer);clearTimeout(debounce);abort?.abort();for(const [target,event,fn] of listeners)target.removeEventListener(event,fn);}};
}
