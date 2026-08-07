import { expect, test, type Page } from './fixtures';
import { adminToken } from './helpers';

/**
 * Admin server "Test Connection" + auto-fill, and the edit-form regression.
 *
 * Covers the WHMCS-style provisioning-server workflow (docs: CLAUDE.md §4):
 *   1. On "Add Server", entering module + hostname + credentials and clicking
 *      "Test Connection" runs a READ-ONLY probe (WHM `version`) - no account
 *      required - and auto-fills the nameservers the panel reports, showing
 *      "Connection successful … Some values have been auto-filled." This is the
 *      fix for the old "NOT_FOUND: cpanel account not found" false failure.
 *   2. The server edit form loads populated and STAYS populated after saving
 *      (regression: fields used to blank out until a manual refresh).
 *
 * The probe targets the local mockserver's WHM API (localhost:9090, no SSL),
 * which answers `version`/`gethostname`/`get_nameserver_config`.
 */

const API_BASE = 'http://localhost:8080';
const ADMIN_EMAIL = 'admin@e2e.test';
const ADMIN_PASSWORD = 'AdminE2E!2026';

interface Envelope<T> {
	data: T | null;
	error: { code: string; message: string } | null;
}

async function uiLogin(page: Page): Promise<void> {
	await page.goto('/login');
	await page.waitForLoadState('networkidle');
	await page.locator('input[name="email"]').fill(ADMIN_EMAIL);
	await page.locator('input[name="password"]').fill(ADMIN_PASSWORD);
	await page.locator('button[type="submit"]').click();
	await page.waitForURL(/\/admin/, { timeout: 15000 });
}

