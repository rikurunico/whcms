<script lang="ts">
	/**
	 * Twenty-One price display - the canonical `Rp1.000.000,-` nominal (see
	 * $lib/money) plus an optional cycle label below (DESIGN §13).
	 */
	import { formatIDR } from '$lib/money';

	interface Props {
		amount: number;
		/** Optional cycle label, e.g. "Monthly". */
		cycle?: string;
	}

	let { amount, cycle }: Props = $props();

	const formatted = $derived(formatIDR(amount));
</script>

<span class="ca-price-block">
	<span class="ca-price-amount">{formatted}</span>
	{#if cycle}
		<span class="ca-price-cycle">{cycle}</span>
	{/if}
</span>
