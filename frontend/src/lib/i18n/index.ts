// SPDX-License-Identifier: AGPL-3.0-or-later
import { browser } from '$app/environment';
import { addMessages, init } from 'svelte-i18n';
import en from './locales/en.json';
import de from './locales/de.json';

const SUPPORTED_LOCALES = ['en', 'de'];
const FALLBACK_LOCALE = 'en';

addMessages('en', en);
addMessages('de', de);

export function setupI18n() {
	let locale: string = FALLBACK_LOCALE;
	if (browser) {
		try {
			const stored = localStorage.getItem('cairn_language');
			if (stored && SUPPORTED_LOCALES.includes(stored)) {
				locale = stored;
			} else {
				locale = SUPPORTED_LOCALES.find((l) => navigator.language.startsWith(l)) ?? FALLBACK_LOCALE;
			}
		} catch {
			locale = SUPPORTED_LOCALES.find((l) => navigator.language.startsWith(l)) ?? FALLBACK_LOCALE;
		}
	}

	return init({ fallbackLocale: FALLBACK_LOCALE, initialLocale: locale });
}
