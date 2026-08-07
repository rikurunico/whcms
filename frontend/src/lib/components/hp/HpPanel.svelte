<script lang="ts">
	import type { Snippet } from 'svelte';
	import { invalidateAll } from '$app/navigation';

	interface Props {
		title: string;
		/** Body content. */
		children: Snippet;
		/** Optional replacement for the default header tool icons. */
		tools?: Snippet;
		/** Remove default body padding (e.g. when the body is a full-bleed table). */
		flush?: boolean;
		/** Stable key for persisting the collapsed state (defaults to title). */
		id?: string;
		/**
		 * Override for the default Refresh button behavior (`invalidateAll()`).
		 * Use this when the page's data has its own server-side cache (e.g. the
		 * admin dashboard's 60s Redis cache) that a plain reload wouldn't bypass -
		 * pass a handler that busts that cache first, then reloads.
		 */
		onRefresh?: () => Promise<void> | void;
	}

	let { title, children, tools, flush = false, id, onRefresh }: Props = $props();

	// title/id are only ever read here, at mount, to compute a stable
	// storage key - this component never re-keys mid-life if a caller were
	// to change its title prop, which doesn't happen in practice (each panel
	// is mounted once with a fixed title/id for its lifetime).
	// svelte-ignore state_referenced_locally
	const storageKey = `hp-panel-collapsed:${id ?? title}`;

	// Collapsed state is persisted per admin (docs/DESIGN.md §9 "posisi tersimpan per
	// admin") since re-expanding is always one click away. Closed state is
	// intentionally session-only (plain $state, not persisted) - with no
	// "restore hidden widgets" affordance anywhere yet, persisting it forever
	// would let an admin close a widget away permanently by accident.
	let collapsed = $state(
		typeof localStorage !== 'undefined' && localStorage.getItem(storageKey) === '1'
	);
	let closed = $state(false);
	let reloading = $state(false);

	function toggleCollapse() {
		collapsed = !collapsed;
		if (typeof localStorage !== 'undefined') {
			if (collapsed) localStorage.setItem(storageKey, '1');
			else localStorage.removeItem(storageKey);
		}
	}

	function closePanel() {
		closed = true;
	}

	async function reload() {
		reloading = true;
		try {
			if (onRefresh) await onRefresh();
			else await invalidateAll();
		} finally {
			reloading = false;
		}
	}
</script>

{#if !closed}
	<section class="hp-panel">
		<div class="hp-panel-hd">
			<span class="title">{title}</span>
			<span class="tools">
				{#if tools}
					{@render tools()}
				{:else}
					<button
						type="button"
						class="hp-panel-tool"
						onclick={reload}
						disabled={reloading}
						aria-label="Refresh"
						title="Refresh"
					>
						<i class="fas fa-sync-alt" class:fa-spin={reloading} aria-hidden="true"></i>
					</button>
					<button
						type="button"
						class="hp-panel-tool"
						onclick={toggleCollapse}
						aria-label={collapsed ? 'Expand' : 'Collapse'}
						title={collapsed ? 'Expand' : 'Collapse'}
					>
						<i class={collapsed ? 'fas fa-chevron-down' : 'fas fa-chevron-up'} aria-hidden="true"
						></i>
					</button>
					<button
						type="button"
						class="hp-panel-tool"
						onclick={closePanel}
						aria-label="Close"
						title="Close"
					>
						<i class="fas fa-times" aria-hidden="true"></i>
					</button>
				{/if}
			</span>
		</div>
		{#if !collapsed}
			<div style={flush ? '' : 'padding:14px 12px'}>
				{@render children()}
			</div>
		{/if}
	</section>
{/if}

<style>
	.hp-panel-tool {
		background: none;
		border: none;
		padding: 0;
		margin: 0;
		color: inherit;
		font: inherit;
		font-size: inherit;
		line-height: 1;
		cursor: pointer;
	}
	.hp-panel-tool:disabled {
		cursor: default;
		opacity: 0.6;
	}
</style>
