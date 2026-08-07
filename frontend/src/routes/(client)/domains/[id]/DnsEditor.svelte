<script lang="ts">
	import { enhance } from '$app/forms';
	import Alert from '$lib/components/Alert.svelte';
	import LoadingButton from '$lib/components/LoadingButton.svelte';
	import { t } from '$lib/i18n';
	import { toast } from '$lib/stores/toast.svelte';
	import type { SubmitFunction } from '@sveltejs/kit';
	import { untrack } from 'svelte';
	import { DNS_TYPES, type DnsRecord } from '../shared';

	interface Props {
		records: DnsRecord[];
		errorText?: string | null;
	}

	let { records, errorText = null }: Props = $props();

	function normalize(rows: DnsRecord[]): DnsRecord[] {
		return rows.map((r) => ({
			type: r.type,
			host: r.host,
			value: r.value,
			ttl: Number(r.ttl) || 0,
			prio: Number(r.prio) || 0
		}));
	}

	// The parent remounts this editor via {#key} whenever fresh DNS data loads,
	// so capturing the initial `records` value here is intentional.
	let rows = $state<DnsRecord[]>(untrack(() => records.map((r) => ({ ...r }))));
	let submitting = $state(false);

	const initial = untrack(() => JSON.stringify(normalize(records)));
	const serialized = $derived(JSON.stringify(normalize(rows)));
	const dirty = $derived(serialized !== initial);

	function addRow() {
		rows.push({ type: 'A', host: '', value: '', ttl: 3600, prio: 0 });
	}

	function removeRow(index: number) {
		rows = rows.filter((_, i) => i !== index);
	}

	const submitDns: SubmitFunction = () => {
		submitting = true;
		return async ({ result, update }) => {
			submitting = false;
			if (result.type === 'success') {
				toast.success(t('clientDomains.dns.saved'));
			}
			await update();
		};
	};

	const inputCls = 'ca-input';
</script>

<div class="ca-card">
	<h2 class="ca-card-header">{t('clientDomains.dns.title')}</h2>
	<div class="ca-card-body">
		<p class="ca-muted mb-4">{t('clientDomains.dns.desc')}</p>

		{#if errorText}
			<div class="mb-4" data-testid="dns-error">
				<Alert type="error">{errorText}</Alert>
			</div>
		{/if}

		<div class="overflow-x-auto rounded-md border border-gray-200">
			<table class="min-w-full divide-y divide-gray-200 text-sm">
				<thead class="bg-gray-50">
					<tr>
						<th
							class="px-3 py-2 text-left text-xs font-semibold tracking-wide text-gray-600 uppercase"
						>
							{t('clientDomains.dns.type')}
						</th>
						<th
							class="px-3 py-2 text-left text-xs font-semibold tracking-wide text-gray-600 uppercase"
						>
							{t('clientDomains.dns.host')}
						</th>
						<th
							class="px-3 py-2 text-left text-xs font-semibold tracking-wide text-gray-600 uppercase"
						>
							{t('clientDomains.dns.value')}
						</th>
						<th
							class="px-3 py-2 text-left text-xs font-semibold tracking-wide text-gray-600 uppercase"
						>
							{t('clientDomains.dns.ttl')}
						</th>
						<th
							class="px-3 py-2 text-left text-xs font-semibold tracking-wide text-gray-600 uppercase"
						>
							{t('clientDomains.dns.prio')}
						</th>
						<th class="px-3 py-2"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100">
					{#if rows.length === 0}
						<tr>
							<td colspan="6" class="px-3 py-8 text-center text-sm text-gray-500">
								{t('clientDomains.dns.empty')}
							</td>
						</tr>
					{:else}
						{#each rows as row, i (i)}
							<tr data-testid={`dns-row-${i}`}>
								<td class="px-3 py-2">
									<select
										class="ca-select"
										style="min-width:5rem"
										bind:value={row.type}
										aria-label={t('clientDomains.dns.type')}
										data-testid={`dns-type-${i}`}
									>
										{#each DNS_TYPES as dnsType (dnsType)}
											<option value={dnsType}>{dnsType}</option>
										{/each}
									</select>
								</td>
								<td class="px-3 py-2">
									<input
										class={inputCls}
										style="min-width:9rem"
										bind:value={row.host}
										placeholder="@"
										required
										aria-label={t('clientDomains.dns.host')}
										data-testid={`dns-host-${i}`}
									/>
								</td>
								<td class="px-3 py-2">
									<input
										class={inputCls}
										style="min-width:12rem"
										bind:value={row.value}
										required
										aria-label={t('clientDomains.dns.value')}
										data-testid={`dns-value-${i}`}
									/>
								</td>
								<td class="px-3 py-2">
									<input
										type="number"
										class={inputCls}
										style="width:6rem"
										bind:value={row.ttl}
										min="0"
										max="604800"
										aria-label={t('clientDomains.dns.ttl')}
										data-testid={`dns-ttl-${i}`}
									/>
								</td>
								<td class="px-3 py-2">
									<input
										type="number"
										class={inputCls}
										style="width:5rem"
										bind:value={row.prio}
										min="0"
										max="65535"
										disabled={row.type !== 'MX'}
										aria-label={t('clientDomains.dns.prio')}
										data-testid={`dns-prio-${i}`}
									/>
								</td>
								<td class="px-3 py-2 text-right">
									<button
										type="button"
										class="text-sm font-medium text-danger hover:underline"
										onclick={() => removeRow(i)}
										data-testid={`dns-remove-${i}`}
									>
										{t('clientDomains.dns.remove')}
									</button>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<form method="POST" action="?/dns" use:enhance={submitDns} class="mt-4">
			<input type="hidden" name="records" value={serialized} />
			<div class="flex flex-wrap items-center justify-between gap-3">
				<button
					type="button"
					class="ca-btn ca-btn-default"
					onclick={addRow}
					data-testid="dns-add-row"
				>
					+ {t('clientDomains.dns.add')}
				</button>
				<div class="flex items-center gap-3">
					{#if dirty}
						<span class="text-xs text-warning" data-testid="dns-unsaved-hint">
							{t('clientDomains.dns.unsaved')}
						</span>
					{/if}
					<span data-testid="dns-save">
						<LoadingButton type="submit" loading={submitting}>
							{t('clientDomains.dns.save')}
						</LoadingButton>
					</span>
				</div>
			</div>
		</form>
	</div>
</div>
