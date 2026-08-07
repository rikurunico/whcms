import { apiFetch } from '$lib/server/api';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { normalizeReturnInfo } from './return';

/**
 * Duitku return URL landing: /payments/return?merchantOrderId=&resultCode=
 * Maps the merchantOrderId to the invoice via GET /api/v1/payments/return.
 * Paid -> straight to the invoice page; otherwise render the processing view
 * (which polls ./status until the callback settles).
 */
export const load: PageServerLoad = async (event) => {
	const merchantOrderId = event.url.searchParams.get('merchantOrderId') ?? '';
	const resultCode = event.url.searchParams.get('resultCode') ?? '';
	const reference = event.url.searchParams.get('reference') ?? '';

	if (!merchantOrderId) {
		return {
			missing: true,
			merchantOrderId: '',
			invoiceId: null,
			status: null,
			errorMessage: null
		};
	}

	const res = await apiFetch<unknown>(event, '/api/v1/payments/return', {
		query: {
			merchantOrderId,
			resultCode: resultCode || undefined,
			reference: reference || undefined
		}
	});

	const info = normalizeReturnInfo(res.data);

	if (info.status === 'paid' && info.invoiceId) {
		redirect(303, `/billing/invoices/${info.invoiceId}`);
	}

	return {
		missing: false,
		merchantOrderId,
		invoiceId: info.invoiceId,
		status: info.status,
		errorMessage: res.error?.message ?? null
	};
};
