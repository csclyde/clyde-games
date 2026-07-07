<script>
	export let selectedProject = 'Cursemark';
	export let reportProjects = () => {};

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
		const savegame = savegames.find(item => item.ID === id);
		const link = document.createElement('a');
		link.href = `https://api.clyde.games/savegame/download?id=${id}`;
		link.download = savegame?.Filename || 'test.sav';
		document.body.appendChild(link);
		link.click();
		link.remove();
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

	function formatBuildDate(build) {
		return formatDate(build);
	}

	function formatSize(size) {
		if (!size) {
			return '0 KB';
		}

		return `${Math.ceil(size / 1024)} KB`;
	}

	function savegameContext(savegame) {
		return [savegame.Project, savegame.Env, savegame.Platform, savegame.Category].filter(Boolean).join(' / ');
	}

	function playerContext(savegame) {
		return [savegame.PID, savegame.SID].filter(Boolean).join(' / ');
	}

	function matchesProject(item) {
		return selectedProject === 'all' || (item.Project || '').toLowerCase() === selectedProject.toLowerCase();
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

	$: visibleSavegames = savegames.filter(matchesProject);
	$: reportProjects('savegames', savegames.map(savegame => savegame.Project));
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Save Games</h2>
			<p>{visibleSavegames.length} file{visibleSavegames.length === 1 ? '' : 's'}</p>
		</div>
	</header>

	{#if loadingSavegames}
		<p class="state-message">loading...</p>
	{:else if savegameError}
		<p class="state-message error">{savegameError}</p>
	{:else if visibleSavegames.length === 0}
		<p class="state-message">No save games found.</p>
	{:else}
		<div class="savegame-list">
		{#each visibleSavegames as savegame}
			<div class:deleting={deletingSavegames[savegame.ID]} class="savegame">
				<div class="savegame-body">
					<div class="savegame-main">
						<p class="name">{savegame.Filename || 'test.sav'}</p>
						{#if savegame.Reason}
							<p class="reason">{savegame.Reason}</p>
						{/if}
						<div class="meta-list">
							{#if savegameContext(savegame)}
								<span>{savegameContext(savegame)}</span>
							{/if}
							{#if playerContext(savegame)}
								<span>{playerContext(savegame)}</span>
							{/if}
							{#if savegame.Commit}
								<span>{savegame.Commit}</span>
							{/if}
							{#if savegame.Hash}
								<span>{savegame.Hash}</span>
							{/if}
						</div>
					</div>
					<div class="savegame-side">
						<div class="stats">
							<p><span>Created</span>{formatDate(savegame.CreatedAt)}</p>
							<p><span>Build</span>{formatBuildDate(savegame.Build)}</p>
							<p><span>Size</span>{formatSize(savegame.Size)}</p>
						</div>
						<div class="actions">
							<button type="button" disabled={deletingSavegames[savegame.ID]} on:click={() => downloadSavegame(savegame.ID)}>
								Download
							</button>
							<button type="button" disabled={deletingSavegames[savegame.ID]} on:click={() => deleteSavegame(savegame.ID)}>
								{deletingSavegames[savegame.ID] ? 'Deleting...' : 'Delete'}
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
		padding: 14px 16px;
		transition: opacity 120ms ease, background-color 120ms ease;
	}

	.savegame.deleting {
		opacity: 0.45;
		pointer-events: none;
	}

	.savegame-body {
		align-items: stretch;
		display: grid;
		gap: 18px;
		grid-template-columns: minmax(0, 1fr) minmax(320px, auto);
	}

	.savegame-main {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
	}

	.savegame-side {
		align-items: flex-end;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.name {
		color: #111;
		font-size: 1rem;
		font-weight: 800;
		line-height: 1.35;
		min-width: 0;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.stats {
		display: grid;
		gap: 12px;
		grid-template-columns: repeat(3, minmax(90px, max-content));
		justify-content: end;
	}

	.stats p {
		color: #333;
		display: grid;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		text-align: left;
	}

	.stats span {
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
		text-align: left;
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

	.actions {
		display: flex;
		gap: 6px;
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
			grid-template-columns: 1fr;
		}

		.savegame-side {
			align-items: flex-start;
		}

		.stats {
			grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
			justify-content: stretch;
			width: 100%;
		}

		.actions {
			margin-left: 0;
		}
	}
</style>
