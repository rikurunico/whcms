import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import type { Announcement } from '../+page.server';

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<Announcement>(
		event,
		`/api/v1/announcements/${encodeURIComponent(event.params.slug)}`
	);

	if (res.error || !res.data) {
		const notFound = res.status === 404 || res.error?.code === 'NOT_FOUND' || !res.data;
		return {
			announcement: null,
			notFound,
			loadError: notFound ? null : (res.error?.message ?? null)
		};
	}

	return { announcement: res.data, notFound: false, loadError: null };
};
