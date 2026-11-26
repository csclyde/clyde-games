<script>
 import {createEventDispatcher} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 export let target;
 const changed=createEventDispatcher();
 const policyLabels={links:'Links',self_promotion:'Self-promotion',ai:'AI rules for publication',flair:'Flair',cadence:'Cadence'};
 let form=JSON.parse(JSON.stringify(target.research||{rules:{},sources:[],examples:[],hold:true}));
 let links=(form.sources||[]).join('\n'),games=(form.games_covered||[]).join('\n'),busy=false,error='';
 form.examples??=[];
 async function save(){
  busy=true;error='';
  const split=value=>value.split(/\r?\n/).map(item=>item.trim()).filter(Boolean);
  try{
   const research={...form,sources:split(links),games_covered:split(games)};
   const saved=await portalRequest('/marketing/cursemark/targets/'+target.id+'/research','PUT',{version:target.version,research});
   changed('saved',saved);
  }catch(e){error=e.message;}
  finally{busy=false;}
 }
</script>

<form on:submit|preventDefault={save} aria-label="Edit research report">
 <label>Research summary<textarea rows="3" maxlength="1600" bind:value={form.takeaway}></textarea></label>
 <label>Verified public source URLs<textarea rows="3" bind:value={links}></textarea></label>
 {#if target.kind==='creator'}
  <label>Games found in coverage<textarea rows="2" bind:value={games}></textarea></label>
 {:else}
  {#each Object.entries(policyLabels) as [id,label]}<label>{label}<textarea rows="2" maxlength="800" bind:value={form.rules[id]}></textarea></label>{/each}
  <label>AI restriction<select bind:value={form.ai_restriction}><option value="">Unspecified</option><option value="final_content">Applies to final public content</option><option value="assisted_submissions">Applies to AI-assisted submissions</option></select></label>
 {/if}
 <label class="check"><input type="checkbox" bind:checked={form.hold}/> Hold</label>
 <label>Hold reason<textarea rows="2" maxlength="1000" bind:value={form.hold_reason}></textarea></label>
 {#each form.examples as example,i}
  <fieldset><legend>Public example {i+1}</legend>
   <label>Title<input maxlength="250" required bind:value={example.title}/></label>
   <label>Public URL<input type="url" required bind:value={example.url}/></label>
   <label>Date<input type="date" required bind:value={example.date}/></label>
   <label>Observed engagement<input maxlength="300" bind:value={example.engagement}/></label>
   <label>What appeared to work<textarea rows="2" maxlength="800" bind:value={example.what_worked}></textarea></label>
   <button type="button" on:click={()=>form.examples=form.examples.filter((_,index)=>index!==i)}>Remove example</button>
  </fieldset>
 {/each}
 <button type="button" disabled={form.examples.length>=4} on:click={()=>form.examples=[...form.examples,{title:'',url:'',date:'',engagement:'',what_worked:''}]}>Add public example</button>
 {#if error}<p role="alert">{error}</p>{/if}
 <div><button class="primary" type="submit" disabled={busy}>Save research</button> <button type="button" disabled={busy} on:click={()=>changed('cancel')}>Cancel</button></div>
</form>

<style>
 form,fieldset,label{display:grid;gap:.5rem}
 fieldset{border:1px solid var(--line);border-radius:4px;min-width:0}
 input,select,textarea,button{box-sizing:border-box;min-width:0;min-height:44px;font:inherit;color:var(--text);background:var(--surface);border:1px solid var(--line-strong);border-radius:4px;padding:.4rem}
 button{justify-self:start;cursor:pointer;color:var(--forest)}
 button.primary{color:white;background:var(--forest);border-color:var(--forest)}
 .check{display:flex;align-items:center}
 p{color:var(--rust);text-align:left}
</style>
