<script>
 import {onMount} from 'svelte';
 import {portalAuth,installPortalFetch,refreshPortalAuth,signIn,signOut,changePortalPassword} from '../portal-auth.mjs';
 let username='',password='',currentPassword='',newPassword='',confirmation='',busy=false,error='',changing=false;
 installPortalFetch();
 onMount(()=>{refreshPortalAuth();const recheck=()=>{if(document.visibilityState==='visible')refreshPortalAuth();};document.addEventListener('visibilitychange',recheck);window.addEventListener('pageshow',recheck);return()=>{document.removeEventListener('visibilitychange',recheck);window.removeEventListener('pageshow',recheck);};});
 async function login(){busy=true;error='';try{await signIn(username,password);}catch(e){error=e.message;}finally{password='';busy=false;}}
 async function logout(){busy=true;error='';try{await signOut();}catch(e){error=e.message;}finally{busy=false;}}
 async function changePassword(){if(newPassword!==confirmation){error='Passwords do not match.';return;}busy=true;error='';try{await changePortalPassword(currentPassword,newPassword);changing=false;}catch(e){error=e.message;}finally{currentPassword='';newPassword='';confirmation='';busy=false;}}
</script>

{#if $portalAuth.phase==='ready'&&$portalAuth.user&&!$portalAuth.user.must_change_password&&!changing}
 {#key $portalAuth.mode==='session'?$portalAuth.user.id:'legacy'}<slot beginPasswordChange={() => { error=""; changing=true; }} signOutAccount={logout} accountBusy={busy}/>{/key}
{:else}
 <main class="auth-shell"><h1>CLYDE GAMES</h1>
 {#if $portalAuth.phase==='checking'}<p role="status">Checking access…</p>
 {:else if $portalAuth.phase==='unavailable'}<h2>Sign-in unavailable</h2><p role="alert">{$portalAuth.error}</p><button disabled={busy} on:click={refreshPortalAuth}>Try again</button>
 {:else if $portalAuth.phase==='logout_error'}<h2>Sign-out pending</h2><p role="alert">{$portalAuth.error}</p><button disabled={busy} on:click={logout}>Retry sign out</button>
 {:else if $portalAuth.user?.must_change_password||changing}
  <h2>Change password</h2><p>{ $portalAuth.user.must_change_password?'Choose your own password before continuing.':'Changing your password signs out your other sessions.'}</p>
  <form on:submit|preventDefault={changePassword}><label>Current password<input type="password" autocomplete="current-password" bind:value={currentPassword} required maxlength="512" disabled={busy}/></label><label>New password<input type="password" autocomplete="new-password" bind:value={newPassword} required minlength="15" maxlength="128" disabled={busy}/></label><label>Confirm new password<input type="password" autocomplete="new-password" bind:value={confirmation} required minlength="15" maxlength="128" disabled={busy}/></label><small>Use a unique passphrase of 15–128 characters.</small><button disabled={busy}>{busy?'Saving…':'Change password'}</button></form>
  {#if !$portalAuth.user.must_change_password}<button disabled={busy} on:click={()=>changing=false}>Cancel</button>{/if}<button disabled={busy} on:click={logout}>Sign out</button>
 {:else}
  <h2>Sign in</h2><form on:submit|preventDefault={login}><label>Username<input autocomplete="username" bind:value={username} required maxlength="32" autocapitalize="none" spellcheck="false" disabled={busy}/></label><label>Password<input type="password" autocomplete="current-password" bind:value={password} required maxlength="512" disabled={busy}/></label><button disabled={busy}>{busy?'Signing in…':'Sign in'}</button></form><p>Accounts are created by an administrator.</p>
 {/if}
 {#if error}<p role="alert">{error}</p>{/if}
 </main>
{/if}
<style>button,input{font:inherit;color:var(--text);border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);padding:8px 12px}button{cursor:pointer}button:disabled{opacity:.55;cursor:default}.auth-shell{max-width:420px;margin:8vh auto;padding:24px;background:var(--surface);border:1px solid var(--line);border-radius:6px;text-align:left}.auth-shell h1{font-size:1.3rem;text-align:left}.auth-shell h2{font-size:1.15rem;text-align:left}.auth-shell p{text-align:left;font-size:.85rem}form,label{display:flex;flex-direction:column;gap:7px}form{gap:16px;margin:18px 0}input{width:100%;box-sizing:border-box}small{color:var(--text-soft);font-size:.75rem}[role=alert]{color:var(--rust)}@media(max-width:760px){.auth-shell{margin:30px 12px;padding:18px}}</style>
