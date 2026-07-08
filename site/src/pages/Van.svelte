<script>
	import SubpageHeader from '../components/SubpageHeader.svelte';
	import VanStep from '../components/VanStep.svelte';
	import steps from '../data/van.json';
</script>

<SubpageHeader label="Camper Van Conversion" />

<main>
	<h2>Camper Van Conversion</h2>
	<p>This is the process I went through to build my camper van.</p>

	<section>
		{#each steps as step}
			<VanStep step={step}/>
		{/each}
	</section>
</main>

<style>
	main {
		max-width: var(--content);
		margin: 0 auto;
		padding: 42px 24px 24px;
	}

	h2,
	p {
		text-align: left;
	}

	h2 {
		border-bottom: 1px solid var(--line);
		padding-bottom: 16px;
	}

	p {
		color: var(--text-muted);
		font-size: 1.2rem;
		font-weight: 500;
	}

	section {
		display: grid;
		gap: 20px;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 420px), 1fr));
		margin-top: 32px;
	}
</style>
