import { apiFetch } from '$lib/server/api';
import { error as kitError, fail, redirect } from '@sveltejs/kit';
import type { RequestEvent } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
	CYCLES,
	parseProductForm,
	type EmailTemplateRow,
	type OptionGroupRow,
	type OptionRow,
	type OptionValueRow,
	type PricingRow,
	type ProductGroupRow,
	type ProductRow,
	type ServerGroupRow
} from '../catalog';
import { syncProductPricing } from '../pricing-sync.server';

/** One spec + its per-cycle pricing (GET /admin/products/:id/specs). */
export interface SpecPriceRow {
	cycle: string;
	unit_price: number;
	unlimited_price: number;
	currency: string;
}
export interface SpecRow {
	id: number;
	product_id: number;
	key: string;
	label: string;
	provision_key: string;
	unit: string;
	included_qty: number;
	min_qty: number;
	max_qty: number;
	step_qty: number;
	default_qty: number;
	allow_unlimited: boolean;
	sort: number;
}
export interface SpecWithPricingRow {
	spec: SpecRow;
	pricing: SpecPriceRow[];
}

/** GET /admin/config-options response element - catalog.OptionGroupTree. */
interface ApiOptionGroupTree {
	group: { id: number; name: string; description: string };
	options: Array<{
		option: { id: number; group_id: number; name: string; sort: number };
		values: Array<{
			id: number;
			option_id: number;
			name: string;
			price_deltas: Record<string, number> | null;
			sort: number;
		}>;
	}>;
}

async function loadOptionTree(
	event: RequestEvent
): Promise<{ optionGroups: OptionGroupRow[]; optionsError: string | null }> {
	const res = await apiFetch<ApiOptionGroupTree[]>(event, '/api/v1/admin/config-options');
	if (res.error) {
		return { optionGroups: [], optionsError: res.error.message };
	}

	const optionGroups: OptionGroupRow[] = (res.data ?? []).map((g): OptionGroupRow => ({
		id: g.group.id,
		name: g.group.name,
		description: g.group.description,
		options: g.options.map((o): OptionRow => ({
			id: o.option.id,
			group_id: o.option.group_id,
			name: o.option.name,
			sort: o.option.sort,
			values: o.values.map((v): OptionValueRow => ({
				id: v.id,
				option_id: v.option_id,
				name: v.name,
				price_deltas: v.price_deltas ?? {},
				sort: v.sort
			}))
		}))
	}));

	return { optionGroups, optionsError: null };
}

export const load: PageServerLoad = async (event) => {
	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) kitError(404, 'Not found');

	const [productRes, pricingRes, groupsRes, serverGroupsRes, templatesRes, optionTree, specsRes] =
		await Promise.all([
			apiFetch<ProductRow>(event, `/api/v1/admin/products/${id}`),
			apiFetch<PricingRow[]>(event, `/api/v1/admin/products/${id}/pricing`),
			apiFetch<ProductGroupRow[]>(event, '/api/v1/admin/product-groups'),
			apiFetch<ServerGroupRow[]>(event, '/api/v1/admin/server-groups'),
			apiFetch<EmailTemplateRow[]>(event, '/api/v1/admin/email-templates'),
			loadOptionTree(event),
			apiFetch<SpecWithPricingRow[]>(event, `/api/v1/admin/products/${id}/specs`)
		]);

	return {
		productId: id,
		product: productRes.data,
		loadError: productRes.error ? productRes.error.message : null,
		notFound: productRes.status === 404,
		pricing: pricingRes.data ?? [],
		groups: groupsRes.data ?? [],
		serverGroups: serverGroupsRes.data ?? [],
		templateKeys: [...new Set((templatesRes.data ?? []).map((tpl) => tpl.key))],
		optionGroups: optionTree.optionGroups,
		optionsError: optionTree.optionsError,
		// A freshly-created spec has no pricing rows yet, and the API returns
		// `pricing: null` (not `[]`) for that case - default it so
		// SpecsEditor's `row.pricing.find(...)` never sees a null.
		specs: (specsRes.data ?? []).map((row) => ({ ...row, pricing: row.pricing ?? [] })),
		specsError: specsRes.error ? specsRes.error.message : null,
		created: event.url.searchParams.get('created') === '1',
		duplicated: event.url.searchParams.get('duplicated') === '1',
		pricingError: event.url.searchParams.get('pricing_error') === '1'
	};
};

/** Build a SpecInput body from the spec form fields. */
function parseSpecForm(form: FormData) {
	return {
		key: String(form.get('key') ?? '').trim(),
		label: String(form.get('label') ?? '').trim(),
		provision_key: String(form.get('provision_key') ?? '').trim(),
		unit: String(form.get('unit') ?? 'count').trim(),
		included_qty: Number(form.get('included_qty') ?? 0) || 0,
		min_qty: Number(form.get('min_qty') ?? 0) || 0,
		max_qty: Number(form.get('max_qty') ?? 0) || 0,
		step_qty: Number(form.get('step_qty') ?? 1) || 1,
		default_qty: Number(form.get('default_qty') ?? 0) || 0,
		allow_unlimited: form.get('allow_unlimited') === 'on' || form.get('allow_unlimited') === 'true',
		sort: Number(form.get('sort') ?? 0) || 0
	};
}

