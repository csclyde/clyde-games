<script>
	import { createEventDispatcher } from 'svelte';

	export let step;

	const dispatch = createEventDispatcher();
</script>

<button
	type="button"
	class="van-card"
	style:--image-accent="var(--ochre)"
	aria-label="Open van conversion image"
	on:click={() => dispatch('select')}
>
	<img src={step.img} alt="Step in the camper van conversion process"/>
	<span>{step.desc}</span>
</button>

<style>
	.van-card {
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--image-accent);
		border-radius: var(--radius);
		color: inherit;
		cursor: pointer;
		display: flex;
		flex-direction: column;
		font: inherit;
		margin: 0;
		padding: 10px;
		text-align: inherit;
		transition:
			background-color 160ms ease,
			border-color 160ms ease,
			box-shadow 160ms ease,
			transform 160ms ease;
		width: 100%;
	}

	.van-card:hover,
	.van-card:focus-visible {
		background: color-mix(in srgb, var(--surface) 86%, var(--image-accent));
		border-color: color-mix(in srgb, var(--line) 70%, var(--image-accent));
		box-shadow: 0 12px 22px color-mix(in srgb, var(--image-accent) 18%, transparent);
		transform: translateY(-3px) scale(1.01);
	}

	.van-card:focus-visible {
		outline: 3px solid color-mix(in srgb, var(--image-accent) 65%, white);
		outline-offset: 3px;
	}

	img {
		aspect-ratio: 4 / 3;
		background: var(--surface-muted);
		display: block;
		filter: saturate(0.94) contrast(0.98);
		height: auto;
		object-fit: cover;
		width: 100%;
	}

	span {
		border-top: 1px solid var(--line);
		color: var(--text-muted);
		display: block;
		font-size: 1rem;
		font-weight: 500;
		line-height: 1.45;
		margin-top: 10px;
		padding: 10px 4px 2px;
		text-align: left;
	}
</style>
