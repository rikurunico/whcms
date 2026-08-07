<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		description?: string;
		icon?: Snippet;
		/** Call-to-action area (button/link). */
		action?: Snippet;
	}

	let { title, description, icon, action }: Props = $props();
</script>

<div class="flex flex-col items-center justify-center gap-2 px-6 py-12 text-center">
	{#if icon}
		<div class="mb-2 text-gray-300">{@render icon()}</div>
	{:else}
		<svg
			class="mb-2 h-12 w-12 text-gray-300"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="1.5"
			aria-hidden="true"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M20 13V7a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6m16 0v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-4m16 0h-5l-1.5 2h-3L9 13H4"
			/>
		</svg>
	{/if}
	<h3 class="text-sm font-semibold text-gray-700">{title}</h3>
	{#if description}
		<p class="max-w-sm text-sm text-gray-500">{description}</p>
	{/if}
	{#if action}
		<div class="mt-3">{@render action()}</div>
	{/if}
</div>
