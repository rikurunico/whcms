import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import type { KBArticle, KBCategory } from '../../+page.server';

/** GET /kb/categories/:slug returns the category flattened + its articles. */
export interface CategoryWithArticles extends KBCategory {
	articles: KBArticle[];
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<CategoryWithArticles>(
		event,
		`/api/v1/kb/categories/${encodeURIComponent(event.params.slug)}`
	);

	if (res.error || !res.data) {
		const notFound = res.status === 404 || res.error?.code === 'NOT_FOUND' || !res.data;
		return {
			category: null,
			notFound,
			loadError: notFound ? null : (res.error?.message ?? null)
		};
	}

	return { category: res.data, notFound: false, loadError: null };
};
