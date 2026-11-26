<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 import {publicMarketingURL,targetPlatform} from '../marketing-targets.mjs';
 import MarketingHandoff from './MarketingHandoff.svelte';
 import MarketingResults from './MarketingResults.svelte';

 export let beatID;
 export let post=null;
 export let target=null;
 export let allowHandoff=true;
 const dispatch=createEventDispatcher();
 const blank=()=>({platform:target?targetPlatform(target):'',community:target?.name||'',audience_fit:'',timing:'',title:'',copy:'',asset_notes:'',rule_evidence:'',rules_url:'',rules_checked:'',decision:'hold',published_url:'',published_note:''});
 const datestring=value=>{
  if(!value)return '';
  const d=new Date(value);
  return new Date(d.getTime()-d.getTimezoneOffset()*60000).toISOString().slice(0,16);
 };
 let draft=post?{...post}:blank(),publication=datestring(post?.published_at);
 let open=false,busy=false,dirty=false,advancedDirty=false,error='',notice='';
 let showPublication=!!post?.published_at;
 const today=new Date().toISOString().slice(0,10);
 $: path='/marketing/cursemark/beats/'+beatID+'/placements'+(post?'/'+post.id:'');
 $: if(post&&!busy&&!dirty&&post.version>draft.version){
  draft={...post};
  publication=datestring(post.published_at);
  showPublication=!!post.published_at;
 }

 async function save(event){
  const textOnly=!!target&&event.submitter?.value!=='details';
  busy=true;error='';notice='';
  try{
   let saved;
   if(post&&textOnly){
    saved=await portalRequest(path,'PATCH',{version:draft.version,target_id:target.id,title:draft.title,copy:draft.copy});
    draft={...draft,version:saved.version,title:saved.title,copy:saved.copy};
   }else{
    let body;
    if(target&&!post){
     body={...blank(),title:draft.title,copy:draft.copy,target_id:target.id};
    }else{
     const date=publication===datestring(draft.published_at)?draft.published_at:new Date(publication);
     body={...draft,published_at:showPublication&&publication?new Date(date).toISOString():null};
     if(!showPublication){
      body.published_url='';body.published_note='';
     }
    }
    saved=await portalRequest(path,post?'PUT':'POST',body);
    if(post){draft={...saved};publication=datestring(saved.published_at);}
    else if(!target){draft=blank();publication='';showPublication=false;open=false;}
   }
   if(!textOnly)advancedDirty=false;
   dirty=advancedDirty;
   notice=target&&textOnly?'Draft saved.':post?'Placement saved.':'Placement added.';
   dispatch('saved',saved);
  }catch(e){error=e.message;}
  finally{busy=false;}
 }

 async function archive(){
  busy=true;error='';
  try{
   const saved=await portalRequest(path+'/archive','PUT',{version:draft.version,archived:!post.archived_at});
   draft={...draft,...saved};
   notice=saved.archived_at?'Placement archived.':'Placement restored.';
   dispatch('saved',draft);
  }catch(e){error=e.message;}
  finally{busy=false;}
 }

 async function copy(text,message){
  try{await navigator.clipboard.writeText(text);notice=message;error='';}
  catch{error='Copy is unavailable. Select the text to copy it manually.';}
 }
</script>

