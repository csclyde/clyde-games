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

	let promise = getSavegames();

	function downloadSavegame(id) {
		const link = document.createElement('a');
		link.href = `https://api.clyde.games/savegame/download?id=${id}`;
		link.download = 'test.sav';
		document.body.appendChild(link);
		link.click();
		link.remove();
	}

	async function deleteSavegame(id) {
		const res = await fetch(`https://api.clyde.games/savegame?id=${id}`, {
			method: 'DELETE'
		});

		if (!res.ok) {
			const error = await parseResponse(res);
			throw new Error(error.error || error);
		}

		promise = getSavegames();
	}
</script>

<main>
	<h2>Save Games</h2>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then savegames}
		<div class="savegame-list">
		{#each savegames as savegame}
			<div class="savegame">
				<div class="savegame-body">
					<p class="name">{savegame.Filename || 'test.sav'}</p>
					<p class="created">{new Date(savegame.CreatedAt).toLocaleString()}</p>
					<p class="size">{Math.ceil(savegame.Size / 1024)} KB</p>
				</div>
				<div class="savegame-footer">
					<small>{savegame.PID}:{savegame.Platform}:{savegame.Project}:{savegame.Env}</small>
					<div class="actions">
						<button type="button" on:click={() => downloadSavegame(savegame.ID)}>
							Download
						</button>
						<button type="button" on:click={() => deleteSavegame(savegame.ID)}>
							Delete
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
	{:catch error}
		<p style="color: red">{error.message}</p>
	{/await}
</main>

<style>
	.savegame-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.savegame {
		display: flex;
		flex-direction: column;
		border: 2px solid black;
		padding: 8px;
		border-radius: 6px;
	}

	.savegame-body {
		display: flex;
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}

	.savegame-footer {
		display: flex;
		align-items: center;
		margin-top: 8px;
	}

	.name {
		flex-grow: 1;
		text-align: start;
		font-weight: 800;
	}

	.created,
	.size {
		flex-shrink: 0;
	}

	.reason {
		margin: 8px 0 4px;
		text-align: start;
	}

	.actions {
		display: flex;
		gap: 4px;
		margin-left: auto;
	}

	p {
		margin: 4px;
	}

	small {
		font-size: xx-small;
	}

	.hash {
		text-align: start;
	}
</style>
