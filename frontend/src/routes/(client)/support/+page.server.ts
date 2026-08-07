import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import { TICKET_STATUSES, type Ticket, type TicketDepartment } from './types';

export const load: PageServerLoad = async (event) => {
	const params = event.url.searchParams;
	const page = Math.max(1, Number(params.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(params.get('per_page')) || 10));
	const statusParam = params.get('status') ?? '';
	const status = (TICKET_STATUSES as readonly string[]).includes(statusParam) ? statusParam : '';

	const [ticketsRes, deptRes] = await Promise.all([
		apiFetch<Ticket[]>(event, '/api/v1/tickets', {
			query: { page, per_page: perPage, status: status || undefined }
		}),
		apiFetch<TicketDepartment[]>(event, '/api/v1/ticket-departments')
	]);

	return {
		tickets: ticketsRes.data ?? [],
		meta: ticketsRes.meta ?? { page, per_page: perPage, total: ticketsRes.data?.length ?? 0 },
		loadError: ticketsRes.error?.message ?? null,
		departments: deptRes.data ?? [],
		status
	};
};
