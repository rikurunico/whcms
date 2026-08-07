import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** Admin server row - domain.Server JSON tags (+ optional joined account count). */
export interface AdminServerRow {
	id: number;
	group_id: number | null;
	name: string;
	module: string;
	hostname: string;
	port: number;
	username: string;
	use_ssl: boolean;
	nameserver1: string;
	nameserver2: string;
	nameserver3: string;
	nameserver4: string;
	max_accounts: number;
	active: boolean;
	created_at: string;
	/** Provisioned accounts on this server (joined count, if the backend provides it). */
	accounts_count?: number;
}

/** domain.ServerGroup JSON tags. */
export interface AdminServerGroup {
	id: number;
	name: string;
	strategy: string;
	created_at: string;
}

/** Result of POST /admin/servers/:id/test-connection - provisioning.TestConnectionResult. */
interface TestConnectionResult {
	ok?: boolean;
	message?: string;
}

/** Rows per page for both the servers table (server-side) and the groups table (client-side). */
const PER_PAGE = 10;

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || PER_PAGE));
	// Server groups are fetched in full (needed for the group-name mapping and
	// the server-form dropdowns); the table paginates them client-side.
	const groupPage = Math.max(1, Number(q.get('gpage')) || 1);
	const groupPerPage = Math.min(1000, Math.max(1, Number(q.get('gper_page')) || PER_PAGE));

	const [serversRes, groupsRes] = await Promise.all([
		apiFetch<AdminServerRow[]>(event, '/api/v1/admin/servers', {
			query: { page, per_page: perPage }
		}),
		apiFetch<AdminServerGroup[]>(event, '/api/v1/admin/server-groups')
	]);

	return {
		servers: serversRes.data ?? [],
		meta: serversRes.meta,
		listError: serversRes.error ? serversRes.error.message : null,
		groups: groupsRes.data ?? [],
		groupsError: groupsRes.error ? groupsRes.error.message : null,
		page,
		perPage,
		groupPage,
		groupPerPage
	};
};

export const actions: Actions = {
	test: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { action: 'test', errorMessage: 'invalid server id' });

		const res = await apiFetch<TestConnectionResult>(
			event,
			`/api/v1/admin/servers/${id}/test-connection`,
			{ method: 'POST' }
		);
		if (res.error || res.data?.ok === false) {
			return fail(res.status >= 400 ? res.status : 502, {
				action: 'test',
				testId: id,
				errorMessage: res.error?.message ?? res.data?.message ?? 'connection failed'
			});
		}
		return { success: true, action: 'test', testId: id, message: res.data?.message ?? '' };
	},

	deleteServer: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { action: 'deleteServer', errorMessage: 'invalid server id' });

		const res = await apiFetch<unknown>(event, `/api/v1/admin/servers/${id}`, {
			method: 'DELETE'
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				action: 'deleteServer',
				errorMessage: res.error.message
			});
		}
		return { success: true, action: 'deleteServer' };
	},

	saveGroup: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id')) || 0;
		const name = String(form.get('name') ?? '').trim();
		const strategy = String(form.get('strategy') ?? 'round_robin');

		if (!name) {
			return fail(400, { action: 'saveGroup', errorKey: 'adminops.servers.groupNameRequired' });
		}

		const res = id
			? await apiFetch<unknown>(event, `/api/v1/admin/server-groups/${id}`, {
					method: 'PATCH',
					body: { name, strategy }
				})
			: await apiFetch<unknown>(event, '/api/v1/admin/server-groups', {
					method: 'POST',
					body: { name, strategy }
				});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				action: 'saveGroup',
				errorMessage: res.error.message
			});
		}
		return { success: true, action: 'saveGroup' };
	},

	deleteGroup: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { action: 'deleteGroup', errorMessage: 'invalid group id' });

		const res = await apiFetch<unknown>(event, `/api/v1/admin/server-groups/${id}`, {
			method: 'DELETE'
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				action: 'deleteGroup',
				errorMessage: res.error.message
			});
		}
		return { success: true, action: 'deleteGroup' };
	}
};
