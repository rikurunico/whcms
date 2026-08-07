import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import type { KBArticle } from '../../+page.server';

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<KBArticle>(
		event,
		`/api/v1/kb/articles/${encodeURIComponent(event.params.slug)}`
	);

	if (res.error || !res.data) {
		const notFound = res.status === 404 || res.error?.code === 'NOT_FOUND' || !res.data;
		return {
			article: null,
			notFound,
			loadError: notFound ? null : (res.error?.message ?? null)
		};
	}

	return { article: res.data, notFound: false, loadError: null };
};
