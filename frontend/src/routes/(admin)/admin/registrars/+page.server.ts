import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** Admin registrar row - domains.RegistrarResponse JSON tags. reseller_id is
 *  a plain (non-secret) identifier; the API key itself is never returned,
 *  only whether one is set (from the DB or the environment). base_url is the
 *  "custom endpoint" override - blank falls back to RDASH_BASE_URL. */
export interface AdminRegistrar {
	id: number;
	name: string;
	active: boolean;
	config: Record<string, unknown> | null;
	reseller_id: string;
	api_key_present: boolean;
	base_url: string;
	created_at: string;
	updated_at: string;
}

/** Result of POST /admin/registrars/:id/test - domains.TestRegistrarResponse. */
interface TestResult {
	ok?: boolean;
	message?: string;
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<AdminRegistrar[]>(event, '/api/v1/admin/registrars');
	return {
		registrars: res.data ?? [],
		listError: res.error ? res.error.message : null
	};
};

export const actions: Actions = {
	save: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { action: 'save', errorMessage: 'invalid registrar id' });

		let config: Record<string, unknown>;
		try {
			const parsed: unknown = JSON.parse(String(form.get('config') ?? '{}'));
			if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
				throw new Error('not an object');
			}
			config = parsed as Record<string, unknown>;
		} catch {
			return fail(400, { action: 'save', errorKey: 'adminops.registrars.invalidConfig' });
		}

		// api_key is tri-state: omit entirely to leave the stored key untouched,
		// send "" only when the admin explicitly checks "Clear stored key",
		// otherwise send whatever new value was typed.
		const body: Record<string, unknown> = {
			active: form.get('active') === 'on',
			config,
			reseller_id: String(form.get('reseller_id') ?? ''),
			base_url: String(form.get('base_url') ?? '')
		};
		const clearApiKey = form.get('clear_api_key') === 'on';
		const apiKeyInput = String(form.get('api_key') ?? '');
		if (clearApiKey) {
			body.api_key = '';
		} else if (apiKeyInput !== '') {
			body.api_key = apiKeyInput;
		}

		const res = await apiFetch<unknown>(event, `/api/v1/admin/registrars/${id}`, {
			method: 'PUT',
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
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { action: 'test', errorMessage: 'invalid registrar id' });

		const res = await apiFetch<TestResult>(event, `/api/v1/admin/registrars/${id}/test`, {
			method: 'POST'
		});
		if (res.error || res.data?.ok === false) {
			return fail(res.status >= 400 ? res.status : 502, {
				action: 'test',
				testId: id,
				errorMessage: res.error?.message ?? res.data?.message ?? 'connection failed'
			});
		}
		return { success: true, action: 'test', testId: id, message: res.data?.message ?? '' };
	}
};
