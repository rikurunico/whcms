import { expect, test, type APIRequestContext } from '@playwright/test';
import {
	adminToken,
	authHeaders,
	API_BASE,
	clickBtn,
	newApi,
	setSessionCookies,
	unique,
	waitForMail
} from './helpers';

/**
 * Admin email surfaces:
 *  - Settings > Mail "Send Test Email" - a synchronous send through the real
 *    mail driver, the operator's way to prove SMTP works.
 *  - The global "_layout" email template - the shared branded wrapper. Its
 *    {{.Content}} slot is mandatory; without it every outbound email would
 *    render blank, so the backend must reject that save.
 *  - Template preview - must render the redesigned layout with sample data
 *    filled in, not "<no value>" placeholders.
 */

test.describe('admin email templates and test send', () => {
	let api: APIRequestContext;
	let token: string;

	test.beforeAll(async () => {
		api = await newApi();
		token = await adminToken(api);
	});

	test.afterAll(async () => {
		await api?.dispose();
	});

	test.beforeEach(async ({ context }) => {
		await setSessionCookies(context, token);
	});

	test('send test email from Settings > Mail and see it delivered', async ({ page }) => {
		const to = `${unique('mailtest')}@e2e.test`;

		await page.goto('/admin/settings');
		await expect(page.getByTestId('settings-form-general')).toBeVisible();

		// Switch to the Mail tab.
		await page.getByTestId('settings-tab-mail').click();
		await expect(page.getByTestId('settings-form-test-email')).toBeVisible();

		await page.getByTestId('settings-test-email-input').fill(to);
		await clickBtn(page, 'settings-test-email-send');

		const success = page.getByTestId('settings-test-email-success');
		await expect(success).toBeVisible({ timeout: 20_000 });
		await expect(success).toContainText(to);

		// The message really went through the mail driver.
		const html = await waitForMail(api, to);
		expect(html).toContain('Konfigurasi email berhasil');
		// ...wrapped in the shared branded layout, not a bare fragment.
		expect(html).toContain('<html');
		expect(html).not.toContain('<no value>');

		// ...and it is recorded in the email log as sent.
		await expect(async () => {
			const res = await api.get(`${API_BASE}/api/v1/admin/email-log`, {
				headers: authHeaders(token),
				params: { search: to }
			});
			expect(res.ok()).toBeTruthy();
			const rows = (await res.json()).data as { to_email: string; status: string }[];
			expect(rows.some((r) => r.to_email === to && r.status === 'sent')).toBeTruthy();
		}).toPass({ timeout: 15_000 });
	});

	test('test send rejects a malformed recipient', async ({ page }) => {
		await page.goto('/admin/settings');
		await page.getByTestId('settings-tab-mail').click();

		// Bypass the browser's type=email guard so the request reaches the API.
		await page
			.getByTestId('settings-test-email-input')
			.evaluate((el: HTMLInputElement) => {
				el.type = 'text';
				el.value = 'definitely-not-an-email';
				el.dispatchEvent(new Event('input', { bubbles: true }));
			});
		await clickBtn(page, 'settings-test-email-send');

		await expect(page.getByTestId('settings-test-email-error')).toBeVisible({ timeout: 15_000 });
	});

	test('global layout template is listed and editable', async ({ page }) => {
		await page.goto('/admin/email-templates');
		await expect(page.getByTestId('template-filter-form')).toBeVisible();

		const layoutRow = page.getByTestId('row-template-_layout-id');
		await expect(layoutRow).toBeVisible();
		await layoutRow.click();

		await expect(page).toHaveURL(/\/admin\/email-templates\/_layout\/id/);
		await expect(page.getByTestId('template-key')).toHaveText('_layout');

		// The editor advertises the Go-template variables the backend really
		// supplies - including the layout-only {{.Content}} slot.
		const vars = page.getByTestId('template-variables');
		await expect(vars).toContainText('{{.Content}}');
		await expect(vars).toContainText('{{.CompanyName}}');
	});

	test('saving the layout without the {{.Content}} slot is rejected', async ({ page }) => {
		await page.goto('/admin/email-templates/_layout/id');
		await expect(page.getByTestId('template-form')).toBeVisible();

		const original = await page.getByTestId('template-body-input').inputValue();
		expect(original).toContain('.Content');

		await page.getByTestId('template-body-input').fill('<html><body>slot removed</body></html>');
		await clickBtn(page, 'template-save-button');

		const err = page.getByTestId('template-save-error');
		await expect(err).toBeVisible({ timeout: 15_000 });

		// The stored layout is untouched - reloading brings the slot back.
		await page.reload();
		await expect(page.getByTestId('template-body-input')).toHaveValue(/\.Content/);
	});

	test('preview renders the branded layout with sample data', async ({ page }) => {
		await page.goto('/admin/email-templates/invoice_created/id');
		await expect(page.getByTestId('template-form')).toBeVisible();

		await clickBtn(page, 'template-preview-button');

		const frame = page.getByTestId('template-preview-frame');
		await expect(frame).toBeVisible({ timeout: 15_000 });

		const srcdoc = (await frame.getAttribute('srcdoc')) ?? '';
		expect(srcdoc).toContain('<html');
		// Sample data is substituted rather than left as Go template defaults.
		expect(srcdoc).toContain('INV-2026-001042');
		expect(srcdoc).not.toContain('<no value>');
		expect(srcdoc).not.toContain('ZgotmplZ');

		await expect(page.getByTestId('template-preview-subject')).toContainText('INV-2026-001042');
	});
});
