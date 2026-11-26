<script>
 import {onMount,tick} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 import MarketingBeatDialog from '../components/MarketingBeatDialog.svelte';
 import MarketingPlacement from '../components/MarketingPlacement.svelte';
 import MarketingResults from '../components/MarketingResults.svelte';
 import MarketingTargets from '../components/MarketingTargets.svelte';

 const base='/marketing/cursemark';
 const tabs=[{id:'beats',label:'Beats'},{id:'targets',label:'Targets'},{id:'imports',label:'Steamworks Imports'}];
 const ctaLabels={wishlist:'Wishlist',purchase:'Purchase'};
 const today=new Date().toISOString().slice(0,10);
 const tabButtons=[];
 let section='beats',beats=[],reports=[],posts=[],result=null;
 let selected=0,requestID=0,aiEnabled=false,loading=true,detailLoading=false,generating=false;
 let targets=[],linked=[],targetsLoading=true,targetsError='',updatingTarget=false;
 let error='',notice='',editor=null,saving=false,editorError='',detailHeading;
 let start='',end='',confirmed=false,file,importing=false,importMessage='',importError='',reportsLoaded=false,reportsLoading=false;
 $: beat=beats.find(item=>item.id===selected);
 $: attached=targets.filter(target=>linked.includes(target.id));
 $: available=targets.filter(target=>!linked.includes(target.id));

 onMount(async()=>{
  loadTargets();
  try{
   const plan=await portalRequest(base+'/beats','GET');
   beats=plan.beats;aiEnabled=plan.ai_enabled;
   if(beats.length)await choose(beats[0].id);
  }catch(e){error=e.message;}
  finally{loading=false;}
 });

 async function loadTargets(){
  targetsLoading=true;targetsError='';
  try{targets=(await portalRequest(base+'/targets','GET')).targets;}
  catch(e){targetsError=e.message;}
  finally{targetsLoading=false;}
 }

 function savedTarget(event){
  const saved=event.detail;
  targets=[...targets.filter(target=>target.id!==saved.id),saved].sort((a,b)=>a.name.localeCompare(b.name));
 }

 function selectTab(id){
  section=id;
  if(id==='imports'&&!reportsLoaded&&!reportsLoading)loadReports();
 }

 function navigateTabs(event,index){
  let next;
  switch(event.key){
   case 'ArrowLeft': next=(index+tabs.length-1)%tabs.length;break;
   case 'ArrowRight': next=(index+1)%tabs.length;break;
   case 'Home': next=0;break;
   case 'End': next=tabs.length-1;break;
   default:return;
  }
  event.preventDefault();
  selectTab(tabs[next].id);
  tabButtons[next].focus();
 }

 async function choose(id){
  const ticket=++requestID;
  selected=id;posts=[];result=null;linked=[];notice='';error='';detailLoading=true;
  await refreshBeat(id,ticket);
  if(ticket===requestID)detailLoading=false;
 }

 async function refreshBeat(id=selected,ticket=requestID){
  try{
   const detail=await portalRequest(base+'/beats/'+id,'GET');
   if(ticket!==requestID)return;
   beats=beats.map(item=>item.id===id?detail.beat:item);
   posts=detail.placements;result=detail.result;linked=detail.target_ids??[];
  }catch(e){if(ticket===requestID)error=e.message;}
 }

 function openEditor(current=null){
  editor={current};editorError='';
 }

 async function saveBeat(event){
  const current=editor.current;
  saving=true;editorError='';
  try{
   const saved=await portalRequest(base+'/beats'+(current?'/'+current.id:''),current?'PUT':'POST',
    {...event.detail,version:current?.version});
   editor=null;
   if(current){
    beats=beats.map(item=>item.id===current.id?saved:item);
   }else{
    beats=[saved,...beats];
    await choose(saved.id);
   }
   notice=current?'Beat saved.':'Beat created.';
   await tick();
   detailHeading?.focus();
  }catch(e){editorError=e.message;}
  finally{saving=false;}
 }

 async function setTarget(target, link){
  const ticket=requestID,id=selected;
  updatingTarget=true;error='';
  try{
   await portalRequest(base+'/beats/'+id+'/targets/'+target.id,link?'PUT':'DELETE');
   if(ticket===requestID){
    linked=link?[...linked,target.id]:linked.filter(value=>value!==target.id);
   }
  }catch(e){if(ticket===requestID)error=e.message;}
  finally{updatingTarget=false;}
 }

 async function generate(target){
  const ticket=requestID,id=selected;
  generating=target.id;notice='';error='';
  try{
   await portalRequest(base+'/beats/'+id+'/targets/'+target.id+'/generate','POST',{version:beat.version});
   if(ticket!==requestID)return;
   await refreshBeat(id,ticket);
   notice='Draft saved for '+target.name+'.';
  }catch(e){if(ticket===requestID)error=e.message;}
  finally{generating=false;}
 }

 function placementSaved(event){
  if(selected===event.detail.beat_id)refreshBeat();
 }

 async function loadReports(){
  reportsLoading=true;importError='';
  try{
   const history=await portalRequest(base+'/reports','GET');
   reports=history.reports;reportsLoaded=true;
  }catch(e){importError=e.message;}
  finally{reportsLoading=false;}
 }

 async function upload(){
  if(!file||!confirmed)return;
  importing=true;importError='';importMessage='';
  const body=new FormData();
  body.set('csv',file);
  body.set('start_date',start);body.set('end_date',end);body.set('full_breakdown','yes');
  try{
   const response=await fetch('/api'+base+'/reports',{method:'POST',body});
   const data=await response.json().catch(()=>({}));
   if(!response.ok)throw Error(data.error||'The import failed. Check the file size and try again.');
   importMessage=data.duplicate?'This file was already imported. Current results were kept.':('Imported '+data.matched+' matching placements.');
   await loadReports();
   if(selected)await refreshBeat();
  }catch(e){importError=e.message;}
  finally{importing=false;}
 }
