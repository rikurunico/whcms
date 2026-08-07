/**
 * FE-CLIENT-SERVICES i18n namespace.
 * All keys live under the unique top-level key `clientsvc`.
 */
import type { Locale } from '../i18n.svelte';

const messages: Partial<Record<Locale, Record<string, unknown>>> = {
	id: {
		clientsvc: {
			list: {
				title: 'Layanan Saya',
				subtitle: 'Semua layanan hosting dan produk yang Anda miliki.',
				colProduct: 'Produk',
				colDomain: 'Domain',
				colPrice: 'Harga',
				colNextDue: 'Jatuh Tempo',
				colStatus: 'Status',
				searchPlaceholder: 'Cari domain atau produk…',
				empty: 'Belum ada layanan',
				emptyDescription: 'Anda belum memiliki layanan. Pesan produk pertama Anda sekarang.',
				orderNow: 'Pesan Layanan',
				productFallback: 'Produk #{id}'
			},
			detail: {
				title: 'Detail Layanan',
				overview: 'Ringkasan Layanan',
				product: 'Produk',
				domain: 'Domain',
				username: 'Username',
				server: 'Server',
				registrationDate: 'Tanggal registrasi',
				nextDueDate: 'Jatuh tempo berikutnya',
				billingCycle: 'Siklus tagihan',
				recurringAmount: 'Biaya berulang',
				setupFee: 'Biaya setup',
				status: 'Status',
				notes: 'Catatan',
				actionsTitle: 'Aksi Layanan',
				noActions: 'Tidak ada aksi yang tersedia untuk layanan ini.',
				notFound: 'Layanan tidak ditemukan',
				notFoundDescription: 'Layanan yang Anda cari tidak ada atau bukan milik akun Anda.',
				backToList: 'Kembali ke Daftar Layanan',
				suspendedTitle: 'Layanan ini sedang ditangguhkan',
				suspendedReason: 'Alasan: {reason}',
				pendingUpgradeTitle: 'Upgrade menunggu pembayaran',
				pendingUpgradeBody: 'Permintaan upgrade Anda akan diproses setelah invoice dibayar.',
				payUpgradeInvoice: 'Bayar Invoice Upgrade',
				renewalUnpaidTitle: 'Invoice perpanjangan belum dibayar',
				renewalUnpaidBody: 'Segera lakukan pembayaran agar layanan Anda tidak ditangguhkan.',
				payRenewalInvoice: 'Bayar Invoice Perpanjangan'
			},
			actions: {
				changePassword: 'Ubah Kata Sandi',
				changePasswordTitle: 'Ubah Kata Sandi Layanan',
				newPassword: 'Kata sandi baru',
				confirmPassword: 'Konfirmasi kata sandi',
				passwordHint: 'Minimal 8 karakter.',
				passwordChanged: 'Kata sandi layanan berhasil diubah.',
				sso: 'Login ke Panel',
				ssoReadyTitle: 'Tautan login panel siap',
				ssoReadyBody: 'Jika panel tidak terbuka otomatis, gunakan tautan berikut (sekali pakai).',
				ssoOpen: 'Buka Panel',
				upgrade: 'Upgrade / Ubah Paket',
				upgradeTitle: 'Upgrade Layanan',
				upgradeProduct: 'Pilih produk',
				upgradeProductPlaceholder: 'Pilih produk…',
				upgradeCycle: 'Siklus tagihan',
				upgradePrice: 'Harga baru',
				upgradeSetupFee: 'Biaya setup',
				upgradeHint: 'Invoice akan dibuat dan upgrade diproses setelah pembayaran diterima.',
				upgradeSubmit: 'Buat Invoice Upgrade',
				upgradeRequested: 'Permintaan upgrade berhasil dibuat.',
				upgradeNoProducts: 'Daftar produk tidak tersedia saat ini. Coba lagi nanti.',
				cancel: 'Ajukan Pembatalan',
				cancelTitle: 'Batalkan Layanan',
				cancelMode: 'Waktu pembatalan',
				cancelImmediate: 'Segera',
				cancelImmediateHint: 'Layanan dihentikan segera setelah permintaan diproses.',
				cancelEndOfTerm: 'Akhir periode tagihan',
				cancelEndOfTermHint: 'Layanan tetap aktif hingga akhir periode berjalan.',
				cancelSubmit: 'Kirim Permintaan Pembatalan',
				cancelSuccess: 'Permintaan pembatalan berhasil dikirim.'
			},
			cycle: {
				one_time: 'Sekali Bayar',
				monthly: 'Bulanan',
				quarterly: 'Per 3 Bulan',
				semiannually: 'Per 6 Bulan',
				annually: 'Tahunan',
				biennially: 'Per 2 Tahun'
			},
			errors: {
				passwordTooShort: 'Kata sandi minimal 8 karakter.',
				passwordMismatch: 'Konfirmasi kata sandi tidak cocok.',
				invalidMode: 'Pilih waktu pembatalan terlebih dahulu.',
				invalidUpgrade: 'Pilih produk dan siklus tagihan terlebih dahulu.',
				ssoUnavailable: 'Login panel tidak tersedia untuk layanan ini.',
				actionFailed: 'Aksi gagal diproses. Silakan coba lagi.'
			}
		}
	},
	en: {
		clientsvc: {
			list: {
				title: 'My Services',
				subtitle: 'All hosting services and products you own.',
				colProduct: 'Product',
				colDomain: 'Domain',
				colPrice: 'Price',
				colNextDue: 'Next Due',
				colStatus: 'Status',
				searchPlaceholder: 'Search domain or product…',
				empty: 'No services yet',
				emptyDescription: 'You do not have any services yet. Order your first product now.',
				orderNow: 'Order Services',
				productFallback: 'Product #{id}'
			},
			detail: {
				title: 'Service Detail',
				overview: 'Service Overview',
				product: 'Product',
				domain: 'Domain',
				username: 'Username',
				server: 'Server',
				registrationDate: 'Registration date',
				nextDueDate: 'Next due date',
				billingCycle: 'Billing cycle',
				recurringAmount: 'Recurring amount',
				setupFee: 'Setup fee',
				status: 'Status',
				notes: 'Notes',
				actionsTitle: 'Service Actions',
				noActions: 'No actions are available for this service.',
				notFound: 'Service not found',
				notFoundDescription:
					'The service you are looking for does not exist or does not belong to your account.',
				backToList: 'Back to Services',
				suspendedTitle: 'This service is currently suspended',
				suspendedReason: 'Reason: {reason}',
				pendingUpgradeTitle: 'Upgrade awaiting payment',
				pendingUpgradeBody: 'Your upgrade request will be processed once the invoice is paid.',
				payUpgradeInvoice: 'Pay Upgrade Invoice',
				renewalUnpaidTitle: 'Renewal invoice unpaid',
				renewalUnpaidBody: 'Please pay soon to avoid service suspension.',
				payRenewalInvoice: 'Pay Renewal Invoice'
			},
			actions: {
				changePassword: 'Change Password',
				changePasswordTitle: 'Change Service Password',
				newPassword: 'New password',
				confirmPassword: 'Confirm password',
				passwordHint: 'Minimum 8 characters.',
				passwordChanged: 'Service password changed successfully.',
				sso: 'Login to Panel',
				ssoReadyTitle: 'Panel login link ready',
				ssoReadyBody: 'If the panel did not open automatically, use this one-time link.',
				ssoOpen: 'Open Panel',
				upgrade: 'Upgrade / Change Package',
				upgradeTitle: 'Upgrade Service',
				upgradeProduct: 'Choose product',
				upgradeProductPlaceholder: 'Choose a product…',
				upgradeCycle: 'Billing cycle',
				upgradePrice: 'New price',
				upgradeSetupFee: 'Setup fee',
				upgradeHint: 'An invoice will be created and the upgrade is processed once paid.',
				upgradeSubmit: 'Create Upgrade Invoice',
				upgradeRequested: 'Upgrade request created successfully.',
				upgradeNoProducts: 'The product list is unavailable right now. Try again later.',
				cancel: 'Request Cancellation',
				cancelTitle: 'Cancel Service',
				cancelMode: 'Cancellation timing',
				cancelImmediate: 'Immediate',
				cancelImmediateHint: 'The service is terminated as soon as the request is processed.',
				cancelEndOfTerm: 'End of billing period',
				cancelEndOfTermHint: 'The service stays active until the current period ends.',
				cancelSubmit: 'Submit Cancellation Request',
				cancelSuccess: 'Cancellation request submitted successfully.'
			},
			cycle: {
				one_time: 'One Time',
				monthly: 'Monthly',
				quarterly: 'Quarterly',
				semiannually: 'Semi-Annually',
				annually: 'Annually',
				biennially: 'Biennially'
			},
			errors: {
				passwordTooShort: 'Password must be at least 8 characters.',
				passwordMismatch: 'Password confirmation does not match.',
				invalidMode: 'Please choose a cancellation timing first.',
				invalidUpgrade: 'Please choose a product and billing cycle first.',
				ssoUnavailable: 'Panel login is not available for this service.',
				actionFailed: 'The action could not be processed. Please try again.'
			}
		}
	}
};

export default messages;
