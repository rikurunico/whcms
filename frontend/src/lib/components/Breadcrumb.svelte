<script lang="ts">
	import type { BreadcrumbItem } from './types';

	interface Props {
		items: BreadcrumbItem[];
	}

	let { items }: Props = $props();
</script>

<nav aria-label="Breadcrumb" class="mb-4">
	<ol class="flex flex-wrap items-center gap-1.5 text-sm text-gray-500">
		{#each items as item, i (i)}
			{#if i > 0}
				<li aria-hidden="true" class="text-gray-300">/</li>
			{/if}
			<li>
				{#if item.href && i < items.length - 1}
					<a href={item.href} class="hover:text-primary hover:underline">{item.label}</a>
				{:else}
					<span
						class="font-medium text-gray-700"
						aria-current={i === items.length - 1 ? 'page' : undefined}
					>
						{item.label}
					</span>
				{/if}
			</li>
		{/each}
	</ol>
</nav>
