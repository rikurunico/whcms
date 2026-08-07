<script lang="ts">
	import { enhance } from '$app/forms';
	import { toast } from '$lib/stores/toast.svelte';
	import type { SubmitFunction } from '@sveltejs/kit';
	import { untrack } from 'svelte';

	interface Registrar {
		id: number;
		name: string;
		active: boolean;
		config: Record<string, unknown> | null;
		reseller_id: string;
		api_key_present: boolean;
		base_url: string;
	}

	interface ActionData {
		action?: string;
		success?: boolean;
		testId?: number;
		message?: string;
		errorKey?: string;
		errorMessage?: string;
	}

	interface Props {
		registrar: Registrar;
		result?: ActionData | null;
	}

	let { registrar, result = null }: Props = $props();

	interface ConfigRow {
		key: string;
		value: string;
	}

	let rows = $state<ConfigRow[]>(
		untrack(() =>
			Object.entries(registrar.config ?? {}).map(([key, v]) => ({
				key,
				value: typeof v === 'string' ? v : JSON.stringify(v)
			}))
		)
	);
	let active = $state(untrack(() => registrar.active));
	let resellerId = $state(untrack(() => registrar.reseller_id));
	let baseUrl = $state(untrack(() => registrar.base_url));
	let apiKey = $state('');
	let clearApiKey = $state(false);
	let expanded = $state(false);
	let saving = $state(false);
	let testing = $state(false);

	function buildConfig(): Record<string, unknown> {
		const out: Record<string, unknown> = {};
		for (const row of rows) {
			const key = row.key.trim();
			if (!key) continue;
			const raw = row.value.trim();
			try {
				out[key] = JSON.parse(raw);
			} catch {
				out[key] = raw;
			}
		}
		return out;
	}

	const configJson = $derived(JSON.stringify(buildConfig()));

	const testResult = $derived(
		result && result.action === 'test' && result.testId === registrar.id
			? {
					ok: result.success === true,
					message:
						(result.success === true ? result.message : result.errorMessage) ||
						(result.success === true ? 'Connection successful' : 'Connection failed')
				}
			: null
	);

	function addRow() {
		rows = [...rows, { key: '', value: '' }];
	}
	function removeRow(i: number) {
		rows = rows.filter((_, idx) => idx !== i);
	}

	function handler(successMsg: string, setLoading: (v: boolean) => void): SubmitFunction {
		return () => {
			setLoading(true);
			return async ({ result: r, update }) => {
				setLoading(false);
				if (r.type === 'success') {
					toast.success(successMsg);
				} else if (r.type === 'failure') {
					const d = r.data as { errorKey?: string; errorMessage?: string } | undefined;
					toast.error(d?.errorMessage ?? 'Action failed');
				}
				// reset: false - SvelteKit's default update() calls the native
				// form.reset(), which snaps every bind:value input (reseller id,
				// api key, config rows, active) back to blank/unchecked before
				// the re-fetched data re-renders. That's what caused fields to
				// go blank right after a successful save until a hard reload.
				await update({ reset: false });
			};
		};
	}
</script>

