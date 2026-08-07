import { apiFetch } from '$lib/server/api';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { normalizeReturnInfo } from '../return';

/**
 * Polling proxy for the payment-return page (browser -> SvelteKit ->
 * GET /api/v1/payments/return?merchantOrderId=).
 */
export const GET: RequestHandler = async (event) => {
	const merchantOrderId = event.url.searchParams.get('merchantOrderId') ?? '';
	if (!merchantOrderId) {
		return json({ status: null, invoice_id: null }, { status: 400 });
	}

	const res = await apiFetch<unknown>(event, '/api/v1/payments/return', {
		query: { merchantOrderId }
	});
	if (res.error) {
		return json({ status: null, invoice_id: null }, { status: res.status || 502 });
	}

	const info = normalizeReturnInfo(res.data);
	return json({ status: info.status, invoice_id: info.invoiceId });
};
