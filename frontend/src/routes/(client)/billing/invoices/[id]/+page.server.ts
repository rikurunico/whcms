import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
	isPayable,
	normalizeCreditBalance,
	normalizeInvoiceDetail,
	normalizePayResult,
	type PaymentMethodInfo
} from '../../billing';

/**
 * Not-found state - the page renders the translated
 * `clientBilling.detail.notFound` alert (no app-wide +error.svelte exists,
 * so error(404) would show SvelteKit's bare untranslated default page).
 */
function notFound() {
	return {
		invoice: null,
		items: [],
		transactions: [],
		methods: null,
		methodsError: null,
		creditBalance: null,
		errorMessage: null
	};
}

export const load: PageServerLoad = async (event) => {
	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) {
		return notFound();
	}

	const res = await apiFetch<unknown>(event, `/api/v1/invoices/${id}`);

	if (res.status === 404 || res.error?.code === 'NOT_FOUND') {
		return notFound();
	}
	if (res.error) {
		return {
			invoice: null,
			items: [],
			transactions: [],
			methods: null,
			methodsError: null,
			creditBalance: null,
			errorMessage: res.error.message
		};
	}

	const detail = normalizeInvoiceDetail(res.data);

	let methods: PaymentMethodInfo[] | null = null;
	let methodsError: string | null = null;
	let creditBalance: number | null = null;

	if (detail.invoice && isPayable(detail.invoice.status)) {
		const [methodsRes, creditRes] = await Promise.all([
			apiFetch<PaymentMethodInfo[]>(event, '/api/v1/payments/methods', {
				query: { invoice_id: id }
			}),
			apiFetch<unknown>(event, '/api/v1/account/credit', { query: { per_page: 1 } })
		]);
		if (methodsRes.error) {
			methodsError = methodsRes.error.message;
		} else {
			methods = methodsRes.data ?? [];
		}
		// Balance shape is a guess (endpoint returns the ledger) - hide the credit
		// option gracefully when the balance cannot be determined.
		if (!creditRes.error) {
			creditBalance = normalizeCreditBalance(creditRes.data);
		}
	}

	return {
		invoice: detail.invoice,
		items: detail.items,
		transactions: detail.transactions,
		methods,
		methodsError,
		creditBalance,
		errorMessage: null
	};
};

export const actions: Actions = {
	pay: async (event) => {
		const id = Number(event.params.id);
		if (!Number.isInteger(id) || id <= 0) {
			return fail(404, { errorKey: 'clientBilling.detail.notFound', method: '' });
		}

		const form = await event.request.formData();
		const method = String(form.get('method') ?? '').trim();
		if (!method) {
			return fail(400, { errorKey: 'clientBilling.detail.selectMethodFirst', method });
		}

		const res = await apiFetch<unknown>(event, `/api/v1/invoices/${id}/pay`, {
			method: 'POST',
			body: { method }
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				errorMessage: res.error.message,
				method
			});
		}

		const payment = normalizePayResult(res.data);

		// VA/QRIS channels (Duitku) and the manual bank-transfer gateway all
		// give us something to render ourselves - show it inline instead of
		// sending the customer to Duitku's hosted page. `reference` alone
		// doesn't count - it's only ever shown as a supplementary detail
		// alongside these, never as the sole renderable instruction - so
		// channels with none of the above (credit card, e-wallets, retail
		// outlets, paylater) still fall back to the redirect.
		const hasInstructions = Boolean(
			payment.vaNumber || payment.qrString || payment.bankAccounts.length > 0
		);
		if (!hasInstructions && payment.paymentUrl) {
			redirect(303, payment.paymentUrl);
		}

		return { success: true, method, payment: hasInstructions ? payment : null };
	}
};
