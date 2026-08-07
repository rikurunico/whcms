import { apiUrl } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/**
 * Streams the invoice PDF from the Go API with the session bearer token.
 * +server.ts routes bypass the (admin) layout guard, so re-check the role here.
 */
export const GET: RequestHandler = async (event) => {
	const { locals, params } = event;

	if (!locals.user || !locals.accessToken) {
		error(401, 'Unauthorized');
	}
	if (locals.user.role !== 'admin' && locals.user.role !== 'staff') {
		error(403, 'Forbidden');
	}

	let upstream: Response;
	try {
		upstream = await event.fetch(`${apiUrl()}/api/v1/admin/invoices/${params.id}/pdf`, {
			headers: { authorization: `Bearer ${locals.accessToken}` }
		});
	} catch {
		error(502, 'PDF service unreachable');
	}

	if (!upstream.ok || !upstream.body) {
		error(upstream.status === 404 ? 404 : 502, 'PDF unavailable');
	}

	return new Response(upstream.body, {
		status: 200,
		headers: {
			'content-type': upstream.headers.get('content-type') ?? 'application/pdf',
			'content-disposition':
				upstream.headers.get('content-disposition') ??
				`attachment; filename="invoice-${params.id}.pdf"`,
			'cache-control': 'private, no-store'
		}
	});
};
