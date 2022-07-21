<script>
	import Project from '../components/Project.svelte';
	import projects from '../data/projects.json';
	import havamal from '../data/havamal.json';
</script>

<main>
	<h1><u>CLYDE GAMES</u></h1>
	<img class='portrait' src="/img/casey.jpeg" alt="Casey at Lake Tahoe">
	<p>Below you will find a selection of games and other projects I have worked on.</p>
	<small>{havamal[Math.floor(Math.random()*havamal.length)]} - The Hávamál</small>

	<section>
		{#each projects as project}
			<Project
				name={project.name}
				href={project.href}
				img={project.img}
				desc={project.desc}
				tools={project.tools}
			/>
		{/each}
	</section>

	<small>Thanks to Jackson Crawford for the Hávamál translations.</small>

</main>

<style>
	main {
		text-align: center;
		padding: 1em;
		max-width: none;
		margin: 0 auto;
	}

	.portrait {
		width: auto;
		height: 124px;
		border-radius: 50%;
		margin-top: 16px;
	}

	p {
		font-size: 2em;
		font-weight: 600;
		margin: 16px;
	}

	section {
		display: flex;
		flex-direction: row;
		flex-wrap: wrap;
		justify-content: center;
	}
</style>