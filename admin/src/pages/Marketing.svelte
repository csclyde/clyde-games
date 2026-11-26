<script>
 import {onMount} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 import MarketingPlacement from '../components/MarketingPlacement.svelte';
 import MarketingResults from '../components/MarketingResults.svelte';

 const base='/marketing/cursemark';
 const goals={traffic:'Tracked Steam visits',wishlists:'Wishlists',purchases:'Purchases',activations:'Activations',demo:'Demo interest',awareness:'Awareness'};
 const measured=['traffic','wishlists','purchases','activations'];
 const emptyBeat=()=>({name:'',asset_kind:'video',asset:'',brief:'',hook:'',why_now:'',cta:'',goal:'wishlists',goal_target:0});
 let beats=[],reports=[],posts=[],draft=emptyBeat(),result=null;
 let aiEnabled=false,loading=true,busy=false,generating=false,selected=0,requestID=0;
 let notice='',error='',showImport=false,importing=false,importMessage='',importError='';
 let start='',end='',confirmed=false,file;
 const today=new Date().toISOString().slice(0,10);
 $: savedBeat=beats.find(beat=>beat.id===selected);
 $: modified=savedBeat&&Object.keys(emptyBeat()).some(key=>draft[key]!==savedBeat[key]);

 async function reloadOverview(){
  const [plan,history]=await Promise.all([portalRequest(base+'/beats','GET'),portalRequest(base+'/reports','GET')]);
  beats=plan.beats;
  aiEnabled=plan.ai_enabled;
  reports=history.reports;
 }

 onMount(async()=>{
  try{await reloadOverview();if(beats.length)await choose(beats[0].id);}
  catch(e){error=e.message;}
  finally{loading=false;}
 });

 async function choose(id){
  const ticket=++requestID;
  selected=id;
  draft=id?{...beats.find(beat=>beat.id===id)}:emptyBeat();
  posts=[];result=null;notice='';error='';
  if(!id)return;
  await refreshBeat(id,ticket,true);
 }

 async function refreshBeat(id=selected,ticket=requestID,reset=false){
  try{
   const detail=await portalRequest(base+'/beats/'+id,'GET');
   if(ticket!==requestID)return;
   if(reset)draft={...detail.beat};
   beats=beats.map(beat=>beat.id===id?detail.beat:beat);
   posts=detail.placements;
   result=detail.result;
  }catch(e){if(ticket===requestID)error=e.message;}
 }

 async function saveBeat(){
  const ticket=requestID,id=selected;
  busy=true;notice='';error='';
  try{
   const body={...draft,goal_target:measured.includes(draft.goal)?(draft.goal_target||0):0};
   const saved=await portalRequest(base+'/beats'+(id?'/'+id:''),id?'PUT':'POST',body);
   beats=id?beats.map(beat=>beat.id===id?saved:beat):[saved,...beats];
   if(ticket!==requestID)return;
   draft={...saved};
   selected=saved.id;
   await refreshBeat(saved.id,ticket);
   notice='Beat saved.';
  }catch(e){if(ticket===requestID)error=e.message;}
  finally{busy=false;}
 }

 async function generate(){
  const ticket=requestID,id=selected;
  generating=true;notice='';error='';
  try{
   const data=await portalRequest(base+'/beats/'+id+'/generate','POST',{version:draft.version});
   if(ticket!==requestID)return;
   await refreshBeat(id,ticket);
   notice=data.placements.length?('Saved '+data.placements.length+' suggested placements. Review each one before posting.'):'No useful opportunities were found.';
  }catch(e){if(ticket===requestID)error=e.message;}
  finally{generating=false;}
 }

 function placementSaved(event){
  const post=event.detail;
  if(selected!==post.beat_id)return;
  refreshBeat();
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
   const history=await portalRequest(base+'/reports','GET');
   reports=history.reports;
   if(selected)await refreshBeat();
  }catch(e){importError=e.message;}
  finally{importing=false;}
 }
</script>

