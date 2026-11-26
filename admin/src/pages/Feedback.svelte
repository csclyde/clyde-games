<script>
 import {onMount,onDestroy,tick} from "svelte";
 import {canWrite} from "../portal-auth.mjs";
 import ReportNotes from "../components/ReportNotes.svelte";
 import {loadInvestigationStatuses,statusKey} from "../investigation-status.mjs";
 import Panel from '../components/Panel.svelte';
 import ReportActions from '../components/ReportActions.svelte';
 import FeedbackMarkdown from '../components/FeedbackMarkdown.svelte';
 import {feedbackPresentation} from '../feedback-presentation.mjs';
	import PlankaTicketModal from '../components/PlankaTicketModal.svelte';

 let investigations={},investigationGeneration=0;
 async function refreshInvestigations(reports){const generation=++investigationGeneration;const result=await loadInvestigationStatuses('feedback',reports);if(generation===investigationGeneration)investigations=result;}
 function noteChanged(e){const d=e.detail;investigations={...investigations,[`${d.project}:${d.id}`]:d.investigation};refreshInvestigations(feedback);}
 $: refreshInvestigations(feedback);
	export let selectedProject = 'all';
	export let reportProjects = () => {};
 export let reportFeedbackCounts = () => {};
 export let refreshSignal=0;
 export let sourceConfig;
 export let reviewFeedback=()=>{};
 let mounted=false,destroyed=false,request=null,controller,queued=false,waitingUpdates=false,updateError="",feedbackRoot,observer;
 const watched=new Map();

	async function getFeedback(signal) {
		const res = await fetch(`/api/feedback`,{signal,cache:'no-store'});
		const feedback = await res.json();

		if (res.ok) {
			if(!Array.isArray(feedback))throw Error("Feedback response was invalid.");
			return feedback;
		} else {
			if (feedback.error == 'No feedback found') {
				return [];
			}

			throw new Error(feedback.error || feedback);
		}
	}

	let feedback = [];
	let loadingFeedback = true;
	let feedbackError = '';
	let helperStatus = {};
	let resolvingFeedback = {};
	let closeErrors = {};
	let translatingFeedback = {};
	let showingOriginalFeedback = {};
	let syncingSources = false;
	let sourceSyncStatus = '';
	let sourceSyncError = '';
	let searchText = '';


 function hasDraft(){return !!ticketComment||syncingSources||Object.keys(resolvingFeedback).length>0||Object.keys(translatingFeedback).length>0||[...(feedbackRoot?.querySelectorAll('.notes form textarea,.notes form select')||[])].some(e=>e.value.trim())||!!feedbackRoot?.querySelector('.notes button:disabled[type=submit]');}
 async function refreshFeedback(quiet=false){
  if(destroyed)return;
  if(quiet&&(document.visibilityState==='hidden'||hasDraft())){waitingUpdates=true;return;}
  if(request){queued=true;return request;}
  controller=new AbortController();const deadline=setTimeout(()=>controller?.abort(),10000);
  request=(async()=>{try{
   const next=await getFeedback(controller.signal);if(destroyed)return;
   if(quiet&&hasDraft()){waitingUpdates=true;return;}
   feedback=next;feedbackError='';updateError='';waitingUpdates=false;
   await tick();markVisible();
  }catch(e){if(!destroyed){if(loadingFeedback)feedbackError=e.message;else updateError=e.message;}}
  finally{clearTimeout(deadline);request=null;controller=null;if(!destroyed){loadingFeedback=false;await tick();markVisible();if(queued){queued=false;queueMicrotask(()=>refreshFeedback(true));}}}})();
  return request;
 }
 function markVisible(){
  if(destroyed||document.visibilityState==='hidden'||!document.hasFocus()||loadingFeedback||feedbackError)return;
  const byProject=Object.create(null);for(const [node,item] of watched){const r=node.getBoundingClientRect();const area=Math.min(r.width,innerWidth)*Math.min(r.height,innerHeight),visible=Math.max(0,Math.min(innerWidth,r.right)-Math.max(0,r.left))*Math.max(0,Math.min(innerHeight,r.bottom)-Math.max(0,r.top));if(area>0&&visible/area>=.5){const p=(item.Project||'').trim().toLowerCase();(byProject[p]??=[]).push(item.ID);}}
  for(const [p,ids] of Object.entries(byProject))reviewFeedback(p,ids);
 }
 function watchFeedback(node,item){
  watched.set(node,item);if(typeof IntersectionObserver!=='undefined'){observer??=new IntersectionObserver(markVisible,{threshold:[0,.5,1]});observer.observe(node);}
  return {update(item){watched.set(node,item);},destroy(){observer?.unobserve(node);watched.delete(node);}};
 }
 onMount(()=>{mounted=true;refreshFeedback();const catchup=()=>{if(document.visibilityState!=='hidden'){refreshFeedback(true);tick().then(markVisible);}};const visible=()=>{if(document.visibilityState!=='hidden')catchup();};window.addEventListener('focus',catchup);window.addEventListener('online',catchup);window.addEventListener('pageshow',catchup);window.addEventListener('scroll',markVisible,{passive:true,capture:true});document.addEventListener('visibilitychange',visible);return()=>{window.removeEventListener('focus',catchup);window.removeEventListener('online',catchup);window.removeEventListener('pageshow',catchup);window.removeEventListener('scroll',markVisible,true);document.removeEventListener('visibilitychange',visible);};});
 onDestroy(()=>{destroyed=true;controller?.abort();observer?.disconnect();watched.clear();});
 $: if(mounted&&refreshSignal)refreshFeedback(true);
 $: if(mounted&&selectedProject){refreshFeedback(true);tick().then(markVisible);}

	function clearSearch() {
		searchText = '';
	}

	async function syncSource(name, endpoint) {
		const projectQuery = selectedProject && selectedProject !== 'all'
			? `?project=${encodeURIComponent(selectedProject)}`
			: '';
		const res = await fetch(`/api/${endpoint}${projectQuery}`, { method: 'POST' });
		const result = await res.json();

		if (!res.ok) {
			throw new Error(`${name}: ${result.error || result}`);
		}

		return { name, ...result };
	}

	async function syncSources() {
		syncingSources = true;
		sourceSyncError = '';
		if (sourceConfig?.steamSyncEnabled === false) {
			sourceSyncStatus = 'Steam sync is disabled for this project.';
			syncingSources = false;
			return;
		}
		sourceSyncStatus = 'Syncing Steam...';
		try {
			const result = await syncSource('Steam', 'steamreviews/import');
			await refreshFeedback();
			sourceSyncStatus = 'Steam: ' + (result.imported || 0) + ' imported, ' + (result.skipped || 0) + ' skipped';
		} catch (error) {
			sourceSyncStatus = '';
			sourceSyncError = error.message;
		} finally {
			syncingSources = false;
		}
	}

	async function closeFeedback(id, disposition = '') {
		if (!$canWrite || resolvingFeedback[id]) return;
		resolvingFeedback = { ...resolvingFeedback, [id]: disposition || 'ticket' };
		closeErrors = { ...closeErrors, [id]: '' };

		try {
			const url = disposition ? '/api/feedback/close?id=' : '/api/resolvefeedback?id=';
			const res = await fetch(url + encodeURIComponent(id), {
				method: 'POST',
				...(disposition ? { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ disposition }) } : {})
			});
			if (!res.ok) {
				const result = await res.json().catch(() => ({}));
				throw new Error(result.error || 'Feedback could not be closed.');
			}
			feedback = feedback.filter(comment => comment.ID !== id);
			await refreshFeedback();
		} catch (error) {
			closeErrors = { ...closeErrors, [id]: error.message };
		} finally {
			const nextResolvingFeedback = { ...resolvingFeedback };
			delete nextResolvingFeedback[id];
			resolvingFeedback = nextResolvingFeedback;
		}
	}

	function valueOrUnknown(value) {
		return value || 'unknown';
	}

	let ticketComment = null;

	async function openTicketModal(comment) {
		ticketComment = comment;
	}

	function ticketName(comment) {
		const firstLine = (comment.Message || 'Player feedback').split('\n')[0].trim();
		return firstLine.length > 120 ? firstLine.slice(0, 117) + '...' : firstLine;
	}

	function ticketDescription(comment) {
		const details = [
			comment.Message || '', '', '---',
			`Feedback ID: ${comment.ID}`,
			`Project: ${valueOrUnknown(comment.Project)}`,
			`Environment: ${valueOrUnknown(comment.Env)}`,
			`Platform: ${valueOrUnknown(comment.Platform)}`,
			`Category: ${valueOrUnknown(comment.Category)}`,
			`Build: ${valueOrUnknown(comment.Build)}`,
			`Commit: ${valueOrUnknown(comment.Commit)}`,
			`Player ID: ${valueOrUnknown(comment.PID)}`
		];
		if (comment.SavegameID) details.push(`Savegame: https://admin.clyde.games/api/savegame/download?id=${comment.SavegameID}`);
		return details.join('\n');
	}

	async function handleTicketCreated() {
		const comment = ticketComment;
		if (!comment) {
			return;
		}

		helperStatus = { ...helperStatus, [comment.ID]: { status: 'Planka ticket created', error: '' } };
		ticketComment = null;
		await closeFeedback(comment.ID);
	}

	function downloadSavegame(comment) {
		if (!comment.SavegameID) {
			return;
		}

		const link = document.createElement('a');
		link.href = `/api/savegame/download?id=${comment.SavegameID}`;
		link.download = comment.SavegameFilename || 'test.sav';
		link.click();
	}

	function getTranslationApi() {
		if (typeof Translator !== 'undefined') {
			return Translator;
		}

		return null;
	}

	async function detectLanguage(text, detector) {
		if (typeof LanguageDetector === 'undefined') {
			throw new Error('Chrome language detection is not available in this browser.');
		}

		const languageDetector = detector || await LanguageDetector.create();
		const results = await languageDetector.detect(text);

		if (results && results.length > 0 && results[0].detectedLanguage) {
			return results[0].detectedLanguage;
		}

		throw new Error('Could not detect the source language.');
	}

	function getLanguageSamples(text) {
		const samples = [];
		const addSample = sample => {
			const normalized = sample.trim();

			if (normalized.length >= 2 && normalized !== text.trim() && !samples.includes(normalized)) {
				samples.push(normalized);
			}
		};

		// Source labels such as "STEAM community:" commonly make the detector
		// classify an otherwise non-English review as English.
		text.split(/\r?\n/).forEach(line => {
			const colonIndex = line.indexOf(':');

			if (colonIndex >= 0 && colonIndex <= 80) {
				addSample(line.slice(colonIndex + 1));
			}
		});

		text.split(/\r?\n/).forEach(addSample);

		const sentenceMatches = text.match(/[^.!?。！？]+[.!?。！？]?/g) || [];
		sentenceMatches.forEach(addSample);

		return samples.slice(0, 12);
	}

	async function detectSourceLanguage(text) {
		if (typeof LanguageDetector === 'undefined') {
			throw new Error('Chrome language detection is not available in this browser.');
		}

		const detector = await LanguageDetector.create();

		try {
			const detectedLanguage = await detectLanguage(text, detector);

			if (detectedLanguage !== 'en') {
				return detectedLanguage;
			}

			for (const sample of getLanguageSamples(text)) {
				const sampleLanguage = await detectLanguage(sample, detector);

				if (sampleLanguage !== 'en' && sampleLanguage !== 'und') {
					return sampleLanguage;
				}
			}

			return detectedLanguage;
		} finally {
			if (detector.destroy) {
				detector.destroy();
			}
		}
	}

	function getTranslatedText(result) {
		if (typeof result === 'string') {
			return result.trim();
		}

		if (!result) {
			return '';
		}

		const possibleText = result.translation || result.translatedText || result.text;

		if (typeof possibleText === 'string') {
			return possibleText.trim();
		}

		return '';
	}

	function getTranslatableMessage(message) {
		const metadataMatch = message.match(/\s+\((Level:.*,\s*Cursemark:.*)\)\s*$/);

		if (!metadataMatch) {
			return {
				text: message,
				suffix: ''
			};
		}

		return {
			text: message.slice(0, metadataMatch.index).trim(),
			suffix: message.slice(metadataMatch.index)
		};
	}

	function displayedMessage(comment) {
		if (comment.Translated && !showingOriginalFeedback[comment.ID]) {
			return comment.Translated;
		}

		return comment.Markdown || comment.Message || '';
	}

	function toggleOriginal(comment) {
		showingOriginalFeedback = {
			...showingOriginalFeedback,
			[comment.ID]: !showingOriginalFeedback[comment.ID]
		};
	}

	async function translateFeedback(comment) {
		translatingFeedback = { ...translatingFeedback, [comment.ID]: true };
		helperStatus = {
			...helperStatus,
			[comment.ID]: {
				error: '',
				status: 'Translating...'
			}
		};

		try {
			const TranslationApi = getTranslationApi();

			if (!TranslationApi) {
				throw new Error('Chrome translation is not available in this browser.');
			}

			const messageParts = getTranslatableMessage(feedbackPresentation(comment, comment.Message).body);
			const sourceLanguage = await detectSourceLanguage(messageParts.text);

			if (sourceLanguage === 'en') {
				helperStatus = {
					...helperStatus,
					[comment.ID]: {
						error: '',
						status: 'Already English'
					}
				};
				return;
			}

			const options = {
				sourceLanguage,
				targetLanguage: 'en'
			};

			if (TranslationApi.availability) {
				const availability = await TranslationApi.availability(options);

				if (availability === 'unavailable') {
					throw new Error('English translation is not available for this language.');
				}
			}

			const translator = await TranslationApi.create(options);
			const translatedText = getTranslatedText(await translator.translate(messageParts.text));

			if (!translatedText) {
				throw new Error('Chrome translation finished without returning translated text.');
			}

			const translatedMessage = translatedText + messageParts.suffix;
			const res = await fetch(`/api/feedback/translation?id=${comment.ID}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ translated: translatedMessage })
			});
			const result = await res.json();

			if (!res.ok) {
				throw new Error(result.error || 'Could not save the translation.');
			}

			feedback = feedback.map(item => {
				if (item.ID !== comment.ID) {
					return item.Replies ? { ...item, Replies: item.Replies.map(reply => reply.ID === comment.ID ? { ...reply, Translated: result.Translated || translatedMessage } : reply) } : item;
				}

				return {
					...item,
					Translated: result.Translated || translatedMessage
				};
			});
			showingOriginalFeedback = { ...showingOriginalFeedback, [comment.ID]: false };

			helperStatus = {
				...helperStatus,
				[comment.ID]: {
					error: '',
					status: sourceLanguage === 'en' ? 'Already English' : 'Translated to English'
				}
			};
		} catch (error) {
			helperStatus = {
				...helperStatus,
				[comment.ID]: {
					error: error.message,
					status: ''
				}
			};
		} finally {
			const nextTranslatingFeedback = { ...translatingFeedback };
			delete nextTranslatingFeedback[comment.ID];
			translatingFeedback = nextTranslatingFeedback;
		}
	}

	let colors = [
		'var(--charcoal)',
		'var(--rust)',
		'var(--safety)',
		'var(--text-soft)',
		'var(--slate)',
		'var(--forest)'
	]

	function metadataValue(value) {
		return valueOrUnknown(value);
	}

	function formatDate(value) {
		const date = new Date(value);

		if (isNaN(date.getTime())) {
			return value || '-';
		}

		return date.toLocaleString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	function formatBuildDate(value) {
		if (!value) {
			return metadataValue(value);
		}

		return formatDate(value);
	}

	function matchesProject(item, project) {
		return project === 'all' || (item.Project || '').toLowerCase() === project.toLowerCase();
	}

	$: projectFeedback = feedback.filter(item => matchesProject(item, selectedProject));
	$: normalizedSearch = searchText.trim().toLowerCase();
	$: visibleFeedback = normalizedSearch
		? projectFeedback.filter(comment => [comment, ...(comment.Replies || [])].some(post => `${post.Message || ''}\n${post.Translated || ''}`.toLowerCase().includes(normalizedSearch))).slice(0, 100)
		: projectFeedback;
	$: if (!loadingFeedback && !feedbackError) reportFeedbackCounts(feedback);
	$: reportProjects('feedback', feedback.map(comment => comment.Project));
</script>

<main bind:this={feedbackRoot}>
 {#if waitingUpdates}<p class="live-update" role="status">Feedback updates are waiting. Finish the current edit, then load updates. <button type="button" on:click={()=>refreshFeedback(true)}>Load updates</button></p>{/if}
 {#if updateError}<p class="live-update" role="status">Feedback could not be refreshed. Showing the last loaded items. <button type="button" on:click={()=>refreshFeedback(true)}>Retry updates</button></p>{/if}
	<header class="page-header">
		<div>

			<p>{visibleFeedback.length} open item{visibleFeedback.length === 1 ? '' : 's'}</p>
		</div>
		<div class="header-actions">
			<button type="button" disabled={!$canWrite || syncingSources} on:click={syncSources}>
				{syncingSources ? 'Syncing...' : 'Sync Sources'}
			</button>
			{#if sourceSyncStatus}
				<small>{sourceSyncStatus}</small>
			{/if}
			{#if sourceSyncError}
				<small class="helper-error">{sourceSyncError}</small>
			{/if}
		</div>
	</header>

	<Panel inset={false}><div class="search">

		<div class="search-controls">
			<input id="feedback-search" aria-label="Search feedback" type="search" bind:value={searchText} placeholder="Search player feedback…" autocomplete="off" />
			{#if searchText}<button class="clear-search" type="button" on:click={clearSearch}>Clear</button>{/if}
		</div>
		{#if normalizedSearch}<small>Showing up to 100 matching visible items.</small>{/if}
	</div>

 </Panel>
	{#if loadingFeedback}
		<p class="state-message">loading...</p>
	{:else if feedbackError}
		<p class="state-message error">{feedbackError}</p>
	{:else if visibleFeedback.length === 0}
		<p class="state-message">{normalizedSearch ? 'No matching feedback.' : 'No open feedback.'}</p>
	{:else}
		<div class="comment-list">
		{#each visibleFeedback as comment (comment.ID)}
 {@const presentation = feedbackPresentation(comment, displayedMessage(comment))}
			<Panel inset={false} class={resolvingFeedback[comment.ID] ? "comment resolving" : "comment"}>
				<div class="comment-body" data-feedback-id={comment.ID} use:watchFeedback={comment}>
					<div class="comment-main">
						<div class="message-header">
							<span class="rating" style="background-color:{ colors[comment.Rating] }"></span>
							<div class="message-stack">
								<FeedbackMarkdown message={presentation.body} discord={presentation.discord}/>
								{#if comment.Translated}
									<button class="original-toggle" type="button" on:click={() => toggleOriginal(comment)}>
										{showingOriginalFeedback[comment.ID] ? 'See Translation' : 'See Original'}
									</button>
								{/if}
								{#if helperStatus[comment.ID] && helperStatus[comment.ID].status}
									<small class="helper-status">{helperStatus[comment.ID].status}</small>
								{/if}
								{#if helperStatus[comment.ID] && helperStatus[comment.ID].error}
									<small class="helper-error">{helperStatus[comment.ID].error}</small>
								{/if}
							</div>
						</div>
 <ReportNotes kind="feedback" id={comment.ID} project={comment.Project} investigation={investigations[statusKey(comment)]} compactHeader on:changed={noteChanged}/>
						<div class="meta-list">
							{#if comment.Project}<span class="project-tag">project: {comment.Project}</span>{/if}
							{#if comment.Env}<span>env: {comment.Env}</span>{/if}
							{#if comment.Platform}<span>platform: {comment.Platform}</span>{/if}
							{#if presentation.category}<span>category: {presentation.category}</span>{/if}
							{#if comment.PID}
								<a class="user-id" href={`/user/${encodeURIComponent(comment.PID)}`}>user: {presentation.username}</a>
							{/if}
							{#if comment.Commit}
								<span>commit: {comment.Commit}</span>
							{/if}
						</div>
					</div>
					<div class="comment-side">
						<div class="stats">
							<p><span>Created</span>{formatDate(comment.CreatedAt)}</p>
							<p><span>Build</span>{formatBuildDate(comment.Build)}</p>
						</div>
						<ReportActions label="Feedback actions">
							<svelte:fragment slot="primary">
								<button class="resolve-action" type="button" disabled={!$canWrite || resolvingFeedback[comment.ID]} on:click={() => closeFeedback(comment.ID, 'resolved')}>
									{resolvingFeedback[comment.ID] === 'resolved' ? 'Resolving...' : 'Resolve'}
								</button>
								<button class="ignore-action" type="button" disabled={!$canWrite || resolvingFeedback[comment.ID]} on:click={() => closeFeedback(comment.ID, 'ignored')}>
									{resolvingFeedback[comment.ID] === 'ignored' ? 'Ignoring...' : 'Ignore'}
								</button>
							</svelte:fragment>
							<svelte:fragment slot="secondary">
								<button type="button" disabled={!$canWrite || translatingFeedback[comment.ID] || resolvingFeedback[comment.ID]} on:click={() => translateFeedback(comment)}>
									{translatingFeedback[comment.ID] ? 'Translating...' : 'Translate'}
								</button>
								<button type="button" disabled={!$canWrite || resolvingFeedback[comment.ID]} on:click={() => openTicketModal(comment)}>
									Make Ticket
								</button>
								<button type="button" disabled={!comment.SavegameID || resolvingFeedback[comment.ID]} on:click={() => downloadSavegame(comment)}>
									Download Save
								</button>
							</svelte:fragment>
						</ReportActions>
						{#if closeErrors[comment.ID]}<p class="state-message error" role="alert">{closeErrors[comment.ID]}</p>{/if}
					</div>
				</div>
				{#if comment.Replies && comment.Replies.length}
					<section class="discussion-replies" aria-label="Discussion replies">
						<h3>{comment.Replies.length} {comment.Replies.length === 1 ? 'reply' : 'replies'}</h3>
						{#each comment.Replies as reply (reply.ID)}
							<article class="discussion-reply">
								<header><strong>{reply.PID || 'Steam user'}</strong><time datetime={reply.CreatedAt}>{formatDate(reply.CreatedAt)}</time></header>
								<FeedbackMarkdown message={displayedMessage(reply).replace(/^Steam discussion: [^\n]*\n\n/, '')} />
								<button class="original-toggle" type="button" disabled={!$canWrite && !reply.Translated || translatingFeedback[reply.ID]} on:click={() => reply.Translated ? toggleOriginal(reply) : translateFeedback(reply)}>
									{translatingFeedback[reply.ID] ? 'Translating...' : reply.Translated ? (showingOriginalFeedback[reply.ID] ? 'See Translation' : 'See Original') : 'Translate'}
								</button>
								{#if helperStatus[reply.ID] && helperStatus[reply.ID].error}<small class="helper-error">{helperStatus[reply.ID].error}</small>{/if}
							</article>
						{/each}
					</section>
				{/if}
			</Panel>
		{/each}
		</div>
	{/if}

</main>

<PlankaTicketModal
	open={!!ticketComment}
	title={ticketComment ? ticketName(ticketComment) : ''}
	description={ticketComment ? ticketDescription(ticketComment) : ''}
	on:close={() => (ticketComment = null)}
	on:created={handleTicketCreated}
/>

<style>
	.discussion-replies { border-top: 1px solid #56616d; margin-top: 1rem; padding-top: .6rem; }
	.discussion-replies h3 { font-size: .85rem; margin: 0 0 .5rem; }
	.discussion-reply { margin: .6rem 0 .6rem .4rem; border-left: 2px solid #56616d; padding: .35rem .65rem; overflow-wrap: anywhere; }
	.discussion-reply header { display: flex; flex-wrap: wrap; gap: .4rem .8rem; font-size: .82rem; margin-bottom: .3rem; }
	main {
		color: var(--text);
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.page-header {
		align-items: flex-start;
		display: flex;
		gap: 18px;
		justify-content: space-between;
		margin-bottom: 14px;
		padding-bottom: 12px;
	}



	.page-header p {
		color: var(--olive);
		font-size: 0.85rem;
		font-weight: 600;
		letter-spacing: 0.02em;
		margin: 6px 0 0;
		text-align: left;
		text-transform: uppercase;
	}

	.header-actions {
		align-items: flex-end;
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding-top: 4px;
		text-align: right;
	}

	.header-actions small {
		max-width: 260px;
		text-align: right;
	}

	.search { min-width:0; }

	.search-controls { display: flex; gap: 6px; }
	.search input { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; color: var(--text); flex: 1; font: inherit; min-width: 0; padding: 7px 10px; }
	.search input:focus { border-color: var(--charcoal); outline: 2px solid transparent; }
	.search small { color: var(--text-muted); display: block; margin-top: 7px; }
	.clear-search { color: var(--rust); }

	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
 margin-top:12px;
	}



	:global(.comment.resolving) {
		opacity: 0.45;
		pointer-events: none;
	}

	.comment-body {
		align-items: stretch;
		display: grid;
		gap: 18px;
		grid-template-columns: minmax(0, 1fr) minmax(320px, auto);
	}

	.comment-main {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
	}

	.comment-side {
		align-items: flex-end;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.stats {
		display: grid;
		gap: 12px;
		grid-template-columns: repeat(2, minmax(90px, max-content));
		justify-content: end;
	}

	.stats p {
		color: var(--text-muted);
		display: grid;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		margin: 0;
		text-align: left;
	}

	.stats span {
		color: var(--text-soft);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.message-header {
		align-items: flex-start;
		display: flex;
		gap: 12px;
	}

	.message-stack {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
	}

	.meta-list {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.meta-list span {
		background: var(--paper-warm);
		border: 1px solid var(--line);
		border-radius: 4px;
		color: var(--text-muted);
		font-size: 0.68rem;
		line-height: 1.25;
		max-width: 100%;
		overflow-wrap: anywhere;
		padding: 3px 6px;
		text-align: left;
	}

	.meta-list .project-tag {
		background: var(--surface);
		border-color: var(--forest);
		color: var(--charcoal);
	}

	.user-id {
		background: var(--paper-warm);
		border: 1px solid var(--line);
		border-radius: 4px;
		color: var(--text-muted);
		font-size: 0.68rem;
		line-height: 1.25;
		max-width: 100%;
		overflow-wrap: anywhere;
		padding: 3px 6px;
		text-align: left;
		text-decoration: none;
	}

	.user-id:hover {
		background: var(--surface);
		border-color: var(--charcoal);
		color: var(--text);
	}

	:global(.comment) p {
		margin: 0;
	}

	.rating {
		border-radius: 50%;
		box-shadow: 0 0 0 2px var(--surface), 0 0 0 3px var(--line);
		flex-shrink: 0;
		height: 14px;
		margin-top: 5px;
		width: 14px;
	}

	.helper-status {
		color: var(--text-soft);
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.original-toggle {
		align-self: flex-start;
		background: none;
		border: 0;
		color: var(--text-muted);
		font-size: 0.68rem;
		font-weight: 600;
		padding: 0;
		text-decoration: underline;
	}

	.original-toggle:hover:not(:disabled) {
		background: none;
		border-color: transparent;
		color: var(--text);
	}

	.helper-error {
		color: var(--rust);
		max-width: 520px;
	}

	button {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: 4px;
		box-sizing: border-box;
		color: var(--text);
		cursor: pointer;
		font: inherit;
		font-size: 0.82rem;
		font-weight: 700;
		line-height: 1;
		padding: 7px 10px;
	}

	button:hover:not(:disabled) {
		background: var(--paper-warm);
		border-color: var(--charcoal);
	}

	button:disabled {
		color: var(--text-soft);
		cursor: default;
	}

	small {
		font-size: 0.68rem;
		line-height: 1.3;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.state-message {
		color: var(--text-soft);
		margin: 32px 0;
		text-align: left;
	}

	.error {
		color: var(--rust);
	}

	@media (max-width: 820px) {
		main {
			padding: 8px 4px 24px;
		}



		.page-header {
			align-items: flex-start;
			flex-direction: column;
		}

		.header-actions {
			align-items: flex-start;
			text-align: left;
		}

		.header-actions small {
			text-align: left;
		}

		.search-controls { flex-wrap: wrap; }
		.search input { flex-basis: 100%; }

		.comment-list {
			gap: 12px;
		}



		.comment-body {
			grid-template-columns: 1fr;
		}

		.message-header {
			gap: 10px;
		}

		.message-stack {
			width: 100%;
		}

		.comment-side {
			align-items: flex-start;
			width: 100%;
		}

		.stats {
			grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
			justify-content: stretch;
			width: 100%;
		}

	}

	@media (max-width: 480px) {
		.stats {
			grid-template-columns: 1fr;
		}
	}

</style>
