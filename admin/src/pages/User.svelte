<script>
 import {canWrite} from "../portal-auth.mjs";
	export let userID = '';
 export let selectedProject = 'cursemark';

	let loadedUserID = '';
	let loading = false;
	let blocking = false;
	let updatingFeedbackBlock = false;
	let blockError = '';
	let error = '';
	let history = { FeedbackBlocked: false, Blocked: false, Feedback: [], Crashes: [], Savegames: [], Events: [] };

	async function loadUser(id) {
		loadedUserID = id;
		loading = true;
		error = '';
		blockError = '';

		try {
			const response = await fetch(`/api/user/${encodeURIComponent(id)}`);
			const result = await response.json();
			if (!response.ok) throw new Error(result.error || result);
			if (loadedUserID === id) history = result;
		} catch (loadError) {
			if (loadedUserID === id) error = loadError.message;
		} finally {
			if (loadedUserID === id) loading = false;
		}
	}

	async function blockUser() {
		if (!confirm(`Block ${userID}? Their metrics events and ordinary save games will be permanently deleted. Feedback attachments will be retained.`)) return;
		blocking = true;
		error = '';
		try {
			const response = await fetch(`/api/user/${encodeURIComponent(userID)}/block`, { method: 'POST' });
			const result = await response.json();
			if (!response.ok) throw new Error(result.error || result);
			history = { ...history, Blocked: true, Savegames: history.Savegames.filter(item => item.FeedbackID > 0), Events: [] };
		} catch (blockError) {
			error = blockError.message;
		} finally {
			blocking = false;
		}
	}

	async function toggleFeedbackBlock() {
		const id = userID;
		const blocked = !history.FeedbackBlocked;
		if (blocked && !confirm('Block future in-game feedback from this player ID across all projects? Past reports and telemetry will be kept.')) return;
		updatingFeedbackBlock = true;
		blockError = '';
		try {
			const response = await fetch(`/api/user/${encodeURIComponent(id)}/feedback-block`, { method: blocked ? 'PUT' : 'DELETE' });
			if (!response.ok) {
				const result = await response.json().catch(() => ({}));
				throw new Error(result.error || 'Could not update the feedback block.');
			}
			const result = await response.json();
			if (loadedUserID === id && userID === id) history = { ...history, FeedbackBlocked: result.feedbackBlocked };
		} catch (requestError) {
			if (loadedUserID === id && userID === id) blockError = requestError.message;
		} finally {
			updatingFeedbackBlock = false;
		}
	}

	function eventDate(item) {
		return item.CreatedAt || item.UpdatedAt;
	}

	function formatDate(value) {
		const date = new Date(value);
		if (isNaN(date.getTime())) return value || '-';
		return date.toLocaleString(undefined, {
			year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit'
		});
	}

	function makeTimeline(data) {
		const entries = [
			...(data.Feedback || []).map(item => ({ kind: 'Feedback', item })),
			...(data.Crashes || []).map(item => ({ kind: 'Crash', item })),
			...(data.Savegames || []).map(item => ({ kind: 'Save game', item })),
			...(data.Events || []).map(item => ({ kind: 'Event', item }))
		].sort((a, b) => new Date(eventDate(b.item)) - new Date(eventDate(a.item))
			|| Number(b.item.ID || 0) - Number(a.item.ID || 0)
			|| a.kind.localeCompare(b.kind));

		return entries.reduce((timeline, entry) => {
			const previous = timeline[timeline.length - 1];
			if (entry.kind === 'Event' && previous?.kind === 'Event' && previous.item.Type === entry.item.Type) {
				previous.count += 1;
				return timeline;
			}

			timeline.push({ ...entry, count: 1 });
			return timeline;
		}, []);
	}

	function historyItemCount(data) {
		return (data.Feedback || []).length
			+ (data.Crashes || []).length
			+ (data.Savegames || []).length
			+ (data.Events || []).length;
	}

	function downloadSavegame(item) {
		const link = document.createElement('a');
		link.href = `/api/savegame/download?id=${item.ID}`;
		link.download = item.Filename || 'test.sav';
		document.body.appendChild(link);
		link.click();
		link.remove();
	}

	$: if (userID && userID !== loadedUserID) loadUser(userID);
	$: scopedHistory = Object.fromEntries(Object.entries(history).map(([k,v]) => [k, Array.isArray(v) ? v.filter(i => selectedProject === 'all' || (i.Project || '').toLowerCase() === selectedProject.toLowerCase()) : v]));
	$: timeline = makeTimeline(scopedHistory);
	$: itemCount = historyItemCount(scopedHistory);
