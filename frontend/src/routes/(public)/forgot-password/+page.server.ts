import { apiFetch } from '$lib/server/api';
import { fail } from '@sveltejs/kit';
import type { Actions } from './$types';

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export const actions: Actions = {
	default: async (event) => {
		const form = await event.request.formData();
		const email = String(form.get('email') ?? '').trim();

		if (!email) {
			return fail(400, { fieldErrorKey: 'feauth.validation.required', email });
		}
		if (!EMAIL_RE.test(email)) {
			return fail(400, { fieldErrorKey: 'feauth.validation.invalidEmail', email });
		}

		// Anti-enumeration: always report success regardless of the API outcome.
		await apiFetch(event, '/api/v1/auth/forgot-password', {
			method: 'POST',
			body: { email },
			token: null
		});

		return { success: true, email };
	}
};
