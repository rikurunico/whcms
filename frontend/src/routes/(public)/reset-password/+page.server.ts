import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const PASSWORD_MIN = 8;

export const load: PageServerLoad = ({ url }) => {
	return { token: url.searchParams.get('token') };
};

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const token = String(form.get('token') ?? '').trim();
		const password = String(form.get('password') ?? '');
		const confirmPassword = String(form.get('confirm_password') ?? '');

		if (!token) {
			return fail(400, { errorKey: 'feauth.reset.missingToken' });
		}

		const fieldErrors: Record<string, string> = {};
		if (!password) {
			fieldErrors.password = 'feauth.validation.required';
		} else if (password.length < PASSWORD_MIN) {
			fieldErrors.password = 'feauth.validation.passwordMin';
		}
		if (password && confirmPassword !== password) {
			fieldErrors.confirm_password = 'feauth.validation.passwordMismatch';
		}
		if (Object.keys(fieldErrors).length > 0) {
			return fail(400, { fieldErrors });
		}

		const res = await apiFetch(event, '/api/v1/auth/reset-password', {
			method: 'POST',
			body: { token, password },
			token: null
		});

		if (res.error) {
			// 400/404/410 -> invalid or expired token.
			const invalid = res.status === 400 || res.status === 404 || res.status === 410;
			return fail(res.status >= 400 ? res.status : 500, {
				errorKey: invalid ? 'feauth.reset.failedExpired' : undefined,
				errorMessage: invalid ? undefined : res.error.message,
				tokenRejected: invalid
			});
		}

		redirect(303, '/login');
	}
};
