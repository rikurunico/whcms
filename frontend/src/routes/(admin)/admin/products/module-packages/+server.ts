import { apiFetch } from '$lib/server/api';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/** Result of GET /admin/server-groups/:id/packages - provisioning.PackageListResult. */
interface PackageListResult {
	ok: boolean;
	message?: string;
	packages?: string[];
}

/**
 * BFF proxy for the Module tab's "Load from Server" button - previews the
 * packages/plans that already exist on a representative server in the chosen
 * group, so the admin can pick an existing name instead of free-typing one
 * (and risking a typo that doesn't match anything on the panel).
 *
 * Normalizes both failure shapes the backend can return into one consistent
 * `{ok:false, message}` for the browser: a hard error (bad/empty group,
 * `res.error`) and a soft probe failure (`res.data.ok === false`, e.g. bad
 * credentials or an unreachable host).
 */
export const GET: RequestHandler = async (event) => {
	const groupId = event.url.searchParams.get('server_group_id');
	if (!groupId) {
		return json({ ok: false, message: 'Select a server group first.' } satisfies PackageListResult);
	}
	const res = await apiFetch<PackageListResult>(
		event,
		`/api/v1/admin/server-groups/${groupId}/packages`
	);
	if (res.error) {
		return json({ ok: false, message: res.error.message } satisfies PackageListResult);
	}
	return json(res.data);
};
