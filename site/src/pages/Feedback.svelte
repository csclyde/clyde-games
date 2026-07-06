<script>
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
	let ticketStatus = {};
	let resolvingFeedback = {};
	let translatingFeedback = {};
	let translations = {};

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

	async function makeTicket(comment) {
		ticketStatus = { ...ticketStatus, [comment.ID]: 'Making ticket...' };

		const res = await fetch(`https://api.clyde.games/feedback/ticket?id=` + comment.ID, {
			method: 'POST'
		});
		const result = await res.json();

		if (res.ok) {
			const cardName = result.card && result.card.name ? result.card.name : 'ticket';
			ticketStatus = { ...ticketStatus, [comment.ID]: `Created ${cardName}` };
		} else {
			ticketStatus = {
				...ticketStatus,
				[comment.ID]: result.error || 'Unable to make ticket'
			};
		}
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

	async function translateFeedback(comment) {
		translatingFeedback = { ...translatingFeedback, [comment.ID]: true };
		translations = {
			...translations,
			[comment.ID]: {
				text: translations[comment.ID] && translations[comment.ID].text,
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
				translations = {
					...translations,
					[comment.ID]: {
						text: '',
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

			translations = {
				...translations,
				[comment.ID]: {
					text: '',
					error: '',
					status: sourceLanguage === 'en' ? 'Already English' : 'Translated to English'
				}
			};
		} catch (error) {
			translations = {
				...translations,
				[comment.ID]: {
					text: translations[comment.ID] && translations[comment.ID].text,
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
		'black',
		'red',
		'orange',
		'gray',
		'blue',
		'green'
	]

	function metadataValue(value) {
		return value || 'unknown';
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

</script>

<main>
	<header class="page-header">
		<div>
			<h2>Player Feedback</h2>
			<p>{feedback.length} open item{feedback.length === 1 ? '' : 's'}</p>
		</div>
	</header>

	{#if loadingFeedback}
		<p class="state-message">loading...</p>
	{:else if feedbackError}
		<p class="state-message error">{feedbackError}</p>
	{:else if feedback.length === 0}
		<p class="state-message">No open feedback.</p>
	{:else}
		<div class="comment-list">
		{#each feedback as comment}
			<div class:resolving={resolvingFeedback[comment.ID]} class="comment">
				<div class="comment-body">
					<div class="comment-main">
						<div class="message-header">
							<span class="rating" style="background-color:{ colors[comment.Rating] }"></span>
							<div class="message-stack">
								<p class="message">{comment.Message}</p>
								{#if translations[comment.ID] && translations[comment.ID].status}
									<small class="translation-status">{translations[comment.ID].status}</small>
								{/if}
								{#if translations[comment.ID] && translations[comment.ID].error}
									<small class="translation-error">{translations[comment.ID].error}</small>
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
							<button type="button" disabled={resolvingFeedback[comment.ID]} on:click={() => makeTicket(comment)}>
								Make Ticket
							</button>
							<button type="button" disabled={resolvingFeedback[comment.ID]} on:click={() => resolveFeedback(comment.ID)}>
								{resolvingFeedback[comment.ID] ? 'Resolving...' : 'Resolve'}
							</button>
						</div>
						{#if ticketStatus[comment.ID]}
							<small class="ticket-status">{ticketStatus[comment.ID]}</small>
						{/if}
					</div>
				</div>
			</div>
		{/each}
		</div>
	{/if}
</main>

<style>
	main {
		color: #171717;
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.page-header {
		border-bottom: 1px solid #d8d8d8;
		margin-bottom: 14px;
		padding-bottom: 12px;
	}

	.page-header h2 {
		font-size: 2.5rem;
		line-height: 1;
		text-align: left;
	}

	.page-header p {
		color: #666;
		font-size: 0.85rem;
		font-weight: 600;
		letter-spacing: 0.02em;
		margin: 6px 0 0;
		text-align: left;
		text-transform: uppercase;
	}

	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.comment {
		background: #fff;
		border: 1px solid #bdbdbd;
		border-left: 4px solid #111;
		border-radius: 4px;
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
		color: #333;
		display: grid;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		margin: 0;
		text-align: left;
	}

	.stats span {
		color: #777;
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
		background: #f4f4f4;
		border: 1px solid #ddd;
		border-radius: 4px;
		color: #555;
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
		box-shadow: 0 0 0 2px #fff, 0 0 0 3px #d8d8d8;
		flex-shrink: 0;
		height: 14px;
		margin-top: 5px;
		width: 14px;
	}

	.message {
		color: #111;
		flex-grow: 1;
		font-size: 1rem;
		line-height: 1.4;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.translation-status {
		color: #777;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.translation-error {
		color: #a40000;
		max-width: 520px;
	}

	button {
		background: #fff;
		border: 1px solid #a8a8a8;
		border-radius: 4px;
		box-sizing: border-box;
		color: #111;
		cursor: pointer;
		font: inherit;
		font-size: 0.82rem;
		font-weight: 700;
		line-height: 1;
		padding: 7px 10px;
	}

	button:hover:not(:disabled) {
		background: #f2f2f2;
		border-color: #666;
	}

	button:disabled {
		color: #777;
		cursor: default;
	}

	small {
		font-size: 0.68rem;
		line-height: 1.3;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.ticket-status {
		color: #555;
		max-width: 220px;
		text-align: right;
	}

	.state-message {
		color: #666;
		margin: 32px 0;
		text-align: left;
	}

	.error {
		color: #a40000;
	}

	@media (max-width: 820px) {
		main {
			padding: 8px 4px 24px;
		}

		.page-header h2 {
			font-size: 2rem;
		}

		.comment-list {
			gap: 12px;
		}

		.comment {
			border: 1px solid #a8a8a8;
			border-top: 4px solid #111;
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

		.ticket-status {
			text-align: left;
		}
	}

	@media (max-width: 480px) {
		.stats {
			grid-template-columns: 1fr;
		}
	}

</style>
