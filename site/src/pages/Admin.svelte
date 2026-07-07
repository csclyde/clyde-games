<script>
	import Feedback from './Feedback.svelte';
	import Crashes from './Crashes.svelte';
	import Savegames from './Savegames.svelte';
	import Metrics from './Metrics.svelte';

	export let tab = 'admin';

	const tabs = [
		{ id: 'admin', label: 'Admin', href: '/admin', component: Metrics },
		{ id: 'feedback', label: 'Feedback', href: '/feedback', component: Feedback },
		{ id: 'crashes', label: 'Crashes', href: '/crashes', component: Crashes },
		{ id: 'savegames', label: 'Save Games', href: '/savegames', component: Savegames }
	];

	$: activeTab = tabs.find(item => item.id === tab) || tabs[0];
</script>

<div class="admin-page">
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

	<svelte:component this={activeTab.component} />
</div>

<style>
	.admin-page {
		color: #171717;
	}

	.admin-tabs {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin: 0 auto;
		max-width: 1400px;
		padding: 16px 16px 0;
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

	@media (max-width: 520px) {
		.admin-tabs {
			padding: 10px 8px 0;
		}

		.admin-tabs a {
			flex: 1 1 calc(50% - 6px);
			text-align: center;
		}
	}
</style>
