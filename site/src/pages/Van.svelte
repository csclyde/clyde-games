<script>
	import VanStep from '../components/VanStep.svelte';
	import steps from '../data/van.json';
</script>

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
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	section {
		display: flex;
		flex-direction: row;
		flex-wrap: wrap;
		justify-content: center;
		gap: 16px;
	}
</style>