/** Shared date-range parsing for the admin report pages. */

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export function isoDate(d: Date): string {
	return d.toISOString().slice(0, 10);
}

/** Default range: first day of the current month -> today (UTC dates). */
export function defaultRange(): { from: string; to: string } {
	const now = new Date();
	const monthStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
	return { from: isoDate(monthStart), to: isoDate(now) };
}

export function parseRange(url: URL): { from: string; to: string } {
	const def = defaultRange();
	const rawFrom = url.searchParams.get('from') ?? '';
	const rawTo = url.searchParams.get('to') ?? '';
	return {
		from: DATE_RE.test(rawFrom) ? rawFrom : def.from,
		to: DATE_RE.test(rawTo) ? rawTo : def.to
	};
}

export type GroupBy = 'day' | 'month';

export function parseGroupBy(url: URL): GroupBy {
	return url.searchParams.get('group_by') === 'month' ? 'month' : 'day';
}
