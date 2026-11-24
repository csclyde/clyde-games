<script>
 import {canWrite} from '../portal-auth.mjs';
 import {createEventDispatcher} from 'svelte';
 import {stateLabels,canonicalState} from '../investigation-status.mjs';
 import InvestigationStatus from './InvestigationStatus.svelte';
 export let id, project='', family='';
 const dispatch=createEventDispatcher();
 const canonical=p=>(p||'').trim().toLowerCase()==='cursemark'?'cursemark':(p||'').trim();
 let summaryLoading=false,summaryScope='',summaryError='',groupStatus=undefined;
 let open=false,items=[],after=0,more=false,loading=false,loaded=false,error='',coverage=null;
 let text='',state='',release='',saving=false,pending=new Map(),status='',activeScope='';
 $: scope=`${id}?project=${encodeURIComponent(canonical(project))}${family?'&family='+encodeURIComponent(family):''}`;
 $: if(scope!==activeScope){activeScope=scope;items=[];after=0;more=false;loaded=false;error='';coverage=null;}
 $: if(scope!==summaryScope){summaryScope=scope;readSummary(scope);}
 $: if(open&&!loaded&&!loading)load();
 async function readSummary(current){summaryLoading=true;summaryError='';groupStatus=undefined;try{const r=await fetch(`/api/crash-investigation/${current}&limit=1`);const d=await r.json();if(!r.ok||!d.investigation||!stateLabels[canonicalState(d.investigation.state)])throw Error(d.error||'Status unavailable');if(current===scope)groupStatus=d.investigation;}catch(e){if(current===scope){summaryError=e.message;groupStatus={state:'unavailable'};}}finally{summaryLoading=false;}}

 async function load(reset=false){
  const current=scope;loading=true;error='';
  try{
   const res=await fetch(`/api/crash-investigation/${current}&after_id=${reset?0:after}&limit=20`);
   const data=await res.json();if(!res.ok||!Array.isArray(data.items)||!data.investigation)throw new Error(data.error||'Could not read investigation notes.');
   if(current!==scope)return;
   items=reset?data.items:[...items,...data.items];after=data.next_after_id;more=data.has_more;coverage=data;groupStatus=data.investigation;loaded=true;
  }catch(e){if(current===scope){error=e.message;loaded=true;}}finally{loading=false;}
 }
 async function save(){
  const payload=JSON.stringify({note:text,state,release_state:release}),current=scope,readScope=scope;
  const identity=JSON.stringify([current,payload]);
  if(!pending.has(identity))pending.set(identity,{key:crypto.randomUUID()});
  const request=pending.get(identity);saving=true;error='';status='';
  try{
   const res=await fetch(`/api/crash-investigation/${id}/notes?project=${encodeURIComponent(canonical(project))}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...JSON.parse(payload),request_key:request.key})});
   const data=await res.json();if(!res.ok)throw new Error(data.error||'Could not save note. Retry the same text safely.');
   dispatch('changed',{id,project:canonical(project)});
   if(readScope!==scope||current!==scope)return;
   text='';state='';release='';pending.delete(identity);status='Note saved. Report status unchanged.';await load(true);
  }catch(e){if(readScope===scope)error=e.message;}finally{saving=false;}
 }
</script>
<details class="crash-investigation" data-anchor-id={id} bind:open>
 <summary>Investigation Notes · <InvestigationStatus statuses={[groupStatus]} inline/>{#if groupStatus?.needs_review && groupStatus?.state!=='unreviewed'} <small>· New evidence / review pending</small>{/if}</summary>
 {#if open}
  {#if coverage}<p class="coverage">{coverage.note_count} notes across {coverage.report_count} stored reports, including resolved history. {#if family}Access violations share this investigation; each report keeps its own incident identity.{/if}</p>{/if}
  {#if loading}<p role="status">Loading investigation notes.</p>{/if}
  {#if loaded&&!items.length&&!loading&&!error}<p>No investigation notes yet.</p>{/if}
  <ol>{#each items as note (note.id)}<li>
   <div class="stamp">{#if note.scope==='group'}Group investigation{:else}Legacy report #{note.report_id}{/if}{#if note.report_resolved} · Resolved history{/if} · <time datetime={note.created_at}>{new Date(note.created_at).toLocaleString()}</time> · {note.actor==='admin'?'Portal admin':'MCP'}{#if note.state} · {stateLabels[canonicalState(note.state)]||note.state}{/if}{#if note.release_state} · {note.release_state==='local'?'Local · unreleased':note.release_state}{/if}</div>
   <p class="text">{note.note}</p>
  </li>{/each}</ol>
  {#if more}<button disabled={loading||saving} on:click={()=>load()}>Load newer notes</button>{/if}
  <button disabled={loading||saving} on:click={()=>load(true)}>Refresh investigation notes</button>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if $canWrite}<form on:submit|preventDefault={save}>

   <label>Note for the next investigation<textarea aria-label={`Group note for crash ${id}`} bind:value={text} maxlength="2000" required rows="3" disabled={saving} placeholder="What was tried, outcome, fix/release state or blocker"></textarea></label>
   <div class="fields"><label>Investigation state<select aria-label={`Group investigation state for ${id}`} bind:value={state} disabled={saving}><option value="">Context only</option><option value="investigating">Investigating</option><option value="blocked">Blocked</option><option value="needs_confirmation">Needs confirmation</option><option value="ready_for_review">Fix complete · ready for review</option></select></label>
   <label>Fix availability<select aria-label={`Group fix availability for ${id}`} bind:value={release} disabled={saving}><option value="">Not specified</option><option value="local">Local · unreleased</option><option value="deployed">Deployed</option><option value="released">Released to players</option></select></label>
   <button disabled={saving||!text.trim()}>{saving?'Saving.':'Add note'}</button></div>
   <small>One investigation belongs to this group; individual reports and their old notes remain evidence. Ready for review leaves it open; the owner closes reports manually. Corrections go in a new note.</small>
  </form>{:else}<p>Read-only account.</p>{/if}
  {#if status}<p role="status">{status}</p>{/if}
 {/if}
</details>
<style>
 .crash-investigation{border-top:1px solid var(--line);margin-top:10px;padding-top:10px;min-width:0;font-size:.8rem;text-align:left}.crash-investigation summary{cursor:pointer;font-weight:700;overflow-wrap:anywhere}.crash-investigation p{margin:7px 0;text-align:left}.coverage,.stamp,small{font-size:.72rem;color:var(--text-soft)}ol{padding-left:20px}li{padding:10px 0;border-bottom:1px solid var(--line)}.text{white-space:pre-wrap;overflow-wrap:anywhere}form{margin-top:14px}label{display:flex;flex-direction:column;gap:5px;min-width:0;margin-top:8px}textarea,select,button{box-sizing:border-box;border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);color:var(--text);font:inherit;padding:7px;max-width:100%}textarea{width:100%;resize:vertical}.fields{display:flex;align-items:end;flex-wrap:wrap;gap:10px;margin:9px 0}button{cursor:pointer}button:disabled{opacity:.55;cursor:default}.error{color:var(--rust)}@media(max-width:550px){.fields label,.fields button{width:100%}}
</style>
