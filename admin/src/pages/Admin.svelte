<script>
 import { onMount } from 'svelte';
 import {createFeedbackActivity,startNavigationPolling} from '../feedback-activity.mjs';
 import {portalAuth,canWrite} from '../portal-auth.mjs';
 import PortalUsers from '../components/PortalUsers.svelte';
	import Feedback from './Feedback.svelte';
	import Crashes from './Crashes.svelte';
	import Metrics from './Metrics.svelte';
	import ServerStats from '../components/ServerStats.svelte';
	import User from './User.svelte';
	import Users from './Users.svelte';
	import Projects from '../components/Projects.svelte';

	import MetricsDashboard from './MetricsDashboard.svelte';
 import PackratDocs from '../components/PackratDocs.svelte';

 export let tab = 'admin';
 export let userID = '';
 export let beginPasswordChange, signOutAccount, accountBusy;
 const tabs = [
  {id:'admin',label:'Overview',href:'/admin',component:Metrics},
  {id:'metrics',label:'Metrics',href:'/metrics',component:MetricsDashboard},
  {id:'feedback',label:'Feedback',href:'/feedback',component:Feedback},
  {id:'crashes',label:'Crashes',href:'/crashes',component:Crashes},

 ];
 let selectedProject = 'cursemark';
 let projects = [{id:'cursemark',name:'Cursemark'}];
 let counts = {};
 let crashCounts = {};
 let navigationError = '';
 let navigationLoading = false;
 let mobileOpen = false;
 let activity={unseen:{},any:false,persistent:true},activityError='',tracker,poller,feedbackRefreshSignal=0;
 let feedbackSignatures={};
 const accountScope=$portalAuth.mode==='session'?`user:${$portalAuth.user.id}`:'legacy';
 function setActivity(next){activity=next;const icon=document.getElementById('portal-favicon');if(icon){icon.href=next.any?'/favicon-unseen.svg':'/favicon.png';icon.type=next.any?'image/svg+xml':'image/png';}}
 function reviewFeedback(project,ids){tracker?.review(project,ids);}
 function reportProjects(source, ids) {
  const known = new Set(projects.map(p => p.id.toLowerCase()));
  const added = [...new Set(ids.filter(Boolean).map(p => p.trim()))].filter(id => !known.has(id.toLowerCase()));
  if(added.length) projects = [...projects, ...added.map(id => ({id,name:id}))];
 }
 function reportFeedbackCounts(items) {
  const next = {};
  for(const item of items) { const id=(item.Project || '').toLowerCase(); next[id]=(next[id] || 0)+1; }
  counts=Object.fromEntries(projects.map(p=>[p.id.toLowerCase(),next[p.id.toLowerCase()] || 0]));
 }
 function reportCrashCounts(items) {
  const next={}; for(const item of items) {const id=(item.Project || '').toLowerCase(); next[id]=(next[id] || 0)+1;}
  crashCounts=Object.fromEntries(projects.map(p=>[p.id.toLowerCase(),next[p.id.toLowerCase()] || 0]));
 }
 function applyNavigation(data) {
  if(!Array.isArray(data)||data.length>200||data.some(p=>!p||typeof p.id!=='string'||typeof p.name!=='string'))throw Error('Project list could not be refreshed.');
  projects=data.some(p=>p.id.toLowerCase()==='cursemark')?data:[{id:'cursemark',name:'Cursemark'},...data];
  counts=Object.fromEntries(data.map(p=>[p.id.toLowerCase(),p.openFeedback]));
  crashCounts=Object.fromEntries(data.map(p=>[p.id.toLowerCase(),p.openCrashes]));
  const next=Object.fromEntries(data.map(p=>[p.id.toLowerCase(),JSON.stringify([p.openFeedback,p.openFeedbackIds,p.feedbackRevision])]));
  if(feedbackSignatures[selectedProject.toLowerCase()]!==next[selectedProject.toLowerCase()])feedbackRefreshSignal++;
  feedbackSignatures=next;
  try{tracker.ingest(data);activityError='';}catch(e){activityError=e.message;}
  navigationError='';navigationLoading=false;
 }
 function selectProject(id) { selectedProject=id; mobileOpen=false; }
 onMount(()=>{
  let storage;try{storage=window.localStorage;}catch{}
  tracker=createFeedbackActivity({scope:accountScope,storage,onChange:setActivity});
  const merge=e=>{if(e.key===tracker.key&&e.newValue)tracker.merge(e.newValue);};window.addEventListener('storage',merge);
  navigationLoading=true;
  poller=startNavigationPolling({document,window,navigator,
   load:async signal=>{const r=await fetch('/api/projects?view=navigation',{signal,cache:'no-store'});if(!r.ok)throw Error(r.status===401?'Sign in required.':'Updates paused; retrying.');return r.json();},
   apply:applyNavigation,onError:message=>{navigationError=message;navigationLoading=false;}});
  return()=>{poller.stop();window.removeEventListener('storage',merge);setActivity({unseen:{},any:false,persistent:true});};
 });
 $: isClyde=selectedProject==='clyde-games';
 $: isPackrat=selectedProject.toLowerCase()==='packrat';
 $: hasMetrics=selectedProject.toLowerCase()==='cursemark';
 $: visibleTabs=tabs.filter(t => t.id==='admin' || (!isClyde && !isPackrat && (t.id==='feedback' || t.id==='crashes' || (t.id==='metrics' && (hasMetrics || selectedProject.toLowerCase()==='necrovale')))));
 $: activeTab=visibleTabs.find(t=>t.id===tab)||visibleTabs[0];
 $: selectedName=isClyde?'Clyde Games':projects.find(p=>p.id.toLowerCase()===selectedProject.toLowerCase())?.name || selectedProject;
 const projectOrder=['pathprodigy','packrat','cursemark','necrovale'];
 function rank(id) {const i=projectOrder.indexOf(id.toLowerCase());return i<0?projectOrder.length:i;}
 $: sortedProjects=[...projects].sort((a,b)=>rank(a.id)-rank(b.id)||a.name.localeCompare(b.name));
