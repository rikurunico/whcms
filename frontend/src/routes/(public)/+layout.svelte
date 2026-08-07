<script lang="ts">
	import { page } from '$app/state';
	import CaShell from '$lib/components/ca/CaShell.svelte';
	import { t } from '$lib/i18n';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	// Auth pages get the centered auth-card variant; everything else is the plain
	// portal column. (The /order tree breaks out to its own store-variant layout.)
	const AUTH_ROUTES = [
		'/login',
		'/register',
		'/forgot-password',
		'/reset-password',
		'/verify-email',
		'/install'
	];
	const variant = $derived(
		AUTH_ROUTES.some((r) => page.url.pathname === r || page.url.pathname.startsWith(r + '/'))
			? 'auth'
			: 'portal'
	);

	// Breadcrumb strip (WHMCS Twenty-One): "Portal Home" on the home page and
	// "Portal Home / <Page>" on the other portal pages. Auth pages get none.
	const SUB_CRUMB: Record<string, string> = {
		'/announcements': 'portal.nav.announcements',
		'/knowledgebase': 'portal.nav.knowledgebase',
		'/network-status': 'portal.nav.networkStatus',
		'/contact': 'portal.nav.contactUs'
	};
	const breadcrumb = $derived.by((): { label: string; href?: string }[] => {
		if (variant === 'auth') return [];
		const path = page.url.pathname;
		if (path === '/') return [{ label: t('portal.breadcrumb.portalHome') }];
		const crumbs: { label: string; href?: string }[] = [
			{ label: t('portal.breadcrumb.portalHome'), href: '/' }
		];
		const subKey = Object.keys(SUB_CRUMB).find((r) => path === r || path.startsWith(r + '/'));
		if (subKey) crumbs.push({ label: t(SUB_CRUMB[subKey]) });
		return crumbs;
	});
</script>

<CaShell {variant} user={data.user} {breadcrumb}>
	{@render children()}
</CaShell>
