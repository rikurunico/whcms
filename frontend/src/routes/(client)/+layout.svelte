<script lang="ts">
	import CaShell from '$lib/components/ca/CaShell.svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
</script>

<CaShell variant="client" user={data.user}>
	{@render children()}
</CaShell>
