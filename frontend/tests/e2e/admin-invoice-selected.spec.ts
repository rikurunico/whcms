import { expect, test, type APIRequestContext } from './fixtures';
import {
	API_BASE,
	adminFindClientId,
	adminToken,
	authHeaders,
	clientWithActiveService,
	newApi,
	setSessionCookies
} from './helpers';

/**
 * "Invoice Selected Items" (client detail -> Services tab): admin checks a
 * service and forces its renewal invoice right now, regardless of
 * billing.renewal_lead_days - mirrors real WHMCS. The behavior this spec
 * exists to prove is the duplicate-invoice guarantee: clicking the action
 * twice for the same selection must create exactly one invoice, the second
 * click reported as skipped ("already invoiced") rather than duplicating -
 * see docs/CONTRACTS.md and internal/modules/billing/service.go
 * (generateServiceRenewalInvoice's row-locked dedupe).
 *
 * Invoice-count verification goes through the CLIENT's own GET /invoices
 * (correctly scoped to their client_id from the JWT) rather than the admin
 * list, since the admin invoices list has no working client_id filter.
 */

async function countClientOwnInvoices(api: APIRequestContext, clientToken: string): Promise<number> {
	const res = await api.get(`${API_BASE}/api/v1/invoices`, {
		headers: authHeaders(clientToken),
		params: { per_page: 1000 }
	});
	expect(res.ok(), `list client invoices: ${await res.text()}`).toBeTruthy();
	const body = await res.json();
	return ((body.data ?? []) as unknown[]).length;
}

test.describe('admin invoice selected items', () => {
	let api: APIRequestContext;
	let token: string;

	test.beforeAll(async () => {
		test.setTimeout(180_000);
		api = await newApi();
		token = await adminToken(api);
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test.beforeEach(async ({ context }) => {
		await setSessionCookies(context, token);
	});

	test('invoicing a selected service creates one invoice; the same selection clicked again is skipped, not duplicated', async ({
		page
	}) => {
		test.setTimeout(120_000);

		const { client, serviceId } = await clientWithActiveService(api, 'invsel');
		const clientId = await adminFindClientId(api, token, client.email);
		const invoicesBefore = await countClientOwnInvoices(api, client.accessToken);

		await page.goto(`/admin/clients/${clientId}?tab=services`);
		await page.waitForLoadState('networkidle');

		const checkbox = page.getByTestId(`service-select-${serviceId}`);
		await expect(checkbox).toBeVisible({ timeout: 15_000 });
		await checkbox.check();

		const invoiceBtn = page.getByTestId('invoice-selected-btn');
		await expect(invoiceBtn).toBeEnabled();
		await invoiceBtn.click();

		await expect(
			page.getByRole('status').filter({ hasText: /invoice\(s\) created/i })
		).toBeVisible({ timeout: 15_000 });

		const invoicesAfterFirst = await countClientOwnInvoices(api, client.accessToken);
		expect(invoicesAfterFirst).toBe(invoicesBefore + 1);

		// Same selection again: must be reported as skipped, and must NOT
		// create a second invoice for the same service.
		await checkbox.check();
		await expect(invoiceBtn).toBeEnabled();
		await invoiceBtn.click();

		await expect(
			page.getByRole('status').filter({ hasText: /skipped \(already invoiced\)/i })
		).toBeVisible({ timeout: 15_000 });

		const invoicesAfterSecond = await countClientOwnInvoices(api, client.accessToken);
		expect(invoicesAfterSecond).toBe(invoicesAfterFirst);
	});
});
