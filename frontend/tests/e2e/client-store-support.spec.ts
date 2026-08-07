import { expect, test, type APIRequestContext } from '@playwright/test';
import { newApi, registerVerifyLogin, setSessionCookies } from './helpers';

/**
 * Two user-facing surfaces that had no dedicated e2e coverage: the public
 * store landing at /order (product catalog + store nav) and the client support
 * ticket *list* at /support (the ticket detail + new-ticket flows are already
 * covered by support-ticket.spec.ts). Per CLAUDE.md §5 every new frontend flow
 * ships its own Playwright spec. Runs against the already-live stack.
 */

test.describe('store landing + client support list', () => {
	let api: APIRequestContext;

	test.beforeAll(async () => {
		api = await newApi();
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test('public store landing lists products and store nav', async ({ page }) => {
		test.setTimeout(45_000);
		await page.goto('/order');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('product-list')).toBeVisible({ timeout: 15_000 });
		await expect(page.getByTestId('product-groups')).toBeVisible();
		await expect(page.getByTestId('domain-search-link')).toBeVisible();

		// WHMCS-style: /order with no ?group= implicitly lands on the first
		// category (no flat "all products" list) - the sidebar highlights it
		// and the heading names it, not a generic catalog title.
		const firstCategory = page.locator('[data-testid="product-groups"] a').first();
		await expect(firstCategory).toHaveClass(/ca-list-item--active/);
		const firstCategoryName = (await firstCategory.textContent())?.trim();
		await expect(page.locator('h1')).toHaveText(firstCategoryName ?? '');

		// Switching category (the seeded "E2E Hosting" group) re-filters the
		// grid to just that group's products and moves the active state.
		await page.getByTestId('product-group-e2e-hosting').click();
		await expect(page).toHaveURL(/\?group=e2e-hosting$/);
		await expect(page.locator('h1')).toHaveText('E2E Hosting');
		await expect(page.getByTestId('product-group-e2e-hosting')).toHaveClass(/ca-list-item--active/);
		await expect(page.getByTestId('product-card-e2e-shared-hosting')).toBeVisible();
	});

	test('client support list renders and links to a new ticket', async ({ page, context }) => {
		test.setTimeout(120_000);
		const client = await registerVerifyLogin(api, 'supportlist');
		await setSessionCookies(context, client.accessToken);

		await page.goto('/support');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('ticket-status-filter')).toBeVisible({ timeout: 15_000 });
		await expect(page.getByTestId('ticket-list')).toBeVisible();

		await page.getByTestId('new-ticket-button').click();
		await expect(page).toHaveURL(/\/support\/new$/, { timeout: 15_000 });
	});
});
