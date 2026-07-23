<script>
	import PlankaTicketModal from '../components/PlankaTicketModal.svelte';

	export let selectedProject = 'cursemark';
	export let reportProjects = () => {};

	async function getFeedback() {
		const res = await fetch(`https://api.clyde.games/feedback`);
		const feedback = await res.json();

		if (res.ok) {
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
	let translatingFeedback = {};
	let showingOriginalFeedback = {};
	let syncingSources = false;
	let sourceSyncStatus = '';
	let sourceSyncError = '';
	let searchText = '';

	async function loadFeedback() {
		loadingFeedback = true;
		feedbackError = '';

		try {
			feedback = await getFeedback();
		} catch (error) {
			feedbackError = error.message;
		} finally {
			loadingFeedback = false;
		}
	}

	loadFeedback();

	async function refreshFeedback() {
		try {
			feedback = await getFeedback();
			feedbackError = '';
		} catch (error) {
			feedbackError = error.message;
		}
	}

	function clearSearch() {
		searchText = '';
	}

	async function syncSource(name, endpoint) {
		const projectQuery = selectedProject && selectedProject !== 'all'
			? `?project=${encodeURIComponent(selectedProject)}`
			: '';
		const res = await fetch(`https://api.clyde.games/${endpoint}${projectQuery}`, { method: 'POST' });
		const result = await res.json();

		if (!res.ok) {
			throw new Error(`${name}: ${result.error || result}`);
		}

		return { name, ...result };
	}

	async function syncSources() {
		syncingSources = true;
		sourceSyncStatus = 'Syncing Steam and Reddit...';
		sourceSyncError = '';

		const results = await Promise.allSettled([
			syncSource('Steam', 'steamreviews/import'),
			syncSource('Reddit', 'reddit/import')
		]);
		const synced = results.filter(result => result.status === 'fulfilled').map(result => result.value);
		const failed = results.filter(result => result.status === 'rejected').map(result => result.reason.message);

		if (synced.length > 0) {
			await refreshFeedback();
			sourceSyncStatus = synced
				.map(result => `${result.name}: ${result.imported || 0} imported, ${result.skipped || 0} skipped`)
				.join(' · ');
		} else {
			sourceSyncStatus = '';
		}
		sourceSyncError = failed.join(' · ');
		syncingSources = false;
	}

	async function resolveFeedback(id) {
		resolvingFeedback = { ...resolvingFeedback, [id]: true };

		try {
			const res = await fetch(`https://api.clyde.games/resolvefeedback?id=` + id, {
				method: 'GET'
			});

			if (res.ok) {
				feedback = feedback.filter(comment => comment.ID !== id);
				await refreshFeedback();
			}
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
		if (comment.SavegameID) details.push(`Savegame: https://api.clyde.games/savegame/download?id=${comment.SavegameID}`);
		return details.join('\n');
	}

	async function handleTicketCreated() {
		const comment = ticketComment;
		if (!comment) {
			return;
		}

		helperStatus = { ...helperStatus, [comment.ID]: { status: 'Planka ticket created', error: '' } };
		ticketComment = null;
		await resolveFeedback(comment.ID);
	}

	function downloadSavegame(comment) {
		if (!comment.SavegameID) {
			return;
		}

		const link = document.createElement('a');
		link.href = `https://api.clyde.games/savegame/download?id=${comment.SavegameID}`;
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

	function messageSegments(message) {
		return message.split(/(https?:\/\/[^\s]+)/g).filter(Boolean).map(part => {
			return {
				text: part,
				isURL: /^https?:\/\//.test(part)
			};
		});
	}

	function displayedMessage(comment) {
		if (comment.Translated && !showingOriginalFeedback[comment.ID]) {
			return comment.Translated;
		}

		return comment.Message;
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

			const messageParts = getTranslatableMessage(comment.Message);
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
			const res = await fetch(`https://api.clyde.games/feedback/translation?id=${comment.ID}`, {
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
					return item;
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

	function matchesProject(item) {
		return selectedProject === 'all' || (item.Project || '').toLowerCase() === selectedProject.toLowerCase();
	}

	$: projectFeedback = feedback.filter(matchesProject);
	$: normalizedSearch = searchText.trim().toLowerCase();
	$: visibleFeedback = normalizedSearch
		? projectFeedback.filter(comment => `${comment.Message || ''}\n${comment.Translated || ''}`.toLowerCase().includes(normalizedSearch)).slice(0, 100)
		: projectFeedback;
	$: reportProjects('feedback', feedback.map(comment => comment.Project));
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Player Feedback</h2>
			<p>{visibleFeedback.length} open item{visibleFeedback.length === 1 ? '' : 's'}</p>
		</div>
		<div class="header-actions">
			<button type="button" disabled={syncingSources} on:click={syncSources}>
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

	<div class="search">
		<label for="feedback-search">Search message text</label>
		<div class="search-controls">
			<input id="feedback-search" type="search" bind:value={searchText} placeholder="Search player feedback…" autocomplete="off" />
			{#if searchText}<button class="clear-search" type="button" on:click={clearSearch}>Clear</button>{/if}
		</div>
		{#if normalizedSearch}<small>Showing up to 100 matching visible items.</small>{/if}
	</div>

	{#if loadingFeedback}
		<p class="state-message">loading...</p>
	{:else if feedbackError}
		<p class="state-message error">{feedbackError}</p>
	{:else if visibleFeedback.length === 0}
		<p class="state-message">{normalizedSearch ? 'No matching feedback.' : 'No open feedback.'}</p>
	{:else}
		<div class="comment-list">
		{#each visibleFeedback as comment}
			<div class:resolving={resolvingFeedback[comment.ID]} class="comment">
				<div class="comment-body">
					<div class="comment-main">
						<div class="message-header">
							<span class="rating" style="background-color:{ colors[comment.Rating] }"></span>
							<div class="message-stack">
								<p class="message">
									{#each messageSegments(displayedMessage(comment)) as segment}
										{#if segment.isURL}
											<a target="_blank" rel="noreferrer" href={segment.text}>{segment.text}</a>
										{:else}
											{segment.text}
										{/if}
									{/each}
								</p>
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
						<div class="meta-list">
							{#if comment.Project}<span>project: {comment.Project}</span>{/if}
							{#if comment.Env}<span>env: {comment.Env}</span>{/if}
							{#if comment.Platform}<span>platform: {comment.Platform}</span>{/if}
							{#if comment.Category}<span>category: {comment.Category}</span>{/if}
							{#if comment.PID}
								<a class="user-id" href={`/user/${encodeURIComponent(comment.PID)}`}>user: {comment.PID}</a>
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
						<div class="actions">
							<button type="button" disabled={translatingFeedback[comment.ID] || resolvingFeedback[comment.ID]} on:click={() => translateFeedback(comment)}>
								{translatingFeedback[comment.ID] ? 'Translating...' : 'Translate'}
							</button>
							<button type="button" disabled={resolvingFeedback[comment.ID]} on:click={() => openTicketModal(comment)}>
								Make Ticket
							</button>
							<button type="button" disabled={!comment.SavegameID || resolvingFeedback[comment.ID]} on:click={() => downloadSavegame(comment)}>
								Download Save
							</button>
							<button type="button" disabled={resolvingFeedback[comment.ID]} on:click={() => resolveFeedback(comment.ID)}>
								{resolvingFeedback[comment.ID] ? 'Resolving...' : 'Resolve'}
							</button>
						</div>
					</div>
				</div>
			</div>
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
	main {
		color: var(--text);
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.page-header {
		align-items: flex-start;
		border-bottom: 1px solid var(--line);
		display: flex;
		gap: 18px;
		justify-content: space-between;
		margin-bottom: 14px;
		padding-bottom: 12px;
	}

	.page-header h2 {
		font-size: 2.5rem;
		line-height: 1;
		text-align: left;
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

	.search { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); margin-bottom: 14px; padding: 12px; }
	.search label { color: var(--text-soft); display: block; font-size: .68rem; font-weight: 800; letter-spacing: .06em; margin-bottom: 6px; text-transform: uppercase; }
	.search-controls { display: flex; gap: 6px; }
	.search input { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; color: var(--text); flex: 1; font: inherit; min-width: 0; padding: 7px 10px; }
	.search input:focus { border-color: var(--charcoal); outline: 2px solid transparent; }
	.search small { color: var(--text-muted); display: block; margin-top: 7px; }
	.clear-search { color: var(--rust); }

	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.comment {
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--forest);
		border-radius: var(--radius);
		box-sizing: border-box;
		padding: 14px 16px;
		transition: opacity 120ms ease, background-color 120ms ease;
	}

	.comment.resolving {
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

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		justify-content: flex-end;
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

	.comment p {
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

	.message {
		color: var(--charcoal);
		flex-grow: 1;
		font-size: 1rem;
		line-height: 1.4;
		overflow-wrap: anywhere;
		text-align: left;
		white-space: pre-wrap;
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

		.page-header h2 {
			font-size: 2rem;
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

		.comment {
			border: 1px solid var(--line-strong);
			border-top: 4px solid var(--forest);
			border-left-width: 1px;
			border-radius: 6px;
			padding: 12px;
			width: 100%;
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

		.actions {
			justify-content: flex-start;
			width: 100%;
		}

		.actions button {
			flex: 1 1 96px;
		}

	}

	@media (max-width: 480px) {
		.stats {
			grid-template-columns: 1fr;
		}
	}

</style>
