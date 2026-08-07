<script lang="ts">
	import type { Snippet } from 'svelte';

	/** Twenty-One card/panel (DESIGN §8): `.ca-card` with an optional light header. */
	interface Props {
		/** Simple text header (ignored if the `header` snippet is provided). */
		title?: string;
		/** Custom header content (overrides `title`). */
		header?: Snippet;
		children: Snippet;
	}

	let { title, header, children }: Props = $props();
</script>

<section class="ca-card">
	{#if header}
		<div class="ca-card-header">{@render header()}</div>
	{:else if title}
		<div class="ca-card-header">{title}</div>
	{/if}
	<div class="ca-card-body">
		{@render children()}
	</div>
</section>
