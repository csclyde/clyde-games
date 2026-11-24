import {writable,derived,get} from 'svelte/store';
export const portalAuth=writable({phase:'checking',mode:null,user:null,error:''});
export const canWrite=derived(portalAuth,s=>s.user?.role==='admin');
let csrf='',generation=0,authEpoch=0,installed=false,originalFetch,channel;
function clear(reason=''){authEpoch++;generation++;csrf='';portalAuth.set({phase:'signed_out',mode:'session',user:null,error:reason});}
function accept(data,changedSession=false){
 if(!data?.user||!['admin','viewer'].includes(data.user.role)||!['legacy','session'].includes(data.mode)||(data.mode==='session'&&(!Number.isInteger(data.user.id)||data.user.id<1||typeof data.csrf!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(data.csrf))))throw Error('Sign-in response was invalid.');
 if(changedSession)authEpoch++;generation++;csrf=data.csrf||'';portalAuth.set({phase:'ready',mode:data.mode,user:data.user,error:''});
}
export function installPortalFetch(){
 if(installed)return;installed=true;originalFetch=globalThis.fetch.bind(globalThis);
 globalThis.fetch=async(input,init={})=>{
  const url=new URL(typeof input==='string'||input instanceof URL?input:input.url,location.href),scoped=url.origin===location.origin&&url.pathname.startsWith('/api/');
  if(!scoped)return originalFetch(input,init);
  const headers=new Headers(typeof Request!=='undefined'&&input instanceof Request?input.headers:undefined);new Headers(init.headers).forEach((v,k)=>headers.set(k,v));
  const method=(init.method||(typeof input!=='string'&&input.method)||'GET').toUpperCase();
  if(!['GET','HEAD','OPTIONS'].includes(method)&&csrf)headers.set('X-CSRF-Token',csrf);
  const epoch=authEpoch;
  const response=await originalFetch(input,{...init,headers,credentials:'same-origin'});
  if(response.status===401&&!url.pathname.endsWith('/auth/login')&&epoch===authEpoch){const hadUser=!!get(portalAuth).user;clear('Your session ended. Sign in again.');if(hadUser)channel?.postMessage('signed_out');}
  return response;
 };
 if(typeof BroadcastChannel!=='undefined'){channel=new BroadcastChannel('clyde-portal-auth');channel.onmessage=e=>{if(e.data==='signed_out')refreshPortalAuth();};}
}
export async function refreshPortalAuth(){
 if(get(portalAuth).phase==='logout_error')return;
 const ticket=++generation,epoch=authEpoch;
 try{const r=await fetch('/api/auth/session',{cache:'no-store'});if(ticket!==generation||epoch!==authEpoch)return;if(r.status===401){clear();return;}const data=await r.json();if(ticket!==generation)return;if(!r.ok)throw Error('Sign-in is temporarily unavailable.');accept(data);}
 catch(e){if(ticket===generation)portalAuth.set({phase:'unavailable',mode:null,user:null,error:e.message});}
}
export async function portalRequest(path,method,body){
 const r=await fetch('/api'+path,{method,headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});
 const data=await r.json().catch(()=>({}));if(!r.ok){const e=Error(data.error||'The request failed. Inspect current state before retrying.');e.status=r.status;e.code=data.code;throw e;}return data;
}
export async function signIn(username,password){const data=await portalRequest('/auth/login','POST',{username,password});accept(data,true);}
export async function changePortalPassword(current_password,new_password){try{const data=await portalRequest('/auth/password','POST',{current_password,new_password});accept(data,true);}catch(e){if(e.code==='password_changed_sign_in'){clear(e.message);channel?.postMessage('signed_out');}throw e;}}
export async function signOut(){
 const previous=get(portalAuth);generation++;
 try{await portalRequest('/auth/logout','POST',{});clear();channel?.postMessage('signed_out');}
 catch(e){if(e.status===401){clear();channel?.postMessage('signed_out');return;}portalAuth.set({phase:'logout_error',mode:previous.mode,user:null,error:'Sign-out was not confirmed. Retry when connected.'});throw e;}
}
