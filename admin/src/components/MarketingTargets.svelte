<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 export let targets=[];
 export let aiEnabled=false;
 export let pending=false;
 const change=createEventDispatcher();
 const base='/marketing/cursemark/targets';
 const policies={links:'Links',self_promotion:'Self-promotion',ai:'AI and disclosure',flair:'Flair',cadence:'Cadence'};
 let form=null,editing=null,busy=false,researching=0,error='',researchError='',notice='';
 function open(target=null){
  editing=target;
  form={name:target?.name||'',url:target?.url||''};
  error='';
 }
 async function save(){
  busy=true;error='';notice='';
  try{
   const body={...form,version:editing?.version};
   const saved=await portalRequest(base+(editing?'/'+editing.id:''),editing?'PUT':'POST',body);
   change('saved',saved);
   notice=editing?'Target saved.':'Target added.';
   form=null;
  }catch(e){error=e.message;}
  finally{busy=false;}
 }
 async function research(target){
  researching=target.id;researchError='';notice='';
  try{
   const saved=await portalRequest(base+'/'+target.id+'/research','POST',{version:target.version});
   change('saved',saved);
   notice='Research updated for '+target.name+'.';
  }catch(e){researchError=e.message;}
  finally{researching=0;}
 }
</script>

<section aria-label="Target catalog">
 <div class="toolbar"><button type="button" on:click={()=>open()}>New Target</button></div>
 {#if form}
  <form aria-label={editing?'Edit target':'New target'} on:submit|preventDefault={save}>
   <label>Target name<input maxlength="160" bind:value={form.name} required/></label>
   <label>Public destination URL<input type="url" maxlength="1000" placeholder="https://www.reddit.com/r/example/" bind:value={form.url} required/></label>
   <div class="buttons"><button type="submit" disabled={busy}>{busy?'Saving…':'Save target'}</button><button type="button" class="secondary" disabled={busy} on:click={()=>{form=null;error='';}}>Cancel</button></div>
   {#if error}<p role="alert" class="error">{error}</p>{/if}
  </form>
 {/if}
 {#if notice}<p role="status">{notice}</p>{/if}
 {#if researchError}<p role="alert" class="error">{researchError}</p>{/if}
 {#if pending}<p role="status">Loading targets…</p>{:else if !targets.length}<p>No targets yet.</p>{/if}
 {#each targets as target (target.id)}
  <article class="card" aria-label={target.name}>
   <header>
    <div><h2>{target.name}</h2><a href={target.url} rel="noreferrer" target="_blank">{target.url}</a></div>
    <div class="buttons">
     <button type="button" class="secondary" aria-label={'Edit '+target.name} disabled={researching===target.id} on:click={()=>open(target)}>Edit</button>
     <button type="button" aria-label={'Update Research for '+target.name} disabled={!aiEnabled||!!researching} on:click={()=>research(target)}>{researching===target.id?'Researching…':'Update Research'}</button>
    </div>
   </header>
   {#if target.research}
    {@const snapshot=target.research}
    <details>
     <summary>{snapshot.hold?'Hold':'Research ready'} · Checked {new Date(target.checked_at).toISOString().slice(0,10)}</summary>
     {#if target.research_url!==target.url}<p>Stored research is for <a href={target.research_url} rel="noreferrer" target="_blank">the previous destination</a>. Update research before drafting.</p>{/if}
     {#if snapshot.hold}<p><strong>Hold.</strong> {snapshot.hold_reason}</p>{/if}
     {#if snapshot.takeaway}<p>{snapshot.takeaway}</p>{/if}
     <h3>Official rules</h3>
     <dl>{#each Object.entries(policies) as [key,label]}<div><dt>{label}</dt><dd>{snapshot.rules[key]||'Unverified'}</dd></div>{/each}</dl>
     {#if snapshot.sources.length}
      <ul>{#each snapshot.sources as source}<li><a href={source} rel="noreferrer" target="_blank">{source}</a></li>{/each}</ul>
     {:else}<p>No verified official rules source was found.</p>{/if}
     <h3>Recent promotion examples</h3>
     {#if snapshot.examples.length}
      <ul class="examples">{#each snapshot.examples as example (example.url)}
       <li><a href={example.url} rel="noreferrer" target="_blank">{example.title}</a>
        <p>{example.date}{example.engagement?' · '+example.engagement:' · Engagement unavailable'}</p>
        {#if example.what_worked}<p><strong>What appeared to work</strong> {example.what_worked}</p>{/if}
       </li>
      {/each}</ul>
     {:else}<p>No recent examples could be verified for this destination.</p>{/if}
    </details>
   {:else}<p>No research saved.</p>{/if}
  </article>
 {/each}
</section>

<style>
 section{text-align:left}
 h2,h3,p{margin:.4rem 0;text-align:left}
 h2{font-size:1.03rem}
 h3{font-size:.92rem;margin-top:.8rem}
 p,summary{font-size:.88rem;color:var(--text-muted)}
 .toolbar{margin:0 0 .75rem}
 .card,form{background:var(--surface);border:1px solid var(--line);border-radius:6px;padding:1rem;margin:0 0 .7rem;overflow-wrap:anywhere}
 header{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:start;gap:.6rem}
 header>div:first-child{min-width:0}
 form{display:grid;max-width:640px;gap:.65rem}
 label{display:grid;gap:.25rem;font-size:.9rem;font-weight:600}
 input,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 button{border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .75rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 .buttons{display:flex;flex-wrap:wrap;gap:.4rem}
 a{color:var(--forest);text-decoration:underline}
 summary{cursor:pointer;margin-top:.65rem;padding:.3rem 0}
 dl{margin:.4rem 0;font-size:.88rem}
 dl div{margin:.35rem 0}
 dt{font-weight:650}
 dd{margin:0;color:var(--text-muted)}
 ul{padding-left:1.25rem}
 .examples li{margin:.55rem 0}
 .error{color:var(--rust)}
</style>
