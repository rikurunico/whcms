import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.DomainAddon JSON tags - always exactly 3 fixed rows (seeded by
 *  migration: id_protection, dns_management, email_forwarding). Admin can
 *  only edit price/active; key/name are immutable. */
export interface DomainAddonRow {
	id: number;
	key: string;
	name: string;
	price: number;
	active: boolean;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<DomainAddonRow[]>(event, '/api/v1/admin/domain-addons');
	return {
		addons: res.data ?? [],
		listError: res.error ? res.error.message : null
	};
};

export const actions: Actions = {
	save: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { id: 0, errorMessage: 'Invalid domain addon id.' });

		const price = Math.max(0, Math.round(Number(form.get('price') ?? 0) || 0));
		const active = form.get('active') !== null;

		const res = await apiFetch(event, `/api/v1/admin/domain-addons/${id}`, {
			method: 'PUT',
			body: { price, active }
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, { id, errorMessage: res.error.message });
		}
		return { id, success: true };
	}
};
