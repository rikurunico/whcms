/**
 * Shared types + response normalizers for the client billing area.
 * Field names follow backend/internal/domain/entities.go JSON tags.
 * Handler DTO wrappers are not final yet (backend built concurrently), so the
 * normalizers accept both flat and nested shapes and snake/camel variants.
 */

export interface Invoice {
	id: number;
	invoice_number: string;
	client_id: number;
	status: string;
	subtotal: number;
	discount: number;
	tax_rate: number;
	tax_total: number;
	credit_applied: number;
	total: number;
	currency: string;
	due_date: string;
	paid_at: string | null;
	notes: string;
	pdf_object_key?: string;
	created_at: string;
	updated_at?: string;
}

export interface InvoiceItem {
	id: number;
	invoice_id: number;
	description: string;
	amount: number;
	taxed: boolean;
	related_type?: string;
	related_id?: number | null;
}

export interface Transaction {
	id: number;
	invoice_id: number;
	gateway: string;
	method_code: string;
	merchant_order_id?: string | null;
	gateway_reference?: string;
	amount: number;
	fee: number;
	status: string;
	paid_at?: string | null;
	expires_at?: string | null;
	/** ports.CreateTxResult marshaled as-is (PascalCase Go field names) -
	 *  only ever read by resumedPaymentInstructions() to reconstruct a
	 *  pending gateway transaction's instructions on page reload. */
	raw?: unknown;
	created_at: string;
}

/** GET /payments/methods item - ports.PaymentMethod JSON tags. */
export interface PaymentMethodInfo {
	code: string;
	name: string;
	image?: string;
	fee: number;
	gateway?: string;
}

/** One manual bank-transfer destination - ports.BankAccount JSON tags. */
export interface BankAccount {
	bank_name: string;
	account_number: string;
	account_holder: string;
}

/** Normalized POST /invoices/:id/pay result (gateway payment instructions).
 *  bankAccounts/note are only ever populated for the manual bank-transfer
 *  gateway (method "bank_transfer") - empty for Duitku, and vice versa for
 *  paymentUrl/vaNumber/qrString.
 *  amount is the TOTAL the customer must actually transfer via this channel
 *  (the invoice's remaining balance plus fee, when the gateway passes its
 *  fee on to the customer - Duitku reports this gross figure itself on the
 *  VA/QRIS it creates). fee is 0 for channels with no surcharge. */
export interface PaymentInstructions {
	reference: string | null;
	paymentUrl: string | null;
	vaNumber: string | null;
	qrString: string | null;
	bankAccounts: BankAccount[];
	note: string | null;
	amount: number | null;
	fee: number;
	expiresAt: string | null;
	expiryMinutes: number | null;
}

export interface InvoiceDetail {
	invoice: Invoice | null;
	items: InvoiceItem[];
	transactions: Transaction[];
}

/** Invoice statuses that show the payment panel + enable status polling. */
export function isPayable(status: string | undefined | null): boolean {
	return status === 'unpaid' || status === 'overdue';
}

type Rec = Record<string, unknown>;

function asRecord(v: unknown): Rec | null {
	return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : null;
}

function pick(rec: Rec | null, ...keys: string[]): unknown {
	if (!rec) return undefined;
	for (const key of keys) {
		const v = rec[key];
		if (v !== undefined && v !== null) return v;
	}
	return undefined;
}

function asString(v: unknown): string | null {
	return typeof v === 'string' && v !== '' ? v : null;
}

function asNumber(v: unknown): number | null {
	if (typeof v === 'number' && Number.isFinite(v)) return v;
	if (typeof v === 'string' && v !== '' && Number.isFinite(Number(v))) return Number(v);
	return null;
}

/** GET /invoices/:id - accepts a flat invoice or `{invoice, items, transactions}`. */
export function normalizeInvoiceDetail(data: unknown): InvoiceDetail {
	const root = asRecord(data);
	if (!root) return { invoice: null, items: [], transactions: [] };

	const nested = asRecord(root.invoice);
	const invoiceRec = nested ?? root;
	const invoice =
		typeof invoiceRec.id === 'number' || typeof invoiceRec.invoice_number === 'string'
			? (invoiceRec as unknown as Invoice)
			: null;

	const items = pick(root, 'items', 'invoice_items') ?? pick(nested, 'items', 'invoice_items');
	const transactions = pick(root, 'transactions') ?? pick(nested, 'transactions');

	return {
		invoice,
		items: Array.isArray(items) ? (items as InvoiceItem[]) : [],
		transactions: Array.isArray(transactions) ? (transactions as Transaction[]) : []
	};
}

function asBankAccounts(v: unknown): BankAccount[] {
	if (!Array.isArray(v)) return [];
	return v
		.map((row) => asRecord(row))
		.filter((row): row is Rec => row !== null)
		.map((row) => ({
			bank_name: asString(pick(row, 'bank_name')) ?? '',
			account_number: asString(pick(row, 'account_number')) ?? '',
			account_holder: asString(pick(row, 'account_holder')) ?? ''
		}));
}

