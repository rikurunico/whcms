import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

/** domain.IntegrationLog JSON tags; request/response are json.RawMessage (redacted). */
export interface IntegrationLogRow {
	id: number;
	provider: string;
	endpoint: string;
	method: string;
	status_code: number;
	success: boolean;
	latency_ms: number;
	request: unknown;
	response: unknown;
	error: string;
	created_at: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const provider = (q.get('provider') ?? '').trim();
	const success = q.get('success') ?? '';

	const res = await apiFetch<IntegrationLogRow[]>(event, '/api/v1/admin/logs/integration', {
		query: {
			page,
			per_page: perPage,
			provider: provider || undefined,
			success: success === 'true' || success === 'false' ? success : undefined
		}
	});

	return {
		logs: res.data ?? [],
		meta: res.meta,
		listError: res.error?.message ?? null,
		page,
		perPage,
		filters: { provider, success }
	};
};
