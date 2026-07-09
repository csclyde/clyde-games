<script>
	import SubpageHeader from '../components/SubpageHeader.svelte';
	import VanStep from '../components/VanStep.svelte';
	import steps from '../data/van.json';

	let selectedIndex = null;

	$: selectedStep = selectedIndex === null ? null : steps[selectedIndex];

	function openGallery(index) {
		selectedIndex = index;
	}

	function closeGallery() {
		selectedIndex = null;
	}

	function showPrevious() {
		selectedIndex = (selectedIndex + steps.length - 1) % steps.length;
	}

	function showNext() {
		selectedIndex = (selectedIndex + 1) % steps.length;
	}

	function handleKeydown(event) {
		if (selectedIndex === null) {
			return;
		}

		if (event.key === 'Escape') {
			closeGallery();
		}

		if (event.key === 'ArrowLeft') {
			showPrevious();
		}

		if (event.key === 'ArrowRight') {
			showNext();
		}
	}
</script>

<svelte:window on:keydown={handleKeydown}/>

<SubpageHeader label="Camper Van Conversion" />

<main>
	<h2>Camper Van Conversion</h2>
	<p>This is the process I went through to build my camper van.</p>

	<section>
		{#each steps as step, index}
			<VanStep step={step} on:select={() => openGallery(index)}/>
		{/each}
	</section>
</main>

{#if selectedStep}
	<div class="gallery-backdrop" on:click={closeGallery}>
		<div
			class="gallery"
			role="dialog"
			aria-modal="true"
			aria-label="Camper van image gallery"
			on:click|stopPropagation
		>
			<div class="image-wrap">
				<img src={selectedStep.img} alt="Step in the camper van conversion process"/>
				<button type="button" class="close" aria-label="Close gallery" on:click={closeGallery}>&times;</button>
				<button type="button" class="arrow previous" aria-label="Previous image" on:click={showPrevious}>&lsaquo;</button>
				<button type="button" class="arrow next" aria-label="Next image" on:click={showNext}>&rsaquo;</button>
			</div>
			<p>{selectedStep.desc}</p>
		</div>
	</div>
{/if}

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

	.gallery-backdrop {
		align-items: center;
		background:
			radial-gradient(circle at 18% 12%, rgba(63, 117, 144, 0.24), transparent 34%),
			linear-gradient(135deg, rgba(16, 22, 24, 0.96), rgba(36, 41, 38, 0.94));
		display: flex;
		inset: 0;
		justify-content: center;
		padding: 18px;
		position: fixed;
		z-index: 20;
	}

	.gallery {
		background: transparent;
		border: 0;
		box-shadow: none;
		max-height: calc(100vh - 36px);
		max-width: min(1400px, 100%);
		overflow: auto;
		padding: 0;
		position: relative;
		width: 100%;
	}

	.image-wrap {
		background:
			linear-gradient(135deg, rgba(255, 255, 255, 0.055), rgba(255, 255, 255, 0.015)),
			#111719;
		border: 1px solid rgba(255, 254, 250, 0.14);
		border-radius: 6px;
		box-shadow:
			0 30px 80px rgba(0, 0, 0, 0.42),
			inset 0 1px 0 rgba(255, 255, 255, 0.08);
		height: min(78vh, 920px);
		overflow: hidden;
		position: relative;
	}

	.gallery img {
		display: block;
		height: 100%;
		object-fit: contain;
		width: 100%;
	}

	.gallery p {
		background: rgba(17, 23, 25, 0.58);
		border: 1px solid rgba(255, 254, 250, 0.1);
		border-top: 3px solid var(--slate);
		border-radius: 0 0 6px 6px;
		color: rgba(255, 254, 250, 0.86);
		font-size: 1.05rem;
		line-height: 1.45;
		margin: 0;
		padding: 14px 16px;
		text-align: left;
	}

	.close,
	.arrow {
		align-items: center;
		backdrop-filter: blur(10px);
		background: rgba(17, 23, 25, 0.62);
		border: 1px solid rgba(255, 254, 250, 0.18);
		border-radius: var(--radius);
		box-shadow: 0 12px 30px rgba(0, 0, 0, 0.24);
		color: rgba(255, 254, 250, 0.88);
		cursor: pointer;
		display: flex;
		font-family: var(--font-sans);
		font-weight: 600;
		justify-content: center;
		position: absolute;
		transition:
			border-color 140ms ease,
			background-color 140ms ease,
			color 140ms ease,
			box-shadow 140ms ease,
			transform 140ms ease;
	}

	.close:hover,
	.arrow:hover,
	.close:focus-visible,
	.arrow:focus-visible {
		background: rgba(255, 254, 250, 0.95);
		border-color: rgba(255, 254, 250, 0.95);
		box-shadow: 0 14px 34px rgba(0, 0, 0, 0.28);
		color: #111719;
	}

	.close:focus-visible,
	.arrow:focus-visible {
		outline: 3px solid var(--slate);
		outline-offset: 2px;
	}

	.close {
		font-size: 1.55rem;
		height: 42px;
		line-height: 1;
		right: 14px;
		top: 14px;
		width: 42px;
	}

	.close:hover,
	.close:focus-visible {
		transform: translateY(-1px);
	}

	.arrow {
		font-size: 3rem;
		height: 72px;
		line-height: 1;
		top: 50%;
		transform: translateY(-50%);
		width: 46px;
	}

	.arrow:hover,
	.arrow:focus-visible {
		transform: translateY(-50%) translateX(0);
	}

	.previous {
		left: 14px;
	}

	.next {
		right: 14px;
	}

	@media (max-width: 640px) {
		.gallery-backdrop {
			padding: 8px;
		}

		.gallery {
			max-height: calc(100vh - 16px);
			padding: 8px;
		}

		.image-wrap {
			height: min(68vh, 620px);
		}

		.close {
			height: 38px;
			right: 10px;
			top: 10px;
			width: 38px;
		}

		.arrow {
			font-size: 2.5rem;
			height: 60px;
			width: 40px;
		}

		.previous {
			left: 10px;
		}

		.next {
			right: 10px;
		}
	}
</style>
