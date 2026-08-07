import { apiUrl } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

/**
 * Streams GET /api/v1/admin/clients/export (real backend route registered
 * before /:id - see clients/handler.go). Layout guards do not run for
 * +server.ts routes, so re-check the role here.
 */
export const GET: RequestHandler = async (event) => {
	const user = event.locals.user;
	if (!user || (user.role !== 'admin' && user.role !== 'staff')) {
		error(403, 'forbidden');
	}

	let upstream: Response;
	try {
		upstream = await event.fetch(`${apiUrl()}/api/v1/admin/clients/export`, {
			headers: {
				authorization: `Bearer ${event.locals.accessToken ?? ''}`,
				accept: 'text/csv'
			}
		});
	} catch {
		error(502, 'export unavailable');
	}

	if (!upstream.ok || !upstream.body) {
		error(upstream.status >= 400 ? upstream.status : 502, 'export failed');
	}

	return new Response(upstream.body, {
		status: 200,
		headers: {
			'content-type': upstream.headers.get('content-type') ?? 'text/csv; charset=utf-8',
			'content-disposition':
				upstream.headers.get('content-disposition') ?? 'attachment; filename="clients.csv"',
			'cache-control': 'no-store'
		}
	});
};
