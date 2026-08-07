import type { LayoutServerLoad } from './$types';

/** Expose the (optional) session user to the public shell so the nav can show
 *  the logged-in account menu when a signed-in visitor browses public pages. */
export const load: LayoutServerLoad = ({ locals }) => {
	return { user: locals.user };
};
