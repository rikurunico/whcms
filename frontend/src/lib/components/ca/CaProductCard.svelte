<script lang="ts">
	/**
	 * Twenty-One "Browse our Products/Services" card (DESIGN §7b): a white card with
	 * a centered title, an optional subtitle, and a full-width OUTLINE-primary CTA
	 * (matching WHMCS's portal-home cards). Distinct from the §8 store product card
	 * (gray header + feature list + green "Order Now"), which the /order catalog
	 * renders with its own markup.
	 */
	interface Props {
		name: string;
		/** Optional descriptive line shown under the title (e.g. domain cards). */
		description?: string;
		orderHref: string;
		ctaLabel: string;
		/** Optional data-testid applied to the card root. */
		testid?: string;
	}

	let { name, description, orderHref, ctaLabel, testid }: Props = $props();
</script>

<div class="ca-browse-card" data-testid={testid}>
	<h3 class="ca-browse-title">{name}</h3>
	{#if description}
		<p class="ca-browse-desc">{description}</p>
	{/if}
	<a class="ca-btn ca-btn-outline-primary ca-btn-block ca-browse-cta" href={orderHref}>{ctaLabel}</a
	>
</div>
