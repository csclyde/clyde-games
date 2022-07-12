<script>
	let steps = [
		{
			img: "/img/van1.jpg",
			desc: "Started with an empty cargo van. Took out all the leftover hardware, cleaned up, and added some insulation to the walls.",
		}, {
			img: "/img/van2.jpg",
			desc: "Insulated the floor with rigid foam board, then cut plywood to fit the space. The plywood is bolted to the van floor to prevent catastrophe.",
		}, {
			img: "/img/van3.jpg",
			desc: "Cut and taped down a vinyl flooring. Lightweight, waterproof, and decently resilient.",
		}, {
			img: "/img/van4.jpg",
			desc: "Cut a hole in the roof for the ceiling fan. The point of no return.",
		}, {
			img: "/img/van5.jpg",
			desc: "Trying to visualize next steps.",
		}, {
			img: "/img/van6.jpg",
			desc: "Bolted birch planks to the steel frame of the van, then ran birch planks between them for a bed frame.",
		}, {
			img: "/img/van7.jpg",
			desc: "Ran slats across the planks, and then added a full-sized mattress.",
		}, {
			img: "/img/van8.jpg",
			desc: "Beginning work on the cabinetry underneath the bed.",
		}, {
			img: "/img/van9.jpg",
			desc: "Completed in the back. Milk-crates for organization, plenty of room for storage. Long items can live on the left side.",
		}, {
			img: "/img/van10.jpg",
			desc: "First drawer for the cabin area. Finnicky work but satisfying.",
		}, {
			img: "/img/van11.jpg",
			desc: "The larger, main drawers built and installed.",
		}, {
			img: "/img/van12.jpg",
			desc: "Face plates and cabinet doors going on. Made with baltic birch, finished with polyurethane. Router template for the handles.",
		}, {
			img: "/img/van13.jpg",
			desc: "Cut and installed an RV window. It's a pop-open, double-paned window. Came with a retractable shade and screen.",
		}, {
			img: "/img/van14.jpg",
			desc: "Finishing up other face-plates for the cabinetry.",
		}, {
			img: "/img/van15.jpg",
			desc: "Installed 100 Watts of solar on the roof. Plenty to keep me charged when staying in one place for a while.",
		}, {
			img: "/img/van16.jpg",
			desc: "Sink cabinet, curtains, window shade.",
		}, {
			img: "/img/van20.jpg",
			desc: "Inside the sink cabinet. A simple fresh and grey water tank supply the sink, with a hand pump faucet.",
		}, {
			img: "/img/van17.jpg",
			desc: "Swivel seat on the passenger side. A nice place to sit and work, and creates significantly more interior space in the van.",
		}, {
			img: "/img/van18.jpg",
			desc: "After using the van for a year or so, I decided to decrease the size of the sink and sink cabinet, and increase the size of the bench. This makes it much more comfortable, and I didn't really need a large sink area anyway. This also gives all the electrical components a better home, under the bench. I added an inverter under there too, for 120V power.",
		}, {
			img: "/img/van22.jpg",
			desc: "Added a 10 gallon water tank, electric pump, and an exterior fill port to the cargo area. Built a slat shelf over top to prevent it all from getting crushed. The pump routes forward to the sink cabinet, to replace the hand-pump faucet.",
		}, {
			img: "/img/van19.jpg",
			desc: "The 12V pump is tucked away in the cargo area, with the tank itself directly behind.",
		}, {
			img: "/img/van23.jpg",
			desc: "Ran a pipe back from the pump for a shower. Very nice after a long hike, especially because the water tank stays cool in its location.",
		}, {
			img: "/img/van24.jpg",
			desc: "The Van in action. It's worked beautifully as a mobile campsite, and allowed me to travel a great deal more than I could have otherwise. Importantly for me, it allows me to take a trip with no planning. There's always somewhere to park, somewhere to get water, somewhere to work.",
		},
	]
</script>

<main>
	<h1>Camper Van Conversion</h1>
	<p>This is the process I went through to build my camper van.</p>

	<div>
		{#each steps as step}
		<div class="step-container">
			<img src={step.img} alt="A step in the conversion">
			<p>{ step.desc }</p>
		</div>
		{/each}
	</div>
</main>

<style>
	div {
		display: flex;
		flex-direction: column;
		flex-wrap: wrap;
		gap: 16px;
	}

	.step-container {
		display: flex;
		flex-direction: row;
		flex-wrap: nowrap;
	}

	img {
		width: 500px;
		height: 500px;
		object-fit: cover;
		border-radius: 12px;
		flex-shrink: 0;
	}

	p {
		font-size: large;
	}
</style>