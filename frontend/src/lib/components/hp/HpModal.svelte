<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		open: boolean;
		title: string;
		onClose: () => void;
		children: Snippet;
	}

	let { open, title, onClose, children }: Props = $props();

	function onKey(e: KeyboardEvent) {
		if (open && e.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={onKey} />

{#if open}
	<div class="hp-modal-scrim">
		<!-- Full-bleed backdrop button: clicking outside the dialog closes it. -->
		<button type="button" class="hp-modal-backdrop" aria-label="Close" onclick={onClose}></button>
		<div class="hp-modal" role="dialog" aria-modal="true" aria-label={title} tabindex="-1">
			<div class="hp-modal-hd">
				<span>{title}</span>
				<button type="button" class="hp-modal-x" onclick={onClose} aria-label="Close">
					<i class="fas fa-times" aria-hidden="true"></i>
				</button>
			</div>
			<div class="hp-modal-bd">
				{@render children()}
			</div>
		</div>
	</div>
{/if}

<style>
	.hp-modal-scrim {
		position: fixed;
		inset: 0;
		z-index: 90;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 80px;
	}
	.hp-modal-backdrop {
		position: fixed;
		inset: 0;
		border: none;
		padding: 0;
		background: rgba(0, 0, 0, 0.45);
		cursor: default;
	}
	.hp-modal {
		position: relative;
		background: #fff;
		border-radius: 5px;
		width: 560px;
		max-width: 92vw;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
		overflow: hidden;
	}
	.hp-modal-hd {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 14px 18px;
		border-bottom: 1px solid #eee;
		font-size: 17px;
		font-weight: 600;
		color: #333;
	}
	.hp-modal-x {
		border: none;
		background: none;
		cursor: pointer;
		color: #999;
		font-size: 16px;
	}
	.hp-modal-bd {
		padding: 18px;
	}
</style>
