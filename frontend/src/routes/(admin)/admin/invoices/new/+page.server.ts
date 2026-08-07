import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { AdminClientLite, AdminInvoice } from '../types';

interface DraftItem {
	description: string;
	amount: number;
	taxed: boolean;
}

function parseItems(raw: string): DraftItem[] | null {
	try {
		const parsed = JSON.parse(raw) as unknown;
		if (!Array.isArray(parsed)) return null;
		const items: DraftItem[] = [];
		for (const entry of parsed) {
			if (typeof entry !== 'object' || entry === null) return null;
			const rec = entry as Record<string, unknown>;
			const description = String(rec.description ?? '').trim();
			const amount = Math.trunc(Number(rec.amount ?? 0));
			const taxed = rec.taxed === true;
			if (!description || !Number.isFinite(amount)) return null;
			items.push({ description, amount, taxed });
		}
		return items;
	} catch {
		return null;
	}
}

export const load: PageServerLoad = async (event) => {
	const search = (event.url.searchParams.get('client_search') ?? '').trim();

	const res = await apiFetch<AdminClientLite[]>(event, '/api/v1/admin/clients', {
		query: { search: search || undefined, per_page: 20, page: 1 }
	});

	return {
		clientSearch: search,
		clients: res.data ?? [],
		clientsError: res.error?.message ?? null
	};
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const clientId = Number(form.get('client_id') ?? 0);
		const dueDate = String(form.get('due_date') ?? '').trim();
		const notes = String(form.get('notes') ?? '').trim();
		const itemsRaw = String(form.get('items_json') ?? '[]');

		const values = { clientId, dueDate, notes, itemsRaw };

		if (!clientId || clientId < 1) {
			return fail(400, { errorKey: 'adminBilling.create.errClient', ...values });
		}
		if (!dueDate) {
			return fail(400, { errorKey: 'adminBilling.create.errDueDate', ...values });
		}
		const items = parseItems(itemsRaw);
		if (!items || items.length === 0) {
			return fail(400, { errorKey: 'adminBilling.create.errItems', ...values });
		}

		const res = await apiFetch<AdminInvoice>(event, '/api/v1/admin/invoices', {
			method: 'POST',
			body: { client_id: clientId, due_date: dueDate, notes, items }
		});

		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 500, {
				errorMessage: res.error?.message ?? 'Failed to create invoice',
				...values
			});
		}

		redirect(303, `/admin/invoices/${res.data.id}`);
	}
};
