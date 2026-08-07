<script lang="ts">
	import { i18n, LOCALES, t } from '$lib/i18n';

	/**
	 * Language/currency chooser (DESIGN §11). Deliberately self-contained (NOT the
	 * shared Modal) so it never exposes another `role="dialog"` that Playwright's
	 * getByRole('dialog') could collide with.
	 */
	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	const LOCALE_LABELS: Record<string, string> = {
		id: 'Bahasa Indonesia',
		en: 'English'
	};

	function choose(locale: (typeof LOCALES)[number]) {
		i18n.setLocale(locale);
		onClose();
	}

	function onKey(e: KeyboardEvent) {
		if (open && e.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={onKey} />

{#if open}
	<div class="ca-langmodal-scrim">
		<button
			type="button"
			class="ca-langmodal-backdrop"
			aria-label={t('portal.footer.close')}
			onclick={onClose}
		></button>
		<div class="ca-langmodal" aria-label={t('portal.footer.language')}>
			<div class="ca-langmodal-hd">
				<span>{t('portal.footer.chooseLanguage')}</span>
				<button
					type="button"
					class="ca-langmodal-x"
					onclick={onClose}
					aria-label={t('portal.footer.close')}
				>
					<i class="fas fa-times" aria-hidden="true"></i>
				</button>
			</div>
			<div class="ca-langmodal-bd">
				{#each LOCALES as locale (locale)}
					<button
						type="button"
						class="ca-langmodal-opt"
						class:active={i18n.locale === locale}
						onclick={() => choose(locale)}
					>
						<span>{LOCALE_LABELS[locale] ?? locale.toUpperCase()}</span>
						<span class="cur">Rp IDR</span>
						{#if i18n.locale === locale}
							<i class="fas fa-check" aria-hidden="true"></i>
						{/if}
					</button>
				{/each}
			</div>
		</div>
	</div>
{/if}

<style>
	.ca-langmodal-scrim {
		position: fixed;
		inset: 0;
		z-index: 90;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 90px;
	}
	.ca-langmodal-backdrop {
		position: fixed;
		inset: 0;
		border: none;
		padding: 0;
		background: rgba(0, 0, 0, 0.45);
		cursor: default;
	}
	.ca-langmodal {
		position: relative;
		background: #fff;
		border-radius: 4px;
		width: 380px;
		max-width: 92vw;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
		overflow: hidden;
		color: #212529;
		font-family:
			'Open Sans',
			-apple-system,
			'Segoe UI',
			sans-serif;
	}
	.ca-langmodal-hd {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 14px 18px;
		border-bottom: 1px solid #eee;
		font-size: 16px;
		font-weight: 600;
	}
	.ca-langmodal-x {
		border: none;
		background: none;
		cursor: pointer;
		color: #999;
		font-size: 16px;
	}
	.ca-langmodal-bd {
		padding: 10px;
	}
	.ca-langmodal-opt {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 11px 14px;
		border: 1px solid transparent;
		border-radius: 4px;
		background: none;
		cursor: pointer;
		font: inherit;
		text-align: left;
		color: #212529;
	}
	.ca-langmodal-opt:hover {
		background: #f4f7fa;
	}
	.ca-langmodal-opt.active {
		border-color: #336699;
		color: #336699;
		font-weight: 600;
	}
	.ca-langmodal-opt .cur {
		margin-left: auto;
		color: #6c757d;
		font-size: 0.85rem;
	}
	.ca-langmodal-opt.active .cur {
		color: #336699;
	}
	.ca-langmodal-opt .fa-check {
		color: #336699;
	}
</style>
