import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import type { NetworkIssueRow } from '../+page.server';

/** "YYYY-MM-DDTHH:MM" (datetime-local) -> RFC3339 UTC, or null when blank. */
function toRFC3339(raw: string): string | null {
	const v = raw.trim();
	return v ? `${v}:00Z` : null;
}

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const title = String(form.get('title') ?? '').trim();
		if (!title) return fail(400, { errorKey: 'adminPortal.network.fillRequired' });

		const body = {
			title,
			body: String(form.get('body') ?? ''),
			type: String(form.get('type') ?? 'issue'),
			severity: String(form.get('severity') ?? 'minor'),
			status: String(form.get('status') ?? 'investigating'),
			affected: String(form.get('affected') ?? ''),
			starts_at: toRFC3339(String(form.get('starts_at') ?? '')),
			ends_at: toRFC3339(String(form.get('ends_at') ?? ''))
		};

		const res = await apiFetch<NetworkIssueRow>(event, '/api/v1/admin/network-issues', {
			method: 'POST',
			body
		});
		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 400, {
				errorMessage: res.error?.message,
				errorKey: res.error ? undefined : 'adminPortal.network.saveFailed'
			});
		}
		redirect(303, `/admin/network-status/${res.data.id}?created=1`);
	}
};
