import { apiFetch } from '$lib/server/api';
import { error, fail, type ActionFailure, type RequestEvent } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/**
 * Grouped settings shape returned by GET/PUT /api/v1/admin/settings
 * (internal/service/settings.Service.Grouped - groups by the key prefix before the
 * first '.', e.g. "billing.tax_rate" -> group "billing", name "tax_rate"). Raw API
 * response - any group/key may be absent until first written.
 */
export interface RawSettingsGroups {
	company?: {
		name?: string;
		logo_key?: string;
		address?: string;
		email?: string;
	};
	billing?: {
		tax_enabled?: boolean;
		tax_rate?: number;
		tax_inclusive?: boolean;
		invoice_due_days?: number;
		renewal_lead_days?: number;
		late_fee_enabled?: boolean;
		late_fee_amount?: number;
		reminder_days?: number[];
		overdue_reminder_days?: number[];
	};
	automation?: {
		suspend_after_days?: number;
		terminate_after_days?: number;
	};
	mail?: {
		from_name?: string;
		from_email?: string;
	};
	tickets?: {
		allowed_extensions?: string[];
		max_attachment_mb?: number;
	};
	security?: {
		captcha_enabled?: boolean;
		captcha_provider?: string;
		captcha_site_key?: string;
		require_email_verification?: boolean;
	};
}

/** Same shape, fully populated with CONTRACTS.md §10 defaults so the view never
 *  has to null-check. */
export interface SettingsGroups {
	company: Required<NonNullable<RawSettingsGroups['company']>>;
	billing: Required<NonNullable<RawSettingsGroups['billing']>>;
	automation: Required<NonNullable<RawSettingsGroups['automation']>>;
	mail: Required<NonNullable<RawSettingsGroups['mail']>>;
	tickets: Required<NonNullable<RawSettingsGroups['tickets']>>;
	security: Required<NonNullable<RawSettingsGroups['security']>>;
}

const DEFAULTS: SettingsGroups = {
	company: { name: '', logo_key: '', address: '', email: '' },
	billing: {
		tax_enabled: false,
		tax_rate: 11,
		tax_inclusive: false,
		invoice_due_days: 3,
		renewal_lead_days: 14,
		late_fee_enabled: false,
		late_fee_amount: 0,
		reminder_days: [7, 3, 1],
		overdue_reminder_days: [1, 3, 7]
	},
	automation: { suspend_after_days: 7, terminate_after_days: 21 },
	mail: { from_name: '', from_email: '' },
	tickets: {
		allowed_extensions: ['jpg', 'jpeg', 'png', 'gif', 'pdf', 'txt', 'zip'],
		max_attachment_mb: 8
	},
	security: {
		captcha_enabled: false,
		captcha_provider: 'turnstile',
		captcha_site_key: '',
		require_email_verification: true
	}
};

function merge(raw: RawSettingsGroups | null): SettingsGroups {
	return {
		company: { ...DEFAULTS.company, ...raw?.company },
		billing: { ...DEFAULTS.billing, ...raw?.billing },
		automation: { ...DEFAULTS.automation, ...raw?.automation },
		mail: { ...DEFAULTS.mail, ...raw?.mail },
		tickets: { ...DEFAULTS.tickets, ...raw?.tickets },
		security: { ...DEFAULTS.security, ...raw?.security }
	};
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<RawSettingsGroups>(event, '/api/v1/admin/settings');
	// Staff without the "settings" permission get a 403 FORBIDDEN from the API
	// (RequirePermission("settings")) - render SvelteKit's error page instead of a
	// half-populated settings form the user can't actually load or save.
	if (res.status === 403) {
		error(403, res.error?.message || 'Forbidden');
	}
	return {
		settings: merge(res.data),
		listError: res.error?.message ?? null
	};
};

/** Mirrors internal/service/settings.Kind (the PUT body value-type whitelist). */
type Kind = 'string' | 'int' | 'number' | 'bool' | 'int_list' | 'string_list';

const KIND: Record<string, Kind> = {
	'company.name': 'string',
	'company.logo_key': 'string',
	'company.address': 'string',
	'company.email': 'string',
	'billing.tax_enabled': 'bool',
	'billing.tax_rate': 'number',
	'billing.tax_inclusive': 'bool',
	'billing.invoice_due_days': 'int',
	'billing.renewal_lead_days': 'int',
	'billing.late_fee_enabled': 'bool',
	'billing.late_fee_amount': 'int',
	'billing.reminder_days': 'int_list',
	'billing.overdue_reminder_days': 'int_list',
	'automation.suspend_after_days': 'int',
	'automation.terminate_after_days': 'int',
	'mail.from_name': 'string',
	'mail.from_email': 'string',
	'tickets.allowed_extensions': 'string_list',
	'tickets.max_attachment_mb': 'int',
	'security.captcha_enabled': 'bool',
	'security.captcha_provider': 'string',
	'security.captcha_site_key': 'string',
	'security.require_email_verification': 'bool'
};

