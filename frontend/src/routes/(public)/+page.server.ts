import { apiFetch } from '$lib/server/api';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

/** Product group card source (subset of the catalog group shape). */
export interface CatalogGroup {
	id: number;
	name: string;
	slug: string;
	sort: number;
	hidden: boolean;
}

/** Product row (only the fields the portal needs). */
export interface CatalogProduct {
	id: number;
	group_id: number;
	name: string;
	slug: string;
	hidden: boolean;
	sort: number;
}

export const load: PageServerLoad = async (event) => {
	// Signed-in visitors are sent to their area - the portal is the anon landing.
	if (event.locals.user) {
		redirect(303, event.locals.user.role === 'client' ? '/dashboard' : '/admin');
	}

	const [groupsRes, productsRes] = await Promise.all([
		apiFetch<CatalogGroup[]>(event, '/api/v1/product-groups', { query: { per_page: 100 } }),
		apiFetch<CatalogProduct[]>(event, '/api/v1/products', { query: { per_page: 100 } })
	]);

	return {
		groups: groupsRes.data ?? [],
		products: productsRes.data ?? [],
		loadError: groupsRes.error?.message ?? productsRes.error?.message ?? null
	};
};
