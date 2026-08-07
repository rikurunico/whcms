import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

/** /admin/reports has no content of its own - land on the revenue report. */
export const load: PageServerLoad = () => {
	redirect(302, '/admin/reports/revenue');
};
