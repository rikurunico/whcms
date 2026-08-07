import { expect, test } from './fixtures';
import { adminToken, authHeaders, newApi, registerVerifyLogin, setSessionCookies } from './helpers';

const API_BASE = 'http://localhost:8080';

/**
 * The manual bank-transfer gateway, end to end: client picks "Bank Transfer"
 * at checkout, sees the admin-configured account instructions inline (no
 * external gateway involved), and an admin confirms receipt through the real
 * HostPanel transactions UI (`POST /admin/transactions/:id/confirm`) - which
 * settles the exact pending row and marks the invoice paid.
 *
 * Setup (register/verify/login, invoice creation, gateway config) is driven
 * directly against the live backend via API calls, same rationale as
 * client-manage.spec.ts - this spec's own job is verifying the two UI
 * surfaces (client instructions, admin confirm button), not re-driving
 * checkout through the browser.
 *
 * The invoice is a plain ADMIN-CREATED manual invoice (POST /admin/invoices,
 * not a product order) at a randomized, never-before-seen amount -
 * deliberately NOT the seeded hosting product's fixed price. GetPaymentMethods
 * caches its aggregated method list per invoice AMOUNT for 10 minutes
 * (service.go methodsCacheTTL); the shared e2e suite runs many hosting orders
 * at that same fixed price, so reusing it here would routinely hit a stale
 * cache entry populated by an earlier spec (before this spec enabled the
 * manual gateway) and never see "Bank Transfer" appear.
 */

test.describe('client checkout: manual bank transfer, confirmed by admin', () => {
	test('client sees bank account instructions; admin confirms via the UI; invoice reaches paid', async ({
		page,
		browser
	}) => {
		test.setTimeout(60_000);

		const api = await newApi();
		const admin = await adminToken(api);

		// Configure the manual gateway - test setup, not itself under test
		// (that's covered by admin-content-crud.spec.ts's gateways test).
		const configureRes = await api.put(`${API_BASE}/api/v1/admin/gateways/manual`, {
			headers: authHeaders(admin),
			data: {
				enabled: true,
				accounts: [
					{ bank_name: 'BCA', account_number: '1234567890', account_holder: 'PT WHCMS Hosting' }
				],
				instructions: 'Include the invoice number in your transfer note.'
			}
		});
		expect(configureRes.ok(), `configure manual gateway: ${await configureRes.text()}`).toBeTruthy();

		try {
			const client = await registerVerifyLogin(api, 'bank-transfer');

			const clientsRes = await api.get(`${API_BASE}/api/v1/admin/clients`, {
				headers: authHeaders(admin),
				params: { search: client.email }
			});
			expect(clientsRes.ok(), `find client: ${await clientsRes.text()}`).toBeTruthy();
			const clients = ((await clientsRes.json()).data ?? []) as Array<{ id: number }>;
			expect(clients, `exactly one client matching ${client.email}`).toHaveLength(1);
			const clientId = clients[0].id;

			// A randomized amount, guaranteed distinct from the fixed-price
			// hosting/domain products every other spec in this suite orders at
			// - see the file doc comment for why this matters.
			const amount = 500_000 + (Date.now() % 100_000);
			const invoiceRes = await api.post(`${API_BASE}/api/v1/admin/invoices`, {
				headers: authHeaders(admin),
				data: {
					client_id: clientId,
					items: [{ description: 'E2E bank-transfer test charge', amount, taxed: false }]
				}
			});
			expect(invoiceRes.ok(), `create manual invoice: ${await invoiceRes.text()}`).toBeTruthy();
			const invoiceId = ((await invoiceRes.json()).data as { id: number }).id;

			// --- Client: pick Bank Transfer, see instructions inline ---------------
			await setSessionCookies(page.context(), client.accessToken, client.refreshToken);
			await page.goto(`/billing/invoices/${invoiceId}`);
			await page.waitForLoadState('networkidle');

			await expect(page.getByTestId('payment-methods')).toBeVisible();
			await page.getByTestId('payment-method-bank_transfer').click();
			await page.getByTestId('pay-button').click();

			await expect(page.getByTestId('payment-instructions')).toBeVisible({ timeout: 10_000 });
			await expect(page.getByTestId('payment-bank-account-0')).toBeVisible();
			await expect(page.getByTestId('payment-bank-account-number-0')).toHaveText('1234567890');
			await expect(page.getByTestId('payment-note')).toBeVisible();
			await expect(page.getByTestId('payment-reference')).toBeVisible();
			// No redirect to any gateway - settled entirely on our own page.
			expect(page.url()).toMatch(new RegExp(`/billing/invoices/${invoiceId}$`));

			// --- Locate the pending manual transaction (test glue) -----------------
			const txRes = await api.get(`${API_BASE}/api/v1/admin/transactions`, {
				headers: authHeaders(admin),
				params: { gateway: 'manual', status: 'pending', per_page: 1000 }
			});
			expect(txRes.ok()).toBeTruthy();
			const txs = ((await txRes.json()).data ?? []) as Array<{ id: number; invoice_id: number }>;
			const tx = txs.find((t) => t.invoice_id === invoiceId);
			expect(tx, 'pending manual transaction for this invoice should exist').toBeTruthy();

			// --- Admin: confirm the payment through the real HostPanel UI ----------
			const adminContext = await browser.newContext();
			try {
				await setSessionCookies(adminContext, admin);
				const adminPage = await adminContext.newPage();
				await adminPage.goto('/admin/transactions?gateway=manual&status=pending');
				await adminPage.waitForLoadState('networkidle');
				await adminPage.getByTestId(`confirm-payment-${tx!.id}`).click();
				await adminPage.getByTestId('confirm-payment-submit').click();
				await expect(adminPage.getByRole('status')).toBeVisible({ timeout: 10_000 });
			} finally {
				await adminContext.close();
			}

			// --- Invoice settles -----------------------------------------------------
			await expect
				.poll(
					async () => {
						const res = await api.get(`${API_BASE}/api/v1/invoices/${invoiceId}`, {
							headers: authHeaders(client.accessToken)
						});
						const body = (await res.json()).data as {
							invoice?: { status?: string };
							status?: string;
						};
						return body.invoice?.status ?? body.status ?? null;
					},
					{ timeout: 20_000, message: 'waiting for the invoice to be marked paid' }
				)
				.toBe('paid');
		} finally {
			// Clean up: disable + clear the manual gateway so downstream specs
			// (payment method aggregation) don't see a stray "Bank Transfer"
			// option left enabled.
			await api.put(`${API_BASE}/api/v1/admin/gateways/manual`, {
				headers: authHeaders(admin),
				data: { enabled: false, accounts: [], instructions: '' }
			});
			await api.dispose();
		}
	});
});
