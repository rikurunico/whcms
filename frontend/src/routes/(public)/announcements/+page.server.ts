import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

export interface Announcement {
	id: number;
	title: string;
	slug: string;
	body: string;
	published: boolean;
	published_at: string | null;
	author_id: number | null;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const page = Math.max(1, Number(event.url.searchParams.get('page') ?? '1') || 1);
	const perPage = Math.min(
		1000,
		Math.max(1, Number(event.url.searchParams.get('per_page') ?? '10') || 10)
	);

	const res = await apiFetch<Announcement[]>(event, '/api/v1/announcements', {
		query: { page, per_page: perPage }
	});

	return {
		announcements: res.data ?? [],
		meta: res.meta,
		page,
		perPage,
		loadError: res.error?.message ?? null
	};
};
