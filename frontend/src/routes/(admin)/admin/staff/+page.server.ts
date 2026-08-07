import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { PERMISSION_MODULES } from './permissions';

/** domain.User JSON tags (staff/admin rows; password never serialized). */
export interface StaffRow {
	id: number;
	email: string;
	role: 'admin' | 'staff' | 'client';
	status: 'active' | 'inactive';
	permissions: unknown;
	twofa_enabled: boolean;
	last_login_at: string | null;
	created_at: string;
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<StaffRow[]>(event, '/api/v1/admin/staff', {
		query: { per_page: 100 }
	});

	return {
		staff: (res.data ?? []).filter((u) => u.role !== 'client'),
		listError: res.error?.message ?? null
	};
};

interface StaffInput {
	email: string;
	password: string;
	status: 'active' | 'inactive';
	permissions: Record<string, boolean>;
}

function readInput(form: FormData): StaffInput {
	const permissions: Record<string, boolean> = {};
	for (const mod of PERMISSION_MODULES) {
		permissions[mod] = form.get(`perm_${mod}`) === 'on';
	}
	return {
		email: String(form.get('email') ?? '').trim(),
		password: String(form.get('password') ?? ''),
		status: form.get('active') === 'on' ? 'active' : 'inactive',
		permissions
	};
}

export const actions: Actions = {
	create: async (event) => {
		const form = await event.request.formData();
		const input = readInput(form);

		const errors: Record<string, string> = {};
		if (!input.email) errors.email = 'adminsupport.staff.emailRequired';
		if (!input.password) errors.password = 'adminsupport.staff.passwordRequired';
		if (Object.keys(errors).length > 0) {
			return fail(400, { fieldErrors: errors, values: { email: input.email } });
		}

		const res = await apiFetch(event, '/api/v1/admin/staff', {
			method: 'POST',
			body: {
				email: input.email,
				password: input.password,
				status: input.status,
				permissions: input.permissions
			}
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				errorMessage: res.error.message,
				values: { email: input.email }
			});
		}
		return { saved: true };
	},

	update: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id') ?? 0);
		if (!Number.isInteger(id) || id <= 0) {
			return fail(400, { errorMessage: 'invalid staff id' });
		}
		const input = readInput(form);

		const errors: Record<string, string> = {};
		if (!input.email) errors.email = 'adminsupport.staff.emailRequired';
		if (Object.keys(errors).length > 0) {
			return fail(400, { fieldErrors: errors, values: { email: input.email } });
		}

		const body: Record<string, unknown> = {
			email: input.email,
			status: input.status,
			permissions: input.permissions
		};
		// Blank password means "keep the current one".
		if (input.password) body.password = input.password;

		const res = await apiFetch(event, `/api/v1/admin/staff/${id}`, {
			method: 'PATCH',
			body
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				errorMessage: res.error.message,
				values: { email: input.email }
			});
		}
		return { saved: true, updated: true };
	},

	delete: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id') ?? 0);
		if (!Number.isInteger(id) || id <= 0) {
			return fail(400, { deleteErrorMessage: 'invalid staff id' });
		}

		const res = await apiFetch(event, `/api/v1/admin/staff/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				deleteErrorMessage: res.error.message
			});
		}
		return { deleted: true };
	}
};