const GENERAL_KEYS = [
	'company.name',
	'company.logo_key',
	'company.address',
	'company.email'
] as const;
const BILLING_KEYS = [
	'billing.tax_enabled',
	'billing.tax_rate',
	'billing.tax_inclusive',
	'billing.invoice_due_days',
	'billing.renewal_lead_days',
	'billing.late_fee_enabled',
	'billing.late_fee_amount',
	'billing.reminder_days',
	'billing.overdue_reminder_days'
] as const;
const AUTOMATION_KEYS = [
	'automation.suspend_after_days',
	'automation.terminate_after_days'
] as const;
const MAIL_KEYS = ['mail.from_name', 'mail.from_email'] as const;
const TICKETS_KEYS = ['tickets.allowed_extensions', 'tickets.max_attachment_mb'] as const;
const SECURITY_KEYS = [
	'security.captcha_enabled',
	'security.captcha_provider',
	'security.captcha_site_key',
	'security.require_email_verification'
] as const;

function coerceNumber(raw: FormDataEntryValue | null): number {
	const n = Number(raw ?? 0);
	return Number.isFinite(n) ? n : 0;
}

function coerceIntList(raw: FormDataEntryValue | null): number[] {
	return String(raw ?? '')
		.split(',')
		.map((s) => s.trim())
		.filter((s) => s.length > 0)
		.map((s) => Math.round(Number(s)))
		.filter((n) => Number.isInteger(n));
}

function coerceStrList(raw: FormDataEntryValue | null): string[] {
	return String(raw ?? '')
		.split(',')
		.map((s) => s.trim())
		.filter((s) => s.length > 0);
}

function coerce(key: string, raw: FormDataEntryValue | null): unknown {
	switch (KIND[key]) {
		case 'bool':
			return raw === 'on' || raw === 'true';
		case 'number':
			return coerceNumber(raw);
		case 'int':
			return Math.round(coerceNumber(raw));
		case 'int_list':
			return coerceIntList(raw);
		case 'string_list':
			return coerceStrList(raw);
		default:
			return String(raw ?? '').trim();
	}
}

function buildUpdates(form: FormData, keys: readonly string[]): Record<string, unknown> {
	const updates: Record<string, unknown> = {};
	for (const key of keys) updates[key] = coerce(key, form.get(key));
	return updates;
}

/** Map backend VALIDATION error details ([{field, message}]) to a per-key error map
 *  (field == the dotted settings key, e.g. "billing.tax_rate"). */
function fieldErrorsFrom(details: unknown[]): Record<string, string> {
	const out: Record<string, string> = {};
	for (const d of details) {
		if (d && typeof d === 'object') {
			const rec = d as Record<string, unknown>;
			const field = typeof rec.field === 'string' ? rec.field : undefined;
			const message = typeof rec.message === 'string' ? rec.message : undefined;
			if (field) out[field] = message ?? 'invalid';
		}
	}
	return out;
}

/**
 * One shape for every action on this page. Keeping it uniform (rather than a
 * per-action union) is what lets the view read `form.testEmailError` without
 * narrowing which action produced it.
 */
interface SettingsActionData {
	tab: string;
	saved?: boolean;
	errorMessage?: string;
	fieldErrors?: Record<string, string>;
	testEmailSent?: boolean;
	testEmailTo?: string;
	testEmailError?: string;
}

type SettingsActionResult = SettingsActionData | ActionFailure<SettingsActionData>;

async function saveTab(
	event: RequestEvent,
	keys: readonly string[],
	tab: string
): Promise<SettingsActionResult> {
	const form = await event.request.formData();
	const updates = buildUpdates(form, keys);

	const res = await apiFetch<RawSettingsGroups>(event, '/api/v1/admin/settings', {
		method: 'PUT',
		body: updates
	});

	if (res.error) {
		return fail<SettingsActionData>(res.status >= 400 ? res.status : 500, {
			tab,
			errorMessage: res.error.message,
			fieldErrors: fieldErrorsFrom(res.error.details)
		});
	}
	return { saved: true, tab } satisfies SettingsActionData;
}

/** notifications.TestEmailResult. */
interface TestEmailResult {
	email_log_id: number;
	to: string;
	subject: string;
}

/**
 * Sends a diagnostic email through the real mailer. The backend sends it
 * synchronously (not via the mail:send queue), so an SMTP misconfiguration
 * comes back here as an EXTERNAL error whose message carries the relay's own
 * words - surface it verbatim, that is the point of the check.
 */
async function sendTestEmail(event: RequestEvent): Promise<SettingsActionResult> {
	const form = await event.request.formData();
	const to = String(form.get('test_email_to') ?? '').trim();
	if (!to) {
		return fail<SettingsActionData>(400, {
			tab: 'mail',
			testEmailError: 'Enter a recipient address.'
		});
	}

	const res = await apiFetch<TestEmailResult>(event, '/api/v1/admin/email/test', {
		method: 'POST',
		body: { to }
	});

	if (res.error) {
		return fail<SettingsActionData>(res.status >= 400 ? res.status : 500, {
			tab: 'mail',
			testEmailTo: to,
			testEmailError: res.error.message
		});
	}
	return { tab: 'mail', testEmailTo: to, testEmailSent: true } satisfies SettingsActionData;
}

export const actions: Actions = {
	saveGeneral: (event) => saveTab(event, GENERAL_KEYS, 'general'),
	saveBilling: (event) => saveTab(event, BILLING_KEYS, 'billing'),
	saveAutomation: (event) => saveTab(event, AUTOMATION_KEYS, 'automation'),
	saveMail: (event) => saveTab(event, MAIL_KEYS, 'mail'),
	saveTickets: (event) => saveTab(event, TICKETS_KEYS, 'tickets'),
	saveSecurity: (event) => saveTab(event, SECURITY_KEYS, 'security'),
	sendTestEmail
};
