/** Shared DTO types for the admin support pages (JSON tags per backend/internal/domain/entities.go). */

export const TICKET_STATUSES = ['open', 'answered', 'customer_reply', 'on_hold', 'closed'] as const;
export type TicketStatus = (typeof TICKET_STATUSES)[number];

export const TICKET_PRIORITIES = ['low', 'medium', 'high'] as const;
export type TicketPriority = (typeof TICKET_PRIORITIES)[number];

export interface TicketDepartment {
	id: number;
	name: string;
	email: string;
	active: boolean;
	sort: number;
}

export interface Ticket {
	id: number;
	ticket_number: string;
	client_id: number | null;
	department_id: number;
	subject: string;
	status: string;
	priority: string;
	assigned_user_id: number | null;
	last_reply_at: string | null;
	closed_at: string | null;
	created_at: string;
	updated_at: string;
	/** Optional joined fields - some list endpoints embed them. */
	client_name?: string;
	department_name?: string;
	assigned_email?: string;
}

/**
 * Matches backend AttachmentView (tickets/dto.go): attachments come back on
 * GET /admin/tickets/:id embedded in each reply as {index, filename, size,
 * content_type} - index is the position used by
 * GET /admin/tickets/:id/attachments/:idx (no object_key is ever exposed to
 * clients/admins, only the resolved presigned-URL index).
 */
export interface TicketAttachment {
	index: number;
	filename: string;
	size: number;
	content_type: string;
}

export interface TicketReply {
	id: number;
	ticket_id: number;
	user_id: number | null;
	author_name: string;
	message: string;
	is_internal: boolean;
	/** json.RawMessage on the wire - may arrive as an array or a JSON string. */
	attachments: unknown;
	created_at: string;
	updated_at: string;
}

/** Reply with attachments normalized to a typed array (done server-side in load). */
export interface NormalizedReply {
	id: number;
	ticket_id: number;
	user_id: number | null;
	author_name: string;
	message: string;
	is_internal: boolean;
	created_at: string;
	attachments: TicketAttachment[];
}

/** Staff/admin user row (domain.User JSON tags; password_hash never serialized). */
export interface StaffUser {
	id: number;
	email: string;
	role: 'admin' | 'staff' | 'client';
	status: 'active' | 'inactive';
	/** json.RawMessage on the wire - {"billing":true,...}, may arrive as object or JSON string. */
	permissions: unknown;
	twofa_enabled: boolean;
	last_login_at: string | null;
	created_at: string;
}

/** attachments is json.RawMessage on the wire - normalize array | JSON string | null. */
export function parseAttachments(raw: unknown): TicketAttachment[] {
	let list: unknown = raw;
	if (typeof raw === 'string' && raw.trim()) {
		try {
			list = JSON.parse(raw);
		} catch {
			return [];
		}
	}
	if (!Array.isArray(list)) return [];
	return list.filter(
		(a): a is TicketAttachment =>
			typeof a === 'object' &&
			a !== null &&
			typeof (a as TicketAttachment).index === 'number' &&
			typeof (a as TicketAttachment).filename === 'string'
	);
}

/** permissions is json.RawMessage on the wire - normalize object | JSON string | null. */
export function parsePermissions(raw: unknown): Record<string, boolean> {
	let obj: unknown = raw;
	if (typeof raw === 'string' && raw.trim()) {
		try {
			obj = JSON.parse(raw);
		} catch {
			return {};
		}
	}
	if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) return {};
	const out: Record<string, boolean> = {};
	for (const [k, v] of Object.entries(obj)) out[k] = v === true;
	return out;
}
