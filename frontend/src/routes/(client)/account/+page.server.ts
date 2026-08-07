import { apiFetch, type ApiError } from '$lib/server/api';
import { fail, type RequestEvent } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const TABS = ['profile', 'security', 'contacts', 'credit'] as const;
export type AccountTab = (typeof TABS)[number];

/** Client profile fields - JSON tags per backend/internal/domain/entities.go (Client). */
interface ClientProfileFields {
	first_name: string;
	last_name: string;
	company: string;
	address1: string;
	address2: string;
	city: string;
	state: string;
	postcode: string;
	country: string;
	phone: string;
}

/**
 * GET /api/v1/auth/me response. The backend may return the client profile either
 * flattened on the user object or nested under `client` - both are tolerated.
 */
interface MeResponse extends Partial<ClientProfileFields> {
	id: number;
	email: string;
	role: 'admin' | 'staff' | 'client';
	client_id: number;
	twofa_enabled?: boolean;
	email_verified_at?: string | null;
	credit_balance?: number;
	client?: (Partial<ClientProfileFields> & { credit_balance?: number }) | null;
	// /auth/me nests the account under `user` (twofa_enabled lives here).
	user?: { twofa_enabled?: boolean } | null;
}

/** client_contacts row (entities.go ClientContact) + optional permissions map. */
interface ClientContact {
	id: number;
	client_id: number;
	first_name: string;
	last_name: string;
	email: string;
	phone: string;
	permissions?: Record<string, boolean> | null;
	created_at?: string;
}

/** credit_ledger row (entities.go CreditLedgerEntry). */
interface CreditLedgerEntry {
	id: number;
	client_id: number;
	delta: number;
	balance_after: number;
	reason: string;
	related_invoice_id: number | null;
	created_at: string;
}

interface TwoFASetupResponse {
	secret?: string;
	otpauth_url?: string;
	otpauth_uri?: string;
	uri?: string;
	url?: string;
}

function profileOf(me: MeResponse | null | undefined) {
	const flat: Partial<ClientProfileFields> = me ?? {};
	const nested: Partial<ClientProfileFields> & { credit_balance?: number } = me?.client ?? {};
	const pick = (key: keyof ClientProfileFields): string => {
		const v = nested[key] ?? flat[key];
		return typeof v === 'string' ? v : '';
	};
	return {
		email: me?.email ?? '',
		first_name: pick('first_name'),
		last_name: pick('last_name'),
		company: pick('company'),
		address1: pick('address1'),
		address2: pick('address2'),
		city: pick('city'),
		state: pick('state'),
		postcode: pick('postcode'),
		country: pick('country') || 'ID',
		phone: pick('phone'),
		credit_balance: nested.credit_balance ?? me?.credit_balance ?? 0
	};
}

/** Map backend VALIDATION error details ([{field, message}]) to per-field messages. */
function detailErrors(error: ApiError | null): Record<string, string> {
	const out: Record<string, string> = {};
	for (const d of error?.details ?? []) {
		if (d && typeof d === 'object') {
			const item = d as { field?: unknown; message?: unknown; error?: unknown };
			if (typeof item.field === 'string' && item.field) {
				out[item.field] =
					typeof item.message === 'string' && item.message
						? item.message
						: typeof item.error === 'string' && item.error
							? item.error
							: (error?.message ?? 'clientcore.validation.required');
			}
		}
	}
	return out;
}

const str = (form: FormData, key: string): string => String(form.get(key) ?? '').trim();

async function fetchMe(event: RequestEvent) {
	return apiFetch<MeResponse>(event, '/api/v1/auth/me');
}

