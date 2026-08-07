import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

/** Recent order row embedded in the dashboard payload (subset of domain.Order + joined client name). */
interface RecentOrder {
	id: number;
	order_number: string;
	client_id: number;
	client_name?: string;
	status: string;
	total: number;
	created_at: string;
}

/** Recent ticket row embedded in the dashboard payload (subset of domain.Ticket). */
interface RecentTicket {
	id: number;
	ticket_number: string;
	client_id?: number | null;
	subject: string;
	status: string;
	priority: string;
	last_reply_at?: string | null;
	created_at: string;
}

/** ports.DashboardStats (backend/internal/ports/ports.go). */
interface DashboardStats {
	clients_active?: number;
	services_active?: number;
	invoices_unpaid?: number;
	invoices_overdue?: number;
	tickets_open?: number;
	orders_pending?: number;
	revenue_today?: number;
	revenue_this_month?: number;
	domains_active?: number;
	services_suspended?: number;
}

/** adminops.ExtraStats (backend/internal/modules/adminops/dto.go). */
interface DashboardExtra {
	orders_today?: number;
	unpaid_total?: number;
	overdue_total?: number;
	services_pending?: number;
}

/** GET /api/v1/admin/dashboard payload (adminops.DashboardData). */
interface AdminDashboard {
	stats?: DashboardStats;
	extra?: DashboardExtra;
	recent_orders?: RecentOrder[];
	recent_tickets?: RecentTicket[];
	module_actions_pending?: number;
	generated_at?: string;
}

/** One point of adminops' Revenue/OrdersReport series (ports.RevenuePoint). */
export interface SeriesPoint {
	period: string;
	amount?: number;
	count?: number;
}

interface RevenueReport {
	points?: SeriesPoint[];
}

interface OrdersReport {
	points?: SeriesPoint[];
}

function isoDate(d: Date): string {
	return d.toISOString().slice(0, 10);
}

const CHART_WINDOW_DAYS = 14;

export const load: PageServerLoad = async (event) => {
	const to = new Date();
	const from = new Date(to.getTime() - (CHART_WINDOW_DAYS - 1) * 24 * 60 * 60 * 1000);
	const range = { from: isoDate(from), to: isoDate(to) };

	const [dashboardRes, revenueRes, ordersRes] = await Promise.all([
		apiFetch<AdminDashboard>(event, '/api/v1/admin/dashboard'),
		apiFetch<RevenueReport>(event, '/api/v1/admin/reports/revenue', {
			query: { ...range, group_by: 'day' }
		}),
		apiFetch<OrdersReport>(event, '/api/v1/admin/reports/orders', { query: range })
	]);

	return {
		dashboard: dashboardRes.data,
		dashboardError: dashboardRes.error ? dashboardRes.error.message : null,
		chartWindowDays: CHART_WINDOW_DAYS,
		revenueSeries: revenueRes.data?.points ?? [],
		ordersSeries: ordersRes.data?.points ?? []
	};
};
