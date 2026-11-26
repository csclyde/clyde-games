<script>
 import {tick} from 'svelte';
 import {marketingComposer,marketingHandoff,targetRecipient} from '../marketing-targets.mjs';
 export let target;
 export let title='';
 export let body='';
 export let steamURL='';
 export let disabled=false;
 let feedback='',manual=null,manualField;
 const composerLabels={gmail:'Open Gmail',reddit:'Open Reddit composer',x:'Open X composer'};
 $: needsSteam=body.includes('{steam_link}')&&!steamURL;
 $: resolvedBody=steamURL?body.replaceAll('{steam_link}',steamURL):body;
 $: recipient=target.kind==='creator'?targetRecipient(target.creator?.email):'';
 $: composer=composerLabels[marketingComposer(target)];

 function openComposer(){
  if(disabled||needsSteam)return;
  const handoff=marketingHandoff(target,title,resolvedBody);
  if(!handoff){
   feedback='This target has no valid composer link. Copy your draft instead.';
   return;
  }
  // The URL and its private fields exist only in this explicit client-side handoff.
  window.open(handoff.url,'_blank','noopener,noreferrer');
  manual=null;
  feedback=handoff.fallback||(recipient?'If Gmail does not fill a field, copy it below.':'');
 }

 async function copy(label,text){
  if(!text)return;
  try{
   await navigator.clipboard.writeText(text);
   feedback=label+' copied.';
   manual=null;
  }catch{
   manual={label,text};
   feedback='Copy the selected text with your keyboard or touch controls.';
   await tick();
   manualField?.focus();
   manualField?.select();
  }
 }
</script>

<div class="handoff" aria-label="Manual handoff">
 <div class="actions">
  {#if composer}
   <button type="button" on:click={openComposer} disabled={disabled||needsSteam}>{composer}</button>
  {/if}
  <button type="button" class="secondary" on:click={()=>copy('Title',title)} disabled={!title}>Copy title</button>
  <button type="button" class="secondary" on:click={()=>copy('Body',resolvedBody)} disabled={!body||needsSteam}>Copy body</button>
  {#if recipient}<button type="button" class="secondary" on:click={()=>copy('Recipient',recipient)}>Copy recipient</button>{/if}
 </div>
 {#if needsSteam}<p>Save the draft to insert its Steam tracking link.</p>
 {:else if !composer&&target.kind==='creator'&&!disabled}<p>No usable contact email saved. Use the public channel and copy the draft.</p>{/if}
 {#if feedback}<p role="status">{feedback}</p>{/if}
 {#if manual}
  <label>{manual.label} to copy<textarea rows="3" readonly value={manual.text} bind:this={manualField}></textarea></label>
  <button type="button" class="secondary" on:click={()=>{manual=null;feedback='';}}>Done copying</button>
 {/if}
</div>

<style>
 .handoff{display:grid;gap:.35rem}
 .actions{display:flex;flex-wrap:wrap;gap:.35rem}
 button,textarea{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 button{border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.38rem .6rem;cursor:pointer;font-weight:650;font-size:.85rem;justify-self:start}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 label{display:grid;gap:.25rem;font-size:.88rem;font-weight:600}
 textarea{width:100%;resize:vertical;color:var(--text);background:var(--surface);border:1px solid var(--line-strong);border-radius:4px;padding:.4rem}
 p{margin:0;color:var(--text-muted);font-size:.85rem;text-align:left}
</style>
