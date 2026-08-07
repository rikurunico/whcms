<script lang="ts">
	/** Twenty-One breadcrumb strip (DESIGN §6): full-width #E9ECEF band, last item plain text. */
	interface Crumb {
		label: string;
		href?: string;
	}

	interface Props {
		items: Crumb[];
	}

	let { items }: Props = $props();
</script>

{#if items.length}
	<nav class="ca-breadcrumb" aria-label="Breadcrumb">
		<div class="ca-container">
			{#each items as item, i (item.label + i)}
				{#if item.href && i < items.length - 1}
					<a class="ca-breadcrumb-item" href={item.href}>{item.label}</a>
				{:else}
					<span class="ca-breadcrumb-item active" aria-current="page">{item.label}</span>
				{/if}
			{/each}
		</div>
	</nav>
{/if}
