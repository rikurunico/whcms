import { apiFetch } from '$lib/server/api';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/** One row of GET /admin/registrars/:id/catalog - domains.RegistrarCatalogResponse. */
interface RegistrarCatalogRow {
	tld: string;
	currency: string;
	register_prices: Record<string, number>;
	renew_prices: Record<string, number>;
	transfer_price: number;
	restore_price: number;
	already_configured: boolean;
}

interface RegistrarCatalogResult {
	ok: boolean;
	message?: string;
	items?: RegistrarCatalogRow[];
}

/**
 * BFF proxy for the TLD Pricing "Import from Registrar" modal - previews the
 * registrar's full TLD pricelist so the admin can pick which extensions to
 * sell instead of hand-typing every one (same shape/precedent as
 * module-packages/+server.ts's "Load from Server" package picker).
 *
 * Normalizes a hard backend error (bad registrar id, registrar connectivity
 * failure - this endpoint has no soft-fail `ok` field of its own) into the
 * same `{ok:false, message}` shape the modal already knows how to render.
 */
export const GET: RequestHandler = async (event) => {
	const registrarId = event.url.searchParams.get('registrar_id');
	if (!registrarId) {
		return json({
			ok: false,
			message: 'No registrar configured.'
		} satisfies RegistrarCatalogResult);
	}
	const res = await apiFetch<RegistrarCatalogRow[]>(
		event,
		`/api/v1/admin/registrars/${registrarId}/catalog`
	);
	if (res.error) {
		return json({ ok: false, message: res.error.message } satisfies RegistrarCatalogResult);
	}
	return json({ ok: true, items: res.data ?? [] } satisfies RegistrarCatalogResult);
};
