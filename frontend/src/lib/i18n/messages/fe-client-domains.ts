/** FE-CLIENT-DOMAINS i18n namespace - keys under `clientDomains.*`. */
import type { Locale } from '../i18n.svelte';

const messages: Partial<Record<Locale, Record<string, unknown>>> = {
	id: {
		clientDomains: {
			list: {
				title: 'Domain Saya',
				subtitle: 'Kelola domain terdaftar pada akun Anda.',
				searchPlaceholder: 'Cari nama domain…',
				statusLabel: 'Status',
				colDomain: 'Domain',
				colStatus: 'Status',
				colExpiry: 'Kedaluwarsa',
				colAmount: 'Biaya Perpanjangan',
				colAutoRenew: 'Perpanjang Otomatis',
				manage: 'Kelola',
				emptyTitle: 'Belum ada domain',
				emptyDesc: 'Anda belum memiliki domain terdaftar pada akun ini.',
				registerCta: 'Daftarkan Domain'
			},
			detail: {
				back: 'Kembali ke daftar domain',
				notFound: 'Domain tidak ditemukan atau Anda tidak memiliki akses.',
				overviewTitle: 'Informasi Domain',
				renewNow: 'Perpanjang Sekarang',
				renewTitle: 'Perpanjang Domain',
				renewPeriod: 'Periode perpanjangan',
				years: '{n} tahun',
				renewTotal: 'Total perkiraan',
				renewHint:
					'Invoice perpanjangan akan dibuat dan Anda akan diarahkan ke halaman pembayaran.',
				status: 'Status',
				registrationDate: 'Tanggal registrasi',
				expiryDate: 'Tanggal kedaluwarsa',
				nextDueDate: 'Jatuh tempo berikutnya',
				billingCycle: 'Siklus tagihan',
				recurringAmount: 'Biaya perpanjangan',
				autoRenew: 'Perpanjang otomatis',
				idProtection: 'Proteksi ID'
			},
			tabs: {
				overview: 'Ringkasan',
				nameservers: 'Nameserver',
				dns: 'DNS',
				epp: 'Kode EPP',
				contact: 'Kontak',
				addons: 'Tambahan'
			},
			ns: {
				title: 'Nameserver',
				desc: 'Atur nameserver domain Anda. Minimal 2 nameserver wajib diisi.',
				label: 'Nameserver {n}',
				optionalHint: 'Opsional',
				save: 'Simpan Nameserver',
				needTwo: 'Minimal 2 nameserver harus diisi.',
				invalidHost: 'Format nameserver tidak valid (contoh: ns1.contoh.com).',
				saved: 'Nameserver berhasil disimpan.'
			},
			dns: {
				title: 'Pengelola DNS',
				desc: 'Tambah, ubah, atau hapus record DNS di bawah, lalu simpan seluruh perubahan sekaligus.',
				type: 'Tipe',
				host: 'Host',
				value: 'Nilai',
				ttl: 'TTL',
				prio: 'Prioritas',
				add: 'Tambah Record',
				remove: 'Hapus',
				save: 'Simpan Perubahan',
				empty: 'Belum ada record DNS. Tambahkan record pertama Anda.',
				unsaved: 'Ada perubahan yang belum disimpan.',
				invalid: 'Data record DNS tidak valid.',
				invalidRow:
					'Record pada baris {row} tidak valid. Periksa tipe, host, nilai, TTL, dan prioritas.',
				saved: 'Record DNS berhasil disimpan.'
			},
			epp: {
				title: 'Kode EPP / Auth',
				desc: 'Kode EPP diperlukan untuk mentransfer domain ke registrar lain. Jaga kerahasiaan kode ini.',
				reveal: 'Tampilkan Kode EPP',
				codeLabel: 'Kode EPP',
				copy: 'Salin',
				copied: 'Kode EPP disalin ke clipboard.',
				unavailable: 'Kode EPP tidak tersedia saat ini.'
			},
			contact: {
				title: 'Kontak Registran',
				desc: 'Data pemilik (registrant) yang terdaftar untuk domain ini.',
				loadFailed:
					'Data kontak tidak dapat dimuat ({message}). Anda tetap dapat mengirim pembaruan.',
				firstName: 'Nama depan',
				lastName: 'Nama belakang',
				company: 'Perusahaan',
				email: 'Email',
				phone: 'Telepon',
				address: 'Alamat',
				city: 'Kota',
				state: 'Provinsi',
				postcode: 'Kode pos',
				country: 'Negara (kode ISO)',
				required: 'Wajib diisi.',
				invalidEmail: 'Format email tidak valid.',
				invalidCountry: 'Gunakan kode negara 2 huruf, contoh: ID.',
				save: 'Simpan Kontak',
				saved: 'Kontak registran berhasil disimpan.'
			},
			addons: {
				title: 'Tambahan Domain',
				desc: 'Aktifkan tambahan untuk domain ini. Biaya perpanjangan tahunan akan disesuaikan otomatis.',
				save: 'Simpan Tambahan',
				saved: 'Tambahan domain berhasil disimpan.',
				perYear: '/tahun',
				dnsRequiredTitle: 'Fitur DNS Management diperlukan',
				dnsRequiredDesc:
					'Aktifkan tambahan DNS Management pada tab Tambahan untuk mengelola record DNS domain ini.',
				goToAddons: 'Buka tab Tambahan'
			},
			cycle: {
				one_time: 'Sekali bayar',
				monthly: 'Bulanan',
				quarterly: '3 Bulanan',
				semiannually: '6 Bulanan',
				annually: 'Tahunan',
				biennially: '2 Tahunan'
			},
			toast: {
				autoRenewOn: 'Perpanjangan otomatis diaktifkan.',
				autoRenewOff: 'Perpanjangan otomatis dinonaktifkan.',
				renewCreated: 'Invoice perpanjangan berhasil dibuat.'
			},
			errors: {
				invalidRequest: 'Permintaan tidak valid.',
				actionFailed: 'Tindakan gagal. Silakan coba lagi.',
				loadFailed: 'Gagal memuat data: {message}'
			}
		}
	},
	en: {
		clientDomains: {
			list: {
				title: 'My Domains',
				subtitle: 'Manage the domains registered on your account.',
				searchPlaceholder: 'Search domain name…',
				statusLabel: 'Status',
				colDomain: 'Domain',
				colStatus: 'Status',
				colExpiry: 'Expiry',
				colAmount: 'Renewal Price',
				colAutoRenew: 'Auto Renew',
				manage: 'Manage',
				emptyTitle: 'No domains yet',
				emptyDesc: 'You have no registered domains on this account yet.',
				registerCta: 'Register a Domain'
			},
			detail: {
				back: 'Back to domain list',
				notFound: 'Domain not found or you do not have access.',
				overviewTitle: 'Domain Information',
				renewNow: 'Renew Now',
				renewTitle: 'Renew Domain',
				renewPeriod: 'Renewal period',
				years: '{n} year(s)',
				renewTotal: 'Estimated total',
				renewHint: 'A renewal invoice will be created and you will be redirected to pay it.',
				status: 'Status',
				registrationDate: 'Registration date',
				expiryDate: 'Expiry date',
				nextDueDate: 'Next due date',
				billingCycle: 'Billing cycle',
				recurringAmount: 'Renewal price',
				autoRenew: 'Auto renew',
				idProtection: 'ID protection'
			},
			tabs: {
				overview: 'Overview',
				nameservers: 'Nameservers',
				dns: 'DNS',
				epp: 'EPP Code',
				contact: 'Contact',
				addons: 'Add-ons'
			},
			ns: {
				title: 'Nameservers',
				desc: 'Configure the nameservers for your domain. At least 2 nameservers are required.',
				label: 'Nameserver {n}',
				optionalHint: 'Optional',
				save: 'Save Nameservers',
				needTwo: 'At least 2 nameservers are required.',
				invalidHost: 'Invalid nameserver format (example: ns1.example.com).',
				saved: 'Nameservers saved successfully.'
			},
			dns: {
				title: 'DNS Manager',
				desc: 'Add, edit, or delete DNS records below, then save all changes at once.',
				type: 'Type',
				host: 'Host',
				value: 'Value',
				ttl: 'TTL',
				prio: 'Priority',
				add: 'Add Record',
				remove: 'Delete',
				save: 'Save Changes',
				empty: 'No DNS records yet. Add your first record.',
				unsaved: 'You have unsaved changes.',
				invalid: 'Invalid DNS record data.',
				invalidRow: 'Record on row {row} is invalid. Check type, host, value, TTL, and priority.',
				saved: 'DNS records saved successfully.'
			},
			epp: {
				title: 'EPP / Auth Code',
				desc: 'The EPP code is required to transfer your domain to another registrar. Keep it secret.',
				reveal: 'Reveal EPP Code',
				codeLabel: 'EPP code',
				copy: 'Copy',
				copied: 'EPP code copied to clipboard.',
				unavailable: 'The EPP code is not available right now.'
			},
			contact: {
				title: 'Registrant Contact',
				desc: 'The registrant (owner) details registered for this domain.',
				loadFailed: 'Contact data could not be loaded ({message}). You can still submit an update.',
				firstName: 'First name',
				lastName: 'Last name',
				company: 'Company',
				email: 'Email',
				phone: 'Phone',
				address: 'Address',
				city: 'City',
				state: 'State/Province',
				postcode: 'Postcode',
				country: 'Country (ISO code)',
				required: 'This field is required.',
				invalidEmail: 'Invalid email format.',
				invalidCountry: 'Use a 2-letter country code, e.g. ID.',
				save: 'Save Contact',
				saved: 'Registrant contact saved successfully.'
			},
			addons: {
				title: 'Domain Add-ons',
				desc: 'Enable add-ons for this domain. The annual renewal cost is adjusted automatically.',
				save: 'Save Add-ons',
				saved: 'Domain add-ons saved successfully.',
				perYear: '/yr',
				dnsRequiredTitle: 'DNS Management add-on required',
				dnsRequiredDesc:
					'Enable the DNS Management add-on on the Add-ons tab to manage this domain’s DNS records.',
				goToAddons: 'Go to the Add-ons tab'
			},
			cycle: {
				one_time: 'One time',
				monthly: 'Monthly',
				quarterly: 'Quarterly',
				semiannually: 'Semi-annually',
				annually: 'Annually',
				biennially: 'Biennially'
			},
			toast: {
				autoRenewOn: 'Auto renew enabled.',
				autoRenewOff: 'Auto renew disabled.',
				renewCreated: 'Renewal invoice created successfully.'
			},
			errors: {
				invalidRequest: 'Invalid request.',
				actionFailed: 'The action failed. Please try again.',
				loadFailed: 'Failed to load data: {message}'
			}
		}
	}
};

export default messages;
