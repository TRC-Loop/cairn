// SPDX-License-Identifier: AGPL-3.0-or-later
import { auth } from '$lib/stores/auth';
import { get } from 'svelte/store';
import { redirect } from '@sveltejs/kit';

export const ssr = false;

export function load() {
	const state = get(auth);
	if (!state.user || state.user.role !== 'admin') {
		throw redirect(307, '/settings/profile');
	}
	return {};
}
