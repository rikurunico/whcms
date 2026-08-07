<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { TabItem } from './types';

	interface Props {
		tabs: TabItem[];
		/** Active tab id (bindable). Ignored for href tabs - pass the current path instead. */
		active?: string;
		onChange?: (id: string) => void;
		/** Optional panel content; receives the active tab id. */
		children?: Snippet<[string]>;
	}

	let { tabs, active = $bindable(tabs[0]?.id ?? ''), onChange, children }: Props = $props();

	function select(tab: TabItem) {
		if (tab.disabled) return;
		active = tab.id;
		onChange?.(tab.id);
	}
</script>

<div>
	<div class="border-b border-gray-200" role="tablist">
		<nav class="-mb-px flex flex-wrap gap-4">
			{#each tabs as tab (tab.id)}
				{@const isActive = active === tab.id}
				{@const cls = `inline-flex items-center gap-1.5 border-b-2 px-1 py-2.5 text-sm font-medium whitespace-nowrap ${
					isActive
						? 'border-primary text-primary'
						: 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'
				} ${tab.disabled ? 'cursor-not-allowed opacity-50' : ''}`}
				{#if tab.href}
					<a href={tab.href} class={cls} aria-current={isActive ? 'page' : undefined}>
						{tab.label}
					</a>
				{:else}
					<button
						type="button"
						role="tab"
						aria-selected={isActive}
						class={cls}
						disabled={tab.disabled}
						onclick={() => select(tab)}
					>
						{tab.label}
					</button>
				{/if}
			{/each}
		</nav>
	</div>
	{#if children}
		<div class="pt-4" role="tabpanel">
			{@render children(active)}
		</div>
	{/if}
</div>
