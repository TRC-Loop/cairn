// SPDX-License-Identifier: AGPL-3.0-or-later
import { toast as sonnerToast } from 'svelte-sonner';
import { get } from 'svelte/store';
import { _ } from 'svelte-i18n';
import { ApiError } from './api/client';

export function toastSuccess(message: string) {
	sonnerToast.success(message, { duration: 3000 });
}

export function toastError(err: unknown, fallback?: string) {
	const t = get(_);
	const fallbackText = fallback ?? t('common.error_generic');

	if (err instanceof ApiError) {
		const key = `errors.${err.code}`;
		const localized = t(key);
		const title = localized !== key ? localized : err.description || fallbackText;
		const description = err.description && err.description !== title ? err.description : undefined;
		sonnerToast.error(title, { description, duration: Infinity });
		return;
	}

	const title = err instanceof Error && err.message ? err.message : fallbackText;
	sonnerToast.error(title, { duration: Infinity });
}
