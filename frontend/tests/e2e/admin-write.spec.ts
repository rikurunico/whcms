import { expect, test, type APIRequestContext, type Page } from './fixtures';
import { adminToken, clickBtn, newApi, registerVerifyLogin, setSessionCookies, unique } from './helpers';

/**
 * Admin write flows: create (and where clean, delete) catalog + content
 * entities, plus a settings save. Each entity uses a unique name so the suite
 * is safe against the shared, never-reset backend.
 */

/** Fill a testid whether it is the input itself or a wrapper around one. */
async function fillField(page: Page, testid: string, value: string): Promise<void> {
	const el = page.getByTestId(testid);
	const inner = el.locator('input, textarea');
	if (await inner.count()) await inner.first().fill(value);
	else await el.fill(value);
}

test.describe('admin write flows', () => {
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

	test('create and delete a product group', async ({ page }) => {
		const name = unique('E2E Group');
		await page.goto('/admin/product-groups');
		await clickBtn(page, 'group-create-button');
		await expect(page.getByTestId('group-form')).toBeVisible();
		await page.getByTestId('group-form').locator('[name="name"]').fill(name);
		await clickBtn(page, 'group-form-submit');
		await expect(page.getByText(name)).toBeVisible({ timeout: 10_000 });

		const row = page.locator('tr', { hasText: name });
		await row.locator('[data-testid^="group-delete-"] button, [data-testid^="group-delete-"]').first().click();
		// Confirm in the modal dialog (scope avoids the many per-row Delete buttons).
		await page.getByRole('dialog').getByRole('button', { name: /delete/i }).click();
		await expect(page.getByText(name)).toHaveCount(0, { timeout: 10_000 });
	});

	test('create a percentage coupon', async ({ page }) => {
		const code = unique('E2E').toUpperCase().replace(/[^A-Z0-9]/g, '');
		await page.goto('/admin/coupons');
		await clickBtn(page, 'coupon-create-button');
		await expect(page.getByTestId('coupon-form')).toBeVisible();
		await page.getByTestId('coupon-form').locator('[name="code"]').fill(code);
		await page.getByTestId('coupon-form').locator('[name="type"]').selectOption('percentage');
		await page.getByTestId('coupon-form').locator('[name="value"]').fill('10');
		await clickBtn(page, 'coupon-form-submit');
		await expect(page.getByText(code)).toBeVisible({ timeout: 10_000 });
	});

	test('create a support department', async ({ page }) => {
		const name = unique('E2E Dept');
		await page.goto('/admin/departments');
		await clickBtn(page, 'department-create-button');
		await expect(page.getByTestId('department-form')).toBeVisible();
		await page.getByTestId('department-form').locator('[name="name"]').fill(name);
		await page.getByTestId('department-form').locator('[name="email"]').fill(`${unique('dept')}@e2e.test`);
		await clickBtn(page, 'department-save-button');
		await expect(page.getByText(name)).toBeVisible({ timeout: 10_000 });
	});

	test('create a knowledgebase category', async ({ page }) => {
		const name = unique('E2E KB Cat');
		await page.goto('/admin/knowledgebase');
		await clickBtn(page, 'kb-category-create-button');
		await expect(page.getByTestId('kb-category-form')).toBeVisible();
		await page.getByTestId('kb-category-form').locator('[name="name"]').fill(name);
		await clickBtn(page, 'kb-category-form-submit');
		await expect(page.getByText(name)).toBeVisible({ timeout: 10_000 });
	});

	test('create a manual invoice for a client', async ({ page }) => {
		test.setTimeout(90_000);
		// A fresh verified client to bill.
		const client = await registerVerifyLogin(api, 'manualinv');

		await page.goto('/admin/invoices/new');
		await page.waitForLoadState('networkidle');
		await fillField(page, 'invoice-client-search', client.email);
		await clickBtn(page, 'invoice-client-search-submit');

		// The client select fills with matching results; pick our client.
		const select = page.locator('select[name="client_id"]');
		await expect(async () => {
			const labels = await select.locator('option').allInnerTexts();
			expect(labels.some((l) => l.includes(client.email))).toBeTruthy();
		}).toPass({ timeout: 15_000 });
		await select.selectOption({ label: (await select.locator('option').allInnerTexts()).find((l) => l.includes(client.email))! });

		await fillField(page, 'invoice-item-description-0', 'E2E manual line item');
		await fillField(page, 'invoice-item-amount-0', '75000');
		await clickBtn(page, 'invoice-create-submit');

		await page.waitForURL(/\/admin\/invoices\/\d+/, { timeout: 15_000 });
		await expect(page).toHaveURL(/\/admin\/invoices\/\d+/);
	});

	test('save general settings', async ({ page }) => {
		await page.goto('/admin/settings');
		await expect(page.getByTestId('settings-form-general')).toBeVisible();
		await page.getByTestId('settings-form-general').locator('[name="company.address"]').fill(`Jl. E2E ${Date.now().toString().slice(-5)}`);
		await clickBtn(page, 'settings-save-general');
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });
	});
});
