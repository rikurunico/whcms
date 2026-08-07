import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

/** /admin/logs has no page of its own - send visitors to the audit log. */
export const load: PageServerLoad = () => {
	redirect(303, '/admin/logs/audit');
};
