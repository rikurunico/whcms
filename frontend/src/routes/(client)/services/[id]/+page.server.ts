import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
	BILLING_CYCLES,
	parsePendingUpgrade,
	type CatalogGroup,
	type PendingUpgrade,
	type Service
} from '../types';

type ActionName = 'changePassword' | 'sso' | 'cancel' | 'upgrade';

/** Uniform action payload so `form` keeps one shape across all named actions. */
interface ActionPayload {
	action: ActionName;
	success: boolean;
	ssoUrl: string | null;
	errorKey: string | null;
	errorMessage: string | null;
}

function ok(action: ActionName, ssoUrl: string | null = null): ActionPayload {
	return { action, success: true, ssoUrl, errorKey: null, errorMessage: null };
}

function actionFail(
	action: ActionName,
	status: number,
	err: { errorKey?: string; errorMessage?: string }
) {
	const httpStatus = status >= 400 && status <= 599 ? status : 502;
	return fail(httpStatus, {
		action,
		success: false,
		ssoUrl: null,
		errorKey: err.errorKey ?? null,
		errorMessage: err.errorMessage ?? null
	} satisfies ActionPayload);
}

/** Guessed shapes for POST /services/:id/upgrade (endpoint not in CONTRACTS §9). */
interface UpgradeResponse {
	invoice_id?: number;
	invoice?: { id: number };
	pending_upgrade?: PendingUpgrade | string | null;
}

interface SSOResponse {
	url?: string;
	sso_url?: string;
}

export const load: PageServerLoad = async (event) => {
	const [svcRes, productsRes] = await Promise.all([
		apiFetch<Service>(event, `/api/v1/services/${event.params.id}`),
		// Catalog for the upgrade picker; failure here must not break the page.
		// GET /products returns grouped data (see CatalogGroup) - flatten before use.
		apiFetch<CatalogGroup[]>(event, '/api/v1/products')
	]);

	const notFound = svcRes.status === 404 || svcRes.error?.code === 'NOT_FOUND';
	// Custom-spec (configurable) products aren't priced/applied correctly by
	// Upgrade (no spec-selection input) - kept out of this picker; the only
	// way to change them today is the initial order/configure flow.
	const products = (productsRes.data ?? [])
		.flatMap((g) => g.products ?? [])
		.filter((p) => !p.hidden && !p.configurable);

	return {
		service: svcRes.data,
		notFound,
		loadError: notFound ? null : (svcRes.error?.message ?? null),
		products
	};
};

export const actions: Actions = {
	changePassword: async (event) => {
		const form = await event.request.formData();
		const password = String(form.get('password') ?? '');
		const confirm = String(form.get('password_confirm') ?? '');

		if (password.length < 8) {
			return actionFail('changePassword', 400, {
				errorKey: 'clientsvc.errors.passwordTooShort'
			});
		}
		if (password !== confirm) {
			return actionFail('changePassword', 400, {
				errorKey: 'clientsvc.errors.passwordMismatch'
			});
		}

		const res = await apiFetch(event, `/api/v1/services/${event.params.id}/change-password`, {
			method: 'POST',
			body: { password }
		});
		if (res.error) {
			return actionFail('changePassword', res.status, { errorMessage: res.error.message });
		}
		return ok('changePassword');
	},

	sso: async (event) => {
		const res = await apiFetch<SSOResponse>(event, `/api/v1/services/${event.params.id}/sso`);
		const ssoUrl = res.data?.url ?? res.data?.sso_url ?? null;
		if (res.error) {
			return actionFail('sso', res.status, { errorMessage: res.error.message });
		}
		if (!ssoUrl) {
			return actionFail('sso', 502, { errorKey: 'clientsvc.errors.ssoUnavailable' });
		}
		return ok('sso', ssoUrl);
	},

	cancel: async (event) => {
		const form = await event.request.formData();
		const mode = String(form.get('mode') ?? '');
		if (mode !== 'immediate' && mode !== 'end_of_term') {
			return actionFail('cancel', 400, { errorKey: 'clientsvc.errors.invalidMode' });
		}

		const res = await apiFetch(event, `/api/v1/services/${event.params.id}/cancel`, {
			method: 'POST',
			body: { mode }
		});
		if (res.error) {
			return actionFail('cancel', res.status, { errorMessage: res.error.message });
		}
		return ok('cancel');
	},

	upgrade: async (event) => {
		const form = await event.request.formData();
		const productId = Number(form.get('product_id'));
		const cycle = String(form.get('cycle') ?? '');
		if (
			!Number.isInteger(productId) ||
			productId <= 0 ||
			!(BILLING_CYCLES as readonly string[]).includes(cycle)
		) {
			return actionFail('upgrade', 400, { errorKey: 'clientsvc.errors.invalidUpgrade' });
		}

		const res = await apiFetch<UpgradeResponse>(
			event,
			`/api/v1/services/${event.params.id}/upgrade`,
			{ method: 'POST', body: { product_id: productId, cycle } }
		);
		if (res.error) {
			return actionFail('upgrade', res.status, { errorMessage: res.error.message });
		}

		const invoiceId =
			res.data?.invoice_id ??
			res.data?.invoice?.id ??
			parsePendingUpgrade(res.data?.pending_upgrade)?.invoice_id ??
			null;
		if (invoiceId) {
			redirect(303, `/billing/invoices/${invoiceId}`);
		}
		return ok('upgrade');
	}
};
