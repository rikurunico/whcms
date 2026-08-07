<script lang="ts">
	import { goto } from '$app/navigation';
	import { page as pageState } from '$app/state';

	const PAGE_SIZE_OPTIONS = [10, 25, 50, 100, 500, 1000];

	interface Props {
		page: number;
		perPage: number;
		total: number;
		/** Query param names, for pages with more than one independent pager (e.g. servers ?gpage=). */
		pageParam?: string;
		perPageParam?: string;
		testid?: string;
	}

	let {
		page,
		perPage,
		total,
		pageParam = 'page',
		perPageParam = 'per_page',
		testid
	}: Props = $props();

	const hasPrev = $derived(page > 1);
	const hasNext = $derived(page * perPage < total);

	function gotoParam(param: string, value: number, resetPage = false) {
		const url = new URL(pageState.url);
		url.searchParams.set(param, String(value));
		if (resetPage) url.searchParams.set(pageParam, '1');
		goto(url, { keepFocus: true, noScroll: true });
	}

	function changePerPage(e: Event) {
		gotoParam(perPageParam, Number((e.currentTarget as HTMLSelectElement).value), true);
	}
</script>

<div class="hp-pager" data-testid={testid}>
	<div class="hp-pager-nav">
		{#if hasPrev}
			<button type="button" onclick={() => gotoParam(pageParam, page - 1)}>« Previous Page</button>
		{:else}
			<span class="muted">« Previous Page</span>
		{/if}
		<span class="cur">{page}</span>
		{#if hasNext}
			<button type="button" onclick={() => gotoParam(pageParam, page + 1)}>Next Page »</button>
		{:else}
			<span class="muted">Next Page »</span>
		{/if}
	</div>
	<label class="hp-pager-size">
		<select
			class="hp-select"
			value={perPage}
			onchange={changePerPage}
			data-testid="page-size-select"
		>
			{#each PAGE_SIZE_OPTIONS as opt (opt)}
				<option value={opt}>{opt}</option>
			{/each}
		</select>
		per page
	</label>
</div>
