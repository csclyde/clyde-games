<script>
	import Feedback from './Feedback.svelte';
	import Crashes from './Crashes.svelte';
	import Savegames from './Savegames.svelte';
	import Metrics from './Metrics.svelte';
	import ServerStats from '../components/ServerStats.svelte';
	import User from './User.svelte';
	import Users from './Users.svelte';
	import Projects from '../components/Projects.svelte';

	export let tab = 'admin';
	export let userID = '';

	const defaultProject = 'all';
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

	function collectProjectOptions(sources) {
		const projects = new Map();
		for (const project of Object.values(sources).flat()) {
			const normalized = normalizeProject(project);
			if (!normalized) continue;
			const key = normalized.toLowerCase();
			if (!projects.has(key) || key === 'cursemark') projects.set(key, key === 'cursemark' ? 'cursemark' : normalized);
		}
		return [...projects.values()].sort((a, b) => a.localeCompare(b));
	}

	$: activeTab = tabs.find(item => item.id === tab) || { id: tab };
	$: projectOptions = collectProjectOptions(projectSources);
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

		{#if tab !== 'user'}
		<label class="project-filter">
			<span>Project</span>
			<select bind:value={selectedProject}>
				<option value="all">All</option>
				{#each visibleProjectOptions as project}
					<option value={project}>{project}</option>
				{/each}
			</select>
		</label>
		{/if}
	</header>

	{#if activeTab.id === 'admin'}
		<ServerStats />
		<Projects {reportProjects} />
	{/if}

	{#if tab === 'user'}
		<User {userID} />
	{:else}
		{#key `${activeTab.id}:${selectedProject}`}
			<svelte:component
				this={activeTab.component || Metrics}
				selectedProject={selectedProject}
				reportProjects={reportProjects}
			/>
		{/key}
	{/if}

	{#if activeTab.id === 'admin'}
		<Users />
	{/if}
</div>

<style>
	.admin-page {
		color: var(--text);
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
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 4px 4px 0 0;
		color: var(--text-muted);
		font-size: 0.9rem;
		font-weight: 800;
		line-height: 1;
		padding: 10px 12px;
		text-decoration: none;
	}

	.admin-tabs a:hover {
		background: var(--paper-warm);
		border-color: var(--line-strong);
	}

	.admin-tabs a.active {
		background: var(--charcoal);
		border-color: var(--charcoal);
		color: var(--surface);
	}

	.project-filter {
		align-items: center;
		display: flex;
		gap: 8px;
		margin-bottom: 1px;
	}

	.project-filter span {
		color: var(--olive);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.project-filter select {
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 4px;
		color: var(--text);
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
