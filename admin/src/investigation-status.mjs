export const canonicalProject=p=>(p||'').trim().toLowerCase()==='cursemark'?'cursemark':(p||'').trim();
export const statusKey=r=>`${canonicalProject(r.Project)}:${r.ID}`;
export const stateLabels={unreviewed:'Unreviewed',investigating:'Investigating',blocked:'Blocked',needs_confirmation:'Needs confirmation',ready_for_review:'Fix complete · ready for review'};
export const canonicalState=s=>s==='fixed'?'ready_for_review':s;
export async function loadInvestigationStatuses(kind,reports,fetcher=fetch){
 const groups=new Map(),out={};
 for(const r of reports){const p=canonicalProject(r.Project);if(!groups.has(p))groups.set(p,new Set());groups.get(p).add(r.ID);}
 await Promise.all([...groups].map(async([project,set])=>{
  const ids=[...set];for(let i=0;i<ids.length;i+=200){const batch=ids.slice(i,i+200);
   try{const res=await fetcher(`/api/report-investigations?kind=${kind}&project=${encodeURIComponent(project)}&ids=${batch.join(',')}`);const data=await res.json();if(!res.ok||!data.complete||data.project!==project||data.kind!==kind||!Array.isArray(data.items)||data.items.length!==batch.length)throw Error('Status unavailable');
    const seen=new Set();for(const s of data.items){if(!batch.includes(s.id)||seen.has(s.id)||!stateLabels[canonicalState(s.state)])throw Error('Invalid status response');seen.add(s.id)}
    for(const s of data.items)out[`${project}:${s.id}`]={...s,state:canonicalState(s.state)};
   }catch(e){for(const id of batch)out[`${project}:${id}`]={state:'unavailable'};}
  }
 }));return out;
}
export function summarizeInvestigation(statuses){
 if(!statuses.length||statuses.some(s=>!s))return {state:'loading',label:'Investigation status loading'};
 if(statuses.some(s=>s.state==='unavailable'))return {state:'unavailable',label:'Investigation status unavailable'};
 const counts=new Map();for(const s of statuses){const state=canonicalState(s.state);counts.set(state,(counts.get(state)||0)+1)}
 const state=counts.size===1?[...counts.keys()][0]:'mixed';
 const label=state==='mixed'?`Mixed investigation states · ${[...counts].map(([s,n])=>`${n} ${stateLabels[s].toLowerCase()}`).join(' · ')}`:stateLabels[state];
 const release=statuses.every(s=>s.release_state===statuses[0].release_state)?statuses[0].release_state:'';
 return {state,label,release,noteCount:statuses.reduce((n,s)=>n+(s.note_count||0),0)};
}
