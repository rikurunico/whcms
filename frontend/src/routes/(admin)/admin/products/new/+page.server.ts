import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
	parseProductForm,
	type EmailTemplateRow,
	type ProductGroupRow,
	type ProductRow,
	type ServerGroupRow
} from '../catalog';
import { syncProductPricing } from '../pricing-sync.server';

export const load: PageServerLoad = async (event) => {
	const [groupsRes, serverGroupsRes, templatesRes] = await Promise.all([
		apiFetch<ProductGroupRow[]>(event, '/api/v1/admin/product-groups'),
		apiFetch<ServerGroupRow[]>(event, '/api/v1/admin/server-groups'),
		apiFetch<EmailTemplateRow[]>(event, '/api/v1/admin/email-templates')
	]);

	const templateKeys = [...new Set((templatesRes.data ?? []).map((tpl) => tpl.key))];

	return {
		groups: groupsRes.data ?? [],
		groupsError: groupsRes.error ? groupsRes.error.message : null,
		serverGroups: serverGroupsRes.data ?? [],
		templateKeys
	};
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const { product, pricing, errorKey } = parseProductForm(form);
		if (errorKey) return fail(400, { errorKey });

		const created = await apiFetch<ProductRow>(event, '/api/v1/admin/products', {
			method: 'POST',
			body: product
		});
		if (created.error || !created.data) {
			return fail(created.status >= 400 ? created.status : 400, {
				errorMessage: created.error?.message,
				errorKey: created.error ? undefined : 'adcatalog.products.createFailed'
			});
		}

		const id = created.data.id;
		const pricingError = await syncProductPricing(event, id, pricing);
		if (pricingError) {
			// Product exists but pricing failed - land on the edit page with a warning.
			redirect(303, `/admin/products/${id}?pricing_error=1`);
		}

		redirect(303, `/admin/products/${id}?created=1`);
	}
};
