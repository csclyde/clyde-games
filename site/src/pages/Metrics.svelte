<script>
	async function parseResponse(res) {
		const text = await res.text();

		try {
			return JSON.parse(text);
		} catch (error) {
			if (res.ok) {
				throw new Error('Expected JSON response from metrics API');
			}

			throw new Error(text || res.statusText);
		}
	}

	async function getBuilds() {
		const res = await fetch(`https://api.clyde.games/event/builds`);
		const builds = await parseResponse(res);

		if (res.ok) {
			return builds;
		} else {
			throw new Error(builds.error || builds);
		}
	}

	let builds = [];
	let loadingBuilds = true;
	let buildError = '';
	let deletingBuild = null;
	let deleteError = '';

	async function loadBuilds() {
		loadingBuilds = true;
		buildError = '';

		try {
			builds = await getBuilds();
		} catch (error) {
			buildError = error.message;
		} finally {
			loadingBuilds = false;
		}
	}

	loadBuilds();

	async function refreshBuilds() {
		try {
			builds = await getBuilds();
			buildError = '';
		} catch (error) {
			buildError = error.message;
		}
	}

	function formatBuildDate(build) {
		const date = new Date(build);

		if (isNaN(date.getTime())) {
			return build;
		}

		return date.toLocaleDateString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	async function deleteBuild(build) {
		if (!confirm(`Delete all metrics events for build ${build}?`)) {
			return;
		}

		deletingBuild = build;
		deleteError = '';

		try {
			const res = await fetch(`https://api.clyde.games/event/build?build=${encodeURIComponent(build)}`, {
				method: 'DELETE'
			});

			if (!res.ok) {
				const error = await parseResponse(res);
				deleteError = error.error || error;
				return;
			}

			builds = builds.filter(item => item.Build !== build);
			await refreshBuilds();
		} catch (error) {
			deleteError = error.message;
		} finally {
			deletingBuild = null;
		}
	}
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Metrics</h2>
			<p>{builds.length} build{builds.length === 1 ? '' : 's'}</p>
		</div>
	</header>

	<section class="builds">
		{#if deleteError}
			<p class="error">{deleteError}</p>
		{/if}

		{#if loadingBuilds}
			<p class="state-message">loading...</p>
		{:else if buildError}
			<p class="state-message error">{buildError}</p>
		{:else}
			{#if builds.length > 0}
				<div class="build-list">
					{#each builds as build}
						<div class:deleting={deletingBuild === build.Build} class="build-row">
							<p class="build-date">{formatBuildDate(build.Build)}</p>
							<p class="build-count">{build.Count} events</p>
							<button
								type="button"
								disabled={deletingBuild === build.Build}
								on:click={() => deleteBuild(build.Build)}
							>
								{deletingBuild === build.Build ? 'Deleting...' : 'Delete All'}
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<p class="empty-state">No builds found.</p>
			{/if}
		{/if}
	</section>
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

	.builds {
		border: 1px solid #cfcfcf;
		border-left: 4px solid #111;
		border-radius: 4px;
		padding: 4px 16px;
	}

	.build-list {
		display: flex;
		flex-direction: column;
	}

	.build-row {
		display: grid;
		grid-template-columns: minmax(150px, max-content) minmax(120px, 1fr) auto;
		align-items: center;
		column-gap: 16px;
		border-top: 1px solid #ddd;
		padding: 12px 0;
		text-align: left;
		transition: opacity 120ms ease;
	}

	.build-row:first-child {
		border-top: 0;
	}

	.build-row.deleting {
		opacity: 0.45;
		pointer-events: none;
	}

	.build-date,
	.build-count {
		margin: 0;
		text-align: left;
	}

	.build-date {
		color: #111;
		font-size: 0.95rem;
		font-weight: 800;
	}

	.build-count {
		color: #666;
		font-size: 0.85rem;
		font-weight: 600;
		text-transform: uppercase;
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

	.empty-state {
		color: #666;
		margin: 12px 0;
		text-align: left;
	}

	.error {
		color: #a40000;
		margin: 12px 0;
		text-align: left;
	}

	.state-message {
		color: #666;
		margin: 12px 0;
		text-align: left;
	}

	@media (max-width: 520px) {
		main {
			padding: 8px;
		}

		.page-header h2 {
			font-size: 2rem;
		}

		.build-row {
			grid-template-columns: 1fr auto;
			row-gap: 4px;
		}

		.build-count {
			grid-column: 1;
		}

		button {
			grid-column: 2;
			grid-row: 1 / span 2;
		}
	}
</style>
