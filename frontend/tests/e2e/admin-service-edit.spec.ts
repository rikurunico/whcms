import { expect, test } from './fixtures';
import {
	ADMIN_EMAIL,
	ADMIN_PASSWORD,
	API_BASE,
	adminToken,
	authHeaders,
	clientWithActiveService,
	setSessionCookies,
	unique
} from './helpers';

/**
 * Admin service detail page: WHMCS-style manual field editor.
 *
 * Motivated by a real incident: a real control panel can reject a generated
 * username as reserved, and no amount of automatic disambiguation-retry can
 * route around it if every candidate is a truncated prefix of the same
 * structurally-blocked domain name - an admin needs to be able to manually
 * correct a stuck service's domain/username/server/etc. directly from this
 * page, the same way real WHMCS lets them. Product, password, and status
 * stay out of this generic editor (they have their own panel-synchronizing
 * flows, covered by admin-ops.spec.ts's suspend/unsuspend coverage instead).
 */

test.describe('admin service edit form', () => {
	test('admin edits every plain field on a service and the values persist after reload', async ({
		page,
		request
	}) => {
		test.setTimeout(60_000);

		const { serviceId } = await clientWithActiveService(request, 'svcedit');
		const adminAccessToken = await adminToken(request);

		// Pick the seeded mock cPanel server (module=cpanel, hostname=localhost)
		// so the Server dropdown has a real, known option to select.
		const serversRes = await request.get(`${API_BASE}/api/v1/admin/servers`, {
			headers: authHeaders(adminAccessToken),
			params: { per_page: 100 }
		});
		expect(serversRes.ok()).toBeTruthy();
		const servers = ((await serversRes.json()).data ?? []) as Array<{
			id: number;
			module: string;
			hostname: string;
		}>;
		const mockServer = servers.find((s) => s.module === 'cpanel' && s.hostname === 'localhost');
		expect(mockServer, 'seeded cPanel mock server must exist').toBeTruthy();

		const suffix = unique('edit');
		const newDomain = `edited-${suffix}.example`;
		const newUsername = 'fixeduser'; // deliberately avoids "test"

		// --- Log in as the seeded admin via the UI ------------------------------
		// Shares the /auth/login rate-limit bucket with every other concurrently
		// running spec - retry across more than one fixed window before failing.
		let adminLoggedIn = false;
		for (let attempt = 1; attempt <= 8 && !adminLoggedIn; attempt++) {
			await page.goto('/login');
			await page.waitForLoadState('networkidle');
			await page.locator('input[name="email"]').fill(ADMIN_EMAIL);
			await page.locator('input[name="password"]').fill(ADMIN_PASSWORD);
			await page.locator('button[type="submit"]').click();
			try {
				await expect(page).toHaveURL(/\/admin$/, { timeout: 20_000 });
				adminLoggedIn = true;
			} catch (err) {
				if (attempt === 8) throw err;
			}
		}

		await page.goto(`/admin/services/${serviceId}`);
		await page.waitForLoadState('networkidle');
		await expect(page.locator('[data-testid="service-update-form"]')).toBeVisible();

		await page.locator('#field-domain').fill(newDomain);
		await page.locator('#field-username').fill(newUsername);
		await page.locator('#field-server_id').selectOption(String(mockServer!.id));
		await page.locator('#field-billing_cycle').selectOption('annually');
		await page.locator('#field-recurring_amount').fill('1200000');
		await page.locator('#field-registration_date').fill('2026-01-15');
		await page.locator('#field-terminated_at').fill('2026-12-31');
		await page.locator('#field-suspend_reason').fill('manual admin note');
		await page.locator('[data-testid="service-update-submit"] button').click();

		await expect(page.getByText(/success/i).last()).toBeVisible({ timeout: 15_000 });

		// Re-navigate (not just reload) to prove the values were actually
		// persisted server-side, not just left over in local form state -
		// matches this suite's established convention for proving persistence.
		await expect(async () => {
			await page.goto(`/admin/services/${serviceId}`);
			await page.waitForLoadState('networkidle');
			await expect(page.locator('#field-domain')).toHaveValue(newDomain, { timeout: 2_000 });
			await expect(page.locator('#field-username')).toHaveValue(newUsername, { timeout: 2_000 });
			await expect(page.locator('#field-server_id')).toHaveValue(String(mockServer!.id), {
				timeout: 2_000
			});
			await expect(page.locator('#field-billing_cycle')).toHaveValue('annually', {
				timeout: 2_000
			});
			await expect(page.locator('#field-recurring_amount')).toHaveValue('1200000', {
				timeout: 2_000
			});
			await expect(page.locator('#field-registration_date')).toHaveValue('2026-01-15', {
				timeout: 2_000
			});
			await expect(page.locator('#field-terminated_at')).toHaveValue('2026-12-31', {
				timeout: 2_000
			});
			await expect(page.locator('#field-suspend_reason')).toHaveValue('manual admin note', {
				timeout: 2_000
			});
		}).toPass({ timeout: 20_000 });

		// Cross-check via the real API too (belt & suspenders on "stored and
		// returned by the backend", not just local UI state).
		const res = await request.get(`${API_BASE}/api/v1/admin/services/${serviceId}`, {
			headers: authHeaders(adminAccessToken)
		});
		expect(res.ok()).toBeTruthy();
		const svc = (await res.json()).data as {
			domain: string;
			username: string;
			server_id: number;
			billing_cycle: string;
			recurring_amount: number;
			suspend_reason: string;
		};
		expect(svc.domain).toBe(newDomain);
		expect(svc.username).toBe(newUsername);
		expect(svc.server_id).toBe(mockServer!.id);
		expect(svc.billing_cycle).toBe('annually');
		expect(svc.recurring_amount).toBe(1_200_000);
		expect(svc.suspend_reason).toBe('manual admin note');
	});

	test('admin upgrades a service via the prorated/invoice path (not Change Package)', async ({
		page,
		request
	}) => {
		test.setTimeout(60_000);

		const { serviceId } = await clientWithActiveService(request, 'svcupgrade');
		const adminAccessToken = await adminToken(request);

		// Self-contained fixture: a fresh product group + product + monthly
		// pricing well above any product this shared catalog could plausibly
		// have, so the upgrade is guaranteed to land on the invoice path
		// (diff > 0), proving this is the billed prorated flow and not
		// Change Package's immediate no-invoice swap.
		const suffix = unique('upgrade');
		const groupRes = await request.post(`${API_BASE}/api/v1/admin/product-groups`, {
			headers: authHeaders(adminAccessToken),
			data: { name: `E2E Upgrade Group ${suffix}` }
		});
		expect(groupRes.ok(), `create group: ${await groupRes.text()}`).toBeTruthy();
		const groupId = ((await groupRes.json()).data as { id: number }).id;

		const productRes = await request.post(`${API_BASE}/api/v1/admin/products`, {
			headers: authHeaders(adminAccessToken),
			data: {
				group_id: groupId,
				name: `E2E Upgrade Target ${suffix}`,
				type: 'shared_hosting',
				module: 'cpanel',
				package_name: `e2e-upgrade-${suffix}`,
				// The service edit page's product picker only loads page 1 (100
				// products) ordered by `sort ASC, id ASC`; this shared E2E catalog
				// accumulates hundreds of throwaway products across runs, so a
				// fresh product needs a very low sort to guarantee it's still on
				// that first page regardless of how many products already exist.
				sort: -999999
			}
		});
		expect(productRes.ok(), `create product: ${await productRes.text()}`).toBeTruthy();
		const targetProduct = (await productRes.json()).data as { id: number; name: string };

		const pricingRes = await request.put(
			`${API_BASE}/api/v1/admin/products/${targetProduct.id}/pricing`,
			{
				headers: authHeaders(adminAccessToken),
				data: { cycle: 'monthly', price: 9_990_000, setup_fee: 0, currency: 'IDR' }
			}
		);
		expect(pricingRes.ok(), `set pricing: ${await pricingRes.text()}`).toBeTruthy();

		await setSessionCookies(page.context(), adminAccessToken);
		await page.goto(`/admin/services/${serviceId}`);
		await page.waitForLoadState('networkidle');

		await page.getByTestId('service-action-upgrade').locator('button').click();
		await page
			.getByTestId('service-upgrade-field-product')
			.selectOption({ label: targetProduct.name });
		await page.getByTestId('service-upgrade-field-cycle').selectOption('monthly');
		await page.getByTestId('service-upgrade-submit').locator('button').click();

		await expect(
			page.getByRole('status').filter({ hasText: /prorated upgrade invoice/i })
		).toBeVisible({ timeout: 15_000 });

		// Re-navigate to prove the pending upgrade was actually persisted
		// server-side, not just reflected in local form state.
		await expect(async () => {
			await page.goto(`/admin/services/${serviceId}`);
			await page.waitForLoadState('networkidle');
			await expect(page.getByTestId('service-pending-upgrade-banner')).toBeVisible({
				timeout: 2_000
			});
		}).toPass({ timeout: 20_000 });

		const svcRes = await request.get(`${API_BASE}/api/v1/admin/services/${serviceId}`, {
			headers: authHeaders(adminAccessToken)
		});
		expect(svcRes.ok()).toBeTruthy();
		const svc = (await svcRes.json()).data as {
			pending_upgrade: { product_id: number } | null;
			status: string;
		};
		expect(svc.pending_upgrade?.product_id).toBe(targetProduct.id);
		// Change Package would have swapped product_id immediately with no
		// pending_upgrade at all - confirm the service itself hasn't moved yet.
		expect(svc.status).toBe('active');
	});
});
