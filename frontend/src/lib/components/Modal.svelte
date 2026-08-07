<script lang="ts">
	import type { Snippet } from 'svelte';
	import { t } from '$lib/i18n';

	interface Props {
		open?: boolean;
		title?: string;
		size?: 'sm' | 'md' | 'lg' | 'xl';
		/** Called when the user dismisses via ✕, backdrop, or Escape. */
		onClose?: () => void;
		children: Snippet;
		footer?: Snippet;
	}

	let { open = $bindable(false), title, size = 'md', onClose, children, footer }: Props = $props();

	const sizes: Record<NonNullable<Props['size']>, string> = {
		sm: 'max-w-sm',
		md: 'max-w-lg',
		lg: 'max-w-2xl',
		xl: 'max-w-4xl'
	};

	function close() {
		open = false;
		onClose?.();
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) close();
	}
</script>

<svelte:window {onkeydown} />

{#if open}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4 sm:p-8">
		<button
			type="button"
			class="fixed inset-0 cursor-default bg-gray-900/50"
			aria-label={t('action.close')}
			onclick={close}
			tabindex="-1"
		></button>
		<div
			class={`relative z-10 mt-8 w-full rounded-lg bg-white shadow-xl ${sizes[size]}`}
			role="dialog"
			aria-modal="true"
			aria-label={title}
		>
			{#if title}
				<div class="flex items-center justify-between border-b border-gray-200 px-5 py-3.5">
					<h2 class="text-base font-semibold text-gray-800">{title}</h2>
					<button
						type="button"
						class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
						aria-label={t('action.close')}
						onclick={close}
					>
						<svg
							class="h-5 w-5"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
						>
							<path stroke-linecap="round" d="M6 6l12 12M18 6L6 18" />
						</svg>
					</button>
				</div>
			{/if}
			<div class="px-5 py-4">
				{@render children()}
			</div>
			{#if footer}
				<div
					class="flex justify-end gap-2 rounded-b-lg border-t border-gray-200 bg-gray-50 px-5 py-3"
				>
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}
