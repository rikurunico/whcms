/**
 * Plain-text date formats shared by the admin tables: `dd/mm/yyyy` and
 * `dd/mm/yyyy hh:mm` in the viewer's local timezone, with an em dash for
 * missing or unparseable values. Use the <DateText> component where a
 * component fits; call these where only a string works (table cells,
 * <option> labels, chart axes).
 */

const pad2 = (n: number) => String(n).padStart(2, '0');

/** `dd/mm/yyyy`, or an em dash when the value is missing or unparseable. */
export function fmtDate(iso: string | null | undefined): string {
	if (!iso) return '—';
	const dt = new Date(iso);
	if (isNaN(dt.getTime())) return '—';
	return `${pad2(dt.getDate())}/${pad2(dt.getMonth() + 1)}/${dt.getFullYear()}`;
}

/** `dd/mm/yyyy hh:mm`, or an em dash when the value is missing or unparseable. */
export function fmtDateTime(iso: string | null | undefined): string {
	if (!iso) return '—';
	const dt = new Date(iso);
	if (isNaN(dt.getTime())) return '—';
	const date = `${pad2(dt.getDate())}/${pad2(dt.getMonth() + 1)}/${dt.getFullYear()}`;
	return `${date} ${pad2(dt.getHours())}:${pad2(dt.getMinutes())}`;
}

/** `12 Jan` (en-GB, UTC) from a `yyyy-mm-dd` day key; used for chart axis labels. */
export function fmtShortDate(day: string): string {
	const dt = new Date(day + 'T00:00:00Z');
	return dt.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', timeZone: 'UTC' });
}
