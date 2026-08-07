import { expect, test, type APIRequestContext } from '@playwright/test';
import { API_BASE, loginApi, newApi, registerVerifyLogin, RESET_LINK, waitForMailMatch, withAuthRateLimitRetry, type Client } from './helpers';

/**
 * Password recovery: forgot-password emails a reset link, reset-password sets a
 * new password, and the new password works for login.
 */

test.describe('password recovery', () => {
	let api: APIRequestContext;
	let client: Client;

	test.beforeAll(async () => {
		test.setTimeout(120_000);
		api = await newApi();
		client = await registerVerifyLogin(api, 'recover');
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test('forgot-password → reset-password → login with the new password', async ({ page }) => {
		test.setTimeout(90_000);

		// 1. Request a reset link.
		await page.goto('/forgot-password');
		await page.waitForLoadState('networkidle');
		await page.locator('[name="email"]').fill(client.email);
		await page.getByTestId('forgot-password-submit').locator('button').click();
		await expect(page.getByTestId('forgot-password-success')).toBeVisible({ timeout: 15_000 });

		// 2. Pull the reset token out of the mail sink (match the reset link only).
		const token = await waitForMailMatch(api, client.email, RESET_LINK);

		// 3. Set a new password via the reset form (redirects away on success).
		const newPassword = 'R3setPass!2026';
		await page.goto(`/reset-password?token=${encodeURIComponent(token)}`);
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('reset-password-form')).toBeVisible();
		await page.locator('[name="password"]').fill(newPassword);
		await page.locator('[name="confirm_password"]').fill(newPassword);
		await page.getByTestId('reset-password-submit').locator('button').click();
		await page.waitForURL((url) => !url.pathname.startsWith('/reset-password'), { timeout: 15_000 });

		// 4. The new password works; the old one no longer does.
		const good = await loginApi(api, client.email, newPassword);
		expect(good.accessToken).toBeTruthy();
		const bad = await withAuthRateLimitRetry(() =>
			api.post(`${API_BASE}/api/v1/auth/login`, { data: { email: client.email, password: client.password, captcha_token: 'e2e-dummy-captcha-token' } })
		);
		expect(bad.ok(), 'old password must be rejected after reset').toBeFalsy();
	});

	test('reset-password with an invalid token is rejected', async ({ page }) => {
		await page.goto('/reset-password?token=not-a-real-token');
		await page.waitForLoadState('networkidle');
		await page.locator('[name="password"]').fill('Whatever!2026');
		await page.locator('[name="confirm_password"]').fill('Whatever!2026');
		await page.getByTestId('reset-password-submit').locator('button').click();
		await expect(page.getByTestId('reset-password-error')).toBeVisible({ timeout: 15_000 });
	});
});
