/** FE-CLIENT-NOTIFICATIONS namespace - client email delivery history page. */
import type { Locale } from '../i18n.svelte';

const messages: Partial<Record<Locale, Record<string, unknown>>> = {
	id: {
		clientEmailHistory: {
			title: 'Riwayat Email',
			subtitle: 'Email yang pernah dikirim ke akun Anda.',
			loadFailed: 'Gagal memuat riwayat email',
			emptyTitle: 'Belum ada email',
			emptyDescription: 'Email yang dikirim ke akun Anda akan tampil di sini.',
			colDate: 'Tanggal',
			colSubject: 'Subjek',
			colTemplate: 'Jenis',
			colStatus: 'Status'
		}
	},
	en: {
		clientEmailHistory: {
			title: 'Email History',
			subtitle: 'Emails that have been sent to your account.',
			loadFailed: 'Failed to load email history',
			emptyTitle: 'No emails yet',
			emptyDescription: 'Emails sent to your account will appear here.',
			colDate: 'Date',
			colSubject: 'Subject',
			colTemplate: 'Type',
			colStatus: 'Status'
		}
	}
};

export default messages;
