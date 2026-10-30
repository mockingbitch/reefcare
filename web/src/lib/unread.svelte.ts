// Unread notification count shown in the navigation. Refreshed on navigation and after
// the notifications page marks items read.
import { api } from '#lib/api/client.js';

class Unread {
	count = $state(0);
	capped = $state(false);

	async refresh(): Promise<void> {
		const r = await api<{ count: number; capped: boolean }>('GET', '/notifications/unread-count');
		if (r.ok) {
			this.count = r.data.count;
			this.capped = r.data.capped;
		}
	}

	clear(): void {
		this.count = 0;
		this.capped = false;
	}
}

export const unread = new Unread();
