import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { extractInvoiceId, normalizeCreditBalance } from '../billing';

const MIN_DEPOSIT = 10000;

export const load: PageServerLoad = async (event) => {
	// Balance display is best-effort - the form works even when the ledger
	// endpoint is unavailable or its shape differs.
	const res = await apiFetch<unknown>(event, '/api/v1/account/credit', {
		query: { per_page: 1 }
	});
	return {
		creditBalance: res.error ? null : normalizeCreditBalance(res.data),
		minDeposit: MIN_DEPOSIT
	};
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const raw = String(form.get('amount') ?? '').trim();
		const amount = Math.floor(Number(raw));

		if (!raw || !Number.isFinite(amount) || amount < MIN_DEPOSIT) {
			return fail(400, { errorKey: 'clientBilling.deposit.minError', amount: raw });
		}

		const res = await apiFetch<unknown>(event, '/api/v1/account/credit/deposit', {
			method: 'POST',
			body: { amount }
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				errorMessage: res.error.message,
				amount: raw
			});
		}

		const invoiceId = extractInvoiceId(res.data);
		if (invoiceId) {
			redirect(303, `/billing/invoices/${invoiceId}`);
		}

		// Invoice id not recognizable in the response - reconciled at E2E.
		return { success: true };
	}
};
