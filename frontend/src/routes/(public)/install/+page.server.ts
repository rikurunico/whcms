import { apiFetch } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

/** Shape shared by both installer phases (backend/internal/installer and
 *  backend/internal/modules/install) - see docs/CONTRACTS.md §15. */
export interface InstallStatus {
	config_ready: boolean;
	installed: boolean;
	missing?: string[];
	reason?: string;
}

export const load: PageServerLoad = async (event) => {
	const res = await apiFetch<InstallStatus>(event, '/api/v1/install/status');
	const status: InstallStatus = res.data ?? {
		config_ready: false,
		installed: false,
		reason: res.error?.message
	};

	if (status.installed) {
		redirect(303, '/login');
	}

	return { status };
};

function formValue(form: FormData, key: string): string {
	return String(form.get(key) ?? '').trim();
}

function buildDatabaseUrl(form: FormData): string {
	const host = formValue(form, 'db_host') || 'localhost';
	const port = formValue(form, 'db_port') || '5432';
	const user = formValue(form, 'db_user') || 'root';
	const password = formValue(form, 'db_password');
	const name = formValue(form, 'db_name') || 'whmcs';
	const sslmode = formValue(form, 'db_sslmode') || 'disable';
	const auth = password ? `${user}:${encodeURIComponent(password)}` : user;
	return `postgres://${auth}@${host}:${port}/${name}?sslmode=${sslmode}`;
}

export const actions: Actions = {
	testDb: async (event) => {
		const form = await event.request.formData();
		const res = await apiFetch(event, '/api/v1/install/bootstrap/test-db', {
			method: 'POST',
			body: { database_url: buildDatabaseUrl(form) }
		});
		if (res.error) return fail(422, { action: 'testDb', ok: false, message: res.error.message });
		return { action: 'testDb', ok: true };
	},

	testRedis: async (event) => {
		const form = await event.request.formData();
		const res = await apiFetch(event, '/api/v1/install/bootstrap/test-redis', {
			method: 'POST',
			body: {
				redis_addr: formValue(form, 'redis_addr'),
				redis_password: formValue(form, 'redis_password')
			}
		});
		if (res.error) return fail(422, { action: 'testRedis', ok: false, message: res.error.message });
		return { action: 'testRedis', ok: true };
	},

	testS3: async (event) => {
		const form = await event.request.formData();
		const res = await apiFetch(event, '/api/v1/install/bootstrap/test-s3', {
			method: 'POST',
			body: {
				rustfs_endpoint: formValue(form, 'rustfs_endpoint'),
				rustfs_access_key: formValue(form, 'rustfs_access_key'),
				rustfs_secret_key: formValue(form, 'rustfs_secret_key'),
				rustfs_bucket: formValue(form, 'rustfs_bucket'),
				rustfs_use_ssl: form.get('rustfs_use_ssl') === 'on'
			}
		});
		if (res.error) return fail(422, { action: 'testS3', ok: false, message: res.error.message });
		return { action: 'testS3', ok: true };
	},

	testMail: async (event) => {
		const form = await event.request.formData();
		const driver = formValue(form, 'mail_driver') || 'log';
		const res = await apiFetch(event, '/api/v1/install/bootstrap/test-mail', {
			method: 'POST',
			body: {
				mail_driver: driver,
				smtp_host: formValue(form, 'smtp_host'),
				smtp_port: Number(formValue(form, 'smtp_port') || '587'),
				smtp_user: formValue(form, 'smtp_user'),
				smtp_pass: formValue(form, 'smtp_pass'),
				mail_http_url: formValue(form, 'mail_http_url'),
				test_recipient: formValue(form, 'mail_test_recipient')
			}
		});
		if (res.error) return fail(422, { action: 'testMail', ok: false, message: res.error.message });
		return { action: 'testMail', ok: true };
	},

	saveConfig: async (event) => {
		const form = await event.request.formData();
		const res = await apiFetch(event, '/api/v1/install/bootstrap/save', {
			method: 'POST',
			body: {
				app_env: 'production',
				app_base_url: formValue(form, 'app_base_url'),
				frontend_url: formValue(form, 'frontend_url'),
				database_url: buildDatabaseUrl(form),
				redis_addr: formValue(form, 'redis_addr'),
				redis_password: formValue(form, 'redis_password'),
				rustfs_endpoint: formValue(form, 'rustfs_endpoint'),
				rustfs_access_key: formValue(form, 'rustfs_access_key'),
				rustfs_secret_key: formValue(form, 'rustfs_secret_key'),
				rustfs_bucket: formValue(form, 'rustfs_bucket'),
				rustfs_use_ssl: form.get('rustfs_use_ssl') === 'on',
				mail_driver: formValue(form, 'mail_driver') || 'log',
				smtp_host: formValue(form, 'smtp_host'),
				smtp_port: Number(formValue(form, 'smtp_port') || '587'),
				smtp_user: formValue(form, 'smtp_user'),
				smtp_pass: formValue(form, 'smtp_pass'),
				mail_http_url: formValue(form, 'mail_http_url')
			}
		});
		if (res.error)
			return fail(422, { action: 'saveConfig', ok: false, message: res.error.message });
		return { action: 'saveConfig', ok: true };
	},

	createAdmin: async (event) => {
		const form = await event.request.formData();
		const email = formValue(form, 'email');
		const password = String(form.get('password') ?? '');
		const res = await apiFetch(event, '/api/v1/install/admin', {
			method: 'POST',
			body: { email, password }
		});
		if (res.error)
			return fail(res.status >= 400 ? res.status : 422, {
				action: 'createAdmin',
				ok: false,
				message: res.error.message,
				email
			});
		return { action: 'createAdmin', ok: true };
	},

	saveSettings: async (event) => {
		const form = await event.request.formData();
		const res = await apiFetch(event, '/api/v1/install/settings', {
			method: 'POST',
			body: {
				company_name: formValue(form, 'company_name'),
				company_email: formValue(form, 'company_email'),
				company_address: formValue(form, 'company_address')
			}
		});
		if (res.error)
			return fail(422, { action: 'saveSettings', ok: false, message: res.error.message });
		redirect(303, '/login');
	}
};
