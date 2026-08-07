/** Shared types + form parsing for the admin product catalog pages (FE-ADMIN-CATALOG). */

export const CYCLES = [
	'one_time',
	'monthly',
	'quarterly',
	'semiannually',
	'annually',
	'biennially'
] as const;
export type Cycle = (typeof CYCLES)[number];

export const PRODUCT_TYPES = ['shared_hosting', 'reseller_hosting', 'domain', 'other'] as const;
export type ProductType = (typeof PRODUCT_TYPES)[number];

export const MODULES = ['none', 'cpanel', 'directadmin'] as const;
export const AUTO_SETUPS = ['on_payment', 'on_order', 'manual'] as const;

/** domain.Product JSON tags. */
export interface ProductRow {
	id: number;
	group_id: number;
	name: string;
	slug: string;
	description: string;
	type: string;
	module: string;
	server_group_id: number | null;
	package_name: string;
	auto_setup: string;
	configurable: boolean;
	shell_access: boolean;
	cgi_access: boolean;
	feature_list: string;
	template_package: string;
	stock_enabled: boolean;
	stock_qty: number;
	hidden: boolean;
	sort: number;
	welcome_email_template: string;
	created_at: string;
	updated_at: string;
}

/** domain.ProductGroup JSON tags. */
export interface ProductGroupRow {
	id: number;
	name: string;
	slug: string;
	sort: number;
	hidden: boolean;
	created_at: string;
	updated_at: string;
}

/** domain.ProductPricing JSON tags. */
export interface PricingRow {
	id?: number;
	product_id?: number;
	cycle: string;
	price: number;
	setup_fee: number;
	currency?: string;
}

/** domain.ServerGroup JSON tags (consumed read-only for the module tab select). */
export interface ServerGroupRow {
	id: number;
	name: string;
	strategy?: string;
}

/** domain.EmailTemplate JSON tags (consumed read-only for the welcome template select). */
export interface EmailTemplateRow {
	id: number;
	key: string;
	locale?: string;
	subject?: string;
}

/** domain.ConfigurableOptionValue with parsed price_deltas. */
export interface OptionValueRow {
	id: number;
	option_id: number;
	name: string;
	price_deltas: Record<string, number> | null;
	sort: number;
}

/** domain.ConfigurableOption + nested values. */
export interface OptionRow {
	id: number;
	group_id: number;
	name: string;
	sort: number;
	values: OptionValueRow[];
}

/** domain.ConfigurableOptionGroup + nested options. */
export interface OptionGroupRow {
	id: number;
	name: string;
	description: string;
	options: OptionRow[];
}

export interface ProductPayload {
	name: string;
	slug: string;
	description: string;
	type: string;
	group_id: number;
	module: string;
	server_group_id: number | null;
	package_name: string;
	auto_setup: string;
	configurable: boolean;
	shell_access: boolean;
	cgi_access: boolean;
	feature_list: string;
	template_package: string;
	stock_enabled: boolean;
	stock_qty: number;
	hidden: boolean;
	sort: number;
	welcome_email_template: string;
}

export interface PricingPayload {
	cycle: Cycle;
	price: number;
	setup_fee: number;
}

export interface ParsedProductForm {
	product: ProductPayload;
	pricing: PricingPayload[];
	/** i18n key of the first validation problem, if any. */
	errorKey: string | null;
}

function num(form: FormData, name: string, fallback = 0): number {
	const raw = String(form.get(name) ?? '').trim();
	if (raw === '') return fallback;
	const n = Number(raw);
	return Number.isFinite(n) ? n : fallback;
}

function bool(form: FormData, name: string): boolean {
	return form.get(name) !== null;
}

/** Parse + validate the multi-tab product form (shared by create and edit actions). */
export function parseProductForm(form: FormData): ParsedProductForm {
	const name = String(form.get('name') ?? '').trim();
	const slug = String(form.get('slug') ?? '')
		.trim()
		.toLowerCase();
	const type = String(form.get('type') ?? '').trim();
	const groupId = num(form, 'group_id', 0);
	const serverGroupId = num(form, 'server_group_id', 0);

	const product: ProductPayload = {
		name,
		slug,
		description: String(form.get('description') ?? '').trim(),
		type,
		group_id: groupId,
		module: String(form.get('module') ?? 'none').trim() || 'none',
		server_group_id: serverGroupId > 0 ? serverGroupId : null,
		package_name: String(form.get('package_name') ?? '').trim(),
		auto_setup: String(form.get('auto_setup') ?? 'on_payment').trim() || 'on_payment',
		configurable: bool(form, 'configurable'),
		shell_access: bool(form, 'shell_access'),
		cgi_access: bool(form, 'cgi_access'),
		feature_list: String(form.get('feature_list') ?? '').trim(),
		template_package: String(form.get('template_package') ?? '').trim(),
		stock_enabled: bool(form, 'stock_enabled'),
		stock_qty: Math.max(0, num(form, 'stock_qty', 0)),
		hidden: bool(form, 'hidden'),
		sort: num(form, 'sort', 0),
		welcome_email_template: String(form.get('welcome_email_template') ?? '').trim()
	};

	const pricing: PricingPayload[] = [];
	let invalidPrice = false;
	for (const cycle of CYCLES) {
		if (!bool(form, `cycle_${cycle}`)) continue;
		const price = num(form, `price_${cycle}`, 0);
		const setupFee = num(form, `setup_${cycle}`, 0);
		if (price < 0 || setupFee < 0) invalidPrice = true;
		pricing.push({ cycle, price: Math.round(price), setup_fee: Math.round(setupFee) });
	}

	let errorKey: string | null = null;
	if (!name || !slug || !type || groupId <= 0) errorKey = 'adcatalog.products.fillRequired';
	else if (pricing.length === 0) errorKey = 'adcatalog.products.noCycles';
	else if (invalidPrice) errorKey = 'adcatalog.products.invalidPrice';

	return { product, pricing, errorKey };
}

/**
 * Which ProductForm tab holds the fix for a given errorKey - Details/Module/
 * Pricing all share one physical <form>, so a validation problem raised on
 * submit doesn't otherwise tell the user which (possibly hidden) tab to look
 * at. Used to auto-switch the active tab alongside the translated message.
 */
export const TAB_FOR_ERROR_KEY: Record<string, string> = {
	'adcatalog.products.fillRequired': 'details',
	'adcatalog.products.noCycles': 'pricing',
	'adcatalog.products.invalidPrice': 'pricing'
};
