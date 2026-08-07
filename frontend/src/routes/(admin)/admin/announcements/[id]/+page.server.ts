import { apiFetch } from '$lib/server/api';
import { error as kitError, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { AnnouncementRow } from '../+page.server';

export const load: PageServerLoad = async (event) => {
	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) kitError(404, 'Not found');

	const res = await apiFetch<AnnouncementRow>(event, `/api/v1/admin/announcements/${id}`);

	return {
		announcementId: id,
		announcement: res.data,
		loadError: res.error ? res.error.message : null,
		notFound: res.status === 404,
		created: event.url.searchParams.get('created') === '1'
	};
};

export const actions: Actions = {
	save: async (event) => {
		const id = Number(event.params.id);
		const form = await event.request.formData();
		const title = String(form.get('title') ?? '').trim();
		const slug = String(form.get('slug') ?? '')
			.trim()
			.toLowerCase();
		if (!id || !title)
			return fail(400, { op: 'save', errorKey: 'adminPortal.announcements.fillRequired' });

		const body: Record<string, unknown> = {
			title,
			body: String(form.get('body') ?? ''),
			published: form.get('published') !== null
		};
		if (slug) body.slug = slug;

		const res = await apiFetch(event, `/api/v1/admin/announcements/${id}`, {
			method: 'PATCH',
			body
		});
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
		const res = await apiFetch(event, `/api/v1/admin/announcements/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		redirect(303, '/admin/announcements?deleted=1');
	}
};
