// SPDX-License-Identifier: AGPL-3.0-or-later
import { setupI18n } from '$lib/i18n';
import { fetchSetupStatus } from '$lib/api/setup';
import { auth } from '$lib/stores/auth';
import { get } from 'svelte/store';
import { redirect } from '@sveltejs/kit';

export const prerender = false;
export const ssr = false;

export async function load({ url }) {
	await setupI18n();

	const path = url.pathname;
	const { setup_complete } = await fetchSetupStatus();

	if (!setup_complete) {
		if (path !== '/setup') {
			throw redirect(307, '/setup');
		}
		return { setupComplete: false };
	}

	if (path === '/setup') {
		throw redirect(307, '/login');
	}

	await auth.init();
	const state = get(auth);

	if (state.user && path === '/login') {
		throw redirect(307, '/dashboard');
	}

	return { setupComplete: true };
}
