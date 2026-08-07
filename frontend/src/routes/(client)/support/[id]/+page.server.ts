import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type {
	NormalizedReply,
	Ticket,
	TicketAttachment,
	TicketDepartment,
	TicketReply
} from '../types';

/** attachments is json.RawMessage on the wire - normalize array | JSON string | null. */
function parseAttachments(raw: unknown): TicketAttachment[] {
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

type TicketDetailPayload =
	(Ticket & { replies?: TicketReply[] }) | { ticket?: Ticket; replies?: TicketReply[] };

export const load: PageServerLoad = async (event) => {
	const [ticketRes, deptRes] = await Promise.all([
		apiFetch<TicketDetailPayload>(event, `/api/v1/tickets/${event.params.id}`),
		apiFetch<TicketDepartment[]>(event, '/api/v1/ticket-departments')
	]);

	// GET /tickets/:id may return the ticket flat (with embedded replies) or a
	// {ticket, replies} composite - accept both, since the DTO is not pinned in CONTRACTS.
	let ticket: Ticket | null = null;
	let rawReplies: TicketReply[] = [];
	const payload = ticketRes.data;
	if (payload) {
		if ('ticket' in payload && payload.ticket) {
			ticket = payload.ticket;
			rawReplies = payload.replies ?? [];
		} else {
			const flat = payload as Ticket & { replies?: TicketReply[] };
			ticket = flat;
			rawReplies = flat.replies ?? [];
		}
	}

	const replies: NormalizedReply[] = rawReplies
		.filter((r) => !r.is_internal)
		.map((r) => ({
			id: r.id,
			ticket_id: r.ticket_id,
			user_id: r.user_id,
			author_name: r.author_name,
			message: r.message,
			created_at: r.created_at,
			attachments: parseAttachments(r.attachments)
		}))
		.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());

	return {
		ticket,
		replies,
		departments: deptRes.data ?? [],
		notFound: ticketRes.status === 404,
		loadError: ticketRes.error?.message ?? null
	};
};

export const actions: Actions = {
	reply: async (event) => {
		const form = await event.request.formData();
		const message = String(form.get('message') ?? '').trim();
		if (!message) {
			return fail(400, {
				replyErrorKey: 'supportfe.detail.messageRequired',
				replyMessage: message
			});
		}

		const body = new FormData();
		body.set('message', message);
		for (const entry of form.getAll('attachments')) {
			if (entry instanceof File && entry.name && entry.size > 0) {
				body.append('attachments', entry, entry.name);
			}
		}

		const res = await apiFetch(event, `/api/v1/tickets/${event.params.id}/replies`, {
			method: 'POST',
			body
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				replyErrorMessage: res.error.message,
				replyMessage: message
			});
		}

		return { replied: true };
	},

	close: async (event) => {
		const res = await apiFetch(event, `/api/v1/tickets/${event.params.id}/close`, {
			method: 'POST'
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				closeErrorMessage: res.error.message
			});
		}

		return { closed: true };
	}
};
