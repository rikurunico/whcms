import { apiFetch, type PageMeta } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import type { Invoice, Transaction } from './billing';

export type BillingTab = 'invoices' | 'transactions';

const INVOICE_STATUSES = ['unpaid', 'paid', 'overdue', 'cancelled', 'refunded'];

export const load: PageServerLoad = async (event) => {
	const { url } = event;

	const tab: BillingTab =
		url.searchParams.get('tab') === 'transactions' ? 'transactions' : 'invoices';
	const page = Math.max(1, Number(url.searchParams.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(url.searchParams.get('per_page')) || 10));
	const rawStatus = url.searchParams.get('status') ?? '';
	const status = INVOICE_STATUSES.includes(rawStatus) ? rawStatus : '';

	let invoices: Invoice[] = [];
	let transactions: Transaction[] = [];
	let meta: PageMeta | null = null;
	let errorMessage: string | null = null;

	if (tab === 'invoices') {
		const res = await apiFetch<Invoice[]>(event, '/api/v1/invoices', {
			query: { page, per_page: perPage, status: status || undefined }
		});
		if (res.error) {
			errorMessage = res.error.message;
		} else {
			invoices = res.data ?? [];
			meta = res.meta;
		}
	} else {
		// Not in CONTRACTS §9 - RESTful guess for the client transaction history tab.
		const res = await apiFetch<Transaction[]>(event, '/api/v1/transactions', {
			query: { page, per_page: perPage }
		});
		if (res.error) {
			errorMessage = res.error.message;
		} else {
			transactions = res.data ?? [];
			meta = res.meta;
		}
	}

	return {
		tab,
		page: meta?.page ?? page,
		perPage: meta?.per_page ?? perPage,
		total: meta?.total ?? 0,
		status,
		statusOptions: INVOICE_STATUSES,
		invoices,
		transactions,
		errorMessage
	};
};
