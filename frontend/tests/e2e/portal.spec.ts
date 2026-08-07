import { expect, test } from '@playwright/test';

/**
 * Public portal content: announcements, knowledgebase, network status and the
 * contact form. Read pages assert the seeded fixtures render; the contact form
 * exercises a real write (creates a guest ticket).
 */

test.describe('public portal', () => {
	test('announcements list shows the seeded announcement and opens its detail', async ({ page }) => {
		await page.goto('/announcements');
		await expect(page.getByTestId('announcements-list')).toBeVisible();
		await expect(page.getByText('Welcome to our hosting portal')).toBeVisible();

		// Open the seeded announcement detail (slug: welcome).
		await page.goto('/announcements/welcome');
		await expect(page.getByTestId('announcement-detail')).toBeVisible();
		await expect(page.getByTestId('announcement-title')).toContainText('Welcome');
	});

	test('knowledgebase browses category and article', async ({ page }) => {
		await page.goto('/knowledgebase');
		await expect(page.getByTestId('kb-home')).toBeVisible();
		await expect(page.getByTestId('kb-search-input')).toBeVisible();

		// Seeded category + article.
		await page.goto('/knowledgebase/category/getting-started');
		await expect(page.getByTestId('kb-category')).toBeVisible();
		await expect(page.getByText('Getting Started')).toBeVisible();

		await page.goto('/knowledgebase/article/how-to-order');
		await expect(page.getByTestId('kb-article')).toBeVisible();
		await expect(page.getByText('How to place an order')).toBeVisible();
	});

	test('knowledgebase search returns results', async ({ page }) => {
		await page.goto('/knowledgebase');
		await page.waitForLoadState('networkidle');
		await page.getByTestId('kb-search-input').fill('order');
		await page.getByTestId('kb-search-input').press('Enter'); // GET form → /knowledgebase?search=order
		await page.waitForURL(/knowledgebase\?search=order/);
		await expect(page.getByTestId('kb-results')).toBeVisible();
		await expect(page.getByText('How to place an order')).toBeVisible();
	});

	test('network status page renders the seeded issues', async ({ page }) => {
		await page.goto('/network-status');
		await expect(page.getByTestId('network-status')).toBeVisible();
		await expect(page.getByText(/latency|maintenance/i).first()).toBeVisible();
	});

	test('contact form submits and shows success', async ({ page }) => {
		await page.goto('/contact');
		await page.waitForLoadState('networkidle');
		await expect(page.getByTestId('contact-form')).toBeVisible();

		await page.locator('[name="name"]').fill('E2E Visitor');
		await page.locator('[name="email"]').fill(`visitor-${Date.now()}@e2e.test`);
		await page.locator('[name="subject"]').fill('Question about hosting');
		// department_id is a <select>; pick the first real option.
		const dept = page.locator('[name="department_id"]');
		if (await dept.count()) {
			const opts = dept.locator('option');
			const n = await opts.count();
			for (let i = 0; i < n; i++) {
				const v = await opts.nth(i).getAttribute('value');
				if (v && v !== '') {
					await dept.selectOption(v);
					break;
				}
			}
		}
		await page.locator('[name="message"]').fill('Hello, I would like to know more about your plans.');
		await page.getByTestId('contact-submit').click();

		await expect(page.getByTestId('contact-success')).toBeVisible({ timeout: 15_000 });
	});
});
