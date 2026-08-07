/** Shared DTO types for the client email-history page (JSON tags per backend/internal/domain/entities.go). */

export const EMAIL_LOG_STATUSES = ['queued', 'sent', 'failed'] as const;
export type EmailLogStatus = (typeof EMAIL_LOG_STATUSES)[number];

export interface EmailLogEntry {
	id: number;
	user_id: number | null;
	to_email: string;
	template_key: string;
	subject: string;
	status: string;
	error: string;
	sent_at: string | null;
	created_at: string;
	updated_at: string;
}
