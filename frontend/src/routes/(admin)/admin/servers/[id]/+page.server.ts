import { apiFetch } from '$lib/server/api';
import { error, fail, redirect } from '@sveltejs/kit';
import { parseServerForm, parseTestConnectionForm } from '../serverForm';
import type { Actions, PageServerLoad } from './$types';

/** Admin server detail - domain.Server JSON tags (secrets never returned). */
interface AdminServer {
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
	package_prefix: string;
	ip_address: string;
	active: boolean;
	created_at: string;
	updated_at: string;
	accounts_count?: number;
}

interface AdminServerGroup {
	id: number;
	name: string;
}

interface TestConnectionResult {
	ok?: boolean;
	message?: string;
	version?: string;
	hostname?: string;
	nameservers?: string[];
}

export const load: PageServerLoad = async (event) => {
	const [serverRes, groupsRes] = await Promise.all([
		apiFetch<AdminServer>(event, `/api/v1/admin/servers/${event.params.id}`),
		apiFetch<AdminServerGroup[]>(event, '/api/v1/admin/server-groups')
	]);

	if (serverRes.error || !serverRes.data) {
		error(serverRes.status >= 400 ? serverRes.status : 500, serverRes.error?.message ?? 'error');
	}

	return { server: serverRes.data, groups: groupsRes.data ?? [] };
};

export const actions: Actions = {
	save: async (event) => {
		const form = await event.request.formData();
		const { body, errorKey } = parseServerForm(form);
		if (!body) return fail(400, { action: 'save', errorKey });

		const res = await apiFetch<unknown>(event, `/api/v1/admin/servers/${event.params.id}`, {
			method: 'PATCH',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				action: 'save',
				errorMessage: res.error.message
			});
		}
		return { success: true, action: 'save' };
	},

	test: async (event) => {
		// Drive the pre-save probe with the current form values so the admin can
		// re-test edited settings; the id lets the backend fall back to the
		// stored (encrypted) secret when the token field was left blank.
		const form = await event.request.formData();
		const res = await apiFetch<TestConnectionResult>(
			event,
			'/api/v1/admin/servers/test-connection',
			{
				method: 'POST',
				body: parseTestConnectionForm(form, Number(event.params.id))
			}
		);
		if (res.error || res.data?.ok === false) {
			return fail(res.status >= 400 ? res.status : 502, {
				action: 'test',
				errorMessage: res.error?.message ?? res.data?.message ?? 'connection failed'
			});
		}
		return {
			success: true,
			action: 'test',
			ok: true,
			message: res.data?.message ?? '',
			version: res.data?.version ?? '',
			hostname: res.data?.hostname ?? '',
			nameservers: res.data?.nameservers ?? []
		};
	},

	delete: async (event) => {
		const res = await apiFetch<unknown>(event, `/api/v1/admin/servers/${event.params.id}`, {
			method: 'DELETE'
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				action: 'delete',
				errorMessage: res.error.message
			});
		}
		redirect(303, '/admin/servers');
	}
};