export const load: PageServerLoad = async (event) => {
	const tabParam = event.url.searchParams.get('tab') ?? 'profile';
	const tab: AccountTab = (TABS as readonly string[]).includes(tabParam)
		? (tabParam as AccountTab)
		: 'profile';

	// The tab fetches depend only on URL params - run them concurrently with
	// /auth/me instead of serially after it.
	const mePromise = fetchMe(event);
	const contactsPromise =
		tab === 'contacts'
			? apiFetch<ClientContact[]>(event, '/api/v1/account/contacts', {
					query: { page: 1, per_page: 100 }
				})
			: null;
	const page = Math.max(1, Number(event.url.searchParams.get('page')) || 1);
	const perPage = Math.min(1000, Math.max(1, Number(event.url.searchParams.get('per_page')) || 10));
	const creditPromise =
		tab === 'credit'
			? apiFetch<
					| CreditLedgerEntry[]
					| { balance?: number; ledger?: CreditLedgerEntry[]; entries?: CreditLedgerEntry[] }
				>(event, '/api/v1/account/credit', { query: { page, per_page: perPage } })
			: null;

	const me = await mePromise;
	const profile = profileOf(me.data);

	let contacts: ClientContact[] = [];
	let contactsError: string | null = null;
	let ledger: CreditLedgerEntry[] = [];
	let ledgerMeta = { page: 1, per_page: 10, total: 0 };
	let ledgerError: string | null = null;
	let creditBalance = profile.credit_balance;

	if (contactsPromise) {
		const res = await contactsPromise;
		contacts = res.data ?? [];
		contactsError = res.error?.message ?? null;
	}

	if (creditPromise) {
		const res = await creditPromise;
		const payload = res.data;
		if (Array.isArray(payload)) {
			ledger = payload;
		} else if (payload && typeof payload === 'object') {
			ledger = payload.ledger ?? payload.entries ?? [];
			if (typeof payload.balance === 'number') creditBalance = payload.balance;
		}
		ledgerMeta = {
			page: res.meta?.page ?? page,
			per_page: res.meta?.per_page ?? perPage,
			total: res.meta?.total ?? ledger.length
		};
		ledgerError = res.error?.message ?? null;
	}

	return {
		tab,
		profile,
		twofaEnabled: me.data?.user?.twofa_enabled ?? me.data?.twofa_enabled ?? false,
		meError: me.error?.message ?? null,
		contacts,
		contactsError,
		ledger,
		ledgerMeta,
		ledgerError,
		creditBalance
	};
};

