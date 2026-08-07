import { apiFetch } from '$lib/server/api';
import { setSessionCookies } from '$lib/server/session';
import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

interface ImpersonateResponse {
	access_token: string;
	refresh_token: string;
}

/**
 * POST /admin/clients/:id/impersonate proxy (FRONTEND.md §2 FE-ADMIN-CORE): calls the
 * backend impersonation endpoint (docs/MODULES.md M-AUTH, RequireRole admin), swaps the
 * session cookies to the returned client tokens and redirects to the client dashboard.
 */
export const POST: RequestHandler = async (event) => {
	if (!event.locals.user || event.locals.user.role !== 'admin') {
		error(403, 'forbidden');
	}

	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) error(404, 'client not found');

	const res = await apiFetch<ImpersonateResponse>(
		event,
		`/api/v1/admin/clients/${id}/impersonate`,
		{ method: 'POST' }
	);

	if (res.error || !res.data?.access_token || !res.data?.refresh_token) {
		error(res.status >= 400 ? res.status : 502, res.error?.message ?? 'impersonation failed');
	}

	setSessionCookies(event.cookies, {
		access_token: res.data.access_token,
		refresh_token: res.data.refresh_token
	});

	redirect(303, '/dashboard');
};
