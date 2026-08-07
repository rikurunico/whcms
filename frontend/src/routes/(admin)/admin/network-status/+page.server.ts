import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.NetworkIssue JSON tags. */
export interface NetworkIssueRow {
	id: number;
	title: string;
	body: string;
	type: 'scheduled' | 'issue' | 'outage';
	severity: 'minor' | 'major' | 'critical';
	status: 'investigating' | 'identified' | 'monitoring' | 'resolved' | 'scheduled';
	affected: string;
	starts_at: string;
	ends_at: string | null;
	created_at: string;
	updated_at: string;
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const page = Math.max(1, Number(q.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(q.get('per_page')) || 10));
	const search = (q.get('search') ?? '').trim();
	const status = q.get('status') ?? '';

	const res = await apiFetch<NetworkIssueRow[]>(event, '/api/v1/admin/network-issues', {
		query: {
			page,
			per_page: perPage,
			search: search || undefined,
			status: status || undefined
		}
	});

	return {
		issues: res.data ?? [],
		meta: res.meta,
		listError: res.error ? res.error.message : null,
		page,
		perPage,
		search,
		status
	};
};

export const actions: Actions = {
	delete: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { op: 'delete', errorKey: 'adminPortal.network.deleteFailed' });
		const res = await apiFetch(event, `/api/v1/admin/network-issues/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'delete', success: true };
	}
};
