import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';

export interface NetworkIssue {
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
	const res = await apiFetch<NetworkIssue[]>(event, '/api/v1/network-status');
	return {
		issues: res.data ?? [],
		loadError: res.error?.message ?? null
	};
};
