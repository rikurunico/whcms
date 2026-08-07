/**
 * FE-ORDER - client-side shopping cart, persisted to localStorage.
 *
 * The cart only stores *display estimates*; the backend recomputes every price
 * at `POST /api/v1/orders` (server is the source of truth, CONTRACTS.md §8).
 * Call `cart.init()` from an $effect (post-hydration) before rendering items to
 * avoid SSR/hydration mismatches - SSR always renders an empty cart.
 */
import { browser } from '$app/environment';

export type BillingCycle =
	'one_time' | 'monthly' | 'quarterly' | 'semiannually' | 'annually' | 'biennially';

/** Order item types per CONTRACTS.md §5 (order_items.item_type). */
export type CartItemType = 'product' | 'domain_register' | 'domain_transfer';

/** Display order for billing cycles. */
export const CYCLE_ORDER: readonly BillingCycle[] = [
	'one_time',
	'monthly',
	'quarterly',
	'semiannually',
	'annually',
	'biennially'
] as const;

/** Selected configurable option, kept for cart display only. */
export interface CartOptionLabel {
	name: string;
	value: string;
	/** Price delta for the chosen billing cycle (display estimate). */
	delta: number;
}

/** A customer-chosen dynamic spec (custom product). `amount` is a display estimate. */
export interface CartSpec {
	key: string;
	label?: string;
	/** Chosen quantity in the spec unit (ignored when `unlimited`). */
	qty: number;
	unlimited?: boolean;
	unit?: string;
	/** Price contribution for the chosen cycle (display estimate, whole IDR). */
	amount: number;
}

/** The customer's raw choice for one spec knob (shared config-page <-> configurator). */
export interface SpecChoice {
	qty: number;
	unlimited: boolean;
}

/** Minimal spec shape the estimate needs (subset of the product-detail SpecRow). */
export interface PricedSpec {
	key: string;
	label: string;
	unit: string;
	included_qty: number;
	default_qty: number;
	pricing: { cycle: BillingCycle; unit_price: number; unlimited_price: number }[];
}

/** specAmount is the IDR a chosen spec adds for a cycle (display estimate). */
export function specAmount(
	spec: PricedSpec,
	choice: SpecChoice | undefined,
	cycle: BillingCycle | null
): number {
	const c = choice ?? { qty: spec.default_qty, unlimited: false };
	if (!cycle) return 0;
	const p = spec.pricing.find((x) => x.cycle === cycle);
	if (!p) return 0;
	if (c.unlimited) return p.unlimited_price;
	const chargeable = Math.max(0, c.qty - spec.included_qty);
	return chargeable * p.unit_price;
}

/** resolveSpecSelections maps raw choices to priced CartSpecs for one cycle. */
export function resolveSpecSelections(
	specs: PricedSpec[],
	choices: Record<string, SpecChoice>,
	cycle: BillingCycle | null
): CartSpec[] {
	return specs.map((s) => {
		const c = choices[s.key] ?? { qty: s.default_qty, unlimited: false };
		return {
			key: s.key,
			label: s.label,
			qty: c.unlimited ? -1 : c.qty,
			unlimited: c.unlimited,
			unit: s.unit,
			amount: specAmount(s, c, cycle)
		};
	});
}

export interface CartItem {
	/** Local unique id (not sent to the API). */
	uid: string;
	item_type: CartItemType;
	product_id?: number;
	product_name?: string;
	product_slug?: string;
	domain?: string;
	cycle: BillingCycle;
	/** Base price for the chosen cycle (display estimate, whole IDR). */
	unit_price: number;
	setup_fee: number;
	/** option_id -> value_id. Converted to the API's array shape on checkout. */
	options?: Record<string, number>;
	/** Human-readable selected options (display only). */
	option_labels?: CartOptionLabel[];
	/** Customer-chosen dynamic specs (custom/configurable products). */
	specs?: CartSpec[];
	/** EPP/auth code for domain transfers (optional - can be provided later). */
	epp_code?: string;
	/** Selected registration/transfer term in years (domain items only). */
	domain_years?: number;
	/** Selected addon keys (id_protection, dns_management, email_forwarding). */
	domain_addons?: string[];
	/** Human-readable selected addons, for cart display only. */
	domain_addon_labels?: { name: string; price: number }[];
}

