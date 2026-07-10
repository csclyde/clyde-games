<script>
	export let selectedProject = 'Cursemark';
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
	let syncingSteam = false;
	let steamSyncStatus = '';
	let steamSyncError = '';

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

	async function syncSteamReviews() {
		syncingSteam = true;
		steamSyncStatus = 'Syncing Steam reviews...';
		steamSyncError = '';

		try {
			const res = await fetch(`https://api.clyde.games/steamreviews/import`, {
				method: 'POST'
			});
			const result = await res.json();

			if (!res.ok) {
				throw new Error(result.error || result);
			}

			await refreshFeedback();
			steamSyncStatus = `Steam synced: ${result.imported || 0} imported, ${result.skipped || 0} skipped`;
		} catch (error) {
			steamSyncStatus = '';
			steamSyncError = error.message;
		} finally {
			syncingSteam = false;
		}
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

	async function copyTextToClipboard(text) {
		if (navigator.clipboard && window.isSecureContext) {
			await navigator.clipboard.writeText(text);
			return;
		}

		const textarea = document.createElement('textarea');
		textarea.value = text;
		textarea.setAttribute('readonly', '');
		textarea.style.position = 'fixed';
		textarea.style.top = '-9999px';
		document.body.appendChild(textarea);
		textarea.select();

		try {
			document.execCommand('copy');
		} finally {
			document.body.removeChild(textarea);
		}
	}

	function valueOrUnknown(value) {
		return value || 'unknown';
	}

	async function copyTicketText(comment) {
		helperStatus = { ...helperStatus, [comment.ID]: { status: 'Copying...', error: '' } };

		try {
			await copyTextToClipboard(comment.Message || '');
			helperStatus = { ...helperStatus, [comment.ID]: { status: 'Copied feedback text', error: '' } };
		} catch (error) {
			helperStatus = {
				...helperStatus,
				[comment.ID]: { status: '', error: error.message || 'Unable to copy text' }
			};
		}
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

	async function detectLanguage(text) {
		if (typeof LanguageDetector === 'undefined') {
			throw new Error('Chrome language detection is not available in this browser.');
		}

		const detector = await LanguageDetector.create();
		const results = await detector.detect(text);

		if (results && results.length > 0 && results[0].detectedLanguage) {
			return results[0].detectedLanguage;
		}

		throw new Error('Could not detect the source language.');
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
			const sourceLanguage = await detectLanguage(messageParts.text);

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

			feedback = feedback.map(item => {
				if (item.ID !== comment.ID) {
					return item;
				}

				return {
					...item,
					Message: translatedMessage
				};
			});

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

	function feedbackContext(comment) {
		return [comment.Project, comment.Env, comment.Platform, comment.Category].filter(Boolean).join(' / ');
	}

	function matchesProject(item) {
		return selectedProject === 'all' || (item.Project || '').toLowerCase() === selectedProject.toLowerCase();
	}

	$: visibleFeedback = feedback.filter(matchesProject);
	$: reportProjects('feedback', feedback.map(comment => comment.Project));
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Player Feedback</h2>
			<p>{visibleFeedback.length} open item{visibleFeedback.length === 1 ? '' : 's'}</p>
		</div>
		<div class="header-actions">
			<button type="button" disabled={syncingSteam} on:click={syncSteamReviews}>
				{syncingSteam ? 'Syncing...' : 'Sync Steam'}
			</button>
			{#if steamSyncStatus}
				<small>{steamSyncStatus}</small>
			{/if}
			{#if steamSyncError}
				<small class="helper-error">{steamSyncError}</small>
			{/if}
		</div>
	</header>

	{#if loadingFeedback}
		<p class="state-message">loading...</p>
	{:else if feedbackError}
		<p class="state-message error">{feedbackError}</p>
	{:else if visibleFeedback.length === 0}
		<p class="state-message">No open feedback.</p>
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
									{#each messageSegments(comment.Message) as segment}
										{#if segment.isURL}
											<a target="_blank" rel="noreferrer" href={segment.text}>{segment.text}</a>
										{:else}
											{segment.text}
										{/if}
									{/each}
								</p>
								{#if helperStatus[comment.ID] && helperStatus[comment.ID].status}
									<small class="helper-status">{helperStatus[comment.ID].status}</small>
								{/if}
								{#if helperStatus[comment.ID] && helperStatus[comment.ID].error}
									<small class="helper-error">{helperStatus[comment.ID].error}</small>
								{/if}
							</div>
						</div>
						<div class="meta-list">
							{#if feedbackContext(comment)}
								<span>{feedbackContext(comment)}</span>
							{/if}
							{#if comment.PID}
								<span>{comment.PID}</span>
							{/if}
							{#if comment.Commit}
								<span>{comment.Commit}</span>
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
							<button type="button" disabled={resolvingFeedback[comment.ID]} on:click={() => copyTicketText(comment)}>
								Copy Text
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
