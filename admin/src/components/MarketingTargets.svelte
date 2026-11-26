<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 export let targets=[];
 export let aiEnabled=false;
 export let pending=false;
 const change=createEventDispatcher();
 const base='/marketing/cursemark/targets';
 const policies={links:'Links',self_promotion:'Self-promotion',ai:'AI rules for publication',flair:'Flair',cadence:'Cadence'};
 const platforms={youtube:'YouTube',twitch:'Twitch',other:'Other'};
 let query='',filter='all',selection=0,readID=0,detail=null,detailLoading=false;
 let form=null,editing=null,busy=false,researching=0,error='',notice='';
 let importOpen=false,file,uploading=false,receipt='';
 $: terms=query.trim().toLowerCase().split(/\s+/);
 $: filtered=targets.filter(t=>(filter==='all'||filter==='destination'&&t.kind==='destination'||
   filter==='creator'&&t.kind==='creator'||t.kind==='creator'&&t.creator_platform===filter)&&
   terms.every(term=>[t.name,t.url,platforms[t.creator_platform]||''].some(text=>text?.toLowerCase().includes(term))));
 $: if(!selection&&targets.length&&!form)select(targets[0].id,false);
 function open(target=null){
  editing=target;
  form={kind:target?.kind||'destination',name:target?.name||'',url:target?.url||'',email:target?.creator?.email||'',
   platform:target?.creator?.platform||'',channels:target?.creator?.channel_urls.join('\n')||'',games:target?.creator?.games_covered.join('\n')||''};
  error='';notice='';
 }
 async function select(id,clear=true){
  const ticket=++readID;
  selection=id;detail=null;detailLoading=true;form=null;editing=null;
  if(clear){error='';notice='';}
  try{
   const data=await portalRequest(base+'/'+id,'GET');
   if(ticket===readID)detail=data;
  }catch(e){if(ticket===readID)error=e.message;}
  finally{if(ticket===readID)detailLoading=false;}
 }
 function publish(target){
  const {creator,research,...summary}=target;
  change('saved',summary);
 }
 async function save(){
  busy=true;error='';notice='';
  try{
   const body={kind:form.kind,name:form.name,version:editing?.version};
   if(form.kind==='creator'){
    body.email=form.email;body.platform=form.platform;
    body.channel_urls=form.channels.split(/\r?\n/).filter(line=>line.trim());
    body.games_covered=form.games.split(/\r?\n/).filter(line=>line.trim());
   }else body.url=form.url;
   const saved=await portalRequest(base+(editing?'/'+editing.id:''),editing?'PUT':'POST',body);
   ++readID;detailLoading=false;selection=saved.id;detail=saved;
   publish(saved);
   notice=editing?'Target saved.':'Target added.';
   form=null;editing=null;query='';filter='all';
  }catch(e){error=e.message;}
  finally{busy=false;}
 }
 async function research(target){
  researching=target.id;error='';notice='';
  try{
   const saved=await portalRequest(base+'/'+target.id+'/research','POST',{version:target.version});
   publish(saved);
   if(selection===target.id){detail=saved;notice='Research updated for '+target.name+'.';}
  }catch(e){if(selection===target.id)error=e.message;}
  finally{researching=0;}
 }
 async function upload(event){
  const uploadForm=event.currentTarget;
  uploading=true;receipt='';error='';
  try{
   const formData=new FormData();formData.set('csv',file);
   const response=await fetch('/api'+base+'/import-creators',{method:'POST',body:formData});
   const data=await response.json().catch(()=>({}));
   if(!response.ok)throw Error(data.error||'The creator import failed.');
   receipt='Added '+data.added+' creators. Skipped '+data.duplicates+' duplicates.';
   change('imported');filter='creator';query='';
   file=null;
   uploadForm.reset();
  }catch(e){error=e.message;}
  finally{uploading=false;}
 }
</script>

