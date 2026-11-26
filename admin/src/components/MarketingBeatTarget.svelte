<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 import {publicMarketingURL,targetPlatform} from '../marketing-targets.mjs';
 import MarketingTargetBadges from './MarketingTargetBadges.svelte';
 import MarketingPlacement from './MarketingPlacement.svelte';

 export let target;
 export let beatID;
 export let placements=[];
 export let busy=false;
 export let beatArchived=false;
 const dispatch=createEventDispatcher();
 let detail=null,loading=true,failed=false,adding=false,requestedVersion,ticket=0;
 let editing=false,saving=false,editors={},newEditor;
 $: subject=detail?{...target,...detail}:target;
 $: publicURL=publicMarketingURL(subject.url);
 $: ruleSource=publicMarketingURL(subject.research?.sources?.[0]);
 $: canAdd=!beatArchived&&!target.archived_at;
 $: editorID='beat-target-editor-'+beatID+'-'+target.id;
 $: preview=[...placements].sort((a,b)=>Number(!!a.published_at)-Number(!!b.published_at)||b.id-a.id)[0];

 $: if(editing&&target.version!==requestedVersion)load(target.id,target.version);

 async function load(id=target.id,version=target.version){
  const sequence=++ticket;
  requestedVersion=version;
  loading=true;failed=false;
  try{
   const value=await portalRequest('/marketing/cursemark/targets/'+id,'GET');
   if(sequence===ticket)detail=value;
  }catch{if(sequence===ticket)failed=true;}
  finally{if(sequence===ticket)loading=false;}
 }

 function saved(event){
  if(event.detail.id&&!placements.some(post=>post.id===event.detail.id))adding=false;
  dispatch('saved',event.detail);
 }

 async function toggleEdit(){
  if(!editing){
   requestedVersion=undefined;
   editing=true;
   return;
  }
  saving=true;
  try{
   for(const placement of placements){
    if(editors[placement.id]&&!(await editors[placement.id].savePending()))return;
   }
   if(newEditor&&!(await newEditor.savePending()))return;
   adding=false;
   editing=false;
  }finally{saving=false;}
 }
</script>

<article class="target" aria-label={target.name}>
 <header>
  <div>
   <h4>{target.name} <MarketingTargetBadges {target}/><small>{targetPlatform(target)}</small></h4>
   {#if editing}<p>
    {#if target.archived_at} · Archived
    {:else if !target.research_ready} · Research needed
    {:else if target.research_hold} · Hold{/if}
   </p>{/if}
  </div>
  <div class="actions">
   <button type="button" class="secondary" aria-expanded={editing} aria-controls={editorID} disabled={saving}
    aria-label={(editing?'Done editing ':'Edit ')+target.name} on:click={toggleEdit}>{saving?'Saving…':editing?'Done':'Edit'}</button>
  </div>
 </header>
 {#if !editing}
  {#if preview}
   <div class="preview">
    <p class="preview-title">{preview.title||'Untitled draft'}</p>
    {#if preview.copy}<p class="preview-body">{preview.copy}</p>{/if}
   </div>
  {:else}<p>No draft yet.</p>{/if}
 {/if}
 <div id={editorID} hidden={!editing} aria-busy={saving}>
 {#if editing}
 <div class="actions editor-actions">
  <button type="button" class="secondary" disabled={saving} on:click={()=>dispatch('view',target.id)}>Target details</button>
  <button type="button" class="secondary" aria-label={'Remove '+target.name+' from Beat'} disabled={busy||saving} on:click={()=>dispatch('remove',target)}>Remove target</button>
 </div>
 {#if subject.research_ready}
  {#if subject.research_hold&&subject.research?.hold_reason}<p class="note">{subject.research.hold_reason}</p>
  {:else if subject.research?.takeaway}<p class="note">{subject.research.takeaway}</p>{/if}
 {/if}
 {#if publicURL||ruleSource}
  <div class="actions">
   {#if publicURL}<a href={publicURL} target="_blank" rel="noreferrer">{target.kind==='creator'?'Public channel':'Open destination'}</a>{/if}
   {#if ruleSource}<a href={ruleSource} target="_blank" rel="noreferrer">{target.kind==='creator'?'Research source':'Rules and sources'}</a>{/if}
  </div>
 {/if}
 {#if failed}<p role="alert">Could not load target details. <button type="button" class="secondary" on:click={()=>load()}>Retry</button></p>{/if}
 {#each placements as post (post.id)}
  <MarketingPlacement {beatID} {post} target={subject} allowHandoff={canAdd&&!loading&&!failed} targetSaving={saving} bind:this={editors[post.id]} on:saved={saved}/>
 {/each}
 {#if canAdd&&(!placements.length||adding)}
  <MarketingPlacement {beatID} target={subject} allowHandoff={!loading&&!failed} targetSaving={saving} bind:this={newEditor} on:saved={saved}/>
  {#if adding}<button type="button" class="secondary more" disabled={saving} on:click={()=>adding=false}>Cancel new draft</button>{/if}
 {:else if canAdd}
  <button type="button" class="secondary more" disabled={saving} on:click={()=>adding=true}>Add another placement</button>
 {:else if !placements.length}
  <p>Restore the {beatArchived?'Beat':'target'} to add a draft.</p>
 {/if}
 {/if}
 </div>
</article>

<style>
 .target{padding:.55rem .75rem;background:var(--paper-warm);border:1px solid var(--line);border-radius:6px;min-width:0;overflow-wrap:anywhere}
 header{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:.25rem .7rem}
 h4{display:flex;align-items:baseline;flex-wrap:wrap;gap:.28rem .4rem;margin:0;color:var(--text);font-size:.98rem}
 p{margin:.3rem 0;color:var(--text-muted);text-align:left;font-size:.85rem}
 small{color:var(--text-muted);font-size:.78rem;font-weight:450}
 .preview-title,.preview-body{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
 .preview-title{font-weight:650;color:var(--text);margin:.15rem 0}
 .preview-body{margin:.12rem 0}
 .editor-actions{margin:.45rem 0}
 p.note{margin:.45rem 0}
 .actions{display:flex;flex-wrap:wrap;align-items:center;gap:.35rem .65rem}
 button{min-height:44px;box-sizing:border-box;min-width:0;font:inherit;font-size:.82rem;border:1px solid var(--line-strong);border-radius:5px;padding:.35rem .62rem;cursor:pointer}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 button.more{margin-top:.55rem}
 a{color:var(--forest);font-size:.85rem;text-decoration:underline}
</style>
