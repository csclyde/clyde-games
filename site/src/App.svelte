<script>
	import router from "page";
	import Home from './pages/Home.svelte';
	import About from './pages/About.svelte';
	import Poetry from './pages/Poetry.svelte';
	import Van from './pages/Van.svelte';
	import AnalyzeWords from "./pages/AnalyzeWords.svelte";
	import UnknownWords from "./pages/UnknownWords.svelte";

	let page;
	let pageProps = {};

	function setPage(component, props = {}) {
		page = component;
		pageProps = props;
	}

	router('/', () => setPage(Home));
	router('/about', () => setPage(About));
	router('/poetry', () => setPage(Poetry));
	router('/van', () => setPage(Van));
	for (const path of ['/admin', '/feedback', '/crashes', '/savegames', '/save-games', '/metrics']) {
		router(path, () => window.location.replace(`https://admin.clyde.games${path}`));
	}
	router('/words/analyze', () => setPage(AnalyzeWords));
	router('/words/unknown', () => setPage(UnknownWords));

	router.start();
</script>

<svelte:component this={page} {...pageProps} />

<footer>
	<small>Contact me at &nbsp;<img class="contact-image" src="/img/em.png" alt="The place where I can be contacted"/></small>
	<a href="/about">About me</a>
	<small><i>Built with: Svelte</i></small>
</footer>

<style>
	footer {
		display: flex;
		flex-direction: column;
		align-items: center;
		color: var(--text-soft);
		padding: 24px;
		margin: 48px auto 24px;
		gap: 8px;
		border-top: 1px solid var(--line);
		max-width: var(--content);
	}

	footer small {
		text-align: center;
	}
	
	.contact-image {
		width: auto;
		height: 10px;
	}
</style>
