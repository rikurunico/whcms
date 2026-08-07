import { apiFetch } from '$lib/server/api';
import type { RequestEvent } from '@sveltejs/kit';
import type { PricingPayload } from './catalog';

/**
 * The real backend (`catalog.PricingInput` / `PUT /admin/products/:id/pricing`,
 * backend/internal/modules/catalog/dto.go + handler.go) upserts ONE billing
 * cycle's price per call ({cycle, price, setup_fee}) - there is no bulk
 * `{pricing: [...]}` full-set-replace endpoint, despite that shape being
 * docs/RECONCILE.md's assumed contract. Upsert every enabled cycle
 * individually, and (for edits) delete any previously-priced cycle that's no
 * longer enabled. Returns an error message, or null on success.
 */
export async function syncProductPricing(
	event: RequestEvent,
	productId: number,
	desired: PricingPayload[],
	previousCycles: string[] = []
): Promise<string | null> {
	for (const row of desired) {
		const res = await apiFetch(event, `/api/v1/admin/products/${productId}/pricing`, {
			method: 'PUT',
			body: { cycle: row.cycle, price: row.price, setup_fee: row.setup_fee }
		});
		if (res.error) return res.error.message;
	}

	const desiredCycles = new Set<string>(desired.map((d) => d.cycle));
	for (const cycle of previousCycles) {
		if (desiredCycles.has(cycle)) continue;
		const res = await apiFetch(event, `/api/v1/admin/products/${productId}/pricing/${cycle}`, {
			method: 'DELETE'
		});
		if (res.error) return res.error.message;
	}

	return null;
}
