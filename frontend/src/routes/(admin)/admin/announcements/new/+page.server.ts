import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import type { AnnouncementRow } from '../+page.server';

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const title = String(form.get('title') ?? '').trim();
		const slug = String(form.get('slug') ?? '')
			.trim()
			.toLowerCase();
		if (!title) return fail(400, { errorKey: 'adminPortal.announcements.fillRequired' });

		const body: Record<string, unknown> = {
			title,
			body: String(form.get('body') ?? ''),
			published: form.get('published') !== null
		};
		if (slug) body.slug = slug;

		const res = await apiFetch<AnnouncementRow>(event, '/api/v1/admin/announcements', {
			method: 'POST',
			body
		});
		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 400, {
				errorMessage: res.error?.message,
				errorKey: res.error ? undefined : 'adminPortal.announcements.saveFailed'
			});
		}
		redirect(303, `/admin/announcements/${res.data.id}?created=1`);
	}
};