/** Parse per-cycle delta inputs (delta_<cycle>) into a price_deltas object. */
function parseDeltas(form: FormData): Record<string, number> {
	const deltas: Record<string, number> = {};
	for (const cycle of CYCLES) {
		const raw = String(form.get(`delta_${cycle}`) ?? '').trim();
		if (raw === '') continue;
		const n = Number(raw);
		if (Number.isFinite(n) && n !== 0) deltas[cycle] = Math.round(n);
	}
	return deltas;
}

function actionFail(scope: string, status: number, message?: string, key?: string) {
	return fail(status >= 400 ? status : 400, {
		scope,
		errorMessage: message,
		errorKey: message ? undefined : key
	});
}

/** Map backend VALIDATION error details ([{field, message}]) to a per-field error map. */
function fieldErrors(details: unknown[]): Record<string, string> {
	const out: Record<string, string> = {};
	for (const d of details) {
		if (d && typeof d === 'object') {
			const rec = d as Record<string, unknown>;
			const field = typeof rec.field === 'string' ? rec.field : undefined;
			const message = typeof rec.message === 'string' ? rec.message : undefined;
			if (field && message) out[field] = message;
		}
	}
	return out;
}

function specActionFail(
	status: number,
	message?: string,
	details: unknown[] = [],
	values?: ReturnType<typeof parseSpecForm>
) {
	return fail(status >= 400 ? status : 400, {
		scope: 'specs',
		errorMessage: message,
		fields: fieldErrors(details),
		values
	});
}

