<script>
	async function parseResponse(res) {
		const text = await res.text();

		try {
			return JSON.parse(text);
		} catch (error) {
			if (res.ok) {
				throw new Error('Expected JSON response from savegame API');
			}

			throw new Error(text || res.statusText);
		}
	}

	async function getSavegames() {
		const res = await fetch(`https://api.clyde.games/savegame`);
		const savegames = await parseResponse(res);

		if (res.ok) {
			return savegames;
		} else {
			throw new Error(savegames.error || savegames);
		}
	}

	let savegames = [];
	let loadingSavegames = true;
	let savegameError = '';
	let deletingSavegames = {};

	async function loadSavegames() {
		loadingSavegames = true;
		savegameError = '';

		try {
			savegames = await getSavegames();
		} catch (error) {
			savegameError = error.message;
		} finally {
			loadingSavegames = false;
		}
	}

	loadSavegames();

	async function refreshSavegames() {
		try {
			savegames = await getSavegames();
			savegameError = '';
		} catch (error) {
			savegameError = error.message;
		}
	}

	function downloadSavegame(id) {
		const link = document.createElement('a');
		link.href = `https://api.clyde.games/savegame/download?id=${id}`;
		link.download = 'test.sav';
		document.body.appendChild(link);
		link.click();
		link.remove();
	}

	async function deleteSavegame(id) {
		deletingSavegames = { ...deletingSavegames, [id]: true };

		try {
			const res = await fetch(`https://api.clyde.games/savegame?id=${id}`, {
				method: 'DELETE'
			});

			if (!res.ok) {
				const error = await parseResponse(res);
				throw new Error(error.error || error);
			}

			savegames = savegames.filter(savegame => savegame.ID !== id);
			await refreshSavegames();
		} catch (error) {
			savegameError = error.message;
		} finally {
			const nextDeletingSavegames = { ...deletingSavegames };
			delete nextDeletingSavegames[id];
			deletingSavegames = nextDeletingSavegames;
		}
	}
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Save Games</h2>
			<p>{savegames.length} file{savegames.length === 1 ? '' : 's'}</p>
		</div>
	</header>

	{#if loadingSavegames}
		<p class="state-message">loading...</p>
	{:else if savegameError}
		<p class="state-message error">{savegameError}</p>
	{:else if savegames.length === 0}
		<p class="state-message">No save games found.</p>
	{:else}
		<div class="savegame-list">
		{#each savegames as savegame}
			<div class:deleting={deletingSavegames[savegame.ID]} class="savegame">
				<div class="savegame-body">
					<p class="name">{savegame.Filename || 'test.sav'}</p>
					<p class="created"><span>Created</span>{new Date(savegame.CreatedAt).toLocaleString()}</p>
					<p class="size"><span>Size</span>{Math.ceil(savegame.Size / 1024)} KB</p>
				</div>
				<div class="savegame-footer">
					<small>{savegame.PID}:{savegame.Platform}:{savegame.Project}:{savegame.Env}</small>
					<div class="actions">
						<button type="button" disabled={deletingSavegames[savegame.ID]} on:click={() => downloadSavegame(savegame.ID)}>
							Download
						</button>
						<button type="button" disabled={deletingSavegames[savegame.ID]} on:click={() => deleteSavegame(savegame.ID)}>
							{deletingSavegames[savegame.ID] ? 'Deleting...' : 'Delete'}
						</button>
					</div>
				</div>
				{#if savegame.Reason}
					<p class="reason">{savegame.Reason}</p>
				{/if}
				<small class="hash">{savegame.Hash}</small>
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

	.savegame-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.savegame {
		background: #fff;
		border: 1px solid #cfcfcf;
		border-left: 4px solid #111;
		border-radius: 4px;
		display: flex;
		flex-direction: column;
		padding: 14px 16px 10px;
		transition: opacity 120ms ease, background-color 120ms ease;
	}

	.savegame.deleting {
		opacity: 0.45;
		pointer-events: none;
	}

	.savegame-body {
		align-items: flex-start;
		display: flex;
		flex-direction: row;
		gap: 18px;
	}

	.savegame-footer {
		display: flex;
		align-items: center;
		color: #666;
		margin-top: 12px;
		min-width: 0;
	}

	.name {
		color: #111;
		flex-grow: 1;
		font-size: 1rem;
		font-weight: 800;
		line-height: 1.35;
		min-width: 0;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.created,
	.size {
		color: #333;
		display: grid;
		flex-shrink: 0;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		text-align: left;
		width: 180px;
	}

	.created span,
	.size span {
		color: #777;
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.reason {
		color: #333;
		font-size: 0.9rem;
		line-height: 1.35;
		margin: 12px 0 4px;
		text-align: left;
	}

	.actions {
		display: flex;
		gap: 6px;
		margin-left: auto;
	}

	p {
		margin: 0;
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

	small {
		font-size: 0.68rem;
		line-height: 1.3;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.hash {
		color: #666;
		margin-top: 8px;
		text-align: left;
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

		.savegame-body {
			display: grid;
			gap: 10px;
		}

		.created,
		.size {
			width: auto;
		}

		.savegame-footer {
			align-items: flex-start;
			flex-direction: column;
			gap: 10px;
		}

		.actions {
			margin-left: 0;
		}
	}
</style>