<svelte:element this={target?'section':'details'} class="placement" class:inline={!!target}
 open={target?undefined:open} aria-label={target?(post?'Placement for ':'New draft for ')+target.name:undefined}
 on:toggle={event=>{if(!target)open=event.currentTarget.open;}}>
 {#if !target}
  <summary>
   {#if post}
    {#if post.rank}<span class="rank">#{post.rank}</span>{/if}
    <strong>{post.community}</strong> <span class="subtle">· {post.platform}</span>
    <span class:hold={post.decision==='hold'&&!post.published_at} class="state">{post.published_at?'Published':post.decision==='hold'?'Hold':'Candidate'}</span>
    {#if post.archived_at}<span class="state">Archived</span>{/if}
   {:else}<strong>Add an unlinked placement</strong>{/if}
  </summary>
 {:else if post}
  <div class="inline-heading">
   <strong>{post.published_at?'Published placement':'Draft'}</strong>
   <span class:hold={post.decision==='hold'&&!post.published_at} class="state">{post.published_at?'Published':post.decision==='hold'?'Hold':'Candidate'}</span>
  </div>
 {/if}
 {#if target||open}
  <form on:submit|preventDefault={save} on:input={event=>{dirty=true;if(event.target.closest('.advanced'))advancedDirty=true;}}>
   {#if target}
    <label>Title<input bind:value={draft.title} maxlength="255"/></label>
    <label>Body<textarea bind:value={draft.copy} rows="5" maxlength="5000"></textarea></label>
    <div class="text-actions">
     <button type="submit" value="text" formnovalidate disabled={busy||(!post&&!draft.title.trim()&&!draft.copy.trim())}>{busy?'Saving draft…':'Save draft'}</button>
     {#if post}<a href={post.steam_url} target="_blank" rel="noreferrer">Steam tracking link</a>{/if}
    </div>
    <MarketingHandoff {target} title={draft.title} body={draft.copy} steamURL={post?.steam_url||''} disabled={!allowHandoff}/>
   {/if}
   {#if !target||post}
    <svelte:element this={target?'details':'div'} class="advanced">
     {#if target}<summary>Placement details and publication</summary>{/if}
     <div class="settings">
      {#if post}
       <div class="links">
        {#if !target}<a href={post.steam_url} target="_blank" rel="noreferrer">Steam tracking link</a>{/if}
        <button type="button" class="secondary" on:click={()=>copy(post.steam_url,'Steam link copied.')}>Copy Steam link</button>
        {#if publicMarketingURL(post.published_url)}<a href={publicMarketingURL(post.published_url)} target="_blank" rel="noreferrer">Open published post</a>{/if}
        <button type="button" class="secondary" disabled={busy} on:click={archive}>{post.archived_at?'Restore placement':'Archive placement'}</button>
       </div>
      {/if}
      <div class="fields">
       <label>Platform<input bind:value={draft.platform} required maxlength="80"/></label>
       <label>Community<input bind:value={draft.community} required maxlength="160"/></label>
      </div>
      <label>Why it fits<textarea bind:value={draft.audience_fit} rows="2" maxlength="1500"></textarea></label>
      <label>Suggested timing<textarea bind:value={draft.timing} rows="2" maxlength="1000"></textarea></label>
      {#if !target}
       <label>Post title<input bind:value={draft.title} maxlength="255"/></label>
       <label>Post copy<textarea bind:value={draft.copy} rows="5" maxlength="5000"></textarea></label>
       {#if post&&draft.copy}
        <button type="button" class="secondary" on:click={()=>copy(draft.copy.replaceAll('{steam_link}',post.steam_url),'Draft copy copied.')}>Copy post text</button>
       {/if}
      {/if}
      <label>Asset notes<textarea bind:value={draft.asset_notes} rows="2" maxlength="1500"></textarea></label>
      <div class="settings rules">
       <h4>Community rules</h4>
       {#if publicMarketingURL(post?.rules_url)}
        <a href={publicMarketingURL(post.rules_url)} target="_blank" rel="noreferrer">{post.source_cited?'Open cited rules':'Open saved rules'}</a>
        {#if post.rules_checked}<span class="subtle">Checked {post.rules_checked}</span>{/if}
       {/if}
       <label>Rule evidence<textarea bind:value={draft.rule_evidence} rows="3" maxlength="2500"></textarea></label>
       <div class="fields">
        <label>Rules source URL<input type="url" bind:value={draft.rules_url} maxlength="1000" required={draft.decision==='candidate'}/></label>
        <label>Rules checked date<input type="date" bind:value={draft.rules_checked} max={today} required={draft.decision==='candidate'}/></label>
       </div>
       <label>Recommendation<select bind:value={draft.decision}><option value="hold">Hold</option><option value="candidate">Candidate for manual review</option></select></label>
      </div>
      <label class="confirm"><input type="checkbox" bind:checked={showPublication}/> Record a post already published</label>
      {#if showPublication}
       <div class="fields">
        <label>Live URL<input type="url" required bind:value={draft.published_url} maxlength="1000"/></label>
        <label>Published date and local time<input type="datetime-local" required bind:value={publication}/></label>
       </div>
       <label>Published details<textarea bind:value={draft.published_note} rows="2" maxlength="3000"></textarea></label>
      {/if}
      <button type="submit" value="details" disabled={busy}>{busy?'Saving…':post?'Save placement details':'Add draft'}</button>
     </div>
    </svelte:element>
   {/if}
  </form>
  {#if post?.published_at}<MarketingResults result={post.result}/>{/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
 {/if}
 {#if notice}<p role="status">{notice}</p>{/if}
</svelte:element>

<style>
 .placement{padding:.75rem;border:1px solid var(--line);border-radius:6px;background:var(--surface);margin:.55rem 0;overflow-wrap:anywhere}
 .placement.inline{background:var(--surface);margin:.55rem 0 0}
 summary{cursor:pointer;padding:.42rem 0;min-height:24px;color:var(--forest)}
 .rank{font-size:.78rem;margin-right:.2rem;color:var(--text-muted)}
 .subtle{color:var(--text-muted);font-size:.84rem}
 .state{margin-left:.4rem;padding:.18rem .4rem;border-radius:4px;background:var(--paper-warm);font-size:.8rem}
 .state.hold{background:#fbebd3;color:#653f15}
 .links,.text-actions,.inline-heading{display:flex;align-items:center;flex-wrap:wrap;gap:.55rem}
 .inline-heading{margin-bottom:.55rem}
 a{color:var(--forest);text-decoration:underline}
 form,.settings{display:grid;gap:.65rem}
 .rules,details.advanced{border-top:1px solid var(--line);padding-top:.55rem}
 .advanced summary{margin-bottom:.5rem}
 h4{margin:0}
 .fields{display:grid;grid-template-columns:1fr 1fr;gap:.65rem}
 label{display:grid;min-width:0;gap:.25rem;font-size:.9rem;font-weight:600}
 input,select,textarea,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input,select,textarea{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 textarea{resize:vertical}
 button{justify-self:start;border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .65rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 .confirm{display:flex;align-items:center;gap:.5rem}
 .confirm input{width:auto}
 p{text-align:left;margin:.55rem 0}
 .error{color:#9d302b}
 @media(max-width:490px){.fields{grid-template-columns:1fr}.placement{padding:.65rem}}
</style>
