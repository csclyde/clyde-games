<script>
 import {createEventDispatcher,onMount} from 'svelte';

 export let beat=null;
 export let busy=false;
 export let error='';
 const dispatch=createEventDispatcher();
 let dialog;
 let name=beat?.name||'',brief=beat?.brief||'',call_to_action=beat?.call_to_action||'';

 onMount(()=>dialog.showModal());
</script>

<dialog bind:this={dialog} aria-labelledby="beat-form-title" on:close={()=>dispatch('dismiss')} on:cancel|preventDefault={()=>{if(!busy)dialog.close();}}>
 <form on:submit|preventDefault={()=>dispatch('save',{name,brief,call_to_action})}>
  <h2 id="beat-form-title">{beat?'Edit Beat':'New Beat'}</h2>
  <label>Beat Name<input maxlength="160" required bind:value={name}/></label>
  <label>Brief<textarea rows="5" maxlength="3000" required bind:value={brief}></textarea></label>
  <label>Call to Action
   <select required bind:value={call_to_action}>
    <option value="" disabled>Choose a call to action</option>
    <option value="wishlist">Wishlist</option>
    <option value="purchase">Purchase</option>
   </select>
  </label>
  {#if error}<p role="alert">{error}</p>{/if}
  <div class="actions">
   <button type="button" class="secondary" disabled={busy} on:click={()=>dialog.close()}>Cancel</button>
   <button type="submit" disabled={busy}>{busy?'Saving…':beat?'Save Beat':'Create Beat'}</button>
  </div>
 </form>
</dialog>

<style>
 dialog{box-sizing:border-box;width:min(33rem,calc(100% - 2rem));max-height:calc(100% - 2rem);overflow:auto;padding:1rem 1.2rem;border:1px solid var(--line-strong);border-radius:7px;background:var(--surface);color:var(--text);text-align:left}
 dialog::backdrop{background:#1b211b90}
 h2{margin:0;text-align:left;font-size:1.15rem}
 form,label{display:grid;gap:.65rem}
 label{gap:.25rem;font-size:.9rem;font-weight:600}
 input,select,textarea,button{box-sizing:border-box;font:inherit;min-width:0;min-height:44px}
 input,select,textarea{width:100%;background:var(--surface);color:var(--text);border:1px solid var(--line-strong);border-radius:4px;padding:.45rem .55rem}
 textarea{resize:vertical}
 .actions{display:flex;flex-wrap:wrap;gap:.5rem;justify-content:flex-end}
 button{border:1px solid var(--forest);border-radius:5px;background:var(--forest);color:white;padding:.4rem .75rem;cursor:pointer;font-weight:650}
 button.secondary{color:var(--forest);background:var(--surface)}
 button:disabled{opacity:.6;cursor:default}
 p{color:var(--rust);text-align:left;margin:0}
</style>
