<script>
	import Project from './components/Project.svelte';
</script>

<main>
	<h1><u>CLYDE GAMES</u></h1>
	<img class='portrait' src="/img/casey.jpeg" alt="Casey at Lake Tahoe">
	<p>Below you will find a selection of games and other projects I have worked on.</p>
	<small>Fire is best for mortals, and sunshine— and also good health if you have it, and living beyond reproach - Hávamál</small>

	<div class="project-list">
		<Project 
			name="Into the Necrovale" 
			href="https://clydegames.itch.io/necrovale"
			img="/img/necrovale.png"
			desc="An action RPG about fighting hordes of monsters with powerful combinations of items."
			tools="Haxe, Heaps.io, pass: lemmeplay"
		/>

		<Project 
			name="VectorLance" 
			href="https://clydegames.itch.io/vectorlance"
			img="/img/vl.png"
			desc="An arcade-style game about popping bubbles in space."
			tools="Haxe, Heaps.io"
		/>
	
		<Project 
			name="Van Conversion" 
			href="/van"
			img="/img/van.png"
			desc="A camper van conversion, to allow easy travel and exploration."
			tools="Baltic Birch, Polyiso Insulation"
		/>
	
		<Project 
			name="Frauki's Adventure!"
			href="https://store.steampowered.com/app/1570850/Fraukis_Adventure/"
			img="/img/frauki.png"
			desc="A 2D game about exploring and fighting difficult enemies."
			tools="JavaScript, Phaser.io, Electron"
		/>

		<Project 
			name="The Stupefied Cretin" 
			href="https://stupefiedcretin.blogspot.com/"
			img="/img/cretin.jpeg"
			desc="A blog about modernity."
			tools="Blogger"
		/>
	
		<Project 
			name="ARC" 
			href="https://clydegames.itch.io/arc"
			img="/img/arc.png"
			desc="A game made in 48 hours, where you use magnets to climb a shaft."
			tools="JavaScript, Phaser.io"
		/>

		<Project 
			name="A Neocortical Theory" 
			href="https://docs.google.com/document/d/e/2PACX-1vR36VpZ6VpXY63o3_MMqPnrSnRXhQtDu79TFigDkL18Ui0iDGFtXpibn8YuZKKFV_olGhcjQiptRera/pub"
			img="/img/neo.png"
			desc="A theory on how the neocortex can be understood algorithmically."
			tools="Research, Thinking"
		/>

		<Project 
			name="Poetry" 
			href="/poetry"
			img="/img/lyre.png"
			desc="Various poems I have written over the years."
			tools="Notebook"
		/>
	</div>

	<hr/>

	<small>Contact me at </small>
	<img class="contact-image" src="img/em.png" alt="The place where I can be contacted"/>
	<br/>
	<a href="/about">About me</a>
</main>

<style>
	main {
		text-align: center;
		padding: 1em;
		max-width: none;
		margin: 0 auto;
	}

	h1 {
		color: black;
		font-size: 6em;
		font-weight: 800;
	}

	.portrait {
		width: auto;
		height: 164px;
		border-radius: 12px;
		position: absolute;
		right: 16px;
		top: 16px;
	}

	p {
		font-size: 2em;
		font-weight: 600;
	}

	.project-list {
		display: flex;
		flex-direction: row;
		flex-wrap: wrap;
		justify-content: center;
	}

	.contact-image {
		width: auto;
		height: 10px;
	}
</style>