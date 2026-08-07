import { apiFetch } from '$lib/server/api';
import { error as kitError, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { CategoryRow } from '../../+page.server';
import type { ArticleRow } from '../+page.server';

export const load: PageServerLoad = async (event) => {
	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) kitError(404, 'Not found');

	const [articleRes, categoriesRes] = await Promise.all([
		apiFetch<ArticleRow>(event, `/api/v1/admin/kb/articles/${id}`),
		apiFetch<CategoryRow[]>(event, '/api/v1/admin/kb/categories')
	]);

	return {
		articleId: id,
		article: articleRes.data,
		loadError: articleRes.error ? articleRes.error.message : null,
		notFound: articleRes.status === 404,
		categories: categoriesRes.data ?? [],
		created: event.url.searchParams.get('created') === '1'
	};
};

export const actions: Actions = {
	save: async (event) => {
		const id = Number(event.params.id);
		const form = await event.request.formData();
		const categoryId = Number(form.get('category_id')) || 0;
		const title = String(form.get('title') ?? '').trim();
		const slug = String(form.get('slug') ?? '')
			.trim()
			.toLowerCase();
		if (!id || !categoryId || !title) {
			return fail(400, { op: 'save', errorKey: 'adminPortal.kb.articleFillRequired' });
		}

		const body: Record<string, unknown> = {
			category_id: categoryId,
			title,
			body: String(form.get('body') ?? ''),
			published: form.get('published') !== null,
			sort: Number(form.get('sort') ?? 0) || 0
		};
		if (slug) body.slug = slug;

		const res = await apiFetch(event, `/api/v1/admin/kb/articles/${id}`, { method: 'PATCH', body });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'save',
				errorMessage: res.error.message
			});
		}
		return { op: 'save', success: true };
	},

	delete: async (event) => {
		const id = Number(event.params.id);
		const res = await apiFetch(event, `/api/v1/admin/kb/articles/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		redirect(303, '/admin/knowledgebase/articles?deleted=1');
	}
};
