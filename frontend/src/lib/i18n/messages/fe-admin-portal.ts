/**
 * FE-ADMIN-PORTAL namespace: admin management screens for the portal content
 * features: announcements, knowledgebase (categories + articles), and network
 * status. This is an ADDITIVE feature dictionary auto-merged by i18n.svelte.ts;
 * it never edits the shared id.ts / en.ts / portal.ts.
 *
 * The HostPanel admin theme labels its chrome in English inline, so these
 * strings are primarily the validation/error-key catalogue that the page
 * server actions return via `errorKey` (mirroring adcatalog.* on the coupons
 * and product-groups pages) plus the page-level chrome for both locales.
 */
import type { Locale } from '../i18n.svelte';

const messages: Partial<Record<Locale, Record<string, unknown>>> = {
	id: {
		adminPortal: {
			announcements: {
				title: 'Pengumuman',
				subtitle: 'Kelola pengumuman & berita portal yang tampil untuk klien.',
				create: 'Pengumuman Baru',
				searchPlaceholder: 'Cari judul pengumuman…',
				published: 'Terbit',
				draft: 'Draf',
				fillRequired: 'Judul wajib diisi.',
				saveFailed: 'Gagal menyimpan pengumuman.',
				deleteFailed: 'Gagal menghapus pengumuman.',
				saved: 'Pengumuman berhasil disimpan.',
				deleted: 'Pengumuman berhasil dihapus.'
			},
			kb: {
				categoriesTitle: 'Kategori Basis Pengetahuan',
				categoriesSubtitle: 'Kelola kategori yang mengelompokkan artikel basis pengetahuan.',
				articlesTitle: 'Artikel Basis Pengetahuan',
				articlesSubtitle: 'Kelola artikel bantuan yang tampil di portal klien.',
				createCategory: 'Kategori Baru',
				createArticle: 'Artikel Baru',
				categoryFillRequired: 'Nama kategori wajib diisi.',
				categorySaveFailed: 'Gagal menyimpan kategori.',
				categoryDeleteFailed: 'Gagal menghapus kategori.',
				categoryHasArticles: 'Kategori masih memiliki artikel dan tidak dapat dihapus.',
				categorySaved: 'Kategori berhasil disimpan.',
				categoryDeleted: 'Kategori berhasil dihapus.',
				articleFillRequired: 'Kategori dan judul wajib diisi.',
				articleSaveFailed: 'Gagal menyimpan artikel.',
				articleDeleteFailed: 'Gagal menghapus artikel.',
				articleSaved: 'Artikel berhasil disimpan.',
				articleDeleted: 'Artikel berhasil dihapus.'
			},
			network: {
				title: 'Status Jaringan',
				subtitle: 'Kelola pemeliharaan terjadwal, gangguan, dan pemadaman.',
				create: 'Entri Baru',
				searchPlaceholder: 'Cari judul entri…',
				fillRequired: 'Judul wajib diisi.',
				saveFailed: 'Gagal menyimpan entri status jaringan.',
				deleteFailed: 'Gagal menghapus entri status jaringan.',
				saved: 'Entri status jaringan berhasil disimpan.',
				deleted: 'Entri status jaringan berhasil dihapus.'
			}
		}
	},
	en: {
		adminPortal: {
			announcements: {
				title: 'Announcements',
				subtitle: 'Manage portal announcements & news shown to clients.',
				create: 'New Announcement',
				searchPlaceholder: 'Search announcement title…',
				published: 'Published',
				draft: 'Draft',
				fillRequired: 'Title is required.',
				saveFailed: 'Failed to save the announcement.',
				deleteFailed: 'Failed to delete the announcement.',
				saved: 'Announcement saved successfully.',
				deleted: 'Announcement deleted successfully.'
			},
			kb: {
				categoriesTitle: 'Knowledgebase Categories',
				categoriesSubtitle: 'Manage the categories that group knowledgebase articles.',
				articlesTitle: 'Knowledgebase Articles',
				articlesSubtitle: 'Manage the help articles shown in the client portal.',
				createCategory: 'New Category',
				createArticle: 'New Article',
				categoryFillRequired: 'Category name is required.',
				categorySaveFailed: 'Failed to save the category.',
				categoryDeleteFailed: 'Failed to delete the category.',
				categoryHasArticles: 'This category still has articles and cannot be deleted.',
				categorySaved: 'Category saved successfully.',
				categoryDeleted: 'Category deleted successfully.',
				articleFillRequired: 'Category and title are required.',
				articleSaveFailed: 'Failed to save the article.',
				articleDeleteFailed: 'Failed to delete the article.',
				articleSaved: 'Article saved successfully.',
				articleDeleted: 'Article deleted successfully.'
			},
			network: {
				title: 'Network Status',
				subtitle: 'Manage scheduled maintenance, issues and outages.',
				create: 'New Entry',
				searchPlaceholder: 'Search entry title…',
				fillRequired: 'Title is required.',
				saveFailed: 'Failed to save the network status entry.',
				deleteFailed: 'Failed to delete the network status entry.',
				saved: 'Network status entry saved successfully.',
				deleted: 'Network status entry deleted successfully.'
			}
		}
	}
};

export default messages;
