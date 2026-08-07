/**
 * i18n namespace for FE-CLIENT-SUPPORT - client support ticket pages
 * (/support, /support/new, /support/[id]). Keys live under `supportfe.*`.
 */
const messages = {
	id: {
		supportfe: {
			list: {
				title: 'Tiket Dukungan',
				subtitle: 'Lihat dan kelola tiket bantuan Anda.',
				newTicket: 'Buka Tiket Baru',
				colNumber: 'No. Tiket',
				colSubject: 'Subjek',
				colDepartment: 'Departemen',
				colPriority: 'Prioritas',
				colStatus: 'Status',
				colLastReply: 'Balasan Terakhir',
				emptyTitle: 'Belum ada tiket',
				emptyDescription: 'Anda belum pernah membuka tiket dukungan.',
				loadFailed: 'Gagal memuat daftar tiket.'
			},
			priority: {
				low: 'Rendah',
				medium: 'Sedang',
				high: 'Tinggi'
			},
			form: {
				title: 'Buka Tiket Baru',
				subtitle: 'Jelaskan masalah Anda dan tim kami akan membantu.',
				department: 'Departemen',
				departmentPlaceholder: 'Pilih departemen',
				subject: 'Subjek',
				subjectPlaceholder: 'Ringkasan singkat masalah Anda',
				priority: 'Prioritas',
				message: 'Pesan',
				messagePlaceholder: 'Jelaskan masalah Anda selengkap mungkin…',
				attachments: 'Lampiran',
				attachmentsHint:
					'Opsional. Maksimal {size} MB per file. Ekstensi umum: jpg, png, pdf, zip, txt, log.',
				submit: 'Kirim Tiket',
				required: 'Wajib diisi.',
				invalidPriority: 'Prioritas tidak valid.',
				departmentsLoadFailed: 'Gagal memuat daftar departemen.',
				createFailed: 'Gagal membuat tiket. Silakan coba lagi.',
				created: 'Tiket berhasil dibuat.'
			},
			detail: {
				department: 'Departemen',
				priority: 'Prioritas',
				opened: 'Dibuka',
				lastReply: 'Balasan terakhir',
				status: 'Status',
				close: 'Tutup Tiket',
				closeConfirmTitle: 'Tutup tiket ini?',
				closeConfirmMessage: 'Tiket yang sudah ditutup tidak dapat dibalas lagi.',
				closed: 'Tiket berhasil ditutup.',
				closeFailed: 'Gagal menutup tiket.',
				closedNotice: 'Tiket ini sudah ditutup dan tidak dapat dibalas lagi.',
				thread: 'Percakapan',
				emptyThread: 'Belum ada pesan pada tiket ini.',
				replyTitle: 'Balas Tiket',
				replyPlaceholder: 'Tulis balasan Anda…',
				replySubmit: 'Kirim Balasan',
				replySent: 'Balasan terkirim.',
				replyFailed: 'Gagal mengirim balasan.',
				messageRequired: 'Pesan wajib diisi.',
				attachmentsLabel: 'Lampiran',
				you: 'Anda',
				staff: 'Staf',
				notFound: 'Tiket tidak ditemukan.',
				loadFailed: 'Gagal memuat tiket.',
				backToList: 'Kembali ke daftar tiket'
			}
		}
	},
	en: {
		supportfe: {
			list: {
				title: 'Support Tickets',
				subtitle: 'View and manage your support tickets.',
				newTicket: 'Open New Ticket',
				colNumber: 'Ticket #',
				colSubject: 'Subject',
				colDepartment: 'Department',
				colPriority: 'Priority',
				colStatus: 'Status',
				colLastReply: 'Last Reply',
				emptyTitle: 'No tickets yet',
				emptyDescription: 'You have not opened any support tickets yet.',
				loadFailed: 'Failed to load tickets.'
			},
			priority: {
				low: 'Low',
				medium: 'Medium',
				high: 'High'
			},
			form: {
				title: 'Open New Ticket',
				subtitle: 'Describe your issue and our team will help you.',
				department: 'Department',
				departmentPlaceholder: 'Select a department',
				subject: 'Subject',
				subjectPlaceholder: 'A short summary of your issue',
				priority: 'Priority',
				message: 'Message',
				messagePlaceholder: 'Describe your issue in as much detail as possible…',
				attachments: 'Attachments',
				attachmentsHint:
					'Optional. Max {size} MB per file. Common extensions: jpg, png, pdf, zip, txt, log.',
				submit: 'Submit Ticket',
				required: 'This field is required.',
				invalidPriority: 'Invalid priority.',
				departmentsLoadFailed: 'Failed to load departments.',
				createFailed: 'Failed to create the ticket. Please try again.',
				created: 'Ticket created successfully.'
			},
			detail: {
				department: 'Department',
				priority: 'Priority',
				opened: 'Opened',
				lastReply: 'Last reply',
				status: 'Status',
				close: 'Close Ticket',
				closeConfirmTitle: 'Close this ticket?',
				closeConfirmMessage: 'A closed ticket can no longer be replied to.',
				closed: 'Ticket closed successfully.',
				closeFailed: 'Failed to close the ticket.',
				closedNotice: 'This ticket is closed and can no longer be replied to.',
				thread: 'Conversation',
				emptyThread: 'No messages on this ticket yet.',
				replyTitle: 'Reply to Ticket',
				replyPlaceholder: 'Write your reply…',
				replySubmit: 'Send Reply',
				replySent: 'Reply sent.',
				replyFailed: 'Failed to send the reply.',
				messageRequired: 'Message is required.',
				attachmentsLabel: 'Attachments',
				you: 'You',
				staff: 'Staff',
				notFound: 'Ticket not found.',
				loadFailed: 'Failed to load the ticket.',
				backToList: 'Back to ticket list'
			}
		}
	}
};

export default messages;
