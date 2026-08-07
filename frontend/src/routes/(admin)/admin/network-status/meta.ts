/**
 * Shared enum option lists + display labels for the network status admin pages
 * (mirrors the domain enums: NetworkIssueType / Severity / Status).
 */
export const TYPES = ['scheduled', 'issue', 'outage'] as const;
export const SEVERITIES = ['minor', 'major', 'critical'] as const;
export const STATUSES = [
	'investigating',
	'identified',
	'monitoring',
	'resolved',
	'scheduled'
] as const;

export const typeLabels: Record<string, string> = {
	scheduled: 'Scheduled Maintenance',
	issue: 'Issue',
	outage: 'Outage'
};

export const severityLabels: Record<string, string> = {
	minor: 'Minor',
	major: 'Major',
	critical: 'Critical'
};

export const statusLabels: Record<string, string> = {
	investigating: 'Investigating',
	identified: 'Identified',
	monitoring: 'Monitoring',
	resolved: 'Resolved',
	scheduled: 'Scheduled'
};

/** hp-badge modifier per status (green/yellow/blue/red family). */
export function statusBadge(status: string): string {
	switch (status) {
		case 'resolved':
			return 'active'; // green
		case 'investigating':
			return 'terminated'; // red
		case 'monitoring':
			return 'suspended'; // blue
		case 'identified':
		case 'scheduled':
		default:
			return 'pending'; // yellow
	}
}

/** hp-badge modifier per severity. */
export function severityBadge(severity: string): string {
	switch (severity) {
		case 'critical':
			return 'terminated'; // red
		case 'major':
			return 'pending'; // yellow
		default:
			return 'inactive'; // gray
	}
}
