import { apiFetch } from '$lib/server/api';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import type { InstallStatus } from '../+page.server';

/**
 * Install status polling proxy (browser -> SvelteKit -> GET /api/v1/install/status).
 * The wizard polls this every ~1.5s while waiting for the bootstrap phase's
 * self-restart to bring the API back up with a complete config.
 */
export const GET: RequestHandler = async (event) => {
	const res = await apiFetch<InstallStatus>(event, '/api/v1/install/status');
	if (res.error) {
		return json({ config_ready: false, installed: false }, { status: 200 });
	}
	return json(res.data);
};
