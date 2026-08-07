import { expect, test } from '@playwright/test';
import {
	createOrder,
	newApi,
	payInvoiceViaMock,
	registerVerifyLogin,
	seededHostingProductId,
	setSessionCookies,
	uniqueDomainLabel
} from './helpers';

/**
 * Client checkout: a pending gateway transaction is resumed on reload
 * instead of silently starting a new one every time - a real bug found
 * 2026-07-27 by re-checking a live invoice: reloading the pay page (or simply
 * re-visiting it) created a BRAND NEW Duitku VA each time, leaving several
 * orphaned pending transactions/VA numbers behind for the same invoice with
 * no way back to whichever one the customer actually intends to pay. Fixed
 * by persisting each pending transaction's expiry (`expires_at`, previously
 * computed once and discarded) and having the invoice page resume the most
 * recent still-valid pending transaction's own instructions on load, with an
 * explicit "Ganti metode pembayaran" escape hatch to actually pick a
 * different channel.
 */

test.describe('client checkout: pending payment resumes on reload instead of starting over', () => {
	test('reload shows the SAME VA instructions; switch method starts a new one; cancel returns to the original', async ({
		page
	}) => {
		test.setTimeout(60_000);

		const api = await newApi();
		const client = await registerVerifyLogin(api, 'pay-resume');
		const hostingId = await seededHostingProductId(api);
		const domain = `${uniqueDomainLabel()}.pay-resume.e2e.test`;
		const invoiceId = await createOrder(api, client.accessToken, [
			{ item_type: 'product', product_id: hostingId, cycle: 'monthly', domain }
		]);

		await setSessionCookies(page.context(), client.accessToken, client.refreshToken);
		await page.goto(`/billing/invoices/${invoiceId}`);
		await page.waitForLoadState('networkidle');

		// --- Pick a VA method -------------------------------------------------------------
		await expect(page.getByTestId('payment-methods')).toBeVisible();
		await page.getByTestId('payment-method-BC').click();
		await page.getByTestId('pay-button').click();
		await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
		const firstVaNumber = await page.getByTestId('payment-va-number').textContent();
		expect(firstVaNumber, 'a VA number must actually be rendered').toBeTruthy();

		// --- Reload: must resume the SAME pending transaction, not start a new one ---------
		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByTestId('payment-va-number')).toHaveText(firstVaNumber ?? '');
		// The method picker must NOT reappear on its own.
		await expect(page.getByTestId('payment-methods')).toHaveCount(0);

		// --- Explicit "Ganti metode pembayaran" reveals the picker again -------------------
		await page.getByTestId('payment-switch-method').click();
		await expect(page.getByTestId('payment-methods')).toBeVisible();
		await expect(page.getByTestId('payment-cancel-switch-method')).toBeVisible();

		// --- Cancelling goes back to the original instructions, unchanged ------------------
		await page.getByTestId('payment-cancel-switch-method').click();
		await expect(page.getByTestId('payment-instructions')).toBeVisible();
		await expect(page.getByTestId('payment-va-number')).toHaveText(firstVaNumber ?? '');

		// --- Actually switching to a different (QRIS) channel replaces the instructions ----
		await page.getByTestId('payment-switch-method').click();
		await page.getByTestId('payment-method-SP').click();
		await page.getByTestId('pay-button').click();
		await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByTestId('payment-qr-image')).toBeVisible();
		await expect(page.getByTestId('payment-va-number')).toHaveCount(0);

		// --- Reload again: now resumes the NEWEST pending transaction (QRIS) --------------
		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByTestId('payment-qr-image')).toBeVisible();
		await expect(page.getByTestId('payment-va-number')).toHaveCount(0);

		// Clean up: actually settle the invoice so it doesn't sit unpaid forever.
		await payInvoiceViaMock(api, client.accessToken, invoiceId, 'BC');
		await api.dispose();
	});
});