/** Coupon validated via POST /api/v1/coupons/validate, normalized for the cart. */
export interface AppliedCoupon {
	code: string;
	type: 'percentage' | 'fixed';
	value: number;
	/** Product ids the coupon applies to; null = applies to everything. */
	applies_to: number[] | null;
	recurring?: boolean;
}

/** One configurable-option selection in the API checkout payload. */
export interface OrderPayloadOption {
	option_id: number;
	value_id: number;
}

/** One dynamic-spec selection in the API checkout payload. */
export interface OrderPayloadSpec {
	key: string;
	qty: number;
	unlimited?: boolean;
}

/** Item shape sent to POST /api/v1/orders (CONTRACTS.md §8 checkout flow). */
export interface OrderPayloadItem {
	item_type: CartItemType;
	product_id?: number;
	domain?: string;
	cycle: BillingCycle;
	options?: OrderPayloadOption[];
	specs?: OrderPayloadSpec[];
	epp_code?: string;
	domain_years?: number;
	domain_addons?: string[];
}

export interface CartTotals {
	subtotal: number;
	discount: number;
	taxRate: number;
	tax: number;
	total: number;
}

/** Display-only tax preview config, sourced from GET /public/config's billing
 *  section (see $lib/server/captcha.ts's loadTaxConfig) - never hardcoded, so
 *  the cart never shows a tax line the server won't actually charge. */
export interface CartTaxConfig {
	enabled: boolean;
	rate: number;
	inclusive: boolean;
}

/** Fail-closed fallback if the real config couldn't be loaded: no tax shown,
 *  matching billing.tax_enabled's own server-side default of false. */
export const DEFAULT_TAX_CONFIG: CartTaxConfig = { enabled: false, rate: 11, inclusive: false };

const STORAGE_KEY = 'whcms.cart.v1';

interface PersistedCart {
	items?: CartItem[];
	coupon?: AppliedCoupon | null;
}

/** Sum of option price deltas for one item. */
export function optionsDelta(item: CartItem): number {
	return (item.option_labels ?? []).reduce((sum, o) => sum + (o.delta || 0), 0);
}

/** Sum of dynamic-spec amounts for one item (display estimate). */
export function specsDelta(item: CartItem): number {
	return (item.specs ?? []).reduce((sum, s) => sum + (s.amount || 0), 0);
}

/** First-term line total for one item (base + option deltas + spec amounts + setup fee). */
export function lineTotal(item: CartItem): number {
	return item.unit_price + optionsDelta(item) + specsDelta(item) + item.setup_fee;
}

function couponEligible(item: CartItem, coupon: AppliedCoupon): boolean {
	if (coupon.applies_to === null) return true;
	return item.product_id !== undefined && coupon.applies_to.includes(item.product_id);
}

/**
 * Client-side totals preview. Display only - the backend recalculates
 * discount/tax/total when the order and invoice are created.
 */
