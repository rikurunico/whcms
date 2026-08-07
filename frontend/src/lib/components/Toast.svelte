<script lang="ts">
	import { toast, type ToastType } from '$lib/stores/toast.svelte';

	const styles: Record<ToastType, string> = {
		success: 'border-success/30 bg-success text-white',
		error: 'border-danger/30 bg-danger text-white',
		info: 'border-info/30 bg-info text-white'
	};
</script>

{#if toast.items.length > 0}
	<div
		class="pointer-events-none fixed top-4 right-4 z-100 flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2"
		aria-live="polite"
	>
		{#each toast.items as item (item.id)}
			<div
				class={`pointer-events-auto flex items-start gap-3 rounded-md border px-4 py-3 text-sm shadow-lg ${styles[item.type]}`}
				role="status"
			>
				<span class="grow">{item.message}</span>
				<button
					type="button"
					class="shrink-0 opacity-80 hover:opacity-100"
					aria-label="Dismiss"
					onclick={() => toast.dismiss(item.id)}
				>
					&times;
				</button>
			</div>
		{/each}
	</div>
{/if}
