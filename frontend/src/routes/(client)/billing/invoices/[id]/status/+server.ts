import { apiFetch } from '$lib/server/api';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { normalizeInvoiceDetail } from '../../../billing';

/**
 * Invoice status polling proxy (browser -> SvelteKit -> GET /api/v1/invoices/:id).
 * The invoice detail page polls this every 4s while the invoice is payable.
 */
export const GET: RequestHandler = async (event) => {
	if (!event.locals.user) {
		return json({ status: null }, { status: 401 });
	}

	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) {
		return json({ status: null }, { status: 404 });
	}

	const res = await apiFetch<unknown>(event, `/api/v1/invoices/${id}`);
	if (res.error) {
		return json({ status: null }, { status: res.status || 502 });
	}

	const { invoice } = normalizeInvoiceDetail(res.data);
	return json({ status: invoice?.status ?? null });
};
