<script>
	import ProjectEditModal from './ProjectEditModal.svelte';

	export let reportProjects = () => {};

	let projects = [];
	let loading = true;
	let error = '';
	let editingProject = null;

	async function loadProjects() {
		loading = true;
		error = '';
		try {
			const response = await fetch('https://api.clyde.games/projects', { cache: 'no-store' });
			const body = await response.json();
			if (!response.ok) throw new Error(body.error || 'Projects are unavailable');
			projects = body;
			reportProjects('projects', projects.map(project => project.id));
		} catch (loadError) {
			error = loadError.message || 'Projects are unavailable';
		} finally {
			loading = false;
		}
	}

	function projectSaved(event) {
		projects = projects.map(project => project.id === event.detail.id
			? { ...project, ...event.detail }
			: project);
	}

	function formatNumber(value) {
		if (value === null || value === undefined) return '0';
		return value.toLocaleString();
	}

	function steamReviewCount(project) {
		const reviews = project.steamReviews;
		if (!reviews || reviews.unavailableReason) return 'Unavailable';
		return `${formatNumber(reviews.countedReviews)} (${formatNumber(reviews.totalReviews)})`;
	}

	function steamReviewScore(project) {
		const reviews = project.steamReviews;
		if (!reviews || reviews.unavailableReason) return '';
		const tag = reviews.reviewScoreTag || 'Unrated';
		const percent = (reviews.positivePercentRaw || 0).toFixed(2);
		return `${percent}% | ${tag}`;
	}

	loadProjects();
</script>

<section class="projects" aria-labelledby="projects-heading">
	<header>
		<div><h2 id="projects-heading">Projects</h2><p>Project IDs are supplied by clients; display names can be edited here.</p></div>
		<button type="button" on:click={loadProjects} disabled={loading}>Refresh</button>
	</header>

	{#if loading && !projects.length}
		<p class="message">Loading projects…</p>
	{:else if error && !projects.length}
		<p class="message error">{error}</p>
	{:else if !projects.length}
		<p class="message">No projects have reported data yet.</p>
	{:else}
		<div class="project-grid">
			{#each projects as project}
				<article>
					<div class="project-heading">
						<div><h3>{project.name}</h3><p class="project-id">{project.id}</p></div>
						<button type="button" on:click={() => editingProject = project}>Edit</button>
					</div>
					<dl>
						<div><dt>Crashes</dt><dd>{project.crashes}</dd></div>
						<div><dt>Feedback</dt><dd>{project.feedback}</dd></div>
						<div><dt>Save games</dt><dd>{project.savegames}</dd></div>
						<div><dt>Events</dt><dd>{project.events}</dd></div>
					</dl>
					{#if project.steamAppId}
						<div class="steam-reviews">
							<div>
								<span>Steam reviews</span>
								<strong>{steamReviewCount(project)}</strong>
							</div>
							<p>{steamReviewScore(project) || 'Review score unavailable'}</p>
						</div>
					{/if}
				</article>
			{/each}
		</div>
		{#if error}<p class="message error">{error}</p>{/if}
	{/if}
</section>

<ProjectEditModal project={editingProject} on:close={() => editingProject = null} on:saved={projectSaved} />

<style>
	.projects { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); margin: 12px auto 0; max-width: 1368px; padding: 14px; }
	header { align-items: center; display: flex; gap: 8px; justify-content: space-between; }
	header { align-items: flex-start; border-bottom: 1px solid var(--line); margin-bottom: 12px; padding-bottom: 10px; }
	h2, h3, p { margin: 0; }
	h2 { font-size: 1.5rem; line-height: 1.15; text-align: left; }
	header p { color: var(--text-muted); font-size: .82rem; margin-top: 3px; text-align: left; }
	.project-grid { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); }
	article { background: var(--paper-warm); border-left: 3px solid var(--olive); border-radius: 3px; padding: 10px; }
	.project-heading { align-items: flex-start; display: flex; gap: 10px; justify-content: space-between; }
	h3 { font-size: 1.1rem; line-height: 1.2; overflow-wrap: anywhere; }
	.project-id { color: var(--text-muted); font-family: monospace; font-size: .74rem; margin-top: 3px; overflow-wrap: anywhere; }
	dl { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; margin: 14px 0 0; }
	dl div { background: var(--surface); border: 1px solid var(--line); border-radius: 3px; padding: 8px; text-align: center; }
	dt { color: var(--text-muted); font-size: .65rem; font-weight: 800; text-transform: uppercase; }
	dd { font-size: 1.15rem; font-weight: 800; margin: 3px 0 0; }
	.steam-reviews { align-items: center; background: var(--surface); border: 1px solid var(--line); border-radius: 3px; display: flex; gap: 10px; justify-content: space-between; margin-top: 8px; padding: 8px 10px; }
	.steam-reviews span { color: var(--text-muted); display: block; font-size: .65rem; font-weight: 800; text-transform: uppercase; }
	.steam-reviews strong { display: block; font-size: .96rem; margin-top: 2px; }
	.steam-reviews p { color: var(--text-soft); font-size: .82rem; font-weight: 800; text-align: right; }
	button { border: 1px solid var(--line); border-radius: 4px; font: inherit; padding: 6px 9px; }
	button { background: var(--paper-warm); color: var(--text); cursor: pointer; font-size: .75rem; font-weight: 700; }
	button:hover:not(:disabled) { border-color: var(--charcoal); }
	button:disabled { cursor: default; opacity: .6; }
	.message { color: var(--text-muted); padding: 16px 0 0; }
	.message.error { color: var(--red, #9b2c2c); }
	@media (max-width: 650px) { .projects { margin: 10px 8px 0; } dl { grid-template-columns: repeat(2, 1fr); } .steam-reviews { align-items: flex-start; flex-direction: column; } .steam-reviews p { text-align: left; } }
</style>
