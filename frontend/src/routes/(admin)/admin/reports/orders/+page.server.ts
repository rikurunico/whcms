import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import { parseRange } from '../range';

interface OrdersStatusRow {
	status: string;
	count: number;
	value?: number;
}

interface OrdersPoint {
	period: string;
	count: number;
	value?: number;
}

interface OrdersReport {
	total?: number;
	total_orders?: number;
	total_value?: number;
	by_status?: OrdersStatusRow[];
	series?: OrdersPoint[];
}

export const load: PageServerLoad = async (event) => {
	const { from, to } = parseRange(event.url);

	const res = await apiFetch<OrdersReport>(event, '/api/v1/admin/reports/orders', {
		query: { from, to }
	});

	const report = res.data ?? {};
	const byStatus = report.by_status ?? [];
	const series = report.series ?? [];
	const totalOrders =
		report.total_orders ?? report.total ?? byStatus.reduce((sum, row) => sum + (row.count || 0), 0);
	const totalValue = report.total_value ?? byStatus.reduce((sum, row) => sum + (row.value ?? 0), 0);

	return {
		from,
		to,
		totalOrders,
		totalValue,
		byStatus,
		series,
		errorMessage: res.error?.message ?? null
	};
};
