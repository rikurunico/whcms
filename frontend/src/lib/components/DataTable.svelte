<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import { t } from '$lib/i18n';
	import EmptyState from './EmptyState.svelte';
	import Pagination from './Pagination.svelte';
	import type { Column, SortDir } from './types';

	interface Props {
		columns: Column[];
		rows: T[];
		loading?: boolean;
		/** Server pagination - provide all three (+ onPageChange) to render the footer. */
		page?: number;
		perPage?: number;
		total?: number;
		onPageChange?: (page: number) => void;
		onPerPageChange?: (perPage: number) => void;
		/** Server sorting state; headers with column.sortable trigger onSortChange. */
		sortKey?: string;
		sortDir?: SortDir;
		onSortChange?: (key: string, dir: SortDir) => void;
		/** Stable row key; defaults to `row.id` when present, else the index. */
		rowKey?: (row: T, index: number) => string | number;
		onRowClick?: (row: T) => void;
		/** Custom cell renderer; fallback prints String(row[column.key]). */
		cell?: Snippet<[{ row: T; column: Column; value: unknown }]>;
		/** Filter/toolbar area rendered above the table. */
		filter?: Snippet;
		/** Custom empty state; fallback renders <EmptyState>. */
		empty?: Snippet;
		emptyTitle?: string;
		emptyDescription?: string;
	}

	let {
		columns,
		rows,
		loading = false,
		page,
		perPage,
		total,
		onPageChange,
		onPerPageChange,
		sortKey,
		sortDir = 'asc',
		onSortChange,
		rowKey,
		onRowClick,
		cell,
		filter,
		empty,
		emptyTitle,
		emptyDescription
	}: Props = $props();

	const paginated = $derived(page !== undefined && perPage !== undefined && total !== undefined);
	const skeletonRows = $derived(Math.min(perPage ?? 5, 8));

	function valueOf(row: T, column: Column): unknown {
		return (row as Record<string, unknown>)[column.key];
	}

	function keyOf(row: T, index: number): string | number {
		if (rowKey) return rowKey(row, index);
		const id = (row as Record<string, unknown>).id;
		return typeof id === 'string' || typeof id === 'number' ? id : index;
	}

	function alignClass(column: Column): string {
		if (column.align === 'right') return 'text-right';
		if (column.align === 'center') return 'text-center';
		return 'text-left';
	}

	function toggleSort(column: Column) {
		if (!column.sortable || !onSortChange) return;
		const dir: SortDir = sortKey === column.key && sortDir === 'asc' ? 'desc' : 'asc';
		onSortChange(column.key, dir);
	}
</script>

<div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
	{#if filter}
		<div class="border-b border-gray-200 bg-gray-50/60 px-4 py-3">{@render filter()}</div>
	{/if}

	<div class="overflow-x-auto">
		<table class="min-w-full divide-y divide-gray-200 text-sm">
			<thead class="bg-gray-50">
				<tr>
					{#each columns as column (column.key)}
						<th
							scope="col"
							class={`px-4 py-2.5 text-xs font-semibold tracking-wide text-gray-600 uppercase ${alignClass(column)} ${column.class ?? ''}`}
							aria-sort={sortKey === column.key
								? sortDir === 'asc'
									? 'ascending'
									: 'descending'
								: undefined}
						>
							{#if column.sortable && onSortChange}
								<button
									type="button"
									class="inline-flex items-center gap-1 uppercase hover:text-primary"
									onclick={() => toggleSort(column)}
								>
									{column.label}
									<span class="text-[10px] leading-none" aria-hidden="true">
										{#if sortKey === column.key}
											{sortDir === 'asc' ? '▲' : '▼'}
										{:else}
											⇅
										{/if}
									</span>
								</button>
							{:else}
								{column.label}
							{/if}
						</th>
					{/each}
				</tr>
			</thead>
			<tbody class="divide-y divide-gray-100">
				{#if loading}
					{#each { length: skeletonRows } as _, i (i)}
						<tr>
							{#each columns as column (column.key)}
								<td class="px-4 py-3">
									<div class="h-4 animate-pulse rounded bg-gray-200"></div>
								</td>
							{/each}
						</tr>
					{/each}
				{:else if rows.length === 0}
					<tr>
						<td colspan={columns.length}>
							{#if empty}
								{@render empty()}
							{:else}
								<EmptyState
									title={emptyTitle ?? t('common.noResults')}
									description={emptyDescription ?? t('table.empty')}
								/>
							{/if}
						</td>
					</tr>
				{:else}
					{#each rows as row, i (keyOf(row, i))}
						<tr
							class={`hover:bg-gray-50 ${onRowClick ? 'cursor-pointer' : ''}`}
							onclick={onRowClick ? () => onRowClick?.(row) : undefined}
						>
							{#each columns as column (column.key)}
								<td class={`px-4 py-2.5 text-gray-700 ${alignClass(column)} ${column.class ?? ''}`}>
									{#if cell}
										{@render cell({ row, column, value: valueOf(row, column) })}
									{:else}
										{valueOf(row, column) ?? '—'}
									{/if}
								</td>
							{/each}
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>

	{#if paginated && !loading}
		<div class="border-t border-gray-200 px-4 py-3">
			<Pagination
				page={page ?? 1}
				perPage={perPage ?? 25}
				total={total ?? 0}
				{onPageChange}
				{onPerPageChange}
			/>
		</div>
	{/if}
</div>
