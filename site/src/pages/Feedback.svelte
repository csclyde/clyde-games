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
					<div class="metadata">
						<p class="created"><span>Created</span>{new Date(comment.CreatedAt).toLocaleString()}</p>
						<p class="created"><span>Build</span>{metadataValue(comment.Build)}</p>
					</div>
					<div class="feedback-content">
						<div class="message-header">
							<span class="rating" style="background-color:{ colors[comment.Rating] }"></span>
							<p class="message">{comment.Message}</p>
						</div>
					</div>
					<div class="action-column">
						<div class="actions">
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
				<div class="comment-footer">
					<small>{comment.PID}:{comment.Platform}:{comment.Project}:{comment.Env}:{metadataValue(comment.Commit)}</small>
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
		border: 1px solid #cfcfcf;
		border-left: 4px solid #111;
		border-radius: 4px;
		display: flex;
		flex-direction: column;
		padding: 14px 16px 10px;
		transition: opacity 120ms ease, background-color 120ms ease;
	}

	.comment.resolving {
		opacity: 0.45;
		pointer-events: none;
	}

	.comment-body {
		align-items: flex-start;
		display: flex;
		flex-direction: row;
		gap: 18px;
	}

	.metadata {
		color: #333;
		display: flex;
		flex-direction: column;
		align-items: start;
		flex-shrink: 0;
		gap: 5px;
		width: 240px;
	}

	.created {
		display: grid;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		margin: 0;
		text-align: left;
	}

	.created span {
		color: #777;
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
	}

	.action-column {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex-shrink: 0;
		gap: 6px;
		margin-left: auto;
	}

	.feedback-content {
		flex-grow: 1;
		min-width: 0;
	}

	.message-header {
		align-items: flex-start;
		display: flex;
		gap: 12px;
	}

	.comment-footer {
		color: #666;
		display: flex;
		margin-top: 12px;
		min-width: 0;
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

	button {
		background: #fff;
		border: 1px solid #a8a8a8;
		border-radius: 4px;
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

	small,
	.comment-footer small {
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
			padding: 8px;
		}

		.page-header h2 {
			font-size: 2rem;
		}

		.comment-body {
			display: grid;
			gap: 12px;
		}

		.metadata {
			display: grid;
			grid-template-columns: 1fr 1fr;
			width: 100%;
		}

		.action-column {
			align-items: flex-start;
			margin-left: 0;
		}

		.ticket-status {
			text-align: left;
		}
	}

</style>
