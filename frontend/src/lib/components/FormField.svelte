<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import type { SelectOption } from './types';

	interface Props {
		label: string;
		name: string;
		type?: 'text' | 'email' | 'password' | 'number' | 'date' | 'select' | 'textarea' | 'checkbox';
		value?: string | number | boolean;
		error?: string;
		hint?: string;
		placeholder?: string;
		required?: boolean;
		disabled?: boolean;
		autocomplete?: HTMLInputAttributes['autocomplete'];
		/** Options for type="select". */
		options?: SelectOption[];
		rows?: number;
	}

	let {
		label,
		name,
		type = 'text',
		value = $bindable(),
		error,
		hint,
		placeholder,
		required = false,
		disabled = false,
		autocomplete,
		options = [],
		rows = 4
	}: Props = $props();

	const id = $derived(`field-${name}`);

	// Function bindings keep the polymorphic `value` prop type-safe per control.
	const getText = () => (value === undefined || value === null ? '' : String(value));
	const setText = (v: string) => (value = v);
	const getNumber = () => (typeof value === 'number' ? value : Number(value ?? 0));
	const setNumber = (v: number) => (value = v);
	const getChecked = () => value === true;
	const setChecked = (v: boolean) => (value = v);

	const base = $derived(
		`block w-full rounded-md border bg-white px-3 py-2 text-sm text-gray-800 shadow-sm placeholder:text-gray-400 focus:outline-2 focus:outline-offset-0 disabled:cursor-not-allowed disabled:bg-gray-50 ${
			error
				? 'border-danger focus:outline-danger/60'
				: 'border-gray-300 focus:border-primary focus:outline-primary/40'
		}`
	);
</script>

<div class="mb-4">
	{#if type === 'checkbox'}
		<label class="flex items-start gap-2 text-sm text-gray-700" for={id}>
			<input
				{id}
				{name}
				type="checkbox"
				class="mt-0.5 h-4 w-4 rounded border-gray-300 accent-primary"
				bind:checked={getChecked, setChecked}
				{required}
				{disabled}
			/>
			<span>
				{label}
				{#if required}<span class="text-danger" aria-hidden="true">*</span>{/if}
			</span>
		</label>
	{:else}
		<label class="mb-1 block text-sm font-medium text-gray-700" for={id}>
			{label}
			{#if required}<span class="text-danger" aria-hidden="true">*</span>{/if}
		</label>

		{#if type === 'select'}
			<select {id} {name} class={base} bind:value={getText, setText} {required} {disabled}>
				{#if placeholder}
					<option value="" disabled>{placeholder}</option>
				{/if}
				{#each options as option (option.value)}
					<option value={option.value}>{option.label}</option>
				{/each}
			</select>
		{:else if type === 'textarea'}
			<textarea
				{id}
				{name}
				class={base}
				bind:value={getText, setText}
				{placeholder}
				{required}
				{disabled}
				{rows}></textarea>
		{:else if type === 'number'}
			<input
				{id}
				{name}
				type="number"
				class={base}
				bind:value={getNumber, setNumber}
				{placeholder}
				{required}
				{disabled}
			/>
		{:else if type === 'date'}
			<input
				{id}
				{name}
				type="date"
				class={base}
				bind:value={getText, setText}
				{required}
				{disabled}
			/>
		{:else if type === 'password'}
			<input
				{id}
				{name}
				type="password"
				class={base}
				bind:value={getText, setText}
				{placeholder}
				{required}
				{disabled}
				{autocomplete}
			/>
		{:else if type === 'email'}
			<input
				{id}
				{name}
				type="email"
				class={base}
				bind:value={getText, setText}
				{placeholder}
				{required}
				{disabled}
				{autocomplete}
			/>
		{:else}
			<input
				{id}
				{name}
				type="text"
				class={base}
				bind:value={getText, setText}
				{placeholder}
				{required}
				{disabled}
				{autocomplete}
			/>
		{/if}
	{/if}

	{#if error}
		<p class="mt-1 text-xs text-danger" role="alert">{error}</p>
	{:else if hint}
		<p class="mt-1 text-xs text-gray-500">{hint}</p>
	{/if}
</div>
