<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		loading?: boolean;
		disabled?: boolean;
		type?: 'button' | 'submit' | 'reset';
		variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
		size?: 'sm' | 'md';
		class?: string;
		onclick?: (e: MouseEvent) => void;
		children: Snippet;
	}

	let {
		loading = false,
		disabled = false,
		type = 'button',
		variant = 'primary',
		size = 'md',
		class: className = '',
		onclick,
		children
	}: Props = $props();

	const variants: Record<NonNullable<Props['variant']>, string> = {
		primary: 'bg-primary text-white hover:bg-primary-dark focus-visible:outline-primary',
		secondary:
			'border border-gray-300 bg-white text-gray-700 hover:bg-gray-50 focus-visible:outline-gray-400',
		danger: 'bg-danger text-white hover:bg-danger/85 focus-visible:outline-danger',
		ghost: 'text-primary hover:bg-primary/10 focus-visible:outline-primary'
	};

	const sizes: Record<NonNullable<Props['size']>, string> = {
		sm: 'px-2.5 py-1.5 text-xs',
		md: 'px-4 py-2 text-sm'
	};
</script>

<button
	{type}
	class={`inline-flex items-center justify-center gap-2 rounded-md font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-60 ${variants[variant]} ${sizes[size]} ${className}`}
	disabled={disabled || loading}
	aria-busy={loading}
	{onclick}
>
	{#if loading}
		<svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4H4z" />
		</svg>
	{/if}
	{@render children()}
</button>
