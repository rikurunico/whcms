/**
 * Canonical Rupiah nominal format for the whole app: `Rp1.000.000,-`.
 *
 * No space after "Rp", "." as thousands separator (id-ID locale), whole rupiah
 * only (money is int64 IDR - CONTRACTS.md §3), and a trailing ",-". Negative
 * amounts get a leading "-", e.g. `-Rp10.000,-`.
 *
 * Use the <MoneyText> / <CaPrice> components where a component fits; call this
 * directly for text-only spots (e.g. <option> labels, StatCard `sub` strings,
 * chart tooltips) where a component cannot be rendered.
 */
export function formatIDR(amount: number | null | undefined): string {
	const rounded = Math.round(Number(amount ?? 0));
	const safe = Number.isFinite(rounded) ? rounded : 0;
	const digits = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 0 }).format(
		Math.abs(safe)
	);
	return `${safe < 0 ? '-' : ''}Rp${digits},-`;
}
