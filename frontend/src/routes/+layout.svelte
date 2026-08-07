<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import Toast from '$lib/components/Toast.svelte';
	import { i18n } from '$lib/i18n';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	// Apply the server-resolved locale before render (SSR) and keep it in sync on navigation.
	const applyLocale = () => i18n.setLocale(data.locale, { persist: false });
	applyLocale();
	$effect(applyLocale);

	// Mark the document once client-side hydration is live. The e2e fixture
	// waits for this after page.goto: on slow machines a click can otherwise
	// land on server-rendered markup before its event listeners exist and be
	// silently lost (tabs, modals, page-size selects).
	$effect(() => {
		document.documentElement.dataset.hydrated = '1';
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<Toast />

{@render children()}
