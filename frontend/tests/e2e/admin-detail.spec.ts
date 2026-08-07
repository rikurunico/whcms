import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import {
	adminToken,
	clientWithActiveService,
	newApi,
	setSessionCookies,
	unique
} from './helpers';

/**
 * Admin detail/edit pages render (HostPanel theme). The *list* pages are swept
 * by admin-read.spec.ts; this spec drives one row through to each *detail* page
 * - the surfaces that had no dedicated e2e - plus the "create client" form.
 *
 * A client with an active hosting service (which yields an order + invoice) is
 * provisioned in beforeAll so the clients/orders/invoices/services lists are
 * non-empty on a fresh backend. Runs against the already-live stack.
 *
 * NB: the admin domains *list* is covered by admin-read.spec.ts and the full
 * domain registration flow by domain-order-register.spec.ts; a per-test domain
 * provisioning just to open domains/[id] would add minutes and flakiness for a
 * read-only page, so it is intentionally left to those specs.
 */

/** Open a list page and click through its first detail link. */
async function openFirst(page: Page, listPath: string, linkSelector: string): Promise<void> {
	await page.goto(listPath);
	await page.waitForLoadState('networkidle');
	const link = page.locator(linkSelector).first();
	await expect(link, `expected a detail link on ${listPath}`).toBeVisible({ timeout: 15_000 });
	await link.click();
}

test.describe('admin detail pages', () => {
	let api: APIRequestContext;
	let token: string;

	test.beforeAll(async () => {
		test.setTimeout(180_000);
		api = await newApi();
		token = await adminToken(api);
		// Guarantee ≥1 client, order, invoice and service exist for the sweeps below.
		await clientWithActiveService(api, 'admindetail');
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test.beforeEach(async ({ context }) => {
		await setSessionCookies(context, token);
	});

	test('client detail renders', async ({ page }) => {
		await openFirst(
			page,
			'/admin/clients',
			'a[href^="/admin/clients/"]:not([href$="/new"]):not([href$="export.csv"])'
		);
		await expect(page).toHaveURL(/\/admin\/clients\/\d+/);
		await expect(page.getByTestId('client-name')).toBeVisible({ timeout: 15_000 });
	});

	test('order detail renders', async ({ page }) => {
		await openFirst(page, '/admin/orders', 'a[href^="/admin/orders/"]');
		await expect(page).toHaveURL(/\/admin\/orders\/\d+/);
		await expect(page.getByTestId('order-number')).toBeVisible({ timeout: 15_000 });
	});

	test('invoice detail renders', async ({ page }) => {
		await openFirst(page, '/admin/invoices', 'a[href^="/admin/invoices/"]:not([href$="/new"])');
		await expect(page).toHaveURL(/\/admin\/invoices\/\d+/);
		await expect(page.getByTestId('invoice-status')).toBeVisible({ timeout: 15_000 });
	});

	test('product edit renders', async ({ page }) => {
		await openFirst(page, '/admin/products', 'a[href^="/admin/products/"]:not([href$="/new"])');
		await expect(page).toHaveURL(/\/admin\/products\/\d+/);
		await expect(page.getByTestId('product-title')).toBeVisible({ timeout: 15_000 });
	});

	test('email template editor renders', async ({ page }) => {
		await openFirst(page, '/admin/email-templates', 'a[href^="/admin/email-templates/"]');
		await expect(page).toHaveURL(/\/admin\/email-templates\/[^/]+\/[^/]+/);
		await expect(page.getByTestId('template-form')).toBeVisible({ timeout: 15_000 });
	});

	test('create a new client via the admin form', async ({ page }) => {
		test.setTimeout(60_000);
		await page.goto('/admin/clients/new');
		await page.waitForLoadState('networkidle');

		const email = `${unique('admin-created')}@e2e.test`;
		await page.getByTestId('client-field-email').fill(email);
		await page.getByTestId('client-field-password').fill('Cl1entPass!2026');
		await page.getByTestId('client-field-first_name').fill('Admin');
		await page.getByTestId('client-field-last_name').fill('Created');
		await page.getByTestId('client-create-submit').click();

		// Success redirects to the new client's detail page.
		await expect(page).toHaveURL(/\/admin\/clients\/\d+/, { timeout: 15_000 });
		await expect(page.getByTestId('client-name')).toBeVisible();
	});
});
