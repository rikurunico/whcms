import { apiFetch } from '$lib/server/api';
import { clearSessionCookies, REFRESH_COOKIE } from '$lib/server/session';
import { redirect, type RequestEvent } from '@sveltejs/kit';

async function logout(event: RequestEvent): Promise<never> {
	const refreshToken = event.cookies.get(REFRESH_COOKIE);
	if (event.locals.accessToken || refreshToken) {
		// Best-effort revoke; local cookies are cleared regardless.
		await apiFetch(event, '/api/v1/auth/logout', {
			method: 'POST',
			body: refreshToken ? { refresh_token: refreshToken } : undefined
		});
	}
	clearSessionCookies(event.cookies);
	event.locals.user = null;
	event.locals.accessToken = null;
	redirect(303, '/login');
}

export const POST = logout;