<section class="catalog" aria-label="Target catalog">
 <aside class="card list" aria-label="Targets list">
  <div class="buttons">
   <button type="button" disabled={busy} on:click={()=>open()}>New Target</button>
   <button type="button" class="secondary" aria-expanded={importOpen} on:click={()=>{importOpen=!importOpen;error='';}}>Import creators</button>
  </div>
  <div class="filters">
   <label>Search targets<input type="search" bind:value={query}/></label>
   <label>Target type<select bind:value={filter}><option value="all">All</option><option value="destination">Public destinations</option><option value="creator">All creators</option>
    <option value="youtube">YouTube</option><option value="twitch">Twitch</option><option value="other">Other creators</option></select></label>
  </div>
  <p class="count">{filtered.length} of {targets.length} targets</p>
  {#if pending}<p role="status">Loading targets…</p>{:else if !filtered.length}<p>No matching targets.</p>{/if}
  <div class="choices">
   {#each filtered as target (target.id)}
    <button type="button" disabled={busy} aria-pressed={selection===target.id&&!form} on:click={()=>select(target.id)}>
     <span>{target.name}</span><small>{target.kind==='creator'?platforms[target.creator_platform]||'Creator':'Public destination'}</small>
    </button>
   {/each}
  </div>
 </aside>
 <div class="content">
  {#if importOpen}
   <section class="card" aria-label="Private creator import">
    <h2>Import creator contacts</h2>
    <p>Upload a private CSV with the headers name, email, platform, channel_urls, games_covered. Use | between multiple channels or games. Duplicate addresses are skipped.</p>
    <form on:submit|preventDefault={upload}>
     <label>Creator CSV<input type="file" accept=".csv,text/csv" required on:change={event=>file=event.currentTarget.files[0]}/></label>
     <button type="submit" disabled={!file||uploading}>{uploading?'Importing…':'Import CSV'}</button>
    </form>
    {#if receipt}<p role="status">{receipt}</p>{/if}
   </section>
  {/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  {#if form}
   <form class="card" aria-label={editing?'Edit target':'New target'} on:submit|preventDefault={save}>
    <h2>{editing?'Edit target':'New target'}</h2>
    {#if !editing}<label>Target kind<select bind:value={form.kind}><option value="destination">Public destination</option><option value="creator">Creator contact</option></select></label>{/if}
    <label>Target name<input maxlength="160" bind:value={form.name} required/></label>
    {#if form.kind==='creator'}
     <label>Private contact email<input type="email" autocomplete="off" maxlength="254" bind:value={form.email} required/></label>
     <label>Creator platform<select required bind:value={form.platform}><option value="" disabled>Select a platform</option>
      {#each Object.entries(platforms) as [id,label]}<option value={id}>{label}</option>{/each}</select></label>
     <label>Public channel URLs (one per line)<textarea rows="3" bind:value={form.channels}></textarea></label>
     <label>Games covered (one per line)<textarea rows="3" bind:value={form.games}></textarea></label>
    {:else}
     <label>Public destination URL<input type="url" maxlength="1000" bind:value={form.url} required/></label>
    {/if}
    <div class="buttons"><button type="submit" disabled={busy}>{busy?'Saving…':'Save target'}</button><button type="button" class="secondary" disabled={busy} on:click={()=>{form=null;editing=null;error='';}}>Cancel</button></div>
   </form>
  {:else if detailLoading}<p role="status">Loading target…</p>
  {:else if detail}
   <article class="card" aria-label={detail.name}>
    <header>
     <h2>{detail.name}</h2>
     <div class="buttons">
      <button type="button" class="secondary" aria-label={'Edit '+detail.name} disabled={researching===detail.id} on:click={()=>open(detail)}>Edit</button>
      <button type="button" aria-label={'Update Research for '+detail.name}
       disabled={!aiEnabled||!!researching||(detail.kind==='creator'&&!detail.creator?.channel_urls.length)}
       on:click={()=>research(detail)}>{researching===detail.id?'Researching…':'Update Research'}</button>
     </div>
    </header>
    {#if detail.creator}
     <dl class="identity">
      <div><dt>Private contact email</dt><dd>{detail.creator.email}</dd></div>
      <div><dt>Creator platform</dt><dd>{platforms[detail.creator.platform]}</dd></div>
      <div><dt>Public channels</dt><dd>{#each detail.creator.channel_urls as link}<a href={link} rel="noreferrer" target="_blank">{link}</a>{:else}None saved{/each}</dd></div>
      <div><dt>Games covered</dt><dd>{detail.creator.games_covered.join(' · ')||'None saved'}</dd></div>
     </dl>
    {:else if detail.url}<a href={detail.url} rel="noreferrer" target="_blank">{detail.url}</a>{/if}
    {#if detail.research}
     {@const snapshot=detail.research}
     <h3>Research</h3>
     <p>{snapshot.hold?'Hold · ':''}Checked {new Date(detail.checked_at).toISOString().slice(0,10)}</p>
     {#if !detail.research_ready}<p>Update research for the current destination or channels before drafting.</p>{/if}
     {#if snapshot.hold}<p>{snapshot.hold_reason}</p>{/if}
     {#if snapshot.takeaway}<p>{snapshot.takeaway}</p>{/if}
     {#if detail.creator}
      {#if snapshot.games_covered?.length}<h3>Games found in public coverage</h3><p>{snapshot.games_covered.join(' · ')}</p>{/if}
     {:else}
      <h3>Official rules</h3>
      <dl>{#each Object.entries(policies) as [key,label]}<div><dt>{label}</dt><dd>{snapshot.rules[key]||'Unverified'}</dd></div>{/each}</dl>
      {#if snapshot.ai_restriction==='assisted_submissions'}<p>The sourced rules explicitly cover AI-assisted submissions.</p>
      {:else if snapshot.ai_restriction==='final_content'}<p>The sourced rule applies to the final public content.</p>{/if}
     {/if}
     {#if snapshot.sources.length}
      <ul>{#each snapshot.sources as source}<li><a href={source} rel="noreferrer" target="_blank">{source}</a></li>{/each}</ul>
     {:else}<p>No verified {detail.creator?'public channel':'official rules'} source was found.</p>{/if}
     <h3>{detail.creator?'Recent creator coverage':'Recent promotion examples'}</h3>
     {#if snapshot.examples.length}
      <ul class="examples">{#each snapshot.examples as example (example.url)}
       <li><a href={example.url} rel="noreferrer" target="_blank">{example.title}</a>
        <p>{example.date}{example.engagement?' · '+example.engagement:' · Engagement unavailable'}</p>
        {#if example.what_worked}<p>{example.what_worked}</p>{/if}
       </li>
      {/each}</ul>
     {:else}<p>No recent examples could be verified.</p>{/if}
    {:else}<p>No research saved.</p>{/if}
   </article>
  {:else if targets.length}<p>Select a target to see its details.</p>{/if}
 </div>
</section>

<style>
 .catalog{display:grid;grid-template-columns:minmax(270px,min(35%,365px)) minmax(0,1fr);gap:1rem;align-items:start;text-align:left}
 .card{background:var(--surface);border:1px solid var(--line);border-radius:6px;padding:.9rem;margin:0 0 .7rem;min-width:0;overflow-wrap:anywhere}
 .list{position:sticky;top:.6rem}
 h2,h3,p{margin:.4rem 0;text-align:left}
 h2{font-size:1.12rem}
 h3{font-size:.94rem;margin-top:1rem}
 p{font-size:.88rem;color:var(--text-muted)}
 .count{margin:.6rem 0 .3rem}
 header,.buttons{display:flex;flex-wrap:wrap;gap:.4rem;align-items:center}
 header{justify-content:space-between;margin-bottom:.55rem}
 .filters,form{display:grid;gap:.6rem}
 .filters{margin-top:.65rem}
 form button{justify-self:start}
 label{display:grid;gap:.25rem;font-size:.88rem;font-weight:600}
 input,textarea,select,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input,textarea,select{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 textarea{resize:vertical}
 button{border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .65rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 .choices{display:grid;gap:.2rem;overflow:auto;max-height:max(15rem,calc(100vh - 21rem));padding:3px}
 .choices button{display:flex;flex-direction:column;align-items:flex-start;text-align:left;background:var(--surface);color:var(--text);border-color:transparent;overflow-wrap:anywhere}
 .choices button[aria-pressed=true],.choices button:hover{background:var(--paper-warm);border-color:var(--line-strong)}
 small{color:var(--text-muted);font-size:.75rem;font-weight:400}
 a{color:var(--forest);text-decoration:underline}
 .identity a{display:block}
 dl{margin:.4rem 0;font-size:.88rem}
 dl div{margin:.35rem 0}
 dt{font-weight:650}
 dd{margin:0;color:var(--text-muted)}
 ul{padding-left:1.25rem}
 .examples li{margin:.55rem 0}
 .error{color:var(--rust)}
 @media(max-width:850px){.catalog{grid-template-columns:1fr;gap:.6rem}.list{position:static;margin-bottom:0}.choices{max-height:11rem}}
</style>
