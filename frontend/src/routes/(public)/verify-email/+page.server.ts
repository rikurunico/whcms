import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

type VerifyStatus = 'ok' | 'missing_token' | 'invalid_token' | 'error';

export const load: PageServerLoad = async (event) => {
	// Post-verification landing (redirected below): refresh-safe success state
	// that never re-POSTs the already-consumed token.
	if (event.url.searchParams.get('verified') === '1') {
		return { status: 'ok' as VerifyStatus, message: null };
	}

	const token = event.url.searchParams.get('token');
	if (!token) {
		return { status: 'missing_token' as VerifyStatus, message: null };
	}

	// The user lands here from the emailed link (GET); verify immediately server-side.
	const res = await apiFetch(event, '/api/v1/auth/verify-email', {
		method: 'POST',
		body: { token },
		token: null
	});

	if (res.error) {
		// 400/404/410 -> invalid or expired token; anything else -> generic API error.
		const invalid = res.status === 400 || res.status === 404 || res.status === 410;
		return {
			status: (invalid ? 'invalid_token' : 'error') as VerifyStatus,
			message: invalid ? null : res.error.message
		};
	}

	// Success -> redirect so the token leaves the URL and a reload stays on success.
	redirect(303, '/verify-email?verified=1');
};

export const actions: Actions = {
	resend: async (event) => {
		const form = await event.request.formData();
		const email = String(form.get('email') ?? '').trim();
		if (!email) {
			return fail(400, { resendErrorKey: 'feauth.validation.required', email });
		}

		const res = await apiFetch(event, '/api/v1/auth/resend-verification', {
			method: 'POST',
			body: { email },
			token: null
		});

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 500, {
				resendErrorKey: 'feauth.register.resendFailed',
				email
			});
		}
		return { resent: true, email };
	}
};
