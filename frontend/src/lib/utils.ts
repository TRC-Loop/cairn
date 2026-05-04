// SPDX-License-Identifier: AGPL-3.0-or-later
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { get } from 'svelte/store';
import { _ } from 'svelte-i18n';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChild<T> = T extends { child?: any } ? Omit<T, 'child'> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, 'children'> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };

export function relativeTime(iso: string | null | undefined): string {
	if (!iso) return get(_)('common.na');
	const then = new Date(iso).getTime();
	if (Number.isNaN(then)) return get(_)('common.na');
	const diffMs = Date.now() - then;
	const sec = Math.round(diffMs / 1000);
	if (sec < 60) return `${Math.max(0, sec)}s ago`;
	const min = Math.round(sec / 60);
	if (min < 60) return `${min}m ago`;
	const hr = Math.round(min / 60);
	if (hr < 48) return `${hr}h ago`;
	const d = Math.round(hr / 24);
	return `${d}d ago`;
}

export function statusColorVar(status: string): string {
	switch (status) {
		case 'up':
			return 'var(--status-up)';
		case 'down':
			return 'var(--status-down)';
		case 'degraded':
			return 'var(--status-degraded)';
		case 'paused':
			return 'var(--status-maintenance)';
		default:
			return 'var(--status-unknown)';
	}
}
