import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** domain.EmailTemplate JSON tags. */
export interface EmailTemplate {
	id: number;
	key: string;
	locale: string;
	subject: string;
	body_html: string;
	body_text: string;
	created_at: string;
	updated_at: string;
}

/** notifications.PreviewResult. */
interface PreviewPayload {
	subject: string;
	body_html: string;
	body_text: string;
}

/**
 * Representative values for every template variable (union of all keys - the
 * renderer simply ignores the ones a given template does not reference).
 * Mirrors the variable table in internal/modules/notifications/dto.go.
 */
function sampleDataFor(locale: string): Record<string, string> {
	const id = locale !== 'en';
	return {
		Name: id ? 'Budi Santoso' : 'John Smith',
		Email: id ? 'budi@example.co.id' : 'john@example.com',
		VerifyURL: 'https://panel.example.com/verify-email?token=sample-token',
		ResetURL: 'https://panel.example.com/reset-password?token=sample-token',
		InvoiceNumber: 'INV-2026-001042',
		InvoiceURL: 'https://panel.example.com/billing/invoices/1042',
		Total: 'Rp 1.250.000',
		Amount: 'Rp 1.250.000',
		DueDate: id ? '10 Agustus 2026' : '10 August 2026',
		ServiceName: 'Business Hosting 20GB',
		Domain: 'contoh.co.id',
		Username: 'contohco',
		Password: 'S4mpl3-P4ssw0rd',
		PanelURL: 'https://srv01.example.com:2083',
		Reason: id ? 'Tagihan belum dibayar' : 'Unpaid invoice',
		NextDueDate: id ? '10 September 2026' : '10 September 2026',
		ExpiryDate: id ? '22 Juli 2027' : '22 July 2027',
		TicketNumber: 'TKT-884210',
		Subject: id ? 'Website tidak bisa diakses' : 'Website is unreachable',
		TicketURL: 'https://panel.example.com/support/884210',
		Detail: 'provisioning: cpanel createacct failed\nserver=srv01 code=500'
	};
}

export const load: PageServerLoad = async (event) => {
	const { key, locale } = event.params;
	const res = await apiFetch<EmailTemplate>(
		event,
		`/api/v1/admin/email-templates/${key}/${locale}`
	);

	return {
		template: res.data,
		notFound: res.status === 404,
		loadError: res.error?.message ?? null
	};
};

export const actions: Actions = {
	save: async (event) => {
		const { key, locale } = event.params;
		const form = await event.request.formData();
		const subject = String(form.get('subject') ?? '').trim();
		const bodyHtml = String(form.get('body_html') ?? '');
		const bodyText = String(form.get('body_text') ?? '');

		const errors: Record<string, string> = {};
		if (!subject) errors.subject = 'adminsupport.common.requiredField';
		if (!bodyHtml.trim()) errors.body_html = 'adminsupport.common.requiredField';
		if (Object.keys(errors).length > 0) {
			return fail(400, {
				fieldErrors: errors,
				values: { subject, body_html: bodyHtml, body_text: bodyText }
			});
		}

		const res = await apiFetch(event, '/api/v1/admin/email-templates', {
			method: 'PUT',
			body: { key, locale, subject, body_html: bodyHtml, body_text: bodyText }
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				errorMessage: res.error.message,
				values: { subject, body_html: bodyHtml, body_text: bodyText }
			});
		}
		return { saved: true };
	},

	preview: async (event) => {
		const { key, locale } = event.params;

		// The backend only previews the STORED template (no ad-hoc subject/body
		// override), so this reflects the last-saved content, not unsaved edits.
		// Sample data is required: without it, per-key placeholders render as
		// "<no value>" (or "#ZgotmplZ" inside an href), which makes the preview
		// useless for judging the design.
		const res = await apiFetch<PreviewPayload>(event, '/api/v1/admin/email-templates/preview', {
			method: 'POST',
			body: { key, locale, sample_data: sampleDataFor(locale) }
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				previewErrorMessage: res.error.message
			});
		}

		const payload = res.data;
		return {
			preview: {
				subject: payload?.subject ?? '',
				html: payload?.body_html ?? ''
			}
		};
	}
};
