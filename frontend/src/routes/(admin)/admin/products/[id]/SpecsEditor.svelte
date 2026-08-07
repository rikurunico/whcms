<script lang="ts">
	import { CYCLES } from '../catalog';
	import type { SpecWithPricingRow, SpecPriceRow } from './+page.server';

	/** The 9 fields shared by the "Add spec" form values and an existing SpecRow -
	 *  lets one snippet render both the create table and an in-place edit table. */
	interface SpecFieldValues {
		key: string;
		label: string;
		provision_key: string;
		unit: string;
		included_qty: number;
		min_qty: number;
		max_qty: number;
		step_qty: number;
		default_qty: number;
	}

	/** Submitted "Add spec" form values, re-echoed back after a failed submit
	 *  (this form is a plain server round-trip, not Svelte-bound state) so a
	 *  validation error doesn't wipe out what the admin already typed. */
	interface SpecFormValues extends SpecFieldValues {
		allow_unlimited: boolean;
	}

	let {
		specs,
		configurable,
		error = null,
		actionError = null,
		fieldErrors = {},
		formValues = null
	}: {
		specs: SpecWithPricingRow[];
		configurable: boolean;
		error?: string | null;
		actionError?: string | null;
		fieldErrors?: Record<string, string>;
		formValues?: SpecFormValues | null;
	} = $props();

	const v = $derived<SpecFormValues>(
		formValues ?? {
			key: '',
			label: '',
			provision_key: 'disk',
			unit: 'gb',
			included_qty: 0,
			min_qty: 0,
			max_qty: 0,
			step_qty: 1,
			default_qty: 0,
			allow_unlimited: false
		}
	);

	const PROVISION_KEYS = [
		'disk',
		'bandwidth',
		'addon_domains',
		'subdomains',
		'parked_domains',
		'email_accounts',
		'databases',
		'ftp_accounts'
	];
	const UNITS = ['gb', 'mb', 'count'];

	function priceFor(row: SpecWithPricingRow, cycle: string): SpecPriceRow | undefined {
		return row.pricing.find((p) => p.cycle === cycle);
	}

	// Editing an existing spec used to have no UI at all - only create (the
	// table below) and delete were wired up, even though the backend's
	// updateSpec action already worked. Toggling this per-row swaps the
	// read-only summary for the exact same table as "Add a spec knob", so
	// editing looks and behaves identically to creating.
	let editingSpecId = $state<number | null>(null);
</script>

