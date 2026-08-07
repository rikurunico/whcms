import { expect, test } from './fixtures';
import {
	adminToken,
	API_BASE,
	authHeaders,
	newApi,
	registerVerifyLogin,
	setSessionCookies,
	unique
} from './helpers';

/**
 * Page-size selector coverage (CLAUDE.md §0.5/§4): every paginated list offers
 * 10/25/50/100/500/1000, defaults to 10, and changing it resets to page 1.
 */

const PAGE_SIZE_OPTIONS = ['10', '25', '50', '100', '500', '1000'];

test.describe('pagination page-size selector', () => {
	test('public announcements: defaults to 10, switching shows more and updates the URL', async ({
		page
	}) => {
		test.setTimeout(60_000);
		const api = await newApi();
		const token = await adminToken(api);
		const stamp = unique('pg-ann');

		// Guarantee at least 12 published announcements exist, regardless of
		// whatever other specs have created on this shared stack. Cleaned up
		// below so the shared /announcements list (portal.spec.ts) isn't
		// permanently pushed past its default page size by this fixture data.
		const createdIds: number[] = [];
		try {
			for (let i = 0; i < 12; i++) {
				const res = await api.post(`${API_BASE}/api/v1/admin/announcements`, {
					headers: authHeaders(token),
					data: { title: `${stamp} #${i}`, body: 'e2e pagination fixture', published: true }
				});
				expect(res.ok(), `create announcement ${i}: ${await res.text()}`).toBeTruthy();
				createdIds.push(((await res.json()).data as { id: number }).id);
			}

			await page.goto('/announcements');

			const select = page.getByTestId('page-size-select');
			await expect(select).toBeVisible();
			await expect(select).toHaveValue('10');
			const options = await select.locator('option').allTextContents();
			expect(options).toEqual(PAGE_SIZE_OPTIONS);

			const rows = page.locator('[data-testid^="announcement-row-"]');
			await expect(rows).toHaveCount(10);

			await select.selectOption('1000');
			await expect(page).toHaveURL(/per_page=1000/);
			await expect(page).toHaveURL(/page=1\b/);
			await expect(select).toHaveValue('1000');
			await expect(rows.nth(11)).toBeVisible();
			const countAfter = await rows.count();
			expect(countAfter).toBeGreaterThan(10);
		} finally {
			for (const id of createdIds) {
				await api.delete(`${API_BASE}/api/v1/admin/announcements/${id}`, { headers: authHeaders(token) });
			}
			await api.dispose();
		}
	});

	test('admin clients list: defaults to 10 and switching page size persists across reload', async ({
		page,
		context
	}) => {
		test.setTimeout(60_000);
		const api = await newApi();
		const token = await adminToken(api);
		await setSessionCookies(context, token);
		await api.dispose();

		await page.goto('/admin/clients');

		const select = page.getByTestId('page-size-select');
		await expect(select).toBeVisible();
		await expect(select).toHaveValue('10');

		await select.selectOption('50');
		await expect(page).toHaveURL(/per_page=50/);
		await expect(page).toHaveURL(/page=1\b/);

		// Reload from a fresh navigation: the server-rendered select must reflect
		// the URL, proving the loader (not just client state) honors per_page.
		await page.reload();
		await expect(page.getByTestId('page-size-select')).toHaveValue('50');
	});

	test('client support tickets: default page size caps rows at 10, switching reveals the rest', async ({
		page,
		context
	}) => {
		test.setTimeout(60_000);
		const api = await newApi();
		const client = await registerVerifyLogin(api, 'pg-tix');

		const deptRes = await api.get(`${API_BASE}/api/v1/ticket-departments`);
		expect(deptRes.ok()).toBeTruthy();
		const departments = (await deptRes.json()).data as Array<{ id: number }>;
		expect(departments.length).toBeGreaterThan(0);
		const departmentId = departments[0].id;

		for (let i = 0; i < 12; i++) {
			const res = await api.post(`${API_BASE}/api/v1/tickets`, {
				headers: authHeaders(client.accessToken),
				data: {
					department_id: departmentId,
					subject: `pg-tix ${unique()} #${i}`,
					priority: 'low',
					message: 'e2e pagination fixture'
				}
			});
			expect(res.ok(), `create ticket ${i}: ${await res.text()}`).toBeTruthy();
		}

		await setSessionCookies(context, client.accessToken, client.refreshToken);
		await api.dispose();

		await page.goto('/support');

		const ticketRows = page.getByTestId('ticket-list').locator('tbody tr');
		const select = page.getByTestId('page-size-select');
		await expect(select).toBeVisible();
		await expect(select).toHaveValue('10');
		await expect(ticketRows).toHaveCount(10);

		await select.selectOption('25');
		await expect(page).toHaveURL(/per_page=25/);
		await expect(page).toHaveURL(/page=1\b/);
		const rowCount = await ticketRows.count();
		expect(rowCount).toBeGreaterThan(10);
	});
});
