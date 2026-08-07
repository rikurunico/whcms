<script lang="ts">
	import { enhance } from '$app/forms';
	import { toast } from '$lib/stores/toast.svelte';
	import { untrack } from 'svelte';

	interface Addon {
		id: number;
		key: string;
		name: string;
		price: number;
		active: boolean;
	}

	interface ActionData {
		id?: number;
		success?: boolean;
		errorMessage?: string;
	}

	interface Props {
		addon: Addon;
		result?: ActionData | null;
	}

	let { addon, result = null }: Props = $props();

	let price = $state(untrack(() => addon.price));
	let active = $state(untrack(() => addon.active));
	let saving = $state(false);

	const rowError = $derived(
		result && result.id === addon.id && result.success !== true
			? (result.errorMessage ?? null)
			: null
	);
</script>

<form
	method="POST"
	action="?/save"
	class="addon-row"
	data-testid={`row-domain-addon-${addon.id}`}
	use:enhance={() => {
		saving = true;
		return async ({ result: r, update }) => {
			saving = false;
			if (r.type === 'success') toast.success(`${addon.name} saved successfully.`);
			// reset: false - see RegistrarCard.svelte for why the default
			// update() must not be allowed to blank out these bound inputs.
			await update({ reset: false });
		};
	}}
>
	<input type="hidden" name="id" value={addon.id} />

	<div class="addon-name">
		<i class="fas fa-puzzle-piece" style="margin-right:8px;color:#888"></i>{addon.name}
	</div>

	<label class="hp-checkline addon-active">
		<input
			type="checkbox"
			name="active"
			bind:checked={active}
			data-testid={`domain-addon-active-${addon.id}`}
		/>
		Active
	</label>

	<div class="addon-price">
		<div class="hp-field-label">Price (annual, Rp)</div>
		<input
			type="number"
			min="0"
			step="1"
			class="hp-input"
			name="price"
			bind:value={price}
			data-testid={`domain-addon-price-${addon.id}`}
		/>
	</div>

	<button
		type="submit"
		class="hp-btn hp-btn-primary"
		disabled={saving}
		data-testid={`domain-addon-save-${addon.id}`}
	>
		{saving ? 'Saving…' : 'Save'}
	</button>

	{#if rowError}
		<div class="hp-alert-red addon-error">
			<i class="fas fa-exclamation-triangle" style="margin-right:8px"></i>{rowError}
		</div>
	{/if}
</form>

<style>
	.addon-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 18px;
		padding: 14px 16px;
		border-bottom: 1px solid #eee;
	}
	.addon-row:last-child {
		border-bottom: none;
	}
	.addon-name {
		flex: 1 1 220px;
		font-weight: 600;
		color: #333;
	}
	.addon-active {
		flex: 0 0 auto;
	}
	.addon-price {
		flex: 0 0 200px;
	}
	.addon-error {
		flex: 1 1 100%;
		margin: 0;
	}
	@media (max-width: 640px) {
		.addon-row {
			flex-direction: column;
			align-items: stretch;
		}
	}
</style>