</script>

<main>
	<header class="page-header">
		<div>
			<p class="eyebrow">User history</p>
			<h2>{userID}</h2>
			<p>{itemCount} item{itemCount === 1 ? '' : 's'} · {timeline.length} shown · newest to oldest</p>
		</div>
		<div class="block-actions">
			{#if !loading && !error}
				{#if history.FeedbackBlocked}<span class="blocked-badge">In-game feedback blocked</span>{/if}
				{#if $canWrite}
					<button class:danger={!history.FeedbackBlocked} type="button" disabled={updatingFeedbackBlock} on:click={toggleFeedbackBlock}>
						{updatingFeedbackBlock ? 'Saving...' : history.FeedbackBlocked ? 'Unblock feedback' : 'Block feedback only'}
					</button>
				{/if}
			{/if}
			{#if history.Blocked}
				<span class="blocked-badge">Telemetry blocked</span>
			{:else}
				<button class="danger" type="button" disabled={!$canWrite || loading || !!error || blocking} on:click={blockUser}>{blocking ? 'Blocking...' : 'Block telemetry across all projects'}</button>
			{/if}
		</div>
	</header>
	{#if blockError}<p class="error" role="alert">{blockError}</p>{/if}

	{#if loading}
		<p class="state-message">loading...</p>
	{:else if error}
		<p class="state-message error">{error}</p>
	{:else if timeline.length === 0}
		<p class="state-message">No history found for this user.</p>
	{:else}
		<div class="timeline">
			{#each timeline as entry}
				<article class="history-item" class:crash={entry.kind === 'Crash'} class:savegame={entry.kind === 'Save game'} class:event={entry.kind === 'Event'}>
					<div class="item-main">
						<div class="item-heading">
							<span class="kind">{entry.kind}</span>
							{#if entry.count > 1}<span class="event-count" title={`${entry.count} consecutive events`}>×{entry.count}</span>{/if}
							<time datetime={eventDate(entry.item)}>{formatDate(eventDate(entry.item))}</time>
						</div>

						{#if entry.kind === 'Feedback'}
							<p class="title">{entry.item.Message}</p>
						{:else if entry.kind === 'Crash'}
							<p class="title">{entry.item.Message || 'Crash report'}</p>
							{#if entry.item.Stack}<p class="detail">{entry.item.Stack}</p>{/if}
						{:else if entry.kind === 'Save game'}
							<p class="title">{entry.item.Filename || 'test.sav'}</p>
							{#if entry.item.Reason}<p class="detail">{entry.item.Reason}</p>{/if}
						{:else}
							<p class="title">{entry.item.Type || 'Telemetry event'}</p>
							{#if entry.item.Level}<p class="detail">Level: {entry.item.Level}</p>{/if}
						{/if}

						<div class="meta-list">
							{#if entry.item.Project}<span class="project-tag">project: {entry.item.Project}</span>{/if}
							{#if entry.item.Env}<span>env: {entry.item.Env}</span>{/if}
							{#if entry.item.Platform}<span>platform: {entry.item.Platform}</span>{/if}
							{#if entry.item.Category}<span>category: {entry.item.Category}</span>{/if}
							{#if entry.item.Build}<span>build: {entry.item.Build}</span>{/if}
							{#if entry.item.Commit}<span>commit: {entry.item.Commit}</span>{/if}
							{#if entry.item.SID}<span>session: {entry.item.SID}</span>{/if}
							{#if entry.kind === 'Crash' && entry.item.Count}<span>{entry.item.Count} occurrence{entry.item.Count === 1 ? '' : 's'}</span>{/if}
							{#if entry.item.Resolved}<span>Resolved</span>{/if}
						</div>
					</div>
					{#if entry.kind === 'Save game'}
						<button type="button" on:click={() => downloadSavegame(entry.item)}>Download</button>
					{/if}
				</article>
			{/each}
		</div>
	{/if}
</main>

<style>
	main { color: var(--text); max-width: 1400px; margin: 0 auto; padding: 12px 16px 32px; }
	.page-header { align-items: center; border-bottom: 1px solid var(--line); display: flex; gap: 16px; justify-content: space-between; margin-bottom: 14px; padding-bottom: 12px; }
	.block-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; justify-content: flex-end; }
	.page-header h2 { font-size: 2.5rem; line-height: 1.05; overflow-wrap: anywhere; text-align: left; }
	.page-header p { color: var(--olive); font-size: .85rem; font-weight: 600; letter-spacing: .02em; margin: 6px 0 0; text-align: left; text-transform: uppercase; }
	.page-header .eyebrow { color: var(--text-soft); font-size: .68rem; font-weight: 800; margin: 0 0 5px; }
	.timeline { display: flex; flex-direction: column; gap: 10px; }
	.history-item { align-items: flex-start; background: var(--surface); border: 1px solid var(--line); border-left: 4px solid var(--forest); border-radius: var(--radius); display: flex; gap: 18px; justify-content: space-between; padding: 14px 16px; }
	.history-item.crash { border-left-color: var(--rust); }
	.history-item.savegame { border-left-color: var(--safety); }
	.history-item.event { border-left-color: var(--slate); }
	.item-main { display: flex; flex: 1; flex-direction: column; gap: 9px; min-width: 0; }
	.item-heading { align-items: center; display: flex; flex-wrap: wrap; gap: 8px; }
	.kind { color: var(--charcoal); font-size: .75rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
	.event-count { background: var(--slate); border-radius: 999px; color: var(--surface); font-size: .68rem; font-weight: 800; line-height: 1; padding: 4px 7px; }
	time { color: var(--text-soft); font-size: .75rem; }
	.title { color: var(--charcoal); font-size: 1rem; line-height: 1.4; margin: 0; overflow-wrap: anywhere; text-align: left; white-space: pre-wrap; }
	.detail { color: var(--text-muted); font-size: .85rem; line-height: 1.35; margin: 0; max-height: 8.1em; overflow: hidden; overflow-wrap: anywhere; text-align: left; white-space: pre-wrap; }
	.meta-list { display: flex; flex-wrap: wrap; gap: 6px; }
	.meta-list span { background: var(--paper-warm); border: 1px solid var(--line); border-radius: 4px; color: var(--text-muted); font-size: .68rem; line-height: 1.25; max-width: 100%; overflow-wrap: anywhere; padding: 3px 6px; text-align: left; }
	.meta-list .project-tag { background: var(--surface); border-color: var(--forest); color: var(--charcoal); }
	button { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; color: var(--text); cursor: pointer; font: inherit; font-size: .82rem; font-weight: 700; line-height: 1; padding: 7px 10px; }
	button:hover { background: var(--paper-warm); border-color: var(--charcoal); }
	button:disabled { cursor: wait; opacity: .6; }
	button.danger { border-color: var(--rust); color: var(--rust); }
	button.danger:hover { background: var(--rust); color: var(--surface); }
	.blocked-badge { background: var(--rust); border-radius: 4px; color: var(--surface); font-size: .75rem; font-weight: 800; padding: 7px 10px; text-transform: uppercase; }
	.state-message { color: var(--text-soft); margin: 32px 0; text-align: left; }
	.error { color: var(--rust); }
	@media (max-width: 820px) {
		main { padding: 8px 4px 24px; }
		.page-header { align-items: flex-start; flex-direction: column; }
		.block-actions { justify-content: flex-start; }
		.page-header h2 { font-size: 2rem; }
		.history-item { border: 1px solid var(--line-strong); border-top: 4px solid var(--forest); border-radius: 6px; flex-direction: column; padding: 12px; }
		.history-item.crash { border-top-color: var(--rust); }
		.history-item.savegame { border-top-color: var(--safety); }
		.history-item.event { border-top-color: var(--slate); }
	}
</style>
