<script lang="ts">
	import { t } from '$lib/i18n';

	/** Twenty-One store sidebar (DESIGN §8): collapsible Categories + Actions cards. */
	interface Category {
		label: string;
		href: string;
		active?: boolean;
		testid?: string;
	}
	interface Action {
		icon: string;
		label: string;
		href: string;
	}

	interface Props {
		categories: Category[];
		actions: Action[];
		/** data-testid on the categories <ul>, e.g. for the store's group list. */
		categoriesTestId?: string;
	}

	let { categories, actions, categoriesTestId }: Props = $props();

	let catOpen = $state(true);
	let actOpen = $state(true);
</script>

<aside class="ca-sidebar">
	{#if categories.length}
		<div class="ca-sidebar-card">
			<button
				type="button"
				class="ca-sidebar-head"
				aria-expanded={catOpen}
				onclick={() => (catOpen = !catOpen)}
			>
				<i class="fas fa-shopping-cart" aria-hidden="true"></i>
				<span>{t('portal.store.categories')}</span>
				<i class="chevron fas {catOpen ? 'fa-chevron-up' : 'fa-chevron-down'}" aria-hidden="true"
				></i>
			</button>
			{#if catOpen}
				<ul class="ca-list-group" data-testid={categoriesTestId}>
					{#each categories as c (c.href + c.label)}
						<li>
							<a
								class="ca-list-item"
								class:ca-list-item--active={c.active}
								href={c.href}
								data-testid={c.testid}
							>
								{c.label}
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}

	{#if actions.length}
		<div class="ca-sidebar-card">
			<button
				type="button"
				class="ca-sidebar-head"
				aria-expanded={actOpen}
				onclick={() => (actOpen = !actOpen)}
			>
				<i class="fas fa-plus" aria-hidden="true"></i>
				<span>{t('portal.store.actions')}</span>
				<i class="chevron fas {actOpen ? 'fa-chevron-up' : 'fa-chevron-down'}" aria-hidden="true"
				></i>
			</button>
			{#if actOpen}
				<ul class="ca-list-group">
					{#each actions as a (a.href + a.label)}
						<li>
							<a class="ca-list-item" href={a.href}>
								<i class={a.icon} aria-hidden="true"></i>
								<span>{a.label}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</aside>
