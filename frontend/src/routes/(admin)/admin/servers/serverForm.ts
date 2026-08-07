/** Shared parsing for the server create/edit form (FE-ADMIN-OPS). */

/** Parse the shared server form fields into the API body. Returns null body when invalid. */
export function parseServerForm(form: FormData): {
	body: Record<string, unknown> | null;
	errorKey?: string;
} {
	const name = String(form.get('name') ?? '').trim();
	const module = String(form.get('module') ?? '').trim();
	const hostname = String(form.get('hostname') ?? '').trim();
	const username = String(form.get('username') ?? '').trim();
	const port = Number(form.get('port')) || 0;

	if (!name || !module || !hostname || !username || port <= 0 || port > 65535) {
		return { body: null, errorKey: 'adminops.servers.fillRequired' };
	}

	const groupIdRaw = String(form.get('group_id') ?? '').trim();
	const password = String(form.get('password') ?? '');
	const apiToken = String(form.get('api_token') ?? '');

	const body: Record<string, unknown> = {
		name,
		module,
		hostname,
		port,
		username,
		use_ssl: form.get('use_ssl') === 'on',
		nameserver1: String(form.get('nameserver1') ?? '').trim(),
		nameserver2: String(form.get('nameserver2') ?? '').trim(),
		nameserver3: String(form.get('nameserver3') ?? '').trim(),
		nameserver4: String(form.get('nameserver4') ?? '').trim(),
		max_accounts: Math.max(0, Number(form.get('max_accounts')) || 0),
		package_prefix: String(form.get('package_prefix') ?? '').trim(),
		ip_address: String(form.get('ip_address') ?? '').trim(),
		active: form.get('active') === 'on',
		group_id: groupIdRaw ? Number(groupIdRaw) : null
	};
	// Secrets are write-only: send only when (re)entered.
	if (password) body.password = password;
	if (apiToken) body.api_token = apiToken;

	return { body };
}

/**
 * Parse the shared server form into a pre-save Test-Connection request body
 * (provisioning.TestConnectionInput). Unlike {@link parseServerForm} it does
 * not require `name` (a connection test only needs where + how to connect), and
 * carries the optional `id` so the backend can fill blank secrets from the
 * stored (encrypted) row when re-testing an existing server.
 */
export function parseTestConnectionForm(form: FormData, id?: number): Record<string, unknown> {
	const password = String(form.get('password') ?? '');
	const apiToken = String(form.get('api_token') ?? '');
	const body: Record<string, unknown> = {
		module: String(form.get('module') ?? '').trim(),
		hostname: String(form.get('hostname') ?? '').trim(),
		port: Number(form.get('port')) || 0,
		username: String(form.get('username') ?? '').trim(),
		use_ssl: form.get('use_ssl') === 'on'
	};
	if (id && id > 0) body.id = id;
	if (password) body.password = password;
	if (apiToken) body.api_token = apiToken;
	return body;
}
