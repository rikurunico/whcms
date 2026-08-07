/**
 * Minimal rune-based i18n helper.
 *
 * Usage in components:
 *   import { t, i18n } from '$lib/i18n';
 *   {t('nav.dashboard')}  {t('common.welcome', { name: user.name })}
 *
 * The locale is resolved server-side from the `locale` cookie (hooks.server.ts ->
 * locals.locale -> root layout data) and applied via `i18n.setLocale(...)` before render.
 * Client-side switching persists back to the cookie.
 */
import { browser } from '$app/environment';
import en from './en';
import id from './id';

export type Locale = 'id' | 'en';
export const DEFAULT_LOCALE: Locale = 'id';
export const LOCALES: readonly Locale[] = ['id', 'en'] as const;

type Dict = { [key: string]: string | Dict };

/**
 * Feature namespace dictionaries: every `src/lib/i18n/messages/<name>.ts`
 * default-exporting `{ id: {...}, en: {...} }` is deep-merged over the base
 * dictionaries at startup. Feature agents add THEIR OWN file there and never
 * edit the shared id.ts / en.ts.
 */
const featureModules = import.meta.glob('./messages/*.ts', { eager: true }) as Record<
	string,
	{ default?: Partial<Record<Locale, Dict>> }
>;

function deepMerge(target: Dict, src: Dict): Dict {
	for (const [k, v] of Object.entries(src)) {
		const existing = target[k];
		if (typeof v === 'object' && v !== null && typeof existing === 'object' && existing !== null) {
			deepMerge(existing, v);
		} else {
			target[k] = v;
		}
	}
	return target;
}

const dictionaries: Record<Locale, Dict> = { id: id as Dict, en: en as Dict };
for (const mod of Object.values(featureModules)) {
	const d = mod?.default;
	if (!d) continue;
	for (const loc of LOCALES) {
		const overlay = d[loc];
		if (overlay) deepMerge(dictionaries[loc], overlay);
	}
}

function lookup(dict: Dict, key: string): string | undefined {
	let node: unknown = dict;
	for (const part of key.split('.')) {
		if (typeof node !== 'object' || node === null) return undefined;
		node = (node as Record<string, unknown>)[part];
	}
	return typeof node === 'string' ? node : undefined;
}

function interpolate(template: string, params?: Record<string, string | number>): string {
	if (!params) return template;
	return template.replace(/\{(\w+)\}/g, (match, name: string) =>
		name in params ? String(params[name]) : match
	);
}

class I18n {
	locale = $state<Locale>(DEFAULT_LOCALE);

	/** Set the active locale. Persists to the `locale` cookie on the client unless persist=false. */
	setLocale(locale: Locale, opts: { persist?: boolean } = {}): void {
		if (!LOCALES.includes(locale)) return;
		this.locale = locale;
		if (browser && (opts.persist ?? true)) {
			document.cookie = `locale=${locale}; path=/; max-age=31536000; samesite=lax`;
		}
	}

	/** Translate a dot-separated key with optional {param} interpolation. */
	t = (key: string, params?: Record<string, string | number>): string => {
		const template =
			lookup(dictionaries[this.locale], key) ?? lookup(dictionaries[DEFAULT_LOCALE], key) ?? key;
		return interpolate(template, params);
	};
}

export const i18n = new I18n();

/** Reactive translate function - re-evaluates when the locale changes. */
export const t = i18n.t;