</script>

<section class="marketing" aria-label="Cursemark marketing">
 <div class="subtabs" role="tablist" aria-label="Marketing sections">
  {#each tabs as tab,index (tab.id)}
   <button type="button" role="tab" id={'marketing-tab-'+tab.id} aria-selected={section===tab.id}
    aria-controls={'marketing-panel-'+tab.id} tabindex={section===tab.id?0:-1} bind:this={tabButtons[index]}
    on:click={()=>selectTab(tab.id)} on:keydown={event=>navigateTabs(event,index)}>{tab.label}</button>
  {/each}
 </div>

 <div role="tabpanel" id="marketing-panel-beats" aria-labelledby="marketing-tab-beats" hidden={section!=='beats'}>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}<p role="status">Loading Beats…</p>{:else}
   <div class="work">
    <aside class="card beat-list" aria-label="Beats">
     <button type="button" on:click={()=>openEditor()}>New Beat</button>
     <div class="beat-choices">
      {#each beats as item (item.id)}
       <button type="button" aria-pressed={item.id===selected} on:click={()=>choose(item.id)}>{item.name}</button>
      {/each}
     </div>
    </aside>
    <div class="detail">
     {#if detailLoading}<p role="status">Loading Beat…</p>
     {:else if beat}
      <section class="card" aria-label="Beat brief">
       <div class="beat-title">
        <h2 tabindex="-1" bind:this={detailHeading}>{beat.name}</h2>
        <button type="button" class="secondary" disabled={generating} on:click={()=>openEditor(beat)}>Edit Beat</button>
       </div>
       {#if beat.brief}<p class="brief">{beat.brief}</p>{/if}
       <p class="cta"><strong>Call to Action</strong> {ctaLabels[beat.call_to_action]||'Choose Wishlist or Purchase'}</p>
       {#if !aiEnabled}<p>OpenAI research and drafting require a server-side key.</p>
       {:else if !beat.call_to_action||!beat.brief.trim()}<p>Complete the brief and call to action to draft posts.</p>{/if}
       {#if notice}<p role="status">{notice}</p>{/if}
      </section>
      <section class="card" aria-label="Beat targets">
       <div class="beat-title"><h3>Targets</h3><button type="button" class="secondary" on:click={()=>selectTab('targets')}>Manage targets</button></div>
       {#if targetsError}<p class="error" role="alert">{targetsError} <button type="button" class="secondary" on:click={loadTargets}>Retry</button></p>
       {:else if !targetsLoading&&!targets.length}<p>Add destinations in Targets to draft posts for this Beat.</p>{/if}
       <div class="pills" aria-label="Available targets">
        {#each available as target (target.id)}
         <button type="button" class="pill" aria-label={'Attach '+target.name} disabled={updatingTarget} on:click={()=>setTarget(target,true)}>+ {target.name}</button>
        {/each}
       </div>
       {#each attached as target (target.id)}
        {@const current=target.research&&target.research_url===target.url}
        <div class="target-row">
         <div class="attached-label"><span>{target.name}</span><button type="button" aria-label={'Remove '+target.name+' from Beat'} disabled={updatingTarget} on:click={()=>setTarget(target,false)}>×</button></div>
         <small>{!current?'Research needed':target.research.hold?'Hold':'Research saved'}</small>
         <button type="button" class="secondary" aria-label={'Generate Post for '+target.name} disabled={!aiEnabled||!!generating||!current||!beat.call_to_action||!beat.brief.trim()} on:click={()=>generate(target)}>{generating===target.id?'Drafting…':'Generate Post'}</button>
        </div>
       {/each}
      </section>
      <section class="card" aria-label="Beat results">
       <h3>Beat results</h3>
       <MarketingResults {result}/>
      </section>
      <section aria-label="Placements">
       <h3>Placements</h3>
       {#each posts as post (post.id)}<MarketingPlacement beatID={selected} {post} on:saved={placementSaved}/>{/each}
       {#if !posts.length}<p>No placements yet.</p>{/if}
       {#key selected}<MarketingPlacement beatID={selected} on:saved={placementSaved}/>{/key}
      </section>
     {:else}<p>No Beats yet.</p>{/if}
    </div>
   </div>
  {/if}
 </div>

 <div role="tabpanel" id="marketing-panel-targets" aria-labelledby="marketing-tab-targets" hidden={section!=='targets'}>
  {#if targetsError}<p class="error" role="alert">{targetsError} <button type="button" class="secondary" on:click={loadTargets}>Retry</button></p>{/if}
  {#if !aiEnabled&&!loading}<p>OpenAI research requires a server-side key.</p>{/if}
  <MarketingTargets {targets} {aiEnabled} pending={targetsLoading} on:saved={savedTarget}/>
 </div>

 <div role="tabpanel" id="marketing-panel-imports" aria-labelledby="marketing-tab-imports" hidden={section!=='imports'}>
  <section class="card imports" aria-label="Steamworks upload">
   <h2>Import the Cursemark UTM breakdown</h2>
   <p>Export the full English breakdown for app 3219180 and enter its inclusive dates. A new file for the same dates refreshes that range. Choose non-overlapping dates for additional reports.</p>
   <form on:submit|preventDefault={upload}>
    <div class="fields">
     <label>Start date<input type="date" bind:value={start} required max={today}/></label>
     <label>End date<input type="date" bind:value={end} required min={start||undefined} max={today}/></label>
    </div>
    <label>Steamworks CSV<input type="file" accept=".csv,text/csv" required on:change={event=>{file=event.currentTarget.files[0];}}/></label>
    <label class="confirm"><input type="checkbox" bind:checked={confirmed}/> This is the full, unfiltered Cursemark breakdown for these dates.</label>
    <button type="submit" disabled={importing||!file||!confirmed}>{importing?'Importing…':'Import CSV'}</button>
   </form>
   {#if importMessage}<p role="status">{importMessage}</p>{/if}
   {#if importError}<p class="error" role="alert">{importError}</p>{/if}
   {#if reportsLoading}<p role="status">Loading imports…</p>
   {:else if !reportsLoaded}<button type="button" class="secondary" on:click={loadReports}>Retry loading imports</button>
   {:else if reports.length}
    <h3>Imported ranges</h3>
    <ul>{#each reports as report (report.id)}<li>{report.start_date} through {report.end_date} · updated {new Date(report.imported_at).toLocaleDateString()}</li>{/each}</ul>
   {:else}<p>No reports imported yet.</p>{/if}
  </section>
 </div>
</section>

{#if editor}
 <MarketingBeatDialog beat={editor.current} busy={saving} error={editorError} on:save={saveBeat} on:dismiss={()=>editor=null}/>
{/if}

<style>
 .marketing{max-width:1368px;margin:12px auto 32px;padding:0 16px;text-align:left;color:var(--text)}
 p{text-align:left;color:var(--text-muted);margin:.55rem 0;line-height:1.45}
 h2,h3{margin:0 0 .55rem;text-align:left;font-size:1rem;overflow-wrap:anywhere}
 .subtabs{display:flex;gap:6px;margin-bottom:14px;flex-wrap:wrap}
 .subtabs button{background:var(--surface);border:1px solid var(--line);border-radius:4px;padding:8px 12px;color:var(--text);font-size:.8rem;font-weight:400}
 .subtabs button[aria-selected=true]{color:var(--surface);background:var(--forest);border-color:var(--forest)}
 .work{display:grid;grid-template-columns:minmax(280px,min(34%,365px)) minmax(0,1fr);gap:1rem;align-items:start}
 .card{background:var(--surface);border:1px solid var(--line);border-radius:7px;padding:1rem;margin:0 0 1rem;min-width:0}
 .beat-list{position:sticky;top:.6rem}
 .beat-list>button{width:100%;margin-bottom:.55rem}
 .beat-choices{display:grid;gap:.25rem;max-height:max(16rem,calc(100vh - 15rem));overflow:auto;scrollbar-gutter:stable;padding:3px}
 .beat-choices button{width:100%;min-height:38px;text-align:left;overflow-wrap:anywhere;color:var(--text);background:var(--surface);border-color:transparent;font-weight:500}
 .beat-choices button:hover{background:var(--paper-warm)}
 .beat-choices button[aria-pressed=true]{background:var(--paper-warm);border-color:var(--forest)}
 .beat-title{display:flex;justify-content:space-between;align-items:center;gap:.65rem}
 .beat-title h2{margin:0;font-size:1.15rem}
 .beat-title button{flex:none}
 .brief{white-space:pre-wrap;overflow-wrap:anywhere}
 .cta{margin-bottom:.8rem}
 .cta strong{margin-right:.25rem}
 .pills{display:flex;gap:.35rem;flex-wrap:wrap;margin:.55rem 0}
 .pill,.attached-label{border-radius:999px;background:var(--paper-warm);border:1px solid var(--line);color:var(--forest)}
 .target-row{display:flex;gap:.5rem;align-items:center;flex-wrap:wrap;padding:.4rem 0}
 .attached-label{display:inline-flex;align-items:center;overflow-wrap:anywhere;padding-left:.7rem}
 .attached-label button{border:0;background:transparent;color:var(--forest);width:44px;flex:none;padding:0;font-size:1.2rem}
 small{color:var(--text-muted)}
 .imports{max-width:760px}
 .imports h3{margin-top:1rem}
 .imports ul{padding-left:1.25rem}
 form{display:grid;gap:.65rem;margin-top:.75rem}
 .fields{display:grid;grid-template-columns:1fr 1fr;gap:.65rem}
 label{display:grid;min-width:0;gap:.25rem;font-size:.9rem;font-weight:600}
 input,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 button{border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .75rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 .confirm{display:flex;align-items:center;gap:.5rem}
 .confirm input{width:auto}
 .imports button{justify-self:start}
 .error{color:var(--rust)}
 @media(max-width:860px){.work{grid-template-columns:1fr;gap:.65rem}.beat-list{position:static;margin-bottom:0}.beat-list>button{width:auto}.beat-choices{max-height:9rem}.beat-choices button{min-height:44px}}
 @media(max-width:520px){.fields{grid-template-columns:1fr}.marketing{padding:0 8px}.card{padding:.7rem}}
</style>