<section class="marketing" aria-label="Cursemark marketing">
 <header class="heading">
  <div><h2>Beats</h2><p>Plan one asset at a time and track the posts you publish.</p></div>
  <button type="button" class="secondary" aria-expanded={showImport} aria-controls="steamworks-import" on:click={()=>showImport=!showImport}>Steamworks imports</button>
 </header>
 {#if showImport}
  <section class="card imports" id="steamworks-import" aria-label="Steamworks imports">
   <h3>Import the Cursemark UTM breakdown</h3>
   <p>Export the full breakdown in English for app 3219180. Enter the export’s inclusive dates. A new file for the same range refreshes that range. Other ranges must be disjoint.</p>
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
   {#if reports.length}
    <details><summary>{reports.length} imported {reports.length===1?'range':'ranges'}</summary>
     <ul>{#each reports as report (report.id)}<li>{report.start_date} through {report.end_date} · updated {new Date(report.imported_at).toLocaleDateString()}</li>{/each}</ul>
    </details>
   {/if}
  </section>
 {/if}
 {#if loading}<p role="status">Loading Beats…</p>{/if}
 {#if error}<p class="error" role="alert">{error}</p>{/if}
 {#if !loading}
  <div class="work">
   <aside class="card beat-list" aria-label="Beats">
    <button type="button" class="new" aria-pressed={!selected} disabled={busy} on:click={()=>choose(0)}>New Beat</button>
    {#each beats as beat (beat.id)}
     <button type="button" class:chosen={beat.id===selected} aria-pressed={beat.id===selected} disabled={busy} on:click={()=>choose(beat.id)}>
      <strong>{beat.name}</strong><small>{beat.asset_kind} · {goals[beat.goal]}</small>
     </button>
    {/each}
   </aside>
   <div class="detail">
    <section class="card" aria-label="Beat brief">
     <h3>{selected?'Beat brief':'New Beat'}</h3>
     <form on:submit|preventDefault={saveBeat}>
      <label>Beat name<input maxlength="160" bind:value={draft.name} required/></label>
      <div class="fields">
       <label>Asset type<select bind:value={draft.asset_kind}><option>video</option><option>image</option><option value="gif">GIF</option><option>trailer</option><option>other</option></select></label>
       <label>Asset link or filename<input maxlength="1000" bind:value={draft.asset} required/></label>
      </div>
      <label>Brief<textarea rows="3" maxlength="3000" bind:value={draft.brief}></textarea></label>
      <div class="fields">
       <label>Hook<textarea rows="2" maxlength="1000" bind:value={draft.hook}></textarea></label>
       <label>Why now<textarea rows="2" maxlength="1000" bind:value={draft.why_now}></textarea></label>
      </div>
      <label>Call to action<input maxlength="1000" bind:value={draft.cta}/></label>
      <div class="fields">
       <label>Goal<select bind:value={draft.goal}>{#each Object.entries(goals) as [id,label]}<option value={id}>{label}</option>{/each}</select></label>
       {#if measured.includes(draft.goal)}<label>Goal target (optional)<input type="number" min="0" max="1000000" bind:value={draft.goal_target}/></label>{/if}
      </div>
      <div class="buttons">
       <button type="submit" disabled={busy||generating}>{busy?'Saving…':'Save Beat'}</button>
       {#if selected}<button type="button" class="secondary" disabled={!aiEnabled||busy||generating||modified} on:click={generate}>{generating?'Researching…':'Research and draft posts'}</button>{/if}
      </div>
     </form>
     {#if !aiEnabled}<p>OpenAI research and drafting require a server-side key.</p>{:else if modified}<p>Save the Beat to use these edits for research.</p>{:else if selected}<p>Research uses the Beat text and asset type. It appends editable drafts.</p>{/if}
     {#if notice}<p role="status">{notice}</p>{/if}
    </section>
    {#if selected}
     <section class="card" aria-label="Beat results">
      <h3>Beat results</h3>
      <MarketingResults {result}/>
      {#if result?.score!=null}<p><strong>{Math.round(result.score)}%</strong> of the {goals[draft.goal].toLowerCase()} target</p>
      {:else if measured.includes(draft.goal)&&draft.goal_target>0}<p>Scoring requires mature Steamworks results for the reported ranges and at least 20 tracked visits.</p>
      {:else if draft.goal==='demo'}<p>Steamworks base-game UTM reports don’t attribute demo downloads.</p>{/if}
     </section>
     <section aria-label="Placements">
      <h3>Placements</h3>
      {#each posts as post (post.id)}<MarketingPlacement beatID={selected} {post} on:saved={placementSaved}/>{/each}
      {#if !posts.length}<p>No placements yet. Research suggestions or add a draft.</p>{/if}
      {#key selected}<MarketingPlacement beatID={selected} on:saved={placementSaved}/>{/key}
     </section>
    {/if}
   </div>
  </div>
 {/if}
</section>

<style>
 .marketing{max-width:1260px;margin:auto;padding:1rem;text-align:left;color:var(--text)}
 .marketing p{text-align:left;color:var(--text-muted);margin:.55rem 0;line-height:1.45}
 h2,h3{margin:0 0 .55rem;text-align:left}
 .heading,.buttons{display:flex;flex-wrap:wrap;gap:.65rem;align-items:center}
 .heading{justify-content:space-between;margin-bottom:1rem}
 .work{display:grid;grid-template-columns:minmax(150px,230px) minmax(0,1fr);gap:1rem;align-items:start}
 .card{background:var(--surface);border:1px solid var(--line);border-radius:7px;padding:1rem;margin:0 0 1rem;min-width:0}
 .beat-list button{display:block;width:100%;text-align:left;margin:0 0 .45rem;overflow-wrap:anywhere;color:var(--text);background:var(--surface);border-color:var(--line)}
 .beat-list small{display:block;font-weight:normal;margin-top:.15rem}
 .beat-list .chosen,.beat-list .new[aria-pressed=true]{background:var(--paper-warm);border-color:var(--forest)}
 form{display:grid;gap:.65rem}
 .fields{display:grid;grid-template-columns:1fr 1fr;gap:.65rem}
 label{display:grid;min-width:0;gap:.25rem;font-size:.9rem;font-weight:600}
 input,select,textarea,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input,select,textarea{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 textarea{resize:vertical}
 button{border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .75rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 .confirm{display:flex;align-items:center;gap:.5rem}
 .confirm input{width:auto}
 .imports button{justify-self:start}
 .marketing .error{color:#9d302b}
 @media(max-width:860px){.work{grid-template-columns:1fr}.beat-list{display:flex;gap:.4rem;overflow:auto}.beat-list button{width:auto;flex:none;max-width:210px;margin:0}}
 @media(max-width:490px){.fields{grid-template-columns:1fr}.marketing,.card{padding:.7rem}}
</style>