export const actions: Actions = {
	save: async (event) => {
		const id = Number(event.params.id);
		const form = await event.request.formData();
		const { product, pricing, errorKey } = parseProductForm(form);
		if (errorKey) return fail(400, { scope: 'product', errorKey });

		const saved = await apiFetch(event, `/api/v1/admin/products/${id}`, {
			method: 'PATCH',
			body: product
		});
		if (saved.error) {
			return actionFail('product', saved.status, saved.error.message);
		}

		const currentPricing = await apiFetch<PricingRow[]>(
			event,
			`/api/v1/admin/products/${id}/pricing`
		);
		const pricingError = await syncProductPricing(
			event,
			id,
			pricing,
			(currentPricing.data ?? []).map((p) => p.cycle)
		);
		if (pricingError) {
			return fail(400, {
				scope: 'product',
				errorKey: 'adcatalog.products.pricingFailed'
			});
		}

		return { scope: 'product', success: true };
	},

	delete: async (event) => {
		const id = Number(event.params.id);
		const res = await apiFetch(event, `/api/v1/admin/products/${id}`, { method: 'DELETE' });
		if (res.error) {
			return actionFail('delete', res.status, res.error.message);
		}
		redirect(303, '/admin/products?deleted=1');
	},

	duplicate: async (event) => {
		const id = Number(event.params.id);
		const res = await apiFetch<ProductRow>(event, `/api/v1/admin/products/${id}/duplicate`, {
			method: 'POST'
		});
		if (res.error || !res.data) {
			return actionFail('product', res.status, res.error?.message);
		}
		redirect(303, `/admin/products/${res.data.id}?duplicated=1`);
	},

	// configurable option groups
	createOptionGroup: async (event) => {
		const form = await event.request.formData();
		const name = String(form.get('name') ?? '').trim();
		if (!name) return actionFail('options', 400, undefined, 'adcatalog.options.fillRequired');
		const res = await apiFetch(event, '/api/v1/admin/config-options/groups', {
			method: 'POST',
			body: { name, description: String(form.get('description') ?? '').trim() }
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true };
	},

	updateOptionGroup: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const name = String(form.get('name') ?? '').trim();
		if (!id || !name)
			return actionFail('options', 400, undefined, 'adcatalog.options.fillRequired');
		const res = await apiFetch(event, `/api/v1/admin/config-options/groups/${id}`, {
			method: 'PATCH',
			body: { name, description: String(form.get('description') ?? '').trim() }
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true };
	},

	deleteOptionGroup: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return actionFail('options', 400, undefined, 'adcatalog.options.deleteFailed');
		const res = await apiFetch(event, `/api/v1/admin/config-options/groups/${id}`, {
			method: 'DELETE'
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true, deleted: true };
	},

	// configurable options
	createOption: async (event) => {
		const form = await event.request.formData();
		const groupId = Number(form.get('group_id'));
		const name = String(form.get('name') ?? '').trim();
		if (!groupId || !name) {
			return actionFail('options', 400, undefined, 'adcatalog.options.fillRequired');
		}
		const res = await apiFetch(event, `/api/v1/admin/config-options/groups/${groupId}/options`, {
			method: 'POST',
			body: { name, sort: Number(form.get('sort') ?? 0) || 0 }
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true };
	},

	updateOption: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const name = String(form.get('name') ?? '').trim();
		if (!id || !name)
			return actionFail('options', 400, undefined, 'adcatalog.options.fillRequired');
		const res = await apiFetch(event, `/api/v1/admin/config-options/options/${id}`, {
			method: 'PATCH',
			body: { name, sort: Number(form.get('sort') ?? 0) || 0 }
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true };
	},

	deleteOption: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return actionFail('options', 400, undefined, 'adcatalog.options.deleteFailed');
		const res = await apiFetch(event, `/api/v1/admin/config-options/options/${id}`, {
			method: 'DELETE'
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true, deleted: true };
	},

	// configurable option values
	createValue: async (event) => {
		const form = await event.request.formData();
		const optionId = Number(form.get('option_id'));
		const name = String(form.get('name') ?? '').trim();
		if (!optionId || !name) {
			return actionFail('options', 400, undefined, 'adcatalog.options.fillRequired');
		}
		const res = await apiFetch(event, `/api/v1/admin/config-options/options/${optionId}/values`, {
			method: 'POST',
			body: {
				name,
				sort: Number(form.get('sort') ?? 0) || 0,
				price_deltas: parseDeltas(form)
			}
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true };
	},

	updateValue: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		const name = String(form.get('name') ?? '').trim();
		if (!id || !name)
			return actionFail('options', 400, undefined, 'adcatalog.options.fillRequired');
		const res = await apiFetch(event, `/api/v1/admin/config-options/values/${id}`, {
			method: 'PATCH',
			body: {
				name,
				sort: Number(form.get('sort') ?? 0) || 0,
				price_deltas: parseDeltas(form)
			}
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true };
	},

	deleteValue: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id'));
		if (!id) return actionFail('options', 400, undefined, 'adcatalog.options.deleteFailed');
		const res = await apiFetch(event, `/api/v1/admin/config-options/values/${id}`, {
			method: 'DELETE'
		});
		if (res.error) return actionFail('options', res.status, res.error.message);
		return { scope: 'options', success: true, deleted: true };
	},

	// dynamic product specs
	createSpec: async (event) => {
		const id = Number(event.params.id);
		const form = await event.request.formData();
		const body = parseSpecForm(form);
		if (!body.key || !body.provision_key) {
			return fail(400, {
				scope: 'specs',
				errorKey: 'adcatalog.specs.fillRequired',
				values: body
			});
		}
		const res = await apiFetch(event, `/api/v1/admin/products/${id}/specs`, {
			method: 'POST',
			body
		});
		if (res.error) return specActionFail(res.status, res.error.message, res.error.details, body);
		return { scope: 'specs', success: true };
	},

	updateSpec: async (event) => {
		const form = await event.request.formData();
		const specId = Number(form.get('spec_id'));
		if (!specId) return actionFail('specs', 400, undefined, 'adcatalog.specs.fillRequired');
		const res = await apiFetch(event, `/api/v1/admin/product-specs/${specId}`, {
			method: 'PATCH',
			body: parseSpecForm(form)
		});
		if (res.error) return specActionFail(res.status, res.error.message, res.error.details);
		return { scope: 'specs', success: true };
	},

	deleteSpec: async (event) => {
		const form = await event.request.formData();
		const specId = Number(form.get('spec_id'));
		if (!specId) return actionFail('specs', 400, undefined, 'adcatalog.specs.deleteFailed');
		const res = await apiFetch(event, `/api/v1/admin/product-specs/${specId}`, {
			method: 'DELETE'
		});
		if (res.error) return actionFail('specs', res.status, res.error.message);
		return { scope: 'specs', success: true, deleted: true };
	},

	saveSpecPricing: async (event) => {
		const form = await event.request.formData();
		const specId = Number(form.get('spec_id'));
		const cycle = String(form.get('cycle') ?? '').trim();
		if (!specId || !cycle)
			return actionFail('specs', 400, undefined, 'adcatalog.specs.fillRequired');
		const res = await apiFetch(event, `/api/v1/admin/product-specs/${specId}/pricing`, {
			method: 'PUT',
			body: {
				cycle,
				unit_price: Number(form.get('unit_price') ?? 0) || 0,
				unlimited_price: Number(form.get('unlimited_price') ?? 0) || 0
			}
		});
		if (res.error) return actionFail('specs', res.status, res.error.message);
		return { scope: 'specs', success: true };
	},

	deleteSpecPricing: async (event) => {
		const form = await event.request.formData();
		const specId = Number(form.get('spec_id'));
		const cycle = String(form.get('cycle') ?? '').trim();
		if (!specId || !cycle)
			return actionFail('specs', 400, undefined, 'adcatalog.specs.deleteFailed');
		const res = await apiFetch(event, `/api/v1/admin/product-specs/${specId}/pricing/${cycle}`, {
			method: 'DELETE'
		});
		if (res.error) return actionFail('specs', res.status, res.error.message);
		return { scope: 'specs', success: true, deleted: true };
	}
};
