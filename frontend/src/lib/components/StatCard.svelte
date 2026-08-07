<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		label: string;
		value: string | number;
		/** Small line under the value, e.g. a delta or hint. */
		sub?: string;
		icon?: Snippet;
		/** Left accent color. */
		accent?: 'primary' | 'green' | 'yellow' | 'red' | 'gray';
		href?: string;
	}

	let { label, value, sub, icon, accent = 'primary', href }: Props = $props();

	const accents: Record<NonNullable<Props['accent']>, string> = {
		primary: 'border-l-primary',
		green: 'border-l-success',
		yellow: 'border-l-warning',
		red: 'border-l-danger',
		gray: 'border-l-gray-400'
	};
</script>

{#snippet body()}
	<div class="flex items-center gap-4">
		{#if icon}
			<div
				class="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary"
			>
				{@render icon()}
			</div>
		{/if}
		<div class="min-w-0">
			<p class="truncate text-xs font-semibold tracking-wide text-gray-500 uppercase">{label}</p>
			<p class="mt-0.5 truncate text-2xl font-bold text-gray-800">{value}</p>
			{#if sub}
				<p class="mt-0.5 truncate text-xs text-gray-500">{sub}</p>
			{/if}
		</div>
	</div>
{/snippet}

{#if href}
	<a
		{href}
		class={`block rounded-lg border border-gray-200 border-l-4 bg-white p-4 shadow-sm transition hover:shadow ${accents[accent]}`}
	>
		{@render body()}
	</a>
{:else}
	<div
		class={`rounded-lg border border-gray-200 border-l-4 bg-white p-4 shadow-sm ${accents[accent]}`}
	>
		{@render body()}
	</div>
{/if}
