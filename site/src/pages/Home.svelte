<script>
	import Project from '../components/Project.svelte';
	import projects from '../data/projects.json';
	import havamal from '../data/havamal.json';

	const accents = [
		'var(--slate)',
		'var(--forest)',
		'var(--ochre)',
		'var(--rust)'
	];
</script>

<main>
	<header class="studio-header">
		<div class="title-block">
			<h1>CLYDE GAMES</h1>
		</div>
		<img class='portrait' src="/img/casey.jpeg" alt="Casey at Lake Tahoe">
	</header>
	<p class="intro">Below you will find a selection of games and other projects I have worked on.</p>
	<small class="note">{havamal[Math.floor(Math.random()*havamal.length)]} - The Hávamál</small>

	<section>
		{#each projects as project, index}
			<Project
				name={project.name}
				href={project.href}
				img={project.img}
				desc={project.desc}
				tools={project.tools}
				accent={accents[index % accents.length]}
			/>
		{/each}
	</section>

	<small>Thanks to Jackson Crawford for the Hávamál translations.</small>

</main>

<style>
	main {
		max-width: var(--content);
		margin: 0 auto;
		padding: 64px 24px 24px;
	}

	.studio-header {
		align-items: end;
		border-bottom: 1px solid var(--line);
		display: grid;
		gap: 24px;
		grid-template-columns: minmax(0, 1fr) auto;
		padding-bottom: 24px;
	}

	.title-block {
		text-align: left;
	}

	.title-block h1 {
		text-align: left;
	}

	.portrait {
		aspect-ratio: 1;
		background: var(--surface-muted);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		filter: saturate(0.92) contrast(0.98);
		height: 132px;
		object-fit: cover;
		padding: 6px;
		width: 132px;
	}

	.intro {
		color: var(--text-muted);
		font-size: clamp(1.2rem, 2.4vw, 1.85rem);
		font-weight: 600;
		line-height: 1.25;
		margin: 28px 0 10px;
		max-width: 780px;
		text-align: left;
	}

	.note {
		border-left: 3px solid var(--safety);
		display: block;
		max-width: 760px;
		padding-left: 12px;
		text-align: left;
	}

	section {
		display: grid;
		gap: 16px;
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		margin: 44px 0 24px;
	}

	main > small:last-child {
		display: block;
		text-align: left;
	}

	@media (max-width: 640px) {
		main {
			padding: 36px 16px 16px;
		}

		.studio-header {
			align-items: start;
			grid-template-columns: 1fr;
		}

		.portrait {
			height: 104px;
			width: 104px;
		}
	}
</style>
