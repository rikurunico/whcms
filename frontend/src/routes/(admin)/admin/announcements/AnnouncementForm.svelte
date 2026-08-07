<script lang="ts">
	import { enhance } from '$app/forms';
	import { slugify } from '$lib/slug';
	import { untrack } from 'svelte';
	import type { AnnouncementRow } from './+page.server';

	interface Props {
		/** null -> create mode. */
		announcement?: AnnouncementRow | null;
		/** Form action, e.g. '?/save'. Omit for the default action. */
		action?: string;
		/** Error text from the failed action (already resolved). */
		errorText?: string | null;
		submitLabel: string;
		onSuccess?: () => void;
	}

	let { announcement = null, action, errorText = null, submitLabel, onSuccess }: Props = $props();

	let title = $state(untrack(() => announcement?.title ?? ''));
	let slug = $state(untrack(() => announcement?.slug ?? ''));
	let body = $state(untrack(() => announcement?.body ?? ''));
	let published = $state(untrack(() => announcement?.published ?? false));

	let slugTouched = $state(untrack(() => announcement !== null));
	$effect(() => {
		if (!slugTouched) slug = slugify(title);
	});

	let submitting = $state(false);
</script>

<form
	method="POST"
	{action}
	data-testid="announcement-form"
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
		<div class="hp-alert-red" data-testid="announcement-form-error">
			<i class="fas fa-exclamation-triangle" style="margin-right:8px"></i>{errorText}
		</div>
	{/if}

	<div class="hp-formrow" data-testid="announcement-title-field">
		<label for="field-title">Title<span style="color:#d9534f"> *</span></label>
		<div class="hp-field">
			<input id="field-title" name="title" bind:value={title} required class="hp-input" />
		</div>
	</div>

	<div class="hp-formrow" data-testid="announcement-slug-field">
		<label for="field-slug">Slug</label>
		<div class="hp-field">
			<input
				id="field-slug"
				name="slug"
				bind:value={slug}
				class="hp-input"
				oninput={() => (slugTouched = true)}
			/>
			<div class="hp-help" style="margin-top:3px">
				Lowercase letters, numbers and dashes. Leave blank to derive from the title.
			</div>
		</div>
	</div>

	<div class="hp-formrow top" data-testid="announcement-body-field">
		<label for="field-body">Body</label>
		<div class="hp-field" style="flex:1 1 520px">
			<textarea id="field-body" name="body" rows="10" bind:value={body} class="hp-textarea"
			></textarea>
		</div>
	</div>

	<div class="hp-formrow" data-testid="announcement-published-field">
		<label for="field-published">&nbsp;</label>
		<div class="hp-field" style="flex:1 1 auto">
			<label class="hp-checkline" style="padding:0">
				<input id="field-published" name="published" type="checkbox" bind:checked={published} />
				Published (visible to clients)
			</label>
		</div>
	</div>

	<div class="hp-form-actions">
		<a href="/admin/announcements" class="hp-btn">Cancel</a>
		<span data-testid="announcement-form-submit">
			<button type="submit" class="hp-btn hp-btn-primary" disabled={submitting}>
				{submitLabel}
			</button>
		</span>
	</div>
</form>
