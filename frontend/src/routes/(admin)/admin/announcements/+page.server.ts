import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** announcements.AnnouncementResponse JSON tags. */
export interface AnnouncementRow {
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
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const search = (q.get('search') ?? '').trim();
	const status = q.get('status') ?? '';

	const res = await apiFetch<AnnouncementRow[]>(event, '/api/v1/admin/announcements', {
		query: {
			page,
			per_page: perPage,
			search: search || undefined,
			status: status || undefined
		}
	});

	return {
		announcements: res.data ?? [],
		meta: res.meta,
		listError: res.error ? res.error.message : null,
		page,
		perPage,
		search,
		status
	};
};

export const actions: Actions = {
	delete: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { op: 'delete', errorKey: 'adminPortal.announcements.deleteFailed' });
		const res = await apiFetch(event, `/api/v1/admin/announcements/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'delete', success: true };
	}
};
