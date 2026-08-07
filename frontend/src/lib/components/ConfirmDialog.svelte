<script lang="ts">
	import { t } from '$lib/i18n';
	import LoadingButton from './LoadingButton.svelte';
	import Modal from './Modal.svelte';

	interface Props {
		open?: boolean;
		title?: string;
		message?: string;
		confirmLabel?: string;
		cancelLabel?: string;
		/** Styles the confirm button red for destructive actions. */
		danger?: boolean;
		/** Shows a spinner on the confirm button while the action runs. */
		loading?: boolean;
		onConfirm: () => void;
		onCancel?: () => void;
	}

	let {
		open = $bindable(false),
		title,
		message,
		confirmLabel,
		cancelLabel,
		danger = false,
		loading = false,
		onConfirm,
		onCancel
	}: Props = $props();

	function cancel() {
		open = false;
		onCancel?.();
	}
</script>

<Modal bind:open title={title ?? t('confirm.title')} size="sm" onClose={onCancel}>
	<p class="text-sm text-gray-600">{message ?? t('confirm.message')}</p>

	{#snippet footer()}
		<LoadingButton variant="secondary" onclick={cancel} disabled={loading}>
			{cancelLabel ?? t('action.cancel')}
		</LoadingButton>
		<LoadingButton variant={danger ? 'danger' : 'primary'} {loading} onclick={onConfirm}>
			{confirmLabel ?? t('action.confirm')}
		</LoadingButton>
	{/snippet}
</Modal>
