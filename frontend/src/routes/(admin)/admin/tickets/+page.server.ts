import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import type { StaffUser, Ticket, TicketDepartment } from './types';

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const status = q.get('status') ?? '';
	const departmentId = q.get('department_id') ?? '';
	const priority = q.get('priority') ?? '';
	const assigned = q.get('assigned_user_id') ?? '';
	const search = (q.get('search') ?? '').trim();

	const [ticketsRes, deptRes, staffRes] = await Promise.all([
		apiFetch<Ticket[]>(event, '/api/v1/admin/tickets', {
			query: {
				page,
				per_page: perPage,
				status: status || undefined,
				department_id: departmentId || undefined,
				priority: priority || undefined,
				assigned_user_id: assigned || undefined,
				search: search || undefined
			}
		}),
		apiFetch<TicketDepartment[]>(event, '/api/v1/ticket-departments'),
		apiFetch<StaffUser[]>(event, '/api/v1/admin/staff')
	]);

	return {
		tickets: ticketsRes.data ?? [],
		meta: ticketsRes.meta,
		listError: ticketsRes.error?.message ?? null,
		departments: deptRes.data ?? [],
		staff: (staffRes.data ?? []).filter((u) => u.role !== 'client'),
		page,
		perPage,
		filters: { status, departmentId, priority, assigned, search }
	};
};
