import { expect, test } from './fixtures';
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
 * Client checkout: VA/QRIS payment channels render their instructions inline
 * (VA number, a real scannable QRIS image) instead of redirecting to the
 * gateway's hosted page - added alongside the fix that inverted the
 * `+page.server.ts` `pay` action's precedence (it used to redirect to
 * `payment_url` unconditionally, before ever checking `va_number`/
 * `qr_string`). Channels with neither (e.g. credit card) still redirect -
 * see hosting-order-pay-activate.spec.ts.
 *
 * Setup (register/verify/login, create the order) is driven directly against
 * the live backend via API calls, same rationale as client-manage.spec.ts:
 * faster/more reliable than re-driving the full order flow through the UI,
 * and this spec only needs to verify the invoice-pay page's rendering.
 */

test.describe('client checkout: VA/QRIS render inline instead of redirecting', () => {
	test('picking a VA method shows the VA number; picking QRIS shows a scannable QR image; both with no redirect', async ({
		page
	}) => {
		test.setTimeout(60_000);

		const api = await newApi();
		const client = await registerVerifyLogin(api, 'pay-instructions');
		const hostingId = await seededHostingProductId(api);
		const domain = `${uniqueDomainLabel()}.pay-instructions.e2e.test`;
		const invoiceId = await createOrder(api, client.accessToken, [
			{ item_type: 'product', product_id: hostingId, cycle: 'monthly', domain }
		]);

		await setSessionCookies(page.context(), client.accessToken, client.refreshToken);
		await page.goto(`/billing/invoices/${invoiceId}`);
		await page.waitForLoadState('networkidle');

		// --- VA channel ("BC" = BCA Virtual Account) -------------------------------------
		await expect(page.getByTestId('payment-methods')).toBeVisible();
		await page.getByTestId('payment-method-BC').click();
		await page.getByTestId('pay-button').click();

		await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByTestId('payment-va-number')).toBeVisible();
		await expect(page.getByTestId('payment-qr-string')).toHaveCount(0);
		expect(page.url()).toMatch(new RegExp(`/billing/invoices/${invoiceId}$`));

		// --- QRIS channel ("SP" = QRIS ShopeePay) ----------------------------------------
		// A reload would now resume the BC pending transaction just created above
		// (see payments-resume.spec.ts) instead of showing the picker again, so
		// switch methods explicitly rather than reloading.
		await page.getByTestId('payment-switch-method').click();
		await expect(page.getByTestId('payment-methods')).toBeVisible();
		await page.getByTestId('payment-method-SP').click();
		await page.getByTestId('pay-button').click();

		await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByTestId('payment-va-number')).toHaveCount(0);
		// The raw qrString is inside a <details> disclosure - open it before
		// asserting its content is visible.
		const rawDetails = page.locator('details', { has: page.getByTestId('payment-qr-string') });
		await rawDetails.locator('summary').click();
		await expect(page.getByTestId('payment-qr-string')).toBeVisible();
		await expect(page.getByTestId('payment-qr-image')).toBeVisible();
		const qrImageSrc = await page.getByTestId('payment-qr-image').getAttribute('src');
		expect(qrImageSrc, 'QR image must be a real rendered data URL, not left empty').toMatch(
			/^data:image\/png;base64,/
		);
		expect(page.url()).toMatch(new RegExp(`/billing/invoices/${invoiceId}$`));

		// Clean up: actually settle the invoice so it doesn't sit unpaid forever.
		await payInvoiceViaMock(api, client.accessToken, invoiceId, 'BC');
		await api.dispose();
	});

	test('a channel with no VA/QRIS payload (credit card) still redirects to the gateway hosted page', async ({
		page
	}) => {
		test.setTimeout(60_000);

		const api = await newApi();
		const client = await registerVerifyLogin(api, 'pay-redirect');
		const hostingId = await seededHostingProductId(api);
		const domain = `${uniqueDomainLabel()}.pay-redirect.e2e.test`;
		const invoiceId = await createOrder(api, client.accessToken, [
			{ item_type: 'product', product_id: hostingId, cycle: 'monthly', domain }
		]);

		await setSessionCookies(page.context(), client.accessToken, client.refreshToken);
		await page.goto(`/billing/invoices/${invoiceId}`);
		await page.waitForLoadState('networkidle');

		await expect(page.getByTestId('payment-methods')).toBeVisible();
		// Credit Card sits behind the "show more" toggle (only major VA
		// banks + QRIS are shown expanded by default).
		await page.getByTestId('payment-methods-toggle').click();
		await page.getByTestId('payment-method-VC').click();
		await page.getByTestId('pay-button').click();

		await page.waitForURL(/^http:\/\/localhost:9090\/payment\//, { timeout: 15_000 });

		await api.dispose();
	});

	test('popular VA/QRIS methods show by default; other channels stay collapsed behind "Lihat Lainnya"', async ({
		page
	}) => {
		test.setTimeout(60_000);

		const api = await newApi();
		const client = await registerVerifyLogin(api, 'pay-methods-expand');
		const hostingId = await seededHostingProductId(api);
		const domain = `${uniqueDomainLabel()}.pay-methods-expand.e2e.test`;
		const invoiceId = await createOrder(api, client.accessToken, [
			{ item_type: 'product', product_id: hostingId, cycle: 'monthly', domain }
		]);

		await setSessionCookies(page.context(), client.accessToken, client.refreshToken);
		await page.goto(`/billing/invoices/${invoiceId}`);
		await page.waitForLoadState('networkidle');

		await expect(page.getByTestId('payment-methods')).toBeVisible();
		// Popular VA/QRIS channels (mock catalog: BC, M2, SP) render immediately.
		await expect(page.getByTestId('payment-method-BC')).toBeVisible();
		await expect(page.getByTestId('payment-method-M2')).toBeVisible();
		await expect(page.getByTestId('payment-method-SP')).toBeVisible();

		// Everything else (credit card, e-wallets, retail, paylater) starts collapsed.
		const toggle = page.getByTestId('payment-methods-toggle');
		await expect(toggle).toBeVisible();
		await expect(page.getByTestId('payment-method-VC')).toHaveCount(0);

		await toggle.click();
		await expect(page.getByTestId('payment-method-VC')).toBeVisible();
		await expect(page.getByTestId('payment-method-OV')).toBeVisible();

		await toggle.click();
		await expect(page.getByTestId('payment-method-VC')).toHaveCount(0);

		await api.dispose();
	});
});
