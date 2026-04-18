// SPDX-License-Identifier: AGPL-3.0-or-later
import { setupI18n } from '$lib/i18n';

export const prerender = false;
export const ssr = false;

export async function load() {
	await setupI18n();
}
