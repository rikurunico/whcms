<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		type?: 'info' | 'success' | 'warning' | 'error';
		title?: string;
		dismissible?: boolean;
		children: Snippet;
	}

	let { type = 'info', title, dismissible = false, children }: Props = $props();

	let visible = $state(true);

	const styles: Record<NonNullable<Props['type']>, string> = {
		info: 'border-info/40 bg-info/10 text-info',
		success: 'border-success/40 bg-success/10 text-success',
		warning: 'border-warning/40 bg-warning/10 text-warning',
		error: 'border-danger/40 bg-danger/10 text-danger'
	};
</script>

{#if visible}
	<div
		class={`flex items-start gap-3 rounded-md border px-4 py-3 text-sm ${styles[type]}`}
		role="alert"
	>
		<div class="grow">
			{#if title}
				<p class="font-semibold">{title}</p>
			{/if}
			<div class={title ? 'mt-0.5' : ''}>{@render children()}</div>
		</div>
		{#if dismissible}
			<button
				type="button"
				class="shrink-0 opacity-70 hover:opacity-100"
				aria-label="Dismiss"
				onclick={() => (visible = false)}
			>
				&times;
			</button>
		{/if}
	</div>
{/if}
