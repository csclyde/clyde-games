<script>
 import {createEventDispatcher} from 'svelte';
 export let project=null;
 const dispatch=createEventDispatcher();
 let loaded=null,name='',steamAppId='',steamSyncEnabled=false,discordSources=[],catalog=[],catalogError='',saving=false,error='';
 $: if(!project){loaded=null}else if(loaded!==project.id){loaded=project.id;name=project.name;steamAppId=project.steamAppId||'';steamSyncEnabled=Boolean(project.steamSyncEnabled);discordSources=(project.discordSources||[]).map(s=>({...s}));error='';loadCatalog(project.id)}
 async function loadCatalog(id){catalogError='';try{const r=await fetch('/api/projects/discord-sources',{cache:'no-store'});const b=await r.json();if(!r.ok)throw new Error(b.error||'Discord sources unavailable');if(project?.id===id)catalog=b.items}catch(e){if(project?.id===id)catalogError=e.message}}
 function close(){if(!saving)dispatch('close')}
 function chooseSource(index,id){const item=catalog.find(s=>s.id===id);if(item){discordSources=discordSources.map((s,i)=>i===index?{...item,enabled:s.enabled}:s)}}
 function addSource(){const next=catalog.find(s=>s.canRead&&s.canReact&&!discordSources.some(d=>d.id===s.id));if(next)discordSources=[...discordSources,{...next,enabled:true}]}
 async function save(){if(!name.trim())return;saving=true;error='';try{const r=await fetch(`/api/projects/${encodeURIComponent(project.id)}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:name.trim(),steamAppId:steamAppId.trim(),steamSyncEnabled,discordSources:discordSources.map(({id,kind,enabled})=>({id,kind,enabled}))})});const b=await r.json();if(!r.ok)throw new Error(b.error||'Unable to update project');dispatch('saved',b);dispatch('close')}catch(e){error=e.message}finally{saving=false}}
</script>
{#if project}
 <!-- svelte-ignore a11y-click-events-have-key-events -->
 <div class="backdrop" role="presentation" on:click|self={close}>
  <form class="modal" role="dialog" aria-modal="true" aria-labelledby="project-modal-title" on:submit|preventDefault={save}>
   <h3 id="project-modal-title">Edit project</h3>
   <fieldset disabled={saving}><legend>Project Name</legend><label for="project-name">Name</label><input id="project-name" bind:value={name} maxlength="255" required /></fieldset>
   <fieldset disabled={saving}><legend>Feedback Syncs</legend>
    <label class="toggle"><input type="checkbox" bind:checked={steamSyncEnabled}/>Steam</label><label for="steam-app-id">Steam app ID</label><input id="steam-app-id" bind:value={steamAppId} inputmode="numeric" maxlength="32" required={steamSyncEnabled}/>
    <h4>Discord</h4>
    {#each discordSources as source,index}
     <div class="discord-source">
      <label class="toggle"><input type="checkbox" bind:checked={source.enabled}/><span>{source.kind==='discord_forum'?'Forum':'Channel'}</span></label>
      <select aria-label={`Discord source ${index+1}`} value={source.id} on:change={e=>chooseSource(index,e.currentTarget.value)}>
       {#if !catalog.some(c=>c.id===source.id)}<option value={source.id}>{source.name||source.id}</option>{/if}
       {#each catalog as item}<option value={item.id} disabled={item.id!==source.id&&discordSources.some(s=>s.id===item.id)}>{item.kind==='discord_forum'?'Forum':'Channel'}: {item.name} ({item.id})</option>{/each}
      </select><button type="button" aria-label={`Remove Discord source ${index+1}`} on:click={()=>discordSources=discordSources.filter((_,i)=>i!==index)}>Remove</button>
     </div>
    {/each}
    <button type="button" on:click={addSource} disabled={!catalog.some(c=>c.canRead&&c.canReact&&!discordSources.some(s=>s.id===c.id))}>Add Discord source</button>
    {#if catalogError}<p class="error">{catalogError}</p>{/if}
   </fieldset>
   {#if error}<p class="error" role="alert">{error}</p>{/if}
   <div class="actions"><button type="button" on:click={close} disabled={saving}>Cancel</button><button class="primary" type="submit" disabled={saving||!name.trim()}>{saving?'Saving…':'Save changes'}</button></div>
  </form>
 </div>
{/if}
<style>
 .backdrop{position:fixed;inset:0;z-index:1000;display:flex;align-items:center;justify-content:center;background:rgba(20,18,14,.58);padding:16px}.modal{box-sizing:border-box;width:100%;max-width:620px;max-height:calc(100dvh - 32px);overflow:auto;background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:22px;box-shadow:0 14px 40px #0004}h3{margin:0 0 18px}fieldset{min-width:0;border:0;padding:0;margin:0 0 22px}legend{font-weight:800;margin-bottom:12px}label{display:block;font-size:.8rem;font-weight:700;margin:8px 0 5px}input,select{box-sizing:border-box;width:100%;min-width:0;padding:9px;border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);color:var(--text);font:inherit}input:focus,select:focus{outline:2px solid var(--forest)}.toggle{display:flex;align-items:center;gap:8px;margin-top:16px}.toggle input{width:auto}h4{margin:20px 0 8px}.discord-source{display:grid;grid-template-columns:90px minmax(0,1fr) auto;gap:8px;align-items:center;margin-bottom:12px}.discord-source .toggle{margin:0}.actions{display:flex;justify-content:flex-end;gap:8px}.error{color:var(--rust);font-size:.84rem;overflow-wrap:anywhere}button{padding:8px 11px;border:1px solid var(--line);border-radius:4px;background:var(--paper-warm);color:var(--text);font:inherit;cursor:pointer;font-size:.8rem}button.primary{background:var(--forest);color:white}button:disabled{opacity:.6;cursor:default}@media(max-width:550px){.modal{padding:16px}.discord-source{grid-template-columns:minmax(0,1fr) auto}.discord-source .toggle{grid-column:1/-1}}
</style>
