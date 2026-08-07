import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.TLDPricing JSON tags. */
export interface TLDPricingRow {
	id: number;
	tld: string;
	registrar_id: number;
	active: boolean;
	min_years: number;
	max_years: number;
	register_prices: Record<string, number>;
	renew_prices: Record<string, number>;
	transfer_price: number;
	restore_price: number;
	created_at: string;
	updated_at: string;
}

/** domain.PremiumLengthPricing JSON tags. */
export interface PremiumLengthPricingRow {
	id: number;
	tld: string;
	char_length: number;
	price: number;
	created_at: string;
	updated_at: string;
}

/** domain.PremiumDomainPricing JSON tags. */
export interface PremiumPricingRow {
	id: number;
	domain_name: string;
	register_price: number;
	renew_price: number;
	transfer_price: number;
	created_at: string;
	updated_at: string;
}

/** Minimal registrar shape for the TLD form's registrar picker. */
export interface RegistrarOption {
	id: number;
	name: string;
	active: boolean;
}

export const load: PageServerLoad = async (event) => {
	const [tldRes, premiumRes, lengthRes, registrarsRes] = await Promise.all([
		apiFetch<TLDPricingRow[]>(event, '/api/v1/admin/tld-pricing'),
		apiFetch<PremiumPricingRow[]>(event, '/api/v1/admin/premium-domain-pricing'),
		apiFetch<PremiumLengthPricingRow[]>(event, '/api/v1/admin/premium-length-pricing'),
		apiFetch<RegistrarOption[]>(event, '/api/v1/admin/registrars')
	]);

	return {
		tldPricing: tldRes.data ?? [],
		tldListError: tldRes.error ? tldRes.error.message : null,
		premiumPricing: premiumRes.data ?? [],
		premiumListError: premiumRes.error ? premiumRes.error.message : null,
		lengthPricing: lengthRes.data ?? [],
		lengthListError: lengthRes.error ? lengthRes.error.message : null,
		registrars: registrarsRes.data ?? []
	};
};

/** Reads register_price_1..10 / renew_price_1..10 into a year->price map,
 *  skipping any year left blank (min/max_years controls which years are
 *  actually sellable - a blank entry means "no override for this year", not
 *  "free"). A year explicitly set to 0 IS kept - that's how a free-domain
 *  promo (e.g. year-1 register price 0) is expressed. */
function parseYearPrices(form: FormData, prefix: string): Record<string, number> {
	const out: Record<string, number> = {};
	for (let year = 1; year <= 10; year++) {
		const raw = form.get(`${prefix}_${year}`);
		if (raw === null) continue;
		const s = String(raw).trim();
		if (s === '') continue;
		const n = Math.round(Number(s));
		if (!Number.isFinite(n) || n < 0) continue;
		out[String(year)] = n;
	}
	return out;
}

interface TLDPricingBody {
	tld: string;
	registrar_id: number;
	active: boolean;
	min_years: number;
	max_years: number;
	register_prices: Record<string, number>;
	renew_prices: Record<string, number>;
	transfer_price: number;
	restore_price: number;
}

interface ParsedTld {
	body: TLDPricingBody;
	errorMessage: string | null;
}

function parseTldForm(form: FormData): ParsedTld {
	const tld = String(form.get('tld') ?? '')
		.trim()
		.toLowerCase()
		.replace(/^\./, '');
	const registrarId = Math.round(Number(form.get('registrar_id') ?? 0) || 0);
	const active = form.get('active') !== null;
	const minYears = Math.round(Number(form.get('min_years') ?? 1) || 1);
	const maxYears = Math.round(Number(form.get('max_years') ?? 1) || 1);
	const transferPrice = Math.max(0, Math.round(Number(form.get('transfer_price') ?? 0) || 0));
	const restorePrice = Math.max(0, Math.round(Number(form.get('restore_price') ?? 0) || 0));

	let errorMessage: string | null = null;
	if (!tld) errorMessage = 'TLD is required.';
	else if (!registrarId) errorMessage = 'Registrar is required.';
	else if (minYears < 1 || minYears > 10 || maxYears < 1 || maxYears > 10) {
		errorMessage = 'Min/max years must be between 1 and 10.';
	} else if (minYears > maxYears) {
		errorMessage = 'Min years must be less than or equal to max years.';
	}

	return {
		body: {
			tld,
			registrar_id: registrarId,
			active,
			min_years: minYears,
			max_years: maxYears,
			register_prices: parseYearPrices(form, 'register_price'),
			renew_prices: parseYearPrices(form, 'renew_price'),
			transfer_price: transferPrice,
			restore_price: restorePrice
		},
		errorMessage
	};
}

interface PremiumLengthPricingBody {
	tld: string;
	char_length: number;
	price: number;
}

interface ParsedLengthTier {
	body: PremiumLengthPricingBody;
	errorMessage: string | null;
}

function parseLengthTierForm(form: FormData): ParsedLengthTier {
	const tld = String(form.get('tld') ?? '')
		.trim()
		.toLowerCase()
		.replace(/^\./, '');
	const charLength = Math.round(Number(form.get('char_length') ?? 0) || 0);
	const price = Math.max(0, Math.round(Number(form.get('price') ?? 0) || 0));

	let errorMessage: string | null = null;
	if (!tld) errorMessage = 'TLD is required.';
	else if (charLength < 1) errorMessage = 'Character length must be at least 1.';

	return {
		body: { tld, char_length: charLength, price },
		errorMessage
	};
}

interface PremiumPricingBody {
	domain_name: string;
	register_price: number;
	renew_price: number;
	transfer_price: number;
}

interface ParsedPremium {
	body: PremiumPricingBody;
	errorMessage: string | null;
}