</script>

<div class="admin-page">
 <button class="project-toggle" aria-expanded={mobileOpen} aria-controls="project-sidebar" on:click={()=>mobileOpen=!mobileOpen}>Projects: {selectedName}{#if activity.any}<span class="feedback-dot" role="img" aria-label="Unseen feedback"></span>{/if}</button>
 <aside id="project-sidebar" class:mobile-open={mobileOpen} aria-label="Projects">
  <div class="sidebar-heading"><strong>CLYDE GAMES</strong><span>Projects</span></div>
  <nav class="project-list" aria-label="Choose project">
   {#each sortedProjects as project}
    <button class:chosen={selectedProject.toLowerCase()===project.id.toLowerCase()} aria-pressed={selectedProject.toLowerCase()===project.id.toLowerCase()} on:click={()=>selectProject(project.id)}>
     <span class="project-name">{project.name}{#if activity.unseen[project.id.toLowerCase()]}<span class="feedback-dot" role="img" aria-label="Unseen feedback"></span>{/if}</span>
     <span class="activity" aria-label="Open report counts">Feedback {counts[project.id.toLowerCase()] ?? 'unavailable'} · Crashes {crashCounts[project.id.toLowerCase()] ?? 'unavailable'}</span>
    </button>
   {/each}
{#if $canWrite || $portalAuth.mode==="session"}   <button class="all-projects" class:chosen={isClyde} aria-pressed={isClyde} on:click={()=>selectProject('clyde-games')}>Clyde Games</button>{/if}
  </nav>
  {#if navigationError||activityError}<div class="sidebar-footer"><p role="status">{navigationError||activityError}</p><button class="retry-updates" on:click={()=>poller?.refresh()}>Retry updates</button></div>{/if}
 </aside>
 <div class="workspace">
  <header class="admin-header"><h1>{selectedName}</h1><nav class="admin-tabs" aria-label="Admin sections">{#each visibleTabs as item}<a href={item.href} class:active={item.id===activeTab.id} aria-current={item.id===activeTab.id?'page':undefined}>{item.label}</a>{/each}</nav></header>
  {#if isClyde}
      {#if $portalAuth.mode==='session'}
    <div class="overview-account">
     <span>{$portalAuth.user.username}{#if $portalAuth.user.role==='viewer'} (read only){/if}</span>
     <button type="button" disabled={accountBusy} on:click={beginPasswordChange}>Change password</button>
     <button type="button" disabled={accountBusy} on:click={signOutAccount}>Sign out</button>
    </div>
   {/if}
   {#if $canWrite}<ServerStats/>{/if}
 {#if $portalAuth.mode==='session'&&$portalAuth.user?.role==='admin'}<PortalUsers/>{/if}
  {:else if isPackrat}
   <PackratDocs/>
  {:else if tab==='user'}
   <User {userID} {selectedProject}/>
  {:else if activeTab.id==='admin'}
   <Projects {reportProjects} {selectedProject} compact={true}/>
   {#if hasMetrics}<Metrics {selectedProject} showGlobalSettings={$canWrite} {reportProjects}/><Users/>{/if}

  {:else}
   {#key activeTab.id==='metrics'?`${activeTab.id}:${selectedProject}`:activeTab.id}
    <svelte:component this={activeTab.component} {selectedProject} {reportProjects} {reportFeedbackCounts} {reportCrashCounts} refreshSignal={feedbackRefreshSignal} sourceConfig={projects.find(p => p.id === selectedProject)} {reviewFeedback}/>
   {/key}
  {/if}
 </div>
</div>
<style>
 .overview-account{display:flex;gap:.5rem;flex-wrap:wrap;align-items:center;justify-content:flex-end;margin:0 0 .75rem;font-size:.8rem}
 .overview-account button{font:inherit;padding:.45rem .65rem;border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);color:var(--text);cursor:pointer}
 .admin-page {color:var(--text);}
 aside {position:fixed;inset:0 auto 0 0;width:240px;display:flex;flex-direction:column;background:var(--surface);border-right:1px solid var(--line);z-index:20;}
 .sidebar-heading {padding:24px 18px 18px;display:grid;gap:14px;text-align:left;}
 .sidebar-heading strong {font-size:.85rem;letter-spacing:.08em;}
 .sidebar-heading span {color:var(--text-muted);font-size:.75rem;text-transform:uppercase;}
 .project-list {overflow-y:auto;overscroll-behavior:contain;min-height:0;flex:1;padding:0 10px;}
 .project-list button {display:flex;flex-direction:column;align-items:flex-start;gap:5px;width:100%;text-align:left;margin:3px 0;padding:12px;border:1px solid transparent;border-radius:5px;background:transparent;color:var(--text);font:inherit;font-size:.9rem;cursor:pointer;}
 .project-list button:hover {background:var(--paper-warm);}
 .project-list button.chosen {background:var(--forest);color:white;border-color:var(--forest);font-weight:700;}
 .project-list button span:first-child {overflow-wrap:anywhere;}
 .project-list .all-projects {margin-top:20px;border-top-color:var(--line);}
 .feedback-dot{display:inline-block;flex:none;width:8px;height:8px;margin-left:8px;border-radius:50%;background:#b64d35;box-shadow:0 0 0 2px var(--surface);vertical-align:middle}.retry-updates{font:inherit;font-size:.75rem;padding:5px 8px;border:1px solid var(--line);background:var(--surface);color:var(--text);cursor:pointer}
 .project-name {font-size:1.03rem;font-weight:700;line-height:1.25;}
 .activity {font-size:.69rem;font-style:italic;line-height:1.4;opacity:.8;}
 .sidebar-footer {padding:14px;text-align:left;border-top:1px solid var(--line);font-size:.75rem;}
 .sidebar-footer p {font-size:.75rem;text-align:left;}
 .workspace {margin-left:240px;min-width:0;}
 .admin-header {padding:24px 16px 8px;max-width:1400px;margin:auto;}
 h1 {font-size:1.7rem;text-align:left;margin:0 0 18px;line-height:1.2;}
 .admin-tabs {display:flex;flex-wrap:wrap;gap:8px;border-bottom:1px solid var(--line);padding-bottom:0;}
 .admin-tabs a {background:transparent;border:0;border-bottom:3px solid transparent;color:var(--text-muted);font-size:.92rem;font-weight:700;padding:12px 16px;text-decoration:none;transition:background .15s,color .15s;}.admin-tabs a:hover {background:var(--paper-warm);color:var(--forest);}.admin-tabs a:focus-visible {outline:2px solid var(--forest);outline-offset:2px;}
 .admin-tabs a.active {background:var(--paper-warm);color:var(--forest);border-bottom-color:var(--forest);}

 .project-toggle {display:none;}
 @media(max-width:760px) {
  .project-toggle {display:block;position:sticky;top:0;z-index:30;width:100%;padding:12px;text-align:left;background:var(--forest);color:white;border:0;font:inherit;font-weight:700;}
  aside {display:none;top:48px;width:min(320px,100%);box-shadow:8px 0 20px #0003;}
  aside.mobile-open {display:flex;}
  .workspace {margin-left:0;}
  .admin-header {padding:16px 8px 8px;}
  .admin-tabs a {padding:8px;font-size:.8rem;}
 }
</style>
