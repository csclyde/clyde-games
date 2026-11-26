<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 export let target;
 const changed=createEventDispatcher();
 const coverageLabels={covered:'Verified public coverage',self_reported:'Creator reported coverage',no_result:'No verified result found',inaccessible:'Channel access limited',not_checked:'Not checked'};
 const replyLabels={replied_to_outreach:'Replied to outreach',inbound_first:'Creator contacted Casey first',bounced:'Delivery failure found',no_reply_found:'No reply found',not_checked:'Not checked'};
 let draft=null,busy=false,error='';
 $: evidence=target.creator_evidence;
 function edit(){
  draft=JSON.parse(JSON.stringify(evidence));
  draft.coverage.items??=[];
  error='';
 }
 function addItem(){draft.coverage.items=[...(draft.coverage.items||[]),{title:'',url:'',date:'',format:'video'}];}
 async function save(){
  busy=true;error='';
  try{
   const {coverage,reply,version}=draft;
   if(coverage.status!=='covered')coverage.items=[];
   if(coverage.status==='not_checked')coverage.checked_at='';
   if(reply.status==='not_checked')reply.checked_at='';
   if(['not_checked','no_reply_found'].includes(reply.status))reply.date='';
   const saved=await portalRequest('/marketing/cursemark/targets/'+target.id+'/creator-evidence','PUT',{version,coverage,reply});
   changed('saved',saved);
   draft=null;
  }catch(e){error=e.message;}
  finally{busy=false;}
 }
</script>

{#if evidence}
 <section aria-label="Cursemark coverage and outreach">
  <header><h3>Cursemark coverage and outreach</h3><button type="button" on:click={edit}>Edit creator report</button></header>
  {#if draft}
   <form on:submit|preventDefault={save}>
    <label>Cursemark coverage
     <select bind:value={draft.coverage.status}>{#each Object.entries(coverageLabels) as [id,label]}<option value={id}>{label}</option>{/each}</select>
    </label>
    {#if draft.coverage.status!=='not_checked'}<label>Coverage checked<input type="date" bind:value={draft.coverage.checked_at} required/></label>{/if}
    <label>Coverage summary<textarea rows="2" maxlength="1000" bind:value={draft.coverage.note}></textarea></label>
    {#if draft.coverage.status==='covered'}
     {#each draft.coverage.items as item,i}
      <fieldset><legend>Public coverage {i+1}</legend>
       <label>Title<input maxlength="250" required bind:value={item.title}/></label>
       <label>Public URL<input type="url" required bind:value={item.url}/></label>
       <label>Date<input type="date" bind:value={item.date}/></label>
       <label>Format<select bind:value={item.format}>{#each ['video','short','stream','post','article','other'] as type}<option value={type}>{type}</option>{/each}</select></label>
       <button type="button" on:click={()=>draft.coverage.items=draft.coverage.items.filter((_,index)=>index!==i)}>Remove item</button>
      </fieldset>
     {/each}
     <button type="button" disabled={draft.coverage.items.length>=8} on:click={addItem}>Add a public source</button>
    {/if}
    <label>Outreach status<select bind:value={draft.reply.status}>{#each Object.entries(replyLabels) as [id,label]}<option value={id}>{label}</option>{/each}</select></label>
    {#if draft.reply.status!=='not_checked'}<label>Outreach checked<input type="date" bind:value={draft.reply.checked_at} required/></label>{/if}
    {#if !['not_checked','no_reply_found'].includes(draft.reply.status)}<label>Event date<input type="date" required bind:value={draft.reply.date}/></label>{/if}
    <label>Outreach summary<textarea rows="2" maxlength="1000" bind:value={draft.reply.note}></textarea></label>
    <div><button class="primary" type="submit" disabled={busy}>Save creator report</button> <button type="button" on:click={()=>draft=null}>Cancel</button></div>
    {#if error}<p role="alert">{error}</p>{/if}
   </form>
  {:else}
   <p><strong>{coverageLabels[evidence.coverage.status]}</strong>{evidence.coverage.checked_at?' · Checked '+evidence.coverage.checked_at:''}</p>
   {#if evidence.coverage.note}<p>{evidence.coverage.note}</p>{/if}
   {#if evidence.coverage.items?.length}
    <ul>{#each evidence.coverage.items as item}<li><a href={item.url} rel="noreferrer" target="_blank">{item.title}</a> · {item.format}{item.date?' · '+item.date:''}</li>{/each}</ul>
   {/if}
   <p><strong>{replyLabels[evidence.reply.status]}</strong>{evidence.reply.date?' · '+evidence.reply.date:''}{evidence.reply.checked_at?' · Checked '+evidence.reply.checked_at:''}</p>
   {#if evidence.reply.note}<p>{evidence.reply.note}</p>{/if}
  {/if}
 </section>
{/if}

<style>
 section{border-top:1px solid var(--line);margin-top:.8rem;padding-top:.7rem;overflow-wrap:anywhere}
 header{display:flex;gap:.5rem;align-items:center;justify-content:space-between;flex-wrap:wrap}
 h3,p{margin:.4rem 0;text-align:left}
 h3{font-size:.92rem}
 p,li{font-size:.87rem}
 a{color:var(--forest);text-decoration:underline}
 form,fieldset,label{display:grid;gap:.5rem}
 form{margin-top:.6rem}
 fieldset{border:1px solid var(--line);border-radius:4px;min-width:0}
 input,select,textarea,button{box-sizing:border-box;min-width:0;min-height:44px;font:inherit;color:var(--text);background:var(--surface);border:1px solid var(--line-strong);border-radius:4px;padding:.4rem}
 button{justify-self:start;cursor:pointer;color:var(--forest)}
 button.primary{color:white;background:var(--forest);border-color:var(--forest)}
</style>
