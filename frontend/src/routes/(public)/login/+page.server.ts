import { apiFetch } from '$lib/server/api';
import { captchaToken, loadCaptchaConfig } from '$lib/server/captcha';
import { setSessionCookies } from '$lib/server/session';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

interface LoginResponse {
	access_token: string;
	refresh_token: string;
	user: {
		id: number;
		email: string;
		role: 'admin' | 'staff' | 'client';
		client_id: number;
		name?: string;
	};
}

/** Only allow same-origin relative redirect targets. */
function safeRedirect(target: string | null, fallback: string, origin: string): string {
	if (!target || !target.startsWith('/') || target.startsWith('//') || target.includes('\\')) {
		return fallback;
	}
	// Belt-and-braces: confirm the WHATWG URL parser (which is what the
	// browser itself uses to resolve the Location header) agrees this
	// resolves to the same origin, not just that it looks like a path.
	try {
		if (new URL(target, origin).origin !== origin) return fallback;
	} catch {
		return fallback;
	}
	return target;
}

function homeFor(role: string): string {
	return role === 'client' ? '/dashboard' : '/admin';
}

export const load: PageServerLoad = async (event) => {
	const { locals, url } = event;
	if (locals.user) {
		redirect(
			303,
			safeRedirect(url.searchParams.get('redirect'), homeFor(locals.user.role), url.origin)
		);
	}
	return { captcha: await loadCaptchaConfig(event) };
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const email = String(form.get('email') ?? '').trim();
		const password = String(form.get('password') ?? '');
		const totpCode = String(form.get('totp_code') ?? '').trim();

		if (!email || !password) {
			return fail(400, { errorKey: 'auth.fillAllFields', email });
		}

		const body: Record<string, string> = { email, password };
		if (totpCode) body.totp_code = totpCode;
		const captcha = captchaToken(form);
		if (captcha) body.captcha_token = captcha;

		const res = await apiFetch<LoginResponse>(event, '/api/v1/auth/login', {
			method: 'POST',
			body,
			token: null
		});

		if (res.error || !res.data) {
			const invalid = res.error?.code === 'UNAUTHORIZED' || res.status === 401;
			return fail(res.status >= 400 ? res.status : 400, {
				errorKey: invalid ? 'auth.invalidCredentials' : undefined,
				errorMessage: invalid ? undefined : (res.error?.message ?? 'Login failed'),
				email
			});
		}

		setSessionCookies(event.cookies, {
			access_token: res.data.access_token,
			refresh_token: res.data.refresh_token
		});

		const target = safeRedirect(
			event.url.searchParams.get('redirect'),
			homeFor(res.data.user?.role ?? 'client'),
			event.url.origin
		);
		redirect(303, target);
	}
};
