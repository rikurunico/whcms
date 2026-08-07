import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.KBCategory JSON tags. */
export interface CategoryRow {
	id: number;
	name: string;
	slug: string;
	description: string;
	sort: number;
	hidden: boolean;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<CategoryRow[]>(event, '/api/v1/admin/kb/categories');
	return {
		categories: res.data ?? [],
		listError: res.error ? res.error.message : null
	};
};

function baseFields(form: FormData) {
	return {
		name: String(form.get('name') ?? '').trim(),
		slug: String(form.get('slug') ?? '')
			.trim()
			.toLowerCase(),
		description: String(form.get('description') ?? ''),
		sort: Number(form.get('sort') ?? 0) || 0,
		hidden: form.get('hidden') !== null
	};
}

/** Build a request body, omitting slug when blank so the backend derives it. */
function toBody(f: ReturnType<typeof baseFields>) {
	const body: Record<string, unknown> = {
		name: f.name,
		description: f.description,
		sort: f.sort,
		hidden: f.hidden
	};
	if (f.slug) body.slug = f.slug;
	return body;
}

export const actions: Actions = {
	create: async (event) => {
		const form = await event.request.formData();
		const f = baseFields(form);
		if (!f.name) return fail(400, { op: 'save', errorKey: 'adminPortal.kb.categoryFillRequired' });
		const res = await apiFetch(event, '/api/v1/admin/kb/categories', {
			method: 'POST',
			body: toBody(f)
		});
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
		const f = baseFields(form);
		if (!id || !f.name)
			return fail(400, { op: 'save', errorKey: 'adminPortal.kb.categoryFillRequired' });
		const res = await apiFetch(event, `/api/v1/admin/kb/categories/${id}`, {
			method: 'PATCH',
			body: toBody(f)
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
		if (!id) return fail(400, { op: 'delete', errorKey: 'adminPortal.kb.categoryDeleteFailed' });
		const res = await apiFetch(event, `/api/v1/admin/kb/categories/${id}`, { method: 'DELETE' });
		if (res.error) {
			// CONFLICT (409) when the category still has articles.
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorKey: res.status === 409 ? 'adminPortal.kb.categoryHasArticles' : undefined,
				errorMessage: res.status === 409 ? undefined : res.error.message
			});
		}
		return { op: 'delete', success: true };
	}
};
