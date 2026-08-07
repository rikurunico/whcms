import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

/** Admin client list row - domain.Client JSON tags + joined user email. */
interface AdminClientRow {
	id: number;
	user_id: number;
	first_name: string;
	last_name: string;
	company: string;
	email?: string;
	city?: string;
	country?: string;
	phone?: string;
	credit_balance: number;
	status: string;
	created_at: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const search = (q.get('search') ?? '').trim();
	const status = q.get('status') ?? '';

	const res = await apiFetch<AdminClientRow[]>(event, '/api/v1/admin/clients', {
		query: {
			page,
			per_page: perPage,
			search: search || undefined,
			status: status || undefined
		}
	});

	return {
		clients: res.data ?? [],
		meta: res.meta,
		listError: res.error ? res.error.message : null,
		page,
		perPage,
		search,
		status
	};
};
