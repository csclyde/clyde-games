<script>
 import {onMount} from 'svelte';
 import {portalRequest} from '../portal-auth.mjs';
 import MarketingTargets from '../components/MarketingTargets.svelte';

 let targets=[],pending=true,error='';

 onMount(()=>{loadTargets();});

 async function loadTargets(){
  pending=true;error='';
  try{targets=(await portalRequest('/marketing/cursemark/targets?include_archived=1','GET')).targets;}
  catch(e){error=e.message;}
  finally{pending=false;}
 }

 function savedTarget(event){
  const saved=event.detail;
  targets=[...targets.filter(target=>target.id!==saved.id),saved];
 }
</script>

<section class="marketing" aria-label="Marketing targets">
 {#if error}<p role="alert">{error} <button type="button" class="secondary" on:click={loadTargets}>Retry</button></p>{/if}
 <MarketingTargets {targets} {pending} on:saved={savedTarget} on:imported={loadTargets}/>
</section>

<style>
 .marketing{max-width:1368px;margin:12px auto 32px;padding:0 16px;text-align:left;color:var(--text)}
 p{text-align:left;color:var(--error,#ff8c8c);margin:.55rem 0;line-height:1.45}
</style>
