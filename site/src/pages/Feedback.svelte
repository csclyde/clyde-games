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
	<h2>Player Feedback</h2>
	<hr/>

	{#if loadingFeedback}
		<p>loading...</p>
	{:else if feedbackError}
		<p style="color: red">{feedbackError}</p>
	{:else}
		<div class="comment-list">
		{#each feedback as comment}
			<div class:resolving={resolvingFeedback[comment.ID]} class="comment">
				<div class="comment-body">
					<div class="metadata">
						<p class="created">Created At: {new Date(comment.CreatedAt).toLocaleString()}</p>
						<p class="created">Built At: {metadataValue(comment.Build)}</p>
					</div>
					<div class="feedback-content">
						<div class="message-header">
							<p class="rating" style="background-color:{ colors[comment.Rating] }"></p>
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
	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.comment {
		display: flex;
		flex-direction: column;
		border: 2px solid black;
		padding: 8px;
		border-radius: 6px;
	}

	.comment.resolving {
		opacity: 0.45;
		pointer-events: none;
	}

	.comment-body {
		display: flex;
		flex-direction: row;
		align-items: center;
		gap: 12px;
	}

	.metadata {
		display: flex;
		flex-direction: column;
		align-items: start;
		flex-shrink: 0;
	}

	.actions {
		display: flex;
		gap: 4px;
		justify-content: flex-end;
	}

	.action-column {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex-shrink: 0;
		gap: 4px;
	}

	.feedback-content {
		flex-grow: 1;
	}

	.message-header {
		display: flex;
		align-items: center;
	}

	.comment-footer {
		display: flex;
		margin-top: 8px;
	}

	.comment p {
		margin: 4px;
	}

	.rating {
		/* background-color: red; */
		border-radius: 50%;
		width: 20px;
		height: 20px;
		text-align: center;
		font-weight: 800;
		flex-shrink: 0;
	}

	.created {
		flex-shrink: 0;
		font-size: small;
	}

	.message {
		flex-grow: 1;
		text-align: start;
	}

	small {
		font-size: xx-small;
	}

	.ticket-status {
		max-width: 220px;
		text-align: start;
	}

</style>
