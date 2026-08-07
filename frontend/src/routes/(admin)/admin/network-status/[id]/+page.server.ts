import { apiFetch } from '$lib/server/api';
import { error as kitError, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { NetworkIssueRow } from '../+page.server';

/** "YYYY-MM-DDTHH:MM" (datetime-local) -> RFC3339 UTC, or null when blank. */
function toRFC3339(raw: string): string | null {
	const v = raw.trim();
	return v ? `${v}:00Z` : null;
}

export const load: PageServerLoad = async (event) => {
	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) kitError(404, 'Not found');

	const res = await apiFetch<NetworkIssueRow>(event, `/api/v1/admin/network-issues/${id}`);

	return {
		issueId: id,
		issue: res.data,
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
		if (!id || !title)
			return fail(400, { op: 'save', errorKey: 'adminPortal.network.fillRequired' });

		const ends = toRFC3339(String(form.get('ends_at') ?? ''));
		const body: Record<string, unknown> = {
			title,
			body: String(form.get('body') ?? ''),
			type: String(form.get('type') ?? 'issue'),
			severity: String(form.get('severity') ?? 'minor'),
			status: String(form.get('status') ?? 'investigating'),
			affected: String(form.get('affected') ?? ''),
			starts_at: toRFC3339(String(form.get('starts_at') ?? '')),
			// ClearEnds wins over EndsAt server-side; set it when the field is blanked.
			ends_at: ends,
			clear_ends: ends === null
		};

		const res = await apiFetch(event, `/api/v1/admin/network-issues/${id}`, {
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
		const res = await apiFetch(event, `/api/v1/admin/network-issues/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		redirect(303, '/admin/network-status?deleted=1');
	}
};
