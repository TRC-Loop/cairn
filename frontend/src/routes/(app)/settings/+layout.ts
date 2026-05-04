// SPDX-License-Identifier: AGPL-3.0-or-later
import { auth } from '$lib/stores/auth';
import { get } from 'svelte/store';
import { redirect } from '@sveltejs/kit';

export const ssr = false;

export async function load() {
	const state = get(auth);
	if (!state.user) {
		throw redirect(307, '/login');
	}
	return { user: state.user };
}
