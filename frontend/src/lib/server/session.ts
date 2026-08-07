import { dev } from '$app/environment';
import type { Cookies } from '@sveltejs/kit';

export const ACCESS_COOKIE = 'access_token';
export const REFRESH_COOKIE = 'refresh_token';

/** Access JWT TTL is 15m (CONTRACTS.md §3); keep the cookie alive slightly longer so
 *  the auto-refresh path in hooks.server.ts is exercised via the refresh cookie instead. */
const ACCESS_MAX_AGE = 60 * 15;
/** Refresh token TTL is 30 days. */
const REFRESH_MAX_AGE = 60 * 60 * 24 * 30;

export interface SessionTokens {
	access_token: string;
	refresh_token: string;
}

const baseOptions = {
	path: '/',
	httpOnly: true,
	sameSite: 'lax',
	secure: !dev
} as const;

/** Persist the token pair as httpOnly cookies. Call after login and after each refresh rotation. */
export function setSessionCookies(cookies: Cookies, tokens: SessionTokens): void {
	cookies.set(ACCESS_COOKIE, tokens.access_token, { ...baseOptions, maxAge: ACCESS_MAX_AGE });
	cookies.set(REFRESH_COOKIE, tokens.refresh_token, { ...baseOptions, maxAge: REFRESH_MAX_AGE });
}

/** Remove both session cookies (logout / invalid session). */
export function clearSessionCookies(cookies: Cookies): void {
	cookies.delete(ACCESS_COOKIE, { path: '/' });
	cookies.delete(REFRESH_COOKIE, { path: '/' });
}
