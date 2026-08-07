<script lang="ts">
	import { page } from '$app/state';
	import HpFooter from '$lib/components/hp/HpFooter.svelte';
	import HpNavbar from '$lib/components/hp/HpNavbar.svelte';
	import HpSidebar from '$lib/components/hp/HpSidebar.svelte';
	import { sidebarKind } from '$lib/components/hp/nav';
	import { i18n } from '$lib/i18n';
	import '$lib/styles/hostpanel.css';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let drawerOpen = $state(false);
	let sidebarMin = $state(false);

	const pathname = $derived(page.url.pathname);
	const search = $derived(page.url.search);
	const kind = $derived(sidebarKind(pathname));
	// The System Settings hub (WHMCS "configlanding") is a full-bleed page with
	// its own built-in search/category navigation - it replaces the standard
	// contextual sidebar rather than stacking alongside it.
	const showSidebar = $derived(pathname !== '/admin/settings/overview');
	const userName = $derived(data.user?.name ?? 'Admin');
	const onlineStaff = $derived(data.onlineStaff ?? []);

	function toggleLocale() {
		i18n.setLocale(i18n.locale === 'id' ? 'en' : 'id');
	}
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Open+Sans:wght@400;600;700;800&display=swap"
		rel="stylesheet"
	/>
	<link
		rel="stylesheet"
		href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css"
		integrity="sha384-t1nt8BQoYMLFN5p42tRAtuAAFQaCQODekUVeKKZrEnEyp4H2R0RHFz0KWpmj7i8g"
		crossorigin="anonymous"
	/>
</svelte:head>

<div class="hp-admin">
	<div class="hp-shell">
		<HpNavbar
			{userName}
			locale={i18n.locale}
			onHamburger={() => (drawerOpen = !drawerOpen)}
			onToggleLocale={toggleLocale}
		/>

		<div class="hp-shell-body">
			{#if showSidebar}
				{#if drawerOpen}
					<button
						class="hp-drawer-scrim"
						aria-label="Close menu"
						tabindex="-1"
						onclick={() => (drawerOpen = false)}
					></button>
				{/if}

				<HpSidebar
					{kind}
					{pathname}
					{search}
					{userName}
					{onlineStaff}
					open={drawerOpen}
					minimised={sidebarMin}
					onCloseDrawer={() => (drawerOpen = false)}
					onToggleMinimise={() => (sidebarMin = !sidebarMin)}
				/>
			{/if}

			<main class="hp-main">
				<div class="hp-content">
					{@render children()}
				</div>
			</main>
		</div>

		<HpFooter />
	</div>
</div>
