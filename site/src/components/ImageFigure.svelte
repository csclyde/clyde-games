<script>
	export let src;
	export let alt = '';
	export let caption = '';
	export let aspect = '4 / 3';
	export let accent = 'var(--slate)';
</script>

<figure style:--image-aspect={aspect} style:--image-accent={accent} class:has-caption={caption}>
	<img {src} {alt}/>
	{#if caption}
		<figcaption>{caption}</figcaption>
	{/if}
</figure>

<style>
	figure {
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--image-accent);
		border-radius: var(--radius);
		margin: 0;
		padding: 8px;
	}

	img {
		aspect-ratio: var(--image-aspect);
		background: var(--surface-muted);
		display: block;
		filter: saturate(0.94) contrast(0.98);
		height: auto;
		object-fit: cover;
		width: 100%;
	}

	figcaption {
		border-top: 1px solid var(--line);
		color: var(--text-muted);
		font-size: 1rem;
		font-weight: 500;
		line-height: 1.45;
		margin-top: 10px;
		padding: 10px 4px 2px;
		text-align: left;
	}

	.has-caption {
		padding: 10px;
	}
</style>
