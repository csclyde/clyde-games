<script>
	import router from 'page';
	import Admin from './pages/Admin.svelte';
 import PortalAuthGate from './components/PortalAuthGate.svelte';

	let tab = 'admin';
	let userID = '';

	router('/', () => router.redirect('/admin'));
	router('/admin', () => tab = 'admin');
	router('/marketing', () => tab = 'marketing');
	router('/metrics', () => tab = 'metrics');
	router('/feedback', () => tab = 'feedback');
	router('/crashes', () => tab = 'crashes');
	router('/savegames', () => router.redirect('/admin'));
	router('/save-games', () => router.redirect('/admin'));
	router('/tools', () => tab = 'tools');
	router('/users', () => router.redirect('/admin'));
	router('/user/:id', context => {
		userID = context.params.id;
		tab = 'user';
	});
	router.start();
</script>

<PortalAuthGate let:beginPasswordChange let:signOutAccount let:accountBusy><Admin {tab} {userID} {beginPasswordChange} {signOutAccount} {accountBusy} /></PortalAuthGate>
