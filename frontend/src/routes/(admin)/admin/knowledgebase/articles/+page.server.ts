import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { CategoryRow } from '../+page.server';

/** domain.KBArticle JSON tags. */
export interface ArticleRow {
	id: number;
	category_id: number;
	title: string;
	slug: string;
	body: string;
	published: boolean;
	views: number;
	sort: number;
	author_id: number | null;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const search = (q.get('search') ?? '').trim();
	const status = q.get('status') ?? '';
	const categoryId = Number(q.get('category_id')) || 0;

	const [articlesRes, categoriesRes] = await Promise.all([
		apiFetch<ArticleRow[]>(event, '/api/v1/admin/kb/articles', {
			query: {
				page,
				per_page: perPage,
				search: search || undefined,
				status: status || undefined,
				category_id: categoryId > 0 ? categoryId : undefined
			}
		}),
		apiFetch<CategoryRow[]>(event, '/api/v1/admin/kb/categories')
	]);

	return {
		articles: articlesRes.data ?? [],
		meta: articlesRes.meta,
		listError: articlesRes.error ? articlesRes.error.message : null,
		categories: categoriesRes.data ?? [],
		page,
		perPage,
		search,
		status,
		categoryId
	};
};

export const actions: Actions = {
	delete: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { op: 'delete', errorKey: 'adminPortal.kb.articleDeleteFailed' });
		const res = await apiFetch(event, `/api/v1/admin/kb/articles/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'delete', success: true };
	}
};
