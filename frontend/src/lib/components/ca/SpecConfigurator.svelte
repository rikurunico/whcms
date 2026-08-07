<script lang="ts">
	import CaPanel from './CaPanel.svelte';
	import { t } from '$lib/i18n';
	import { formatIDR } from '$lib/money';
	import { specAmount, type BillingCycle, type SpecChoice } from '$lib/stores/cart.svelte';
	import type { SpecRow } from '../../../routes/(public)/order/product/[slug]/+page.server';

	let {
		specs,
		cycle,
		choices = $bindable({})
	}: {
		specs: SpecRow[];
		cycle: BillingCycle | null;
		/** key -> chosen qty / unlimited (shared with the parent via bind:choices). */
		choices?: Record<string, SpecChoice>;
	} = $props();

	function cur(s: SpecRow): SpecChoice {
		return choices[s.key] ?? { qty: s.default_qty, unlimited: false };
	}

	function setQty(s: SpecRow, qty: number) {
		choices = { ...choices, [s.key]: { qty, unlimited: cur(s).unlimited } };
	}

	function setUnlimited(s: SpecRow, unlimited: boolean) {
		choices = { ...choices, [s.key]: { qty: cur(s).qty, unlimited } };
	}

	function unitLabel(u: string): string {
		return u === 'gb' ? 'GB' : u === 'mb' ? 'MB' : '';
	}
</script>

{#if specs.length > 0}
	<CaPanel title={t('orderfe.product.specs')}>
		<div class="space-y-4">
			{#each specs as spec (spec.key)}
				{@const c = cur(spec)}
				{@const amount = specAmount(spec, c, cycle)}
				<div class="ca-formrow" style="margin-bottom:0;">
					<div style="display:flex;justify-content:space-between;align-items:baseline;gap:12px;">
						<label class="ca-label" for={`spec-${spec.key}`}>
							{spec.label || spec.key}
							{#if spec.included_qty > 0}
								<span class="ca-muted" style="font-weight:400;">
									({t('orderfe.product.specIncluded', {
										qty: String(spec.included_qty),
										unit: unitLabel(spec.unit)
									})})
								</span>
							{/if}
						</label>
						<span style="font-weight:600;color:var(--ca-primary);white-space:nowrap;">
							{c.unlimited ? t('orderfe.product.specUnlimited') : `${c.qty}${unitLabel(spec.unit)}`}
							{#if amount > 0}
								<span class="ca-muted" style="font-weight:400;">+{formatIDR(amount)}</span>
							{/if}
						</span>
					</div>

					{#if !c.unlimited}
						<input
							id={`spec-${spec.key}`}
							type="range"
							min={spec.min_qty}
							max={spec.max_qty > 0 ? spec.max_qty : spec.min_qty + spec.step_qty * 100}
							step={spec.step_qty}
							value={c.qty}
							oninput={(e) => setQty(spec, Number(e.currentTarget.value))}
							style="width:100%;"
							data-testid={`spec-${spec.key}-input`}
						/>
					{/if}

					{#if spec.allow_unlimited}
						<label class="ca-check" style="margin-top:6px;">
							<input
								type="checkbox"
								checked={c.unlimited}
								onchange={(e) => setUnlimited(spec, e.currentTarget.checked)}
								data-testid={`spec-${spec.key}-unlimited`}
							/>
							<span>{t('orderfe.product.specUnlimited')}</span>
						</label>
					{/if}
				</div>
			{/each}
		</div>
	</CaPanel>
{/if}
