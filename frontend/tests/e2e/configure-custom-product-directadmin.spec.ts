import { expect, test } from './fixtures';
import {
	API_BASE,
	MOCK_BASE,
	adminToken,
	authHeaders,
	createOrder,
	payInvoiceViaMock,
	pollListStatus,
	registerVerifyLogin,
	unique,
	uniqueDomainLabel
} from './helpers';

/**
 * DirectAdmin counterpart to configure-custom-product.spec.ts: an admin
 * creates a configurable DirectAdmin product with Shell Access + CGI Access
 * on, plus a TemplatePackage pointing at a pre-existing DA package carrying a
 * long-tail setting (clamav) our own adapter never models. A client orders
 * it; the worker provisions the service, which for a configurable product
 * first reads that template package's raw fields via the mock, merges them
 * under its own resolved limits/toggles (which must win), and creates the
 * per-service package on the mock DirectAdmin before the account.
 *
 * Drives the API directly (not the storefront UI - that mechanics is covered
 * by admin-ops.spec.ts's Module-tab coverage) since what's under test here is
 * backend/adapter behavior, not the checkout flow. Runs against the live
 * stack (mockserver:9090, api:8080, worker).
 */

const DA_USERNAME = 'e2eadmin';
const DA_PASSWORD = 'e2e-da-password';

function daBasicAuthHeader(): Record<string, string> {
	return { Authorization: `Basic ${Buffer.from(`${DA_USERNAME}:${DA_PASSWORD}`).toString('base64')}` };
}

test.describe('custom-spec product (DirectAdmin) → template package clone → pay → package provisioned', () => {
	test('admin defines a configurable DA product with a template package; client orders it and the per-service package inherits the template plus its own toggles', async ({
		request
	}) => {
		test.setTimeout(180_000);

		const suffix = unique('e2e-da');
		const slug = `${suffix}-product`;
		const domain = `${uniqueDomainLabel()}.hosting.e2e.test`;

		// --- 1. Admin: pre-create a template package directly on the mock DA ----
		// clamav is a long-tail setting our own adapter never sets - proving the
		// merge genuinely inherits fields it doesn't model, not just the ones it
		// happens to also send itself.
		const templatePackageName = `${suffix}_template`;
		const templateRes = await request.post(`${MOCK_BASE}/CMD_API_MANAGE_USER_PACKAGES`, {
			headers: daBasicAuthHeader(),
			form: { action: 'create', add: 'Submit', packagename: templatePackageName, clamav: 'ON' }
		});
		expect(templateRes.ok(), 'create DA template package on the mock').toBeTruthy();

		// --- 2. Admin: create a configurable DirectAdmin product ----------------
		const admin = await adminToken(request);
		const adminAuth = authHeaders(admin);

		const seededRes = await request.get(`${API_BASE}/api/v1/admin/products`, {
			headers: adminAuth,
			params: { search: 'e2e-shared-hosting' }
		});
		expect(seededRes.ok()).toBeTruthy();
		const seeded = ((await seededRes.json()).data ?? []).find(
			(p: { slug: string }) => p.slug === 'e2e-shared-hosting'
		);
		expect(seeded, 'seeded hosting product must exist').toBeTruthy();

		const serversRes = await request.get(`${API_BASE}/api/v1/admin/servers`, { headers: adminAuth });
		expect(serversRes.ok()).toBeTruthy();
		const mockServer = ((await serversRes.json()).data ?? []).find(
			(s: { module: string; hostname: string }) => s.module === 'directadmin' && s.hostname === 'localhost'
		);
		expect(mockServer, 'seeded DirectAdmin mock server must exist').toBeTruthy();
		const serverGroupId = mockServer.group_id as number;

		const createRes = await request.post(`${API_BASE}/api/v1/admin/products`, {
			headers: adminAuth,
			data: {
				group_id: seeded.group_id,
				name: `Custom DA Hosting ${suffix}`,
				slug,
				type: 'shared_hosting',
				module: 'directadmin',
				server_group_id: serverGroupId,
				auto_setup: 'on_payment',
				configurable: true,
				shell_access: true,
				cgi_access: true,
				template_package: templatePackageName
			}
		});
		expect(createRes.ok(), `create configurable DA product: ${await createRes.text()}`).toBeTruthy();
		const productId = (await createRes.json()).data.id as number;

		expect(
			(
				await request.put(`${API_BASE}/api/v1/admin/products/${productId}/pricing`, {
					headers: adminAuth,
					data: { cycle: 'monthly', price: 50000, setup_fee: 0 }
				})
			).ok()
		).toBeTruthy();

		const specRes = await request.post(`${API_BASE}/api/v1/admin/products/${productId}/specs`, {
			headers: adminAuth,
			data: {
				key: 'disk',
				label: 'Disk space',
				provision_key: 'disk',
				unit: 'gb',
				included_qty: 5,
				min_qty: 5,
				max_qty: 100,
				step_qty: 5,
				default_qty: 10
			}
		});
		expect(specRes.ok(), 'create disk spec').toBeTruthy();
		const specId = (await specRes.json()).data.id as number;
		expect(
			(
				await request.put(`${API_BASE}/api/v1/admin/product-specs/${specId}/pricing`, {
					headers: adminAuth,
					data: { cycle: 'monthly', unit_price: 1000, unlimited_price: 0 }
				})
			).ok()
		).toBeTruthy();

		// --- 3. Client: order it with a specific disk quantity via the API ------
		const client = await registerVerifyLogin(request, 'e2e-da-client');
		const invoiceId = await createOrder(request, client.accessToken, [
			{
				item_type: 'product',
				product_id: productId,
				cycle: 'monthly',
				domain,
				specs: [{ key: 'disk', qty: 50, unlimited: false }]
			}
		]);
		await payInvoiceViaMock(request, client.accessToken, invoiceId);

		// --- 4. Wait for the worker to provision the service ---------------------
		const svc = await pollListStatus(
			request,
			client.accessToken,
			'/api/v1/services',
			(r) => r.domain === domain,
			'active'
		);
		const packageName = (svc.panel_meta as { package_name?: string } | undefined)?.package_name;
		expect(packageName, 'service must record the dynamic package name').toBeTruthy();

		// --- 5. Assert the per-service DA package inherited the template's ------
		// long-tail field, while our own resolved limit and toggles win.
		let dynamicPackage: { name: string; params: Record<string, string> } | undefined;
		await expect
			.poll(
				async () => {
					const res = await request.get(`${MOCK_BASE}/mock/da/packages`);
					if (!res.ok()) return null;
					const pkgs = ((await res.json()).packages ?? []) as Array<{
						name: string;
						params: Record<string, string>;
					}>;
					dynamicPackage = pkgs.find((p) => p.name === packageName);
					return dynamicPackage?.params?.quota ?? null;
				},
				{ timeout: 30_000, message: 'waiting for the dynamic DA package to be created' }
			)
			.toBe(String(50 * 1024)); // 50 GB → 51200 MB, our resolved limit — not the template's

		expect(dynamicPackage?.params?.clamav, 'long-tail template field must be inherited').toBe('ON');
		expect(dynamicPackage?.params?.cgi).toBe('ON');
		expect(dynamicPackage?.params?.ssh).toBe('ON');
	});
});
