import { createHmac } from 'node:crypto';
import { expect, test, type APIRequestContext } from './fixtures';
import { loginApi, newApi, registerVerifyLogin, setSessionCookies, type Client } from './helpers';

/**
 * Client account management: profile update, sub-account contacts, password
 * change, and TOTP two-factor enable/disable (computed codes).
 */

// --- minimal RFC 6238 TOTP (SHA1, 6 digits, 30s) ---------------------------
function base32Decode(s: string): Buffer {
	const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
	let bits = '';
	for (const c of s.replace(/[^A-Za-z2-7]/g, '').toUpperCase()) {
		const idx = alphabet.indexOf(c);
		if (idx === -1) continue;
		bits += idx.toString(2).padStart(5, '0');
	}
	const bytes: number[] = [];
	for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
	return Buffer.from(bytes);
}

function totp(secret: string, stepShift = 0): string {
	const step = Math.floor(Date.now() / 1000 / 30) + stepShift;
	const key = base32Decode(secret);
	const buf = Buffer.alloc(8);
	buf.writeBigInt64BE(BigInt(step));
	const hmac = createHmac('sha1', key).update(buf).digest();
	const offset = hmac[hmac.length - 1] & 0xf;
	const code =
		(((hmac[offset] & 0x7f) << 24) |
			((hmac[offset + 1] & 0xff) << 16) |
			((hmac[offset + 2] & 0xff) << 8) |
			(hmac[offset + 3] & 0xff)) %
		1_000_000;
	return code.toString().padStart(6, '0');
}

test.describe('client account', () => {
	let api: APIRequestContext;
	let client: Client;

	test.beforeAll(async () => {
		test.setTimeout(120_000);
		api = await newApi();
		client = await registerVerifyLogin(api, 'account');
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test('update profile persists across a reload', async ({ context, page }) => {
		await setSessionCookies(context, client.accessToken, client.refreshToken);
		await page.goto('/account');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('account-profile-panel')).toBeVisible();

		const phone = `0812${Date.now().toString().slice(-8)}`;
		const state = `Jawa Timur ${Date.now().toString().slice(-6)}`;
		await page.locator('[name="phone"]').fill(phone);
		await page.locator('[name="company"]').fill('E2E Corp');
		// address1/city/state/postcode: PATCH /account/profile (clients.UpdateProfile)
		// covers these too, not just phone/company - a real bug (2026-07-27) had
		// the account form silently discard all of these on every save, because
		// it posted to PATCH /auth/me instead, whose DTO never declared them.
		await page.locator('[name="address1"]').fill('Jl. Merdeka No. 1');
		await page.locator('[name="city"]').fill('Surabaya');
		await page.locator('[name="state"]').fill(state);
		await page.locator('[name="postcode"]').fill('60111');
		await page.getByTestId('profile-save').locator('button').click();
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		await page.reload();
		await page.waitForLoadState('networkidle');
		await expect(page.locator('[name="phone"]')).toHaveValue(phone);
		await expect(page.locator('[name="company"]')).toHaveValue('E2E Corp');
		await expect(page.locator('[name="address1"]')).toHaveValue('Jl. Merdeka No. 1');
		await expect(page.locator('[name="city"]')).toHaveValue('Surabaya');
		await expect(page.locator('[name="state"]')).toHaveValue(state);
		await expect(page.locator('[name="postcode"]')).toHaveValue('60111');
	});

	test('province (state) is a required field on the profile form', async ({ context, page }) => {
		await setSessionCookies(context, client.accessToken, client.refreshToken);
		await page.goto('/account');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('account-profile-panel')).toBeVisible();

		const stateInput = page.locator('[name="state"]');
		await expect(stateInput).toHaveAttribute('required', '');

		// Clearing it and attempting to save must be blocked by native HTML
		// validation (form never actually submits) - the reload above proves
		// the field round-trips once filled in; this proves it can't be blanked.
		await stateInput.fill('');
		await page.getByTestId('profile-save').locator('button').click();
		await expect(stateInput).toHaveJSProperty('validity.valid', false);
	});

	test('add a sub-account contact', async ({ context, page }) => {
		await setSessionCookies(context, client.accessToken, client.refreshToken);
		await page.goto('/account?tab=contacts');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('account-contacts-panel')).toBeVisible();

		await page.getByTestId('contact-add-button').locator('button').click();
		await expect(page.getByTestId('contact-form')).toBeVisible();

		const contactEmail = `sub-${Date.now()}@e2e.test`;
		await page.getByTestId('contact-form').locator('[name="first_name"]').fill('Sub');
		await page.getByTestId('contact-form').locator('[name="last_name"]').fill('Account');
		await page.getByTestId('contact-form').locator('[name="email"]').fill(contactEmail);
		await page.getByTestId('contact-save').locator('button').click();

		await expect(page.getByText('Sub Account')).toBeVisible({ timeout: 10_000 });
	});

	test('change password and log in with the new one', async ({ context, page }) => {
		// Dedicated client so a password change never disturbs the shared session.
		const pw = await registerVerifyLogin(api, 'pwchange');
		await setSessionCookies(context, pw.accessToken, pw.refreshToken);
		await page.goto('/account?tab=security');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('account-password-panel')).toBeVisible();

		const newPassword = 'N3wClientPass!2026';
		await page.locator('[name="current_password"]').fill(pw.password);
		await page.locator('[name="new_password"]').fill(newPassword);
		await page.locator('[name="confirm_password"]').fill(newPassword);
		await page.getByTestId('password-save').locator('button').click();
		await expect(page.getByRole('status')).toBeVisible({ timeout: 10_000 });

		const good = await loginApi(api, pw.email, newPassword);
		expect(good.accessToken).toBeTruthy();
	});

	test('enable then disable TOTP two-factor auth', async ({ context, page }) => {
		test.setTimeout(90_000);
		const tf = await registerVerifyLogin(api, '2fa');
		await setSessionCookies(context, tf.accessToken, tf.refreshToken);
		await page.goto('/account?tab=security');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('account-twofa-panel')).toBeVisible();

		// Start setup -> reveals the shared secret.
		await page.getByTestId('twofa-setup-button').locator('button').click();
		await expect(page.getByTestId('twofa-secret')).toBeVisible({ timeout: 10_000 });
		const secret = (await page.getByTestId('twofa-secret').innerText()).trim();
		expect(secret.length).toBeGreaterThan(8);

		// Enable with a computed code. The backend accepts a ±1 step skew, so a
		// single fresh code is reliable.
		await page.getByTestId('twofa-enable-form').locator('[name="totp_code"]').fill(totp(secret));
		await page.getByTestId('twofa-enable-submit').locator('button').click();
		await expect(page.getByTestId('twofa-disable-form')).toBeVisible({ timeout: 15_000 });

		// Disable via the confirm dialog (requires password + a fresh code).
		await page.getByTestId('twofa-disable-form').locator('[name="password"]').fill(tf.password);
		await page.getByTestId('twofa-disable-form').locator('[name="totp_code"]').fill(totp(secret));
		await page.getByTestId('twofa-disable-button').locator('button').click();
		await page.getByRole('button', { name: 'Confirm' }).click();
		await expect(page.getByTestId('twofa-setup-form')).toBeVisible({ timeout: 10_000 });
	});
});
