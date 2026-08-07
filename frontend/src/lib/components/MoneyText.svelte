<script lang="ts">
	import { formatIDR } from '$lib/money';

	interface Props {
		/** Whole-rupiah amount (backend int64). */
		amount: number;
		/** Accepted for API compatibility; the app is IDR-only (CONTRACTS.md §3). */
		currency?: string;
		class?: string;
	}

	let { amount, class: className = '' }: Props = $props();

	// Canonical app-wide format: "Rp1.000.000,-" (see $lib/money).
	const formatted = $derived(formatIDR(amount));
</script>

<span class={`tabular-nums ${className}`}>{formatted}</span>
