import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { AdminClientLite, AdminInvoice, AdminInvoiceItem, AdminTransaction } from '../types';

/**
 * The real backend response for GET /admin/invoices/:id is
 * `{invoice: {...}, items: [...]}` - NOT the flat invoice directly. (Every
 * OTHER admin detail endpoint used by this app returns its entity flat, so
 * this one nested exception is easy to miss; a prior version of this loader
 * assumed flat and set `invoice = res.data`, which silently produced an
 * object with no `status`/`invoice_number`/etc. - anything that merely
 * displayed those fields as text rendered blank, but the status banner's
 * `invoice.status.toUpperCase()` crashed the whole page with a 500.)
 */
interface AdminInvoiceDetailResponse {
	invoice: AdminInvoice;
	items?: AdminInvoiceItem[];
}

export const load: PageServerLoad = async (event) => {
	const id = event.params.id;

	// The transactions list only depends on the route param, so it runs
	// concurrently with the invoice fetch.
	const invoicePromise = apiFetch<AdminInvoiceDetailResponse>(event, `/api/v1/admin/invoices/${id}`);
	const txPromise = apiFetch<AdminTransaction[]>(event, '/api/v1/admin/transactions', {
		query: { invoice_id: id, per_page: 50 }
	});
	const res = await invoicePromise;

	if (res.error || !res.data?.invoice) {
		return {
			invoice: null as AdminInvoice | null,
			client: null as AdminClientLite | null,
			transactions: [] as AdminTransaction[],
			transactionsError: null as string | null,
			errorMessage: res.error?.message ?? 'Invoice not found',
			notFound: res.status === 404
		};
	}

	const invoice: AdminInvoice = { ...res.data.invoice, items: res.data.items ?? [] };

	// Client block: use the embedded client when present, otherwise fetch it.
	let client: AdminClientLite | null = invoice.client ?? null;
	if (!client && invoice.client_id) {
		const clientRes = await apiFetch<AdminClientLite>(
			event,
			`/api/v1/admin/clients/${invoice.client_id}`
		);
		client = clientRes.data ?? null;
	}

	// Transaction history for this invoice (admin transactions list filtered).
	const txRes = await txPromise;

	return {
		invoice,
		client,
		transactions: txRes.data ?? [],
		transactionsError: txRes.error?.message ?? null,
		errorMessage: null as string | null,
		notFound: false
	};
};

function actionError(res: { status: number; error: { message: string } | null }) {
	return fail(res.status >= 400 ? res.status : 500, {
		errorMessage: res.error?.message ?? 'Request failed'
	});
}

export const actions: Actions = {
	update: async (event) => {
		const form = await event.request.formData();
		const dueDate = String(form.get('due_date') ?? '').trim();
		const notes = String(form.get('notes') ?? '').trim();

		if (!dueDate) {
			return fail(400, { errorKey: 'adminBilling.create.errDueDate' });
		}

		const res = await apiFetch<AdminInvoice>(event, `/api/v1/admin/invoices/${event.params.id}`, {
			method: 'PATCH',
			body: { due_date: dueDate, notes }
		});
		if (res.error) return actionError(res);
		return { success: 'update' as const };
	},

	addPayment: async (event) => {
		const form = await event.request.formData();
		const amount = Math.trunc(Number(form.get('amount') ?? 0));
		const method = String(form.get('method') ?? '').trim();

		if (!Number.isFinite(amount) || amount <= 0) {
			return fail(400, { errorKey: 'adminBilling.detail.errAmount' });
		}
		if (!method) {
			return fail(400, { errorKey: 'adminBilling.detail.errMethod' });
		}

		const res = await apiFetch(event, `/api/v1/admin/invoices/${event.params.id}/payment`, {
			method: 'POST',
			body: { amount, method }
		});
		if (res.error) return actionError(res);
		return { success: 'addPayment' as const };
	},

	refund: async (event) => {
		const form = await event.request.formData();
		const reason = String(form.get('reason') ?? '').trim();

		const res = await apiFetch(event, `/api/v1/admin/invoices/${event.params.id}/refund`, {
			method: 'POST',
			body: { reason }
		});
		if (res.error) return actionError(res);
		return { success: 'refund' as const };
	},

	cancel: async (event) => {
		const res = await apiFetch(event, `/api/v1/admin/invoices/${event.params.id}/cancel`, {
			method: 'POST',
			body: {}
		});
		if (res.error) return actionError(res);
		return { success: 'cancel' as const };
	},

	addItem: async (event) => {
		const form = await event.request.formData();
		const description = String(form.get('description') ?? '').trim();
		const amount = Math.trunc(Number(form.get('amount') ?? 0));
		const taxed = form.get('taxed') === 'on' || form.get('taxed') === 'true';

		if (!description) {
			return fail(400, { errorKey: 'adminBilling.detail.errItemDescription' });
		}
		if (!Number.isFinite(amount)) {
			return fail(400, { errorKey: 'adminBilling.detail.errAmount' });
		}

		const res = await apiFetch(event, `/api/v1/admin/invoices/${event.params.id}/items`, {
			method: 'POST',
			body: { description, amount, taxed }
		});
		if (res.error) return actionError(res);
		return { success: 'addItem' as const };
	},

	deleteItem: async (event) => {
		const form = await event.request.formData();
		const itemId = Number(form.get('item_id') ?? 0);

		if (!itemId || itemId < 1) {
			return fail(400, { errorMessage: 'Missing item id' });
		}

		const res = await apiFetch(event, `/api/v1/admin/invoices/${event.params.id}/items/${itemId}`, {
			method: 'DELETE'
		});
		if (res.error) return actionError(res);
		return { success: 'deleteItem' as const };
	}
};
