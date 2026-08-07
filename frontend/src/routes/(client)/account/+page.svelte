<script lang="ts">
	import { enhance } from '$app/forms';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		Alert,
		Breadcrumb,
		ConfirmDialog,
		DataTable,
		DateText,
		FormField,
		LoadingButton,
		Modal,
		MoneyText,
		StatusBadge,
		Tabs,
		toast,
		type Column,
		type TabItem
	} from '$lib/components';
	import { t } from '$lib/i18n';
	import { untrack } from 'svelte';
	import type { PageProps } from './$types';

	let { data, form }: PageProps = $props();

	type ContactRow = (typeof data.contacts)[number];

	const tabItems: TabItem[] = $derived([
		{ id: 'profile', label: t('clientcore.account.tabProfile'), href: '/account?tab=profile' },
		{ id: 'security', label: t('clientcore.account.tabSecurity'), href: '/account?tab=security' },
		{ id: 'contacts', label: t('clientcore.account.tabContacts'), href: '/account?tab=contacts' },
		{ id: 'credit', label: t('clientcore.account.tabCredit'), href: '/account?tab=credit' }
	]);

	/** Per-field errors may be i18n keys (own validation) or raw backend messages. */
	function fieldError(errors: Record<string, string> | undefined, key: string): string | undefined {
		const v = errors?.[key];
		if (!v) return undefined;
		return v.startsWith('clientcore.') ? t(v) : v;
	}

	/** Top-level action messages may be i18n keys (own fallbacks) or raw backend messages. */
	function serverText(v: string): string {
		return v.startsWith('clientcore.') ? t(v) : v;
	}

	async function copyText(text: string) {
		try {
			await navigator.clipboard.writeText(text);
			toast.success(t('clientcore.common.copied'));
		} catch {
			toast.error(t('toast.error'));
		}
	}

	// Profile tab
	let pv = $state(
		untrack(() => ({
			first_name: form?.profileValues?.first_name ?? data.profile.first_name,
			last_name: form?.profileValues?.last_name ?? data.profile.last_name,
			company: form?.profileValues?.company ?? data.profile.company,
			address1: form?.profileValues?.address1 ?? data.profile.address1,
			address2: form?.profileValues?.address2 ?? data.profile.address2,
			city: form?.profileValues?.city ?? data.profile.city,
			state: form?.profileValues?.state ?? data.profile.state,
			postcode: form?.profileValues?.postcode ?? data.profile.postcode,
			country: form?.profileValues?.country ?? data.profile.country,
			phone: form?.profileValues?.phone ?? data.profile.phone
		}))
	);
	let profileSubmitting = $state(false);

	// Security tab
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let passwordSubmitting = $state(false);

	let setupSubmitting = $state(false);
	let enableSubmitting = $state(false);
	let disableSubmitting = $state(false);
	let enableCode = $state('');
	let disableCode = $state('');
	let disablePassword = $state('');
	let disableOpen = $state(false);
	let disableFormEl = $state<HTMLFormElement | null>(null);

	let twofaSetupLocal = $state<{ secret: string; otpauth: string } | null>(null);
	const twofaSetup = $derived(form?.twofaSetup ?? twofaSetupLocal);
	$effect(() => {
		if (form?.twofaSetup) {
			twofaSetupLocal = form.twofaSetup;
		} else if (form?.twofaEnabled || form?.twofaDisabled) {
			twofaSetupLocal = null;
		}
	});

	// Contacts tab
	interface ContactFormState {
		id: number;
		first_name: string;
		last_name: string;
		email: string;
		phone: string;
		perm_invoices: boolean;
		perm_services: boolean;
		perm_domains: boolean;
		perm_tickets: boolean;
	}

	function emptyContact(): ContactFormState {
		return {
			id: 0,
			first_name: '',
			last_name: '',
			email: '',
			phone: '',
			perm_invoices: false,
			perm_services: false,
			perm_domains: false,
			perm_tickets: false
		};
	}

	let cf = $state<ContactFormState>(
		untrack(() => (form?.contactValues ? { ...form.contactValues } : emptyContact()))
	);
	let contactModalOpen = $state(untrack(() => Boolean(form?.contactErrors && form?.contactValues)));
	let contactSubmitting = $state(false);

	function openAddContact() {
		cf = emptyContact();
		contactModalOpen = true;
	}

	function openEditContact(c: ContactRow) {
		cf = {
			id: c.id,
			first_name: c.first_name,
			last_name: c.last_name,
			email: c.email,
			phone: c.phone,
			perm_invoices: c.permissions?.invoices ?? false,
			perm_services: c.permissions?.services ?? false,
			perm_domains: c.permissions?.domains ?? false,
			perm_tickets: c.permissions?.tickets ?? false
		};
		contactModalOpen = true;
	}

	let deleteOpen = $state(false);
	let deleteTarget = $state<ContactRow | null>(null);
	let deleteSubmitting = $state(false);
	let deleteFormEl = $state<HTMLFormElement | null>(null);

	function askDelete(c: ContactRow) {
		deleteTarget = c;
		deleteOpen = true;
	}

	function permSummary(p: Record<string, boolean> | null | undefined): string {
		if (!p) return '—';
		const on: string[] = [];
		if (p.invoices) on.push(t('clientcore.account.permInvoices'));
		if (p.services) on.push(t('clientcore.account.permServices'));
		if (p.domains) on.push(t('clientcore.account.permDomains'));
		if (p.tickets) on.push(t('clientcore.account.permTickets'));
		return on.length > 0 ? on.join(', ') : '—';
	}

	let contactColumns: Column[] = $derived([
		{ key: 'name', label: t('clientcore.account.contactName') },
		{ key: 'email', label: t('auth.email') },
		{ key: 'phone', label: t('clientcore.account.phone') },
		{ key: 'permissions', label: t('clientcore.account.permissions') },
		{ key: 'actions', label: t('clientcore.common.actions'), align: 'right' }
	]);

	// Credit tab
	let ledgerColumns: Column[] = $derived([
		{ key: 'created_at', label: t('clientcore.common.date') },
		{ key: 'reason', label: t('clientcore.account.colReason') },
		{ key: 'delta', label: t('clientcore.account.colDelta'), align: 'right' },
		{ key: 'balance_after', label: t('clientcore.account.colBalance'), align: 'right' }
	]);

	function gotoLedgerPage(p: number) {
		const url = new URL(page.url);
		url.searchParams.set('tab', 'credit');
		url.searchParams.set('page', String(p));
		goto(`${url.pathname}${url.search}`);
	}

	function changeLedgerPerPage(perPage: number) {
		const url = new URL(page.url);
		url.searchParams.set('tab', 'credit');
		url.searchParams.set('per_page', String(perPage));
		url.searchParams.set('page', '1');
		goto(`${url.pathname}${url.search}`);
	}
