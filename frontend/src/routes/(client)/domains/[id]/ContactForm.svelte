<script lang="ts">
	import { enhance } from '$app/forms';
	import Alert from '$lib/components/Alert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import LoadingButton from '$lib/components/LoadingButton.svelte';
	import { t } from '$lib/i18n';
	import { toast } from '$lib/stores/toast.svelte';
	import type { SubmitFunction } from '@sveltejs/kit';
	import type { RegistrantContact } from '../shared';

	interface Props {
		contact: RegistrantContact | null;
		errorText?: string | null;
		fieldErrors?: Record<string, string> | null;
		values?: Partial<RegistrantContact> | null;
	}

	let { contact, errorText = null, fieldErrors = null, values = null }: Props = $props();

	const init = (key: keyof RegistrantContact, fallback = ''): string =>
		values?.[key] ?? contact?.[key] ?? fallback;

	let firstName = $state(init('first_name'));
	let lastName = $state(init('last_name'));
	let company = $state(init('company'));
	let email = $state(init('email'));
	let phone = $state(init('phone'));
	let address1 = $state(init('address1'));
	let city = $state(init('city'));
	let stateProv = $state(init('state'));
	let postcode = $state(init('postcode'));
	let country = $state(init('country', 'ID'));

	let submitting = $state(false);

	const fieldError = (key: string): string | undefined =>
		fieldErrors?.[key] ? t(fieldErrors[key]) : undefined;

	const submitContact: SubmitFunction = () => {
		submitting = true;
		return async ({ result, update }) => {
			submitting = false;
			if (result.type === 'success') {
				toast.success(t('clientDomains.contact.saved'));
			}
			await update();
		};
	};
</script>

<div class="ca-card">
	<h2 class="ca-card-header">{t('clientDomains.contact.title')}</h2>
	<div class="ca-card-body">
		<p class="ca-muted mb-4">{t('clientDomains.contact.desc')}</p>

		{#if errorText}
			<div class="mb-4" data-testid="contact-error">
				<Alert type="error">{errorText}</Alert>
			</div>
		{/if}

		<form method="POST" action="?/contact" use:enhance={submitContact} data-testid="contact-form">
			<div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
				<FormField
					label={t('clientDomains.contact.firstName')}
					name="first_name"
					bind:value={firstName}
					error={fieldError('first_name')}
					required
				/>
				<FormField
					label={t('clientDomains.contact.lastName')}
					name="last_name"
					bind:value={lastName}
					error={fieldError('last_name')}
					required
				/>
				<FormField
					label={`${t('clientDomains.contact.company')} (${t('common.optional')})`}
					name="company"
					bind:value={company}
					error={fieldError('company')}
				/>
				<FormField
					label={t('clientDomains.contact.email')}
					name="email"
					type="email"
					bind:value={email}
					error={fieldError('email')}
					required
				/>
				<FormField
					label={t('clientDomains.contact.phone')}
					name="phone"
					bind:value={phone}
					error={fieldError('phone')}
					placeholder="+62…"
					required
				/>
				<FormField
					label={t('clientDomains.contact.address')}
					name="address1"
					bind:value={address1}
					error={fieldError('address1')}
					required
				/>
				<FormField
					label={t('clientDomains.contact.city')}
					name="city"
					bind:value={city}
					error={fieldError('city')}
					required
				/>
				<FormField
					label={t('clientDomains.contact.state')}
					name="state"
					bind:value={stateProv}
					error={fieldError('state')}
					required
				/>
				<FormField
					label={t('clientDomains.contact.postcode')}
					name="postcode"
					bind:value={postcode}
					error={fieldError('postcode')}
					required
				/>
				<FormField
					label={t('clientDomains.contact.country')}
					name="country"
					bind:value={country}
					error={fieldError('country')}
					placeholder="ID"
					required
				/>
			</div>

			<div class="mt-2 flex justify-end">
				<span data-testid="contact-save">
					<LoadingButton type="submit" loading={submitting}>
						{t('clientDomains.contact.save')}
					</LoadingButton>
				</span>
			</div>
		</form>
	</div>
</div>
