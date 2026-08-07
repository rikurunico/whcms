/** Shared component prop types. */

/** DataTable column definition. `key` is a dot-free property name of the row object. */
export interface Column {
	key: string;
	label: string;
	sortable?: boolean;
	align?: 'left' | 'center' | 'right';
	/** Optional width utility/style hint, e.g. 'w-32'. */
	class?: string;
}

export type SortDir = 'asc' | 'desc';

export interface SelectOption {
	value: string;
	label: string;
}

export interface BreadcrumbItem {
	label: string;
	href?: string;
}

export interface TabItem {
	id: string;
	label: string;
	/** When set, the tab renders as a link instead of a button. */
	href?: string;
	disabled?: boolean;
}
