<script>
	import PackratDocs from '../components/PackratDocs.svelte';
</script>

<main class="tools-page" aria-label="Admin tools">
	<PackratDocs />
</main>

<style>
	.tools-page {
		margin: 0 auto;
		max-width: 1400px;
		padding: 0 16px;
	}

	@media (max-width: 520px) {
		.tools-page {
			padding: 0;
		}
	}
</style>
