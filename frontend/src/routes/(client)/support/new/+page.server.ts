import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { TICKET_PRIORITIES, type Ticket, type TicketDepartment } from '../types';

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<TicketDepartment[]>(event, '/api/v1/ticket-departments');
	return {
		departments: (res.data ?? []).filter((d) => d.active !== false),
		loadError: res.error?.message ?? null
	};
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const departmentId = String(form.get('department_id') ?? '').trim();
		const subject = String(form.get('subject') ?? '').trim();
		const priority = String(form.get('priority') ?? '').trim() || 'medium';
		const message = String(form.get('message') ?? '').trim();
		const values = { department_id: departmentId, subject, priority, message };

		const fieldErrors: Record<string, string> = {};
		if (!departmentId) fieldErrors.department_id = 'supportfe.form.required';
		if (!subject) fieldErrors.subject = 'supportfe.form.required';
		if (!message) fieldErrors.message = 'supportfe.form.required';
		if (!(TICKET_PRIORITIES as readonly string[]).includes(priority)) {
			fieldErrors.priority = 'supportfe.form.invalidPriority';
		}
		if (Object.keys(fieldErrors).length > 0) {
			return fail(400, { fieldErrors, values });
		}

		// Multipart POST /tickets - attachments go straight to S3 backend-side.
		const body = new FormData();
		body.set('department_id', departmentId);
		body.set('subject', subject);
		body.set('priority', priority);
		body.set('message', message);
		for (const entry of form.getAll('attachments')) {
			if (entry instanceof File && entry.name && entry.size > 0) {
				body.append('attachments', entry, entry.name);
			}
		}

		const res = await apiFetch<Ticket | { ticket?: Ticket }>(event, '/api/v1/tickets', {
			method: 'POST',
			body
		});

		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 500, {
				errorMessage: res.error?.message,
				errorKey: res.error ? undefined : 'supportfe.form.createFailed',
				values
			});
		}

		const created = res.data as { id?: number; ticket?: { id?: number } };
		const id = created.id ?? created.ticket?.id;
		redirect(303, id ? `/support/${id}` : '/support');
	}
};
