import { apiFetch } from '$lib/server/api';
import type { BillingCycle } from '$lib/stores/cart.svelte';
import type { PageServerLoad } from './$types';

/** JSON shapes per backend/internal/domain/entities.go tags. */
export interface ProductPricingRow {
	id: number;
	product_id: number;
	cycle: BillingCycle;
	price: number;
	setup_fee: number;
	currency: string;
}

export interface OptionValueRow {
	id: number;
	option_id: number;
	name: string;
	/** Per-cycle price deltas, e.g. {"monthly": 10000, "annually": 100000}. */
	price_deltas: Partial<Record<BillingCycle, number>> | null;
	sort: number;
}

export interface OptionRow {
	id: number;
	group_id: number;
	name: string;
	sort: number;
	values: OptionValueRow[];
}

export interface OptionGroupRow {
	id: number;
	name: string;
	description: string;
	options: OptionRow[];
}

export interface SpecPriceRow {
	cycle: BillingCycle;
	unit_price: number;
	unlimited_price: number;
	currency: string;
}

/** A configurable spec knob (dynamic/custom products). */
export interface SpecRow {
	key: string;
	label: string;
	provision_key: string;
	unit: 'gb' | 'mb' | 'count';
	included_qty: number;
	min_qty: number;
	max_qty: number;
	step_qty: number;
	default_qty: number;
	allow_unlimited: boolean;
	pricing: SpecPriceRow[];
}

export interface ProductDetail {
	id: number;
	group_id: number;
	name: string;
	slug: string;
	description: string;
	type: 'shared_hosting' | 'reseller_hosting' | 'domain' | 'other';
	hidden: boolean;
	/** Whether the customer configures their own specs (dynamic product). */
	configurable?: boolean;
	/** Pricing rows attached by GET /products/:slug (nesting shape guessed - see report). */
	pricing?: ProductPricingRow[];
	/** Configurable option groups attached by GET /products/:slug (nesting shape guessed). */
	option_groups?: OptionGroupRow[];
	/** Dynamic spec knobs (only on configurable products). */
	specs?: SpecRow[];
}

/** GET /api/v1/domains/addons response row (domain.DomainAddon JSON tags). */
export interface DomainAddonOption {
	key: string;
	name: string;
	price: number;
}

export const load: PageServerLoad = async (event) => {
	const [res, addonsRes] = await Promise.all([
		apiFetch<ProductDetail>(event, `/api/v1/products/${encodeURIComponent(event.params.slug)}`),
		apiFetch<DomainAddonOption[]>(event, '/api/v1/domains/addons')
	]);
	const addons = addonsRes.data ?? [];

	if (res.status === 404 || res.error?.code === 'NOT_FOUND') {
		return { product: null, notFound: true, loadError: null, addons };
	}
	if (res.error || !res.data) {
		return { product: null, notFound: false, loadError: res.error?.message ?? 'error', addons };
	}
	return { product: res.data, notFound: false, loadError: null, addons };
};
