<script>
	export let userID = '';

	let loadedUserID = '';
	let loading = false;
	let error = '';
	let history = { Feedback: [], Crashes: [], Savegames: [], Events: [] };

	async function loadUser(id) {
		loadedUserID = id;
		loading = true;
		error = '';

		try {
			const response = await fetch(`https://api.clyde.games/user/${encodeURIComponent(id)}`);
			const result = await response.json();
			if (!response.ok) throw new Error(result.error || result);
			if (loadedUserID === id) history = result;
		} catch (loadError) {
			if (loadedUserID === id) error = loadError.message;
		} finally {
			if (loadedUserID === id) loading = false;
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

	function context(item) {
		return [item.Project, item.Env, item.Platform, item.Category].filter(Boolean).join(' / ');
	}

	function makeTimeline(data) {
		return [
			...(data.Feedback || []).map(item => ({ kind: 'Feedback', item })),
			...(data.Crashes || []).map(item => ({ kind: 'Crash', item })),
			...(data.Savegames || []).map(item => ({ kind: 'Save game', item })),
			...(data.Events || []).map(item => ({ kind: 'Event', item }))
		].sort((a, b) => new Date(eventDate(a.item)) - new Date(eventDate(b.item)));
	}

	function downloadSavegame(item) {
		const link = document.createElement('a');
		link.href = `https://api.clyde.games/savegame/download?id=${item.ID}`;
		link.download = item.Filename || 'test.sav';
		document.body.appendChild(link);
		link.click();
		link.remove();
	}

	$: if (userID && userID !== loadedUserID) loadUser(userID);
	$: timeline = makeTimeline(history);
</script>

<main>
	<header class="page-header">
		<div>
			<p class="eyebrow">User history</p>
			<h2>{userID}</h2>
			<p>{timeline.length} item{timeline.length === 1 ? '' : 's'} · oldest to newest</p>
		</div>
	</header>

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
							{#if context(entry.item)}<span>{context(entry.item)}</span>{/if}
							{#if entry.item.Build}<span>{entry.item.Build}</span>{/if}
							{#if entry.item.Commit}<span>{entry.item.Commit}</span>{/if}
							{#if entry.item.SID}<span>Session {entry.item.SID}</span>{/if}
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
	.page-header { border-bottom: 1px solid var(--line); margin-bottom: 14px; padding-bottom: 12px; }
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
	time { color: var(--text-soft); font-size: .75rem; }
	.title { color: var(--charcoal); font-size: 1rem; line-height: 1.4; margin: 0; overflow-wrap: anywhere; text-align: left; white-space: pre-wrap; }
	.detail { color: var(--text-muted); font-size: .85rem; line-height: 1.35; margin: 0; max-height: 8.1em; overflow: hidden; overflow-wrap: anywhere; text-align: left; white-space: pre-wrap; }
	.meta-list { display: flex; flex-wrap: wrap; gap: 6px; }
	.meta-list span { background: var(--paper-warm); border: 1px solid var(--line); border-radius: 4px; color: var(--text-muted); font-size: .68rem; line-height: 1.25; max-width: 100%; overflow-wrap: anywhere; padding: 3px 6px; text-align: left; }
	button { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; color: var(--text); cursor: pointer; font: inherit; font-size: .82rem; font-weight: 700; line-height: 1; padding: 7px 10px; }
	button:hover { background: var(--paper-warm); border-color: var(--charcoal); }
	.state-message { color: var(--text-soft); margin: 32px 0; text-align: left; }
	.error { color: var(--rust); }
	@media (max-width: 820px) {
		main { padding: 8px 4px 24px; }
		.page-header h2 { font-size: 2rem; }
		.history-item { border: 1px solid var(--line-strong); border-top: 4px solid var(--forest); border-radius: 6px; flex-direction: column; padding: 12px; }
		.history-item.crash { border-top-color: var(--rust); }
		.history-item.savegame { border-top-color: var(--safety); }
		.history-item.event { border-top-color: var(--slate); }
	}
</style>
