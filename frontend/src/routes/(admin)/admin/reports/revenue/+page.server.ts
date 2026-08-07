import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import { parseGroupBy, parseRange } from '../range';

export interface RevenuePoint {
	period: string;
	amount: number;
	count?: number;
}

export interface GatewaySplit {
	gateway: string;
	amount: number;
	count?: number;
}

/**
 * The real backend payload (adminops.RevenueReportData, backend/internal/
 * modules/adminops/dto.go) uses `points`/`total_amount`/`total_count` - NOT
 * the `series`/`total`/`transaction_count` names docs/RECONCILE.md's FE
 * builders had guessed. `by_gateway` does match. Read the real field names
 * here; `series`/`total`/`txCount` below are just this loader's own return
 * shape for the page component.
 */
interface RevenueReport {
	total_amount?: number;
	total_count?: number;
	points?: RevenuePoint[];
	by_gateway?: GatewaySplit[];
}

export const load: PageServerLoad = async (event) => {
	const { from, to } = parseRange(event.url);
	const groupBy = parseGroupBy(event.url);

	const res = await apiFetch<RevenueReport>(event, '/api/v1/admin/reports/revenue', {
		query: { from, to, group_by: groupBy }
	});

	const report = res.data ?? {};
	const series = report.points ?? [];
	const byGateway = report.by_gateway ?? [];
	const total = report.total_amount ?? series.reduce((sum, p) => sum + (p.amount || 0), 0);
	const txCount = report.total_count ?? series.reduce((sum, p) => sum + (p.count ?? 0), 0);

	return {
		from,
		to,
		groupBy,
		total,
		txCount,
		series,
		byGateway,
		errorMessage: res.error?.message ?? null
	};
};