/**
 * Finds the most recent still-valid pending gateway transaction for an
 * invoice, so a page reload can resume its payment instructions instead of
 * silently starting a brand new gateway transaction (a new VA/QRIS/reference)
 * on every visit - the credit gateway never leaves a pending row (it settles
 * immediately), so no gateway-specific filtering is needed here.
 * `nowMs` is injected (rather than read internally) so this stays pure/testable.
 */
export function findResumablePayment(
	transactions: Transaction[],
	nowMs: number
): Transaction | null {
	const candidates = transactions
		.filter((t) => t.status === 'pending')
		.filter((t) => !t.expires_at || new Date(t.expires_at).getTime() > nowMs)
		.sort((a, b) => b.id - a.id);
	return candidates[0] ?? null;
}

/**
 * Reconstructs POST /invoices/:id/pay's response shape from a pending
 * Transaction row for the resume case above. `raw` is `ports.CreateTxResult`
 * marshaled as-is (PascalCase Go field names, no json tags) - amount/fee
 * instead come from the transaction row's own persisted columns (net + fee),
 * which is the authoritative source (see payments.Service.payWithGateway).
 */
export function resumedPaymentInstructions(tx: Transaction): PaymentInstructions {
	const raw = asRecord(tx.raw);
	return {
		reference: asString(pick(raw, 'Reference')) ?? tx.gateway_reference ?? null,
		paymentUrl: asString(pick(raw, 'PaymentURL')),
		vaNumber: asString(pick(raw, 'VANumber')),
		qrString: asString(pick(raw, 'QRString')),
		bankAccounts: asBankAccounts(pick(raw, 'BankAccounts')),
		note: asString(pick(raw, 'Note')),
		amount: tx.amount + tx.fee,
		fee: tx.fee,
		expiresAt: tx.expires_at ?? null,
		expiryMinutes: null
	};
}

/** POST /invoices/:id/pay - normalizes gateway instructions across naming variants. */
export function normalizePayResult(data: unknown): PaymentInstructions {
	const root = asRecord(data);
	const rec = asRecord(pick(root, 'payment', 'result', 'transaction')) ?? root;
	return {
		reference: asString(pick(rec, 'reference', 'gateway_reference')),
		paymentUrl: asString(pick(rec, 'payment_url', 'paymentUrl')),
		vaNumber: asString(pick(rec, 'va_number', 'vaNumber')),
		qrString: asString(pick(rec, 'qr_string', 'qrString')),
		bankAccounts: asBankAccounts(pick(rec, 'bank_accounts', 'bankAccounts')),
		note: asString(pick(rec, 'note')),
		amount: asNumber(pick(rec, 'amount', 'total')),
		fee: asNumber(pick(rec, 'fee')) ?? 0,
		expiresAt: asString(pick(rec, 'expires_at', 'expired_at', 'expiry_at', 'expiresAt')),
		expiryMinutes: asNumber(pick(rec, 'expiry_minutes', 'expiryMinutes'))
	};
}

/** Reads `balance_after` from the newest entry of a CreditLedgerEntry[] slice. */
function balanceFromLedger(entries: unknown[]): number | null {
	if (entries.length === 0) return 0; // empty ledger → no credit
	return asNumber(pick(asRecord(entries[0]), 'balance_after', 'balanceAfter'));
}

/**
 * GET /account/credit - extracts the balance; null when the shape is unknown.
 * CONTRACTS §9 defines the endpoint as the credit ledger, so `data` is most
 * likely CreditLedgerEntry[] (entities.go: `balance_after`, newest first);
 * flat `{balance}` / `{credit_balance}` and nested ledger keys also accepted.
 */
export function normalizeCreditBalance(data: unknown): number | null {
	if (Array.isArray(data)) return balanceFromLedger(data);
	const root = asRecord(data);
	const direct = asNumber(pick(root, 'balance', 'credit_balance'));
	if (direct !== null) return direct;
	const entries = pick(root, 'ledger', 'entries');
	return Array.isArray(entries) ? balanceFromLedger(entries) : null;
}

/** Extracts the created/related invoice id from mutation responses (deposit, pay). */
export function extractInvoiceId(data: unknown): number | null {
	const root = asRecord(data);
	if (!root) return null;
	const direct = asNumber(pick(root, 'invoice_id', 'invoiceId'));
	if (direct) return direct;
	const nested = asRecord(root.invoice);
	const nestedId = asNumber(nested?.id);
	if (nestedId) return nestedId;
	// Flat invoice object: only trust `id` when it looks like an invoice.
	if (typeof root.invoice_number === 'string') return asNumber(root.id);
	return null;
}
