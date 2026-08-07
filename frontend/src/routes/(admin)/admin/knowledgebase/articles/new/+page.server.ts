import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { CategoryRow } from '../../+page.server';
import type { ArticleRow } from '../+page.server';

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<CategoryRow[]>(event, '/api/v1/admin/kb/categories');
	return {
		categories: res.data ?? [],
		categoriesError: res.error ? res.error.message : null
	};
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const categoryId = Number(form.get('category_id')) || 0;
		const title = String(form.get('title') ?? '').trim();
		const slug = String(form.get('slug') ?? '')
			.trim()
			.toLowerCase();
		if (!categoryId || !title) return fail(400, { errorKey: 'adminPortal.kb.articleFillRequired' });

		const body: Record<string, unknown> = {
			category_id: categoryId,
			title,
			body: String(form.get('body') ?? ''),
			published: form.get('published') !== null,
			sort: Number(form.get('sort') ?? 0) || 0
		};
		if (slug) body.slug = slug;

		const res = await apiFetch<ArticleRow>(event, '/api/v1/admin/kb/articles', {
			method: 'POST',
			body
		});
		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 400, {
				errorMessage: res.error?.message,
				errorKey: res.error ? undefined : 'adminPortal.kb.articleSaveFailed'
			});
		}
		redirect(303, `/admin/knowledgebase/articles/${res.data.id}?created=1`);
	}
};
