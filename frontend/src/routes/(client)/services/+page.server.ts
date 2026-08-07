import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import { SERVICE_STATUSES, type Service } from './types';

export const load: PageServerLoad = async (event) => {
	const sp = event.url.searchParams;
	const page = Math.max(1, Number(sp.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(sp.get('per_page')) || 10));
	const statusParam = sp.get('status') ?? '';
	const status = (SERVICE_STATUSES as readonly string[]).includes(statusParam) ? statusParam : '';
	const search = (sp.get('search') ?? '').trim();

	const res = await apiFetch<Service[]>(event, '/api/v1/services', {
		query: {
			page,
			per_page: perPage,
			status: status || undefined,
			search: search || undefined
		}
	});

	return {
		services: res.data ?? [],
		meta: res.meta ?? { page, per_page: perPage, total: (res.data ?? []).length },
		loadError: res.error?.message ?? null,
		filters: { status, search }
	};
};
