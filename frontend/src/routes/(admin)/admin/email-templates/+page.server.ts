import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

/** domain.EmailTemplate JSON tags (list may omit bodies). */
export interface EmailTemplateRow {
	id: number;
	key: string;
	locale: string;
	subject: string;
	body_html?: string;
	body_text?: string;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	// No pager in the UI: this page renders every template key grouped by
	// category (docs/DESIGN.md §8.5), so the fetch must cover the whole fixed set.
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 1000));
	const search = (q.get('search') ?? '').trim();

	const res = await apiFetch<EmailTemplateRow[]>(event, '/api/v1/admin/email-templates', {
		query: { page, per_page: perPage, search: search || undefined }
	});

	return {
		templates: res.data ?? [],
		meta: res.meta,
		listError: res.error?.message ?? null,
		page,
		perPage,
		search
	};
};
