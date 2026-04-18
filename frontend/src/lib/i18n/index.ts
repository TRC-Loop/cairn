// SPDX-License-Identifier: AGPL-3.0-or-later
import { browser } from '$app/environment';
import { init } from 'svelte-i18n';

const SUPPORTED_LOCALES = ['en', 'de'];
const FALLBACK_LOCALE = 'en';

export function setupI18n() {
	const locale = browser
		? (SUPPORTED_LOCALES.find((l) => navigator.language.startsWith(l)) ?? FALLBACK_LOCALE)
		: FALLBACK_LOCALE;

	return init({ fallbackLocale: FALLBACK_LOCALE, initialLocale: locale });
}
