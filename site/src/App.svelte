<script>
	import router from "page";
	import Home from './pages/Home.svelte';
	import About from './pages/About.svelte';
	import Poetry from './pages/Poetry.svelte';
	import Van from './pages/Van.svelte';
	import Feedback from './pages/Feedback.svelte';
	import AnalyzeWords from "./pages/AnalyzeWords.svelte";
	import UnknownWords from "./pages/UnknownWords.svelte";

	let page;

	router('/', () => page = Home);
	router('/about', () => page = About);
	router('/poetry', () => page = Poetry);
	router('/van', () => page = Van);
	router('/feedback', () => page = Feedback);
	router('/analyze-words', () => page = AnalyzeWords);
	router('/unknown-words', () => page = UnknownWords);

	router.start();
</script>

<svelte:component this={page} />

<footer>
	<small>Contact me at &nbsp;<img class="contact-image" src="img/em.png" alt="The place where I can be contacted"/></small>
	<a href="/about">About me</a>
	<small><i>Built with: Svelte</i></small>
</footer>

<style>
	footer {
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: 24px;
		margin: 24px;
		gap: 8px;
		border-top: 1px black solid;
	}

	footer small {
		text-align: center;
	}
	
	.contact-image {
		width: auto;
		height: 10px;
	}
</style>