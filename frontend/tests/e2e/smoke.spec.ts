import { expect, test } from '@playwright/test';

test('root renders the public portal for anonymous visitors, login still reachable', async ({
	page
}) => {
	// The root '/' now serves the Twenty-One public portal (no longer a redirect
	// to /login) for anonymous visitors.
	await page.goto('/');
	await expect(page.getByTestId('portal-home')).toBeVisible();

	// Login remains reachable and functional.
	await page.goto('/login');
	await expect(page.locator('input[name="email"]')).toBeVisible();
	await expect(page.locator('input[name="password"]')).toBeVisible();
	await expect(page.locator('button[type="submit"]')).toBeVisible();
});
