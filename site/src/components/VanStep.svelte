<script>
	import ImageFigure from './ImageFigure.svelte';

	export let step;
</script>

<ImageFigure
	src={step.img}
	alt="Step in the camper van conversion process"
	caption={step.desc}
	aspect="4 / 3"
	accent="var(--ochre)"
/>
