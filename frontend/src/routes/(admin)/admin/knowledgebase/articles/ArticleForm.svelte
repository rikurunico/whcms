<script lang="ts">
	import { enhance } from '$app/forms';
	import { slugify } from '$lib/slug';
	import { untrack } from 'svelte';
	import type { CategoryRow } from '../+page.server';
	import type { ArticleRow } from './+page.server';

	interface Props {
		/** null -> create mode. */
		article?: ArticleRow | null;
		categories: CategoryRow[];
		/** Form action, e.g. '?/save'. Omit for the default action. */
		action?: string;
		/** Error text from the failed action (already resolved). */
		errorText?: string | null;
		submitLabel: string;
		onSuccess?: () => void;
	}

	let {
		article = null,
		categories,
		action,
		errorText = null,
		submitLabel,
		onSuccess
	}: Props = $props();

	let categoryId = $state(untrack(() => (article ? String(article.category_id) : '')));
	let title = $state(untrack(() => article?.title ?? ''));
	let slug = $state(untrack(() => article?.slug ?? ''));
	let body = $state(untrack(() => article?.body ?? ''));
	let published = $state(untrack(() => article?.published ?? false));
	let sort = $state(untrack(() => article?.sort ?? 0));

	let slugTouched = $state(untrack(() => article !== null));
	$effect(() => {
		if (!slugTouched) slug = slugify(title);
	});

	let submitting = $state(false);
</script>

<form
	method="POST"
	{action}
	data-testid="kb-article-form"
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
		<div class="hp-alert-red" data-testid="kb-article-form-error">
			<i class="fas fa-exclamation-triangle" style="margin-right:8px"></i>{errorText}
		</div>
	{/if}

	<div class="hp-formrow" data-testid="kb-article-category-field">
		<label for="field-category_id">Category<span style="color:#d9534f"> *</span></label>
		<div class="hp-field">
			<select
				id="field-category_id"
				name="category_id"
				bind:value={categoryId}
				required
				class="hp-select"
			>
				<option value="" disabled>Select a category…</option>
				{#each categories as c (c.id)}
					<option value={String(c.id)}>{c.name}</option>
				{/each}
			</select>
		</div>
	</div>

	<div class="hp-formrow" data-testid="kb-article-title-field">
		<label for="field-title">Title<span style="color:#d9534f"> *</span></label>
		<div class="hp-field">
			<input id="field-title" name="title" bind:value={title} required class="hp-input" />
		</div>
	</div>

	<div class="hp-formrow" data-testid="kb-article-slug-field">
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

	<div class="hp-formrow top" data-testid="kb-article-body-field">
		<label for="field-body">Body</label>
		<div class="hp-field" style="flex:1 1 520px">
			<textarea id="field-body" name="body" rows="12" bind:value={body} class="hp-textarea"
			></textarea>
		</div>
	</div>

	<div class="hp-formrow" data-testid="kb-article-sort-field">
		<label for="field-sort">Sort Order</label>
		<div class="hp-field">
			<input id="field-sort" name="sort" type="number" bind:value={sort} class="hp-input" />
		</div>
	</div>

	<div class="hp-formrow" data-testid="kb-article-published-field">
		<label for="field-published">&nbsp;</label>
		<div class="hp-field" style="flex:1 1 auto">
			<label class="hp-checkline" style="padding:0">
				<input id="field-published" name="published" type="checkbox" bind:checked={published} />
				Published (visible to clients)
			</label>
		</div>
	</div>

	<div class="hp-form-actions">
		<a href="/admin/knowledgebase/articles" class="hp-btn">Cancel</a>
		<span data-testid="kb-article-form-submit">
			<button type="submit" class="hp-btn hp-btn-primary" disabled={submitting}>
				{submitLabel}
			</button>
		</span>
	</div>
</form>
