<script lang="ts">
	/** Twenty-One help/account tile (DESIGN §7c/§7d): FA icon + label, colored top border. */
	type Border = 'teal' | 'red' | 'yellow' | 'gray' | 'green' | 'dark';

	interface Props {
		/** Font Awesome class, e.g. "fas fa-bullhorn". */
		icon: string;
		label: string;
		href: string;
		border?: Border;
		/** Optional data-testid applied to the tile anchor. */
		testid?: string;
	}

	let { icon, label, href, border = 'gray', testid }: Props = $props();
</script>

<a class="ca-tile ca-tile--{border}" {href} data-testid={testid}>
	<i class="ca-tile-icon {icon}" aria-hidden="true"></i>
	<span class="ca-tile-label">{label}</span>
</a>
