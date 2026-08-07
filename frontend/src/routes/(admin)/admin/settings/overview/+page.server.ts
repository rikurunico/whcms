import { apiFetch } from '$lib/server/api';
import type { PageServerLoad } from './$types';
import type { RawSettingsGroups } from '../+page.server';

/**
 * SmartSearch-style "System Settings" landing page (docs/DESIGN.md §7). This route
 * is pure navigation/visual sugar over already-built admin pages - it has no
 * backend surface of its own.
 *
 * The one piece of "real" data it needs is the setup-completion progress bar.
 * Rather than fabricate a static percentage, this reuses the same
 * `/api/v1/admin/settings` endpoint the classic settings page calls, but reads
 * the RAW (pre-default-merge) response: per `internal/service/settings.Service.Grouped`,
 * a group key (company/billing/automation/mail/tickets) is only present in the
 * response once an admin has actually saved that tab at least once. Counting
 * present groups therefore gives a truthful "N of 5 settings areas configured"
 * signal instead of a made-up number.
 */
const SETTINGS_GROUP_KEYS = ['company', 'billing', 'automation', 'mail', 'tickets'] as const;

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<RawSettingsGroups>(event, '/api/v1/admin/settings');
	const raw = res.data;

	const totalGroups = SETTINGS_GROUP_KEYS.length;
	const configuredKeys = raw
		? SETTINGS_GROUP_KEYS.filter((key) => {
				const group = raw[key];
				return !!group && Object.keys(group).length > 0;
			})
		: [];

	return {
		// A 403 (staff without the "settings" permission) or any other API
		// error just means the progress bar reads 0% - the navigation cards
		// below are still useful, so this doesn't hard-error the page.
		listError: res.error ? res.error.message : null,
		configuredGroups: configuredKeys.length,
		configuredKeys,
		totalGroups,
		setupPercent: totalGroups > 0 ? Math.round((configuredKeys.length / totalGroups) * 100) : 0
	};
};
