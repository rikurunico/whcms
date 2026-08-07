import { apiUrl } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/**
 * Streams GET /api/v1/invoices/:id/pdf to the browser with the session bearer.
 * Handles both upstream behaviors allowed by CONTRACTS §9: a direct byte stream
 * or a JSON envelope carrying a presigned URL (-> 302 redirect).
 */
export const GET: RequestHandler = async (event) => {
	if (!event.locals.user) {
		error(401, 'Unauthorized');
	}

	const id = Number(event.params.id);
	if (!Number.isInteger(id) || id <= 0) {
		error(404, 'Invoice not found');
	}

	const headers: Record<string, string> = {};
	if (event.locals.accessToken) {
		headers.authorization = `Bearer ${event.locals.accessToken}`;
	}

	let upstream: Response;
	try {
		upstream = await event.fetch(`${apiUrl()}/api/v1/invoices/${id}/pdf`, { headers });
	} catch {
		error(502, 'PDF service unavailable');
	}

	const contentType = upstream.headers.get('content-type') ?? '';

	if (contentType.includes('application/json')) {
		interface PdfEnvelope {
			data?: string | { url?: string; pdf_url?: string } | null;
			error?: { message?: string } | null;
		}
		let payload: PdfEnvelope | null = null;
		try {
			payload = (await upstream.json()) as PdfEnvelope;
		} catch {
			payload = null;
		}
		const data = payload?.data;
		const presigned = typeof data === 'string' ? data : (data?.url ?? data?.pdf_url ?? null);
		if (upstream.ok && presigned) {
			redirect(302, presigned);
		}
		error(upstream.ok ? 502 : upstream.status, payload?.error?.message ?? 'PDF unavailable');
	}

	if (!upstream.ok || !upstream.body) {
		error(upstream.status || 502, 'PDF unavailable');
	}

	return new Response(upstream.body, {
		status: 200,
		headers: {
			'content-type': contentType || 'application/pdf',
			'content-disposition':
				upstream.headers.get('content-disposition') ?? `attachment; filename="invoice-${id}.pdf"`,
			'cache-control': 'private, no-store'
		}
	});
};
