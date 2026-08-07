import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

/** domain.AuditLog JSON tags; before/after are json.RawMessage. */
export interface AuditLogRow {
	id: number;
	user_id: number | null;
	action: string;
	entity: string;
	entity_id: number;
	before: unknown;
	after: unknown;
	ip: string;
	created_at: string;
	/** Optional joined field - some list endpoints embed the actor email. */
	user_email?: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const userId = (q.get('user_id') ?? '').trim();
	const entity = (q.get('entity') ?? '').trim();
	const from = q.get('from') ?? '';
	const to = q.get('to') ?? '';

	const res = await apiFetch<AuditLogRow[]>(event, '/api/v1/admin/logs/audit', {
		query: {
			page,
			per_page: perPage,
			user_id: userId || undefined,
			entity: entity || undefined,
			from: from || undefined,
			to: to || undefined
		}
	});

	return {
		logs: res.data ?? [],
		meta: res.meta,
		listError: res.error?.message ?? null,
		page,
		perPage,
		filters: { userId, entity, from, to }
	};
};
