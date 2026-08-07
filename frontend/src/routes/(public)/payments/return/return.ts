/**
 * Normalizer for GET /api/v1/payments/return (merchantOrderId -> invoice mapping).
 * The exact response DTO is not final (backend built concurrently); accept
 * snake/camel variants and nested invoice objects.
 */

export interface ReturnInfo {
	invoiceId: number | null;
	status: string | null;
}

type Rec = Record<string, unknown>;

function asRecord(v: unknown): Rec | null {
	return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : null;
}

function asNumber(v: unknown): number | null {
	if (typeof v === 'number' && Number.isFinite(v)) return v;
	if (typeof v === 'string' && v !== '' && Number.isFinite(Number(v))) return Number(v);
	return null;
}

function asString(v: unknown): string | null {
	return typeof v === 'string' && v !== '' ? v : null;
}

export function normalizeReturnInfo(data: unknown): ReturnInfo {
	const root = asRecord(data);
	if (!root) return { invoiceId: null, status: null };

	const invoice = asRecord(root.invoice);
	const invoiceId = asNumber(root.invoice_id) ?? asNumber(root.invoiceId) ?? asNumber(invoice?.id);
	const status =
		asString(root.status) ??
		asString(root.invoice_status) ??
		asString(root.payment_status) ??
		asString(invoice?.status);

	return { invoiceId, status };
}
