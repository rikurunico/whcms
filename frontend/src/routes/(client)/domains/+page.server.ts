import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { DomainRow } from './shared';

export const load: PageServerLoad = async (event) => {
	const sp = event.url.searchParams;
	const page = Math.max(1, Number(sp.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(sp.get('per_page')) || 10));
	const search = (sp.get('search') ?? '').trim();
	const status = sp.get('status') ?? '';

	const res = await apiFetch<DomainRow[]>(event, '/api/v1/domains', {
		query: {
			page,
			per_page: perPage,
			search: search || undefined,
			status: status || undefined
		}
	});

	return {
		domains: res.data ?? [],
		meta: res.meta ?? { page, per_page: perPage, total: res.data?.length ?? 0 },
		loadError: res.error?.message ?? null,
		search,
		status
	};
};

export const actions: Actions = {
	/** Inline auto-renew toggle from the list - PATCH /domains/:id {auto_renew}. */
	autorenew: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const value = String(form.get('value') ?? '') === 'true';

		if (!Number.isInteger(id) || id <= 0) {
			return fail(400, { errorKey: 'clientDomains.errors.invalidRequest' });
		}

		const res = await apiFetch<unknown>(event, `/api/v1/domains/${id}`, {
			method: 'PATCH',
			body: { auto_renew: value }
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, { errorMessage: res.error.message });
		}

		return { success: true, autoRenew: value };
	}
};
