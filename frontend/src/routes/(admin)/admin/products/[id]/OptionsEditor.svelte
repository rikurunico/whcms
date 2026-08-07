<script lang="ts">
	import { enhance } from '$app/forms';
	import type { SubmitFunction } from '@sveltejs/kit';
	import HpModal from '$lib/components/hp/HpModal.svelte';
	import MoneyText from '$lib/components/MoneyText.svelte';
	import { toast } from '$lib/stores/toast.svelte';
	import { CYCLES, type OptionGroupRow, type OptionRow, type OptionValueRow } from '../catalog';

	interface Props {
		optionGroups: OptionGroupRow[];
		/** Load-time error message. */
		error?: string | null;
		/** Failed options-action error text (already translated). */
		actionError?: string | null;
	}

	let { optionGroups, error = null, actionError = null }: Props = $props();

	let busy = $state(false);

	function enhanceWith(successMessage: string, close: () => void): SubmitFunction {
		return () => {
			busy = true;
			return async ({ result, update }) => {
				busy = false;
				if (result.type === 'success') {
					close();
					toast.success(successMessage);
				}
				await update();
			};
		};
	}

	// group modal
	let groupModalOpen = $state(false);
	let editingGroup = $state<OptionGroupRow | null>(null);
	let gName = $state('');
	let gDesc = $state('');

	function openGroupCreate() {
		editingGroup = null;
		gName = '';
		gDesc = '';
		groupModalOpen = true;
	}
	function openGroupEdit(g: OptionGroupRow) {
		editingGroup = g;
		gName = g.name;
		gDesc = g.description;
		groupModalOpen = true;
	}

	// option modal
	let optionModalOpen = $state(false);
	let editingOption = $state<OptionRow | null>(null);
	let optionGroupId = $state(0);
	let oName = $state('');
	let oSort = $state(0);

	function openOptionCreate(g: OptionGroupRow) {
		editingOption = null;
		optionGroupId = g.id;
		oName = '';
		oSort = 0;
		optionModalOpen = true;
	}
	function openOptionEdit(o: OptionRow) {
		editingOption = o;
		optionGroupId = o.group_id;
		oName = o.name;
		oSort = o.sort;
		optionModalOpen = true;
	}

	// value modal
	let valueModalOpen = $state(false);
	let editingValue = $state<OptionValueRow | null>(null);
	let valueOptionId = $state(0);
	let vName = $state('');
	let vSort = $state(0);
	let vDeltas = $state<Record<string, number>>({});

	function emptyDeltas(): Record<string, number> {
		const d: Record<string, number> = {};
		for (const c of CYCLES) d[c] = 0;
		return d;
	}
	function openValueCreate(o: OptionRow) {
		editingValue = null;
		valueOptionId = o.id;
		vName = '';
		vSort = 0;
		vDeltas = emptyDeltas();
		valueModalOpen = true;
	}
	function openValueEdit(v: OptionValueRow) {
		editingValue = v;
		valueOptionId = v.option_id;
		vName = v.name;
		vSort = v.sort;
		vDeltas = { ...emptyDeltas(), ...(v.price_deltas ?? {}) };
		valueModalOpen = true;
	}

	const cycleLabels: Record<string, string> = {
		one_time: 'One time',
		monthly: 'Monthly',
		quarterly: 'Quarterly',
		semiannually: 'Semi-annually',
		annually: 'Annually',
		biennially: 'Biennially'
	};

	// delete confirm
	let confirmOpen = $state(false);
	let deleteKind = $state<'group' | 'option' | 'value'>('group');
	let deleteId = $state(0);
	let deleteName = $state('');
	let deleteFormEl = $state<HTMLFormElement | null>(null);

	const deleteAction = $derived(
		deleteKind === 'group'
			? '?/deleteOptionGroup'
			: deleteKind === 'option'
				? '?/deleteOption'
				: '?/deleteValue'
	);
	const deleteMessage = $derived(
		deleteKind === 'group'
			? `Delete option group "${deleteName}" with all its options and values?`
			: deleteKind === 'option'
				? `Delete option "${deleteName}" with all its values?`
				: `Delete value "${deleteName}"?`
	);

	function askDelete(kind: 'group' | 'option' | 'value', id: number, name: string) {
		deleteKind = kind;
		deleteId = id;
		deleteName = name;
		confirmOpen = true;
	}
