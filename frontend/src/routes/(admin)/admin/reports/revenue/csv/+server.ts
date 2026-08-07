import { apiFetch } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { parseGroupBy, parseRange } from '../../range';
import type { GatewaySplit, RevenuePoint } from '../+page.server';

interface RevenueReport {
	total?: number;
	transaction_count?: number;
	series?: RevenuePoint[];
	by_gateway?: GatewaySplit[];
}

function csvField(value: string | number): string {
	const s = String(value);
	return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/**
 * Builds the revenue CSV from the JSON report endpoint (no separate CSV API needed).
 * +server.ts routes bypass the (admin) layout guard, so re-check the role here.
 */
export const GET: RequestHandler = async (event) => {
	const { locals, url } = event;

	if (!locals.user || !locals.accessToken) {
		error(401, 'Unauthorized');
	}
	if (locals.user.role !== 'admin' && locals.user.role !== 'staff') {
		error(403, 'Forbidden');
	}

	const { from, to } = parseRange(url);
	const groupBy = parseGroupBy(url);

	const res = await apiFetch<RevenueReport>(event, '/api/v1/admin/reports/revenue', {
		query: { from, to, group_by: groupBy }
	});
	if (res.error) {
		error(res.status >= 400 ? res.status : 502, res.error.message);
	}

	const series = res.data?.series ?? [];
	const byGateway = res.data?.by_gateway ?? [];

	const lines: string[] = ['period,amount,count'];
	for (const p of series) {
		lines.push([csvField(p.period), csvField(p.amount ?? 0), csvField(p.count ?? '')].join(','));
	}
	lines.push('');
	lines.push('gateway,amount,count');
	for (const g of byGateway) {
		lines.push([csvField(g.gateway), csvField(g.amount ?? 0), csvField(g.count ?? '')].join(','));
	}

	return new Response(lines.join('\n') + '\n', {
		status: 200,
		headers: {
			'content-type': 'text/csv; charset=utf-8',
			'content-disposition': `attachment; filename="revenue-${from}-${to}.csv"`,
			'cache-control': 'private, no-store'
		}
	});
};
