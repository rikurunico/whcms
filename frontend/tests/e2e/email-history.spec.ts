import { expect, test, type APIRequestContext } from '@playwright/test';
import { newApi, registerVerifyLogin, setSessionCookies, type Client } from './helpers';

/**
 * Client-facing email delivery history (`/account/email-history`).
 * registerVerifyLogin already sends a real verify_email through
 * notifications.SendTemplate, which is tied to the new client's user id - so
 * by the time the client logs in, at least one row of their own history
 * already exists with no further setup needed.
 */

test.describe('client email history', () => {
	let api: APIRequestContext;
	let client: Client;

	test.beforeAll(async () => {
		test.setTimeout(120_000);
		api = await newApi();
		client = await registerVerifyLogin(api, 'emailhistory');
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test.beforeEach(async ({ context }) => {
		await setSessionCookies(context, client.accessToken, client.refreshToken);
	});

	test('shows the verification email sent at registration and filters by status', async ({
		page
	}) => {
		await page.goto('/account/email-history');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('email-history-status-filter')).toBeVisible({
			timeout: 15_000
		});
		await expect(page.getByTestId('email-history-list')).toBeVisible();
		await expect(page.locator('[data-testid^="row-email-"]').first()).toBeVisible();

		// Filtering to "sent" must still show the (already-delivered) verify
		// email row, driving the load function's status query param.
		await page.getByRole('link', { name: 'Sent', exact: true }).click();
		await expect(page).toHaveURL(/status=sent$/);
		await expect(page.locator('[data-testid^="row-email-"]').first()).toBeVisible();

		// A status with no matching rows shows the empty state instead of a stale list.
		await page.getByRole('link', { name: 'Failed', exact: true }).click();
		await expect(page).toHaveURL(/status=failed$/);
		await expect(page.locator('[data-testid^="row-email-"]')).toHaveCount(0);
	});

	test('another client cannot see this client’s email history', async ({
		page,
		context
	}) => {
		test.setTimeout(120_000);
		const other = await registerVerifyLogin(api, 'emailhistoryother');
		await setSessionCookies(context, other.accessToken, other.refreshToken);

		await page.goto('/account/email-history');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('email-history-list')).toBeVisible();
		// The other client only has their own verify_email row, never this
		// suite's client's, so no row should reference the shared fixture.
		const rows = page.locator('[data-testid^="row-email-"]');
		await expect(rows).toHaveCount(1);
	});
});
