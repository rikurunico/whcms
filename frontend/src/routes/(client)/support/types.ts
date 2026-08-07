/** Shared DTO types for the client support pages (JSON tags per backend/internal/domain/entities.go). */

export const TICKET_STATUSES = ['open', 'answered', 'customer_reply', 'closed'] as const;
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
}

/**
 * Matches backend AttachmentView (tickets/dto.go): attachments come back on
 * GET /tickets/:id embedded in each reply as {index, filename, size,
 * content_type} - index is the position used by
 * GET /tickets/:id/attachments/:idx (no object_key is ever exposed to
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
	created_at: string;
	attachments: TicketAttachment[];
}
