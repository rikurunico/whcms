<script lang="ts">
	interface Props {
		value: string | number;
		label: string;
		/** Font Awesome icon class, e.g. "fas fa-shopping-cart". */
		icon: string;
		/** Card background color. */
		color: string;
		href?: string;
	}

	let { value, label, icon, color, href }: Props = $props();
</script>

{#snippet body()}
	<div class="hp-stat-ico"><i class={icon} aria-hidden="true"></i></div>
	<div class="hp-stat-bd">
		<div class="hp-stat-val">{value}</div>
		<div class="hp-stat-lbl">{label}</div>
	</div>
{/snippet}

{#if href}
	<a class="hp-stat" style={`background:${color}`} {href}>
		{@render body()}
	</a>
{:else}
	<div class="hp-stat" style={`background:${color}`}>
		{@render body()}
	</div>
{/if}
