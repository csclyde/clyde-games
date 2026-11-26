<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 import MarketingResults from './MarketingResults.svelte';

 export let beatID;
 export let post=null;
 const dispatch=createEventDispatcher();
 const blank=()=>({platform:'',community:'',audience_fit:'',timing:'',title:'',copy:'',asset_notes:'',rule_evidence:'',rules_url:'',rules_checked:'',decision:'hold',published_url:'',published_note:''});
 const datestring=value=>{
  if(!value)return '';
  const d=new Date(value);
  return new Date(d.getTime()-d.getTimezoneOffset()*60000).toISOString().slice(0,16);
 };
 let draft=post?{...post}:blank(),publication=datestring(post?.published_at);
 let open=false,busy=false,error='',notice='';
 let showPublication=!!post?.published_at;
 const today=new Date().toISOString().slice(0,10);

 async function save(){
  busy=true;error='';notice='';
  const id=post?.id;
  try{
   const date=publication===datestring(draft.published_at)?draft.published_at:new Date(publication);
   const body={...draft,published_at:showPublication&&publication?new Date(date).toISOString():null};
   if(!showPublication){
    body.published_url='';body.published_note='';
   }
   const saved=await portalRequest('/marketing/cursemark/beats/'+beatID+'/placements'+(id?'/'+id:''),id?'PUT':'POST',body);
   if(id){draft={...saved};publication=datestring(saved.published_at);}
   else {draft=blank();publication='';showPublication=false;open=false;}
   notice=id?'Placement saved.':'Placement added.';
   dispatch('saved',saved);
  }catch(e){error=e.message;}
  finally{busy=false;}
 }

 async function copy(text,message){
  try{await navigator.clipboard.writeText(text);notice=message;error='';}
  catch{error='Copy is unavailable. Select the text to copy it manually.';}
 }
</script>

<details class="placement" bind:open>
 <summary>
  {#if post}
   {#if post.rank}<span class="rank">#{post.rank}</span>{/if}
   <strong>{post.community}</strong> <span class="subtle">· {post.platform}</span>
   <span class:hold={post.decision==='hold'&&!post.published_at} class="state">{post.published_at?'Published':post.decision==='hold'?'Hold':'Candidate'}</span>
  {:else}<strong>Add a placement</strong>{/if}
 </summary>
 {#if open}
  {#if post}
   <div class="links">
    <a href={post.steam_url} target="_blank" rel="noreferrer">Steam tracking link</a>
    <button type="button" class="secondary" on:click={()=>copy(post.steam_url,'Steam link copied.')}>Copy Steam link</button>
    {#if post.published_url}<a href={post.published_url} target="_blank" rel="noreferrer">Open published post</a>{/if}
   </div>
  {/if}
  <form on:submit|preventDefault={save}>
   <div class="fields">
    <label>Platform<input bind:value={draft.platform} required maxlength="80"/></label>
    <label>Community<input bind:value={draft.community} required maxlength="160"/></label>
   </div>
   <label>Why it fits<textarea bind:value={draft.audience_fit} rows="2" maxlength="1500"></textarea></label>
   <label>Suggested timing<textarea bind:value={draft.timing} rows="2" maxlength="1000"></textarea></label>
   <label>Post title<input bind:value={draft.title} maxlength="255"/></label>
   <label>Post copy<textarea bind:value={draft.copy} rows="5" maxlength="5000"></textarea></label>
   {#if post&&draft.copy}
    <button type="button" class="secondary copy" on:click={()=>copy(draft.copy.replaceAll('{steam_link}',post.steam_url),'Draft copy copied.')}>Copy post text</button>
   {/if}
   <label>Asset notes<textarea bind:value={draft.asset_notes} rows="2" maxlength="1500"></textarea></label>
   <div class="rules">
    <h4>Community rules</h4>
    {#if post?.rules_url}
     <a href={post.rules_url} target="_blank" rel="noreferrer">{post.source_cited?'Open cited rules':'Open saved rules'}</a>
     {#if post.rules_checked}<span class="subtle"> · checked {post.rules_checked}</span>{/if}
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
   <button type="submit" disabled={busy}>{busy?'Saving…':post?'Save placement':'Add draft'}</button>
  </form>
  {#if post?.published_at}<MarketingResults result={post.result}/>{/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
 {/if}
 {#if notice}<p role="status">{notice}</p>{/if}
</details>

<style>
 .placement{padding:.8rem 1rem;border:1px solid var(--line);border-radius:6px;background:var(--surface);margin:.55rem 0;overflow-wrap:anywhere}
 summary{cursor:pointer;padding:.42rem 0;min-height:24px}
 .rank{font-size:.78rem;margin-right:.2rem;color:var(--text-muted)}
 .subtle{color:var(--text-muted);font-size:.84rem}
 .state{margin-left:.5rem;padding:.2rem .4rem;border-radius:4px;background:var(--paper-warm);font-size:.8rem}
 .state.hold{background:#fbebd3;color:#653f15}
 .links{display:flex;align-items:center;flex-wrap:wrap;gap:.65rem;margin:.65rem 0}
 a{color:var(--forest);text-decoration:underline}
 form,.rules{display:grid;gap:.65rem}
 .rules{border-top:1px solid var(--line);padding-top:.65rem}
 h4{margin:0}
 .fields{display:grid;grid-template-columns:1fr 1fr;gap:.65rem}
 label{display:grid;min-width:0;gap:.25rem;font-size:.9rem;font-weight:600}
 input,select,textarea,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input,select,textarea{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 textarea{resize:vertical}
 button{justify-self:start;border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .75rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 .confirm{display:flex;align-items:center;gap:.5rem}
 .confirm input{width:auto}
 p{text-align:left;margin:.55rem 0}
 .error{color:#9d302b}
 @media(max-width:490px){.fields{grid-template-columns:1fr}.placement{padding:.65rem}}
</style>