{#snippet specFieldsTable(
	idPrefix: string,
	values: SpecFieldValues,
	errors: Record<string, string>
)}
	<div class="hp-scroll">
		<table class="hp-table add-spec-table">
			<thead>
				<tr>
					<th>Key<span class="req">*</span></th>
					<th>Label</th>
					<th>Provision key</th>
					<th>Unit</th>
					<th>Included</th>
					<th>Min</th>
					<th>Max (0 = ∞)</th>
					<th>Step</th>
					<th>Default</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<td>
						<input
							id={`field-${idPrefix}-key`}
							class="hp-input"
							name="key"
							placeholder="disk"
							required
							value={values.key}
							class:hp-input-error={!!errors.key}
						/>
						{#if errors.key}<div class="field-error">{errors.key}</div>{/if}
					</td>
					<td>
						<input
							id={`field-${idPrefix}-label`}
							class="hp-input"
							name="label"
							placeholder="Disk space"
							value={values.label}
							class:hp-input-error={!!errors.label}
						/>
						{#if errors.label}<div class="field-error">{errors.label}</div>{/if}
					</td>
					<td>
						<select
							id={`field-${idPrefix}-provision_key`}
							class="hp-select"
							name="provision_key"
							value={values.provision_key}
							class:hp-input-error={!!errors.provision_key}
						>
							{#each PROVISION_KEYS as k (k)}<option value={k}>{k}</option>{/each}
						</select>
						{#if errors.provision_key}<div class="field-error">{errors.provision_key}</div>{/if}
					</td>
					<td>
						<select
							id={`field-${idPrefix}-unit`}
							class="hp-select"
							name="unit"
							value={values.unit}
							class:hp-input-error={!!errors.unit}
						>
							{#each UNITS as u (u)}<option value={u}>{u}</option>{/each}
						</select>
						{#if errors.unit}<div class="field-error">{errors.unit}</div>{/if}
					</td>
					<td>
						<input
							class="hp-input"
							type="number"
							min="0"
							id={`field-${idPrefix}-included_qty`}
							name="included_qty"
							value={values.included_qty}
							class:hp-input-error={!!errors.included_qty}
						/>
						{#if errors.included_qty}<div class="field-error">{errors.included_qty}</div>{/if}
					</td>
					<td>
						<input
							class="hp-input"
							type="number"
							min="0"
							id={`field-${idPrefix}-min_qty`}
							name="min_qty"
							value={values.min_qty}
							class:hp-input-error={!!errors.min_qty}
						/>
						{#if errors.min_qty}<div class="field-error">{errors.min_qty}</div>{/if}
					</td>
					<td>
						<input
							class="hp-input"
							type="number"
							min="0"
							id={`field-${idPrefix}-max_qty`}
							name="max_qty"
							value={values.max_qty}
							class:hp-input-error={!!errors.max_qty}
						/>
						{#if errors.max_qty}<div class="field-error">{errors.max_qty}</div>{/if}
					</td>
					<td>
						<input
							class="hp-input"
							type="number"
							min="1"
							id={`field-${idPrefix}-step_qty`}
							name="step_qty"
							value={values.step_qty}
							class:hp-input-error={!!errors.step_qty}
						/>
						{#if errors.step_qty}<div class="field-error">{errors.step_qty}</div>{/if}
					</td>
					<td>
						<input
							class="hp-input"
							type="number"
							min="0"
							id={`field-${idPrefix}-default_qty`}
							name="default_qty"
							value={values.default_qty}
							class:hp-input-error={!!errors.default_qty}
						/>
						{#if errors.default_qty}<div class="field-error">{errors.default_qty}</div>{/if}
					</td>
				</tr>
			</tbody>
		</table>
	</div>
{/snippet}

<section class="hp-panel" data-testid="specs-editor" style="padding:16px 20px">
	<h2 class="hp-h2" style="font-size:16px;margin:0 0 14px">
		<i class="fas fa-sliders-h" style="margin-right:8px"></i>Dynamic Specs
	</h2>
	{#if !configurable}
		<div class="hp-alert-yellow" data-testid="specs-not-configurable">
			<i class="fas fa-info-circle" style="margin-right:8px"></i>
			Enable "Configurable (customer sets specs)" on the product and pick a provisioning module, then
			save, to define spec knobs here.
		</div>
	{/if}
	{#if error}
		<div class="hp-alert-red">{error}</div>
	{/if}
	{#if actionError}
		<div class="hp-alert-red" data-testid="specs-action-error">{actionError}</div>
	{/if}

	<!-- Existing specs -->
	{#if specs.length > 0}
		<div class="specs-list">
			{#each specs as row (row.spec.id)}
				<div class="spec-row" data-testid={`admin-spec-row-${row.spec.id}`}>
					{#if editingSpecId === row.spec.id}
						<form
							method="POST"
							action="?/updateSpec"
							class="add-spec-form no-border"
							data-testid={`admin-spec-edit-form-${row.spec.id}`}
						>
							<input type="hidden" name="spec_id" value={row.spec.id} />
							<input type="hidden" name="sort" value={row.spec.sort} />
							{@render specFieldsTable(`espec-${row.spec.id}`, row.spec, {})}
							<div class="add-spec-footer">
								<label class="hp-checkline" style="padding:0">
									<input
										type="checkbox"
										name="allow_unlimited"
										checked={row.spec.allow_unlimited}
									/>
									Allow unlimited
								</label>
								<span style="display:flex;gap:8px">
									<button type="button" class="hp-btn" onclick={() => (editingSpecId = null)}>
										Cancel
									</button>
									<button
										type="submit"
										class="hp-btn hp-btn-primary"
										data-testid={`admin-spec-edit-submit-${row.spec.id}`}
									>
										Save
									</button>
								</span>
							</div>
						</form>
					{:else}
						<div class="spec-row-head">
							<strong>{row.spec.label || row.spec.key}</strong>
							<code class="spec-code">{row.spec.provision_key} · {row.spec.unit}</code>
							<span class="spec-limits">
								min {row.spec.min_qty} · max {row.spec.max_qty || '∞'} · step {row.spec.step_qty} · included
								{row.spec.included_qty} · default {row.spec.default_qty}
								{#if row.spec.allow_unlimited}· unlimited allowed{/if}
							</span>
							<span style="flex:1 1 auto"></span>
							<span data-testid={`spec-edit-${row.spec.id}`}>
								<button
									type="button"
									class="hp-btn hp-btn-sm"
									onclick={() => (editingSpecId = row.spec.id)}
								>
									<i class="far fa-edit"></i> Edit
								</button>
							</span>
						</div>
					{/if}

					<!-- per-cycle pricing -->
					<div class="pricing-grid">
						<span class="pricing-head">Cycle</span>
						<span class="pricing-head">Unit price (IDR)</span>
						<span class="pricing-head">Unlimited price (IDR)</span>
						<span></span>
						{#each CYCLES as cycle (cycle)}
							{@const pr = priceFor(row, cycle)}
							<form
								method="POST"
								action="?/saveSpecPricing"
								class="pricing-row-form"
								data-testid={`admin-spec-pricing-${row.spec.id}-${cycle}`}
							>
								<input type="hidden" name="spec_id" value={row.spec.id} />
								<input type="hidden" name="cycle" value={cycle} />
								<span class="cycle-cell">{cycle}</span>
								<input
									class="hp-input"
									type="number"
									min="0"
									name="unit_price"
									value={pr?.unit_price ?? 0}
								/>
								<input
									class="hp-input"
									type="number"
									min="0"
									name="unlimited_price"
									value={pr?.unlimited_price ?? 0}
								/>
								<button type="submit" class="hp-btn hp-btn-sm">Save</button>
							</form>
						{/each}
					</div>

					<form method="POST" action="?/deleteSpec" style="margin-top:10px">
						<input type="hidden" name="spec_id" value={row.spec.id} />
						<button type="submit" class="hp-btn hp-btn-danger hp-btn-sm">
							<i class="fas fa-trash-alt"></i> Delete spec
						</button>
					</form>
				</div>
			{/each}
		</div>
	{:else if configurable}
		<p class="no-specs-hint">No specs yet. Add the first knob below.</p>
	{/if}

	<!-- Add spec -->
	{#if configurable}
		<form method="POST" action="?/createSpec" class="add-spec-form" data-testid="admin-spec-form">
			<h3 class="add-spec-title">Add a spec knob</h3>
			{@render specFieldsTable('spec', v, fieldErrors)}
			<div class="add-spec-footer">
				<label class="hp-checkline" style="padding:0">
					<input type="checkbox" name="allow_unlimited" checked={v.allow_unlimited} /> Allow unlimited
				</label>
				<button type="submit" class="hp-btn hp-btn-primary" data-testid="admin-spec-submit">
					<i class="fas fa-plus"></i> Add spec
				</button>
			</div>
		</form>
	{/if}
</section>

<style>
	.specs-list {
		display: flex;
		flex-direction: column;
		gap: 14px;
		margin-bottom: 18px;
	}
	.spec-row {
		border: 1px solid #e2e2e2;
		border-radius: 4px;
		padding: 14px;
	}
	.spec-row-head {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 10px;
		margin-bottom: 10px;
	}
	.spec-code {
		background: #f2f2f2;
		border-radius: 3px;
		padding: 1px 6px;
		font-size: 12px;
	}
	.spec-limits {
		color: #888;
		font-size: 12px;
	}
	.pricing-grid {
		display: grid;
		grid-template-columns: 120px 1fr 1fr auto;
		gap: 8px 10px;
		align-items: center;
		font-size: 13px;
		overflow-x: auto;
	}
	.pricing-head {
		font-weight: 600;
		color: #444;
	}
	.pricing-row-form {
		display: contents;
	}
	.cycle-cell {
		font-weight: 600;
		color: #444;
		text-transform: capitalize;
	}
	.no-specs-hint {
		color: #888;
		font-size: 13px;
		margin-bottom: 18px;
	}
	.add-spec-form {
		border-top: 1px solid #e2e2e2;
		padding-top: 16px;
	}
	.add-spec-form.no-border {
		border-top: none;
		padding-top: 0;
		margin-bottom: 12px;
	}
	.add-spec-title {
		font-size: 14px;
		font-weight: 600;
		color: #444;
		margin: 0 0 12px;
	}
	.add-spec-table th {
		white-space: nowrap;
		padding: 12px 14px;
	}
	.add-spec-table td {
		vertical-align: top;
		min-width: 110px;
		padding: 12px 14px;
	}
	.req {
		color: #d9534f;
	}
	.field-error {
		color: #c43c35;
		font-size: 11px;
		margin-top: 4px;
	}
	.add-spec-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		margin-top: 14px;
	}
</style>
