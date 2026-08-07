import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';

/** Created client - domain.Client JSON tags (some backends may nest it under {client}). */
interface CreatedClient {
	id?: number;
	client?: { id?: number };
}

/** Map backend VALIDATION error details ([{field, message}]) to a per-field error map. */
function fieldErrors(details: unknown[]): Record<string, string> {
	const out: Record<string, string> = {};
	for (const d of details) {
		if (d && typeof d === 'object') {
			const rec = d as Record<string, unknown>;
			const field = typeof rec.field === 'string' ? rec.field : undefined;
			const message =
				typeof rec.message === 'string'
					? rec.message
					: typeof rec.error === 'string'
						? rec.error
						: undefined;
			if (field) out[field] = message ?? 'invalid';
		}
	}
	return out;
}

const PROFILE_FIELDS = [
	'first_name',
	'last_name',
	'company',
	'address1',
	'address2',
	'city',
	'state',
	'postcode',
	'country',
	'phone'
] as const;

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();

		const values: Record<string, string> = {
			email: String(form.get('email') ?? '').trim(),
			password: String(form.get('password') ?? '')
		};
		for (const f of PROFILE_FIELDS) {
			values[f] = String(form.get(f) ?? '').trim();
		}

		if (!values.email || !values.password || !values.first_name || !values.last_name) {
			return fail(400, { errorKey: 'admincore.clients.fillRequired', values, fields: {} });
		}

		const res = await apiFetch<CreatedClient>(event, '/api/v1/admin/clients', {
			method: 'POST',
			body: { ...values, country: values.country || 'ID' }
		});

		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 400, {
				errorMessage: res.error?.message,
				errorKey: res.error ? undefined : 'admincore.clients.createFailed',
				fields: fieldErrors(res.error?.details ?? []),
				values
			});
		}

		const id = res.data.id ?? res.data.client?.id;
		redirect(303, id ? `/admin/clients/${id}` : '/admin/clients');
	}
};
