<script lang="ts">
	import { t } from '$lib/i18n';

	const PAGE_SIZE_OPTIONS = [10, 25, 50, 100, 500, 1000];

	interface Props {
		page: number;
		perPage: number;
		total: number;
		onPageChange?: (page: number) => void;
		onPerPageChange?: (perPage: number) => void;
	}

	let { page, perPage, total, onPageChange, onPerPageChange }: Props = $props();

	function changePerPage(e: Event) {
		onPerPageChange?.(Number((e.currentTarget as HTMLSelectElement).value));
	}

	const totalPages = $derived(Math.max(1, Math.ceil(total / Math.max(1, perPage))));
	const from = $derived(total === 0 ? 0 : (page - 1) * perPage + 1);
	const to = $derived(Math.min(page * perPage, total));

	/** Compact page window: 1 … p-1 p p+1 … N */
	const pages = $derived.by(() => {
		const window = new Set<number>([1, totalPages, page - 1, page, page + 1]);
		const list = [...window].filter((p) => p >= 1 && p <= totalPages).sort((a, b) => a - b);
		const out: (number | '…')[] = [];
		let prev = 0;
		for (const p of list) {
			if (prev && p - prev > 1) out.push('…');
			out.push(p);
			prev = p;
		}
		return out;
	});

	function goto(p: number) {
		if (p < 1 || p > totalPages || p === page) return;
		onPageChange?.(p);
	}
</script>

<nav class="flex flex-wrap items-center justify-between gap-3 text-sm" aria-label="Pagination">
	<div class="flex flex-wrap items-center gap-3">
		<p class="text-gray-500">{t('table.showing', { from, to, total })}</p>
		{#if onPerPageChange}
			<label class="flex items-center gap-1.5 text-gray-500">
				<select
					class="rounded-md border border-gray-300 bg-white px-2 py-1 text-sm"
					value={perPage}
					onchange={changePerPage}
					data-testid="page-size-select"
				>
					{#each PAGE_SIZE_OPTIONS as opt (opt)}
						<option value={opt}>{opt}</option>
					{/each}
				</select>
				{t('table.perPage')}
			</label>
		{/if}
	</div>
	<div class="flex items-center gap-1">
		<button
			type="button"
			class="rounded-md border border-gray-300 bg-white px-2.5 py-1.5 text-gray-600 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
			disabled={page <= 1}
			onclick={() => goto(page - 1)}
		>
			{t('action.previous')}
		</button>
		{#each pages as p, i (i)}
			{#if p === '…'}
				<span class="px-2 text-gray-400">…</span>
			{:else}
				<button
					type="button"
					class={`min-w-9 rounded-md border px-2.5 py-1.5 ${
						p === page
							? 'border-primary bg-primary font-semibold text-white'
							: 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'
					}`}
					aria-current={p === page ? 'page' : undefined}
					onclick={() => goto(p)}
				>
					{p}
				</button>
			{/if}
		{/each}
		<button
			type="button"
			class="rounded-md border border-gray-300 bg-white px-2.5 py-1.5 text-gray-600 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
			disabled={page >= totalPages}
			onclick={() => goto(page + 1)}
		>
			{t('action.next')}
		</button>
	</div>
</nav>
