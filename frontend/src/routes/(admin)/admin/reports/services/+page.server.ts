import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

interface ServicesStatusRow {
	status: string;
	count: number;
}

interface ServicesProductRow {
	product_id?: number;
	product_name?: string;
	name?: string;
	count: number;
}

interface ServicesReport {
	total?: number;
	by_status?: ServicesStatusRow[];
	by_product?: ServicesProductRow[];
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<ServicesReport>(event, '/api/v1/admin/reports/services');

	const report = res.data ?? {};
	const byStatus = report.by_status ?? [];
	const byProduct = (report.by_product ?? []).map((row) => ({
		name: row.product_name || row.name || `#${row.product_id ?? '?'}`,
		count: row.count || 0
	}));
	const total = report.total ?? byStatus.reduce((sum, row) => sum + (row.count || 0), 0);

	return {
		total,
		byStatus,
		byProduct,
		errorMessage: res.error?.message ?? null
	};
};
