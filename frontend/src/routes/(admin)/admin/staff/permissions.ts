/**
 * Per-module staff permissions (CONTRACTS §3 RBAC - JSONB map on the users row).
 *
 * Kept in a plain module (not `+page.server.ts`) because SvelteKit only allows a
 * fixed set of special exports (`load`, `actions`, page options) from `+page.server.ts` -
 * exporting anything else there breaks the build. Both `+page.server.ts` (form parsing)
 * and `+page.svelte` (checkbox list) import from here instead.
 */
export const PERMISSION_MODULES = [
	'clients',
	'orders',
	'billing',
	'services',
	'domains',
	'support',
	'products',
	'servers',
	'reports',
	'settings',
	'logs',
	'announcements',
	'knowledgebase',
	'network'
] as const;

export type PermissionModule = (typeof PERMISSION_MODULES)[number];

/** permissions is json.RawMessage on the wire - normalize object | JSON string | null. */
export function parsePermissions(raw: unknown): Record<string, boolean> {
	let obj: unknown = raw;
	if (typeof raw === 'string' && raw.trim()) {
		try {
			obj = JSON.parse(raw);
		} catch {
			return {};
		}
	}
	if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) return {};
	const out: Record<string, boolean> = {};
	for (const [k, v] of Object.entries(obj)) out[k] = v === true;
	return out;
}
