/**
 * FE-ADMIN-BILLING shared response types.
 * Field names follow backend/internal/domain/entities.go JSON tags; optional
 * aggregate fields (client_name, invoice_number on transactions, embedded
 * items/client) are best-effort - the UI falls back gracefully when absent.
 */

export type InvoiceStatus = 'draft' | 'unpaid' | 'paid' | 'overdue' | 'cancelled' | 'refunded';
export type TransactionStatus = 'pending' | 'success' | 'failed' | 'expired' | 'refunded';
export type Gateway = 'duitku' | 'credit' | 'manual';

export const INVOICE_STATUSES: InvoiceStatus[] = [
	'draft',
	'unpaid',
	'paid',
	'overdue',
	'cancelled',
	'refunded'
];

export const TRANSACTION_STATUSES: TransactionStatus[] = [
	'pending',
	'success',
	'failed',
	'expired',
	'refunded'
];

export const GATEWAYS: Gateway[] = ['duitku', 'credit', 'manual'];

export interface AdminInvoiceItem {
	id: number;
	invoice_id: number;
	description: string;
	amount: number;
	taxed: boolean;
	related_type: string;
	related_id: number | null;
	created_at?: string;
}

export interface AdminClientLite {
	id: number;
	first_name: string;
	last_name: string;
	company?: string;
	email?: string;
	address1?: string;
	address2?: string;
	city?: string;
	state?: string;
	postcode?: string;
	country?: string;
	phone?: string;
	credit_balance?: number;
	status?: string;
}

export interface AdminInvoice {
	id: number;
	invoice_number: string;
	client_id: number;
	status: InvoiceStatus;
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
	updated_at: string;
	/** Optional aggregates the admin endpoints may embed. */
	client_name?: string;
	client_email?: string;
	items?: AdminInvoiceItem[];
	client?: AdminClientLite;
}

export interface AdminTransaction {
	id: number;
	invoice_id: number;
	gateway: Gateway;
	method_code: string;
	merchant_order_id: string | null;
	gateway_reference: string;
	amount: number;
	fee: number;
	status: TransactionStatus;
	paid_at: string | null;
	created_at: string;
	/** Optional aggregates. */
	invoice_number?: string;
	client_name?: string;
}

/** Client display name helper shared by admin billing pages. */
export function clientDisplayName(c: AdminClientLite | null | undefined): string {
	if (!c) return '';
	const name = [c.first_name, c.last_name].filter(Boolean).join(' ').trim();
	return name || c.email || `#${c.id}`;
}
