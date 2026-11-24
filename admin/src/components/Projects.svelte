<script>
 import {canWrite} from "../portal-auth.mjs";
 import Panel from './Panel.svelte';
	import ProjectEditModal from './ProjectEditModal.svelte';

	export let reportProjects = () => {};
 export let selectedProject = 'cursemark';
 export let compact = false;
 $: visibleProjects = projects.filter(p => selectedProject === 'all' || p.id.toLowerCase() === selectedProject.toLowerCase());

	let projects = [];
	let loading = true;
	let error = '';
	let editingProject = null;

	async function loadProjects() {
		loading = true;
		error = '';
		try {
			const response = await fetch('/api/projects', { cache: 'no-store' });
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

<Panel class={compact ? "projects compact" : "projects"} title="Summary">
 <div slot="actions">{#if compact && visibleProjects.length === 1}<button type="button" disabled={!$canWrite} on:click={() => editingProject = visibleProjects[0]}>Edit</button>{/if}</div>

	<div class="project-content" aria-busy={loading}>
	{#if loading && !projects.length}
		<p class="message">Loading projects…</p>
	{:else if error && !projects.length}
		<p class="message error">{error}</p>
	{:else if !projects.length}
		<p class="message">No projects have reported data yet.</p>
	{:else}
		<div class="project-grid">
			{#each visibleProjects as project}
				<article>
					{#if !compact}<div class="project-heading">
						{#if !compact}<div><h3>{project.name}</h3><p class="project-id">{project.id}</p></div>{/if}
						<button type="button" disabled={!$canWrite} on:click={() => editingProject = project}>Edit</button>
					</div>{/if}
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
	</div>
</Panel>

<ProjectEditModal project={editingProject} on:close={() => editingProject = null} on:saved={projectSaved} />

<style>
 :global(.compact) .project-content { min-height: 0; }
 :global(.compact) article { background: transparent; border: 0; padding: 0; }
 :global(.compact) .project-grid { display: block; }
 :global(.compact) dl div { background: transparent; border: 0; border-right: 1px solid var(--line); border-radius: 0; }
 :global(.compact) dl div:last-child { border-right: 0; }
 :global(.compact) .steam-reviews { background: transparent; border: 0; border-top: 1px solid var(--line); border-radius: 0; padding: 12px 0 0; }




	h3, p { margin: 0; }


	.project-content { min-height: 230px; }
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
	@media (max-width: 650px) {  dl { grid-template-columns: repeat(2, 1fr); } .steam-reviews { align-items: flex-start; flex-direction: column; } .steam-reviews p { text-align: left; } }
</style>
