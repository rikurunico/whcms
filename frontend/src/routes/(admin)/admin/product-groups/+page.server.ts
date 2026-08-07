import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.ProductGroup JSON tags. */
interface ProductGroupRow {
	id: number;
	name: string;
	slug: string;
	sort: number;
	hidden: boolean;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<ProductGroupRow[]>(event, '/api/v1/admin/product-groups');
	return {
		groups: res.data ?? [],
		listError: res.error ? res.error.message : null
	};
};

function parseGroupForm(form: FormData) {
	return {
		name: String(form.get('name') ?? '').trim(),
		slug: String(form.get('slug') ?? '')
			.trim()
			.toLowerCase(),
		sort: Number(form.get('sort') ?? 0) || 0,
		hidden: form.get('hidden') !== null
	};
}

export const actions: Actions = {
	create: async (event) => {
		const form = await event.request.formData();
		const body = parseGroupForm(form);
		if (!body.name || !body.slug) {
			return fail(400, { op: 'save', errorKey: 'adcatalog.groups.fillRequired' });
		}
		const res = await apiFetch(event, '/api/v1/admin/product-groups', { method: 'POST', body });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'save',
				errorMessage: res.error.message
			});
		}
		return { op: 'save', success: true };
	},

	update: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const body = parseGroupForm(form);
		if (!id || !body.name || !body.slug) {
			return fail(400, { op: 'save', errorKey: 'adcatalog.groups.fillRequired' });
		}
		const res = await apiFetch(event, `/api/v1/admin/product-groups/${id}`, {
			method: 'PATCH',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'save',
				errorMessage: res.error.message
			});
		}
		return { op: 'save', success: true };
	},

	delete: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { op: 'delete', errorKey: 'adcatalog.groups.deleteFailed' });
		const res = await apiFetch(event, `/api/v1/admin/product-groups/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'delete', success: true };
	}
};
