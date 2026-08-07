import { expect, test } from '@playwright/test';
import { withAuthRateLimitRetry } from './helpers';

/**
 * PRD §13.2 Flow 3 - domain-order-register:
 *   search a domain name that does NOT contain "taken" (mockserver reports it
 *   available) -> add as a domain registration -> checkout+pay (same Duitku
 *   mechanism as hosting) -> confirm the domain reaches `active` in the
 *   client domains list.
 *
 * Account setup (register/verify/login) is driven directly against the real
 * backend + mock mail sink for speed/determinism (flow 1 already covers that
 * chain through the UI); everything domain-specific below drives the real
 * frontend UI end to end.
 */

const API_BASE = 'http://localhost:8080';
const MOCK_BASE = 'http://localhost:9090';

function uniqueSuffix(): string {
	return `${Date.now().toString(36)}${Math.floor(Math.random() * 1e6).toString(36)}`;
}

interface MailMessage {
	to: string;
	html?: string;
	text?: string;
}

test.describe('Flow 3: domain search -> order -> register -> active', () => {
	test('client registers an available domain, pays, and it reaches active', async ({ page, request }) => {
		test.setTimeout(220_000);

		const stamp = uniqueSuffix();
		const email = `e2e-domain-${stamp}@e2e.test`;
		const password = 'DomainE2E!2026';
		// Must NOT contain "taken" so the RDash mock reports it available.
		const domainName = `e2e-avail-${stamp}.com`;

		// --- 1. Register a fresh client directly against the backend --------
		const registerRes = await withAuthRateLimitRetry(() =>
			request.post(`${API_BASE}/api/v1/auth/register`, {
				data: {
					email,
					password,
					captcha_token: 'e2e-dummy-captcha-token',
					first_name: 'Domain',
					last_name: 'Tester',
					address1: 'Jl. Testing No. 1',
					city: 'Jakarta',
					state: 'DKI Jakarta',
					postcode: '12345',
					country: 'ID',
					phone: '+6281234500000'
				}
			})
		);
		expect(registerRes.ok(), `register failed: ${registerRes.status()} ${await registerRes.text()}`).toBeTruthy();

		// --- 2. Pull the verification token from the mock mail sink ---------
		let token = '';
		await expect
			.poll(
				async () => {
					const res = await request.get(`${MOCK_BASE}/mail/messages`, { params: { to: email } });
					if (!res.ok()) return '';
					const messages = (await res.json()) as MailMessage[];
					for (const m of messages) {
						const body = `${m.html ?? ''}\n${m.text ?? ''}`;
						const match = body.match(/verify-email\?token=([A-Za-z0-9_-]+)/);
						if (match) {
							token = match[1];
							return token;
						}
					}
					return '';
				},
				{ timeout: 30_000, intervals: [500, 1000, 2000] }
			)
			.not.toBe('');
		expect(token).not.toBe('');

		// --- 3. Verify the email via the real backend endpoint ---------------
		const verifyRes = await withAuthRateLimitRetry(() =>
			request.post(`${API_BASE}/api/v1/auth/verify-email`, { data: { token } })
		);
		expect(verifyRes.ok(), `verify-email failed: ${verifyRes.status()} ${await verifyRes.text()}`).toBeTruthy();

		// --- 4. Log in through the real UI (sets the session cookie) --------
		// The shared backend runs alongside other concurrently-executing E2E
		// specs (docs/E2E.md), so an occasional slow response or transient
		// rate-limit hiccup is expected here, not a real bug - retry the
		// submission a couple of times before failing for good.
		await page.goto('/login');
		let loggedIn = false;
		for (let attempt = 1; attempt <= 3 && !loggedIn; attempt++) {
			// SvelteKit hydration race: filling before client JS hydration wires
			// up bind:value can get silently wiped by the hydration effect
			// snapping the field back to its initial empty state - wait for
			// network-idle before touching any control, every attempt.
			await page.waitForLoadState('networkidle');
			await page.locator('input[name="email"]').fill(email);
			await page.locator('input[name="password"]').fill(password);
			await page.locator('button[type="submit"]').click();
			try {
				await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });
				loggedIn = true;
			} catch (err) {
				if (attempt === 3) throw err;
				await page.goto('/login');
			}
		}

		// --- 5. Search for a domain that is available ------------------------
		await page.goto(`/order/domain?q=${encodeURIComponent(domainName)}`);
		// SvelteKit hydration race: a click can silently no-op if it lands
		// before client JS finishes hydrating and wiring up the button's
		// handler (it still passes Playwright's actionability checks). This
		// widens under concurrent-suite load, so wait for network-idle first.
		await page.waitForLoadState('networkidle');
		const resultRow = page.getByTestId(`domain-result-${domainName}`);
		await expect(resultRow).toBeVisible({ timeout: 15_000 });

		const registerBtn = page.getByTestId(`domain-register-${domainName}`);
		await expect(registerBtn).toBeVisible();
		// Retry the click itself in case it lands just before hydration
		// attaches the handler (see comment above).
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

		// --- 6. Continue to cart via the page's own CTA, confirm the item, and
		// check out. The button only renders once something is in the cart.
		const continueToCartBtn = page.getByTestId('domain-continue-to-cart');
		await expect(continueToCartBtn).toBeVisible();
		await continueToCartBtn.click();
		await expect(page).toHaveURL(/\/order\/cart$/, { timeout: 15_000 });
		const cartItems = page.getByTestId('cart-items');
		await expect(cartItems).toContainText(domainName, { timeout: 15_000 });

		const checkoutBtn = page.getByTestId('checkout-submit');
		await expect(checkoutBtn).toBeEnabled();
		await checkoutBtn.click();

		await expect(page).toHaveURL(/\/billing\/invoices\/\d+$/, { timeout: 20_000 });
		const invoiceUrl = page.url();
		const invoiceIdMatch = invoiceUrl.match(/\/billing\/invoices\/(\d+)/);
		expect(invoiceIdMatch).not.toBeNull();

		// --- 7. Pay the invoice via a Duitku method, same as hosting flow ----
		// "VC" (credit card) has no VA/QRIS raw payload to render inline, so it's
		// the channel guaranteed to redirect to the mock's hosted payment page -
		// see payments-instructions.spec.ts for the VA/QRIS inline-render coverage.
		await expect(page.getByTestId('invoice-status')).toBeVisible();
		// Credit Card sits behind the "show more" toggle (only major VA
		// banks + QRIS are shown expanded by default).
		await page.getByTestId('payment-methods-toggle').click();
		const methodBtn = page.getByTestId('payment-method-VC'); // Credit Card
		await expect(methodBtn).toBeVisible({ timeout: 20_000 });
		await methodBtn.click();

		await Promise.all([
			page.waitForURL(new RegExp(`${MOCK_BASE.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/payment/`)),
			page.getByTestId('pay-button').getByRole('button').click()
		]);

		// Mock Duitku hosted payment page -> click "Bayar Sekarang".
		await expect(page.locator('#pay-now')).toBeVisible();
		await page.locator('#pay-now').click();

		// Redirected back through /payments/return -> eventually the invoice page.
		await expect(page).toHaveURL(/\/billing\/invoices\/\d+/, { timeout: 30_000 });

		// The payment panel disappears once the invoice is no longer payable
		// (i.e. it settled to paid).
		await expect(page.getByTestId('payment-panel')).toHaveCount(0, { timeout: 20_000 });

		// --- 8. Confirm the domain reaches `active` in the client domains list
		await expect
			.poll(
				async () => {
					await page.goto(`/domains?search=${encodeURIComponent(domainName)}&status=active`);
					return page.locator('table').getByText(domainName).count();
				},
				{ timeout: 90_000, intervals: [2000, 3000, 5000] }
			)
			.toBeGreaterThan(0);
	});
});