</script>

<div class="hp-panel" data-testid="options-editor" style="padding:16px 20px">
	<div
		style="display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:10px;margin-bottom:14px"
	>
		<div>
			<h2 class="hp-h2" style="font-size:16px;margin:0 0 3px">Configurable Options</h2>
			<p class="hp-help" style="margin:0">
				Option groups are global — usable by every product during order configuration.
			</p>
		</div>
		<span data-testid="option-group-create-button">
			<button type="button" class="hp-btn hp-btn-primary" onclick={openGroupCreate}>
				<i class="fas fa-plus"></i>New Option Group
			</button>
		</span>
	</div>

	{#if error}
		<div class="hp-alert-red" data-testid="options-load-error">
			<i class="fas fa-exclamation-triangle" style="margin-right:8px"></i>{error}
		</div>
	{/if}

	{#if actionError}
		<div class="hp-alert-red" data-testid="options-action-error">
			<i class="fas fa-exclamation-triangle" style="margin-right:8px"></i>{actionError}
		</div>
	{/if}

	<div class="hp-scroll">
		<table class="hp-table">
			<thead>
				<tr>
					<th>Option / Values</th>
					<th class="c" style="width:80px">Sort</th>
					<th style="width:230px"></th>
				</tr>
			</thead>
			<tbody>
				{#each optionGroups as group (group.id)}
					<tr class="hp-group-row" data-testid={`row-option-group-${group.id}`}>
						<td colspan="3">
							<div
								style="display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:8px"
							>
								<span>
									{group.name}
									{#if group.description}
										<span style="font-weight:400;color:#777"> — {group.description}</span>
									{/if}
								</span>
								<span style="display:flex;gap:6px;flex-wrap:wrap">
									<span data-testid={`option-add-${group.id}`}>
										<button
											type="button"
											class="hp-btn hp-btn-xs"
											onclick={() => openOptionCreate(group)}
										>
											Add Option
										</button>
									</span>
									<span data-testid={`option-group-edit-${group.id}`}>
										<button
											type="button"
											class="hp-btn hp-btn-xs"
											onclick={() => openGroupEdit(group)}>Edit</button
										>
									</span>
									<span data-testid={`option-group-delete-${group.id}`}>
										<button
											type="button"
											class="hp-btn hp-btn-danger hp-btn-xs"
											onclick={() => askDelete('group', group.id, group.name)}
										>
											Delete
										</button>
									</span>
								</span>
							</div>
						</td>
					</tr>
					{#if group.options.length === 0}
						<tr>
							<td colspan="3" style="text-align:center;color:#999;padding:14px">
								No options in this group yet.
							</td>
						</tr>
					{:else}
						{#each group.options as option (option.id)}
							<tr class="hp-row" data-testid={`row-option-${option.id}`}>
								<td>
									<div style="font-weight:600;color:#444">{option.name}</div>
									{#if option.values.length === 0}
										<div style="color:#999;font-size:12px;margin-top:4px">No values yet.</div>
									{:else}
										<div style="display:flex;flex-wrap:wrap;gap:6px;margin-top:6px">
											{#each option.values as value (value.id)}
												<span
													data-testid={`row-option-value-${value.id}`}
													style="display:inline-flex;align-items:center;gap:6px;background:#eef2f7;color:#375a7f;border-radius:3px;padding:3px 4px 3px 8px;font-size:11.5px"
												>
													{value.name}
													{#each Object.entries(value.price_deltas ?? {}) as [cycle, delta] (cycle)}
														{#if delta !== 0}
															<span title={cycleLabels[cycle] ?? cycle}
																>+<MoneyText amount={delta} /></span
															>
														{/if}
													{/each}
													<button
														type="button"
														data-testid={`value-edit-${value.id}`}
														onclick={() => openValueEdit(value)}
														title="Edit"
														style="border:none;background:none;color:inherit;cursor:pointer;padding:1px 3px;font:inherit"
													>
														<i class="far fa-edit"></i>
													</button>
													<button
														type="button"
														data-testid={`value-delete-${value.id}`}
														onclick={() => askDelete('value', value.id, value.name)}
														title="Delete"
														style="border:none;background:none;color:inherit;cursor:pointer;padding:1px 3px;font:inherit"
													>
														<i class="far fa-trash-alt"></i>
													</button>
												</span>
											{/each}
										</div>
									{/if}
								</td>
								<td class="c" style="color:#999">{option.sort}</td>
								<td class="r" style="white-space:nowrap">
									<span data-testid={`value-add-${option.id}`}>
										<button
											type="button"
											class="hp-btn hp-btn-xs"
											onclick={() => openValueCreate(option)}
										>
											Add Value
										</button>
									</span>
									<span data-testid={`option-edit-${option.id}`}>
										<button
											type="button"
											class="hp-btn hp-btn-xs"
											onclick={() => openOptionEdit(option)}
										>
											Edit
										</button>
									</span>
									<span data-testid={`option-delete-${option.id}`}>
										<button
											type="button"
											class="hp-btn hp-btn-danger hp-btn-xs"
											onclick={() => askDelete('option', option.id, option.name)}
										>
											Delete
										</button>
									</span>
								</td>
							</tr>
						{/each}
					{/if}
				{:else}
					<tr>
						<td colspan="3" style="text-align:center;color:#999;padding:28px">
							No option groups yet. Create one like "Extra RAM" or "Server Location".
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>

<!-- group create/edit modal -->
<HpModal
	open={groupModalOpen}
	title={editingGroup ? 'Edit Option Group' : 'New Option Group'}
	onClose={() => (groupModalOpen = false)}
>
	<form
		method="POST"
		action={editingGroup ? '?/updateOptionGroup' : '?/createOptionGroup'}
		data-testid="option-group-form"
		use:enhance={enhanceWith(
			'Configurable options saved successfully.',
			() => (groupModalOpen = false)
		)}
	>
		{#if editingGroup}
			<input type="hidden" name="id" value={editingGroup.id} />
		{/if}
		<div class="hp-formrow">
			<label for="field-group-name">Group Name<span style="color:#d9534f"> *</span></label>
			<div class="hp-field" style="flex:1 1 auto">
				<input id="field-group-name" name="name" bind:value={gName} required class="hp-input" />
			</div>
		</div>
		<div class="hp-formrow top">
			<label for="field-group-description">Description</label>
			<div class="hp-field" style="flex:1 1 auto">
				<textarea
					id="field-group-description"
					name="description"
					rows="3"
					bind:value={gDesc}
					class="hp-textarea"></textarea>
			</div>
		</div>
		<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:14px">
			<button type="button" class="hp-btn" onclick={() => (groupModalOpen = false)} disabled={busy}
				>Cancel</button
			>
			<span data-testid="option-group-form-submit">
				<button type="submit" class="hp-btn hp-btn-primary" disabled={busy}
					>{busy ? 'Saving…' : 'Save'}</button
				>
			</span>
		</div>
	</form>
</HpModal>

<!-- option create/edit modal -->
<HpModal
	open={optionModalOpen}
	title={editingOption ? 'Edit Option' : 'Add Option'}
	onClose={() => (optionModalOpen = false)}
>
	<form
		method="POST"
		action={editingOption ? '?/updateOption' : '?/createOption'}
		data-testid="option-form"
		use:enhance={enhanceWith(
			'Configurable options saved successfully.',
			() => (optionModalOpen = false)
		)}
	>
		{#if editingOption}
			<input type="hidden" name="id" value={editingOption.id} />
		{:else}
			<input type="hidden" name="group_id" value={optionGroupId} />
		{/if}
		<div class="hp-formrow">
			<label for="field-option-name">Option Name<span style="color:#d9534f"> *</span></label>
			<div class="hp-field" style="flex:1 1 auto">
				<input id="field-option-name" name="name" bind:value={oName} required class="hp-input" />
			</div>
		</div>
		<div class="hp-formrow">
			<label for="field-option-sort">Sort</label>
			<div class="hp-field">
				<input
					id="field-option-sort"
					name="sort"
					type="number"
					bind:value={oSort}
					class="hp-input"
				/>
			</div>
		</div>
		<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:14px">
			<button type="button" class="hp-btn" onclick={() => (optionModalOpen = false)} disabled={busy}
				>Cancel</button
			>
			<span data-testid="option-form-submit">
				<button type="submit" class="hp-btn hp-btn-primary" disabled={busy}
					>{busy ? 'Saving…' : 'Save'}</button
				>
			</span>
		</div>
	</form>
</HpModal>

<!-- value create/edit modal -->
<HpModal
	open={valueModalOpen}
	title={editingValue ? 'Edit Value' : 'Add Value'}
	onClose={() => (valueModalOpen = false)}
>
	<form
		method="POST"
		action={editingValue ? '?/updateValue' : '?/createValue'}
		data-testid="option-value-form"
		use:enhance={enhanceWith(
			'Configurable options saved successfully.',
			() => (valueModalOpen = false)
		)}
	>
		{#if editingValue}
			<input type="hidden" name="id" value={editingValue.id} />
		{:else}
			<input type="hidden" name="option_id" value={valueOptionId} />
		{/if}
		<div class="hp-formrow">
			<label for="field-value-name">Value Name<span style="color:#d9534f"> *</span></label>
			<div class="hp-field" style="flex:1 1 auto">
				<input id="field-value-name" name="name" bind:value={vName} required class="hp-input" />
			</div>
		</div>
		<div class="hp-formrow">
			<label for="field-value-sort">Sort</label>
			<div class="hp-field">
				<input
					id="field-value-sort"
					name="sort"
					type="number"
					bind:value={vSort}
					class="hp-input"
				/>
			</div>
		</div>

		<p style="margin:14px 0 2px;font-size:13px;font-weight:600;color:#555">Price delta per cycle</p>
		<p class="hp-help" style="margin:0 0 8px">
			Additional price (Rp) per cycle. Leave empty or 0 for no surcharge.
		</p>
		<div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:8px 16px">
			{#each CYCLES as cycle (cycle)}
				<label
					style="display:flex;align-items:center;justify-content:space-between;gap:8px;font-size:13px;color:#555"
				>
					<span>{cycleLabels[cycle] ?? cycle}</span>
					<input
						type="number"
						name={`delta_${cycle}`}
						step="1"
						bind:value={vDeltas[cycle]}
						data-testid={`delta-${cycle}-input`}
						class="hp-input"
						style="width:130px"
					/>
				</label>
			{/each}
		</div>

		<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:14px">
			<button type="button" class="hp-btn" onclick={() => (valueModalOpen = false)} disabled={busy}
				>Cancel</button
			>
			<span data-testid="option-value-form-submit">
				<button type="submit" class="hp-btn hp-btn-primary" disabled={busy}
					>{busy ? 'Saving…' : 'Save'}</button
				>
			</span>
		</div>
	</form>
</HpModal>

<!-- delete confirm -->
<form
	method="POST"
	action={deleteAction}
	class="hidden"
	bind:this={deleteFormEl}
	use:enhance={enhanceWith('Deleted successfully.', () => (confirmOpen = false))}
>
	<input type="hidden" name="id" value={deleteId} />
</form>

<HpModal open={confirmOpen} title="Delete" onClose={() => (confirmOpen = false)}>
	<p style="color:#555;margin-bottom:16px">{deleteMessage}</p>
	<div style="display:flex;justify-content:flex-end;gap:8px">
		<button type="button" class="hp-btn" onclick={() => (confirmOpen = false)} disabled={busy}
			>Cancel</button
		>
		<button
			type="button"
			class="hp-btn hp-btn-danger"
			onclick={() => deleteFormEl?.requestSubmit()}
			disabled={busy}
		>
			{busy ? 'Deleting…' : 'Delete'}
		</button>
	</div>
</HpModal>

<style>
	.hidden {
		display: none;
	}
	.hp-btn-xs {
		padding: 3px 8px;
		font-size: 11.5px;
	}
</style>
