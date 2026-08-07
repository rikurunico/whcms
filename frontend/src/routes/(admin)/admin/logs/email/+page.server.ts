import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.EmailLogEntry JSON tags. */
export interface EmailLogRow {
	id: number;
	to_email: string;
	template_key: string;
	subject: string;
	status: 'queued' | 'sent' | 'failed' | string;
	error: string;
	sent_at: string | null;
	created_at: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const status = q.get('status') ?? '';
	const search = (q.get('search') ?? '').trim();

	const res = await apiFetch<EmailLogRow[]>(event, '/api/v1/admin/logs/email', {
		query: {
			page,
			per_page: perPage,
			status: status || undefined,
			search: search || undefined
		}
	});

	return {
		logs: res.data ?? [],
		meta: res.meta,
		listError: res.error?.message ?? null,
		page,
		perPage,
		filters: { status, search }
	};
};

export const actions: Actions = {
	retry: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id') ?? 0);
		if (!Number.isInteger(id) || id <= 0) {
			return fail(400, { retryErrorMessage: 'invalid email log id' });
		}

		// Retry lives in the notifications module under /admin/email-log, not
		// /admin/logs/email (that path is the adminops read-only log viewer).
		const res = await apiFetch(event, `/api/v1/admin/email-log/${id}/retry`, {
			method: 'POST'
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				retryErrorMessage: res.error.message,
				retryId: id
			});
		}
		return { retried: true, retryId: id };
	}
};
