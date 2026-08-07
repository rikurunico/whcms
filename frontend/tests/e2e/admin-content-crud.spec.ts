import { expect, test, type APIRequestContext } from '@playwright/test';
import { adminToken, clickBtn, newApi, setSessionCookies, unique } from './helpers';

/**
 * Admin content CRUD (HostPanel theme): create -> edit -> delete for
 * announcements and network-status entries, create a knowledgebase article,
 * and exercise the registrar "Test connection" action. These admin write
 * flows previously had no dedicated e2e coverage; per CLAUDE.md §5 every new
 * admin feature/flow ships its own Playwright spec.
 *
 * Runs against the already-live stack (frontend :5173, api :8080,
 * mockserver :9090). Unique, timestamp-suffixed names keep it safe under the
 * shared, never-reset backend + DB; only this spec's own rows are asserted on.
 */

test.describe('admin content CRUD', () => {
	let api: APIRequestContext;
	let token: string;

	test.beforeAll(async () => {
		api = await newApi();
		token = await adminToken(api);
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test.beforeEach(async ({ context }) => {
		await setSessionCookies(context, token);
	});

	test('announcement: create, edit, then delete', async ({ page }) => {
		test.setTimeout(60_000);
		const title = unique('E2E Announcement');

		// --- create: default action redirects to the edit page ---------------
		await page.goto('/admin/announcements/new');
		await page.waitForLoadState('networkidle');
		await page.getByTestId('announcement-form').locator('[name="title"]').fill(title);
		await page.getByTestId('announcement-form').locator('[name="body"]').fill('Body created by e2e.');
		await clickBtn(page, 'announcement-form-submit');

		await expect(page).toHaveURL(/\/admin\/announcements\/\d+/, { timeout: 15_000 });
		await expect(page.getByTestId('announcement-created-alert')).toBeVisible();
		await expect(page.getByTestId('announcement-title')).toHaveText(title);

		// --- edit: rename + save; the heading reflects the persisted title ----
		const newTitle = unique('E2E Announcement Edited');
		await page.getByTestId('announcement-form').locator('[name="title"]').fill(newTitle);
		await clickBtn(page, 'announcement-form-submit');
		await expect(page.getByTestId('announcement-title')).toHaveText(newTitle, { timeout: 15_000 });

		// --- delete via the confirm modal -> back to the list ------------------
		await clickBtn(page, 'announcement-delete-button');
		const dialog = page.getByRole('dialog');
		await expect(dialog).toBeVisible();
		await dialog.getByRole('button', { name: /^Delete$/ }).click();
		await expect(page).toHaveURL(/\/admin\/announcements(\?|$)/, { timeout: 15_000 });
		await expect(page.getByText(newTitle)).toHaveCount(0);
	});

	test('network-status: create, edit, then delete', async ({ page }) => {
		test.setTimeout(60_000);
		const title = unique('E2E Network');

		await page.goto('/admin/network-status/new');
		await page.waitForLoadState('networkidle');
		await page.getByTestId('network-form').locator('[name="title"]').fill(title);
		await page.getByTestId('network-form').locator('[name="body"]').fill('Investigating an e2e incident.');
		await clickBtn(page, 'network-form-submit');

		await expect(page).toHaveURL(/\/admin\/network-status\/\d+/, { timeout: 15_000 });
		await expect(page.getByTestId('network-created-alert')).toBeVisible();
		await expect(page.getByTestId('network-title')).toHaveText(title);

		const newTitle = unique('E2E Network Edited');
		await page.getByTestId('network-form').locator('[name="title"]').fill(newTitle);
		await clickBtn(page, 'network-form-submit');
		await expect(page.getByTestId('network-title')).toHaveText(newTitle, { timeout: 15_000 });

		await clickBtn(page, 'network-delete-button');
		const dialog = page.getByRole('dialog');
		await expect(dialog).toBeVisible();
		await dialog.getByRole('button', { name: /^Delete$/ }).click();
		await expect(page).toHaveURL(/\/admin\/network-status(\?|$)/, { timeout: 15_000 });
	});

	test('knowledgebase: create an article', async ({ page }) => {
		test.setTimeout(60_000);
		const title = unique('E2E KB Article');

		await page.goto('/admin/knowledgebase/articles/new');
		await page.waitForLoadState('networkidle');

		const form = page.getByTestId('kb-article-form');
		const category = form.locator('[name="category_id"]');
		// The category <select> starts on a disabled placeholder; pick the first
		// real category (option value is the numeric id).
		const values = await category
			.locator('option')
			.evaluateAll((opts) => opts.map((o) => (o as HTMLOptionElement).value).filter((v) => /^\d+$/.test(v)));
		expect(values.length, 'at least one KB category must exist').toBeGreaterThan(0);
		await category.selectOption(values[0]);
		await form.locator('[name="title"]').fill(title);
		await clickBtn(page, 'kb-article-form-submit');

		await expect(page).toHaveURL(/\/admin\/knowledgebase\/articles\/\d+/, { timeout: 15_000 });
	});

	test('registrar: test connection returns a result', async ({ page }) => {
		test.setTimeout(45_000);
		await page.goto('/admin/registrars');
		await page.waitForLoadState('networkidle');

		const testBtn = page.locator('[data-testid^="registrar-test-"]').first();
		await expect(testBtn, 'at least one registrar module should be listed').toBeVisible({ timeout: 15_000 });
		const tid = await testBtn.getAttribute('data-testid');
		const id = tid!.replace('registrar-test-', '');
		await testBtn.click();
		await expect(page.getByTestId(`registrar-test-result-${id}`)).toBeVisible({ timeout: 20_000 });
	});

	// Registrar credentials are now dynamic (encrypted DB storage, admin-
	// configurable, no restart needed) instead of env-only - this drives the
	// real save/reload/clear flow and confirms it round-trips through the API,
	// never exposing the key itself.
	test('registrar: reseller id + API key save encrypted, persist, and clear', async ({ page }) => {
		test.setTimeout(45_000);
		await page.goto('/admin/registrars');
		await page.waitForLoadState('networkidle');

		const configureBtn = page.locator('[data-testid^="registrar-configure-"]').first();
		await expect(configureBtn, 'at least one registrar module should be listed').toBeVisible({ timeout: 15_000 });
		const tid = await configureBtn.getAttribute('data-testid');
		const id = tid!.replace('registrar-configure-', '');
		await configureBtn.click();

		const resellerID = unique('reseller');
		const apiKey = unique('apikey');
		await page.getByTestId(`registrar-reseller-id-${id}`).fill(resellerID);
		await page.getByTestId(`registrar-api-key-${id}`).fill(apiKey);
		await clickBtn(page, `registrar-save-${id}`);
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		// Regression check: right after a successful save (no reload), the form
		// must still show what was just typed, not blank out - SvelteKit's
		// use:enhance update() resets bound inputs to blank by default unless
		// called with {reset: false}.
		await expect(page.getByTestId(`registrar-reseller-id-${id}`)).toHaveValue(resellerID);

		// Persisted server-side: re-fetch via the API (not just local UI state).
		const res = await api.get(`http://localhost:8080/api/v1/admin/registrars/${id}`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(res.ok()).toBeTruthy();
		const saved = (await res.json()).data as { reseller_id: string; api_key_present: boolean };
		expect(saved.reseller_id).toBe(resellerID);
		expect(saved.api_key_present).toBe(true);

		// Reload: the field shows the persisted reseller id; the key is masked
		// (never round-tripped as plaintext) and the "Clear stored key" option
		// is now offered.
		await page.reload();
		await page.waitForLoadState('networkidle');
		await page.getByTestId(`registrar-configure-${id}`).click();
		await expect(page.getByTestId(`registrar-reseller-id-${id}`)).toHaveValue(resellerID);
		await expect(page.getByTestId(`registrar-api-key-${id}`)).toHaveValue('');
		await expect(page.getByTestId(`registrar-clear-api-key-${id}`)).toBeVisible();

		// Clear the stored key: api_key_present stays true because the E2E
		// stack's RDASH_API_KEY env var is still set (DB-or-env), proving the
		// clear only cleared the DB value, not the effective fallback.
		await page.getByTestId(`registrar-clear-api-key-${id}`).check();
		await clickBtn(page, `registrar-save-${id}`);
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const afterClear = await api.get(`http://localhost:8080/api/v1/admin/registrars/${id}`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		const clearedData = (await afterClear.json()).data as { reseller_id: string; api_key_present: boolean };
		expect(clearedData.reseller_id, 'clearing the key must not touch reseller_id').toBe(resellerID);
		expect(clearedData.api_key_present, 'env RDASH_API_KEY is still the fallback').toBe(true);
	});

	// Custom Endpoint (registrars.base_url): lets an admin flip a registrar
	// between the mockserver and a real upstream with no restart. Proven by
	// actual effect, not just persistence: pointing it at an unreachable
	// address breaks Test Connection, and clearing it restores success again
	// (falls back to the E2E stack's RDASH_BASE_URL, the mockserver).
	test('registrar: custom endpoint overrides the live base URL with no restart', async ({ page }) => {
		test.setTimeout(45_000);
		await page.goto('/admin/registrars');
		await page.waitForLoadState('networkidle');

		const configureBtn = page.locator('[data-testid^="registrar-configure-"]').first();
		await expect(configureBtn, 'at least one registrar module should be listed').toBeVisible({ timeout: 15_000 });
		const tid = await configureBtn.getAttribute('data-testid');
		const id = tid!.replace('registrar-configure-', '');
		await configureBtn.click();

		// Point the registrar at an address nothing listens on: Test Connection
		// must now fail, proving the override actually took effect on the live
		// adapter (not just persisted in the DB). Gate on the ?/save response
		// itself, not the "Registrar saved" toast - both saves in this test
		// produce the identical toast text, and toasts don't dismiss instantly,
		// so a toast-based wait can match a stale one left over from the prior
		// save and race ahead of this save's actual completion.
		await page.getByTestId(`registrar-base-url-${id}`).fill('http://127.0.0.1:1/v1');
		await Promise.all([
			page.waitForResponse((r) => r.url().includes('/admin/registrars') && r.url().includes('?/save')),
			clickBtn(page, `registrar-save-${id}`)
		]);

		const saved = await api.get(`http://localhost:8080/api/v1/admin/registrars/${id}`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect((await saved.json()).data.base_url).toBe('http://127.0.0.1:1/v1');

		await clickBtn(page, `registrar-test-${id}`);
		const resultBox = page.getByTestId(`registrar-test-result-${id}`);
		await expect(resultBox).toBeVisible({ timeout: 20_000 });
		await expect(resultBox.locator('.hp-alert-red')).toBeVisible();

		// Clear the override: falls back to the env-configured mockserver, so
		// Test Connection must succeed again.
		await page.getByTestId(`registrar-base-url-${id}`).fill('');
		await expect(page.getByTestId(`registrar-base-url-${id}`)).toHaveValue('');
		await Promise.all([
			page.waitForResponse((r) => r.url().includes('/admin/registrars') && r.url().includes('?/save')),
			clickBtn(page, `registrar-save-${id}`)
		]);

		const cleared = await api.get(`http://localhost:8080/api/v1/admin/registrars/${id}`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect((await cleared.json()).data.base_url).toBe('');

		await clickBtn(page, `registrar-test-${id}`);
		await expect(resultBox.locator('.hp-info')).toBeVisible({ timeout: 20_000 });
	});

	// Duitku gateway API key: same dynamic/encrypted pattern as the registrar.
	test('gateways: Duitku API key saves encrypted, persists, and clears', async ({ page }) => {
		test.setTimeout(45_000);
		await page.goto('/admin/gateways');
		await page.waitForLoadState('networkidle');

		const apiKey = unique('duitku-key');
		await page.getByTestId('gateway-api-key').locator('input[type="password"]').fill(apiKey);
		await clickBtn(page, 'gateway-save');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const res = await api.get('http://localhost:8080/api/v1/admin/gateways', {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(res.ok()).toBeTruthy();
		const saved = (await res.json()).data as { duitku: { api_key_set: boolean } };
		expect(saved.duitku.api_key_set).toBe(true);

		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('gateway-api-key').locator('input[type="password"]')).toHaveValue('');
		await expect(page.getByTestId('gateway-clear-api-key')).toBeVisible();

		// Clear the stored key: DUITKU_API_KEY env fallback keeps it "set".
		await page.getByTestId('gateway-clear-api-key').check();
		await clickBtn(page, 'gateway-save');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const afterClear = await api.get('http://localhost:8080/api/v1/admin/gateways', {
			headers: { Authorization: `Bearer ${token}` }
		});
		const clearedData = (await afterClear.json()).data as { duitku: { api_key_set: boolean } };
		expect(clearedData.duitku.api_key_set, 'env DUITKU_API_KEY is still the fallback').toBe(true);
	});

	test('gateways: Duitku Base URL override saves, persists, and clears back to the mode-derived default', async ({
		page
	}) => {
		test.setTimeout(45_000);
		await page.goto('/admin/gateways');
		await page.waitForLoadState('networkidle');

		// This env's live gateway calls must keep hitting the local mockserver
		// for every other payment spec in this suite to keep working - set a
		// throwaway-but-valid override, prove it round-trips, then clear it
		// back to blank (mode-derived / DUITKU_BASE_URL env fallback) rather
		// than leaving a stray override behind for later specs.
		const override = 'https://sandbox.duitku.com';
		await page.getByTestId('gateway-base-url').locator('input').fill(override);
		await clickBtn(page, 'gateway-save');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const res = await api.get('http://localhost:8080/api/v1/admin/gateways', {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(res.ok()).toBeTruthy();
		const saved = (await res.json()).data as { duitku: { base_url: string } };
		expect(saved.duitku.base_url).toBe(override);

		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('gateway-base-url').locator('input')).toHaveValue(override);

		// Clear it back to blank so downstream specs keep hitting the mockserver.
		await page.getByTestId('gateway-base-url').locator('input').fill('');
		await clickBtn(page, 'gateway-save');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const afterClear = await api.get('http://localhost:8080/api/v1/admin/gateways', {
			headers: { Authorization: `Bearer ${token}` }
		});
		const clearedData = (await afterClear.json()).data as { duitku: { base_url: string } };
		expect(clearedData.duitku.base_url).toBe('');
	});

	test('gateways: Manual bank-transfer accounts save, persist, and clear', async ({ page }) => {
		test.setTimeout(45_000);
		await page.goto('/admin/gateways');
		await page.waitForLoadState('networkidle');

		await page.getByTestId('gateway-manual-enabled').locator('input[type="checkbox"]').check();
		await page
			.getByTestId('gateway-manual-bank-name-0')
			.fill('BCA');
		await page.getByTestId('gateway-manual-account-number-0').fill('1234567890');
		await page.getByTestId('gateway-manual-account-holder-0').fill('PT WHCMS Hosting');

		await page.getByTestId('gateway-manual-add-account').click();
		await page.getByTestId('gateway-manual-bank-name-1').fill('Mandiri');
		await page.getByTestId('gateway-manual-account-number-1').fill('0987654321');
		await page.getByTestId('gateway-manual-account-holder-1').fill('PT WHCMS Hosting');

		await page
			.getByTestId('gateway-manual-instructions')
			.locator('textarea')
			.fill('Include the invoice number in your transfer note.');

		await clickBtn(page, 'gateway-manual-save');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const res = await api.get('http://localhost:8080/api/v1/admin/gateways', {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(res.ok()).toBeTruthy();
		const saved = (await res.json()).data as {
			manual: {
				enabled: boolean;
				accounts: Array<{ bank_name: string; account_number: string; account_holder: string }>;
				instructions: string;
			};
		};
		expect(saved.manual.enabled).toBe(true);
		expect(saved.manual.accounts).toHaveLength(2);
		expect(saved.manual.accounts[0].bank_name).toBe('BCA');
		expect(saved.manual.accounts[1].bank_name).toBe('Mandiri');
		expect(saved.manual.instructions).toBe('Include the invoice number in your transfer note.');

		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('gateway-manual-bank-name-0')).toHaveValue('BCA');
		await expect(page.getByTestId('gateway-manual-bank-name-1')).toHaveValue('Mandiri');
		await expect(
			page.getByTestId('gateway-manual-enabled').locator('input[type="checkbox"]')
		).toBeChecked();

		// Clean up: disable + clear accounts so downstream specs (payment method
		// aggregation) don't see a stray "Bank Transfer" option left enabled.
		await page.getByTestId('gateway-manual-enabled').locator('input[type="checkbox"]').uncheck();
		await page.getByTestId('gateway-manual-remove-account-1').click();
		await page.getByTestId('gateway-manual-bank-name-0').fill('');
		await page.getByTestId('gateway-manual-account-number-0').fill('');
		await page.getByTestId('gateway-manual-account-holder-0').fill('');
		await clickBtn(page, 'gateway-manual-save');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const afterClear = await api.get('http://localhost:8080/api/v1/admin/gateways', {
			headers: { Authorization: `Bearer ${token}` }
		});
		const clearedData = (await afterClear.json()).data as {
			manual: { enabled: boolean; accounts: unknown[] };
		};
		expect(clearedData.manual.enabled).toBe(false);
		expect(clearedData.manual.accounts).toHaveLength(0);
	});
});
