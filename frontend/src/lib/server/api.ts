import { env } from '$env/dynamic/private';
import type { RequestEvent } from '@sveltejs/kit';

/** Base URL of the Go API (server-side only). */
export function apiUrl(): string {
	return env.API_URL || 'http://localhost:8080';
}

/** Error payload shape of the backend envelope (pkg/httpx). */
export interface ApiErrorPayload {
	code: string;
	message: string;
	details?: unknown[];
}

/** Pagination meta returned in list responses. */
export interface PageMeta {
	page: number;
	per_page: number;
	total: number;
}

/** Backend response envelope: {"data": ..., "meta": {...}|null, "error": {...}|null} */
interface Envelope<T> {
	data: T | null;
	meta?: PageMeta | null;
	error: ApiErrorPayload | null;
}

export class ApiError extends Error {
	readonly code: string;
	readonly status: number;
	readonly details: unknown[];

	constructor(code: string, message: string, status: number, details: unknown[] = []) {
		super(message);
		this.name = 'ApiError';
		this.code = code;
		this.status = status;
		this.details = details;
	}
}

export interface ApiFetchOptions {
	method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
	/** JSON-serialized unless it is FormData/URLSearchParams (sent as-is). */
	body?: unknown;
	/** Query string params; undefined/null values are skipped. */
	query?: Record<string, string | number | boolean | undefined | null>;
	headers?: Record<string, string>;
	/** Override the bearer token (defaults to event.locals.accessToken). */
	token?: string | null;
}

export interface ApiResult<T> {
	data: T | null;
	meta: PageMeta | null;
	error: ApiError | null;
	status: number;
}

/**
 * Typed fetch wrapper for the Go API. Adds `Authorization: Bearer` from the current
 * session and unwraps the JSON envelope into `{ data, meta, error }`.
 *
 * `path` is appended to API_URL as-is - include the `/api/v1` prefix,
 * e.g. `apiFetch(event, '/api/v1/invoices', { query: { page: 1 } })`.
 *
 * Never throws for API/network errors; inspect `result.error` (an ApiError) instead,
 * or use `unwrap(result)` when you want load-style exceptions.
 */
export async function apiFetch<T>(
	event: RequestEvent,
	path: string,
	opts: ApiFetchOptions = {}
): Promise<ApiResult<T>> {
	const url = new URL(apiUrl() + path);
	for (const [k, v] of Object.entries(opts.query ?? {})) {
		if (v !== undefined && v !== null) url.searchParams.set(k, String(v));
	}

	const headers: Record<string, string> = {
		accept: 'application/json',
		'accept-language': event.locals.locale ?? 'id',
		// Forward the real end-user address to the API over this internal
		// service-to-service hop (API_URL points at the api container
		// directly). Without this, the Go API's c.IP() sees this SvelteKit
		// server's own address for every user, collapsing the shared public
		// rate limiter (and the login-lockout key) into one global bucket -
		// see backend/internal/transport/http/middleware.go RateLimit.
		'x-forwarded-for': event.getClientAddress(),
		...opts.headers
	};
	const token = opts.token !== undefined ? opts.token : event.locals.accessToken;
	if (token) headers.authorization = `Bearer ${token}`;

	let body: BodyInit | undefined;
	if (opts.body instanceof FormData || opts.body instanceof URLSearchParams) {
		body = opts.body;
	} else if (opts.body !== undefined) {
		headers['content-type'] = 'application/json';
		body = JSON.stringify(opts.body);
	}

	let res: Response;
	try {
		res = await event.fetch(url, { method: opts.method ?? 'GET', headers, body });
	} catch (e) {
		return {
			data: null,
			meta: null,
			error: new ApiError('NETWORK', e instanceof Error ? e.message : 'network error', 0),
			status: 0
		};
	}

	if (res.status === 204) {
		return { data: null, meta: null, error: null, status: res.status };
	}

	let envelope: Envelope<T>;
	try {
		envelope = (await res.json()) as Envelope<T>;
	} catch {
		return {
			data: null,
			meta: null,
			error: new ApiError(
				res.ok ? 'INTERNAL' : 'EXTERNAL',
				`invalid response from API (HTTP ${res.status})`,
				res.status
			),
			status: res.status
		};
	}

	if (!res.ok || envelope.error) {
		const payload = envelope.error ?? { code: 'INTERNAL', message: `HTTP ${res.status}` };
		return {
			data: null,
			meta: null,
			error: new ApiError(payload.code, payload.message, res.status, payload.details ?? []),
			status: res.status
		};
	}

	return { data: envelope.data, meta: envelope.meta ?? null, error: null, status: res.status };
}

/** Throw the ApiError (SvelteKit will map it to an error page) or return non-null data. */
export function unwrap<T>(result: ApiResult<T>): T {
	if (result.error) throw result.error;
	return result.data as T;
}