</script>

<svelte:head>
	<title>{t('clientcore.account.title')} — {t('common.appName')}</title>
</svelte:head>

<Breadcrumb
	items={[{ label: t('nav.home'), href: '/dashboard' }, { label: t('clientcore.account.title') }]}
/>

<h1 class="ca-h1">{t('clientcore.account.title')}</h1>

{#if data.meError}
	<div class="mb-4" data-testid="account-error">
		<Alert type="error">{data.meError}</Alert>
	</div>
{/if}

<div data-testid="account-tabs">
	<Tabs tabs={tabItems} active={data.tab} />
</div>

<div class="pt-5">
	{#if data.tab === 'profile'}
		<!-- ============ PROFILE ============ -->
		<section class="ca-card" data-testid="account-profile-panel">
			<h2 class="ca-card-header">
				{t('clientcore.account.profileTitle')}
			</h2>
			<div class="ca-card-body">
				<p class="ca-muted mb-4">{t('clientcore.account.profileDesc')}</p>

				{#if form?.profileMessage}
					<div class="mb-4">
						<Alert type="error">{serverText(form.profileMessage)}</Alert>
					</div>
				{/if}

				<form
					method="POST"
					action="?tab=profile&/updateProfile"
					data-testid="profile-form"
					use:enhance={() => {
						profileSubmitting = true;
						return async ({ result, update }) => {
							profileSubmitting = false;
							if (result.type === 'success') {
								toast.success(t('clientcore.account.profileSaved'));
							}
							await update({ reset: false });
						};
					}}
				>
					<div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
						<FormField
							label={t('auth.email')}
							name="email"
							type="email"
							value={data.profile.email}
							hint={t('clientcore.account.emailHint')}
							disabled
						/>
						<div></div>
						<FormField
							label={t('clientcore.account.firstName')}
							name="first_name"
							bind:value={pv.first_name}
							error={fieldError(form?.profileErrors, 'first_name')}
							required
						/>
						<FormField
							label={t('clientcore.account.lastName')}
							name="last_name"
							bind:value={pv.last_name}
							error={fieldError(form?.profileErrors, 'last_name')}
							required
						/>
						<FormField
							label={`${t('clientcore.account.company')} (${t('common.optional')})`}
							name="company"
							bind:value={pv.company}
							error={fieldError(form?.profileErrors, 'company')}
						/>
						<FormField
							label={t('clientcore.account.phone')}
							name="phone"
							bind:value={pv.phone}
							error={fieldError(form?.profileErrors, 'phone')}
							autocomplete="tel"
							required
						/>
						<FormField
							label={t('clientcore.account.address1')}
							name="address1"
							bind:value={pv.address1}
							error={fieldError(form?.profileErrors, 'address1')}
							required
						/>
						<FormField
							label={`${t('clientcore.account.address2')} (${t('common.optional')})`}
							name="address2"
							bind:value={pv.address2}
							error={fieldError(form?.profileErrors, 'address2')}
						/>
						<FormField
							label={t('clientcore.account.city')}
							name="city"
							bind:value={pv.city}
							error={fieldError(form?.profileErrors, 'city')}
							required
						/>
						<FormField
							label={t('clientcore.account.state')}
							name="state"
							bind:value={pv.state}
							error={fieldError(form?.profileErrors, 'state')}
							required
						/>
						<FormField
							label={t('clientcore.account.postcode')}
							name="postcode"
							bind:value={pv.postcode}
							error={fieldError(form?.profileErrors, 'postcode')}
							required
						/>
						<FormField
							label={t('clientcore.account.country')}
							name="country"
							bind:value={pv.country}
							error={fieldError(form?.profileErrors, 'country')}
							hint={t('clientcore.account.countryHint')}
							required
						/>
					</div>

					<div class="mt-2 flex justify-end">
						<span class="inline-flex" data-testid="profile-save">
							<LoadingButton type="submit" loading={profileSubmitting}>
								{t('action.save')}
							</LoadingButton>
						</span>
					</div>
				</form>
			</div>
		</section>
	{:else if data.tab === 'security'}
		<!-- ============ SECURITY ============ -->
		<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
			<section class="ca-card" data-testid="account-password-panel">
				<h2 class="ca-card-header">
					{t('clientcore.account.passwordTitle')}
				</h2>
				<div class="ca-card-body">
					{#if form?.passwordMessage}
						<div class="mb-4">
							<Alert type="error">{serverText(form.passwordMessage)}</Alert>
						</div>
					{/if}

					<form
						method="POST"
						action="?tab=security&/changePassword"
						data-testid="password-form"
						use:enhance={() => {
							passwordSubmitting = true;
							return async ({ result, update }) => {
								passwordSubmitting = false;
								if (result.type === 'success') {
									currentPassword = '';
									newPassword = '';
									confirmPassword = '';
									toast.success(t('clientcore.account.passwordSaved'));
								}
								await update({ reset: false });
							};
						}}
					>
						<FormField
							label={t('clientcore.account.currentPassword')}
							name="current_password"
							type="password"
							bind:value={currentPassword}
							error={fieldError(form?.passwordErrors, 'current_password')}
							autocomplete="current-password"
							required
						/>
						<FormField
							label={t('clientcore.account.newPassword')}
							name="new_password"
							type="password"
							bind:value={newPassword}
							error={fieldError(form?.passwordErrors, 'new_password')}
							hint={t('clientcore.validation.passwordMin')}
							autocomplete="new-password"
							required
						/>
						<FormField
							label={t('auth.confirmPassword')}
							name="confirm_password"
							type="password"
							bind:value={confirmPassword}
							error={fieldError(form?.passwordErrors, 'confirm_password')}
							autocomplete="new-password"
							required
						/>
						<div class="flex justify-end">
							<span class="inline-flex" data-testid="password-save">
								<LoadingButton type="submit" loading={passwordSubmitting}>
									{t('action.save')}
								</LoadingButton>
							</span>
						</div>
					</form>
				</div>
			</section>

			<section class="ca-card" data-testid="account-twofa-panel">
				<div class="ca-card-header">
					<span>{t('clientcore.account.twofaTitle')}</span>
					<span data-testid="twofa-status">
						<StatusBadge
							status={data.twofaEnabled ? 'active' : 'inactive'}
							label={data.twofaEnabled
								? t('clientcore.account.twofaOn')
								: t('clientcore.account.twofaOff')}
						/>
					</span>
				</div>
				<div class="ca-card-body">
					<p class="ca-muted mb-4">{t('clientcore.account.twofaDesc')}</p>

					{#if form?.twofaMessage}
						<div class="mb-4">
							<Alert type="error">{serverText(form.twofaMessage)}</Alert>
						</div>
					{/if}

					{#if data.twofaEnabled}
						<form
							method="POST"
							action="?tab=security&/disable2fa"
							bind:this={disableFormEl}
							data-testid="twofa-disable-form"
							use:enhance={() => {
								disableSubmitting = true;
								return async ({ result, update }) => {
									disableSubmitting = false;
									disableOpen = false;
									if (result.type === 'success') {
										disableCode = '';
										disablePassword = '';
										toast.success(t('clientcore.account.twofaDisabledToast'));
									}
									await update({ reset: false });
								};
							}}
						>
							<FormField
								label={t('auth.password')}
								name="password"
								type="password"
								bind:value={disablePassword}
								autocomplete="current-password"
							/>
							<FormField
								label={t('auth.totpCode')}
								name="totp_code"
								bind:value={disableCode}
								error={fieldError(
									form?.twofaCodeError ? { totp_code: form.twofaCodeError } : undefined,
									'totp_code'
								)}
								hint={t('clientcore.account.twofaDisableHint')}
								placeholder="123456"
								autocomplete="one-time-code"
							/>
							<span class="inline-flex" data-testid="twofa-disable-button">
								<LoadingButton
									variant="danger"
									loading={disableSubmitting}
									onclick={() => (disableOpen = true)}
								>
									{t('clientcore.account.twofaDisable')}
								</LoadingButton>
							</span>
						</form>

						<ConfirmDialog
							bind:open={disableOpen}
							title={t('clientcore.account.twofaDisable')}
							message={t('clientcore.account.twofaDisableConfirm')}
							danger
							loading={disableSubmitting}
							onConfirm={() => disableFormEl?.requestSubmit()}
						/>
					{:else if twofaSetup}
						<div class="mb-4 space-y-3">
							<div>
								<span class="mb-1 block text-sm font-medium text-gray-700">
									{t('clientcore.account.twofaSecret')}
								</span>
								<div class="flex items-center gap-2">
									<code
										class="block grow overflow-x-auto rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm"
										data-testid="twofa-secret"
									>
										{twofaSetup.secret}
									</code>
									<span class="inline-flex" data-testid="twofa-copy-secret">
										<LoadingButton
											variant="secondary"
											size="sm"
											onclick={() => copyText(twofaSetup?.secret ?? '')}
										>
											{t('clientcore.common.copy')}
										</LoadingButton>
									</span>
								</div>
							</div>
							{#if twofaSetup.otpauth}
								<div>
									<span class="mb-1 block text-sm font-medium text-gray-700">
										{t('clientcore.account.twofaOtpauth')}
									</span>
									<div class="flex items-center gap-2">
										<code
											class="block grow overflow-x-auto rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-xs break-all"
											data-testid="twofa-otpauth"
										>
											{twofaSetup.otpauth}
										</code>
										<span class="inline-flex" data-testid="twofa-copy-otpauth">
											<LoadingButton
												variant="secondary"
												size="sm"
												onclick={() => copyText(twofaSetup?.otpauth ?? '')}
											>
												{t('clientcore.common.copy')}
											</LoadingButton>
										</span>
									</div>
								</div>
							{/if}
							<p class="text-xs text-gray-500">{t('clientcore.account.twofaManualHint')}</p>
						</div>

						<form
							method="POST"
							action="?tab=security&/enable2fa"
							data-testid="twofa-enable-form"
							use:enhance={() => {
								enableSubmitting = true;
								return async ({ result, update }) => {
									enableSubmitting = false;
									if (result.type === 'success') {
										enableCode = '';
										toast.success(t('clientcore.account.twofaEnabledToast'));
									}
									await update({ reset: false });
								};
							}}
						>
							<FormField
								label={t('auth.totpCode')}
								name="totp_code"
								bind:value={enableCode}
								error={fieldError(
									form?.twofaCodeError ? { totp_code: form.twofaCodeError } : undefined,
									'totp_code'
								)}
								placeholder="123456"
								autocomplete="one-time-code"
								required
							/>
							<span class="inline-flex" data-testid="twofa-enable-submit">
								<LoadingButton type="submit" loading={enableSubmitting}>
									{t('clientcore.account.twofaEnable')}
								</LoadingButton>
							</span>
						</form>
					{:else}
						<form
							method="POST"
							action="?tab=security&/setup2fa"
							data-testid="twofa-setup-form"
							use:enhance={() => {
								setupSubmitting = true;
								return async ({ update }) => {
									setupSubmitting = false;
									await update({ reset: false });
								};
							}}
						>
							<span class="inline-flex" data-testid="twofa-setup-button">
								<LoadingButton type="submit" loading={setupSubmitting}>
									{t('clientcore.account.twofaSetupButton')}
								</LoadingButton>
							</span>
						</form>
					{/if}
				</div>
			</section>
		</div>
	{:else if data.tab === 'contacts'}
		<!-- ============ CONTACTS ============ -->
		<section data-testid="account-contacts-panel">
			<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="ca-h3" style="margin-bottom:2px">
						{t('clientcore.account.contactsTitle')}
					</h2>
					<p class="ca-muted">{t('clientcore.account.contactsDesc')}</p>
				</div>
				<span class="inline-flex" data-testid="contact-add-button">
					<LoadingButton onclick={openAddContact}>
						{t('clientcore.account.addContact')}
					</LoadingButton>
				</span>
			</div>

			{#if data.contactsError}
				<div class="mb-4" data-testid="contacts-error">
					<Alert type="error">{data.contactsError}</Alert>
				</div>
			{/if}

			<DataTable
				rows={data.contacts}
				columns={contactColumns}
				emptyTitle={t('clientcore.account.noContacts')}
				emptyDescription={t('clientcore.account.noContactsDesc')}
			>
				{#snippet cell({ row, column, value })}
					{#if column.key === 'name'}
						<span class="font-medium text-gray-800" data-testid={`row-contact-${row.id}`}>
							{row.first_name}
							{row.last_name}
						</span>
					{:else if column.key === 'permissions'}
						<span class="text-xs text-gray-500">{permSummary(row.permissions)}</span>
					{:else if column.key === 'actions'}
						<div class="flex justify-end gap-2">
							<span class="inline-flex" data-testid={`contact-edit-${row.id}`}>
								<LoadingButton variant="ghost" size="sm" onclick={() => openEditContact(row)}>
									{t('action.edit')}
								</LoadingButton>
							</span>
							<span class="inline-flex" data-testid={`contact-delete-${row.id}`}>
								<LoadingButton variant="danger" size="sm" onclick={() => askDelete(row)}>
									{t('action.delete')}
								</LoadingButton>
							</span>
						</div>
					{:else}
						{String(value ?? '—')}
					{/if}
				{/snippet}
			</DataTable>

			<Modal
				bind:open={contactModalOpen}
				title={cf.id ? t('clientcore.account.editContact') : t('clientcore.account.addContact')}
			>
				{#if form?.contactMessage}
					<div class="mb-4">
						<Alert type="error">{serverText(form.contactMessage)}</Alert>
					</div>
				{/if}

				<form
					method="POST"
					action="?tab=contacts&/saveContact"
					data-testid="contact-form"
					use:enhance={() => {
						contactSubmitting = true;
						return async ({ result, update }) => {
							contactSubmitting = false;
							if (result.type === 'success') {
								contactModalOpen = false;
								toast.success(t('clientcore.account.contactSaved'));
							}
							await update({ reset: false });
						};
					}}
				>
					<input type="hidden" name="id" value={cf.id} />
					<div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
						<FormField
							label={t('clientcore.account.firstName')}
							name="first_name"
							bind:value={cf.first_name}
							error={fieldError(form?.contactErrors, 'first_name')}
							required
						/>
						<FormField
							label={`${t('clientcore.account.lastName')} (${t('common.optional')})`}
							name="last_name"
							bind:value={cf.last_name}
							error={fieldError(form?.contactErrors, 'last_name')}
						/>
						<FormField
							label={t('auth.email')}
							name="email"
							type="email"
							bind:value={cf.email}
							error={fieldError(form?.contactErrors, 'email')}
							required
						/>
						<FormField
							label={`${t('clientcore.account.phone')} (${t('common.optional')})`}
							name="phone"
							bind:value={cf.phone}
							error={fieldError(form?.contactErrors, 'phone')}
						/>
					</div>

					<fieldset class="mb-4">
						<legend class="mb-2 text-sm font-medium text-gray-700">
							{t('clientcore.account.permissions')}
						</legend>
						<div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
							<FormField
								label={t('clientcore.account.permInvoices')}
								name="perm_invoices"
								type="checkbox"
								bind:value={cf.perm_invoices}
							/>
							<FormField
								label={t('clientcore.account.permServices')}
								name="perm_services"
								type="checkbox"
								bind:value={cf.perm_services}
							/>
							<FormField
								label={t('clientcore.account.permDomains')}
								name="perm_domains"
								type="checkbox"
								bind:value={cf.perm_domains}
							/>
							<FormField
								label={t('clientcore.account.permTickets')}
								name="perm_tickets"
								type="checkbox"
								bind:value={cf.perm_tickets}
							/>
						</div>
					</fieldset>

					<div class="flex justify-end gap-2">
						<LoadingButton
							variant="secondary"
							onclick={() => (contactModalOpen = false)}
							disabled={contactSubmitting}
						>
							{t('action.cancel')}
						</LoadingButton>
						<span class="inline-flex" data-testid="contact-save">
							<LoadingButton type="submit" loading={contactSubmitting}>
								{t('action.save')}
							</LoadingButton>
						</span>
					</div>
				</form>
			</Modal>

			<form
				method="POST"
				action="?tab=contacts&/deleteContact"
				class="hidden"
				bind:this={deleteFormEl}
				use:enhance={() => {
					deleteSubmitting = true;
					return async ({ result, update }) => {
						deleteSubmitting = false;
						deleteOpen = false;
						if (result.type === 'success') {
							deleteTarget = null;
							toast.success(t('clientcore.account.contactDeleted'));
						} else if (result.type === 'failure') {
							toast.error(t('toast.error'));
						}
						await update({ reset: false });
					};
				}}
			>
				<input type="hidden" name="id" value={deleteTarget?.id ?? ''} />
			</form>

			<ConfirmDialog
				bind:open={deleteOpen}
				title={t('clientcore.account.deleteContact')}
				message={t('clientcore.account.deleteContactMessage', {
					name: `${deleteTarget?.first_name ?? ''} ${deleteTarget?.last_name ?? ''}`.trim()
				})}
				danger
				loading={deleteSubmitting}
				onConfirm={() => deleteFormEl?.requestSubmit()}
				onCancel={() => (deleteTarget = null)}
			/>
		</section>
	{:else if data.tab === 'credit'}
		<!-- ============ CREDIT ============ -->
		<section data-testid="account-credit-panel">
			<div class="ca-card" style="border-left:4px solid var(--ca-success)">
				<div class="ca-card-body flex flex-wrap items-center justify-between gap-4">
					<div>
						<p class="text-xs font-semibold tracking-wide text-gray-500 uppercase">
							{t('clientcore.account.creditTitle')}
						</p>
						<p class="mt-1 text-3xl font-bold text-gray-800" data-testid="credit-balance">
							<MoneyText amount={data.creditBalance} />
						</p>
						<p class="ca-muted" style="margin-top:4px">{t('clientcore.account.creditDesc')}</p>
					</div>
					<a href="/billing/deposit" class="ca-btn ca-btn-primary" data-testid="deposit-link">
						{t('billing.addFunds')}
					</a>
				</div>
			</div>

			{#if data.ledgerError}
				<div class="mb-4" data-testid="ledger-error">
					<Alert type="error">{data.ledgerError}</Alert>
				</div>
			{/if}

			<h2 class="ca-h3">
				{t('clientcore.account.ledgerTitle')}
			</h2>
			<div data-testid="credit-ledger-table">
				<DataTable
					rows={data.ledger}
					columns={ledgerColumns}
					page={data.ledgerMeta.page}
					perPage={data.ledgerMeta.per_page}
					total={data.ledgerMeta.total}
					onPageChange={gotoLedgerPage}
					onPerPageChange={changeLedgerPerPage}
					emptyTitle={t('clientcore.account.noLedger')}
					emptyDescription={t('clientcore.account.noLedgerDesc')}
				>
					{#snippet cell({ row, column, value })}
						{#if column.key === 'created_at'}
							<span data-testid={`row-credit-${row.id}`}>
								<DateText value={row.created_at} mode="datetime" />
							</span>
						{:else if column.key === 'delta'}
							<span class={row.delta >= 0 ? 'font-medium text-success' : 'font-medium text-danger'}>
								{row.delta >= 0 ? '+' : ''}<MoneyText amount={row.delta} />
							</span>
						{:else if column.key === 'balance_after'}
							<MoneyText amount={row.balance_after} />
						{:else}
							{String(value ?? '—')}
						{/if}
					{/snippet}
				</DataTable>
			</div>
		</section>
	{/if}
</div>
