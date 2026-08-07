import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

export interface KBCategory {
	id: number;
	name: string;
	slug: string;
	description: string;
	sort: number;
	hidden: boolean;
}

export interface KBArticle {
	id: number;
	category_id: number;
	title: string;
	slug: string;
	body: string;
	published: boolean;
	views: number;
	sort: number;
}

export const load: PageServerLoad = async (event) => {
	// The header KB search submits GET with name="search"; also accept ?q= .
	const term = (
		event.url.searchParams.get('search') ??
		event.url.searchParams.get('q') ??
		''
	).trim();

	// Categories and search results are independent - fetch them concurrently.
	const catsPromise = apiFetch<KBCategory[]>(event, '/api/v1/kb/categories');
	const artPromise = term
		? apiFetch<KBArticle[]>(event, '/api/v1/kb/articles', {
				query: { search: term, per_page: 50 }
			})
		: null;
	const catsRes = await catsPromise;

	let results: KBArticle[] | null = null;
	let resultsError: string | null = null;
	if (artPromise) {
		const artRes = await artPromise;
		results = artRes.data ?? [];
		resultsError = artRes.error?.message ?? null;
	}

	return {
		categories: catsRes.data ?? [],
		results,
		term,
		loadError: catsRes.error?.message ?? null,
		resultsError
	};
};
