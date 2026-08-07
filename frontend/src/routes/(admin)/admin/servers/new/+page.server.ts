import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import { parseServerForm, parseTestConnectionForm } from '../serverForm';
import type { Actions, PageServerLoad } from './$types';

interface AdminServerGroup {
	id: number;
	name: string;
}

interface CreatedServer {
	id: number;
}

/** provisioning.TestConnectionResult. */
interface TestConnectionResult {
	ok?: boolean;
	message?: string;
	version?: string;
	hostname?: string;
	nameservers?: string[];
}

export const load: PageServerLoad = async (event) => {
	const groupsRes = await apiFetch<AdminServerGroup[]>(event, '/api/v1/admin/server-groups');
	return { groups: groupsRes.data ?? [] };
};

export const actions: Actions = {
	save: async (event) => {
		const form = await event.request.formData();
		const { body, errorKey } = parseServerForm(form);
		if (!body) return fail(400, { errorKey });

		const res = await apiFetch<CreatedServer>(event, '/api/v1/admin/servers', {
			method: 'POST',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, { errorMessage: res.error.message });
		}

		redirect(303, res.data?.id ? `/admin/servers/${res.data.id}` : '/admin/servers');
	},

	test: async (event) => {
		const form = await event.request.formData();
		const res = await apiFetch<TestConnectionResult>(
			event,
			'/api/v1/admin/servers/test-connection',
			{
				method: 'POST',
				body: parseTestConnectionForm(form)
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
	}
};
