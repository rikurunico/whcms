<script lang="ts">
	import { page } from '$app/state';
	import CaShell from '$lib/components/ca/CaShell.svelte';
	import CaStoreSidebar from '$lib/components/ca/CaStoreSidebar.svelte';
	import { t } from '$lib/i18n';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	const pathname = $derived(page.url.pathname);
	const activeGroupSlug = $derived(page.url.searchParams.get('group') ?? '');

	const visibleGroups = $derived(
		data.groups.filter((g) => !g.hidden).sort((a, b) => a.sort - b.sort || a.id - b.id)
	);

	// WHMCS-style: the sidebar lists categories (product groups), not a flat
	// catalog - clicking one filters /order to just that group. On the store
	// index with no ?group=, the first category is implicitly active (same
	// fallback the page itself uses), matching a fresh visit landing on it.
	const categories = $derived(
		visibleGroups.map((g, i) => ({
			label: g.name,
			href: `/order?group=${encodeURIComponent(g.slug)}`,
			active: pathname === '/order' && (activeGroupSlug ? activeGroupSlug === g.slug : i === 0),
			testid: `product-group-${g.slug}`
		}))
	);

	const actions = $derived([
		{ icon: 'fas fa-globe', label: t('portal.nav.registerDomain'), href: '/order/domain' },
		{ icon: 'fas fa-exchange-alt', label: t('portal.nav.transferDomain'), href: '/order/domain' },
		{ icon: 'fas fa-shopping-cart', label: t('orderfe.nav.cart'), href: '/order/cart' }
	]);
</script>

<CaShell variant="store" user={data.user}>
	{#snippet sidebar()}
		<CaStoreSidebar {categories} {actions} categoriesTestId="product-groups" />
	{/snippet}
	{@render children()}
</CaShell>