export function cartTotals(
	items: CartItem[],
	coupon: AppliedCoupon | null,
	tax: CartTaxConfig = DEFAULT_TAX_CONFIG
): CartTotals {
	const subtotal = items.reduce((sum, i) => sum + lineTotal(i), 0);

	let discount = 0;
	if (coupon && items.length > 0) {
		// Discount base excludes setup fees (matches typical WHMCS behaviour).
		const base = items
			.filter((i) => couponEligible(i, coupon))
			.reduce((sum, i) => sum + i.unit_price + optionsDelta(i), 0);
		discount =
			coupon.type === 'percentage'
				? Math.floor((base * coupon.value) / 100)
				: Math.min(coupon.value, base);
	}

	const taxable = Math.max(0, subtotal - discount);
	if (!tax.enabled || tax.rate <= 0) {
		return { subtotal, discount, taxRate: 0, tax: 0, total: taxable };
	}
	// Mirrors domain.CalcTax's exclusive/inclusive formulas server-side.
	const taxAmount = tax.inclusive
		? Math.round((taxable * tax.rate) / (100 + tax.rate))
		: Math.round((taxable * tax.rate) / 100);
	const total = tax.inclusive ? taxable : taxable + taxAmount;
	return { subtotal, discount, taxRate: tax.rate, tax: taxAmount, total };
}

/** Loose domain-name validation (label.label.tld, lowercase). */
export function isValidDomain(name: string): boolean {
	if (name.length < 4 || name.length > 253) return false;
	return /^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(name);
}

function newUid(): string {
	return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

class CartStore {
	items = $state<CartItem[]>([]);
	coupon = $state<AppliedCoupon | null>(null);
	/** True once localStorage has been read (client only). */
	ready = $state(false);

	/** Idempotent; call from an $effect so it runs after hydration. */
	init(): void {
		if (!browser || this.ready) return;
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			if (raw) {
				const parsed = JSON.parse(raw) as PersistedCart;
				if (Array.isArray(parsed.items)) this.items = parsed.items;
				this.coupon = parsed.coupon ?? null;
			}
		} catch {
			// Corrupted persisted state - start with an empty cart.
		}
		this.ready = true;
	}

	get count(): number {
		return this.items.length;
	}

	add(item: Omit<CartItem, 'uid'>): CartItem {
		const withUid: CartItem = { ...item, uid: newUid() };
		this.items = [...this.items, withUid];
		this.#persist();
		return withUid;
	}

	remove(uid: string): void {
		this.items = this.items.filter((i) => i.uid !== uid);
		if (this.items.length === 0) this.coupon = null;
		this.#persist();
	}

	setCoupon(coupon: AppliedCoupon | null): void {
		this.coupon = coupon;
		this.#persist();
	}

	clear(): void {
		this.items = [];
		this.coupon = null;
		this.#persist();
	}

	/** True when the given domain name is already in the cart as a domain item. */
	hasDomain(name: string): boolean {
		return this.items.some(
			(i) =>
				i.domain === name &&
				(i.item_type === 'domain_register' || i.item_type === 'domain_transfer')
		);
	}

	/** Items in the shape expected by POST /api/v1/orders. */
	toOrderPayloadItems(): OrderPayloadItem[] {
		return this.items.map((i) => {
			const out: OrderPayloadItem = { item_type: i.item_type, cycle: i.cycle };
			if (i.product_id !== undefined) out.product_id = i.product_id;
			if (i.domain) out.domain = i.domain;
			// Convert the option_id->value_id map to the API's array shape.
			if (i.options && Object.keys(i.options).length > 0) {
				out.options = Object.entries(i.options).map(([oid, vid]) => ({
					option_id: Number(oid),
					value_id: vid
				}));
			}
			if (i.specs && i.specs.length > 0) {
				out.specs = i.specs.map((s) => ({
					key: s.key,
					qty: s.unlimited ? 0 : s.qty,
					unlimited: s.unlimited || undefined
				}));
			}
			if (i.epp_code) out.epp_code = i.epp_code;
			if (i.domain_years) out.domain_years = i.domain_years;
			if (i.domain_addons && i.domain_addons.length > 0) out.domain_addons = i.domain_addons;
			return out;
		});
	}

	#persist(): void {
		if (!browser) return;
		try {
			const payload: PersistedCart = { items: this.items, coupon: this.coupon };
			localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
		} catch {
			// Storage full/unavailable - cart still works in-memory.
		}
	}
}

/** Global cart store (module-level singleton, localStorage-backed). */
export const cart = new CartStore();
