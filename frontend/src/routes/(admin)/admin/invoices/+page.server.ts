import { apiFetch, type PageMeta } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import { INVOICE_STATUSES, type AdminInvoice, type InvoiceStatus } from './types';

export const load: PageServerLoad = async (event) => {
	const { url } = event;

	const page = Math.max(1, Number(url.searchParams.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(url.searchParams.get('per_page')) || 10));
	const rawStatus = url.searchParams.get('status') ?? '';
	const status = (INVOICE_STATUSES as string[]).includes(rawStatus)
		? (rawStatus as InvoiceStatus)
		: '';
	const search = (url.searchParams.get('search') ?? '').trim();
	const dateFrom = url.searchParams.get('date_from') ?? '';
	const dateTo = url.searchParams.get('date_to') ?? '';

	let invoices: AdminInvoice[] = [];
	let meta: PageMeta | null = null;
	let errorMessage: string | null = null;

	const res = await apiFetch<AdminInvoice[]>(event, '/api/v1/admin/invoices', {
		query: {
			page,
			per_page: perPage,
			status: status || undefined,
			search: search || undefined,
			date_from: dateFrom || undefined,
			date_to: dateTo || undefined
		}
	});

	if (res.error) {
		errorMessage = res.error.message;
	} else {
		invoices = res.data ?? [];
		meta = res.meta;
	}

	return {
		invoices,
		page: meta?.page ?? page,
		perPage: meta?.per_page ?? perPage,
		total: meta?.total ?? 0,
		status,
		search,
		dateFrom,
		dateTo,
		statusOptions: INVOICE_STATUSES,
		errorMessage
	};
};
