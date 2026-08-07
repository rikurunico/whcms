<script lang="ts">
	interface Props {
		/** ISO string, epoch millis, or Date. Falsy values render an em dash. */
		value: string | number | Date | null | undefined;
		/** 'date' -> 12 Jan 2026 · 'datetime' -> 12 Jan 2026 14.30 */
		mode?: 'date' | 'datetime';
		locale?: string;
		class?: string;
	}

	let { value, mode = 'date', locale = 'id-ID', class: className = '' }: Props = $props();

	const date = $derived(value ? new Date(value) : null);

	const formatted = $derived.by(() => {
		if (!date || Number.isNaN(date.getTime())) return '—';
		const options: Intl.DateTimeFormatOptions = {
			timeZone: 'Asia/Jakarta',
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			...(mode === 'datetime' ? { hour: '2-digit', minute: '2-digit' } : {})
		};
		return new Intl.DateTimeFormat(locale, options).format(date);
	});
</script>

<span
	class={className}
	title={date && !Number.isNaN(date.getTime()) ? date.toISOString() : undefined}>{formatted}</span
>
