<script>
 import {onMount} from 'svelte';
 import Panel from './Panel.svelte';
 import {portalAuth,portalRequest,refreshPortalAuth} from '../portal-auth.mjs';
 let users=[],loading=true,busy=false,error='',status='',username='',role='viewer',dialog,createError='',temporary=null,reveal=false;
 async function load(){loading=true;error='';try{const r=await portalRequest('/auth/users','GET');users=r.items;}catch(e){error=e.message;}finally{loading=false;}}
 onMount(load);
 async function perform(path,method,body){
  busy=true;error='';status='';temporary=null;reveal=false;
  try{const r=await portalRequest(path,method,body);if(r.temporary_password)temporary={username:r.user.username,password:r.temporary_password};status='Saved. Existing affected sessions were revoked where applicable.';await load();await refreshPortalAuth();return true;}catch(e){error=e.message;}finally{busy=false;}
 }
 async function create(){createError='';if(await perform('/auth/users','POST',{username,role})){dialog.close();}else{createError=error;error='';}}
 function openCreate(){username='';role='viewer';createError='';dialog.showModal();}
 function update(user,field,value){perform(`/auth/users/${user.id}`,'PATCH',{version:user.version,[field]:value});}
</script>
<Panel><section class="portal-users">
 <div class="user-header"><h2>Portal users</h2><button type="button" disabled={busy} on:click={openCreate}>Add user</button></div>
 <dialog bind:this={dialog} on:cancel={event => { if(busy) event.preventDefault(); }} aria-labelledby="add-portal-user-title">
  <h2 id="add-portal-user-title">Add user</h2>
  <form class="create" on:submit|preventDefault={create}>
   <label>Username<input bind:value={username} minlength="3" maxlength="32" required pattern={/^[a-zA-Z][a-zA-Z0-9_.\-]{2,31}$/.source} autocomplete="off" disabled={busy}/></label>
   <label>Role<select bind:value={role} disabled={busy}><option value="viewer">Viewer (read only)</option><option value="admin">Administrator</option></select></label>
   {#if createError}<p role="alert">{createError}</p>{/if}
   <div class="actions"><button type="button" disabled={busy} on:click={() => dialog.close()}>Cancel</button><button disabled={busy}>{busy?'Adding…':'Add user'}</button></div>
  </form>
 </dialog>
 {#if temporary}<div class="credential" role="status"><b>Temporary password for {temporary.username}</b><p>Shown only for this operation. Save it in your password manager and share it through a private channel. The user must change it at first login.</p><input aria-label="Generated temporary password" type={reveal?'text':'password'} value={temporary.password} readonly autocomplete="off"/><button on:click={()=>reveal=!reveal}>{reveal?'Hide':'Reveal'}</button><button on:click={()=>{temporary=null;reveal=false;}}>Clear</button></div>{/if}
 {#if loading}<p role="status">Loading users…</p>{:else}<div class="table-scroll"><table><thead><tr><th>Username</th><th>Role</th><th>Access</th><th>Actions</th></tr></thead><tbody>{#each users as user (user.id)}<tr><td data-label="Username">{user.username}{#if user.id===$portalAuth.user?.id} · you{/if}{#if user.must_change_password}<small>Password change required</small>{/if}</td><td data-label="Role"><select aria-label={`Role for ${user.username}`} value={user.role} disabled={busy} on:change={e=>update(user,'role',e.currentTarget.value)}><option value="viewer">Viewer</option><option value="admin">Admin</option></select></td><td data-label="Access">{user.disabled?'Disabled':'Enabled'}</td><td class="actions" data-label="Actions"><button disabled={busy} on:click={()=>update(user,'disabled',!user.disabled)}>{user.disabled?'Enable':'Disable'}</button><button disabled={busy||user.id===$portalAuth.user?.id} on:click={()=>perform(`/auth/users/${user.id}/reset-password`,'POST',{version:user.version})}>Reset password</button><button disabled={busy} on:click={()=>perform(`/auth/users/${user.id}/revoke`,'POST',{version:user.version})}>Sign out sessions</button></td></tr>{/each}</tbody></table></div>{/if}
 {#if error}<p role="alert">{error}</p>{/if}{#if status}<p role="status">{status}</p>{/if}
 </section></Panel>
<style>.user-header{display:flex;justify-content:space-between;align-items:center;gap:.5rem}.portal-users dialog{width:min(360px,calc(100vw - 2rem));box-sizing:border-box;border:1px solid var(--line-strong);border-radius:6px;background:var(--surface);color:var(--text);padding:1rem}.portal-users dialog::backdrop{background:#0008}.portal-users .create{display:grid;align-items:stretch}.portal-users .create .actions{justify-content:flex-end}.portal-users{text-align:left;min-width:0}.portal-users h2{text-align:left;font-size:1.15rem}.portal-users p{text-align:left;font-size:.8rem;color:var(--text-soft)}label{display:flex;flex-direction:column;gap:5px;font-size:.78rem}.create{display:flex;align-items:end;flex-wrap:wrap;gap:12px;margin:18px 0}input,select,button{font:inherit;padding:7px;border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);color:var(--text);box-sizing:border-box}button{cursor:pointer;font-size:.75rem}button:disabled{opacity:.5;cursor:default}.table-scroll{overflow:auto}table{width:100%;border-collapse:collapse;font-size:.8rem;text-align:left}th,td{padding:10px 6px;border-bottom:1px solid var(--line)}small{display:block;color:var(--text-soft)}.actions{display:flex;flex-wrap:wrap;gap:6px}.credential{padding:14px;border:1px solid var(--forest);margin:16px 0}.credential input{max-width:100%;font-family:var(--font-mono);margin-right:6px}.portal-users [role=alert]{color:var(--rust)}@media(max-width:550px){.create label,.create button{width:100%}table,tbody,tr,td{display:block;width:100%;box-sizing:border-box}thead{display:none}tr{border-bottom:1px solid var(--line);padding:10px 0}td{border:0;padding:6px 0;overflow-wrap:anywhere}td::before{content:attr(data-label);display:block;font-size:.7rem;color:var(--text-soft);margin-bottom:5px}.actions{display:flex}.actions::before{flex-basis:100%}.actions button{flex:1}}</style>