<div class="reg-row" class:on={registrar.active} data-testid={`row-registrar-${registrar.id}`}>
	<div class="reg-head">
		<div style="display:flex;align-items:center;gap:14px;min-width:0">
			<i class="fas fa-globe" style="font-size:22px;color:#888;width:32px;text-align:center"></i>
			<div style="min-width:0">
				<span style="color:#333;font-weight:600">» {registrar.name}</span>
				<span
					class={`hp-badge ${registrar.active ? 'active' : 'inactive'}`}
					style="margin-left:8px"
				>
					{registrar.active ? 'Active' : 'Inactive'}
				</span>
				<span
					class="reg-key"
					data-testid={`registrar-key-presence-${registrar.id}`}
					style={`margin-left:8px;font-size:11.5px;color:${registrar.api_key_present === true ? '#43a047' : registrar.api_key_present === false ? '#d9534f' : '#aaa'}`}
				>
					{registrar.api_key_present === true
						? 'API key present'
						: registrar.api_key_present === false
							? 'API key missing'
							: 'API key unknown'}
				</span>
			</div>
		</div>
		<div style="display:flex;gap:8px;white-space:nowrap">
			<form
				method="POST"
				action="?/test"
				use:enhance={handler('Connection successful', (v) => (testing = v))}
				style="display:inline"
			>
				<input type="hidden" name="id" value={registrar.id} />
				<button
					type="submit"
					class="hp-btn"
					style="padding:6px 12px"
					disabled={testing}
					data-testid={`registrar-test-${registrar.id}`}
				>
					{testing ? 'Testing…' : 'Test'}
				</button>
			</form>
			<button
				type="button"
				class="hp-btn"
				style="padding:6px 12px"
				onclick={() => (expanded = !expanded)}
				data-testid={`registrar-configure-${registrar.id}`}
			>
				Configure
			</button>
		</div>
	</div>

	{#if testResult}
		<div data-testid={`registrar-test-result-${registrar.id}`} style="padding:0 16px 12px">
			<div class={testResult.ok ? 'hp-info' : 'hp-alert-red'} style="margin:0">
				{testResult.message}
			</div>
		</div>
	{/if}

	{#if expanded}
		<form
			method="POST"
			action="?/save"
			style="padding:4px 16px 16px"
			use:enhance={handler('Registrar saved', (v) => (saving = v))}
		>
			<input type="hidden" name="id" value={registrar.id} />
			<input type="hidden" name="config" value={configJson} />

			<label class="hp-checkline">
				<input
					type="checkbox"
					name="active"
					bind:checked={active}
					data-testid={`registrar-active-${registrar.id}`}
				/>
				Module Active
			</label>

			<div
				style="font-size:12px;font-weight:700;color:#666;text-transform:uppercase;margin:10px 0 3px"
			>
				Credentials
			</div>
			<div class="hp-help" style="margin-bottom:8px">
				Stored encrypted; environment variables are the fallback for whichever field is left blank.
			</div>
			<div class="hp-formrow" style="margin-bottom:8px">
				<label for={`registrar-reseller-id-${registrar.id}`}>Reseller ID</label>
				<input
					id={`registrar-reseller-id-${registrar.id}`}
					type="text"
					class="hp-input"
					name="reseller_id"
					bind:value={resellerId}
					placeholder="(env fallback)"
					data-testid={`registrar-reseller-id-${registrar.id}`}
				/>
			</div>
			<div class="hp-formrow" style="margin-bottom:4px">
				<label for={`registrar-api-key-${registrar.id}`}>API Key</label>
				<input
					id={`registrar-api-key-${registrar.id}`}
					type="password"
					class="hp-input"
					name="api_key"
					autocomplete="off"
					bind:value={apiKey}
					disabled={clearApiKey}
					placeholder={registrar.api_key_present
						? '•••••••• (leave blank to keep unchanged)'
						: 'Not set'}
					data-testid={`registrar-api-key-${registrar.id}`}
				/>
			</div>
			{#if registrar.api_key_present}
				<label class="hp-checkline" style="margin-bottom:8px">
					<input
						type="checkbox"
						name="clear_api_key"
						bind:checked={clearApiKey}
						data-testid={`registrar-clear-api-key-${registrar.id}`}
					/>
					Clear stored key
				</label>
			{/if}

			<div
				style="font-size:12px;font-weight:700;color:#666;text-transform:uppercase;margin:10px 0 3px"
			>
				Endpoint
			</div>
			<div class="hp-help" style="margin-bottom:8px">
				Custom endpoint — point this registrar at a different base URL (e.g. the local mock server)
				without restarting anything. Leave blank to use the environment default.
			</div>
			<div class="hp-formrow" style="margin-bottom:8px">
				<label for={`registrar-base-url-${registrar.id}`}>Custom Endpoint</label>
				<input
					id={`registrar-base-url-${registrar.id}`}
					type="text"
					class="hp-input"
					name="base_url"
					bind:value={baseUrl}
					placeholder="(env fallback, e.g. https://api.dewabiz.co.id/v1)"
					data-testid={`registrar-base-url-${registrar.id}`}
				/>
			</div>

			<div
				style="font-size:12px;font-weight:700;color:#666;text-transform:uppercase;margin:10px 0 3px"
			>
				Configuration
			</div>
			<div class="hp-help" style="margin-bottom:8px">
				Key/value pairs passed to the registrar module.
			</div>

			<div style="display:flex;flex-direction:column;gap:8px">
				{#each rows as row, i (i)}
					<div style="display:flex;align-items:center;gap:8px">
						<input
							type="text"
							class="hp-input"
							style="flex:0 0 33%;font-family:monospace;font-size:12px"
							placeholder="Key"
							bind:value={row.key}
							data-testid={`registrar-config-key-${registrar.id}-${i}`}
						/>
						<input
							type="text"
							class="hp-input"
							style="flex:1;font-family:monospace;font-size:12px"
							placeholder="Value"
							bind:value={row.value}
							data-testid={`registrar-config-value-${registrar.id}-${i}`}
						/>
						<button
							type="button"
							class="hp-btn"
							style="padding:6px 10px"
							aria-label="Remove row"
							title="Remove row"
							onclick={() => removeRow(i)}>&times;</button
						>
					</div>
				{/each}
			</div>

			<div style="display:flex;align-items:center;justify-content:space-between;margin-top:12px">
				<button
					type="button"
					class="cell-link"
					style="background:none;border:none;cursor:pointer;font:inherit"
					onclick={addRow}
					data-testid={`registrar-add-config-row-${registrar.id}`}
				>
					+ Add Row
				</button>
				<button
					type="submit"
					class="hp-btn hp-btn-primary"
					disabled={saving}
					data-testid={`registrar-save-${registrar.id}`}
				>
					{saving ? 'Saving…' : 'Save Changes'}
				</button>
			</div>
		</form>
	{/if}
</div>

<style>
	.reg-row {
		border: 1px solid #ddd;
		border-radius: 4px;
		background: #fff;
		margin-bottom: 8px;
	}
	.reg-row.on {
		background: #eefbe9;
	}
	.reg-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 14px;
		padding: 12px 16px;
	}
	@media (max-width: 640px) {
		.reg-head {
			flex-direction: column;
			align-items: flex-start;
		}
	}
</style>