export const actions: Actions = {
	updateProfile: async (event) => {
		const form = await event.request.formData();
		const values = {
			first_name: str(form, 'first_name'),
			last_name: str(form, 'last_name'),
			company: str(form, 'company'),
			address1: str(form, 'address1'),
			address2: str(form, 'address2'),
			city: str(form, 'city'),
			state: str(form, 'state'),
			postcode: str(form, 'postcode'),
			country: str(form, 'country').toUpperCase() || 'ID',
			phone: str(form, 'phone')
		};

		const errors: Record<string, string> = {};
		for (const key of [
			'first_name',
			'last_name',
			'address1',
			'city',
			'state',
			'postcode',
			'phone'
		] as const) {
			if (!values[key]) errors[key] = 'clientcore.validation.required';
		}
		if (Object.keys(errors).length > 0) {
			return fail(400, {
				profileErrors: errors,
				profileMessage: undefined as string | undefined,
				profileValues: values
			});
		}

		// PATCH /account/profile (clients.UpdateProfile), NOT /auth/me - the
		// latter never persisted address1/address2/city/state/postcode/country
		// at all (UpdateMeRequest never declared them, so Fiber's JSON binding
		// silently dropped every one of these fields on every save).
		const res = await apiFetch(event, '/api/v1/account/profile', { method: 'PATCH', body: values });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				profileErrors: detailErrors(res.error),
				profileMessage: res.error.message as string | undefined,
				profileValues: values
			});
		}
		return { profileSaved: true };
	},

	changePassword: async (event) => {
		const form = await event.request.formData();
		const current = String(form.get('current_password') ?? '');
		const next = String(form.get('new_password') ?? '');
		const confirm = String(form.get('confirm_password') ?? '');

		const errors: Record<string, string> = {};
		if (!current) errors.current_password = 'clientcore.validation.required';
		if (next.length < 8) errors.new_password = 'clientcore.validation.passwordMin';
		if (next !== confirm) errors.confirm_password = 'clientcore.validation.passwordMismatch';
		if (Object.keys(errors).length > 0) {
			return fail(400, {
				passwordErrors: errors,
				passwordMessage: undefined as string | undefined
			});
		}

		const res = await apiFetch(event, '/api/v1/auth/me', {
			method: 'PATCH',
			body: { current_password: current, new_password: next }
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				passwordErrors: detailErrors(res.error),
				passwordMessage: res.error.message as string | undefined
			});
		}
		return { passwordSaved: true };
	},

	setup2fa: async (event) => {
		const res = await apiFetch<TwoFASetupResponse>(event, '/api/v1/auth/2fa/setup', {
			method: 'POST'
		});
		if (res.error || !res.data) {
			return fail(res.status >= 400 ? res.status : 400, {
				twofaMessage: res.error?.message ?? 'clientcore.validation.generic'
			});
		}
		return {
			twofaSetup: {
				secret: res.data.secret ?? '',
				otpauth: res.data.otpauth_url ?? res.data.otpauth_uri ?? res.data.uri ?? res.data.url ?? ''
			}
		};
	},

	enable2fa: async (event) => {
		const form = await event.request.formData();
		const code = str(form, 'totp_code');
		if (!code) {
			return fail(400, { twofaCodeError: 'clientcore.validation.codeRequired' });
		}
		const res = await apiFetch(event, '/api/v1/auth/2fa/enable', {
			method: 'POST',
			body: { code }
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, { twofaMessage: res.error.message });
		}
		return { twofaEnabled: true };
	},

	disable2fa: async (event) => {
		const form = await event.request.formData();
		const code = str(form, 'totp_code');
		const password = str(form, 'password');
		if (!code) {
			return fail(400, { twofaCodeError: 'clientcore.validation.codeRequired' });
		}
		const res = await apiFetch(event, '/api/v1/auth/2fa/disable', {
			method: 'POST',
			body: { code, password }
		});
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, { twofaMessage: res.error.message });
		}
		return { twofaDisabled: true };
	},

	saveContact: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id') ?? 0) || 0;
		const values = {
			id,
			first_name: str(form, 'first_name'),
			last_name: str(form, 'last_name'),
			email: str(form, 'email'),
			phone: str(form, 'phone'),
			perm_invoices: form.get('perm_invoices') === 'on',
			perm_services: form.get('perm_services') === 'on',
			perm_domains: form.get('perm_domains') === 'on',
			perm_tickets: form.get('perm_tickets') === 'on'
		};

		const errors: Record<string, string> = {};
		if (!values.first_name) errors.first_name = 'clientcore.validation.required';
		if (!values.email) errors.email = 'clientcore.validation.required';
		else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(values.email)) {
			errors.email = 'clientcore.validation.invalidEmail';
		}
		if (Object.keys(errors).length > 0) {
			return fail(400, {
				contactErrors: errors,
				contactMessage: undefined as string | undefined,
				contactValues: values
			});
		}

		const body = {
			first_name: values.first_name,
			last_name: values.last_name,
			email: values.email,
			phone: values.phone,
			permissions: {
				invoices: values.perm_invoices,
				services: values.perm_services,
				domains: values.perm_domains,
				tickets: values.perm_tickets
			}
		};

		const res = id
			? await apiFetch(event, `/api/v1/account/contacts/${id}`, { method: 'PATCH', body })
			: await apiFetch(event, '/api/v1/account/contacts', { method: 'POST', body });

		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, {
				contactErrors: detailErrors(res.error),
				contactMessage: res.error.message as string | undefined,
				contactValues: values
			});
		}
		return { contactSaved: true };
	},

	deleteContact: async (event) => {
		const form = await event.request.formData();
		const id = Number(form.get('id') ?? 0) || 0;
		if (!id) {
			return fail(400, { contactMessage: 'clientcore.validation.invalidContact' });
		}
		const res = await apiFetch(event, `/api/v1/account/contacts/${id}`, { method: 'DELETE' });
		if (res.error) {
			return fail(res.status >= 400 ? res.status : 400, { contactMessage: res.error.message });
		}
		return { contactDeleted: true };
	}
};
