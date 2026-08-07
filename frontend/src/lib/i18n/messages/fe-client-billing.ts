/** FE-CLIENT-BILLING namespace - client billing area + public payment return page. */
import type { Locale } from '../i18n.svelte';

const messages: Partial<Record<Locale, Record<string, unknown>>> = {
	id: {
		clientBilling: {
			title: 'Tagihan',
			subtitle: 'Kelola invoice dan riwayat transaksi Anda.',
			tabs: {
				invoices: 'Invoice',
				transactions: 'Transaksi'
			},
			invoices: {
				empty: 'Belum ada invoice',
				emptyDesc: 'Invoice Anda akan tampil di sini setelah Anda melakukan pemesanan.',
				colDate: 'Tanggal',
				colStatus: 'Status',
				filterStatus: 'Status'
			},
			transactions: {
				empty: 'Belum ada transaksi',
				emptyDesc: 'Riwayat pembayaran Anda akan tampil di sini.',
				colDate: 'Tanggal',
				colInvoice: 'Invoice',
				colGateway: 'Gateway',
				colMethod: 'Metode',
				colFee: 'Biaya',
				colStatus: 'Status'
			},
			detail: {
				notFound: 'Invoice tidak ditemukan.',
				billedTo: 'Ditagihkan kepada',
				invoiceInfo: 'Informasi invoice',
				itemDescription: 'Deskripsi',
				itemAmount: 'Jumlah',
				paidBanner: 'Invoice ini sudah lunas.',
				unpaidBanner: 'Invoice ini belum dibayar. Jatuh tempo:',
				overdueBanner: 'Invoice ini telah melewati jatuh tempo:',
				cancelledBanner: 'Invoice ini telah dibatalkan.',
				refundedBanner: 'Pembayaran invoice ini telah dikembalikan.',
				draftBanner: 'Invoice ini masih berupa draf.',
				taxWithRate: 'Pajak ({rate}%)',
				creditApplied: 'Kredit terpakai',
				paymentTitle: 'Pembayaran',
				chooseMethod: 'Pilih metode pembayaran',
				fee: 'Biaya',
				feeIncluded: 'Sudah termasuk biaya admin {fee} dari penyedia pembayaran.',
				showMoreMethods: 'Lihat Lainnya ({count})',
				showFewerMethods: 'Sembunyikan',
				switchMethod: 'Ganti metode pembayaran',
				cancelSwitchMethod: 'Batal, kembali ke instruksi sebelumnya',
				creditOption: 'Saldo Kredit',
				creditOptionDesc: 'Bayar langsung memakai saldo kredit Anda',
				methodsError: 'Metode pembayaran tidak dapat dimuat. Silakan muat ulang halaman.',
				selectMethodFirst: 'Pilih metode pembayaran terlebih dahulu.',
				instructionsTitle: 'Instruksi pembayaran',
				vaNumber: 'Nomor Virtual Account',
				qrString: 'Kode QRIS',
				qrStringRaw: 'Tampilkan teks QRIS mentah',
				bankAccounts: 'Transfer ke salah satu rekening berikut',
				reference: 'Referensi',
				payAmount: 'Jumlah yang harus dibayar',
				expiresAt: 'Berlaku sampai',
				expiryMinutes: 'Berlaku {minutes} menit',
				copy: 'Salin',
				copied: 'Tersalin!',
				awaitingPayment: 'Menunggu pembayaran — status akan diperbarui otomatis.',
				paymentReceived: 'Pembayaran diterima. Terima kasih!',
				payFailed: 'Pembayaran gagal diproses. Silakan coba lagi.',
				previousTransactions: 'Riwayat transaksi invoice ini'
			},
			deposit: {
				description:
					'Deposit dana ke saldo kredit akun Anda. Kami akan membuat invoice deposit yang dapat Anda bayar dengan metode pembayaran yang tersedia.',
				amountLabel: 'Jumlah deposit (Rp)',
				minHint: 'Minimal Rp10.000,- tanpa desimal.',
				minError: 'Jumlah deposit minimal Rp10.000,-.',
				submit: 'Buat Invoice Deposit',
				created:
					'Invoice deposit berhasil dibuat. Silakan selesaikan pembayaran melalui halaman Tagihan.',
				failed: 'Deposit gagal diproses. Silakan coba lagi.'
			},
			return: {
				title: 'Status Pembayaran',
				processing: 'Pembayaran Anda sedang diproses…',
				processingHint:
					'Halaman ini memperbarui status secara otomatis. Mohon jangan tutup halaman ini.',
				paid: 'Pembayaran berhasil!',
				failedInfo:
					'Pembayaran belum terkonfirmasi. Anda dapat memeriksa status di halaman invoice.',
				viewInvoice: 'Lihat Invoice',
				backToBilling: 'Kembali ke Tagihan',
				missingOrder: 'Referensi pembayaran tidak ditemukan pada URL.'
			}
		}
	},
	en: {
		clientBilling: {
			title: 'Billing',
			subtitle: 'Manage your invoices and payment history.',
			tabs: {
				invoices: 'Invoices',
				transactions: 'Transactions'
			},
			invoices: {
				empty: 'No invoices yet',
				emptyDesc: 'Your invoices will appear here once you place an order.',
				colDate: 'Date',
				colStatus: 'Status',
				filterStatus: 'Status'
			},
			transactions: {
				empty: 'No transactions yet',
				emptyDesc: 'Your payment history will appear here.',
				colDate: 'Date',
				colInvoice: 'Invoice',
				colGateway: 'Gateway',
				colMethod: 'Method',
				colFee: 'Fee',
				colStatus: 'Status'
			},
			detail: {
				notFound: 'Invoice not found.',
				billedTo: 'Billed to',
				invoiceInfo: 'Invoice information',
				itemDescription: 'Description',
				itemAmount: 'Amount',
				paidBanner: 'This invoice has been paid.',
				unpaidBanner: 'This invoice is unpaid. Due date:',
				overdueBanner: 'This invoice is past its due date:',
				cancelledBanner: 'This invoice has been cancelled.',
				refundedBanner: 'This invoice has been refunded.',
				draftBanner: 'This invoice is still a draft.',
				taxWithRate: 'Tax ({rate}%)',
				creditApplied: 'Credit applied',
				paymentTitle: 'Payment',
				chooseMethod: 'Choose a payment method',
				fee: 'Fee',
				feeIncluded: 'Includes a {fee} admin fee charged by the payment provider.',
				showMoreMethods: 'Show More ({count})',
				showFewerMethods: 'Show Less',
				switchMethod: 'Switch payment method',
				cancelSwitchMethod: 'Cancel, back to previous instructions',
				creditOption: 'Credit Balance',
				creditOptionDesc: 'Pay instantly using your credit balance',
				methodsError: 'Payment methods could not be loaded. Please refresh the page.',
				selectMethodFirst: 'Please select a payment method first.',
				instructionsTitle: 'Payment instructions',
				vaNumber: 'Virtual Account number',
				qrString: 'QRIS code',
				qrStringRaw: 'Show raw QRIS text',
				bankAccounts: 'Transfer to one of the following accounts',
				reference: 'Reference',
				payAmount: 'Amount to pay',
				expiresAt: 'Valid until',
				expiryMinutes: 'Valid for {minutes} minutes',
				copy: 'Copy',
				copied: 'Copied!',
				awaitingPayment: 'Awaiting payment — the status refreshes automatically.',
				paymentReceived: 'Payment received. Thank you!',
				payFailed: 'The payment could not be processed. Please try again.',
				previousTransactions: 'Transactions for this invoice'
			},
			deposit: {
				description:
					'Deposit funds into your account credit balance. We will create a deposit invoice that you can pay with any available payment method.',
				amountLabel: 'Deposit amount (Rp)',
				minHint: 'Minimum Rp10.000,- whole rupiah only.',
				minError: 'The minimum deposit amount is Rp10.000,-.',
				submit: 'Create Deposit Invoice',
				created: 'Deposit invoice created. Please complete the payment from the Billing page.',
				failed: 'The deposit could not be processed. Please try again.'
			},
			return: {
				title: 'Payment Status',
				processing: 'Your payment is being processed…',
				processingHint: 'This page refreshes the status automatically. Please keep it open.',
				paid: 'Payment successful!',
				failedInfo:
					'The payment is not confirmed yet. You can check the status on the invoice page.',
				viewInvoice: 'View Invoice',
				backToBilling: 'Back to Billing',
				missingOrder: 'No payment reference was found in the URL.'
			}
		}
	}
};

export default messages;
