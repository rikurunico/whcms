/**
 * Shared Playwright test fixture: `page.goto` additionally waits for the
 * root layout's hydration marker (`<html data-hydrated>`) before returning.
 *
 * Without this, a spec's first interaction after navigation can race client
 * hydration on slow machines (CI runners serve the frontend through the dev
 * server, so first hits compile on demand): the element is already in the
 * server-rendered DOM, Playwright clicks it, but no event listener exists
 * yet and the click is silently lost - tabs never switch, modals never open.
 *
 * Specs import { test, expect } from './fixtures' instead of
 * '@playwright/test'; everything else re-exports unchanged.
 */
import { test as base, type Page } from '@playwright/test';

export * from '@playwright/test';

/**
 * Wait for the current document to be hydrated. Also useful mid-test: a form
 * submit that itself raced hydration falls back to a NATIVE post, which loads
 * a fresh document whose pure-JS controls need hydration all over again.
 * Best effort: error pages or non-app URLs may never hydrate.
 */
export async function waitForHydration(page: Page): Promise<void> {
	await page
		.waitForSelector('html[data-hydrated]', { state: 'attached', timeout: 15_000 })
		.catch(() => {});
}

export const test = base.extend({
	page: async ({ page }, use) => {
		const originalGoto = page.goto.bind(page);
		page.goto = (async (url: string, options?: Parameters<typeof originalGoto>[1]) => {
			const response = await originalGoto(url, options);
			await waitForHydration(page);
			return response;
		}) as typeof page.goto;
		await use(page);
	},
});
