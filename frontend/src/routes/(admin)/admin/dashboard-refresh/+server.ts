import { apiFetch } from '$lib/server/api';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/**
 * Hard-refresh proxy for the admin dashboard's "Refresh" panel buttons.
 *
 * GET /api/v1/admin/dashboard is cached server-side for 60s (Redis), so a
 * plain page reload (`invalidateAll()`) can still serve stale data within
 * that window. This endpoint calls the same API with `?refresh=1`, which
 * bypasses the cache read and recomputes from the database - the fresh
 * result also repopulates the cache, so the page's own subsequent load (via
 * `invalidateAll()` right after this call) gets the fresh data too, without
 * a second cache-bypass round trip or any `?refresh=1` left sitting in the
 * browser's address bar.
 */
export const POST: RequestHandler = async (event) => {
	const res = await apiFetch(event, '/api/v1/admin/dashboard', { query: { refresh: '1' } });
	if (res.error) {
		return json({ ok: false, error: res.error.message }, { status: res.status || 502 });
	}
	return json({ ok: true });
};
