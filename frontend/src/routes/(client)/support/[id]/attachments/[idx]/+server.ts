import { apiUrl } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/**
 * Attachment download proxy. The browser cannot attach the Bearer token, so this
 * handler asks the Go API for the attachment and forwards the browser to it.
 *
 * Real upstream contract (backend/internal/modules/tickets/handler.go):
 *   GET /api/v1/tickets/:id/attachments/:idx  (idx = position over the visible
 *   thread's attachments, as returned in each reply's AttachmentView.index -
 *   NOT a query-string object key, despite docs/RECONCILE.md's earlier guess).
 * Response is always a 302/303 redirect to a presigned S3 URL.
 */
export const GET: RequestHandler = async (event) => {
	if (!event.locals.user) {
		redirect(303, `/login?redirect=${encodeURIComponent(event.url.pathname + event.url.search)}`);
	}

	const filename = event.url.searchParams.get('filename') ?? '';

	const upstreamUrl = new URL(
		`${apiUrl()}/api/v1/tickets/${event.params.id}/attachments/${event.params.idx}`
	);

	let upstream: Response;
	try {
		upstream = await event.fetch(upstreamUrl, {
			headers: {
				authorization: `Bearer ${event.locals.accessToken ?? ''}`,
				accept: 'application/json'
			},
			redirect: 'manual'
		});
	} catch {
		error(502, 'attachment service unreachable');
	}

	const location = upstream.headers.get('location');
	if (location) redirect(303, location);

	if (!upstream.ok) {
		error(upstream.status === 404 ? 404 : 502, 'attachment unavailable');
	}

	const contentType = upstream.headers.get('content-type') ?? '';
	if (contentType.includes('application/json')) {
		const body = (await upstream.json().catch(() => null)) as {
			data?: { url?: string; download_url?: string } | null;
		} | null;
		const url = body?.data?.url ?? body?.data?.download_url;
		if (typeof url === 'string' && url) redirect(303, url);
		error(502, 'attachment unavailable');
	}

	// Backend streamed the file directly - pipe it through with its headers.
	const headers = new Headers();
	if (contentType) headers.set('content-type', contentType);
	const disposition = upstream.headers.get('content-disposition');
	if (disposition) {
		headers.set('content-disposition', disposition);
	} else if (filename) {
		headers.set('content-disposition', `inline; filename="${filename.replaceAll('"', '')}"`);
	}
	return new Response(upstream.body, { status: 200, headers });
};
