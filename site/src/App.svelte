<script>
	import router from "page";
	import Home from './Home.svelte';
	import About from './About.svelte';
	import Poetry from './Poetry.svelte';
	import Van from './Van.svelte';
	import Feedback from './Feedback.svelte';

	let page;

	router('/', () => page = Home);
	router('/about', () => page = About);
	router('/poetry', () => page = Poetry);
	router('/van', () => page = Van);
	router('/feedback', () => page = Feedback);

	router.start();
</script>

<svelte:component this={page} />

<style>
</style>