/**
 * Slug prefill used while the user types a name/title: lowercase, NFKD
 * diacritic strip, runs of non-alphanumerics collapsed to single hyphens,
 * clamped to 64 chars. The prefilled input stays editable and the server
 * validates the final value.
 */
export function slugify(input: string): string {
	return input
		.toLowerCase()
		.normalize('NFKD')
		.replace(/[\u0300-\u036f]/g, '')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 64);
}
