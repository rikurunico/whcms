import { apiFetch } from '$lib/server/api';
import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

/** ports.PresenceEntry - one admin/staff user active within the presence window. */
export interface OnlineStaffEntry {
	user_id: number;
	email: string;
	role: string;
}

export const load: LayoutServerLoad = async (event) => {
	const { locals, url } = event;
	if (!locals.user) {
		redirect(303, `/login?redirect=${encodeURIComponent(url.pathname + url.search)}`);
	}
	if (locals.user.role !== 'admin' && locals.user.role !== 'staff') {
		redirect(303, '/dashboard');
	}

	const res = await apiFetch<OnlineStaffEntry[]>(event, '/api/v1/admin/staff/online');

	return { user: locals.user, onlineStaff: res.data ?? [] };
};
