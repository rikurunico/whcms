/** Shared types/constants for the client domains routes (FE-CLIENT-DOMAINS). */

export const DOMAIN_TABS = ['overview', 'nameservers', 'dns', 'epp', 'contact', 'addons'] as const;
export type DomainTab = (typeof DOMAIN_TABS)[number];

/** CONTRACTS.md §4 domain status enum. */
export const DOMAIN_STATUSES = [
	'pending',
	'active',
	'pending_transfer',
	'expired',
	'cancelled'
] as const;

/** Matches what the registrar actually accepts (no TXT/NS/SRV/CAA; SPF is its
 *  own legacy record type there, not folded into TXT). */
export const DNS_TYPES = ['A', 'AAAA', 'CNAME', 'MX', 'SPF'] as const;

/** Mirrors backend `domain.Domain` JSON tags (entities.go). */
export interface DomainRow {
	id: number;
	client_id: number;
	registrar_id: number;
	name: string;
	status: string;
	registration_date: string | null;
	expiry_date: string | null;
	next_due_date: string | null;
	recurring_amount: number;
	billing_cycle: string;
	auto_renew: boolean;
	nameservers: unknown;
	id_protection: boolean;
	dns_management_enabled: boolean;
	email_forwarding_enabled: boolean;
	registrar_meta?: unknown;
	created_at?: string;
	updated_at?: string;
}

/** GET /api/v1/domains/addons response row (domain.DomainAddon JSON tags). */
export interface DomainAddonOption {
	key: string;
	name: string;
	price: number;
}

/** Mirrors `ports.DNSRecord` JSON tags. */
export interface DnsRecord {
	type: string;
	host: string;
	value: string;
	ttl: number;
	prio: number;
}

/** Mirrors `ports.RegistrantContact` JSON tags. */
export interface RegistrantContact {
	first_name: string;
	last_name: string;
	company: string;
	email: string;
	phone: string;
	address1: string;
	city: string;
	state: string;
	postcode: string;
	country: string;
}

export const CONTACT_FIELDS = [
	'first_name',
	'last_name',
	'company',
	'email',
	'phone',
	'address1',
	'city',
	'state',
	'postcode',
	'country'
] as const;
