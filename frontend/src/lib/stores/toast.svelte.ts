import { browser } from '$app/environment';

export type ToastType = 'success' | 'error' | 'info';

export interface ToastItem {
	id: number;
	type: ToastType;
	message: string;
}

const DEFAULT_TIMEOUT_MS = 4500;

class ToastStore {
	items = $state<ToastItem[]>([]);
	#seq = 0;

	show(type: ToastType, message: string, timeoutMs = DEFAULT_TIMEOUT_MS): number {
		const id = ++this.#seq;
		this.items = [...this.items, { id, type, message }];
		if (browser && timeoutMs > 0) {
			setTimeout(() => this.dismiss(id), timeoutMs);
		}
		return id;
	}

	success(message: string, timeoutMs?: number): number {
		return this.show('success', message, timeoutMs);
	}

	error(message: string, timeoutMs?: number): number {
		return this.show('error', message, timeoutMs);
	}

	info(message: string, timeoutMs?: number): number {
		return this.show('info', message, timeoutMs);
	}

	dismiss(id: number): void {
		this.items = this.items.filter((t) => t.id !== id);
	}

	clear(): void {
		this.items = [];
	}
}

/** Global toast store - render `<Toast />` once in the root layout as the outlet. */
export const toast = new ToastStore();
