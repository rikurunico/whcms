import { expect, test, type APIRequestContext } from './fixtures';
import {
	API_BASE,
	MOCK_BASE,
	adminToken,
	authHeaders,
	createOrder,
	newApi,
	payInvoiceViaMock,
	pollListStatus,
	registerVerifyLogin,
	setSessionCookies,
	uniqueDomainLabel
} from './helpers';

/**
 * Admin-configured TLD year-matrix pricing, min/max registration years, a
 * domain addon, and a premium domain override - verified end to end from
 * /admin/domains/pricing & /admin/domains/addons through the real storefront
 * search/cart/checkout flow to the resulting paid, active domain.
 */

interface AdminRegistrar {
	id: number;
}

async function rdashRegistrarId(api: APIRequestContext, token: string): Promise<number> {
	const res = await api.get(`${API_BASE}/api/v1/admin/registrars`, { headers: authHeaders(token) });
	const regs = ((await res.json()).data ?? []) as AdminRegistrar[];
	expect(regs.length).toBeGreaterThan(0);
	return regs[0].id;
}

test.describe('domain TLD pricing, years bounds, and addons end to end', () => {
	let api: APIRequestContext;
	let admin: string;

	test.beforeAll(async () => {
		api = await newApi();
		admin = await adminToken(api);
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test('registering with a 2-year TLD-matrix price and a domain addon charges the exact matrix total and freezes the correct renewal rate', async ({
		page
	}) => {
		test.setTimeout(180_000);

		const registrarId = await rdashRegistrarId(api, admin);
		const tld = uniqueDomainLabel();

		// --- Admin: configure non-linear TLD pricing (2yr != 2 × 1yr) ---------
		const createRes = await api.post(`${API_BASE}/api/v1/admin/tld-pricing`, {
			headers: authHeaders(admin),
			data: {
				tld,
				registrar_id: registrarId,
				active: true,
				min_years: 1,
				max_years: 2,
				register_prices: { '1': 100000, '2': 180000 },
				renew_prices: { '1': 110000 },
				transfer_price: 90000
			}
		});
		expect(createRes.ok(), `create tld pricing: ${await createRes.text()}`).toBeTruthy();
		const tldPricingId = ((await createRes.json()).data as { id: number }).id;

		// --- Admin: activate the DNS Management addon with a known price -----
		const addonsRes = await api.get(`${API_BASE}/api/v1/admin/domain-addons`, { headers: authHeaders(admin) });
		const addons = (await addonsRes.json()).data as { id: number; key: string; price: number; active: boolean }[];
		const dnsAddon = addons.find((a) => a.key === 'dns_management');
		expect(dnsAddon).toBeTruthy();
		const originalAddon = { price: dnsAddon!.price, active: dnsAddon!.active };
		const putAddon = await api.put(`${API_BASE}/api/v1/admin/domain-addons/${dnsAddon!.id}`, {
			headers: authHeaders(admin),
			data: { price: 15000, active: true }
		});
		expect(putAddon.ok()).toBeTruthy();

		try {
			// --- Client: search, pick 2 years + the addon, register ------------
			const client = await registerVerifyLogin(api, 'tldpricing');
			await setSessionCookies(page.context(), client.accessToken, client.refreshToken);

			const domainName = `${uniqueDomainLabel()}.${tld}`;
			await page.goto(`/order/domain?q=${encodeURIComponent(domainName)}`);
			await page.waitForLoadState('networkidle');

			const resultRow = page.getByTestId(`domain-result-${domainName}`);
			await expect(resultRow).toBeVisible({ timeout: 15_000 });

			// Years selector is bounded to exactly [1, 2] for this TLD.
			const yearsSelect = page.getByTestId(`domain-years-${domainName}`);
			await expect(yearsSelect.locator('option')).toHaveCount(2);
			await yearsSelect.selectOption('2');

			await page.getByTestId(`domain-addon-${domainName}-dns_management`).check();

			const registerBtn = page.getByTestId(`domain-register-${domainName}`);
			const inCartRow = page.getByTestId(`domain-in-cart-${domainName}`);
			for (let attempt = 1; attempt <= 3; attempt++) {
				await registerBtn.click();
				try {
					await expect(inCartRow).toBeVisible({ timeout: 5_000 });
					break;
				} catch (err) {
					if (attempt === 3) throw err;
				}
			}

			await page.goto('/order/cart');
			await expect(page.getByTestId('cart-items')).toContainText(domainName, { timeout: 15_000 });
			await expect(page.getByTestId('cart-items')).toContainText('DNS Management');
			// The cart PREVIEW must show the real 2yr matrix price (180000), not
			// a naive 1yr-price × years estimate (100000×2=200000) - this is a
			// client-side estimate only (the server recomputes at checkout), but
			// it must match what actually gets charged below or it's misleading.
			await expect(page.getByTestId('cart-item-0')).toContainText((180000 + 15000 * 2).toLocaleString('id-ID'));

			const checkoutBtn = page.getByTestId('checkout-submit');
			await expect(checkoutBtn).toBeEnabled();
			await checkoutBtn.click();
			await expect(page).toHaveURL(/\/billing\/invoices\/\d+$/, { timeout: 20_000 });

			// --- Verify the checkout charged the exact matrix + addon total ---
			const ordersRes = await api.get(`${API_BASE}/api/v1/orders`, { headers: authHeaders(client.accessToken) });
			const orders = ((await ordersRes.json()).data ?? []) as { id: number }[];
			expect(orders.length).toBeGreaterThan(0);
			const orderRes = await api.get(`${API_BASE}/api/v1/orders/${orders[0].id}`, {
				headers: authHeaders(client.accessToken)
			});
			const orderDetail = (await orderRes.json()).data as {
				items: { item_type: string; domain: string; unit_price: number }[];
			};
			const domainItem = orderDetail.items.find(
				(i) => i.item_type === 'domain_register' && i.domain === domainName
			);
			expect(domainItem, 'domain_register item present on the order').toBeTruthy();
			// 180000 (2yr matrix, not 100000×2) + 15000 addon × 2 years.
			expect(domainItem!.unit_price).toBe(180000 + 15000 * 2);

			// --- Pay, then confirm the frozen renewal rate on the domain -------
			// "VC" (credit card) has no VA/QRIS raw payload to render inline, so
			// it's the channel guaranteed to redirect to the mock's hosted
			// payment page - see payments-instructions.spec.ts for the VA/QRIS
			// inline-render coverage this flow deliberately doesn't exercise.
			await expect(page.getByTestId('invoice-status')).toBeVisible();
			// Credit Card sits behind the "show more" toggle (only major VA
			// banks + QRIS are shown expanded by default).
			await page.getByTestId('payment-methods-toggle').click();
			const methodBtn = page.getByTestId('payment-method-VC');
			await expect(methodBtn).toBeVisible({ timeout: 20_000 });
			await methodBtn.click();
			await Promise.all([
				page.waitForURL(new RegExp(`${MOCK_BASE.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/payment/`)),
				page.getByTestId('pay-button').getByRole('button').click()
			]);
			await expect(page.locator('#pay-now')).toBeVisible();
			await page.locator('#pay-now').click();
			await expect(page).toHaveURL(/\/billing\/invoices\/\d+/, { timeout: 30_000 });
			await expect(page.getByTestId('payment-panel')).toHaveCount(0, { timeout: 20_000 });

			const dom = await pollListStatus(
				api,
				client.accessToken,
				'/api/v1/domains',
				(r) => r.name === domainName,
				'active',
				90_000
			);
			// 110000 (year-1 renew rate) + 15000 addon, frozen at checkout -
			// not derived from unit_price/years (which would give (180000+30000)/2=105000).
			expect(dom.recurring_amount).toBe(110000 + 15000);
			expect(dom.dns_management_enabled).toBe(true);
		} finally {
			await api.put(`${API_BASE}/api/v1/admin/domain-addons/${dnsAddon!.id}`, {
				headers: authHeaders(admin),
				data: originalAddon
			});
			await api.delete(`${API_BASE}/api/v1/admin/tld-pricing/${tldPricingId}`, { headers: authHeaders(admin) });
		}
	});

	test('a premium domain pricing override is flagged and priced above the standard TLD rate', async ({ page }) => {
		const registrarId = await rdashRegistrarId(api, admin);
		const tld = uniqueDomainLabel();
		const premiumDomain = `${uniqueDomainLabel()}.${tld}`;

		const createTld = await api.post(`${API_BASE}/api/v1/admin/tld-pricing`, {
			headers: authHeaders(admin),
			data: {
				tld,
				registrar_id: registrarId,
				active: true,
				min_years: 1,
				max_years: 5,
				register_prices: { '1': 100000 },
				renew_prices: { '1': 110000 },
				transfer_price: 90000
			}
		});
		expect(createTld.ok()).toBeTruthy();
		const tldPricingId = ((await createTld.json()).data as { id: number }).id;

		const createPremium = await api.post(`${API_BASE}/api/v1/admin/premium-domain-pricing`, {
			headers: authHeaders(admin),
			data: {
				domain_name: premiumDomain,
				register_price: 9000000,
				renew_price: 8000000,
				transfer_price: 5000000
			}
		});
		expect(createPremium.ok(), `create premium pricing: ${await createPremium.text()}`).toBeTruthy();
		const premiumId = ((await createPremium.json()).data as { id: number }).id;

		try {
			await page.goto(`/order/domain?q=${encodeURIComponent(premiumDomain)}`);
			await page.waitForLoadState('networkidle');
			const resultRow = page.getByTestId(`domain-result-${premiumDomain}`);
			await expect(resultRow).toBeVisible({ timeout: 15_000 });
			await expect(resultRow.getByText('Premium')).toBeVisible();
			await expect(resultRow).toContainText('9.000.000');
		} finally {
			await api.delete(`${API_BASE}/api/v1/admin/premium-domain-pricing/${premiumId}`, {
				headers: authHeaders(admin)
			});
			await api.delete(`${API_BASE}/api/v1/admin/tld-pricing/${tldPricingId}`, { headers: authHeaders(admin) });
		}
	});

	test('a premium length-tier (Dewabiz-style 2-character) rule is flagged and priced above the standard TLD rate', async ({
		page
	}) => {
		const registrarId = await rdashRegistrarId(api, admin);
		const tld = uniqueDomainLabel();
		// A 2-character label under the test TLD (matches the length-tier rule).
		const shortDomain = `ab.${tld}`;

		const createTld = await api.post(`${API_BASE}/api/v1/admin/tld-pricing`, {
			headers: authHeaders(admin),
			data: {
				tld,
				registrar_id: registrarId,
				active: true,
				min_years: 1,
				max_years: 5,
				register_prices: { '1': 100000 },
				renew_prices: { '1': 110000 },
				transfer_price: 90000
			}
		});
		expect(createTld.ok()).toBeTruthy();
		const tldPricingId = ((await createTld.json()).data as { id: number }).id;

		const createTier = await api.post(`${API_BASE}/api/v1/admin/premium-length-pricing`, {
			headers: authHeaders(admin),
			data: { tld, char_length: 2, price: 485000000 }
		});
		expect(createTier.ok(), `create length-tier pricing: ${await createTier.text()}`).toBeTruthy();
		const tierId = ((await createTier.json()).data as { id: number }).id;

		try {
			await page.goto(`/order/domain?q=${encodeURIComponent(shortDomain)}`);
			await page.waitForLoadState('networkidle');
			const resultRow = page.getByTestId(`domain-result-${shortDomain}`);
			await expect(resultRow).toBeVisible({ timeout: 15_000 });
			await expect(resultRow.getByText('Premium')).toBeVisible();
			await expect(resultRow).toContainText('485.000.000');
		} finally {
			await api.delete(`${API_BASE}/api/v1/admin/premium-length-pricing/${tierId}`, {
				headers: authHeaders(admin)
			});
			await api.delete(`${API_BASE}/api/v1/admin/tld-pricing/${tldPricingId}`, { headers: authHeaders(admin) });
		}
	});

	test('toggling a domain addon on a premium length-tier domain keeps the premium renewal rate (not the standard TLD rate)', async ({
		page
	}) => {
		const registrarId = await rdashRegistrarId(api, admin);
		const tld = uniqueDomainLabel();
		const shortDomain = `ab.${tld}`;

		const createTld = await api.post(`${API_BASE}/api/v1/admin/tld-pricing`, {
			headers: authHeaders(admin),
			data: {
				tld,
				registrar_id: registrarId,
				active: true,
				min_years: 1,
				max_years: 5,
				// Deliberately far cheaper than the premium tier - proves a
				// regression would show up as a large price drop, not a
				// rounding-sized discrepancy.
				register_prices: { '1': 100000 },
				renew_prices: { '1': 110000 },
				transfer_price: 90000
			}
		});
		expect(createTld.ok()).toBeTruthy();
		const tldPricingId = ((await createTld.json()).data as { id: number }).id;

		const createTier = await api.post(`${API_BASE}/api/v1/admin/premium-length-pricing`, {
			headers: authHeaders(admin),
			data: { tld, char_length: 2, price: 485000000 }
		});
		expect(createTier.ok(), `create length-tier pricing: ${await createTier.text()}`).toBeTruthy();
		const tierId = ((await createTier.json()).data as { id: number }).id;

		const addonsRes = await api.get(`${API_BASE}/api/v1/admin/domain-addons`, { headers: authHeaders(admin) });
		const addons = (await addonsRes.json()).data as { id: number; key: string; price: number; active: boolean }[];
		const dnsAddon = addons.find((a) => a.key === 'dns_management');
		expect(dnsAddon).toBeTruthy();
		const originalAddon = { price: dnsAddon!.price, active: dnsAddon!.active };
		expect(
			(
				await api.put(`${API_BASE}/api/v1/admin/domain-addons/${dnsAddon!.id}`, {
					headers: authHeaders(admin),
					data: { price: 15000, active: true }
				})
			).ok()
		).toBeTruthy();

		try {
			const client = await registerVerifyLogin(api, 'premiumaddon');
			const invoiceId = await createOrder(api, client.accessToken, [
				{ item_type: 'domain_register', domain: shortDomain, domain_years: 1 }
			]);
			await payInvoiceViaMock(api, client.accessToken, invoiceId);
			const dom = await pollListStatus(
				api,
				client.accessToken,
				'/api/v1/domains',
				(r) => r.name === shortDomain,
				'active'
			);
			// The premium length-tier price is frozen as-is from registration.
			expect(dom.recurring_amount).toBe(485000000);

			await setSessionCookies(page.context(), client.accessToken, client.refreshToken);

			// SvelteKit hydration race: a checkbox/save click can silently
			// no-op if it lands before client JS finishes wiring up handlers
			// on a freshly navigated page - retry the whole interaction
			// (reload included) rather than just the click.
			let dnsEnabled = false;
			for (let attempt = 1; attempt <= 3 && !dnsEnabled; attempt++) {
				await page.goto(`/domains/${dom.id}?tab=addons`);
				await page.waitForLoadState('networkidle');
				await page.getByTestId('addon-checkbox-dns_management').check();
				await page.getByTestId('addons-save').locator('button').click();
				await expect(page.getByTestId('addon-checkbox-dns_management')).toBeChecked({ timeout: 10_000 });

				const check = await api.get(`${API_BASE}/api/v1/domains/${dom.id}`, {
					headers: authHeaders(client.accessToken)
				});
				dnsEnabled = ((await check.json()).data as { dns_management_enabled: boolean }).dns_management_enabled;
			}

			// Must stay the premium rate + addon - never fall back to the
			// standard TLD renew price (110000 + 15000).
			const after = await api.get(`${API_BASE}/api/v1/domains/${dom.id}`, {
				headers: authHeaders(client.accessToken)
			});
			const afterBody = (await after.json()).data as { recurring_amount: number; dns_management_enabled: boolean };
			expect(afterBody.dns_management_enabled).toBe(true);
			expect(afterBody.recurring_amount).toBe(485000000 + 15000);
		} finally {
			await api.put(`${API_BASE}/api/v1/admin/domain-addons/${dnsAddon!.id}`, {
				headers: authHeaders(admin),
				data: originalAddon
			});
			await api.delete(`${API_BASE}/api/v1/admin/premium-length-pricing/${tierId}`, {
				headers: authHeaders(admin)
			});
			await api.delete(`${API_BASE}/api/v1/admin/tld-pricing/${tldPricingId}`, { headers: authHeaders(admin) });
		}
	});
});
