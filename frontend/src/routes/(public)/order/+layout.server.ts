import { apiFetch } from '$lib/server/api';
import type { LayoutServerLoad } from './$types';

/** domain.ProductGroup JSON tags (public/non-hidden groups only). */
export interface CatalogGroup {
	id: number;
	name: string;
	slug: string;
	sort: number;
	hidden: boolean;
}

export const load: LayoutServerLoad = async (event) => {
	const groupsRes = await apiFetch<CatalogGroup[]>(event, '/api/v1/product-groups', {
		query: { per_page: 100 }
	});
	return {
		user: event.locals.user,
		groups: groupsRes.data ?? []
	};
};
