<script>
 import {canWrite} from "../portal-auth.mjs";
 import {createEventDispatcher} from "svelte";
 import {stateLabels,canonicalState} from "../investigation-status.mjs";
 import InvestigationStatus from './InvestigationStatus.svelte';
 const dispatch=createEventDispatcher();
 export let kind;
 export let id;
 export let project = '';
 export let compactHeader = false;
 export let investigation = undefined;
 let open=false,items=[],after=0,more=false,loading=false,saving=false,loaded=false,error='',status='';
 let text='',state='',release='',pending=null;
 const canonical=p=>(p||'').trim().toLowerCase()==='cursemark'?'cursemark':(p||'').trim();
 $: scope=`${kind}/${id}?project=${encodeURIComponent(canonical(project))}`;
 $: if(open&&!loaded&&!loading) load();
 async function load(reset=false){
  const current=scope;loading=true;error='';
  try {
   const res=await fetch(`/api/report-notes/${current}&after_id=${reset?0:after}&limit=20`);const data=await res.json();if(!res.ok)throw new Error(data.error||'Could not read notes.');
   if(current!==scope)return;
   if(data.investigation)dispatch("changed",{id,project:canonical(project),investigation:data.investigation});
   items=reset?data.items:[...items,...data.items];after=data.next_after_id;more=data.has_more;loaded=true;
  }catch(e){error=e.message;loaded=true;}finally{loading=false;}
 }
 async function save(){
  const payload=JSON.stringify({note:text,state,release_state:release});
  if(!pending||pending.payload!==payload)pending={payload,key:crypto.randomUUID()};
  const current=scope;const request=pending;saving=true;error='';status='';
  try{
   const res=await fetch(`/api/report-notes/${current}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...JSON.parse(payload),request_key:request.key})});
   const data=await res.json();if(!res.ok)throw new Error(data.error||'Could not save note. Retry the same text safely.');
   if(current!==scope)return;
   text='';state='';release='';pending=null;status='Note saved. Report status unchanged.';await load(true);
  }catch(e){error=e.message;}finally{saving=false;}
 }
</script>

<details class="notes" data-report-kind={kind} data-report-id={id} bind:open>
 <summary>Investigation Notes{#if compactHeader} · <InvestigationStatus statuses={[investigation]} inline/>{:else} · {kind} #{id}{/if}</summary>
 {#if loading}<p role="status">Loading notes…</p>{/if}
 {#if loaded&&!items.length&&!loading&&!error}<p>No notes yet.</p>{/if}
 <ol>{#each items as note (note.id)}<li>
  <div class="stamp"><time datetime={note.created_at}>{new Date(note.created_at).toLocaleString()}</time> · {note.actor==='admin'?'Portal admin':'MCP'}{#if note.state} · {stateLabels[canonicalState(note.state)]||note.state}{/if}{#if note.release_state} · {note.release_state==='local'?'Local · unreleased':note.release_state}{/if}</div>
  <p class="text">{note.note}</p>

 </li>{/each}</ol>
 {#if more}<button disabled={loading} on:click={()=>load()}>Load newer notes</button>{/if}
 <button disabled={loading||saving} on:click={()=>load(true)}>Refresh notes</button>
 {#if error}<p class="error" role="alert">{error}</p>{/if}
 {#if $canWrite}<form on:submit|preventDefault={save}>
  <label>Note for the next investigation<textarea aria-label={`Note for ${kind} ${id}`} bind:value={text} maxlength="2000" required rows="3" disabled={saving} placeholder="What was tried, outcome, current fix/release state or blocker"></textarea></label>
  <div class="fields"><label>Investigation state<select aria-label={`Investigation state for ${id}`} bind:value={state} disabled={saving}><option value="">Context only</option><option value="investigating">Investigating</option><option value="blocked">Blocked</option><option value="needs_confirmation">Needs confirmation</option><option value="ready_for_review">Fix complete · ready for review</option></select></label>
  <label>Fix availability<select aria-label={`Fix availability for ${id}`} bind:value={release} disabled={saving}><option value="">Not specified</option><option value="local">Local · unreleased</option><option value="deployed">Deployed</option><option value="released">Released to players</option></select></label>
  <button disabled={saving||!text.trim()}>{saving?'Saving…':'Add note'}</button></div>
  <small>Concise database notes are the record; no report files are required. Ready for review leaves this item open. Only the owner closes reports manually. Corrections go in a new note.</small>
 </form>{:else}<p>Read-only account.</p>{/if}
 {#if status}<p role="status">{status}</p>{/if}
</details>

<style>
 .notes{margin-top:14px;border-top:1px solid var(--line);padding-top:10px;min-width:0;text-align:left;font-size:.8rem}.notes summary{cursor:pointer;font-weight:700}.notes p{margin:7px 0;text-align:left}.stamp,small{color:var(--text-soft);font-size:.72rem}.text{white-space:pre-wrap;overflow-wrap:anywhere}.notes ol{padding-left:20px}.notes li{padding:10px 0;border-bottom:1px solid var(--line)}form{margin-top:14px}label{display:flex;flex-direction:column;gap:5px;min-width:0}textarea,select,button{box-sizing:border-box;border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);color:var(--text);font:inherit;padding:7px}textarea{width:100%;resize:vertical}.fields{display:flex;align-items:end;flex-wrap:wrap;gap:10px;margin:9px 0}button{cursor:pointer}button:disabled{opacity:.55;cursor:default}.error{color:var(--rust)}@media(max-width:550px){.fields label,.fields button{width:100%}}
</style>