function parsePremiumForm(form: FormData): ParsedPremium {
	const domainName = String(form.get('domain_name') ?? '')
		.trim()
		.toLowerCase();
	const registerPrice = Math.max(0, Math.round(Number(form.get('register_price') ?? 0) || 0));
	const renewPrice = Math.max(0, Math.round(Number(form.get('renew_price') ?? 0) || 0));
	const transferPrice = Math.max(0, Math.round(Number(form.get('transfer_price') ?? 0) || 0));

	let errorMessage: string | null = null;
	if (!domainName) errorMessage = 'Domain name is required.';

	return {
		body: {
			domain_name: domainName,
			register_price: registerPrice,
			renew_price: renewPrice,
			transfer_price: transferPrice
		},
		errorMessage
	};
}

export const actions: Actions = {
	createTld: async (event) => {
		const form = await event.request.formData();
		const { body, errorMessage } = parseTldForm(form);
		if (errorMessage) return fail(400, { op: 'tld-save', errorMessage });
		const res = await apiFetch(event, '/api/v1/admin/tld-pricing', { method: 'POST', body });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'tld-save',
				errorMessage: res.error.message
			});
		}
		return { op: 'tld-save', success: true };
	},

	updateTld: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const { body, errorMessage } = parseTldForm(form);
		if (!id) return fail(400, { op: 'tld-save', errorMessage: 'Invalid TLD pricing id.' });
		if (errorMessage) return fail(400, { op: 'tld-save', errorMessage });
		const res = await apiFetch(event, `/api/v1/admin/tld-pricing/${id}`, { method: 'PUT', body });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'tld-save',
				errorMessage: res.error.message
			});
		}
		return { op: 'tld-save', success: true };
	},

	importTld: async (event) => {
		const form = await event.request.formData();
		const registrarId = Math.round(Number(form.get('registrar_id') ?? 0) || 0);
		const markupPercent = Math.max(0, Number(form.get('markup_percent') ?? 0) || 0);
		const tlds = form
			.getAll('tlds')
			.map((v) => String(v).trim().toLowerCase())
			.filter(Boolean);

		if (!registrarId)
			return fail(400, { op: 'tld-import', errorMessage: 'No registrar configured.' });
		if (tlds.length === 0) {
			return fail(400, { op: 'tld-import', errorMessage: 'Select at least one TLD to import.' });
		}

		const res = await apiFetch<{ imported: string[]; skipped: string[] }>(
			event,
			'/api/v1/admin/tld-pricing/import',
			{
				method: 'POST',
				body: { registrar_id: registrarId, tlds, markup_percent: markupPercent }
			}
		);
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'tld-import',
				errorMessage: res.error.message
			});
		}
		return {
			op: 'tld-import',
			success: true,
			imported: res.data?.imported ?? [],
			skipped: res.data?.skipped ?? []
		};
	},

	deleteTld: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { op: 'tld-delete', errorMessage: 'Invalid TLD pricing id.' });
		const res = await apiFetch(event, `/api/v1/admin/tld-pricing/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'tld-delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'tld-delete', success: true };
	},

	createPremium: async (event) => {
		const form = await event.request.formData();
		const { body, errorMessage } = parsePremiumForm(form);
		if (errorMessage) return fail(400, { op: 'premium-save', errorMessage });
		const res = await apiFetch(event, '/api/v1/admin/premium-domain-pricing', {
			method: 'POST',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'premium-save',
				errorMessage: res.error.message
			});
		}
		return { op: 'premium-save', success: true };
	},

	updatePremium: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const { body, errorMessage } = parsePremiumForm(form);
		if (!id) return fail(400, { op: 'premium-save', errorMessage: 'Invalid premium pricing id.' });
		if (errorMessage) return fail(400, { op: 'premium-save', errorMessage });
		const res = await apiFetch(event, `/api/v1/admin/premium-domain-pricing/${id}`, {
			method: 'PUT',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'premium-save',
				errorMessage: res.error.message
			});
		}
		return { op: 'premium-save', success: true };
	},

	deletePremium: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id)
			return fail(400, { op: 'premium-delete', errorMessage: 'Invalid premium pricing id.' });
		const res = await apiFetch(event, `/api/v1/admin/premium-domain-pricing/${id}`, {
			method: 'DELETE'
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'premium-delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'premium-delete', success: true };
	},

	createLengthTier: async (event) => {
		const form = await event.request.formData();
		const { body, errorMessage } = parseLengthTierForm(form);
		if (errorMessage) return fail(400, { op: 'length-save', errorMessage });
		const res = await apiFetch(event, '/api/v1/admin/premium-length-pricing', {
			method: 'POST',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'length-save',
				errorMessage: res.error.message
			});
		}
		return { op: 'length-save', success: true };
	},

	updateLengthTier: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const { body, errorMessage } = parseLengthTierForm(form);
		if (!id) return fail(400, { op: 'length-save', errorMessage: 'Invalid length-tier id.' });
		if (errorMessage) return fail(400, { op: 'length-save', errorMessage });
		const res = await apiFetch(event, `/api/v1/admin/premium-length-pricing/${id}`, {
			method: 'PUT',
			body
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'length-save',
				errorMessage: res.error.message
			});
		}
		return { op: 'length-save', success: true };
	},

	deleteLengthTier: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return fail(400, { op: 'length-delete', errorMessage: 'Invalid length-tier id.' });
		const res = await apiFetch(event, `/api/v1/admin/premium-length-pricing/${id}`, {
			method: 'DELETE'
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				op: 'length-delete',
				errorMessage: res.error.message
			});
		}
		return { op: 'length-delete', success: true };
	}
};