test.describe('admin server Test Connection + edit form', () => {
	test('Test Connection probes the panel and auto-fills nameservers', async ({ page }) => {
		await uiLogin(page);

		await page.goto('/admin/servers/new');
		await page.waitForLoadState('networkidle');

		await page.locator('[data-testid="server-field-name"]').fill('E2E Probe Target');
		await page.locator('[data-testid="server-field-module"]').selectOption('cpanel');
		// Uncheck SSL first (it re-derives the port), then pin the mock port.
		const ssl = page.locator('[data-testid="server-field-use_ssl"]');
		if (await ssl.isChecked()) await ssl.uncheck();
		await page.locator('[data-testid="server-field-hostname"]').fill('localhost');
		await page.locator('[data-testid="server-field-port"]').fill('9090');
		await page.locator('[data-testid="server-field-username"]').fill('e2eadmin');
		await page.locator('[data-testid="server-field-api_token"]').fill('e2e-probe-token');

		await page.locator('[data-testid="server-test-button"] button').click();

		const result = page.locator('[data-testid="server-test-result"]');
		await expect(result).toBeVisible({ timeout: 15000 });
		await expect(result).toContainText('Connection successful');
		await expect(result).toContainText('auto-filled');

		// Nameservers reported by the mock are auto-populated into blank slots.
		await expect(page.locator('[data-testid="server-field-nameserver1"]')).toHaveValue(
			'ns1.mock.local'
		);
		await expect(page.locator('[data-testid="server-field-nameserver2"]')).toHaveValue(
			'ns2.mock.local'
		);
	});

	test('Test Connection works with username + password (HTTP Basic auth)', async ({ page }) => {
		await uiLogin(page);
		await page.goto('/admin/servers/new');
		await page.waitForLoadState('networkidle');

		await page.locator('[data-testid="server-field-name"]').fill('E2E Basic Auth');
		await page.locator('[data-testid="server-field-module"]').selectOption('cpanel');
		const ssl = page.locator('[data-testid="server-field-use_ssl"]');
		if (await ssl.isChecked()) await ssl.uncheck();
		await page.locator('[data-testid="server-field-hostname"]').fill('localhost');
		await page.locator('[data-testid="server-field-port"]').fill('9090');
		await page.locator('[data-testid="server-field-username"]').fill('e2eadmin');
		// Password only - NO API token. Used to 403; now authenticates via Basic.
		await page.locator('[data-testid="server-field-password"]').fill('e2e-password');

		await page.locator('[data-testid="server-test-button"] button').click();

		const result = page.locator('[data-testid="server-test-result"]');
		await expect(result).toBeVisible({ timeout: 15000 });
		await expect(result).toContainText('Connection successful');
	});

	test('Servers and Server Groups tables paginate at 10 per page', async ({ page, request }) => {
		const token = await adminToken(request);
		// Seed 11 groups so the groups table has >1 page regardless of what other
		// specs do concurrently (they don't touch server groups).
		const ts = Date.now();
		const createdIds: number[] = [];
		for (let i = 0; i < 11; i++) {
			const res = await request.post(`${API_BASE}/api/v1/admin/server-groups`, {
				headers: { Authorization: `Bearer ${token}` },
				data: { name: `E2E Pager Group ${ts} #${i}`, strategy: 'round_robin' }
			});
			const env = (await res.json()) as Envelope<{ id: number }>;
			if (env.data?.id) createdIds.push(env.data.id);
		}
		expect(createdIds.length).toBe(11);

		try {
			await uiLogin(page);
			await page.goto('/admin/servers');
			await page.waitForLoadState('networkidle');

			// Servers table (server-side, per_page=10): never more than 10 rows,
			// pager rendered. Exclude the group rows, whose testid also starts with
			// "row-server-".
			const serverRows = page.locator(
				'[data-testid^="row-server-"]:not([data-testid^="row-server-group-"])'
			);
			expect(await serverRows.count()).toBeLessThanOrEqual(10);
			await expect(page.locator('[data-testid="servers-pager"]')).toBeVisible();

			// Groups table (client-side slice): exactly 10 on page 1 (>=11 exist).
			const groupRows = page.locator('[data-testid^="row-server-group-"]');
			await expect(groupRows).toHaveCount(10);
			const groupsPager = page.locator('[data-testid="server-groups-pager"]');
			await expect(groupsPager).toBeVisible();

			// Next groups page (?gpage=2) shows the remainder, still capped at 10.
			await groupsPager.getByRole('button', { name: /Next Page/ }).click();
			await expect(page).toHaveURL(/gpage=2/);
			const n = await groupRows.count();
			expect(n).toBeGreaterThan(0);
			expect(n).toBeLessThanOrEqual(10);
		} finally {
			for (const id of createdIds) {
				await request.delete(`${API_BASE}/api/v1/admin/server-groups/${id}`, {
					headers: { Authorization: `Bearer ${token}` }
				});
			}
		}
	});

	test('edit form loads populated and stays populated after saving', async ({ page, request }) => {
		const token = await adminToken(request);
		const name = `E2E Edit Server ${Date.now()}`;
		const create = await request.post(`${API_BASE}/api/v1/admin/servers`, {
			headers: { Authorization: `Bearer ${token}` },
			data: {
				name,
				module: 'cpanel',
				hostname: 'localhost',
				port: 9090,
				username: 'e2eadmin',
				api_token: 'e2e-edit-token',
				use_ssl: false
			}
		});
		const env = (await create.json()) as Envelope<{ id: number }>;
		expect(create.status(), await create.text()).toBe(201);
		const id = env.data!.id;

		await uiLogin(page);
		await page.goto(`/admin/servers/${id}`);
		await page.waitForLoadState('networkidle');

		// Loads populated (the bug: these were blank until a manual refresh).
		await expect(page.locator('[data-testid="server-field-name"]')).toHaveValue(name);
		await expect(page.locator('[data-testid="server-field-hostname"]')).toHaveValue('localhost');

		// Saving must not blank the bound fields.
		await page.locator('[data-testid="server-save-button"] button').click();
		await expect(page.locator('[data-testid="server-field-name"]')).toHaveValue(name);
		await expect(page.locator('[data-testid="server-field-hostname"]')).toHaveValue('localhost');

		// Cleanup.
		await request.delete(`${API_BASE}/api/v1/admin/servers/${id}`, {
			headers: { Authorization: `Bearer ${token}` }
		});
	});

	// Package name prefix: real WHM reseller accounts commonly require every
	// package name to carry the reseller's own prefix - a bare "whcms_s<id>"
	// fails createacct there. Found 2026-07-27 against a live reseller server.
	test('package name prefix saves, persists, and clears back to blank', async ({
		page,
		request
	}) => {
		const token = await adminToken(request);
		const name = `E2E Prefix Server ${Date.now()}`;
		const create = await request.post(`${API_BASE}/api/v1/admin/servers`, {
			headers: { Authorization: `Bearer ${token}` },
			data: {
				name,
				module: 'cpanel',
				hostname: 'localhost',
				port: 9090,
				username: 'e2eadmin',
				api_token: 'e2e-prefix-token',
				use_ssl: false
			}
		});
		const env = (await create.json()) as Envelope<{ id: number }>;
		expect(create.status(), await create.text()).toBe(201);
		const id = env.data!.id;

		await uiLogin(page);
		await page.goto(`/admin/servers/${id}`);
		await page.waitForLoadState('networkidle');

		await expect(page.locator('[data-testid="server-field-package_prefix"]')).toHaveValue('');
		await page.locator('[data-testid="server-field-package_prefix"]').fill('reseller_');
		await Promise.all([
			page.waitForResponse((r) => r.url().includes('?/save') && r.status() === 200),
			page.locator('[data-testid="server-save-button"] button').click()
		]);

		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.locator('[data-testid="server-field-package_prefix"]')).toHaveValue(
			'reseller_'
		);

		// Clearing it back to blank also persists (not just non-blank values).
		// Gate the reload on the save's own response (not the field's in-place
		// bound value, which reset:false keeps showing regardless of whether
		// the PATCH has actually landed yet) - reloading too early re-reads the
		// still-unsaved "reseller_" from the backend.
		await page.locator('[data-testid="server-field-package_prefix"]').fill('');
		await Promise.all([
			page.waitForResponse((r) => r.url().includes('?/save') && r.status() === 200),
			page.locator('[data-testid="server-save-button"] button').click()
		]);
		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.locator('[data-testid="server-field-package_prefix"]')).toHaveValue('');

		// Cleanup.
		await request.delete(`${API_BASE}/api/v1/admin/servers/${id}`, {
			headers: { Authorization: `Bearer ${token}` }
		});
	});
});
