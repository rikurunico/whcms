import { apiFetch } from '$lib/server/api';
import type { BillingCycle } from '$lib/stores/cart.svelte';
import type { PageServerLoad } from './$types';

/** JSON shapes per backend/internal/domain/entities.go tags. */
export interface CatalogPricing {
	id: number;
	product_id: number;
	cycle: BillingCycle;
	price: number;
	setup_fee: number;
	currency: string;
}

export interface CatalogProduct {
	id: number;
	group_id: number;
	name: string;
	slug: string;
	description: string;
	type: 'shared_hosting' | 'reseller_hosting' | 'domain' | 'other';
	stock_enabled: boolean;
	in_stock: boolean;
	/** Pricing rows embedded by the public catalog (catalog.PublicProduct). */
	pricing?: CatalogPricing[];
}

/**
 * GET /api/v1/products returns the visible catalog *grouped* by product group
 * (catalog.PublicGroup[]) - each group embeds its already-visible products.
 * The store page wants a flat product list, so we flatten it here, preserving
 * the backend order (products come pre-sorted by sort,id within each group).
 */
interface ApiPublicGroup {
	id: number;
	name: string;
	slug: string;
	sort: number;
	products: CatalogProduct[] | null;
}

export const load: PageServerLoad = async (event) => {
	const [{ groups }, catalogRes] = await Promise.all([
		event.parent(),
		apiFetch<ApiPublicGroup[]>(event, '/api/v1/products', { query: { per_page: 100 } })
	]);

	const products = (catalogRes.data ?? []).flatMap((g) => g.products ?? []);

	return {
		groups,
		products,
		loadError: catalogRes.error?.message ?? null,
		activeGroupSlug: event.url.searchParams.get('group') ?? ''
	};
};
