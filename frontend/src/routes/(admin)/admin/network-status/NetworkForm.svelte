<script lang="ts">
	import { enhance } from '$app/forms';
	import { untrack } from 'svelte';
	import type { NetworkIssueRow } from './+page.server';
	import { SEVERITIES, STATUSES, TYPES, severityLabels, statusLabels, typeLabels } from './meta';

	interface Props {
		/** null -> create mode. */
		issue?: NetworkIssueRow | null;
		/** Form action, e.g. '?/save'. Omit for the default action. */
		action?: string;
		/** Error text from the failed action (already resolved). */
		errorText?: string | null;
		submitLabel: string;
		onSuccess?: () => void;
	}

	let { issue = null, action, errorText = null, submitLabel, onSuccess }: Props = $props();

	/** ISO string -> the value a <input type="datetime-local"> expects (UTC wall-clock). */
	function toLocalInput(iso: string | null | undefined): string {
		if (!iso) return '';
		return iso.slice(0, 16);
	}

	function nowInput(): string {
		return new Date().toISOString().slice(0, 16);
	}

	let title = $state(untrack(() => issue?.title ?? ''));
	let body = $state(untrack(() => issue?.body ?? ''));
	let type = $state(untrack(() => issue?.type ?? 'issue'));
	let severity = $state(untrack(() => issue?.severity ?? 'minor'));
	let status = $state(untrack(() => issue?.status ?? 'investigating'));
	let affected = $state(untrack(() => issue?.affected ?? ''));
	let startsAt = $state(untrack(() => (issue ? toLocalInput(issue.starts_at) : nowInput())));
	let endsAt = $state(untrack(() => toLocalInput(issue?.ends_at)));

	let submitting = $state(false);
</script>

<form
	method="POST"
	{action}
	data-testid="network-form"
	use:enhance={() => {
		submitting = true;
		return async ({ result, update }) => {
			submitting = false;
			if (result.type === 'success' || result.type === 'redirect') onSuccess?.();
			await update();
		};
	}}
>
	{#if errorText}
		<div class="hp-alert-red" data-testid="network-form-error">
			<i class="fas fa-exclamation-triangle" style="margin-right:8px"></i>{errorText}
		</div>
	{/if}

	<div class="hp-formrow" data-testid="network-title-field">
		<label for="field-title">Title<span style="color:#d9534f"> *</span></label>
		<div class="hp-field">
			<input id="field-title" name="title" bind:value={title} required class="hp-input" />
		</div>
	</div>

	<div class="hp-formrow top" data-testid="network-body-field">
		<label for="field-body">Body</label>
		<div class="hp-field" style="flex:1 1 520px">
			<textarea id="field-body" name="body" rows="8" bind:value={body} class="hp-textarea"
			></textarea>
		</div>
	</div>

	<div class="hp-formrow" data-testid="network-type-field">
		<label for="field-type">Type</label>
		<div class="hp-field">
			<select id="field-type" name="type" bind:value={type} class="hp-select">
				{#each TYPES as tp (tp)}
					<option value={tp}>{typeLabels[tp]}</option>
				{/each}
			</select>
		</div>
	</div>

	<div class="hp-formrow" data-testid="network-severity-field">
		<label for="field-severity">Severity</label>
		<div class="hp-field">
			<select id="field-severity" name="severity" bind:value={severity} class="hp-select">
				{#each SEVERITIES as sv (sv)}
					<option value={sv}>{severityLabels[sv]}</option>
				{/each}
			</select>
		</div>
	</div>

	<div class="hp-formrow" data-testid="network-status-field">
		<label for="field-status">Status</label>
		<div class="hp-field">
			<select id="field-status" name="status" bind:value={status} class="hp-select">
				{#each STATUSES as st (st)}
					<option value={st}>{statusLabels[st]}</option>
				{/each}
			</select>
		</div>
	</div>

	<div class="hp-formrow" data-testid="network-affected-field">
		<label for="field-affected">Affected</label>
		<div class="hp-field">
			<input id="field-affected" name="affected" bind:value={affected} class="hp-input" />
			<div class="hp-help" style="margin-top:3px">
				Affected servers/services, e.g. "Server WEB-03, Jakarta DC".
			</div>
		</div>
	</div>

	<div class="hp-formrow" data-testid="network-starts-field">
		<label for="field-starts_at">Starts At</label>
		<div class="hp-field">
			<input
				id="field-starts_at"
				name="starts_at"
				type="datetime-local"
				bind:value={startsAt}
				class="hp-input"
			/>
		</div>
	</div>

	<div class="hp-formrow" data-testid="network-ends-field">
		<label for="field-ends_at">Ends At</label>
		<div class="hp-field">
			<input
				id="field-ends_at"
				name="ends_at"
				type="datetime-local"
				bind:value={endsAt}
				class="hp-input"
			/>
			<div class="hp-help" style="margin-top:3px">
				Leave blank if it is ongoing / not yet resolved.
			</div>
		</div>
	</div>

	<div class="hp-form-actions">
		<a href="/admin/network-status" class="hp-btn">Cancel</a>
		<span data-testid="network-form-submit">
			<button type="submit" class="hp-btn hp-btn-primary" disabled={submitting}>
				{submitLabel}
			</button>
		</span>
	</div>
</form>
