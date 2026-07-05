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

	let promise = getBuilds();
	let deletingBuild = null;
	let deleteError = '';

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
		const res = await fetch(`https://api.clyde.games/event/build?build=${encodeURIComponent(build)}`, {
			method: 'DELETE'
		});

		deletingBuild = null;

		if (!res.ok) {
			const error = await parseResponse(res);
			deleteError = error.error || error;
			return;
		}

		promise = getBuilds();
	}
</script>

<main>
	<h2>Metrics</h2>
	<hr/>

	<section class="builds">
		<h3>Builds</h3>

		{#if deleteError}
			<p class="error">{deleteError}</p>
		{/if}

		{#await promise}
			<p>loading...</p>
		{:then builds}
			{#if builds.length > 0}
				<div class="build-list">
					{#each builds as build}
						<div class="build-row">
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
		{:catch error}
			<p style="color: red">{error.message}</p>
		{/await}
	</section>
</main>

<style>
	.builds {
		border: 2px solid black;
		border-radius: 6px;
		padding: 8px;
	}

	.builds h3 {
		margin: 0 0 8px;
		text-align: start;
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
		padding: 10px 0;
		text-align: start;
	}

	.build-row:first-child {
		border-top: 0;
	}

	.build-date,
	.build-count {
		margin: 0;
		text-align: start;
	}

	.build-date {
		font-weight: 800;
	}

	.build-count {
		color: #444;
	}

	.empty-state {
		margin: 0;
		text-align: start;
	}

	.error {
		color: red;
		margin: 0 0 8px;
		text-align: start;
	}

	@media (max-width: 520px) {
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
