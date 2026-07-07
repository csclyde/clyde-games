<script>
	import Feedback from './Feedback.svelte';
	import Crashes from './Crashes.svelte';
	import Savegames from './Savegames.svelte';
	import Metrics from './Metrics.svelte';

	export let tab = 'admin';

	const defaultProject = 'Cursemark';
	const tabs = [
		{ id: 'admin', label: 'Admin', href: '/admin', component: Metrics },
		{ id: 'feedback', label: 'Feedback', href: '/feedback', component: Feedback },
		{ id: 'crashes', label: 'Crashes', href: '/crashes', component: Crashes },
		{ id: 'savegames', label: 'Save Games', href: '/savegames', component: Savegames }
	];

	let selectedProject = defaultProject;
	let projectSources = {};

	function normalizeProject(project) {
		return (project || '').trim();
	}

	function reportProjects(source, projects) {
		const projectMap = new Map();
		for (const project of projects.map(normalizeProject).filter(Boolean)) {
			const key = project.toLowerCase();
			if (!projectMap.has(key)) {
				projectMap.set(key, project);
			}
		}

		const nextProjects = [...projectMap.values()];
		projectSources = { ...projectSources, [source]: nextProjects };
	}

	$: activeTab = tabs.find(item => item.id === tab) || tabs[0];
	$: projectOptions = [...new Set(Object.values(projectSources).flat())].sort((a, b) => a.localeCompare(b));
	$: visibleProjectOptions = selectedProject && selectedProject !== 'all' && !projectOptions.includes(selectedProject)
		? [selectedProject, ...projectOptions]
		: projectOptions;
</script>

<div class="admin-page">
	<header class="admin-header">
		<nav class="admin-tabs" aria-label="Admin sections">
			{#each tabs as item}
				<a
					class:active={item.id === activeTab.id}
					aria-current={item.id === activeTab.id ? 'page' : undefined}
					href={item.href}
				>
					{item.label}
				</a>
			{/each}
		</nav>

		<label class="project-filter">
			<span>Project</span>
			<select bind:value={selectedProject}>
				<option value="all">All</option>
				{#each visibleProjectOptions as project}
					<option value={project}>{project}</option>
				{/each}
			</select>
		</label>
	</header>

	<svelte:component
		this={activeTab.component}
		selectedProject={selectedProject}
		reportProjects={reportProjects}
	/>
</div>

<style>
	.admin-page {
		color: #171717;
	}

	.admin-header {
		align-items: flex-end;
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		justify-content: space-between;
		margin: 0 auto;
		max-width: 1400px;
		padding: 16px 16px 0;
	}

	.admin-tabs {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.admin-tabs a {
		background: #fff;
		border: 1px solid #bdbdbd;
		border-radius: 4px 4px 0 0;
		color: #333;
		font-size: 0.9rem;
		font-weight: 800;
		line-height: 1;
		padding: 10px 12px;
		text-decoration: none;
	}

	.admin-tabs a:hover {
		background: #f4f4f4;
		border-color: #777;
	}

	.admin-tabs a.active {
		background: #111;
		border-color: #111;
		color: #fff;
	}

	.project-filter {
		align-items: center;
		display: flex;
		gap: 8px;
		margin-bottom: 1px;
	}

	.project-filter span {
		color: #555;
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.project-filter select {
		background: #fff;
		border: 1px solid #bdbdbd;
		border-radius: 4px;
		color: #111;
		font: inherit;
		font-size: 0.9rem;
		font-weight: 700;
		min-width: 160px;
		padding: 7px 10px;
	}

	@media (max-width: 520px) {
		.admin-header {
			align-items: stretch;
			padding: 10px 8px 0;
		}

		.admin-tabs a {
			flex: 1 1 calc(50% - 6px);
			text-align: center;
		}

		.project-filter {
			justify-content: space-between;
			width: 100%;
		}

		.project-filter select {
			flex: 1;
			min-width: 0;
		}
	}
</style>
